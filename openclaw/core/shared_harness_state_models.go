package core

import "time"

const (
	HarnessParticipantRolesManager          = "manager"
	HarnessParticipantRolesPlanner          = "planner"
	HarnessParticipantRolesCoder            = "coder"
	HarnessParticipantRolesReviewer         = "reviewer"
	HarnessParticipantRolesTester           = "tester"
	HarnessParticipantRolesSecurityReviewer = "security_reviewer"
	HarnessParticipantRolesOpsVerifier      = "ops_verifier"
	HarnessParticipantRolesDocsWriter       = "docs_writer"
	HarnessParticipantRolesResearcher       = "researcher"
	HarnessParticipantRolesOperator         = "operator"
	HarnessParticipantRolesCustom           = "custom"
)

const (
	HarnessStateStatusesActive    = "active"
	HarnessStateStatusesCompleted = "completed"
	HarnessStateStatusesFailed    = "failed"
	HarnessStateStatusesCancelled = "cancelled"
	HarnessStateStatusesBlocked   = "blocked"
	HarnessStateStatusesUnknown   = "unknown"
)

const (
	HarnessConflictPoliciesAllow     = "allow"
	HarnessConflictPoliciesWarn      = "warn"
	HarnessConflictPoliciesSerialize = "serialize"
	HarnessConflictPoliciesEscalate  = "escalate"
	HarnessConflictPoliciesReject    = "reject"
)

const (
	HarnessConflictTypesWriteWrite         = "write_write"
	HarnessConflictTypesReadWrite          = "read_write"
	HarnessConflictTypesAssumption         = "assumption"
	HarnessConflictTypesVerifierObligation = "verifier_obligation"
)

type SharedHarnessState struct {
	ID                  string                      `json:"id"`
	SessionID           string                      `json:"session_id,omitempty"`
	ParentSessionID     string                      `json:"parent_session_id,omitempty"`
	HarnessContractID   string                      `json:"harness_contract_id,omitempty"`
	CreatedAtUtc        time.Time                   `json:"created_at_utc"`
	UpdatedAtUtc        time.Time                   `json:"updated_at_utc"`
	Status              string                      `json:"status"`
	Goal                string                      `json:"goal"`
	Participants        []HarnessParticipant        `json:"participants"`
	Actions             []HarnessStateAction        `json:"actions"`
	SharedReadSet       []HarnessResourceRef        `json:"shared_read_set"`
	SharedWriteSet      []HarnessResourceRef        `json:"shared_write_set"`
	Assumptions         []HarnessAssumption         `json:"assumptions"`
	VersionDependencies []HarnessVersionDependency  `json:"version_dependencies"`
	VerifierObligations []HarnessVerifierObligation `json:"verifier_obligations"`
	Conflicts           []HarnessConflict           `json:"conflicts"`
	EvidenceBundleIds   []string                    `json:"evidence_bundle_ids"`
	GovernanceLedgerIds []string                    `json:"governance_ledger_ids"`
	Tags                []string                    `json:"tags"`
	Metadata            map[string]string           `json:"metadata"`
}

func NewDefaultSharedHarnessState() SharedHarnessState {
	now := time.Now().UTC()
	return SharedHarnessState{
		CreatedAtUtc:        now,
		UpdatedAtUtc:        now,
		Status:              HarnessStateStatusesActive,
		Participants:        []HarnessParticipant{},
		Actions:             []HarnessStateAction{},
		SharedReadSet:       []HarnessResourceRef{},
		SharedWriteSet:      []HarnessResourceRef{},
		Assumptions:         []HarnessAssumption{},
		VersionDependencies: []HarnessVersionDependency{},
		VerifierObligations: []HarnessVerifierObligation{},
		Conflicts:           []HarnessConflict{},
		EvidenceBundleIds:   []string{},
		GovernanceLedgerIds: []string{},
		Tags:                []string{},
		Metadata:            make(map[string]string),
	}
}

type HarnessParticipant struct {
	ID                  string     `json:"id"`
	AgentID             string     `json:"agent_id,omitempty"`
	SessionID           string     `json:"session_id,omitempty"`
	Role                string     `json:"role"`
	DisplayName         string     `json:"display_name,omitempty"`
	ModelProfileID      string     `json:"model_profile_id,omitempty"`
	ToolPreset          string     `json:"tool_preset,omitempty"`
	StartedAtUtc        time.Time  `json:"started_at_utc"`
	CompletedAtUtc      *time.Time `json:"completed_at_utc,omitempty"`
	Status              string     `json:"status"`
	ParentParticipantID string     `json:"parent_participant_id,omitempty"`
	Notes               string     `json:"notes,omitempty"`
}

func NewDefaultHarnessParticipant() HarnessParticipant {
	return HarnessParticipant{
		Role:         HarnessParticipantRolesCustom,
		StartedAtUtc: time.Now().UTC(),
		Status:       HarnessStateStatusesActive,
	}
}

type HarnessStateAction struct {
	ID                  string                      `json:"id"`
	ParticipantID       string                      `json:"participant_id,omitempty"`
	Title               string                      `json:"title"`
	Summary             string                      `json:"summary,omitempty"`
	Status              string                      `json:"status"`
	ToolName            string                      `json:"tool_name,omitempty"`
	ReadSet             []HarnessResourceRef        `json:"read_set"`
	WriteSet            []HarnessResourceRef        `json:"write_set"`
	Assumptions         []HarnessAssumption         `json:"assumptions"`
	VersionDependencies []HarnessVersionDependency  `json:"version_dependencies"`
	VerifierObligations []HarnessVerifierObligation `json:"verifier_obligations"`
	EvidenceBundleID    string                      `json:"evidence_bundle_id,omitempty"`
	HarnessContractID   string                      `json:"harness_contract_id,omitempty"`
	RiskLevel           string                      `json:"risk_level,omitempty"`
	StartedAtUtc        time.Time                   `json:"started_at_utc"`
	CompletedAtUtc      *time.Time                  `json:"completed_at_utc,omitempty"`
}

