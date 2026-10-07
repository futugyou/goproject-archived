package primitives

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
)

type HarnessContractSerializationScenario struct {
	BaseScenario
}

func NewHarnessContractSerializationScenario() *HarnessContractSerializationScenario {
	s := &HarnessContractSerializationScenario{
		BaseScenario: NewBaseScenario("harness.contract_serialization", "Harness Contract serialization", HarnessRegressionCategoryHarness, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *HarnessContractSerializationScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	contract := core.HarnessContract{
		Id:               "hctr_regression",
		Status:           core.HarnessContractStatusProposed,
		Goal:             "Verify harness model serialization.",
		ApprovalRequired: core.HarnessContractApprovalRequirementsRequired,
		PlannedActions: []core.HarnessContractAction{
			{
				Id:         "act_read",
				Title:      "Read docs",
				ToolName:   "read_file",
				ActionType: "read",
			},
		},
		VerificationPlan: []core.HarnessContractVerificationStep{
			{
				Id:    "verify_json",
				Title: "JSON round-trip",
				Kind:  "serialization",
			},
		},
	}

	jsondata, err := json.Marshal(contract)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	var restored core.HarnessContract
	if err := json.Unmarshal(jsondata, &restored); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	if restored.Id == contract.Id &&
		len(restored.PlannedActions) == 1 &&
		len(restored.VerificationPlan) == 1 {
		return s.Passed("Harness Contract model round-tripped through source-generated JSON.", ""), nil
	}

	return s.Failed("Harness Contract model did not round-trip correctly.", "", ""), nil
}
