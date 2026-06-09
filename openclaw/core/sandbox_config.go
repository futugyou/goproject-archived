package core

import "strings"

const (
	SandboxProviderNames_None        = "None"
	SandboxProviderNames_OpenSandbox = "OpenSandbox"
)

func SandboxProviderNamesNormalize(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return SandboxProviderNames_None
	}
	return trimmed
}

type SandboxConfig struct {
	Provider   string                        `json:"provider"`
	Endpoint   *string                       `json:"endpoint,omitempty"`
	ApiKey     *string                       `json:"api_key,omitempty"`
	DefaultTTL int                           `json:"default_ttl"`
	Tools      map[string]*SandboxToolConfig `json:"tools"`
}

func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		Provider:   SandboxProviderNames_None,
		DefaultTTL: 300,
		Tools:      make(map[string]*SandboxToolConfig),
	}
}

type SandboxToolConfig struct {
	Mode     *string `json:"mode,omitempty"`
	Template *string `json:"template,omitempty"`
	TTL      *int    `json:"ttl,omitempty"`
}

type ToolSandboxModeResolution struct {
	Provider       string           `json:"provider"`
	ModeSource     string           `json:"mode_source"`
	DefaultMode    ToolSandboxMode  `json:"default_mode"`
	ConfiguredMode *ToolSandboxMode `json:"configured_mode,omitempty"`
	EffectiveMode  ToolSandboxMode  `json:"effective_mode"`
	Reason         string           `json:"reason"`
}

type BuiltInCandidate struct {
	ToolName    string          `json:"tool_name"`
	DefaultMode ToolSandboxMode `json:"default_mode"`
}

func IsOpenSandboxProviderConfigured(config *GatewayConfig) bool {
	if config == nil {
		return false
	}
	return strings.EqualFold(
		SandboxProviderNamesNormalize(config.Sandbox.Provider),
		SandboxProviderNames_OpenSandbox,
	)
}

func TryParseMode(value string) (ToolSandboxMode, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ToolSandboxMode_None, false
	}

	if strings.EqualFold(trimmed, string(ToolSandboxMode_None)) {
		return ToolSandboxMode_None, true
	}
	if strings.EqualFold(trimmed, string(ToolSandboxMode_Prefer)) {
		return ToolSandboxMode_Prefer, true
	}
	if strings.EqualFold(trimmed, string(ToolSandboxMode_Require)) {
		return ToolSandboxMode_Require, true
	}

	return ToolSandboxMode_None, false
}

func ResolveMode(config *GatewayConfig, toolName string, defaultMode ToolSandboxMode) ToolSandboxMode {
	return ResolveModeDetailed(config, toolName, defaultMode).EffectiveMode
}

func ResolveModeDetailed(config *GatewayConfig, toolName string, defaultMode ToolSandboxMode) *ToolSandboxModeResolution {
	provider := SandboxProviderNamesNormalize(config.Sandbox.Provider)
	configuredMode := ToolSandboxMode_None
	hasConfiguredMode := false

	var toolConfig *SandboxToolConfig
	if config.Sandbox.Tools != nil {
		toolConfig, hasConfiguredMode = config.Sandbox.Tools[toolName]
	}

	if hasConfiguredMode && toolConfig != nil && toolConfig.Mode != nil {
		hasConfiguredMode = true
		configuredMode, _ = TryParseMode(*toolConfig.Mode)
	} else {
		hasConfiguredMode = false
	}

	selectedMode := defaultMode
	if hasConfiguredMode {
		selectedMode = configuredMode
	}

	modeSource := "tool-default"
	if hasConfiguredMode {
		modeSource = "tool-config"
	}

	var pConfiguredMode *ToolSandboxMode
	if hasConfiguredMode {
		pConfiguredMode = &configuredMode
	}

	if strings.EqualFold(provider, SandboxProviderNames_None) {
		return &ToolSandboxModeResolution{
			Provider:       provider,
			ModeSource:     modeSource,
			DefaultMode:    defaultMode,
			ConfiguredMode: pConfiguredMode,
			EffectiveMode:  ToolSandboxMode_None,
			Reason:         "sandbox provider is None, the global sandbox off switch",
		}
	}

	reason := "using tool default sandbox mode"
	if hasConfiguredMode {
		reason = "tool-specific sandbox mode configured"
	}

	return &ToolSandboxModeResolution{
		Provider:       provider,
		ModeSource:     modeSource,
		DefaultMode:    defaultMode,
		ConfiguredMode: pConfiguredMode,
		EffectiveMode:  selectedMode,
		Reason:         reason,
	}
}

func ResolveTemplate(config *GatewayConfig, toolName string) *string {
	if config.Sandbox.Tools == nil {
		return nil
	}
	if toolConfig, ok := config.Sandbox.Tools[toolName]; ok {
		return toolConfig.Template
	}
	return nil
}

func ResolveTimeToLiveSeconds(config *GatewayConfig, toolName string, requestedTimeToLiveSeconds *int) int {
	if requestedTimeToLiveSeconds != nil && *requestedTimeToLiveSeconds > 0 {
		return *requestedTimeToLiveSeconds
	}

	if config.Sandbox.Tools != nil {
		if toolConfig, ok := config.Sandbox.Tools[toolName]; ok && toolConfig.TTL != nil && *toolConfig.TTL > 0 {
			return *toolConfig.TTL
		}
	}

	return config.Sandbox.DefaultTTL
}

func IsRequireSandboxed(config *GatewayConfig, toolName string, defaultMode ToolSandboxMode) bool {
	return IsOpenSandboxProviderConfigured(config) &&
		ResolveMode(config, toolName, defaultMode) == ToolSandboxMode_Require
}

func EnumerateBuiltInCandidates(config *GatewayConfig) []BuiltInCandidate {
	var candidates []BuiltInCandidate

	if !config.Tooling.ReadOnlyMode && config.Tooling.AllowShell {
		candidates = append(candidates, BuiltInCandidate{ToolName: "process", DefaultMode: ToolSandboxMode_Prefer})
	}

	if !config.Tooling.ReadOnlyMode && config.Tooling.AllowShell {
		candidates = append(candidates, BuiltInCandidate{ToolName: "shell", DefaultMode: ToolSandboxMode_Prefer})
	}

	if !config.Tooling.ReadOnlyMode && config.Plugins.Native.CodeExec.Enabled {
		candidates = append(candidates, BuiltInCandidate{ToolName: "code_exec", DefaultMode: ToolSandboxMode_Prefer})
	}

	if config.Tooling.EnableBrowserTool {
		candidates = append(candidates, BuiltInCandidate{ToolName: "browser", DefaultMode: ToolSandboxMode_Prefer})
	}

	return candidates
}
