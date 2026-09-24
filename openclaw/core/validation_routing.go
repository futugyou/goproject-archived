package core

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func DecisionRoutingConfigurationProvider(cfg *DecisionRoutingConfig) string {
	if cfg.ProviderName != "" {
		return strings.ToLower(cfg.ProviderName)
	}
	// 兜底逻辑：若未显式指定 ProviderName，可根据默认 Endpoint 或特有字段判断
	if strings.Contains(cfg.Endpoint, "127.0.0.1") || strings.HasPrefix(cfg.Model, "laya@") {
		return "laya"
	}
	return "jev"
}

func DecisionRoutingConfigurationNormalizeMode(cfg *DecisionRoutingConfig) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case "disabled", "shadow", "active":
		return mode, nil
	default:
		return "", fmt.Errorf("decision routing mode must be disabled, shadow, or active")
	}
}

func DecisionRoutingConfigurationValidate(cfg *DecisionRoutingConfig) (*url.URL, error) {
	mode, err := DecisionRoutingConfigurationNormalizeMode(cfg)
	if err != nil {
		return nil, err
	}

	provider := DecisionRoutingConfigurationProvider(cfg)
	switch provider {
	case "jev":
		return validateJev(cfg, mode)
	case "laya":
		return validateLaya(cfg, mode)
	}

	return nil, fmt.Errorf("unsupported decision provider: %s", provider)
}

func validateLaya(cfg *DecisionRoutingConfig, mode string) (*url.URL, error) {
	if err := DecisionRoutingConfigurationValidateLimits(cfg); err != nil {
		return nil, err
	}

	// 1. Endpoint 校验（必须为本地 Loopback IP、HTTP/HTTPS、无 userinfo/query/fragment）
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, fmt.Errorf("Laya.Endpoint must use a literal loopback IP, without credentials, query, or fragment")
	}

	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, fmt.Errorf("Laya.Endpoint must use a literal loopback IP, without credentials, query, or fragment")
	}

	host := endpoint.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("Laya.Endpoint must use a literal loopback IP, without credentials, query, or fragment")
	}

	// 2. Model 校验 (以 "laya@" 开头，后跟 40 位小写十六进制字符串)
	if !strings.HasPrefix(cfg.Model, "laya@") || !DecisionRoutingConfigurationIsHex(cfg.Model[5:], 40) {
		return nil, fmt.Errorf("Laya.Model must be laya@ followed by the pinned 40-character Hugging Face revision")
	}

	// 3. CalibrationId 校验
	if (mode == "active" || cfg.CalibrationId != "") && !DecisionRoutingConfigurationIsHex(cfg.CalibrationId, 64) {
		return nil, fmt.Errorf("Laya active routing requires the SHA-256 CalibrationId of an evaluated calibration artifact")
	}

	// 4. Language 校验
	if len(cfg.Language) > 35 {
		return nil, fmt.Errorf("Laya.Language must be empty or a language tag such as en or de-DE")
	}
	for _, r := range cfg.Language {
		if !isAsciiLetterOrDigit(r) && r != '-' {
			return nil, fmt.Errorf("Laya.Language must be empty or a language tag such as en or de-DE")
		}
	}

	// 5. InputUsdPerMillionTokens 校验
	if cfg.InputUsdPerMillionTokens != 0 {
		return nil, fmt.Errorf("Laya has no metered API token price; account for local compute separately")
	}

	return endpoint, nil
}

// validateJev Jev 的独立校验逻辑占位（对应原 C# JevRoutingConfiguration.Validate）
func validateJev(cfg *DecisionRoutingConfig, mode string) (*url.URL, error) {
	if err := DecisionRoutingConfigurationValidateLimits(cfg); err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || cfg.Endpoint == "" {
		return nil, fmt.Errorf("invalid Jev endpoint")
	}
	return endpoint, nil
}

// ValidateLimits 校验通用限制及阈值
func DecisionRoutingConfigurationValidateLimits(cfg *DecisionRoutingConfig) error {
	if cfg.TimeoutMs < 1 || cfg.TimeoutMs > 30000 ||
		cfg.MaxStateChars < 256 || cfg.MaxStateChars > 32000 ||
		cfg.HistoryMessages < 0 || cfg.HistoryMessages > 20 ||
		cfg.MaxConcurrentRequests < 1 || cfg.MaxConcurrentRequests > 128 ||
		cfg.CircuitFailureThreshold < 1 || cfg.CircuitFailureThreshold > 100 ||
		cfg.CircuitBreakSeconds < 1 || cfg.CircuitBreakSeconds > 3600 {
		return fmt.Errorf("decision request, state, concurrency, or circuit limits are outside their supported ranges")
	}

	if !isProbability(cfg.MinConfidence) ||
		!isProbability(cfg.DowngradeMinConfidence) ||
		cfg.DowngradeMinConfidence < cfg.MinConfidence ||
		!isProbability(cfg.MinProbabilityMargin) ||
		!isProbability(cfg.HighRiskThreshold) ||
		cfg.InputUsdPerMillionTokens < 0 || cfg.InputUsdPerMillionTokens > 1000000 {
		return fmt.Errorf("decision thresholds must be finite probabilities, downgrade confidence cannot be lower than minimum confidence, and price cannot be negative")
	}

	return nil
}

// IsHex 校验字符串是否完全由指定长度的小写十六进制字符（ASCII）组成
func DecisionRoutingConfigurationIsHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func isProbability(value float64) bool {
	return value >= 0 && value <= 1
}

func isAsciiLetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
