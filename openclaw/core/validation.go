package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var ProviderSmokeProbeInstance = &ProviderSmokeProbe{}

type ProviderSmokeProbe struct{}

// 执行单次供应商连接可用性探测
func (p *ProviderSmokeProbe) Probe(ctx context.Context, httpClient *http.Client, config LlmProviderConfig, registry *ProviderSmokeRegistry) (*ProviderSmokeProbeResult, error) {
	provider := p.NormalizeProvider(config.Provider)
	if provider == "" {
		return &ProviderSmokeProbeResult{
			Status:  "skip",
			Summary: "Provider smoke skipped because no provider is configured.",
		}, nil
	}

	if reg, found := registry.TryGet(provider); found && reg != nil {
		if reg.Probe != nil {
			return reg.Probe(ctx, config)
		}

		detail := reg.SkipReason
		if strings.TrimSpace(detail) == "" {
			detail = fmt.Sprintf("Provider '%s' does not expose a smoke probe.", provider)
		}

		return &ProviderSmokeProbeResult{
			Status:  "skip",
			Summary: fmt.Sprintf("Provider smoke skipped for '%s'.", provider),
			Detail:  detail,
		}, nil
	}

	apiKey := ""
	if config.ApiKey != "" {
		apiKey = SecretResolverInstance.Resolve(config.ApiKey)
	}

	if !p.HasRequiredCredentials(provider, apiKey, config.AuthMode) {
		detail := "Set the configured env: secret or provide a valid API key reference."
		return &ProviderSmokeProbeResult{
			Status:  "skip",
			Summary: fmt.Sprintf("Provider smoke skipped because credentials for '%s' are not resolved.", provider),
			Detail:  detail,
		}, nil
	}

	req, err := p.BuildRequest(ctx, provider, config, apiKey)
	if err != nil {
		detail := err.Error()
		return &ProviderSmokeProbeResult{
			Status:  "skip",
			Summary: fmt.Sprintf("Provider smoke skipped for '%s'.", provider),
			Detail:  detail,
		}, nil
	}

	// 处理超时边界控制
	timeoutCtx, cancel := context.WithTimeout(ctx, p.GetProbeTimeout(config.TimeoutSeconds))
	defer cancel()
	req = req.WithContext(timeoutCtx)

	response, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() == nil && timeoutCtx.Err() == context.DeadlineExceeded {
			detail := "The gateway can still work if the provider is reachable later."
			return &ProviderSmokeProbeResult{
				Status:  "skip",
				Summary: fmt.Sprintf("Provider smoke skipped for '%s/%s' because the probe timed out.", provider, config.Model),
				Detail:  detail,
			}, nil
		}
		// 其他请求连接失败或不可达异常
		detail := err.Error()
		return &ProviderSmokeProbeResult{
			Status:  "skip",
			Summary: fmt.Sprintf("Provider smoke skipped for '%s/%s' because the upstream endpoint is unreachable.", provider, config.Model),
			Detail:  detail,
		}, nil
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return &ProviderSmokeProbeResult{
			Status:  "pass",
			Summary: fmt.Sprintf("Provider smoke passed for '%s/%s'.", provider, config.Model),
		}, nil
	}

	detail := p.SafeReadBody(response.Body, timeoutCtx)
	summary := fmt.Sprintf("Provider smoke failed for '%s/%s' with HTTP %d.", provider, config.Model, response.StatusCode)

	// 处理需要熔断/跳过的特定临时上游状态码
	if response.StatusCode == http.StatusTooManyRequests ||
		response.StatusCode == http.StatusServiceUnavailable ||
		response.StatusCode == http.StatusBadGateway ||
		response.StatusCode == http.StatusGatewayTimeout {

		fullDetail := summary
		if strings.TrimSpace(detail) != "" {
			fullDetail = fmt.Sprintf("%s %s", summary, detail)
		}

		return &ProviderSmokeProbeResult{
			Status:  "skip",
			Summary: fmt.Sprintf("Provider smoke skipped for '%s/%s' because the upstream is temporarily unavailable.", provider, config.Model),
			Detail:  fullDetail,
		}, nil
	}

	return &ProviderSmokeProbeResult{
		Status:  "fail",
		Summary: summary,
		Detail:  detail,
	}, nil
}

