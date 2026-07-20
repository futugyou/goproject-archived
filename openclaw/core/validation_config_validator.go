package core

import (
	"fmt"
	"regexp"
	"strings"
)

var BuiltInLlmProviders = map[string]struct{}{
	"openai":            {},
	"anthropic":         {},
	"claude":            {},
	"gemini":            {},
	"google":            {},
	"ollama":            {},
	"azure-openai":      {},
	"openai-compatible": {},
	"aperture":          {},
	"anthropic-vertex":  {},
	"amazon-bedrock":    {},
	"groq":              {},
	"together":          {},
	"lmstudio":          {},
	"embedded":          {},
}

var ConfigValidatorInstance = &ConfigValidator{}

type ConfigValidator struct{}

func (c *ConfigValidator) supportsExplicitCacheTtl(providerId, dialect string) bool {
	var provider = strings.TrimSpace(providerId)
	var normalizedDialect = strings.TrimSpace(dialect)
	if normalizedDialect == "" {
		normalizedDialect = "auto"
	}

	if provider == "anthropic" || provider == "claude" || provider == "anthropic-vertex" {
		return true
	}

	if provider == "amazon-bedrock" {
		return normalizedDialect == "anthropic" || normalizedDialect == "auto"
	}

	if provider == "gemini" || provider == "google" {
		return normalizedDialect == "gemini" || normalizedDialect == "auto"
	}

	return false
}

func (c *ConfigValidator) supportsTailnetIdentity(provider string) bool {
	return provider == "aperture" || provider == "openai-compatible"
}

func (c *ConfigValidator) isTailnetIdentityAuth(authMode string) bool {
	return authMode == "tailnet-identity"
}

func (c *ConfigValidator) isValidProviderAuthMode(authMode string) bool {
	normalized := strings.ToLower(strings.TrimSpace(authMode))
	if normalized == "" {
		normalized = "bearer"
	}
	return normalized == "tailnet-identity" || normalized == "bearer"
}

func (c *ConfigValidator) validatePromptCaching(prefix, providerId string, caching *PromptCachingConfig, errorMsg *[]string, isDynamicProvider bool) {
	if caching == nil || (caching.Enabled != nil && *caching.Enabled != true) {
		return
	}

	var retention = strings.ToLower(strings.TrimSpace(caching.Retention))
	if retention == "" {
		retention = "auto"
	}

	if retention != "none" && retention != "short" && retention != "long" && retention != "auto" {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s.Retention must be one of: none, short, long, auto", prefix))
	}

	var dialect = strings.ToLower(strings.TrimSpace(caching.Dialect))
	if retention == "" {
		retention = "auto"
	}

	if dialect != "auto" && dialect != "openai" && dialect != "anthropic" && dialect != "gemini" && dialect != "none" {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s.Dialect must be one of: auto, openai, anthropic, gemini, none.", prefix))
	}

	var provider = strings.TrimSpace(providerId)
	var requireExplicitDialect = provider == "openai-compatible" || provider == "aperture" || provider == "groq" || provider == "together" || provider == "lmstudio" || isDynamicProvider

	if requireExplicitDialect && dialect == "auto" {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s.Dialect must be explicit for provider '%s'.", prefix, provider))
	}

	if caching.KeepWarmEnabled != nil && *caching.KeepWarmEnabled == true {
		if caching.KeepWarmIntervalMinutes < 5 {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.KeepWarmIntervalMinutes must be >= 5 when keep-warm is enabled.", prefix))
		}

		if !c.supportsExplicitCacheTtl(provider, dialect) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.KeepWarmEnabled is only valid for providers with explicit cache TTL semantics.", prefix))
		}
	}
}

func (c *ConfigValidator) validateRegexPattern(path, pattern string, errorMsg *[]string) {
	_, err := regexp.Compile(pattern)
	if err != nil {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s is not a valid regex: %s", path, err.Error()))
	}
}

func (c *ConfigValidator) validateRegexList(path string, patterns []string, errorMsg *[]string) {
	for i := range patterns {
		if !IsBlank(patterns[i]) {
			c.validateRegexPattern(fmt.Sprintf("%s[%d]", path, i), patterns[i], errorMsg)
		}
	}
}