func NewDefaultHarnessStateAction() HarnessStateAction {
	return HarnessStateAction{
		Status:              HarnessStateStatusesActive,
		ReadSet:             []HarnessResourceRef{},
		WriteSet:            []HarnessResourceRef{},
		Assumptions:         []HarnessAssumption{},
		VersionDependencies: []HarnessVersionDependency{},
		VerifierObligations: []HarnessVerifierObligation{},
		StartedAtUtc:        time.Now().UTC(),
	}
}

type HarnessReadWriteSet struct {
	ReadSet  []HarnessResourceRef `json:"read_set"`
	WriteSet []HarnessResourceRef `json:"write_set"`
}

func NewDefaultHarnessReadWriteSet() HarnessReadWriteSet {
	return HarnessReadWriteSet{
		ReadSet:  []HarnessResourceRef{},
		WriteSet: []HarnessResourceRef{},
	}
}

type HarnessResourceRef struct {
	Kind        string `json:"kind"`
	Path        string `json:"path,omitempty"`
	Key         string `json:"key,omitempty"`
	ID          string `json:"id,omitempty"`
	Uri         string `json:"uri,omitempty"`
	Description string `json:"description,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Version     string `json:"version,omitempty"`
	IsSensitive bool   `json:"is_sensitive"`
}

func NewDefaultHarnessResourceRef() HarnessResourceRef {
	return HarnessResourceRef{
		Kind:        HarnessContractResourceKindsUnknown,
		IsSensitive: false,
	}
}

type HarnessAssumption struct {
	ID               string `json:"id"`
	Key              string `json:"key,omitempty"`
	Value            string `json:"value,omitempty"`
	Text             string `json:"text"`
	Verified         bool   `json:"verified"`
	EvidenceBundleID string `json:"evidence_bundle_id,omitempty"`
}

type HarnessVersionDependency struct {
	ID          string              `json:"id"`
	Resource    *HarnessResourceRef `json:"resource,omitempty"`
	Version     string              `json:"version,omitempty"`
	Description string              `json:"description,omitempty"`
	Required    bool                `json:"required"`
}

func NewDefaultHarnessVersionDependency() HarnessVersionDependency {
	return HarnessVersionDependency{
		Required: true,
	}
}

type HarnessVerifierObligation struct {
	ID               string              `json:"id"`
	Title            string              `json:"title"`
	Verifier         string              `json:"verifier,omitempty"`
	Required         bool                `json:"required"`
	Resource         *HarnessResourceRef `json:"resource,omitempty"`
	Status           string              `json:"status"`
	Summary          string              `json:"summary,omitempty"`
	EvidenceBundleID string              `json:"evidence_bundle_id,omitempty"`
}

func NewDefaultHarnessVerifierObligation() HarnessVerifierObligation {
	return HarnessVerifierObligation{
		Required: true,
		Status:   HarnessStateStatusesUnknown,
	}
}

type HarnessConflict struct {
	ID             string               `json:"id"`
	Type           string               `json:"type"`
	Summary        string               `json:"summary"`
	Participants   []string             `json:"participants"`
	Actions        []string             `json:"actions"`
	Resources      []HarnessResourceRef `json:"resources"`
	Policy         string               `json:"policy"`
	Severity       string               `json:"severity"`
	Status         string               `json:"status"`
	Recommendation string               `json:"recommendation,omitempty"`
}

func NewDefaultHarnessConflict() HarnessConflict {
	return HarnessConflict{
		Participants: []string{},
		Actions:      []string{},
		Resources:    []HarnessResourceRef{},
		Policy:       HarnessConflictPoliciesWarn,
		Severity:     HarnessContractRiskLevelsMedium,
		Status:       HarnessStateStatusesActive,
	}
}

type SharedHarnessStateListQuery struct {
	SessionID         string     `json:"session_id,omitempty"`
	ParentSessionID   string     `json:"parent_session_id,omitempty"`
	HarnessContractID string     `json:"harness_contract_id,omitempty"`
	Status            string     `json:"status,omitempty"`
	Tag               string     `json:"tag,omitempty"`
	CreatedFromUtc    *time.Time `json:"created_from_utc,omitempty"`
	CreatedToUtc      *time.Time `json:"created_to_utc,omitempty"`
	Limit             int        `json:"limit"`
}

func NewDefaultSharedHarnessStateListQuery() SharedHarnessStateListQuery {
	return SharedHarnessStateListQuery{
		Limit: 100,
	}
}

type SharedHarnessStateListResponse struct {
	Items []SharedHarnessState `json:"items"`
}

func NewDefaultSharedHarnessStateListResponse() SharedHarnessStateListResponse {
	return SharedHarnessStateListResponse{
		Items: []SharedHarnessState{},
	}
}

type SharedHarnessStateDetailResponse struct {
	State *SharedHarnessState `json:"state,omitempty"`
}

type SharedHarnessStateMutationResponse struct {
	Success bool                `json:"success"`
	State   *SharedHarnessState `json:"state,omitempty"`
	Message string              `json:"message"`
	Error   string              `json:"error,omitempty"`
}
