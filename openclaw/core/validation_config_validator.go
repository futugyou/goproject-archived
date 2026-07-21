package core

import (
	"fmt"
	"net/netip"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
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

func (c *ConfigValidator) validateRootSet(field string, roots []string, errorMsg *[]string) {
	if len(roots) == 0 {
		return
	}
	wildcardCount := 0

	for _, v := range roots {
		if v == "*" {
			wildcardCount++
		}
	}

	if wildcardCount > 0 && len(roots) > wildcardCount {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s cannot mix '*' with explicit paths.", field))
	}

	for _, root := range roots {
		if root == "*" {
			continue
		}

		var resolved = c.resolveConfiguredPath(root)
		if IsBlank(resolved) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s entries must resolve to non-empty absolute paths.", field))
			continue
		}

		if !filepath.IsAbs(resolved) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s entries must be absolute paths (got '%s').", field, root))
		}
	}
}

func (c *ConfigValidator) validateDmPolicy(field, value string, errorMsg *[]string) {
	if IsBlank(value) {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s must be 'open', 'pairing', or 'closed'.", field))
		return
	}

	if value != "open" && value != "pairing" && value != "closed" {
		*errorMsg = append(*errorMsg, fmt.Sprintf("%s must be 'open', 'pairing', or 'closed'.", field))
	}
}

func (c *ConfigValidator) validateNotionConfig(config *NotionConfig, errorMsg *[]string) {
	if config == nil || !config.Enabled {
		return
	}

	if IsBlank(SecretResolverInstance.Resolve(config.ApiKeyRef)) {
		*errorMsg = append(*errorMsg, "Plugins.Native.Notion.ApiKeyRef must resolve to a token when Notion is enabled.")
	}

	baseURL, err := url.Parse(config.BaseUrl)
	if err != nil || (baseURL != nil && !baseURL.IsAbs()) {
		*errorMsg = append(*errorMsg, "Plugins.Native.Notion.BaseUrl must be a valid absolute URL when Notion is enabled.")
	}

	if IsBlank(config.ApiVersion) {
		*errorMsg = append(*errorMsg, "Plugins.Native.Notion.ApiVersion must be set when Notion is enabled.")
	}

	if config.MaxSearchResults < 1 {
		*errorMsg = append(*errorMsg, fmt.Sprintf("Plugins.Native.Notion.MaxSearchResults must be >= 1 (got %d).", config.MaxSearchResults))
	}

	hasAnyTarget := !IsBlank(config.DefaultPageId) ||
		!IsBlank(config.DefaultDatabaseId) ||
		slices.IndexFunc(config.AllowedPageIds, func(s string) bool { return !IsBlank(s) }) != -1 ||
		slices.IndexFunc(config.AllowedDatabaseIds, func(s string) bool { return !IsBlank(s) }) != -1

	if !hasAnyTarget {
		*errorMsg = append(*errorMsg, "Plugins.Native.Notion requires at least one allowed/default page or database id when enabled.")
	}
}

func (c *ConfigValidator) validateUrlSafety(path string, config *UrlSafetyConfig, errorMsg *[]string) {
	if config == nil {
		return
	}

	for _, cidr := range config.BlockedCidrs {
		if IsBlank(cidr) {
			continue
		}

		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.BlockedCidrs entry '%s' must be a valid CIDR block.", path, cidr))
			continue
		}

		addr := prefix.Addr()
		prefixLength := prefix.Bits()

		var maxPrefix = 128
		if addr.Is4() {
			maxPrefix = 32
		}

		if prefixLength < 0 || prefixLength > maxPrefix {
			*errorMsg = append(*errorMsg, fmt.Sprintf("%s.BlockedCidrs entry '%s' has an invalid prefix length.", path, cidr))
		}
	}
}

