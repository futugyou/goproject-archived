package core

import "time"

const (
	HarnessContractStatusDraft      = "draft"
	HarnessContractStatusProposed   = "proposed"
	HarnessContractStatusApproved   = "approved"
	HarnessContractStatusExecuting  = "executing"
	HarnessContractStatusVerified   = "verified"
	HarnessContractStatusFailed     = "failed"
	HarnessContractStatusRejected   = "rejected"
	HarnessContractStatusRolledBack = "rolled_back"
	HarnessContractStatusCancelled  = "cancelled"
)

const (
	HarnessContractRiskLevelsLow      = "low"
	HarnessContractRiskLevelsMedium   = "medium"
	HarnessContractRiskLevelsHigh     = "high"
	HarnessContractRiskLevelsCritical = "critical"
)

const (
	HarnessContractApprovalRequirementsNone             = "none"
	HarnessContractApprovalRequirementsOptional         = "optional"
	HarnessContractApprovalRequirementsRequired         = "required"
	HarnessContractApprovalRequirementsAlreadySatisfied = "already_satisfied"
)

const (
	HarnessContractResourceKindsFile           = "file"
	HarnessContractResourceKindsDirectory      = "directory"
	HarnessContractResourceKindsMemoryNote     = "memory_note"
	HarnessContractResourceKindsProfile        = "profile"
	HarnessContractResourceKindsAutomation     = "automation"
	HarnessContractResourceKindsSkill          = "skill"
	HarnessContractResourceKindsProviderPolicy = "provider_policy"
	HarnessContractResourceKindsSession        = "session"
	HarnessContractResourceKindsEndpoint       = "endpoint"
	HarnessContractResourceKindsExternalApi    = "external_api"
	HarnessContractResourceKindsDatabase       = "database"
	HarnessContractResourceKindsUnknown        = "unknown"
)

type HarnessContract struct {
	ID                 string                            `json:"id"`
	Status             string                            `json:"status"`
	Goal               string                            `json:"goal"`
	UserRequestSummary *string                           `json:"user_request_summary,omitempty"`
	SourceSessionID    *string                           `json:"source_session_id,omitempty"`
	ActorID            *string                           `json:"actor_id,omitempty"`
	ChannelID          *string                           `json:"channel_id,omitempty"`
	SenderID           *string                           `json:"sender_id,omitempty"`
	CreatedAtUtc       time.Time                         `json:"created_at_utc"`
	UpdatedAtUtc       time.Time                         `json:"updated_at_utc"`
	ApprovedAtUtc      *time.Time                        `json:"approved_at_utc,omitempty"`
	CompletedAtUtc     *time.Time                        `json:"completed_at_utc,omitempty"`
	RiskLevel          *string                           `json:"risk_level,omitempty"`
	ApprovalRequired   string                            `json:"approval_required"`
	ApprovalReason     *string                           `json:"approval_reason,omitempty"`
	PlannedActions     []HarnessContractAction           `json:"planned_actions"`
	ReadSet            []HarnessContractResourceRef      `json:"read_set"`
	WriteSet           []HarnessContractResourceRef      `json:"write_set"`
	ToolsRequired      []HarnessContractToolRequirement  `json:"tools_required"`
	Assumptions        []HarnessContractAssumption       `json:"assumptions"`
	Constraints        []HarnessContractConstraint       `json:"constraints"`
	VerificationPlan   []HarnessContractVerificationStep `json:"verification_plan"`
	RollbackPlan       []HarnessContractRollbackStep     `json:"rollback_plan"`
	SuccessCriteria    []string                          `json:"success_criteria"`
	Tags               []string                          `json:"tags"`
	Metadata           *HarnessContractMetadata          `json:"metadata,omitempty"`
}

func DefaultHarnessContract() HarnessContract {
	now := time.Now().UTC()
	return HarnessContract{
		ID:               "",
		Status:           HarnessContractStatusDraft,
		Goal:             "",
		CreatedAtUtc:     now,
		UpdatedAtUtc:     now,
		ApprovalRequired: HarnessContractApprovalRequirementsNone,
		PlannedActions:   []HarnessContractAction{},
		ReadSet:          []HarnessContractResourceRef{},
		WriteSet:         []HarnessContractResourceRef{},
		ToolsRequired:    []HarnessContractToolRequirement{},
		Assumptions:      []HarnessContractAssumption{},
		Constraints:      []HarnessContractConstraint{},
		VerificationPlan: []HarnessContractVerificationStep{},
		RollbackPlan:     []HarnessContractRollbackStep{},
		SuccessCriteria:  []string{},
		Tags:             []string{},
	}
}