// IsProviderConfigured 检查指定供应商是否已正确配置基础参数
func (p *ProviderSmokeProbe) IsProviderConfigured(config LlmProviderConfig, registry *ProviderSmokeRegistry) bool {
	provider := p.NormalizeProvider(config.Provider)
	if provider == "" || strings.TrimSpace(config.Model) == "" {
		return false
	}

	if reg, found := registry.TryGet(provider); found && reg != nil {
		return reg.TreatAsConfigured
	}

	apiKey := ""
	if config.ApiKey != "" {
		apiKey = SecretResolverInstance.Resolve(config.ApiKey)
	}

	return p.HasRequiredCredentials(provider, apiKey, config.AuthMode)
}

func (p *ProviderSmokeProbe) BuildRequest(ctx context.Context, provider string, config LlmProviderConfig, apiKey string) (*http.Request, error) {
	switch provider {
	case "openai", "openai-compatible", "aperture", "groq", "together", "lmstudio", "azure-openai":
		suppressAuth := p.IsTailnetIdentityAuth(config.AuthMode) && p.SupportsTailnetIdentity(provider)
		return p.BuildOpenAiStyleRequest(ctx, provider, config, apiKey, suppressAuth)
	case "ollama":
		return p.BuildOllamaRequest(ctx, config)
	case "anthropic", "claude", "anthropic-vertex", "amazon-bedrock":
		return p.BuildAnthropicStyleRequest(ctx, provider, config, apiKey)
	case "gemini", "google":
		return p.BuildGeminiRequest(ctx, config, apiKey)
	default:
		return nil, fmt.Errorf("provider '%s' does not have a built-in smoke probe", provider)
	}
}

func (p *ProviderSmokeProbe) BuildOpenAiStyleRequest(ctx context.Context, provider string, config LlmProviderConfig, apiKey string, suppressAuthorization bool) (*http.Request, error) {
	var defaultBase string
	switch provider {
	case "openai":
		defaultBase = "https://api.openai.com/v1"
	case "groq":
		defaultBase = "https://api.groq.com/openai/v1"
	case "together":
		defaultBase = "https://api.together.xyz/v1"
	case "lmstudio":
		defaultBase = "http://127.0.0.1:1234/v1"
	}

	endpoint := p.AppendPath(config.Endpoint, defaultBase, "chat/completions")
	if endpoint == "" {
		return nil, fmt.Errorf("provider '%s' requires OpenClaw:Llm:Endpoint to run a smoke probe", provider)
	}

	payload := map[string]any{
		"model":       config.Model,
		"messages":    []map[string]string{{"role": "user", "content": "Reply with READY."}},
		"temperature": 0,
		"max_tokens":  8,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if !suppressAuthorization && strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}
	return req, nil
}

