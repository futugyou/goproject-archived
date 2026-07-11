package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	if config.ApiKey != nil {
		apiKey = SecretResolverInstance.Resolve(*config.ApiKey)
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

	detailPtr := p.SafeReadBody(response.Body, timeoutCtx)
	var detail string
	if detailPtr != nil {
		detail = *detailPtr
	}
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
		Detail:  *detailPtr,
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
	if config.ApiKey != nil {
		apiKey = SecretResolverInstance.Resolve(*config.ApiKey)
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

func (p *ProviderSmokeProbe) SafeReadBody(body io.Reader, ctx context.Context) *string {
	// 使用带 context 的 LimitedReader 避免读取无限的错误报文
	limitedReader := io.LimitReader(body, 400)
	payloadBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil
	}

	payload := string(payloadBytes)
	if strings.TrimSpace(payload) == "" {
		return nil
	}
	return &payload
}
