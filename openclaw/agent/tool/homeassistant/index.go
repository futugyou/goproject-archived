package homeassistant

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type Entry struct {
	EntityId     string `json:"entity_id,omitempty"`
	FriendlyName string `json:"friendly_name,omitempty"`
	AreaName     string `json:"area,omitempty"`
	DeviceName   string `json:"device,omitempty"`
	Platform     string `json:"platform,omitempty"`
	State        string `json:"state,omitempty"`
}

const (
	RegistryRefreshMinutes int = 15
	ServicesRefreshMinutes int = 15
	StatesRefreshSeconds   int = 10
)

type HomeAssistantIndex struct {
	config              core.HomeAssistantConfig
	wsapi               *HomeAssistantWsApi
	lastStatesRefresh   time.Time
	lastRegistryRefresh time.Time
	lastServicesRefresh time.Time
	byEntityId          map[string]*Entry
	servicesByDomain    map[string][]string

	mu sync.Mutex
}

func NewHomeAssistantIndex(config core.HomeAssistantConfig) *HomeAssistantIndex {
	return &HomeAssistantIndex{
		config:           config,
		wsapi:            NewHomeAssistantWsApi(config),
		byEntityId:       map[string]*Entry{},
		servicesByDomain: map[string][]string{},
	}
}

func (h *HomeAssistantIndex) Get(entityId string) *Entry {
	h.mu.Lock()
	defer h.mu.Unlock()

	if v, ok := h.byEntityId[entityId]; ok {
		return v
	}
	return nil
}

type AssistantState struct {
	EntityId   string  `json:"entity_id"`
	State      *string `json:"state,omitempty"`
	Attributes struct {
		FriendlyName string `json:"friendly_name"`
	} `json:"attributes"`
}

func (h *HomeAssistantIndex) refreshStatesFromJson(raw string) {
	var docs []AssistantState
	if err := json.Unmarshal([]byte(raw), &docs); err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, item := range docs {
		if item.EntityId == "" {
			continue
		}

		entry, ok := h.byEntityId[item.EntityId]
		if !ok {
			entry = &Entry{EntityId: item.EntityId}
		}

		if item.State != nil {
			entry.State = *item.State
		}

		if item.Attributes.FriendlyName != "" {
			entry.FriendlyName = item.Attributes.FriendlyName
		}

		h.byEntityId[item.EntityId] = entry
	}
}

type ServiceState struct {
	Domain   string         `json:"domain"`
	Services map[string]any `json:"services"`
}

