package core

import (
	"time"
)

const (
	ConfidenceUnknown = "unknown"
	ConfidenceLow     = "low"
	ConfidenceMedium  = "medium"
	ConfidenceHigh    = "high"
)

const (
	ItemKindToolCall           = "tool_call"
	ItemKindTestResult         = "test_result"
	ItemKindBuildResult        = "build_result"
	ItemKindStaticAnalysis     = "static_analysis"
	ItemKindSecurityCheck      = "security_check"
	ItemKindApproval           = "approval"
	ItemKindHumanReview        = "human_review"
	ItemKindRuntimeEvent       = "runtime_event"
	ItemKindAuditEvent         = "audit_event"
	ItemKindDoctorReport       = "doctor_report"
	ItemKindPostureCheck       = "posture_check"
	ItemKindModelResponse      = "model_response"
	ItemKindMemoryLookup       = "memory_lookup"
	ItemKindVerificationResult = "verification_result"
	ItemKindNote               = "note"
	ItemKindUnknown            = "unknown"
)

const (
	CheckStatusNotRun  = "not_run"
	CheckStatusRunning = "running"
	CheckStatusPassed  = "passed"
	CheckStatusFailed  = "failed"
	CheckStatusSkipped = "skipped"
	CheckStatusWarning = "warning"
	CheckStatusUnknown = "unknown"
)

const (
	RiskLevelUnknown  = "unknown"
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

type EvidenceBundle struct {
	ID                 string                  `json:"id"`
	Title              string                  `json:"title"`
	Summary            string                  `json:"summary"`
	CreatedAtUtc       time.Time               `json:"created_at_utc"`
	UpdatedAtUtc       time.Time               `json:"updated_at_utc"`
	SourceSessionID    string                  `json:"source_session_id,omitempty"`
	HarnessContractID  string                  `json:"harness_contract_id,omitempty"`
	LearningProposalID string                  `json:"learning_proposal_id,omitempty"`
	ToolCallID         string                  `json:"tool_call_id,omitempty"`
	AutomationRunID    string                  `json:"automation_run_id,omitempty"`
	ActorID            string                  `json:"actor_id,omitempty"`
	ChannelID          string                  `json:"channel_id,omitempty"`
	SenderID           string                  `json:"sender_id,omitempty"`
	Confidence         string                  `json:"confidence"`
	Items              []EvidenceItem          `json:"items"`
	Checks             []EvidenceCheck         `json:"checks"`
	Risks              []EvidenceRisk          `json:"risks"`
	Assumptions        []EvidenceAssumption    `json:"assumptions"`
	UntestedAreas      []EvidenceUntestedArea  `json:"untested_areas"`
	HumanReviews       []EvidenceHumanReview   `json:"human_reviews"`
	Tags               []string                `json:"tags" gorm:"type:text[];not null;default:'{}'"`
	Metadata           *EvidenceBundleMetadata `json:"metadata,omitempty"`
}

func DefaultEvidenceBundle() EvidenceBundle {
	now := time.Now().UTC()
	return EvidenceBundle{
		CreatedAtUtc:  now,
		UpdatedAtUtc:  now,
		Confidence:    ConfidenceUnknown,
		Items:         []EvidenceItem{},
		Checks:        []EvidenceCheck{},
		Risks:         []EvidenceRisk{},
		Assumptions:   []EvidenceAssumption{},
		UntestedAreas: []EvidenceUntestedArea{},
		HumanReviews:  []EvidenceHumanReview{},
		Tags:          []string{},
	}
}

type EvidenceItem struct {
	ID              string            `json:"id"`
	Kind            string            `json:"kind"`
	Title           string            `json:"title"`
	Summary         string            `json:"summary"`
	Source          *EvidenceSource   `json:"source,omitempty"`
	CreatedAtUtc    time.Time         `json:"created_at_utc"`
	ToolName        string            `json:"tool_name,omitempty"`
	ToolCallID      string            `json:"tool_call_id,omitempty"`
	RuntimeEventID  string            `json:"runtime_event_id,omitempty"`
	AuditEventID    string            `json:"audit_event_id,omitempty"`
	Status          string            `json:"status,omitempty"`
	InputSummary    string            `json:"input_summary,omitempty"`
	OutputSummary   string            `json:"output_summary,omitempty"`
	ErrorSummary    string            `json:"error_summary,omitempty"`
	RedactedPayload string            `json:"redacted_payload,omitempty"`
	Metadata        map[string]string `json:"metadata"`
}

func DefaultEvidenceItem() EvidenceItem {
	return EvidenceItem{
		Kind:         ItemKindUnknown,
		CreatedAtUtc: time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
}

type EvidenceCheck struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Kind           string     `json:"kind,omitempty"`
	Required       bool       `json:"required"`
	Status         string     `json:"status"`
	StartedAtUtc   *time.Time `json:"started_at_utc,omitempty"`
	CompletedAtUtc *time.Time `json:"completed_at_utc,omitempty"`
	Summary        string     `json:"summary"`
	Details        string     `json:"details,omitempty"`
	Command        string     `json:"command,omitempty"`
	ExitCode       *int       `json:"exit_code,omitempty"`
	Error          string     `json:"error,omitempty"`
}

func DefaultEvidenceCheck() EvidenceCheck {
	return EvidenceCheck{
		Required: true,
		Status:   CheckStatusUnknown,
	}
}

type EvidenceRisk struct {
	RiskLevel     string     `json:"risk_level"`
	Description   string     `json:"description"`
	Mitigation    string     `json:"mitigation,omitempty"`
	Accepted      bool       `json:"accepted"`
	AcceptedBy    string     `json:"accepted_by,omitempty"`
	AcceptedAtUtc *time.Time `json:"accepted_at_utc,omitempty"`
}

func DefaultEvidenceRisk() EvidenceRisk {
	return EvidenceRisk{
		RiskLevel: RiskLevelUnknown,
	}
}

type EvidenceAssumption struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Verified       bool   `json:"verified"`
	EvidenceItemID string `json:"evidence_item_id,omitempty"`
}

