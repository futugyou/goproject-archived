package core

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"
)

type ProviderSmokeProbeResult struct {
	Status  string
	Summary string
	Detail  string
}

type ProbeFunc func(ctx context.Context, config LlmProviderConfig) (*ProviderSmokeProbeResult, error)

type ProviderSmokeRegistration struct {
	ProviderID        string
	Probe             ProbeFunc
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

func (r *ProviderSmokeRegistry) RegisterHandler(providerID string, probe ProbeFunc, treatAsConfigured bool) {
	normalized := r.normalize(providerID)
	if normalized == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.registrations[normalized] = ProviderSmokeRegistration{
		ProviderID:        normalized,
		Probe:             probe,
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

func (r *ProviderSmokeRegistry) TryGet(providerID string) (*ProviderSmokeRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, exists := r.registrations[r.normalize(providerID)]
	return &reg, exists
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

const DefaultOllamaBaseUrl = "http://127.0.0.1:11434"

type OllamaResult struct {
	BaseUrl                   string
	UsesCompatibilityEndpoint bool
}

func OllamaNormalizeBaseUrl(endpoint string) string {
	return OllamaNormalize(endpoint).BaseUrl
}

func OllamaUsesCompatibilityEndpoint(endpoint string) bool {
	return OllamaNormalize(endpoint).UsesCompatibilityEndpoint
}

func OllamaNormalize(endpoint string) OllamaResult {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return OllamaResult{BaseUrl: DefaultOllamaBaseUrl, UsesCompatibilityEndpoint: false}
	}

	trimmed = strings.TrimRight(trimmed, "/")

	parsedUrl, err := url.Parse(trimmed)
	if err != nil || parsedUrl.Scheme == "" || parsedUrl.Host == "" {
		return OllamaResult{BaseUrl: trimmed, UsesCompatibilityEndpoint: false}
	}

	path := strings.TrimRight(parsedUrl.Path, "/")

	// 检查是否以 /v1 结尾（不区分大小写）
	if strings.EqualFold(path, "/v1") {
		parsedUrl.Path = ""
		parsedUrl.RawQuery = ""

		baseUrl := strings.TrimRight(parsedUrl.String(), "/")
		return OllamaResult{
			BaseUrl:                   baseUrl,
			UsesCompatibilityEndpoint: true,
		}
	}

	return OllamaResult{BaseUrl: trimmed, UsesCompatibilityEndpoint: false}
}
