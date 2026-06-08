package core

import "time"

// HarnessExecutionModes
const (
	HarnessExecutionModesNormal            = "normal"
	HarnessExecutionModesSupervised        = "supervised"
	HarnessExecutionModesPlanExecuteVerify = "plan-execute-verify"
)

// PlanExecuteVerifyContractTriggers
const (
	PlanExecuteVerifyContractTriggersHighRiskTools      = "high_risk_tools"
	PlanExecuteVerifyContractTriggersWriteTools         = "write_tools"
	PlanExecuteVerifyContractTriggersShell              = "shell"
	PlanExecuteVerifyContractTriggersBrowser            = "browser"
	PlanExecuteVerifyContractTriggersExternalApi        = "external_api"
	PlanExecuteVerifyContractTriggersMultiToolWorkflows = "multi_tool_workflows"
)

// PlanExecuteVerifyStatus
const (
	PlanExecuteVerifyStatusNotStarted       = "not_started"
	PlanExecuteVerifyStatusContractCreated  = "contract_created"
	PlanExecuteVerifyStatusAwaitingApproval = "awaiting_approval"
	PlanExecuteVerifyStatusExecuting        = "executing"
	PlanExecuteVerifyStatusVerifying        = "verifying"
	PlanExecuteVerifyStatusVerified         = "verified"
	PlanExecuteVerifyStatusFailed           = "failed"
	PlanExecuteVerifyStatusRejected         = "rejected"
	PlanExecuteVerifyStatusEscalated        = "escalated"
	PlanExecuteVerifyStatusRolledBack       = "rolled_back"
	PlanExecuteVerifyStatusCancelled        = "cancelled"
)

// PlanExecuteVerifyDecisionKinds
const (
	PlanExecuteVerifyDecisionKindsProceed         = "proceed"
	PlanExecuteVerifyDecisionKindsRequireApproval = "require_approval"
	PlanExecuteVerifyDecisionKindsReject          = "reject"
	PlanExecuteVerifyDecisionKindsEscalate        = "escalate"
	PlanExecuteVerifyDecisionKindsRevisePlan      = "revise_plan"
	PlanExecuteVerifyDecisionKindsRollback        = "rollback"
)

// HarnessVerificationStatus
const (
	HarnessVerificationStatusPassed  = "passed"
	HarnessVerificationStatusFailed  = "failed"
	HarnessVerificationStatusWarning = "warning"
	HarnessVerificationStatusSkipped = "skipped"
	HarnessVerificationStatusUnknown = "unknown"
)

type PlanExecuteVerifyRun struct {
	Id                string                     `json:"id"`
	Status            string                     `json:"status"`
	Decision          string                     `json:"decision"`
	HarnessContractId *string                    `json:"harness_contract_id,omitempty"`
	EvidenceBundleId  *string                    `json:"evidence_bundle_id,omitempty"`
	SourceSessionId   *string                    `json:"source_session_id,omitempty"`
	ActorId           *string                    `json:"actor_id,omitempty"`
	ChannelId         *string                    `json:"channel_id,omitempty"`
	SenderId          *string                    `json:"sender_id,omitempty"`
	Goal              string                     `json:"goal"`
	ToolName          *string                    `json:"tool_name,omitempty"`
	RiskLevel         string                     `json:"risk_level"`
	ApprovalRequired  bool                       `json:"approval_required"`
	Approved          bool                       `json:"approved"`
	StartedAtUtc      time.Time                  `json:"started_at_utc"`
	UpdatedAtUtc      time.Time                  `json:"updated_at_utc"`
	CompletedAtUtc    *time.Time                 `json:"completed_at_utc,omitempty"`
	Verification      *HarnessVerificationResult `json:"verification,omitempty"`
	Warnings          []string                   `json:"warnings"`
	Recommendations   []string                   `json:"recommendations"`
}

func DefaultPlanExecuteVerifyRun() PlanExecuteVerifyRun {
	now := time.Now().UTC()
	return PlanExecuteVerifyRun{
		Status:          PlanExecuteVerifyStatusNotStarted,
		Decision:        PlanExecuteVerifyDecisionKindsProceed,
		RiskLevel:       HarnessContractRiskLevelsLow,
		StartedAtUtc:    now,
		UpdatedAtUtc:    now,
		Warnings:        []string{},
		Recommendations: []string{},
	}
}

type PlanExecuteVerifyDecision struct {
	Decision                  string                `json:"decision"`
	RequiresPlanExecuteVerify bool                  `json:"requires_plan_execute_verify"`
	RequiresApproval          bool                  `json:"requires_approval"`
	RiskLevel                 string                `json:"risk_level"`
	Summary                   string                `json:"summary"`
	Run                       *PlanExecuteVerifyRun `json:"run,omitempty"`
}

func DefaultPlanExecuteVerifyDecision() PlanExecuteVerifyDecision {
	return PlanExecuteVerifyDecision{
		Decision:  PlanExecuteVerifyDecisionKindsProceed,
		RiskLevel: HarnessContractRiskLevelsLow,
	}
}

type HarnessVerificationResult struct {
	Status          string                     `json:"status"`
	Summary         string                     `json:"summary"`
	Checks          []HarnessVerificationCheck `json:"checks"`
	Risks           []string                   `json:"risks"`
	UntestedAreas   []string                   `json:"untested_areas"`
	Recommendations []string                   `json:"recommendations"`
	StartedAtUtc    time.Time                  `json:"started_at_utc"`
	CompletedAtUtc  *time.Time                 `json:"completed_at_utc,omitempty"`
}

func DefaultHarnessVerificationResult() HarnessVerificationResult {
	return HarnessVerificationResult{
		Status:          HarnessVerificationStatusUnknown,
		Checks:          []HarnessVerificationCheck{},
		Risks:           []string{},
		UntestedAreas:   []string{},
		Recommendations: []string{},
		StartedAtUtc:    time.Now().UTC(),
	}
}

type HarnessVerificationCheck struct {
	Id       string  `json:"id"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Required bool    `json:"required"`
	Summary  string  `json:"summary"`
	Details  *string `json:"details,omitempty"`
}

func DefaultHarnessVerificationCheck() HarnessVerificationCheck {
	return HarnessVerificationCheck{
		Status:   HarnessVerificationStatusUnknown,
		Required: true,
	}
}

type PlanExecuteVerifyRunListResponse struct {
	Items []PlanExecuteVerifyRun `json:"items"`
}

func DefaultPlanExecuteVerifyRunListResponse() PlanExecuteVerifyRunListResponse {
	return PlanExecuteVerifyRunListResponse{
		Items: []PlanExecuteVerifyRun{},
	}
}

type PlanExecuteVerifyRunDetailResponse struct {
	Run *PlanExecuteVerifyRun `json:"run,omitempty"`
}

type PlanExecuteVerifyRunMutationResponse struct {
	Success bool                  `json:"success"`
	Run     *PlanExecuteVerifyRun `json:"run,omitempty"`
	Message string                `json:"message"`
	Error   *string               `json:"error,omitempty"`
}
