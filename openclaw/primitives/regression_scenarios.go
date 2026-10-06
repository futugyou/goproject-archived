package primitives

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type QuickstartConfigLoadsScenario struct {
	BaseScenario
}

func NewQuickstartConfigLoadsScenario() *QuickstartConfigLoadsScenario {
	s := &QuickstartConfigLoadsScenario{
		BaseScenario: NewBaseScenario("onboarding.quickstart_config", "Quickstart config loads", HarnessRegressionCategoryOnboarding, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *QuickstartConfigLoadsScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	if regCtx.Config != nil {
		return s.Passed("Config loaded successfully.", fmt.Sprintf("Config path: %s", regCtx.ConfigPath)), nil
	}

	if !regCtx.ConfigPathExplicit && !regCtx.ConfigExists {
		return s.Skipped("No default config was found; run setup or pass --config to check a specific config.", fmt.Sprintf("Expected config path: %s", regCtx.ConfigPath)), nil
	}

	return s.Failed("Config could not be loaded.", fmt.Sprintf("Config path: %s", regCtx.ConfigPath), regCtx.ConfigLoadError), nil
}

type ProviderConfigShapeScenario struct {
	BaseScenario
}

func NewProviderConfigShapeScenario() *ProviderConfigShapeScenario {
	s := &ProviderConfigShapeScenario{
		BaseScenario: NewBaseScenario("providers.config_shape", "Provider config shape", HarnessRegressionCategoryProviders, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *ProviderConfigShapeScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		return HarnessRegressionScenarioResult{}, err
	}

	if regCtx.Config == nil {
		return s.Skipped("Provider shape was skipped because no config is loaded.", ""), nil
	}

	config := regCtx.Config
	var failures []string
	var notes []string
	provider := normalize(config.Llm.Provider)
	model := normalize(config.Llm.Model)

	if strings.TrimSpace(provider) == "" {
		failures = append(failures, "OpenClaw:Llm:Provider must be set.")
	}
	if strings.TrimSpace(model) == "" {
		failures = append(failures, "OpenClaw:Llm:Model must be set.")
	}
	if !isValidAuthMode(config.Llm.AuthMode) {
		failures = append(failures, "OpenClaw:Llm:AuthMode must be bearer or tailnet-identity.")
	}
	if requiresEndpoint(provider) && !isAbsoluteHTTPURL(config.Llm.Endpoint) {
		failures = append(failures, fmt.Sprintf("Provider '%s' requires an absolute http(s) endpoint.", provider))
	}

	for _, profile := range config.Models.Profiles {
		if strings.TrimSpace(profile.Id) == "" {
			failures = append(failures, "Models.Profiles[].Id must be set.")
		}
		if strings.TrimSpace(profile.Provider) == "" {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s.Provider must be set.", profile.Id))
		}
		if strings.TrimSpace(profile.Model) == "" {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s.Model must be set.", profile.Id))
		}
		if strings.TrimSpace(profile.AuthMode) != "" && !isValidAuthMode(profile.AuthMode) {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s.AuthMode must be bearer or tailnet-identity.", profile.Id))
		}

		baseURL := profile.BaseUrl
		if baseURL == "" {
			baseURL = config.Llm.Endpoint
		}
		if requiresEndpoint(normalize(profile.Provider)) && !isAbsoluteHTTPURL(derefString(&baseURL)) {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s requires an absolute http(s) base URL.", profile.Id))
		}
	}

	if regCtx.Offline {
		notes = append(notes, "Credential and network checks were skipped because offline mode is enabled.")
	}

	if len(failures) > 0 {
		return s.Failed("Provider/model shape has blocking issue(s).", HarnessRegressionScenarioTextJoin(failures), ""), nil
	}

	details := HarnessRegressionScenarioTextJoin(notes)

	return s.Passed("Provider/model shape is valid without external network calls.", details), nil
}

func isValidAuthMode(value string) bool {
	v := strings.TrimSpace(value)
	return v == "" ||
		strings.EqualFold(v, "bearer") ||
		strings.EqualFold(v, "tailnet-identity")
}

func requiresEndpoint(provider string) bool {
	switch provider {
	case "openai-compatible", "aperture", "groq", "together", "lmstudio", "anthropic-vertex", "amazon-bedrock", "azure-openai":
		return true
	default:
		return false
	}
}

func isAbsoluteHTTPURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || !u.IsAbs() {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type PublicBindHardeningScenario struct {
	BaseScenario
}

func NewPublicBindHardeningScenario() *PublicBindHardeningScenario {
	s := &PublicBindHardeningScenario{
		BaseScenario: NewBaseScenario("security.public_bind_hardening", "Public bind hardening", HarnessRegressionCategorySecurity, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *PublicBindHardeningScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	if regCtx.Config != nil {
		return s.Skipped("Public bind posture was skipped because no config is loaded.", ""), nil
	}

	var config = regCtx.Config
	if util.IsLoopbackBind(config.BindAddress) {
		return s.Passed("Gateway bind address is loopback-only.", ""), nil
	}

	failures := []string{}
	if config.AuthToken == "" {
		failures = append(failures, "AuthToken must be configured for non-loopback binds.")
	}
	if config.Canvas.Enabled && !config.Canvas.AllowOnPublicBind {
		failures = append(failures, "Canvas command forwarding is enabled on a public bind without Canvas.AllowOnPublicBind.")
	}
	if (config.Plugins.Enabled || config.Plugins.DynamicNative.Enabled || config.Plugins.Mcp.Enabled) &&
		!config.Security.AllowPluginBridgeOnPublicBind {
		failures = append(failures, "Plugin execution is enabled on a public bind without AllowPluginBridgeOnPublicBind.")
	}
	if UnsafeLocalToolingExposed(config) && !config.Security.AllowUnsafeToolingOnPublicBind {
		failures = append(failures, "Unsafe local tooling is exposed on a public bind without explicit opt-in.")
	}
	if !config.Security.RequireRequesterMatchForHttpToolApproval {
		failures = append(failures, "Requester-matched HTTP tool approvals are disabled on a public bind.")
	}

	if len(failures) > 0 {
		return s.Failed("Public/non-loopback bind is missing required hardening.", HarnessRegressionScenarioTextJoin(failures), ""), nil
	}
	return s.Passed("Public bind hardening posture is acceptable.", ""), nil
}

func TryResolveExecutionBackend(config *core.GatewayConfig, toolName string) (backendName string, result bool) {
	if !config.Execution.Enabled {
		return
	}
	if route, ok := config.Execution.Tools[toolName]; ok && route.Backend != "" {
		backendName = route.Backend
		result = true
		return
	}

	if toolName == "process" {
		if route, ok := config.Execution.Tools["shell"]; ok && route.Backend != "" {
			backendName = route.Backend
			result = true
			return
		}
	}

	return
}

func IsLocalExecutionRoute(config *core.GatewayConfig, toolName string) bool {
	backendName, ok := TryResolveExecutionBackend(config, toolName)
	if ok {
		return backendName == "local"
	}

	return config.Execution.DefaultBackend == "local"
}

func IsUnsafeLocalExecutionToolExposed(
	config *core.GatewayConfig,
	toolName string,
	defaultMode core.ToolSandboxMode) bool {
	return config.Tooling.AllowShell &&
		IsLocalExecutionRoute(config, toolName) &&
		!core.IsRequireSandboxed(config, toolName, defaultMode)
}

func UnsafeLocalToolingExposed(config *core.GatewayConfig) bool {
	if IsUnsafeLocalExecutionToolExposed(config, "shell", core.ToolSandboxModePrefer) {
		return true
	}

	if IsUnsafeLocalExecutionToolExposed(config, "process", core.ToolSandboxModePrefer) {
		return true
	}

	if config.Plugins.Native.CodeExec.Enabled &&
		IsLocalExecutionRoute(config, "code_exec") &&
		!core.IsRequireSandboxed(config, "code_exec", core.ToolSandboxModePrefer) {
		return true
	}

	return slices.Contains(config.Tooling.AllowedWriteRoots, "*") || slices.Contains(config.Tooling.AllowedReadRoots, "*")
}