func (p *ProviderSmokeProbe) BuildOllamaRequest(ctx context.Context, config LlmProviderConfig) (*http.Request, error) {
	baseUrl := strings.TrimRight(OllamaNormalizeBaseUrl(config.Endpoint), "/")
	endpoint := fmt.Sprintf("%s/api/chat", baseUrl)

	payload := map[string]any{
		"model":  config.Model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with READY."},
		},
		"options": map[string]any{
			"temperature": 0,
			"num_predict": 8,
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (p *ProviderSmokeProbe) BuildAnthropicStyleRequest(ctx context.Context, provider string, config LlmProviderConfig, apiKey string) (*http.Request, error) {
	var defaultBase string
	if provider == "anthropic" || provider == "claude" {
		defaultBase = "https://api.anthropic.com/v1"
	}

	endpoint := p.AppendPath(config.Endpoint, defaultBase, "messages")
	if endpoint == "" {
		return nil, fmt.Errorf("provider '%s' requires OpenClaw:Llm:Endpoint to run a smoke probe", provider)
	}

	payload := map[string]any{
		"model":      config.Model,
		"max_tokens": 8,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with READY."},
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	return req, nil
}

func (p *ProviderSmokeProbe) BuildGeminiRequest(ctx context.Context, config LlmProviderConfig, apiKey string) (*http.Request, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("gemini smoke probes require a resolved API key")
	}

	endpoint := p.BuildGeminiEndpoint(config.Endpoint, config.Model, apiKey)

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": "Reply with READY."},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0,
			"maxOutputTokens": 8,
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (p *ProviderSmokeProbe) AppendPath(configuredBase, defaultBase, relativePath string) string {
	baseValue := configuredBase
	if strings.TrimSpace(baseValue) == "" {
		baseValue = defaultBase
	}
	if strings.TrimSpace(baseValue) == "" {
		return ""
	}

	if strings.Contains(strings.ToLower(baseValue), strings.ToLower(relativePath)) {
		return baseValue
	}

	return fmt.Sprintf("%s/%s", strings.TrimRight(baseValue, "/"), relativePath)
}

func (p *ProviderSmokeProbe) BuildGeminiEndpoint(configuredEndpoint, model, apiKey string) string {
	var endpoint string
	if strings.TrimSpace(configuredEndpoint) == "" {
		endpoint = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	} else {
		endpoint = configuredEndpoint
	}

	if strings.Contains(strings.ToLower(endpoint), ":generatecontent") {
		return p.AppendQueryParameter(endpoint, "key", apiKey)
	}

	combined := fmt.Sprintf("%s/models/%s:generateContent", strings.TrimRight(endpoint, "/"), model)
	return p.AppendQueryParameter(combined, "key", apiKey)
}

func (p *ProviderSmokeProbe) AppendQueryParameter(uri, key, value string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

func (p *ProviderSmokeProbe) HasRequiredCredentials(provider, apiKey, authMode string) bool {
	switch provider {
	case "ollama", "lmstudio", "embedded":
		return true
	case "aperture", "openai-compatible":
		if p.IsTailnetIdentityAuth(authMode) {
			return true
		}
	}
	return strings.TrimSpace(apiKey) != ""
}

func (p *ProviderSmokeProbe) NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func (p *ProviderSmokeProbe) IsTailnetIdentityAuth(authMode string) bool {
	return strings.EqualFold(strings.TrimSpace(authMode), "tailnet-identity")
}

func (p *ProviderSmokeProbe) SupportsTailnetIdentity(provider string) bool {
	return provider == "aperture" || provider == "openai-compatible"
}

func (p *ProviderSmokeProbe) GetProbeTimeout(configuredTimeoutSeconds int) time.Duration {
	if configuredTimeoutSeconds <= 0 {
		return 12 * time.Second
	}
	// Clamp 4 到 15 秒
	seconds := configuredTimeoutSeconds
	if seconds < 4 {
		seconds = 4
	} else if seconds > 15 {
		seconds = 15
	}
	return time.Duration(seconds) * time.Second
}

func (p *ProviderSmokeProbe) SafeReadBody(body io.Reader, ctx context.Context) string {
	// 使用带 context 的 LimitedReader 避免读取无限的错误报文
	limitedReader := io.LimitReader(body, 400)
	payloadBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return ""
	}

	payload := string(payloadBytes)
	return strings.TrimSpace(payload)
}

var ModelDoctorEvaluatorInstance = &ModelDoctorEvaluator{}

type ModelDoctorEvaluator struct{}

// 评估当前的网关配置，并可选地结合注册表与历史调用记录生成诊断报告
func (m *ModelDoctorEvaluator) Build(config *GatewayConfig, registry IModelProfileRegistry, recentTurns []TurnTokenUsageRecord) *ModelSelectionDoctorResponse {
	if registry != nil {
		return m.BuildFromRegistry(registry, recentTurns)
	}

	statuses := m.BuildStatusesFromConfig(config)
	warnings := make([]string, 0)
	errors := make([]string, 0)
	defaultProfileID := m.ResolveDefaultProfileIDFromStatuses(config, statuses)

	if len(statuses) == 0 {
		errors = append(errors, "No model profiles are registered.")
	}
	if strings.TrimSpace(defaultProfileID) == "" {
		errors = append(errors, "No default model profile is configured.")
	}

	for _, status := range statuses {
		if len(status.ValidationIssues) > 0 {
			warnings = append(warnings, "Profile '"+status.Id+"' has validation issues: "+strings.Join(status.ValidationIssues, "; "))
		}
		warnings = append(warnings, m.BuildPresetWarnings(&status, config, recentTurns)...)
	}

	return &ModelSelectionDoctorResponse{
		DefaultProfileId: defaultProfileID,
		Errors:           errors,
		Warnings:         warnings,
		Profiles:         statuses,
	}
}

// 基于注册表和历史记录构建诊断报告
func (m *ModelDoctorEvaluator) BuildFromRegistry(
	registry IModelProfileRegistry,
	recentTurns []TurnTokenUsageRecord,
) *ModelSelectionDoctorResponse {
	statuses, err := registry.ListStatuses()
	if err != nil {
		return nil
	}
	warnings := make([]string, 0)
	errors := make([]string, 0)

	if len(statuses) == 0 {
		errors = append(errors, "No model profiles are registered.")
	}
	if strings.TrimSpace(registry.DefaultProfileId()) == "" {
		errors = append(errors, "No default model profile is configured.")
	}

	for _, status := range statuses {
		if len(status.ValidationIssues) > 0 {
			warnings = append(warnings, "Profile '"+status.Id+"' has validation issues: "+strings.Join(status.ValidationIssues, "; "))
		}
		warnings = append(warnings, m.BuildPresetWarnings(&status, nil, recentTurns)...)
	}

	return &ModelSelectionDoctorResponse{
		DefaultProfileId: registry.DefaultProfileId(),
		Errors:           errors,
		Warnings:         warnings,
		Profiles:         statuses,
	}
}

func (m *ModelDoctorEvaluator) BuildStatusesFromConfig(config *GatewayConfig) []ModelProfileStatus {
	if config == nil {
		return []ModelProfileStatus{}
	}

	profiles := []ModelProfileConfig{}
	if len(config.Models.Profiles) > 0 {
		profiles = config.Models.Profiles
	} else {
		p := m.createImplicitProfile(config)
		if p != nil {
			profiles = []ModelProfileConfig{*p}
		}
	}

	defaultProfileID := m.ResolveDefaultProfileIDFromConfigs(config, profiles)
	statuses := make([]ModelProfileStatus, 0, len(profiles))

	for _, profile := range profiles {
		normalizedID := m.normalize(profile.Id)
		if normalizedID == "" {
			normalizedID = "default"
		}

		ProviderId := m.normalize(profile.Provider)
		if ProviderId == "" {
			ProviderId = m.normalize(config.Llm.Provider)
		}
		if ProviderId == "" {
			ProviderId = "unknown"
		}

		modelID := m.normalize(profile.Model)
		if modelID == "" {
			modelID = m.normalize(config.Llm.Model)
		}
		if modelID == "" {
			modelID = "unknown"
		}

		validationIssues := m.validateProfile(config, &profile, ProviderId)

		authMode := m.normalize(profile.AuthMode)
		if authMode == "" {
			authMode = m.normalize(config.Llm.AuthMode)
		}
		if authMode == "" {
			authMode = "bearer"
		}

		sendRequestMetadata := config.Llm.SendRequestMetadata
		if profile.SendRequestMetadata != nil {
			sendRequestMetadata = *profile.SendRequestMetadata
		}

		baseURLSecret := m.resolveSecretValue(profile.BaseUrl)
		if baseURLSecret == "" {
			baseURLSecret = m.resolveSecretValue(config.Llm.Endpoint)
		}

		usesCompatibilityTransport := ProviderId == "ollama" && OllamaNormalize(baseURLSecret).UsesCompatibilityEndpoint

		statuses = append(statuses, ModelProfileStatus{
			Id:                         normalizedID,
			PresetId:                   m.normalize(profile.PresetId),
			ProviderId:                 ProviderId,
			ModelId:                    modelID,
			IsDefault:                  strings.EqualFold(normalizedID, defaultProfileID),
			IsImplicit:                 len(config.Models.Profiles) == 0 && strings.EqualFold(normalizedID, "default"),
			IsAvailable:                len(validationIssues) == 0,
			ProviderGateway:            m.resolveProviderGateway(&profile, ProviderId),
			AuthMode:                   authMode,
			SendRequestMetadata:        sendRequestMetadata,
			Tags:                       m.resolveTags(&profile),
			Capabilities:               m.resolveCapabilities(&profile, ProviderId),
			PromptCaching:              m.mergePromptCaching(config.Llm.PromptCaching, profile.PromptCaching),
			ValidationIssues:           validationIssues,
			FallbackProfileIds:         m.normalizeDistinct(profile.FallbackProfileIds),
			FallbackModels:             m.normalizeDistinct(profile.FallbackModels),
			CompatibilityNotes:         m.resolveCompatibilityNotes(&profile, config),
			UsesCompatibilityTransport: usesCompatibilityTransport,
		})
	}

	// 排序逻辑：优先 IsDefault 降序，其次 ID 升序（不区分大小写）
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].IsDefault && !statuses[j].IsDefault {
			return true
		}
		if !statuses[i].IsDefault && statuses[j].IsDefault {
			return false
		}
		return strings.ToLower(statuses[i].Id) < strings.ToLower(statuses[j].Id)
	})

	return statuses
}

