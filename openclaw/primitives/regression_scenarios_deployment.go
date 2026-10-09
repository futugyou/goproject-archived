package primitives

import (
	"context"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type TailscaleServeProfileNonPublicScenario struct {
	BaseScenario
}

func NewTailscaleServeProfileNonPublicScenario() *TailscaleServeProfileNonPublicScenario {
	s := &TailscaleServeProfileNonPublicScenario{
		BaseScenario: NewBaseScenario("deployment.tailscale_serve_profile_non_public",
			"Tailscale Serve profile non-public bind", HarnessRegressionCategoryDeployment, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *TailscaleServeProfileNonPublicScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	synthetic := &core.GatewayConfig{}
	synthetic.Deployment.Mode = "tailscale-serve"

	if !core.TailscaleServeAdvisorInstance.IsTailscaleServeConfigured(synthetic) ||
		!util.IsLoopbackBind(synthetic.BindAddress) {
		return s.Failed("Tailscale Serve profile default is no longer loopback-only.", "", ""), nil
	}

	if regCtx.Config != nil &&
		core.TailscaleServeAdvisorInstance.IsTailscaleServeConfigured(regCtx.Config) &&
		!util.IsLoopbackBind(regCtx.Config.BindAddress) {
		return s.Failed("Loaded Tailscale Serve config is public-bound.", "Tailscale Serve should proxy to a loopback OpenClaw gateway by default.", ""), nil
	}

	return s.Passed("Tailscale Serve profile remains loopback-bound by default.", ""), nil
}
