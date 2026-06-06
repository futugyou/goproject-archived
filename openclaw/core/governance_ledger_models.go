package core

import "time"

// --- Const Groups (Enums) ---

// GovernanceDecisions
const (
	DecisionApproved  = "approved"
	DecisionRejected  = "rejected"
	DecisionEscalated = "escalated"
	DecisionExpired   = "expired"
	DecisionRevoked   = "revoked"
	DecisionUnknown   = "unknown"
)

// GovernanceDecisionStatuses
const (
	StatusActive     = "active"
	StatusExpired    = "expired"
	StatusRevoked    = "revoked"
	StatusSuperseded = "superseded"
)

// GovernanceScopes
const (
	ScopeOnce    = "once"
	ScopeSession = "session"
	ScopeActor   = "actor"
	ScopeChannel = "channel"
	ScopeProject = "project"
	ScopeTool    = "tool"
	ScopeGlobal  = "global"
	ScopeUnknown = "unknown"
)

// GovernanceLedgerSources
const (
	SourceManual                = "manual"
	SourceToolApproval          = "tool_approval"
	SourceApprovalTimeout       = "approval_timeout"
	SourceApprovalGrantConsumed = "approval_grant_consumed"
	SourceHarnessContract       = "harness_contract"
	SourceEvidenceReview        = "evidence_review"
	SourceLearningProposal      = "learning_proposal"
	SourceUnknown               = "unknown"
)

// --- Core Models & Query Objects ---

type GovernanceLedgerEntry struct {
	Id                 string                    `json:"id"`
	CreatedAtUtc       time.Time                 `json:"created_at_utc"`
	UpdatedAtUtc       time.Time                 `json:"updated_at_utc"`
	Decision           string                    `json:"decision"`
	Status             string                    `json:"status"`
	Source             string                    `json:"source"`
	ActionType         *string                   `json:"action_type,omitempty"`
	ToolName           *string                   `json:"tool_name,omitempty"`
	ActionSummary      string                    `json:"action_summary"`
	ArgumentSummary    *string                   `json:"argument_summary,omitempty"`
	RedactedArguments  *string                   `json:"redacted_arguments,omitempty"`
	RiskLevel          string                    `json:"risk_level"`
	Scope              string                    `json:"scope"`
	ScopeKey           *string                   `json:"scope_key,omitempty"`
	SessionId          *string                   `json:"session_id,omitempty"`
	HarnessContractId  *string                   `json:"harness_contract_id,omitempty"`
	EvidenceBundleId   *string                   `json:"evidence_bundle_id,omitempty"`
	LearningProposalId *string                   `json:"learning_proposal_id,omitempty"`
	ApprovalId         *string                   `json:"approval_id,omitempty"`
	ActorId            *string                   `json:"actor_id,omitempty"`
	ChannelId          *string                   `json:"channel_id,omitempty"`
	SenderId           *string                   `json:"sender_id,omitempty"`
	DecidedBy          *string                   `json:"decided_by,omitempty"`
	DecisionReason     *string                   `json:"decision_reason,omitempty"`
	ExpiresAtUtc       *time.Time                `json:"expires_at_utc,omitempty"`
	RevokedAtUtc       *time.Time                `json:"revoked_at_utc,omitempty"`
	RevokedBy          *string                   `json:"revoked_by,omitempty"`
	RevocationReason   *string                   `json:"revocation_reason,omitempty"`
	PolicyHint         *GovernancePolicyHint     `json:"policy_hint,omitempty"`
	Tags               []string                  `json:"tags"`
	Metadata           *GovernanceLedgerMetadata `json:"metadata,omitempty"`
}

// DefaultGovernanceLedgerEntry
func DefaultGovernanceLedgerEntry() GovernanceLedgerEntry {
	now := time.Now().UTC()
	return GovernanceLedgerEntry{
		CreatedAtUtc: now,
		UpdatedAtUtc: now,
		Decision:     DecisionUnknown,
		Status:       StatusActive,
		Source:       SourceManual,
		RiskLevel:    RiskLevelUnknown,
		Scope:        ScopeUnknown,
		Tags:         []string{},
	}
}

type GovernancePolicyHint struct {
	SuggestedFutureBehavior *string `json:"suggested_future_behavior,omitempty"`
	SuggestedScope          *string `json:"suggested_scope,omitempty"`
	Confidence              *string `json:"confidence,omitempty"`
	RequiresReview          bool    `json:"requires_review"`
	Notes                   *string `json:"notes,omitempty"`
}

// DefaultGovernancePolicyHint
func DefaultGovernancePolicyHint() GovernancePolicyHint {
	return GovernancePolicyHint{
		RequiresReview: true,
	}
}

type GovernanceLedgerMetadata struct {
	CreatedBy     *string           `json:"created_by,omitempty"`
	CorrelationId *string           `json:"correlation_id,omitempty"`
	Properties    map[string]string `json:"properties"`
}

// DefaultGovernanceLedgerMetadata
func DefaultGovernanceLedgerMetadata() GovernanceLedgerMetadata {
	return GovernanceLedgerMetadata{
		Properties: make(map[string]string),
	}
}

type GovernanceLedgerListQuery struct {
	Decision       *string    `json:"decision,omitempty"`
	Status         *string    `json:"status,omitempty"`
	ToolName       *string    `json:"tool_name,omitempty"`
	ActionType     *string    `json:"action_type,omitempty"`
	RiskLevel      *string    `json:"risk_level,omitempty"`
	Scope          *string    `json:"scope,omitempty"`
	SessionId      *string    `json:"session_id,omitempty"`
	ActorId        *string    `json:"actor_id,omitempty"`
	ChannelId      *string    `json:"channel_id,omitempty"`
	DecidedBy      *string    `json:"decided_by,omitempty"`
	Tag            *string    `json:"tag,omitempty"`
	CreatedFromUtc *time.Time `json:"created_from_utc,omitempty"`
	CreatedToUtc   *time.Time `json:"created_to_utc,omitempty"`
	Limit          int        `json:"limit"`
}

// DefaultGovernanceLedgerListQuery
func DefaultGovernanceLedgerListQuery() GovernanceLedgerListQuery {
	return GovernanceLedgerListQuery{
		Limit: 100,
	}
}

type GovernanceLedgerRevokeRequest struct {
	RevokedBy *string `json:"revoked_by,omitempty"`
	Reason    *string `json:"reason,omitempty"`
}

// --- Payload Responses ---

type GovernanceLedgerListResponse struct {
	Items []GovernanceLedgerEntry `json:"items"`
}

type GovernanceLedgerDetailResponse struct {
	Entry *GovernanceLedgerEntry `json:"entry,omitempty"`
}

type GovernanceLedgerMutationResponse struct {
	Success bool                   `json:"success"`
	Entry   *GovernanceLedgerEntry `json:"entry,omitempty"`
	Message string                 `json:"message"`
	Error   *string                `json:"error,omitempty"`
}