func (m *ModelDoctorEvaluator) ResolveDefaultProfileIDFromConfigs(config *GatewayConfig, profiles []ModelProfileConfig) string {
	configured := ""
	if config != nil {
		configured = m.normalize(config.Models.DefaultProfile)
	}
	if configured != "" {
		return configured
	}

	if len(profiles) == 0 {
		return ""
	}

	id := m.normalize(profiles[0].Id)
	if id == "" {
		return "default"
	}
	return id
}

func (m *ModelDoctorEvaluator) ResolveDefaultProfileIDFromStatuses(config *GatewayConfig, statuses []ModelProfileStatus) string {
	configured := ""
	if config != nil {
		configured = m.normalize(config.Models.DefaultProfile)
	}
	if configured != "" {
		return configured
	}

	for _, item := range statuses {
		if item.IsDefault {
			return item.Id
		}
	}

	if len(statuses) > 0 {
		return statuses[0].Id
	}

	return ""
}

func (m *ModelDoctorEvaluator) createImplicitProfile(config *GatewayConfig) *ModelProfileConfig {
	if config == nil {
		return nil
	}
	return &ModelProfileConfig{
		Id:             "default",
		Provider:       config.Llm.Provider,
		Model:          config.Llm.Model,
		BaseUrl:        config.Llm.Endpoint,
		ApiKey:         config.Llm.ApiKey,
		FallbackModels: config.Llm.FallbackModels,
		Capabilities:   m.guessCapabilities(config.Llm.Provider),
		PromptCaching:  m.clonePromptCaching(config.Llm.PromptCaching),
	}
}

