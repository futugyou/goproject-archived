package primitives

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
)

type GovernanceLedgerSerializationScenario struct {
	BaseScenario
}

func NewGovernanceLedgerSerializationScenario() *GovernanceLedgerSerializationScenario {
	s := &GovernanceLedgerSerializationScenario{
		BaseScenario: NewBaseScenario("harness.governance_ledger_serialization", "Governance Ledger serialization", HarnessRegressionCategoryHarness, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *GovernanceLedgerSerializationScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	entry := core.GovernanceLedgerEntry{
		Id:                "gov_regression",
		Decision:          core.DecisionApproved,
		Status:            core.StatusActive,
		Source:            core.SourceToolApproval,
		ToolName:          "shell",
		ActionSummary:     "Run a supervised shell command.",
		RiskLevel:         "high",
		Scope:             core.ScopeSession,
		HarnessContractId: "hctr_regression",
		EvidenceBundleId:  "evb_regression",
		PolicyHint: &core.GovernancePolicyHint{
			RequiresReview: true,
			Notes:          "Review before reuse.",
		},
	}
	jsondata, err := json.Marshal(entry)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	var restored core.GovernanceLedgerEntry
	if err := json.Unmarshal(jsondata, &restored); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	if restored.Id == entry.Id &&
		restored.Decision == "approved" &&
		restored.PolicyHint != nil && restored.PolicyHint.RequiresReview {
		return s.Passed("Governance Ledger entry round-tripped through source-generated JSON.", ""), nil
	}

	return s.Failed("Governance Ledger entry did not round-trip correctly.", "", ""), nil
}
