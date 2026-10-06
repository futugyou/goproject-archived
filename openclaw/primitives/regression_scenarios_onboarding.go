package primitives

import (
	"context"
	"fmt"
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