type EvidenceUntestedArea struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Reason      string `json:"reason,omitempty"`
	RiskLevel   string `json:"risk_level,omitempty"`
}

type EvidenceHumanReview struct {
	Reviewer      string    `json:"reviewer,omitempty"`
	Decision      string    `json:"decision,omitempty"`
	Notes         string    `json:"notes,omitempty"`
	ReviewedAtUtc time.Time `json:"reviewed_at_utc"`
}

func DefaultEvidenceHumanReview() EvidenceHumanReview {
	return EvidenceHumanReview{
		ReviewedAtUtc: time.Now().UTC(),
	}
}

type EvidenceSource struct {
	Kind        string `json:"kind,omitempty"`
	ID          string `json:"id,omitempty"`
	Path        string `json:"path,omitempty"`
	Uri         string `json:"uri,omitempty"`
	Description string `json:"description,omitempty"`
}

type EvidenceBundleMetadata struct {
	CreatedBy     string            `json:"created_by,omitempty"`
	Source        string            `json:"source,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Properties    map[string]string `json:"properties"`
}

func DefaultEvidenceBundleMetadata() EvidenceBundleMetadata {
	return EvidenceBundleMetadata{
		Properties: make(map[string]string),
	}
}

type EvidenceBundleListQuery struct {
	SourceSessionID    string     `json:"source_session_id,omitempty"`
	HarnessContractID  string     `json:"harness_contract_id,omitempty"`
	LearningProposalID string     `json:"learning_proposal_id,omitempty"`
	ActorID            string     `json:"actor_id,omitempty"`
	ChannelID          string     `json:"channel_id,omitempty"`
	Confidence         string     `json:"confidence,omitempty"`
	Tag                string     `json:"tag,omitempty"`
	CreatedFromUtc     *time.Time `json:"created_from_utc,omitempty"`
	CreatedToUtc       *time.Time `json:"created_to_utc,omitempty"`
	Limit              int        `json:"limit"`
}

func DefaultEvidenceBundleListQuery() EvidenceBundleListQuery {
	return EvidenceBundleListQuery{
		Limit: 100,
	}
}

type EvidenceBundleListResponse struct {
	Items []EvidenceBundle `json:"items"`
}

func DefaultEvidenceBundleListResponse() EvidenceBundleListResponse {
	return EvidenceBundleListResponse{
		Items: []EvidenceBundle{},
	}
}

type EvidenceBundleDetailResponse struct {
	Bundle *EvidenceBundle `json:"bundle,omitempty"`
}

type EvidenceBundleMutationResponse struct {
	Success bool            `json:"success"`
	Bundle  *EvidenceBundle `json:"bundle,omitempty"`
	Message string          `json:"message"`
	Error   string          `json:"error,omitempty"`
}
