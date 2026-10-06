package primitives

import (
	"context"
	"slices"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

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