func (m *ModelDoctorEvaluator) validateProfile(config *GatewayConfig, profile *ModelProfileConfig, providerId string) []string {
	var issues []string
	if profile == nil || config == nil {
		return issues
	}

	if strings.TrimSpace(profile.Id) == "" {
		issues = append(issues, "Profile id is required.")
	}
	if strings.TrimSpace(providerId) == "" {
		issues = append(issues, "Provider is required.")
	}
	if strings.TrimSpace(profile.Model) == "" && strings.TrimSpace(config.Llm.Model) == "" {
		issues = append(issues, "Model is required.")
	}

	if m.requiresEndpoint(providerId) &&
		strings.TrimSpace(m.resolveSecretValue(profile.BaseUrl)) == "" &&
		strings.TrimSpace(m.resolveSecretValue(config.Llm.Endpoint)) == "" {
		issues = append(issues, "BaseUrl is required for this provider unless inherited from OpenClaw:Llm:Endpoint.")
	}

	if m.requiresCredentials(providerId, profile, config) &&
		strings.TrimSpace(m.resolveSecretValue(profile.ApiKey)) == "" &&
		strings.TrimSpace(m.resolveSecretValue(config.Llm.ApiKey)) == "" {
		issues = append(issues, "ApiKey is required for this provider unless inherited from OpenClaw:Llm:ApiKey.")
	}

	return issues
}

func (m *ModelDoctorEvaluator) requiresEndpoint(providerId string) bool {
	switch providerId {
	case "openai-compatible", "aperture", "groq", "together", "lmstudio", "anthropic-vertex", "amazon-bedrock", "azure-openai":
		return true
	}
	return false
}

