package primitives

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
)

type EvidenceBundleSerializationScenario struct {
	BaseScenario
}

func NewEvidenceBundleSerializationScenario() *EvidenceBundleSerializationScenario {
	s := &EvidenceBundleSerializationScenario{
		BaseScenario: NewBaseScenario("harness.evidence_bundle_serialization", "Evidence Bundle serialization", HarnessRegressionCategoryHarness, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *EvidenceBundleSerializationScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	bundle := core.EvidenceBundle{
		Id:                "evb_regression",
		Title:             "Harness regression evidence",
		Summary:           "Synthetic evidence bundle for regression serialization.",
		Confidence:        core.ConfidenceHigh,
		HarnessContractId: "hctr_regression",
		Checks: []core.EvidenceCheck{
			{
				Id:      "check_round_trip",
				Name:    "Round-trip",
				Status:  core.CheckStatusPassed,
				Summary: "Model round-tripped.",
			},
		},
	}
	jsondata, err := json.Marshal(bundle)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	var restored core.EvidenceBundle
	if err := json.Unmarshal(jsondata, &restored); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	if restored.Id == bundle.Id &&
		len(restored.Checks) == 1 &&
		restored.HarnessContractId == bundle.HarnessContractId {
		return s.Passed("Evidence Bundle model round-tripped through source-generated JSON.", ""), nil
	}

	return s.Failed("Evidence Bundle model did not round-trip correctly.", "", ""), nil
}
