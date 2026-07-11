package core

import (
	"context"
	"sort"
	"strings"
	"sync"
)

type ProviderSmokeProbeResult struct {
	Status  string
	Summary string
	Detail  string
}

type ProbeFunc func(ctx context.Context, config LlmProviderConfig) (ProviderSmokeProbeResult, error)

type ProviderSmokeRegistration struct {
	ProviderID        string
	ProbeAsync        ProbeFunc
	TreatAsConfigured bool
	SkipReason        string
}

type ProviderSmokeRegistry struct {
	mu            sync.RWMutex
	registrations map[string]ProviderSmokeRegistration
}

func NewProviderSmokeRegistry() *ProviderSmokeRegistry {
	return &ProviderSmokeRegistry{
		registrations: make(map[string]ProviderSmokeRegistration),
	}
}

func (r *ProviderSmokeRegistry) RegisterHandler(providerID string, probeAsync ProbeFunc, treatAsConfigured bool) {
	normalized := r.normalize(providerID)
	if normalized == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.registrations[normalized] = ProviderSmokeRegistration{
		ProviderID:        normalized,
		ProbeAsync:        probeAsync,
		TreatAsConfigured: treatAsConfigured,
	}
}

func (r *ProviderSmokeRegistry) RegisterMetadata(providerID string, treatAsConfigured bool, skipReason string) {
	normalized := r.normalize(providerID)
	if normalized == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.registrations[normalized] = ProviderSmokeRegistration{
		ProviderID:        normalized,
		TreatAsConfigured: treatAsConfigured,
		SkipReason:        skipReason,
	}
}

func (r *ProviderSmokeRegistry) TryGet(providerID string) (ProviderSmokeRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, exists := r.registrations[r.normalize(providerID)]
	return reg, exists
}

// 返回按 ProviderID 字母顺序排序后的切片副本
func (r *ProviderSmokeRegistry) Snapshot() []ProviderSmokeRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ProviderSmokeRegistration, 0, len(r.registrations))
	for _, reg := range r.registrations {
		result = append(result, reg)
	}

	// 按照 ProviderID 不区分大小写排序（由于存储时已经过存储标准化，直接比较即可）
	sort.Slice(result, func(i, j int) bool {
		return result[i].ProviderID < result[j].ProviderID
	})

	return result
}

// 内部标准化方法
func (r *ProviderSmokeRegistry) normalize(providerID string) string {
	return strings.ToLower(strings.TrimSpace(providerID))
}
