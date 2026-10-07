package primitives

import (
	"context"
	"net/url"

	"github.com/futugyou/openclaw/core"
)

type UrlSafetyDefaultsScenario struct {
	BaseScenario
}

func NewUrlSafetyDefaultsScenario() *UrlSafetyDefaultsScenario {
	s := &UrlSafetyDefaultsScenario{
		BaseScenario: NewBaseScenario("security.url_safety_defaults", "URL safety defaults", HarnessRegressionCategorySecurity, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *UrlSafetyDefaultsScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	defaults := core.UrlSafetyConfig{}
	blocked := []string{
		"http://localhost/",
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://169.254.169.254/",
	}

	allowedFailures := []string{}
	for _, b := range blocked {
		u, err := url.Parse(b)
		if err != nil {
			continue
		}
		result := core.UrlSafetyInstance.ValidateHttpUrl(ctx, *u, &defaults)
		if result.Allowed {
			allowedFailures = append(allowedFailures, b)
		}
	}

	if len(allowedFailures) > 0 {
		return s.Failed("Default URL safety did not block private target(s).", HarnessRegressionScenarioTextJoin(allowedFailures), ""), nil
	}

	if regCtx.Config != nil {
		if !regCtx.Config.Tooling.UrlSafety.Enabled || !regCtx.Config.Tooling.UrlSafety.BlockPrivateNetworkTargets {
			return s.Warning("Default URL safety blocks private targets, but the loaded config weakens URL safety.", "Set Tooling.UrlSafety.Enabled=true and Tooling.UrlSafety.BlockPrivateNetworkTargets=true."), nil
		}
	}
	return s.Passed("Default URL safety blocks loopback, private, and metadata targets.", ""), nil
}
