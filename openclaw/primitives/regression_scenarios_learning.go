package primitives

import (
	"context"

	"github.com/futugyou/openclaw/core"
)

type LearningProposalReviewFirstScenario struct {
	BaseScenario
}

func NewLearningProposalReviewFirstScenario() *LearningProposalReviewFirstScenario {
	s := &LearningProposalReviewFirstScenario{
		BaseScenario: NewBaseScenario("harness.learning_proposal_review_first",
			"Learning proposal review-first default", HarnessRegressionCategoryHarness, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *LearningProposalReviewFirstScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	defaults := core.DefaultLearningConfig()
	if !defaults.ReviewRequired {
		return s.Failed("LearningConfig defaults no longer require review.", "", ""), nil
	}

	if regCtx.Config != nil && regCtx.Config.Learning.Enabled && !regCtx.Config.Learning.ReviewRequired {
		return s.Warning("Learning defaults require review, but the loaded config disables learning review.",
			"Set Learning:ReviewRequired=true for review-first harness evolution.", ""), nil
	}

	return s.Passed("Learning proposals default to review-required.", ""), nil
}