func (m *ModelDoctorEvaluator) requiresCredentials(providerId string, profile *ModelProfileConfig, config *GatewayConfig) bool {
	if providerId == "ollama" || providerId == "lmstudio" || providerId == "embedded" {
		return false
	}

	authMode := ""
	if profile != nil {
		m.normalize(profile.AuthMode)
	}
	if authMode == "" {
		authMode = m.normalize(config.Llm.AuthMode)
	}

	if (providerId == "aperture" || providerId == "openai-compatible") &&
		strings.EqualFold(authMode, "tailnet-identity") {
		return false
	}

	return true
}

func (m *ModelDoctorEvaluator) guessCapabilities(providerId string) *ModelCapabilities {
	provider := m.normalize(providerId)
	if provider == "embedded" {
		return &ModelCapabilities{
			SupportsStreaming:      true,
			SupportsSystemMessages: true,
			MaxContextTokens:       4096,
			MaxOutputTokens:        1024,
		}
	}

	supportsTools := false
	switch provider {
	case "openai", "openai-compatible", "aperture", "azure-openai", "groq", "together", "lmstudio", "anthropic", "claude", "anthropic-vertex", "amazon-bedrock", "gemini", "google":
		supportsTools = true
	}

	supportsVision := false
	switch provider {
	case "openai", "openai-compatible", "aperture", "azure-openai", "gemini", "google", "ollama", "amazon-bedrock":
		supportsVision = true
	}

	supportsPromptCaching := false
	switch provider {
	case "openai", "azure-openai", "anthropic", "claude", "anthropic-vertex", "gemini", "google":
		supportsPromptCaching = true
	}

	supportsExplicitCacheRetention := provider == "anthropic" || provider == "claude" || provider == "anthropic-vertex"

	supportsJSONSchema := false
	supportsStructuredOutputs := false
	supportsParallelToolCalls := false
	supportsReasoningEffort := false
	supportsAudioInput := false

	switch provider {
	case "openai", "openai-compatible", "aperture", "azure-openai":
		supportsJSONSchema = true
		supportsStructuredOutputs = true
		supportsParallelToolCalls = true
		supportsReasoningEffort = true
		supportsAudioInput = true
	}

	reportsCacheWriteTokens := provider == "anthropic" || provider == "claude" || provider == "anthropic-vertex"

	return &ModelCapabilities{
		SupportsTools:                  supportsTools,
		SupportsVision:                 supportsVision,
		SupportsJsonSchema:             supportsJSONSchema,
		SupportsStructuredOutputs:      supportsStructuredOutputs,
		SupportsStreaming:              true,
		SupportsParallelToolCalls:      supportsParallelToolCalls,
		SupportsReasoningEffort:        supportsReasoningEffort,
		SupportsSystemMessages:         true,
		SupportsImageInput:             supportsVision,
		SupportsVideoInput:             supportsVision,
		SupportsAudioInput:             supportsAudioInput,
		SupportsPromptCaching:          supportsPromptCaching,
		SupportsExplicitCacheRetention: supportsExplicitCacheRetention,
		ReportsCacheReadTokens:         supportsPromptCaching,
		ReportsCacheWriteTokens:        reportsCacheWriteTokens,
	}
}

func (m *ModelDoctorEvaluator) mergePromptCaching(root *PromptCachingConfig, profile *PromptCachingConfig) *PromptCachingConfig {
	if root == nil {
		return nil
	}
	res := &PromptCachingConfig{
		Enabled:                 root.Enabled,
		Retention:               root.Retention,
		Dialect:                 root.Dialect,
		KeepWarmEnabled:         root.KeepWarmEnabled,
		KeepWarmIntervalMinutes: root.KeepWarmIntervalMinutes,
		TraceEnabled:            root.TraceEnabled,
		TraceFilePath:           root.TraceFilePath,
	}

	if profile != nil {
		if profile.Enabled != nil {
			res.Enabled = profile.Enabled
		}
		if profile.Retention != "" {
			res.Retention = profile.Retention
		}
		if profile.Dialect != "" {
			res.Dialect = profile.Dialect
		}
		if profile.KeepWarmEnabled != nil {
			res.KeepWarmEnabled = profile.KeepWarmEnabled
		}
		if profile.KeepWarmIntervalMinutes != 0 {
			res.KeepWarmIntervalMinutes = profile.KeepWarmIntervalMinutes
		}
		if profile.TraceEnabled != nil {
			res.TraceEnabled = profile.TraceEnabled
		}
		if profile.TraceFilePath != "" {
			res.TraceFilePath = profile.TraceFilePath
		}
	}
	return res
}