func (c *ConfigValidator) validateCodingBackends(config *CodingBackendsConfig, errorMsg *[]string) {
	if config == nil {
		return
	}
	backendIds := map[string]struct{}{}

	for _, backend := range config.EnumerateConfiguredBackends {
		if IsBlank(backend.BackendId) {
			*errorMsg = append(*errorMsg, "CodingBackends entries must set BackendId.")
			continue
		}

		if _, ok := backendIds[backend.BackendId]; ok {
			*errorMsg = append(*errorMsg, fmt.Sprintf("CodingBackends backend id '%s' must be unique.", backend.BackendId))
		}
		backendIds[backend.BackendId] = struct{}{}

		if backend.TimeoutSeconds < 1 {
			*errorMsg = append(*errorMsg, fmt.Sprintf("CodingBackends.%s.TimeoutSeconds must be >= 1 (got {backend.TimeoutSeconds}).", backend.BackendId))
		}

		if IsBlank(backend.Provider) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("CodingBackends.%s.Provider must be set.", backend.BackendId))
		}

		if !backend.WriteEnabled && !backend.ReadOnlyByDefault {
			*errorMsg = append(*errorMsg, fmt.Sprintf("CodingBackends.%s must set ReadOnlyByDefault=true when WriteEnabled=false.", backend.BackendId))
		}

		if backend.RequireWorkspace && !IsBlank(backend.DefaultWorkspacePath) && !filepath.IsAbs(backend.DefaultWorkspacePath) {
			*errorMsg = append(*errorMsg, fmt.Sprintf("CodingBackends.%s.DefaultWorkspacePath must be absolute when set.", backend.BackendId))
		}

		var credentialSourceCount = 0
		if !IsBlank(backend.Credentials.SecretRef) {
			credentialSourceCount++
		}
		if !IsBlank(backend.Credentials.TokenFilePath) {
			credentialSourceCount++
		}
		if !IsBlank(backend.Credentials.ConnectedAccountId) {
			credentialSourceCount++
		}

		if credentialSourceCount > 1 {
			*errorMsg = append(*errorMsg, fmt.Sprintf("CodingBackends.%s.Credentials must specify at most one of SecretRef, TokenFilePath, or ConnectedAccountId.", backend.BackendId))
		}
	}
}

func (c *ConfigValidator) validateFractalMemory(config *FractalMemoryConfig, errorMsg *[]string) {
	if config == nil {
		return
	}
	if !slices.Contains([]string{"mcp"}, config.Mode) {
		*errorMsg = append(*errorMsg, "Memory.Fractal.Mode must be 'mcp'.")
	}

	if config.Enabled && IsBlank(config.McpCommand) {
		*errorMsg = append(*errorMsg, "Memory.Fractal.McpCommand must be set when Fractal Memory is enabled.")
	}

	if config.DefaultDepth < 0 || config.DefaultDepth > 3 {
		*errorMsg = append(*errorMsg, fmt.Sprintf("Memory.Fractal.DefaultDepth must be between 0 and 3 (got %d).", config.DefaultDepth))
	}

	if !slices.Contains([]string{"index", "state", "timeline", "decisions", "children"}, config.DefaultView) {
		*errorMsg = append(*errorMsg, "Memory.Fractal.DefaultView must be one of 'index', 'state', 'timeline', 'decisions', or 'children'.")
	}

	if !slices.Contains([]string{"compact", "standard", "verbose"}, config.DefaultExportMode) {
		*errorMsg = append(*errorMsg, "Memory.Fractal.DefaultExportMode must be one of 'compact', 'standard', or 'verbose'.")
	}

	if !slices.Contains([]string{"off", "manual", "pulse", "auto"}, config.AutoContextMode) {
		*errorMsg = append(*errorMsg, "Memory.Fractal.AutoContextMode must be one of 'off', 'manual', 'pulse', or 'auto'.")
	}

	if config.MaxContextChars < 1024 {
		*errorMsg = append(*errorMsg, fmt.Sprintf("Memory.Fractal.MaxContextChars must be >= 1024 (got %d).", config.MaxContextChars))
	}

	if config.MaxContextTokens < 256 {
		*errorMsg = append(*errorMsg, fmt.Sprintf("Memory.Fractal.MaxContextTokens must be >= 256 (got %d).", config.MaxContextTokens))
	}

}

