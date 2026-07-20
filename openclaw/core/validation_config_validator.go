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

func (c *ConfigValidator) validateApertureProviderConfig(path, endpointPropertyName, provider, endpoint, apiKey, authMode string, errorMsg *[]string) {
	if provider != "aperture" {
		return
	}

	if IsBlank(endpoint) {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s.%s must be set when Provider='aperture'.", path, endpointPropertyName))
	} else {
		baseURL, err := url.Parse(endpoint)
		if err != nil || (baseURL != nil && (!baseURL.IsAbs() || (baseURL.Scheme != "http" && baseURL.Scheme != "https"))) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.%s must be an absolute http(s) URL when Provider='aperture'.", path, endpointPropertyName))
		}
	}

	if !c.isTailnetIdentityAuth(authMode) && IsBlank(apiKey) {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s.ApiKey must be set when Provider='aperture' and AuthMode is not 'tailnet-identity'.", path))
	}
}

func (c *ConfigValidator) resolveConfiguredPath(path string) string {
	return ConfigPathResolverInstance.Resolve(path)
}

func (c *ConfigValidator) validateDynamicTurnRoutingTier(tierName string, target *DynamicTurnRoutingTierTarget, profileIds map[string]struct{}, errorMsg *[]string) {
	if target == nil || IsBlank(target.ModelProfileId) || profileIds == nil {
		return
	}

	if _, ok := profileIds[target.ModelProfileId]; ok {
		return
	}
	*errorMsg = append(*errorMsg, fmt.Sprintf("DynamicTurnRouting.%s.ModelProfileId '%s' does not exist in Models.Profiles.", tierName, target.ModelProfileId))
}

func (c *ConfigValidator) builtInLlmProvidersContains(provider string) bool {
	_, ok := BuiltInLlmProviders[provider]
	return ok
}