func (m *ModelDoctorEvaluator) clonePromptCaching(caching *PromptCachingConfig) *PromptCachingConfig {
	return m.mergePromptCaching(caching, nil)
}

func (m *ModelDoctorEvaluator) normalizeDistinct(values []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, val := range values {
		norm := m.normalize(val)
		if norm != "" {
			lower := strings.ToLower(norm)
			if !seen[lower] {
				seen[lower] = true
				result = append(result, norm)
			}
		}
	}
	return result
}

func (m *ModelDoctorEvaluator) normalize(value string) string {
	return strings.TrimSpace(value)
}

func (m *ModelDoctorEvaluator) resolveSecretValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	resolved := SecretResolverInstance.Resolve(value)
	if resolved != "" {
		return resolved
	}
	return value
}

func (m *ModelDoctorEvaluator) resolveCapabilities(profile *ModelProfileConfig, ProviderId string) *ModelCapabilities {
	if profile != nil {
		if profile.Capabilities != nil {
			return profile.Capabilities
		}

		preset, ok := TryGetLocalModelPreset(profile.PresetId)
		if ok && preset != nil {
			return preset.Capabilities
		}
	}

	return m.guessCapabilities(ProviderId)
}

func (m *ModelDoctorEvaluator) resolveTags(profile *ModelProfileConfig) []string {
	if profile == nil {
		return []string{}
	}

	configured := m.normalizeDistinct(profile.Tags)
	preset, ok := TryGetLocalModelPreset(profile.PresetId)
	if !ok || preset == nil {
		return configured
	}

	// 拼接并去重
	combined := append(configured, preset.Tags...)
	return m.normalizeDistinct(combined)
}

func (m *ModelDoctorEvaluator) resolveProviderGateway(profile *ModelProfileConfig, providerId string) string {
	if profile == nil {
		return ""
	}
	hasApertureTag := false
	for _, tag := range profile.Tags {
		if strings.EqualFold(tag, "aperture") {
			hasApertureTag = true
			break
		}
	}

	if strings.EqualFold(providerId, "aperture") ||
		hasApertureTag ||
		strings.Contains(strings.ToLower(profile.BaseUrl), "aperture") {
		res := "Aperture"
		return res
	}

	return ""
}

func (m *ModelDoctorEvaluator) resolveCompatibilityNotes(profile *ModelProfileConfig, config *GatewayConfig) []string {
	var notes []string
	if profile == nil || config == nil {
		return notes
	}

	provider := m.normalize(profile.Provider)
	if provider == "" {
		provider = m.normalize(config.Llm.Provider)
	}

	baseURLSecret := m.resolveSecretValue(profile.BaseUrl)
	if baseURLSecret == "" {
		baseURLSecret = m.resolveSecretValue(config.Llm.Endpoint)
	}

	if provider == "ollama" && OllamaNormalize(baseURLSecret).UsesCompatibilityEndpoint {
		notes = append(notes, "Using legacy /v1 compatibility endpoint; migrate to the native Ollama base URL.")
	}

	if provider == "ollama" && strings.TrimSpace(profile.PresetId) == "" {
		notes = append(notes, "No local preset is configured; setup and doctor guidance will be more limited until a PresetId is added.")
	}

	preset, ok := TryGetLocalModelPreset(profile.PresetId)
	if ok && preset != nil {
		notes = append(notes, preset.CompatibilityNotes...)
	}

	return m.normalizeDistinct(notes)
}

