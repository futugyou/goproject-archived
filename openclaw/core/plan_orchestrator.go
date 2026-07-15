package core

import "context"

type PlanExecuteVerifyDecisionKinds string
type HarnessContractRiskLevels string
type PlanExecuteVerifyToolContext struct {
	Session                  Session                  `json:"session"`
	CorrelationID            string                   `json:"correlation_id"`
	CallID                   string                   `json:"call_id,omitempty"`
	ToolName                 string                   `json:"tool_name"`
	ArgumentsJSON            string                   `json:"arguments_json"`
	ActionDescriptor         ToolActionDescriptor     `json:"action_descriptor"`
	GovernanceDescriptor     ToolGovernanceDescriptor `json:"governance_descriptor"`
	ExistingApprovalRequired bool                     `json:"existing_approval_required"`
	IsStreaming              bool                     `json:"is_streaming"`
	ToolCallCount            int                      `json:"tool_call_count"`
}

func NewPlanExecuteVerifyToolContext() *PlanExecuteVerifyToolContext {
	return &PlanExecuteVerifyToolContext{
		ToolCallCount: 1,
	}
}

type NoopPlanExecuteVerifyOrchestrator struct{}

var NoopPlanExecuteVerifyOrchestratorInstance = &NoopPlanExecuteVerifyOrchestrator{}

var _ IPlanExecuteVerifyOrchestrator = (*NoopPlanExecuteVerifyOrchestrator)(nil)

func (n *NoopPlanExecuteVerifyOrchestrator) EvaluateTool(ctx context.Context, toolCtx *PlanExecuteVerifyToolContext) (*PlanExecuteVerifyDecision, error) {
	return &PlanExecuteVerifyDecision{
		Decision:                  PlanExecuteVerifyDecisionKindsProceed,
		RequiresPlanExecuteVerify: false,
		RequiresApproval:          false,
		RiskLevel:                 HarnessContractRiskLevelsLow,
		Summary:                   "Plan-Execute-Verify mode is disabled.",
	}, nil
}

func (n *NoopPlanExecuteVerifyOrchestrator) RecordApprovalDecision(ctx context.Context, run *PlanExecuteVerifyRun, approved bool) error {
	return nil
}

func (n *NoopPlanExecuteVerifyOrchestrator) CompleteTool(ctx context.Context, run *PlanExecuteVerifyRun, invocation ToolInvocation) (*PlanExecuteVerifyRun, error) {
	return run, nil
}

func (n *NoopPlanExecuteVerifyOrchestrator) VerifyRun(ctx context.Context, runID string) (*PlanExecuteVerifyRun, error) {
	return nil, nil
}

func (n *NoopPlanExecuteVerifyOrchestrator) GetRun(id string) *PlanExecuteVerifyRun {
	return nil
}

func (n *NoopPlanExecuteVerifyOrchestrator) ListRuns(limit int) []PlanExecuteVerifyRun {
	return []PlanExecuteVerifyRun{}
}