func (c *ConfigValidator) Validate(config *GatewayConfig) []string {
	errorMsg := []string{}
	if config == nil {
		return errorMsg
	}

	// Port
	if config.Port < 1 || config.Port > 65535 {
		errorMsg = append(errorMsg, fmt.Sprintf("Port must be between 1 and 65535 (got %d).", config.Port))
	}

	// LLM
	if IsBlank(config.Llm.Model) {
		errorMsg = append(errorMsg, "Llm.Model must be set.")
	}

	var pluginBackedProvidersPossible = config.Plugins.Enabled || config.Plugins.DynamicNative.Enabled || config.Plugins.Mcp.Enabled
	_, ok := BuiltInLlmProviders[config.Llm.Provider]
	if !pluginBackedProvidersPossible && !ok {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.Provider '%s' is not a supported built-in provider.", config.Llm.Provider))
	}

	if config.Llm.MaxTokens < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.MaxTokens must be >= 1 (got %d).", config.Llm.MaxTokens))
	}

	if config.Llm.Temperature < 0 || config.Llm.Temperature > 2 {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.Temperature must be between 0 and 2 (got %f).", config.Llm.Temperature))
	}

	if config.Llm.TimeoutSeconds < 0 {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.TimeoutSeconds must be >= 0 (got %d).", config.Llm.TimeoutSeconds))
	}

	if config.Llm.RetryCount < 0 {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.RetryCount must be >= 0 (got %d).", config.Llm.RetryCount))
	}

	if config.LocalInference.Port < 0 || config.LocalInference.Port > 65535 {
		errorMsg = append(errorMsg, fmt.Sprintf("LocalInference.Port must be between 0 and 65535 (got %d).", config.LocalInference.Port))
	}

	if config.LocalInference.ContextSize < 0 {
		errorMsg = append(errorMsg, fmt.Sprintf("LocalInference.ContextSize must be >= 0 (got %d).", config.LocalInference.ContextSize))
	}

	if config.LocalInference.StartupTimeoutSeconds < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("LocalInference.StartupTimeoutSeconds must be >= 1 (got %d).", config.LocalInference.StartupTimeoutSeconds))
	}

	if config.LocalInference.ReasoningBudget < -1 {
		errorMsg = append(errorMsg, fmt.Sprintf("LocalInference.ReasoningBudget must be >= -1 (got %d).", config.LocalInference.ReasoningBudget))
	}

	if config.Llm.CircuitBreakerThreshold < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.CircuitBreakerThreshold must be >= 1 (got %d).", config.Llm.CircuitBreakerThreshold))
	}

	if config.Llm.CircuitBreakerCooldownSeconds < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.CircuitBreakerCooldownSeconds must be >= 1 (got %d).", config.Llm.CircuitBreakerCooldownSeconds))
	}

	if !c.isValidProviderAuthMode(config.Llm.AuthMode) {
		errorMsg = append(errorMsg, "Llm.AuthMode must be 'bearer' or 'tailnet-identity'.")
	} else if c.isTailnetIdentityAuth(config.Llm.AuthMode) && !c.supportsTailnetIdentity(config.Llm.Provider) {
		errorMsg = append(errorMsg, fmt.Sprintf("Llm.AuthMode 'tailnet-identity' is not supported for provider '%s'.", config.Llm.Provider))
	}

	c.validateApertureProviderConfig("Llm", "Endpoint", config.Llm.Provider, config.Llm.Endpoint, config.Llm.ApiKey, config.Llm.AuthMode, &errorMsg)
	c.validatePromptCaching("Llm.PromptCaching", config.Llm.Provider, config.Llm.PromptCaching, &errorMsg, false)
	c.validateModelProfiles(config, &errorMsg, pluginBackedProvidersPossible)

	// Memory
	mp := config.Memory.Provider
	if mp != "file" && mp != "sqlite" && mp != "mempalace" {
		errorMsg = append(errorMsg, fmt.Sprintf("Memory.Provider '%s' must be 'file', 'sqlite', or 'mempalace'.", config.Memory.Provider))
	}

	if IsBlank(config.Memory.StoragePath) {
		errorMsg = append(errorMsg, "Memory.StoragePath must be set.")
	}
	if config.Memory.MaxHistoryTurns < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Memory.MaxHistoryTurns must be >= 1 (got %d).", config.Memory.MaxHistoryTurns))
	}
	if config.Memory.EnableCompaction {
		if config.Memory.CompactionThreshold < 4 {
			errorMsg = append(errorMsg, fmt.Sprintf("Memory.CompactionThreshold must be >= 4 (got %d).", config.Memory.CompactionThreshold))
		}
		if config.Memory.CompactionKeepRecent < 2 {
			errorMsg = append(errorMsg, fmt.Sprintf("Memory.CompactionKeepRecent must be >= 2 (got %d).", config.Memory.CompactionKeepRecent))
		}
		if config.Memory.CompactionKeepRecent >= config.Memory.CompactionThreshold {
			errorMsg = append(errorMsg, "Memory.CompactionKeepRecent must be less than CompactionThreshold.")
		}
		if config.Memory.CompactionThreshold <= config.Memory.MaxHistoryTurns {
			errorMsg = append(errorMsg, "Memory.CompactionThreshold must be greater than MaxHistoryTurns when EnableCompaction=true.")
		}
	}

	c.validateFractalMemory(config.Memory.Fractal, &errorMsg)

	if config.Memory.Retention.SweepIntervalMinutes < 5 {
		errorMsg = append(errorMsg, fmt.Sprintf("Memory.Retention.SweepIntervalMinutes must be >= 5 (got %d).", config.Memory.Retention.SweepIntervalMinutes))
	}
	if config.Memory.Retention.SessionTtlDays < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Memory.Retention.SessionTtlDays must be >= 1 (got %d).", config.Memory.Retention.SessionTtlDays))
	}
	if config.Memory.Retention.BranchTtlDays < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Memory.Retention.BranchTtlDays must be >= 1 (got %d).", config.Memory.Retention.BranchTtlDays))
	}
	if config.Memory.Retention.ArchiveRetentionDays < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Memory.Retention.ArchiveRetentionDays must be >= 1 (got %d).", config.Memory.Retention.ArchiveRetentionDays))
	}
	if config.Memory.Retention.MaxItemsPerSweep < 10 {
		errorMsg = append(errorMsg, fmt.Sprintf("Memory.Retention.MaxItemsPerSweep must be >= 10 (got %d).", config.Memory.Retention.MaxItemsPerSweep))
	}

	// Sessions
	if config.MaxConcurrentSessions < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("MaxConcurrentSessions must be >= 1 (got %d).", config.MaxConcurrentSessions))
	}
	if config.SessionTimeoutMinutes < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("SessionTimeoutMinutes must be >= 1 (got %d).", config.SessionTimeoutMinutes))
	}

	// WebSocket
	if config.WebSocket.MaxMessageBytes < 256 {
		errorMsg = append(errorMsg, fmt.Sprintf("WebSocket.MaxMessageBytes must be >= 256 (got %d).", config.WebSocket.MaxMessageBytes))
	}
	if config.WebSocket.MaxConnections < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("WebSocket.MaxConnections must be >= 1 (got %d).", config.WebSocket.MaxConnections))
	}
	if config.WebSocket.MaxConnectionsPerIp < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("WebSocket.MaxConnectionsPerIp must be >= 1 (got %d).", config.WebSocket.MaxConnectionsPerIp))
	}

	// Tooling
	if config.Tooling.ToolTimeoutSeconds < 0 {
		errorMsg = append(errorMsg, fmt.Sprintf("Tooling.ToolTimeoutSeconds must be >= 0 (got %d).", config.Tooling.ToolTimeoutSeconds))
	}

	c.validateExternalCli(&config.ExternalCli, &errorMsg)

	c.validateUrlSafety("Tooling.UrlSafety", &config.Tooling.UrlSafety, &errorMsg)

	if config.Plugins.Native.WebFetch.UrlSafety != nil {
		c.validateUrlSafety("Plugins.Native.WebFetch.UrlSafety", config.Plugins.Native.WebFetch.UrlSafety, &errorMsg)
	}

	if config.Tooling.WorkspaceOnly {
		var resolvedWorkspaceRoot = c.resolveConfiguredPath(config.Tooling.WorkspaceRoot)
		if IsBlank(resolvedWorkspaceRoot) {
			errorMsg = append(errorMsg, "Tooling.WorkspaceRoot must resolve to a non-empty absolute path when WorkspaceOnly=true.")
		} else if !filepath.IsAbs(resolvedWorkspaceRoot) {
			errorMsg = append(errorMsg, "Tooling.WorkspaceRoot must resolve to an absolute path when WorkspaceOnly=true.")
		}
	}

	c.validateRootSet("Tooling.AllowedReadRoots", config.Tooling.AllowedReadRoots, &errorMsg)
	c.validateRootSet("Tooling.AllowedWriteRoots", config.Tooling.AllowedWriteRoots, &errorMsg)

	// Sandbox
	var sandboxProvider = SandboxProviderNamesNormalize(config.Sandbox.Provider)
	if sandboxProvider != SandboxProviderNames_None && sandboxProvider != SandboxProviderNames_OpenSandbox {
		errorMsg = append(errorMsg, "Sandbox.Provider must be 'None' or 'OpenSandbox'.")
	}

	if config.Sandbox.DefaultTTL < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Sandbox.DefaultTTL must be >= 1 (got %d).", config.Sandbox.DefaultTTL))
	}

	if sandboxProvider == SandboxProviderNames_OpenSandbox && IsBlank(config.Sandbox.Endpoint) {
		errorMsg = append(errorMsg, "Sandbox.Endpoint must be set when Sandbox.Provider='OpenSandbox'.")
	}

	for toolName, toolConfig := range config.Sandbox.Tools {
		if !IsBlank(toolConfig.Mode) {
			if _, ok := TryParseMode(toolConfig.Mode); !ok {
				errorMsg = append(errorMsg, fmt.Sprintf("Sandbox.Tools.%s.Mode must be 'None', 'Prefer', or 'Require'.", toolName))
			}
		}

		if toolConfig.TTL != nil && (*toolConfig.TTL <= 0) {
			errorMsg = append(errorMsg, fmt.Sprintf("Sandbox.Tools.%s.TTL must be >= 1 when set (got %d).", toolName, *toolConfig.TTL))
		}

		if sandboxProvider == SandboxProviderNames_OpenSandbox && ResolveMode(config, toolName, SandboxProviderNames_None) != SandboxProviderNames_None && IsBlank(toolConfig.Template) {
			errorMsg = append(errorMsg, fmt.Sprintf("Sandbox.Tools.%s.Template must be set when sandboxing is enabled for that tool.", toolName))
		}
	}

	if sandboxProvider == SandboxProviderNames_OpenSandbox {

		for _, candidate := range EnumerateBuiltInCandidates(config) {
			if ResolveMode(config, candidate.ToolName, candidate.DefaultMode) != SandboxProviderNames_None && IsBlank(ResolveTemplate(config, candidate.ToolName)) {
				errorMsg = append(errorMsg, fmt.Sprintf("Sandbox.Tools.%s.Template must be set because %s defaults to sandbox mode '%s'.", candidate.ToolName, candidate.ToolName, ResolveMode(config, candidate.ToolName, candidate.DefaultMode)))
			}
		}
	}

	c.validateCodingBackends(&config.CodingBackends, &errorMsg)

	// Delegation
	if config.Delegation.Enabled {
		if config.Delegation.MaxDepth < 1 {
			errorMsg = append(errorMsg, fmt.Sprintf("Delegation.MaxDepth must be >= 1 (got %d).", config.Delegation.MaxDepth))
		}
		if len(config.Delegation.Profiles) == 0 {
			errorMsg = append(errorMsg, "Delegation is enabled but no profiles are configured.")
		}
		for name, profile := range config.Delegation.Profiles {
			if IsBlank(profile.Name) {
				errorMsg = append(errorMsg, fmt.Sprintf("Delegation profile '%s' has no Name.", name))
			}
			if profile.MaxIterations < 1 {
				errorMsg = append(errorMsg, fmt.Sprintf("Delegation profile '%s' has MaxIterations < 1.", name))
			}
		}
	}

	c.validateWorkflows(&config.Workflows, &errorMsg)

	// Middleware
	if config.SessionTokenBudget < 0 {
		errorMsg = append(errorMsg, fmt.Sprintf("SessionTokenBudget must be >= 0 (got %d).", config.SessionTokenBudget))
	}
	if config.SessionRateLimitPerMinute < 0 {
		errorMsg = append(errorMsg, fmt.Sprintf("SessionRateLimitPerMinute must be >= 0 (got %d).", config.SessionRateLimitPerMinute))
	}

	// Plugin bridge transport
	var transportMode = config.Plugins.Transport.Mode
	if IsBlank(transportMode) {
		transportMode = "stdio"
	}
	if transportMode != "stdio" && transportMode != "socket" && transportMode != "hybrid" {
		errorMsg = append(errorMsg, "Plugins.Transport.Mode must be 'stdio', 'socket', or 'hybrid'.")
	}

	var runtimeOrchestrator = RuntimeOrchestratorNormalize(config.Runtime.Orchestrator)
	if runtimeOrchestrator != RuntimeOrchestratorNative && runtimeOrchestrator != RuntimeOrchestratorMaf {
		errorMsg = append(errorMsg, "Runtime.Orchestrator must be 'native' or 'maf'.")
	}

	c.validateNotionConfig(&config.Plugins.Native.Notion, &errorMsg)
	// MCP plugin servers
	if config.Plugins.Mcp.Enabled {
		if config.Plugins.Mcp.Servers == nil {
			errorMsg = append(errorMsg, "Plugins.Mcp.Servers must be provided when MCP is enabled.")
		} else {
			for serverId, server := range config.Plugins.Mcp.Servers {
				if !server.Enabled {
					continue
				}
				var transport = server.NormalizeTransport()
				if transport != "stdio" && transport != "http" {
					errorMsg = append(errorMsg, fmt.Sprintf("Plugins.Mcp.Servers.%s.Transport must be 'stdio' or 'http'.", serverId))
					continue
				}

				if server.StartupTimeoutSeconds < 1 {
					errorMsg = append(errorMsg, fmt.Sprintf("Plugins.Mcp.Servers.%s.StartupTimeoutSeconds must be >= 1 (got %d).", serverId, server.StartupTimeoutSeconds))
				}
				if server.RequestTimeoutSeconds < 1 {
					errorMsg = append(errorMsg, fmt.Sprintf("Plugins.Mcp.Servers.%s.RequestTimeoutSeconds must be >= 1 (got %d).", serverId, server.RequestTimeoutSeconds))
				}

				if transport == "stdio" {
					if IsBlank(server.Command) {
						errorMsg = append(errorMsg, fmt.Sprintf("Plugins.Mcp.Servers.%s.Command must be set when Transport='stdio'.", serverId))
					}
				} else {
					baseURL, err := url.Parse(server.URL)
					if err != nil || (baseURL != nil && (!baseURL.IsAbs() || (baseURL.Scheme != "http" && baseURL.Scheme != "https"))) {
						errorMsg = append(errorMsg, fmt.Sprintf("Plugins.Mcp.Servers.%s.Url must be an absolute http(s) URL when Transport='http'.", serverId))
					}

				}
			}
		}
	}

	// Channels
	if config.Channels.Sms.Twilio.MaxInboundChars < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.Sms.Twilio.MaxInboundChars must be >= 1 (got %d).", config.Channels.Sms.Twilio.MaxInboundChars))
	}

	if config.Channels.Sms.Twilio.MaxRequestBytes < 1024 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.Sms.Twilio.MaxRequestBytes must be >= 1024 (got %d).", config.Channels.Sms.Twilio.MaxRequestBytes))
	}

	if config.Channels.Telegram.MaxInboundChars < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.Telegram.MaxInboundChars must be >= 1 (got %d).", config.Channels.Telegram.MaxInboundChars))
	}

	if config.Channels.Telegram.MaxRequestBytes < 1024 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.Telegram.MaxRequestBytes must be >= 1024 (got %d).", config.Channels.Telegram.MaxRequestBytes))
	}

	if config.Channels.WhatsApp.MaxInboundChars < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.WhatsApp.MaxInboundChars must be >= 1 (got %d).", config.Channels.WhatsApp.MaxInboundChars))
	}

	if config.Channels.WhatsApp.MaxRequestBytes < 1024 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.WhatsApp.MaxRequestBytes must be >= 1024 (got %d).", config.Channels.WhatsApp.MaxRequestBytes))
	}

	if !slices.Contains([]string{"official", "bridge", "first_party_worker"}, config.Channels.WhatsApp.Type) {
		errorMsg = append(errorMsg, "Channels.WhatsApp.Type must be 'official', 'bridge', or 'first_party_worker'.")
	}

	if config.Channels.WhatsApp.ValidateSignature {
		var appSecret = SecretResolverInstance.Resolve(config.Channels.WhatsApp.WebhookAppSecretRef)
		if IsBlank(appSecret) {
			appSecret = config.Channels.WhatsApp.WebhookAppSecret
		}

		if IsBlank(appSecret) {
			errorMsg = append(errorMsg, "Channels.WhatsApp.ValidateSignature is true but WebhookAppSecret/WebhookAppSecretRef is not configured.")
		}
	}

	if config.Channels.WhatsApp.Type == "first_party_worker" {
		var worker = config.Channels.WhatsApp.FirstPartyWorker
		if !slices.Contains([]string{"baileys", "baileys_csharp", "whatsmeow", "simulated"}, worker.Driver) {
			errorMsg = append(errorMsg, "Channels.WhatsApp.FirstPartyWorker.Driver must be 'baileys', 'baileys_csharp', 'whatsmeow', or 'simulated'.")
		}
		if len(worker.Accounts) == 0 {
			errorMsg = append(errorMsg, "Channels.WhatsApp.FirstPartyWorker.Accounts must contain at least one account.")
		}

		for _, account := range worker.Accounts {
			if IsBlank(account.AccountId) {
				errorMsg = append(errorMsg, "Channels.WhatsApp.FirstPartyWorker.Accounts[].AccountId must be set.")
			}
			if IsBlank(account.SessionPath) {
				errorMsg = append(errorMsg, fmt.Sprintf("Channels.WhatsApp.FirstPartyWorker account '%s' must set SessionPath.", account.AccountId))
			}
			if account.PairingMode != "qr" && account.PairingMode != "pairing_code" {
				errorMsg = append(errorMsg, fmt.Sprintf("Channels.WhatsApp.FirstPartyWorker account '%s' PairingMode must be 'qr' or 'pairing_code'.", account.AccountId))
			}
			if account.PairingMode == "pairing_code" && IsBlank(account.PhoneNumber) {
				errorMsg = append(errorMsg, fmt.Sprintf("Channels.WhatsApp.FirstPartyWorker account '%s' requires PhoneNumber for pairing_code mode.", account.AccountId))
			}
		}
	}
	if config.Channels.Teams.MaxInboundChars < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.Teams.MaxInboundChars must be >= 1 (got %d).", config.Channels.Teams.MaxInboundChars))
	}
	if config.Channels.Teams.MaxRequestBytes < 1024 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.Teams.MaxRequestBytes must be >= 1024 (got %d).", config.Channels.Teams.MaxRequestBytes))
	}
	if !slices.Contains([]string{"open", "allowlist", "disabled"}, config.Channels.Teams.GroupPolicy) {
		errorMsg = append(errorMsg, "Channels.Teams.GroupPolicy must be 'open', 'allowlist', or 'disabled'.")
	}
	if config.Channels.Teams.ReplyStyle != "thread" && config.Channels.Teams.ReplyStyle != "top-level" {
		errorMsg = append(errorMsg, "Channels.Teams.ReplyStyle must be 'thread' or 'top-level'.")
	}
	if config.Channels.Teams.ChunkMode != "length" && config.Channels.Teams.ChunkMode != "newline" {
		errorMsg = append(errorMsg, "Channels.Teams.ChunkMode must be 'length' or 'newline'.")
	}
	if config.Channels.Teams.TextChunkLimit < 1 {
		errorMsg = append(errorMsg, fmt.Sprintf("Channels.Teams.TextChunkLimit must be >= 1 (got %d).", config.Channels.Teams.TextChunkLimit))
	}
	if config.Channels.Teams.Enabled {
		var teamsAppId = SecretResolverInstance.Resolve(config.Channels.Teams.AppIdRef)
		if IsBlank(teamsAppId) {
			teamsAppId = config.Channels.Teams.AppId
		}
		var teamsAppPassword = SecretResolverInstance.Resolve(config.Channels.Teams.AppPasswordRef)
		if IsBlank(teamsAppPassword) {
			teamsAppPassword = config.Channels.Teams.AppPassword
		}
		var teamsTenantId = SecretResolverInstance.Resolve(config.Channels.Teams.TenantIdRef)
		if IsBlank(teamsTenantId) {
			teamsTenantId = config.Channels.Teams.TenantId
		}
		if IsBlank(teamsAppId) {
			errorMsg = append(errorMsg, "Channels.Teams.AppId/AppIdRef must be configured when Teams is enabled.")
		}
		if IsBlank(teamsAppPassword) {
			errorMsg = append(errorMsg, "Channels.Teams.AppPassword/AppPasswordRef must be configured when Teams is enabled.")
		}
		if IsBlank(teamsTenantId) {
			errorMsg = append(errorMsg, "Channels.Teams.TenantId/TenantIdRef must be configured when Teams is enabled.")
		}
	}
	if config.Channels.AllowlistSemantics != "legacy" && config.Channels.AllowlistSemantics != "strict" {
		errorMsg = append(errorMsg, "Channels.AllowlistSemantics must be 'legacy' or 'strict'.")
	}

	c.validateDmPolicy("Channels.Sms.DmPolicy", config.Channels.Sms.DmPolicy, &errorMsg)
	c.validateDmPolicy("Channels.Telegram.DmPolicy", config.Channels.Telegram.DmPolicy, &errorMsg)
	c.validateDmPolicy("Channels.WhatsApp.DmPolicy", config.Channels.WhatsApp.DmPolicy, &errorMsg)
	c.validateDmPolicy("Channels.Teams.DmPolicy", config.Channels.Teams.DmPolicy, &errorMsg)
	c.validateDmPolicy("Channels.Slack.DmPolicy", config.Channels.Slack.DmPolicy, &errorMsg)
	c.validateDmPolicy("Channels.Discord.DmPolicy", config.Channels.Discord.DmPolicy, &errorMsg)
	c.validateDmPolicy("Channels.Signal.DmPolicy", config.Channels.Signal.DmPolicy, &errorMsg)

	// Cron
	if config.Cron.Enabled {
		for _, job := range config.Cron.Jobs {
			if IsBlank(job.Name) {
				errorMsg = append(errorMsg, "Cron job name must be set.")
			}
			if IsBlank(job.Prompt) {
				errorMsg = append(errorMsg, fmt.Sprintf("Cron job '%s' prompt must be set.", job.Name))
			}
			if !IsValidCronExpression(job.CronExpression) {
				errorMsg = append(errorMsg, fmt.Sprintf("Cron job '%s' has invalid CronExpression '%s'.", job.Name, job.CronExpression))
			}
		}
	}

	// Webhooks
	if config.Webhooks.Enabled {
		for name, endpoint := range config.Webhooks.Endpoints {
			if endpoint.MaxBodyLength < 1 {
				errorMsg = append(errorMsg, fmt.Sprintf("Webhook endpoint '%s' MaxBodyLength must be >= 1 (got {endpoint.MaxBodyLength}).", name))
			}
			if endpoint.MaxRequestBytes < 1024 {
				errorMsg = append(errorMsg, fmt.Sprintf("Webhook endpoint '%s' MaxRequestBytes must be >= 1024 (got {endpoint.MaxRequestBytes}).", name))
			}
			if endpoint.ValidateHmac {
				var secret = SecretResolverInstance.Resolve(endpoint.Secret)
				if IsBlank(secret) {
					errorMsg = append(errorMsg, fmt.Sprintf("Webhook endpoint '%s' has ValidateHmac=true but no Secret is configured.  Set OpenClaw:Webhooks:Endpoints:<name>:Secret.", name))
				}
			}
		}
	}

	return errorMsg
}