func (m *ModelDoctorEvaluator) BuildPresetWarnings(status *ModelProfileStatus, config *GatewayConfig, recentTurns []TurnTokenUsageRecord) []string {
	warnings := make([]string, 0)

	if strings.EqualFold(status.ProviderId, "ollama") && strings.TrimSpace(status.PresetId) == "" {
		warnings = append(warnings, "Profile '"+status.Id+"' is an Ollama profile without a PresetId. Use a local preset so doctor and setup can apply local-model guidance.")
	}

	if strings.EqualFold(status.ProviderId, "embedded") {
		if strings.TrimSpace(status.PresetId) == "" {
			warnings = append(warnings, "Profile '"+status.Id+"' is embedded local but has no PresetId. Use an embedded preset so model install and verification can find the package.")
		}
		if config != nil && !config.LocalInference.Enabled {
			warnings = append(warnings, "Profile '"+status.Id+"' uses the embedded provider but OpenClaw:LocalInference:Enabled is false.")
		}
		if len(status.FallbackProfileIds) == 0 && len(status.FallbackModels) == 0 {
			warnings = append(warnings, "Profile '"+status.Id+"' has no fallback profile configured for tool-heavy or long-context routes.")
		}
	}

	if status.UsesCompatibilityTransport {
		warnings = append(warnings, "Profile '"+status.Id+"' is still using the legacy Ollama /v1 compatibility endpoint.")
	}

	if strings.EqualFold(status.ProviderId, "ollama") &&
		status.Capabilities.SupportsTools &&
		len(status.FallbackProfileIds) == 0 &&
		len(status.FallbackModels) == 0 {
		warnings = append(warnings, "Profile '"+status.Id+"' is local-agentic but has no fallback profile configured for unsupported features or context overflow.")
	}

	preset, ok := TryGetLocalModelPreset(status.PresetId)
	if ok && preset != nil && len(recentTurns) > 0 {
		var matchingTurns []TurnTokenUsageRecord
		for _, turn := range recentTurns {
			if strings.EqualFold(turn.ProviderId, status.ProviderId) &&
				strings.EqualFold(turn.ModelId, status.ModelId) {
				matchingTurns = append(matchingTurns, turn)
			}
		}

		// 按 TimestampUtc 降序排序并取前 20 条
		sort.Slice(matchingTurns, func(i, j int) bool {
			return matchingTurns[i].TimestampUtc.After(matchingTurns[j].TimestampUtc)
		})
		if len(matchingTurns) > 20 {
			matchingTurns = matchingTurns[:20]
		}

		if len(matchingTurns) > 0 {
			inputTokens := make([]int64, len(matchingTurns))
			for i, turn := range matchingTurns {
				inputTokens[i] = turn.InputTokens
			}
			sort.Slice(inputTokens, func(i, j int) bool {
				return inputTokens[i] < inputTokens[j]
			})

			// 算 p95 对应的分位数
			idx := int(math.Floor(float64(len(matchingTurns)-1) * 0.95))
			p95 := inputTokens[idx]
			threshold := int64(float64(preset.RecommendedContextTokens) * 0.85)

			if p95 >= threshold {
				warnings = append(warnings, "Profile '"+status.Id+"' is seeing recent prompt sizes near its effective context headroom (p95 "+
					fmt.Sprint(p95)+" tokens vs recommended "+fmt.Sprint(preset.RecommendedContextTokens)+").")
			}
		}
	}

	if config != nil && strings.EqualFold(status.ProviderId, "ollama") {
		for key, route := range config.Routing.Routes {
			if strings.TrimSpace(route.ModelProfileId) == "" {
				continue
			}

			if !strings.EqualFold(route.ModelProfileId, status.Id) {
				continue
			}

			if route.ModelRequirements.SupportsTools &&
				!status.Capabilities.SupportsTools && len(status.FallbackProfileIds) == 0 {
				warnings = append(warnings, "Route '"+key+"' selects local profile '"+status.Id+"' for tool-required traffic without a compatible fallback profile.")
			}

			if route.ModelRequirements.SupportsJsonSchema &&
				!status.Capabilities.SupportsJsonSchema && len(status.FallbackProfileIds) == 0 {
				warnings = append(warnings, "Route '"+key+"' selects local profile '"+status.Id+"' for structured-output traffic without a compatible fallback profile.")
			}
		}
	}

	return warnings
}