func (h *HomeAssistantIndex) refreshServicesFromJson(raw string) {
	var docs []ServiceState
	if err := json.Unmarshal([]byte(raw), &docs); err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.servicesByDomain = make(map[string][]string)

	for _, item := range docs {
		if item.Domain == "" {
			continue
		}

		var keys []string
		for k := range maps.Keys(item.Services) {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		h.servicesByDomain[item.Domain] = keys
	}
}

func (h *HomeAssistantIndex) Query(domain, area, name string) []*Entry {
	var snapshot []*Entry

	h.mu.Lock()
	for _, v := range h.byEntityId {
		snapshot = append(snapshot, v)
	}
	h.mu.Unlock()

	if domain != "" {
		snapshot = slices.DeleteFunc(snapshot, func(e *Entry) bool {
			return !strings.HasPrefix(e.EntityId, domain+".")
		})
	}

	if area != "" {
		snapshot = slices.DeleteFunc(snapshot, func(e *Entry) bool {
			return e.AreaName == "" || !strings.EqualFold(e.AreaName, area)
		})
	}

	if name != "" {
		snapshot = slices.DeleteFunc(snapshot, func(e *Entry) bool {
			return (e.FriendlyName == "" || !strings.Contains(e.FriendlyName, name) && !strings.Contains(e.EntityId, name))
		})
	}

	slices.SortFunc(snapshot, func(a, b *Entry) int {
		aHasFriendly := !util.IsBlank(a.FriendlyName)
		bHasFriendly := !util.IsBlank(b.FriendlyName)
		if aHasFriendly != bHasFriendly {
			if aHasFriendly {
				return -1
			}
			return 1
		}

		aHasArea := !util.IsBlank(a.AreaName)
		bHasArea := !util.IsBlank(b.AreaName)
		if aHasArea != bHasArea {
			if aHasArea {
				return -1
			}
			return 1
		}

		return strings.Compare(strings.ToLower(a.EntityId), strings.ToLower(b.EntityId))
	})

	return snapshot
}

func (h *HomeAssistantIndex) GetServicesForDomain(ctx context.Context, rest HomeAssistantRestClient, domain string) ([]string, error) {
	var shouldRefresh = false
	h.mu.Lock()
	if time.Now().UTC().Sub(h.lastServicesRefresh) > time.Duration(ServicesRefreshMinutes)*(time.Minute) {
		h.lastServicesRefresh = time.Now().UTC()
		shouldRefresh = true
	}
	h.mu.Unlock()

	if shouldRefresh {
		raw, err := rest.GetServices(ctx)
		if err != nil {
			return nil, err
		}
		h.refreshServicesFromJson(raw)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if v, ok := h.servicesByDomain[domain]; ok {
		return v, nil
	}

	return []string{}, nil
}

func (h *HomeAssistantIndex) EnsureWarm(ctx context.Context, rest HomeAssistantRestClient) error {
	var shouldRefresh = false
	h.mu.Lock()
	if time.Now().UTC().Sub(h.lastStatesRefresh) > time.Duration(StatesRefreshSeconds)*(time.Second) {
		h.lastStatesRefresh = time.Now().UTC()
		shouldRefresh = true
	}
	h.mu.Unlock()

	if !shouldRefresh {
		return nil
	}

	raw, err := rest.GetStates(ctx)
	if err != nil {
		return err
	}
	h.refreshStatesFromJson(raw)
	return nil
}

func (h *HomeAssistantIndex) RefreshRegistries(ctx context.Context) error {
	var shouldRefresh = false
	h.mu.Lock()
	if time.Now().UTC().Sub(h.lastRegistryRefresh) > time.Duration(RegistryRefreshMinutes)*(time.Minute) {
		h.lastRegistryRefresh = time.Now().UTC()
		shouldRefresh = true
	}
	h.mu.Unlock()

	if !shouldRefresh {
		return nil
	}

	var areas = h.wsapi.ListAreas(ctx)
	var devices = h.wsapi.ListDevices(ctx)
	var entities = h.wsapi.ListEntityRegistry(ctx)

	h.mu.Lock()
	defer h.mu.Unlock()

	areaById := util.SliceToMap(areas, func(a AreaEntry) string { return a.AreaId }, func(a AreaEntry) string { return a.Name })
	deviceById := util.SliceToMap(devices, func(a DeviceEntry) string { return a.DeviceId }, func(a DeviceEntry) DeviceEntry { return a })

	for _, er := range entities {
		entry, ok := h.byEntityId[er.EntityId]
		if !ok {
			entry = &Entry{EntityId: er.EntityId}
		}

		entry.Platform = er.Platform

		var areaId = er.AreaId
		if areaId == "" && er.DeviceId != "" {
			if dev, ok := deviceById[er.DeviceId]; ok {
				areaId = dev.AreaId
			}
		}

		if areaId != "" {
			if dev, ok := areaById[areaId]; ok {
				entry.AreaName = dev
			}
		}
		if er.DeviceId != "" {
			if dev, ok := deviceById[er.DeviceId]; ok {
				entry.DeviceName = dev.Name
			}
		}

		if er.Name != "" {
			entry.FriendlyName = er.Name
		}

		h.byEntityId[er.EntityId] = entry
	}

	return nil
}