type HarnessContractAction struct {
	ID               string                       `json:"id"`
	Title            string                       `json:"title"`
	Description      *string                      `json:"description,omitempty"`
	ToolName         *string                      `json:"tool_name,omitempty"`
	ActionType       *string                      `json:"action_type,omitempty"`
	RiskLevel        *string                      `json:"risk_level,omitempty"`
	RequiresApproval bool                         `json:"requires_approval"`
	ReadSet          []HarnessContractResourceRef `json:"read_set"`
	WriteSet         []HarnessContractResourceRef `json:"write_set"`
	ExpectedOutcome  *string                      `json:"expected_outcome,omitempty"`
	Status           *string                      `json:"status,omitempty"`
}

func DefaultHarnessContractAction() HarnessContractAction {
	return HarnessContractAction{
		Title:    "",
		ReadSet:  []HarnessContractResourceRef{},
		WriteSet: []HarnessContractResourceRef{},
	}
}

type HarnessContractToolRequirement struct {
	ToolName         string  `json:"tool_name"`
	Purpose          *string `json:"purpose,omitempty"`
	RequiresApproval bool    `json:"requires_approval"`
	ApprovalScope    *string `json:"approval_scope,omitempty"`
}

type HarnessContractResourceRef struct {
	Kind        string  `json:"kind"`
	Path        *string `json:"path,omitempty"`
	Key         *string `json:"key,omitempty"`
	ID          *string `json:"id,omitempty"`
	Description *string `json:"description,omitempty"`
	Scope       *string `json:"scope,omitempty"`
	IsSensitive bool    `json:"is_sensitive"`
}

func DefaultHarnessContractResourceRef() HarnessContractResourceRef {
	return HarnessContractResourceRef{
		Kind: HarnessContractResourceKindsUnknown,
	}
}

type HarnessContractVerificationStep struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Kind           *string `json:"kind,omitempty"`
	Command        *string `json:"command,omitempty"`
	ToolName       *string `json:"tool_name,omitempty"`
	CheckName      *string `json:"check_name,omitempty"`
	ExpectedSignal *string `json:"expected_signal,omitempty"`
	Required       bool    `json:"required"`
	Status         *string `json:"status,omitempty"`
	ResultSummary  *string `json:"result_summary,omitempty"`
}

func DefaultHarnessContractVerificationStep() HarnessContractVerificationStep {
	return HarnessContractVerificationStep{
		Title:    "",
		Required: true,
	}
}

type HarnessContractRollbackStep struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	ToolName    *string `json:"tool_name,omitempty"`
	Target      *string `json:"target,omitempty"`
	Status      *string `json:"status,omitempty"`
}

func DefaultHarnessContractRollbackStep() HarnessContractRollbackStep {
	return HarnessContractRollbackStep{
		Title: "",
	}
}

type HarnessContractAssumption struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Verified bool   `json:"verified"`
}

func DefaultHarnessContractAssumption() HarnessContractAssumption {
	return HarnessContractAssumption{
		Text: "",
	}
}

type HarnessContractConstraint struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Scope *string `json:"scope,omitempty"`
}

func DefaultHarnessContractConstraint() HarnessContractConstraint {
	return HarnessContractConstraint{
		Text: "",
	}
}

type HarnessContractMetadata struct {
	CreatedBy     *string           `json:"created_by,omitempty"`
	Source        *string           `json:"source,omitempty"`
	CorrelationID *string           `json:"correlation_id,omitempty"`
	Properties    map[string]string `json:"properties"`
}

func DefaultHarnessContractMetadata() HarnessContractMetadata {
	return HarnessContractMetadata{
		Properties: make(map[string]string),
	}
}

type HarnessContractListQuery struct {
	Status          *string    `json:"status,omitempty"`
	RiskLevel       *string    `json:"risk_level,omitempty"`
	SourceSessionID *string    `json:"source_session_id,omitempty"`
	ActorID         *string    `json:"actor_id,omitempty"`
	ChannelID       *string    `json:"channel_id,omitempty"`
	Tag             *string    `json:"tag,omitempty"`
	CreatedFromUtc  *time.Time `json:"created_from_utc,omitempty"`
	CreatedToUtc    *time.Time `json:"created_to_utc,omitempty"`
	Limit           int        `json:"limit"`
}

func DefaultHarnessContractListQuery() HarnessContractListQuery {
	return HarnessContractListQuery{
		Limit: 100,
	}
}

type HarnessContractStatusUpdateRequest struct {
	Status string `json:"status"`
}

type HarnessContractListResponse struct {
	Items []HarnessContract `json:"items"`
}

func DefaultHarnessContractListResponse() HarnessContractListResponse {
	return HarnessContractListResponse{
		Items: []HarnessContract{},
	}
}

type HarnessContractDetailResponse struct {
	Contract *HarnessContract `json:"contract,omitempty"`
}

type HarnessContractMutationResponse struct {
	Success  bool             `json:"success"`
	Contract *HarnessContract `json:"contract,omitempty"`
	Message  string           `json:"message"`
	Error    *string          `json:"error,omitempty"`
}

func DefaultHarnessContractMutationResponse() HarnessContractMutationResponse {
	return HarnessContractMutationResponse{
		Message: "",
	}
}