func (c *ConfigValidator) validateModelProfiles(config *GatewayConfig, errorMsg *[]string, pluginBackedProvidersPossible bool) {
	hasExplicitProfiles := len(config.Models.Profiles) > 0
	profileIds := make(map[string]struct{})

	for _, profile := range config.Models.Profiles {
		if strings.TrimSpace(profile.Id) == "" {
			*errorMsg = append(*errorMsg, "Models.Profiles[].Id must be set.")
			continue
		}

		// 检查重复 ID（不区分大小写）
		idLower := strings.ToLower(profile.Id)
		if _, exists := profileIds[idLower]; exists {
			*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles contains duplicate id '%s'.", profile.Id))
		} else {
			profileIds[idLower] = struct{}{}
		}

		if strings.TrimSpace(profile.Provider) == "" {
			*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.Provider must be set.", profile.Id))
		} else if !pluginBackedProvidersPossible && !c.builtInLlmProvidersContains(profile.Provider) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.Provider '%s' is not a supported built-in provider.", profile.Id, profile.Provider))
		}

		if strings.TrimSpace(profile.Model) == "" {
			*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.Model must be set.", profile.Id))
		}

		if strings.TrimSpace(profile.AuthMode) != "" && !c.isValidProviderAuthMode(profile.AuthMode) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.AuthMode must be 'bearer' or 'tailnet-identity'.", profile.Id))
		} else if c.isTailnetIdentityAuth(profile.AuthMode) && !c.supportsTailnetIdentity(profile.Provider) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.AuthMode 'tailnet-identity' is not supported for provider '%s'.", profile.Id, profile.Provider))
		}

		c.validateApertureProviderConfig(
			fmt.Sprintf("Models.Profiles.%s", profile.Id),
			"BaseUrl",
			profile.Provider,
			profile.BaseUrl,
			profile.ApiKey,
			profile.AuthMode,
			errorMsg,
		)

		if strings.TrimSpace(profile.PresetId) != "" {
			preset, exists := TryGetLocalModelPackage(profile.PresetId)
			if !exists {
				*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.PresetId '%s' is not a known local model preset.", profile.Id, profile.PresetId))
			} else if !strings.EqualFold(profile.Provider, preset.Provider) {
				*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.PresetId '%s' requires Provider='%s'.", profile.Id, profile.PresetId, preset.Provider))
			}
		}

		if profile.Capabilities != nil {
			if profile.Capabilities.MaxContextTokens < 0 {
				*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.Capabilities.MaxContextTokens must be >= 0.", profile.Id))
			}
			if profile.Capabilities.MaxOutputTokens < 0 {
				*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.Capabilities.MaxOutputTokens must be >= 0.", profile.Id))
			}
		}

		isDynamicProvider := pluginBackedProvidersPossible && !c.builtInLlmProvidersContains(profile.Provider)
		c.validatePromptCaching(
			fmt.Sprintf("Models.Profiles.%s.PromptCaching", profile.Id),
			profile.Provider,
			profile.PromptCaching,
			errorMsg,
			isDynamicProvider,
		)
	}

	if !hasExplicitProfiles {
		profileIds["default"] = struct{}{}
	}

	if strings.TrimSpace(config.Models.DefaultProfile) != "" {
		if _, exists := profileIds[strings.ToLower(config.Models.DefaultProfile)]; !exists {
			*errorMsg = append(*errorMsg, fmt.Sprintf("Models.DefaultProfile '%s' does not exist in Models.Profiles.", config.Models.DefaultProfile))
		}
	}

	if config.DynamicTurnRouting.Enabled {
		policy := config.DynamicTurnRouting.Policy
		tierMap := config.DynamicTurnRouting.Policy.Tiers

		if policy.MarginUpgradeThreshold < 0.0 || policy.MarginUpgradeThreshold > 1.0 {
			*errorMsg = append(*errorMsg, "DynamicTurnRouting.Policy.MarginUpgradeThreshold must be between 0 and 1.")
		}

		if policy.R1RescueThreshold < 0.0 || policy.R1RescueThreshold > 1.0 {
			*errorMsg = append(*errorMsg, "DynamicTurnRouting.Policy.R1RescueThreshold must be between 0 and 1.")
		}

		if policy.UnderRoutingSafetyThreshold < 0.0 || policy.UnderRoutingSafetyThreshold > 1.0 {
			*errorMsg = append(*errorMsg, "DynamicTurnRouting.Policy.UnderRoutingSafetyThreshold must be between 0 and 1.")
		}

		if policy.DeepConversationTurnIndexThreshold < 0 {
			*errorMsg = append(*errorMsg, "DynamicTurnRouting.Policy.DeepConversationTurnIndexThreshold must be >= 0.")
		}

		classifierPath := config.DynamicTurnRouting.Assets.ClassifierModelPath
		embeddingPath := config.DynamicTurnRouting.Assets.EmbeddingModelPath
		tokenizerPath := config.DynamicTurnRouting.Assets.TokenizerPath

		usesBundlePath := strings.TrimSpace(config.DynamicTurnRouting.BundlePath) != ""
		if !usesBundlePath {
			if strings.TrimSpace(classifierPath) != "" && strings.TrimSpace(embeddingPath) == "" {
				*errorMsg = append(*errorMsg, "DynamicTurnRouting requires an embedding model when classifier routing is enabled.")
			}

			if strings.TrimSpace(embeddingPath) != "" && strings.TrimSpace(tokenizerPath) == "" {
				*errorMsg = append(*errorMsg, "DynamicTurnRouting requires a tokenizer path when embeddings are configured.")
			}
		}

		c.validateDynamicTurnRoutingTier("Policy.Tiers.T0", tierMap.T0, profileIds, errorMsg)
		c.validateDynamicTurnRoutingTier("Policy.Tiers.T1", tierMap.T1, profileIds, errorMsg)
		c.validateDynamicTurnRoutingTier("Policy.Tiers.T2", tierMap.T2, profileIds, errorMsg)
		c.validateDynamicTurnRoutingTier("Policy.Tiers.T3", tierMap.T3, profileIds, errorMsg)
	}

	for _, profile := range config.Models.Profiles {
		for _, fallbackId := range profile.FallbackProfileIds {
			if strings.TrimSpace(fallbackId) == "" {
				continue
			}
			if _, exists := profileIds[strings.ToLower(fallbackId)]; !exists {
				*errorMsg = append(*errorMsg, fmt.Sprintf("Models.Profiles.%s.FallbackProfileIds contains unknown profile '%s'.", profile.Id, fallbackId))
			}
		}
	}

	for routeId, route := range config.Routing.Routes {
		if strings.TrimSpace(route.ModelProfileId) != "" {
			if _, exists := profileIds[strings.ToLower(route.ModelProfileId)]; !exists {
				*errorMsg = append(*errorMsg, fmt.Sprintf("Routing.Routes.%s.ModelProfileId '%s' does not exist in Models.Profiles.", routeId, route.ModelProfileId))
			}
		}

		for _, fallbackId := range route.FallbackModelProfileIds {
			if strings.TrimSpace(fallbackId) == "" {
				continue
			}
			if _, exists := profileIds[strings.ToLower(fallbackId)]; !exists {
				*errorMsg = append(*errorMsg, fmt.Sprintf("Routing.Routes.%s.FallbackModelProfileIds contains unknown profile '%s'.", routeId, fallbackId))
			}
		}
	}
}
