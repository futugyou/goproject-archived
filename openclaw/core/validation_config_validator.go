package core

import (
	"fmt"
	"net/url"
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

func (c *ConfigValidator) validateExternalCli(config *ExternalCliOptions, errorMsg *[]string) {
	if config == nil {
		return
	}
	if config.DefaultTimeoutSeconds < 1 {
		*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.DefaultTimeoutSeconds must be >= 1 (got %d).", config.DefaultTimeoutSeconds))
	}
	if config.MaxStdoutBytes < 1 {
		*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.MaxStdoutBytes must be >= 1 (got %d).", config.MaxStdoutBytes))
	}
	if config.MaxStderrBytes < 1 {
		*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.MaxStderrBytes must be >= 1 (got %d).", config.MaxStderrBytes))
	}
	if config.AllowFreeformCommands {
		*errorMsg = append(*errorMsg, "ExternalCli.AllowFreeformCommands is not supported by this native connector; use named allowlisted commands.")
	}

	var presetIds = config.Presets
	for i := 0; i < len(presetIds); i++ {
		if IsBlank(presetIds[i]) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Presets[%d] must not be empty.", i))
		}
	}

	for _, presetId := range ExternalCliPresetCatalogInstance.FindUnknownPresetIds(*config) {
		*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Presets contains unknown preset '%s'.", presetId))
	}

	var effectiveConfig = ExternalCliPresetCatalogInstance.Apply(*config)
	for connectorName, connector := range effectiveConfig.Connectors {
		if IsBlank(connectorName) {
			*errorMsg = append(*errorMsg, "ExternalCli.Connectors contains an empty connector name.")
		}
		if connector.Enabled && IsBlank(connector.Executable) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.Executable must be set when connector is enabled.", connectorName))
		}

		var defaultFormat = NormalizeOutputFormat(connector.DefaultOutputFormat)
		if defaultFormat != connector.DefaultOutputFormat {
			*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.DefaultOutputFormat must be one of: json, ndjson, csv, text, table.", connectorName))
		}

		c.validateRegexList(fmt.Sprintf("ExternalCli.Connectors.%s.RedactionRules", connectorName), connector.RedactionRules, errorMsg)

		for commandName, command := range connector.Commands {
			if IsBlank(commandName) {
				*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.Commands contains an empty command name.", connectorName))
			}
			if len(command.ArgsTemplate) == 0 {
				*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.Commands.%s.ArgsTemplate must contain at least one argument.", connectorName, commandName))
			}
			if command.SupportsDryRun && len(command.DryRunArgsTemplate) == 0 {
				*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.Commands.%s.DryRunArgsTemplate must be set when SupportsDryRun=true.", connectorName, commandName))
			}
			if command.TimeoutSeconds != nil && *command.TimeoutSeconds <= 0 {
				*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.Commands.%s.TimeoutSeconds must be >= 1 when set.", connectorName, commandName))
			}

			var risk = NormalizeRiskLevel(command.RiskLevel)
			if risk != command.RiskLevel {
				*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.Commands.%s.RiskLevel must be low, medium, or high.", connectorName, commandName))
			}

			if !IsBlank(command.StructuredOutput) {
				var format = NormalizeOutputFormat(command.StructuredOutput)
				if format != command.StructuredOutput {
					*errorMsg = append(*errorMsg, fmt.Sprintf("ExternalCli.Connectors.%s.Commands.%s.StructuredOutput must be one of: json, ndjson, csv, text, table.", connectorName, commandName))
				}
			}

			c.validateRegexList(fmt.Sprintf("ExternalCli.Connectors.%s.Commands.%s.RedactionRules", connectorName, commandName), command.RedactionRules, errorMsg)
			for parameterName, parameter := range command.Parameters {
				if !IsBlank(parameter.Pattern) {
					c.validateRegexPattern(fmt.Sprintf("ExternalCli.Connectors.%s.Commands.%s.Parameters.%s.Pattern", connectorName, commandName, parameterName), parameter.Pattern, errorMsg)
				}
			}
		}
	}
}

func (c *ConfigValidator) validateWorkflows(config *WorkflowsConfig, errorMsg *[]string) {
	if config == nil || !config.Enabled {
		return
	}

	if len(config.Backends) == 0 {
		*errorMsg = append(*errorMsg, "Workflows is enabled but no backends are configured.")
		return
	}

	for backendId, backend := range config.Backends {
		var path = fmt.Sprintf("Workflows.Backends.%s", backendId)
		if IsBlank(backendId) {
			*errorMsg = append(*errorMsg, "Workflows.Backends contains an empty backend id.")
			path = "Workflows.Backends.<empty>"
		}

		if !backend.Enabled {
			continue
		}

		var kind = strings.TrimSpace(backend.Kind)
		if kind == "" {
			kind = "maf-durable-http"
		}

		if kind != "maf-durable-http" {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.Kind must be '{AgentWorkflowBackendKinds.MafDurableHttp}'.", path))
		}

		baseURL, err := url.Parse(backend.BaseUrl)
		if err != nil || (baseURL != nil && (!baseURL.IsAbs() || (baseURL.Scheme != "http" && baseURL.Scheme != "https"))) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.BaseUrl must be an absolute http(s) URL.", path))
		}

		if backend.PollIntervalSeconds < 1 {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.PollIntervalSeconds must be >= 1 (got %d).", path, backend.PollIntervalSeconds))
		}
		if backend.TimeoutSeconds < 5 {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.TimeoutSeconds must be >= 5 (got %d).", path, backend.TimeoutSeconds))
		}
	}
}
