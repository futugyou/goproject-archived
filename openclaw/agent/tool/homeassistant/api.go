package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/futugyou/openclaw/core"
	"github.com/gorilla/websocket"
)

type AreaEntry struct {
	AreaId string
	Name   string
}
type DeviceEntry struct {
	DeviceId string
	Name     string
	AreaId   string
}
type EntityRegistryEntry struct {
	EntityId string
	Platform string
	DeviceId string
	AreaId   string
	Name     string
}

type HAMessage struct {
	ID      int             `json:"id,omitempty"`
	Type    string          `json:"type"` // type auth_required, auth_ok, result, event
	Success bool            `json:"success,omitempty"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func parseAreas(data json.RawMessage) []AreaEntry {
	type innerAreaEntry struct {
		AreaId string `json:"area_id,omitempty"`
		Name   string `json:"name,omitempty"`
	}

	var raw []innerAreaEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return []AreaEntry{}
	}
	list := []AreaEntry{}
	for _, item := range raw {
		if item.AreaId != "" && item.Name != "" {
			list = append(list, AreaEntry{
				AreaId: item.AreaId,
				Name:   item.Name,
			})
		}
	}

	return list
}

func parseDevices(data json.RawMessage) []DeviceEntry {
	type innerDeviceEntry struct {
		DeviceId   string `json:"id,omitempty"`
		NameByUser string `json:"name_by_user,omitempty"`
		Name       string `json:"name,omitempty"`
		AreaId     string `json:"area_id,omitempty"`
	}
	var raw []innerDeviceEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return []DeviceEntry{}
	}
	list := []DeviceEntry{}
	for _, item := range raw {
		if item.AreaId != "" && item.Name != "" {
			name := item.NameByUser
			if name == "" {
				name = item.Name
			}
			if item.DeviceId != "" {

				list = append(list, DeviceEntry{
					DeviceId: item.DeviceId,
					Name:     name,
					AreaId:   item.AreaId,
				})
			}
		}
	}

	return list
}

func parseEntityRegistry(data json.RawMessage) []EntityRegistryEntry {
	type innerEntityRegistryEntry struct {
		EntityId string `json:"entity_id,omitempty"`
		Platform string `json:"platform,omitempty"`
		DeviceId string `json:"device_id,omitempty"`
		AreaId   string `json:"area_id,omitempty"`
		Name     string `json:"name,omitempty"`
	}
	var raw []innerEntityRegistryEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return []EntityRegistryEntry{}
	}
	list := []EntityRegistryEntry{}
	for _, item := range raw {
		if item.EntityId != "" {
			list = append(list, EntityRegistryEntry{
				EntityId: item.EntityId,
				DeviceId: item.DeviceId,
				Name:     item.Name,
				AreaId:   item.AreaId,
				Platform: item.Platform,
			})
		}
	}

	return list
}

type HomeAssistantWsApi struct {
	config core.HomeAssistantConfig
}

func (b *HomeAssistantWsApi) buildWebSocketUrl() (string, error) {
	baseURL, err := url.Parse(b.config.BaseURL)
	if err != nil || (baseURL != nil && (!baseURL.IsAbs() || (baseURL.Scheme != "http" && baseURL.Scheme != "https"))) {
		return "", fmt.Errorf("invalid Home Assistant BaseUrl: %s", b.config.BaseURL)
	}

	var scheme = "ws"
	if baseURL.Scheme == "https" {
		scheme = "wss"
	}

	u := *baseURL
	u.Scheme = scheme
	u.Path = "/api/websocket"
	u.RawQuery = ""

	return u.String(), nil
}

func (b *HomeAssistantWsApi) call(ctx context.Context, callType string) (json.RawMessage, error) {
	url, err := b.buildWebSocketUrl()
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var first HAMessage
	if err := conn.ReadJSON(&first); err != nil {
		return nil, err
	}
	if first.Type != "auth_required" {
		return nil, fmt.Errorf("expected auth_required, got %s", first.Type)
	}

	var token = core.SecretResolverInstance.Resolve(b.config.TokenRef)
	authReq := map[string]string{
		"type":         "auth",
		"access_token": token,
	}
	if err := conn.WriteJSON(authReq); err != nil {
		return nil, err
	}

	var authReply HAMessage
	if err := conn.ReadJSON(&authReply); err != nil {
		return nil, err
	}
	if authReply.Type != "auth_ok" {
		return nil, fmt.Errorf("auth failed: %s %s", authReply.Type, authReply.Message)
	}

	var requestId = 1
	subReq := map[string]any{
		"id":   requestId,
		"type": callType,
	}
	if err := conn.WriteJSON(subReq); err != nil {
		return nil, err
	}

	for {
		var msg HAMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return nil, err
		}

		if msg.Type != "result" || msg.ID != requestId {
			continue
		}

		if !msg.Success {
			errstr := msg.Error
			if errstr == "" {
				errstr = "(unknown error)"
			}
			return nil, fmt.Errorf("Home Assistant WS call '%s' failed: %s", callType, errstr)
		}

		if msg.Result == nil {
			return nil, errors.New("Home Assistant WS: missing result.")
		}

		return msg.Result, nil
	}
}

func (b *HomeAssistantWsApi) ListAreas(ctx context.Context) []AreaEntry {
	result, err := b.call(ctx, "config/area_registry/list")
	if err != nil {
		return []AreaEntry{}
	}
	return parseAreas(result)
}

func (b *HomeAssistantWsApi) ListDevices(ctx context.Context) []DeviceEntry {
	result, err := b.call(ctx, "config/device_registry/list")
	if err != nil {
		return []DeviceEntry{}
	}
	return parseDevices(result)
}

func (b *HomeAssistantWsApi) ListEntityRegistry(ctx context.Context) []EntityRegistryEntry {
	result, err := b.call(ctx, "config/entity_registry/list")
	if err != nil {
		return []EntityRegistryEntry{}
	}
	return parseEntityRegistry(result)
}
