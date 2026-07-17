package core

import (
	"sync/atomic"
	"time"
)

type SessionState uint8

const (
	SessionStateActive SessionState = iota
	SessionStatePaused
	SessionStateExpired
)

type SessionRunState uint8

const (
	SessionRunState_Idle SessionRunState = iota
	SessionRunState_Running
	SessionRunState_Continuing
	SessionRunState_Paused
	SessionRunState_Blocked
	SessionRunState_BudgetLimited
	SessionRunState_Completed
	SessionRunState_Failed
)
const (
	SessionCheckpointKindsToolBatch = "tool_batch"
)

const (
	SessionCheckpointStatesReadyToResume = "ready_to_resume"
	SessionCheckpointStatesCompleted     = "completed"
	SessionCheckpointStatesFailed        = "failed"
)

// ============================================================================
// Structs & Factories
// ============================================================================

type BackgroundRunMetadata struct {
	RunId                      string     `json:"run_id"`
	Objective                  string     `json:"objective"`
	StartedAtUtc               time.Time  `json:"started_at_utc"`
	LastContinuedAtUtc         *time.Time `json:"last_continued_at_utc,omitempty"`
	LastNotificationAtUtc      time.Time  `json:"last_notification_at_utc"`
	ContinuationCount          int        `json:"continuation_count"`
	ContinuationSequence       int        `json:"continuation_sequence"`
	ConsecutiveNoProgressCount int        `json:"consecutive_no_progress_count"`
	ToolCallCount              int64      `json:"tool_call_count"`
	TokenBudget                int64      `json:"token_budget"`
	MaxContinuationTurns       int        `json:"max_continuation_turns"`
	LastCheckpointId           string     `json:"last_checkpoint_id"`
	LastStopReason             string     `json:"last_stop_reason"`
}

type Session struct {
	totalInputTokens      *int64
	totalOutputTokens     *int64
	totalCacheReadTokens  *int64
	totalCacheWriteTokens *int64

	Id                           string                          `json:"id"`
	ChannelId                    string                          `json:"channel_id"`
	SenderId                     string                          `json:"sender_id"`
	StableSessionBinding         *StableSessionBindingInfo       `json:"stable_session_binding,omitempty"`
	CreatedAt                    time.Time                       `json:"created_at"`
	UpdatedAt                    time.Time                       `json:"updated_at"`
	LastActiveAt                 time.Time                       `json:"last_active_at"`
	History                      []ChatTurn                      `json:"history"`
	State                        SessionState                    `json:"state"`
	RunState                     SessionRunState                 `json:"run_state"`
	BackgroundRun                *BackgroundRunMetadata          `json:"background_run"`
	ModelOverride                string                          `json:"model_override,omitempty"`
	ModelProfileId               string                          `json:"model_profile_id,omitempty"`
	PreferredModelTags           []string                        `json:"preferred_model_tags" gorm:"type:text[];not null;default:'{}'"`
	FallbackModelProfileIds      []string                        `json:"fallback_model_profile_ids" gorm:"type:text[];not null;default:'{}'"`
	ModelRequirements            ModelSelectionRequirements      `json:"model_requirements"`
	SystemPromptOverride         string                          `json:"system_prompt_override,omitempty"`
	RoutePresetId                string                          `json:"route_preset_id,omitempty"`
	RouteAllowedTools            []string                        `json:"route_allowed_tools" gorm:"type:text[];not null;default:'{}'"`
	ReasoningEffort              string                          `json:"reasoning_effort,omitempty"`
	VerboseMode                  bool                            `json:"verbose_mode"`
	ResponseMode                 string                          `json:"response_mode"`
	ContractPolicy               *ContractPolicy                 `json:"contract_policy,omitempty"`
	Delegation                   *SessionDelegationMetadata      `json:"delegation,omitempty"`
	DelegatedSessions            []SessionDelegationChildSummary `json:"delegated_sessions"`
	ContractAttachedAtUtc        *time.Time                      `json:"contract_attached_at_utc,omitempty"`
	ContractBaselineInputTokens  int64                           `json:"contract_baseline_input_tokens"`
	ContractBaselineOutputTokens int64                           `json:"contract_baseline_output_tokens"`
	ContractBaselineToolCalls    int                             `json:"contract_baseline_tool_calls"`
	ContractAccumulatedCostUsd   float64                         `json:"contract_accumulated_cost_usd"`
	ExecutionCheckpoint          *SessionExecutionCheckpoint     `json:"execution_checkpoint,omitempty"`
}

func DefaultSession() *Session {
	now := time.Now().UTC()
	return &Session{
		totalInputTokens:        new(int64),
		totalOutputTokens:       new(int64),
		totalCacheReadTokens:    new(int64),
		totalCacheWriteTokens:   new(int64),
		CreatedAt:               now,
		LastActiveAt:            now,
		History:                 []ChatTurn{},
		State:                   SessionStateActive,
		RunState:                SessionRunState_Idle,
		PreferredModelTags:      []string{},
		FallbackModelProfileIds: []string{},
		RouteAllowedTools:       []string{},
		ResponseMode:            SessionResponseModesDefault,
		DelegatedSessions:       []SessionDelegationChildSummary{},
	}
}

type StableSessionBindingInfo struct {
	ExternalSessionId string    `json:"external_session_id"`
	Namespace         string    `json:"namespace"`
	OwnerKey          string    `json:"owner_key"`
	BoundAtUtc        time.Time `json:"bound_at_utc"`
}

func DefaultStableSessionBindingInfo() *StableSessionBindingInfo {
	return &StableSessionBindingInfo{
		BoundAtUtc: time.Now().UTC(),
	}
}

type ChatTurn struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Timestamp time.Time        `json:"timestamp"`
	ToolCalls []ToolInvocation `json:"tool_calls,omitempty"`
}

func DefaultChatTurn() *ChatTurn {
	return &ChatTurn{
		Timestamp: time.Now().UTC(),
	}
}

type ToolInvocation struct {
	CallId                 string        `json:"call_id,omitempty"`
	ToolName               string        `json:"tool_name"`
	Arguments              string        `json:"arguments"`
	Result                 string        `json:"result,omitempty"`
	Duration               time.Duration `json:"duration"`
	ResultStatus           string        `json:"result_status,omitempty"`
	FailureCode            string        `json:"failure_code,omitempty"`
	FailureMessage         string        `json:"failure_message,omitempty"`
	NextStep               string        `json:"next_step,omitempty"`
	GovernanceAllowed      *bool         `json:"governance_allowed,omitempty"`
	GovernanceAction       string        `json:"governance_action,omitempty"`
	GovernanceReason       string        `json:"governance_reason,omitempty"`
	GovernancePolicyId     string        `json:"governance_policy_id,omitempty"`
	GovernanceRuleId       string        `json:"governance_rule_id,omitempty"`
	GovernanceTrustScore   *float64      `json:"governance_trust_score,omitempty"`
	GovernanceEvaluationMs *float64      `json:"governance_evaluation_ms,omitempty"`
	GovernanceUnavailable  *bool         `json:"governance_unavailable,omitempty"`
}

type SessionExecutionCheckpoint struct {
	CheckpointId           string                      `json:"checkpoint_id"`
	Kind                   string                      `json:"kind"`
	State                  string                      `json:"state"`
	Sequence               int                         `json:"sequence"`
	Iteration              int                         `json:"iteration"`
	HistoryCount           int                         `json:"history_count"`
	CorrelationId          string                      `json:"correlation_id,omitempty"`
	CreatedAtUtc           time.Time                   `json:"created_at_utc"`
	PersistedAtUtc         *time.Time                  `json:"persisted_at_utc,omitempty"`
	LastResumeAttemptAtUtc *time.Time                  `json:"last_resume_attempt_at_utc,omitempty"`
	CompletedAtUtc         *time.Time                  `json:"completed_at_utc,omitempty"`
	CompletionReason       string                      `json:"completion_reason,omitempty"`
	ToolCalls              []SessionCheckpointToolCall `json:"tool_calls"`
}

func DefaultSessionExecutionCheckpoint() *SessionExecutionCheckpoint {
	return &SessionExecutionCheckpoint{
		Kind:         SessionCheckpointKindsToolBatch,
		State:        SessionCheckpointStatesReadyToResume,
		CreatedAtUtc: time.Now().UTC(),
		ToolCalls:    []SessionCheckpointToolCall{},
	}
}

func (s *Session) GetTotalInputTokens() int64 {
	return atomic.LoadInt64(s.totalInputTokens)
}

func (s *Session) SetTotalInputTokens(val int64) {
	atomic.StoreInt64(s.totalInputTokens, val)
}

func (s *Session) GetTotalOutputTokens() int64 {
	return atomic.LoadInt64(s.totalOutputTokens)
}

func (s *Session) SetTotalOutputTokens(val int64) {
	atomic.StoreInt64(s.totalOutputTokens, val)
}

func (s *Session) GetTotalCacheReadTokens() int64 {
	return atomic.LoadInt64(s.totalCacheReadTokens)
}

func (s *Session) SetTotalCacheReadTokens(val int64) {
	atomic.StoreInt64(s.totalCacheReadTokens, val)
}

func (s *Session) GetTotalCacheWriteTokens() int64 {
	return atomic.LoadInt64(s.totalCacheWriteTokens)
}

func (s *Session) SetTotalCacheWriteTokens(val int64) {
	atomic.StoreInt64(s.totalCacheWriteTokens, val)
}

func (s *Session) AddTokenUsage(inputTokens int64, outputTokens int64) {
	if inputTokens != 0 {
		atomic.AddInt64(s.totalInputTokens, inputTokens)
	}
	if outputTokens != 0 {
		atomic.AddInt64(s.totalOutputTokens, outputTokens)
	}
}

func (s *Session) AddCacheUsage(cacheReadTokens int64, cacheWriteTokens int64) {
	if cacheReadTokens != 0 {
		atomic.AddInt64(s.totalCacheReadTokens, cacheReadTokens)
	}
	if cacheWriteTokens != 0 {
		atomic.AddInt64(s.totalCacheWriteTokens, cacheWriteTokens)
	}
}

func (s *Session) GetTotalTokens() int64 {
	return s.GetTotalInputTokens() + s.GetTotalOutputTokens()
}

// Constants for SessionCheckpointToolCall
const (
	SessionCheckpointToolCallDefaultResultStatus = "Completed"
)

type SessionCheckpointToolCall struct {
	CallId         string `json:"call_id,omitempty"`
	ToolName       string `json:"tool_name"`
	ResultStatus   string `json:"result_status"`
	FailureCode    string `json:"failure_code,omitempty"`
	DurationMs     int64  `json:"duration_ms"`
	ArgumentsBytes int    `json:"arguments_bytes"`
	ResultBytes    int    `json:"result_bytes"`
}

func NewDefaultSessionCheckpointToolCall() SessionCheckpointToolCall {
	return SessionCheckpointToolCall{
		ResultStatus: SessionCheckpointToolCallDefaultResultStatus,
	}
}

// -----------------------------------------------------------------------------

// Constants for SessionDelegationMetadata
const (
	SessionDelegationMetadataDefaultStatus = "running"
)

type SessionDelegationMetadata struct {
	ParentSessionId      string                           `json:"parent_session_id,omitempty"`
	ParentChannelId      string                           `json:"parent_channel_id,omitempty"`
	ParentSenderId       string                           `json:"parent_sender_id,omitempty"`
	Profile              string                           `json:"profile"`
	RequestedTask        string                           `json:"requested_task"`
	AllowedTools         []string                         `json:"allowed_tools"`
	Depth                int                              `json:"depth"`
	StartedAtUtc         time.Time                        `json:"started_at_utc"`
	CompletedAtUtc       *time.Time                       `json:"completed_at_utc,omitempty"`
	Status               string                           `json:"status"`
	FinalResponsePreview string                           `json:"final_response_preview,omitempty"`
	ToolUsage            []SessionDelegationToolUsage     `json:"tool_usage"`
	ProposedChanges      []SessionDelegationChangeSummary `json:"proposed_changes"`
}

func NewDefaultSessionDelegationMetadata() SessionDelegationMetadata {
	return SessionDelegationMetadata{
		Profile:         "",
		RequestedTask:   "",
		AllowedTools:    []string{},
		StartedAtUtc:    time.Now().UTC(),
		Status:          SessionDelegationMetadataDefaultStatus,
		ToolUsage:       []SessionDelegationToolUsage{},
		ProposedChanges: []SessionDelegationChangeSummary{},
	}
}

// -----------------------------------------------------------------------------

type SessionDelegationToolUsage struct {
	ToolName   string `json:"tool_name"`
	Action     string `json:"action"`
	Summary    string `json:"summary"`
	IsMutation bool   `json:"is_mutation"`
	Count      int    `json:"count"`
}

// -----------------------------------------------------------------------------

type SessionDelegationChangeSummary struct {
	ToolName string `json:"tool_name"`
	Action   string `json:"action"`
	Summary  string `json:"summary"`
}

// -----------------------------------------------------------------------------

// Constants for SessionDelegationChildSummary
const (
	SessionDelegationChildSummaryDefaultStatus = "running"
)

type SessionDelegationChildSummary struct {
	SessionId            string                           `json:"session_id"`
	Profile              string                           `json:"profile"`
	TaskPreview          string                           `json:"task_preview"`
	StartedAtUtc         time.Time                        `json:"started_at_utc"`
	CompletedAtUtc       *time.Time                       `json:"completed_at_utc,omitempty"`
	Status               string                           `json:"status"`
	ToolUsage            []SessionDelegationToolUsage     `json:"tool_usage"`
	ProposedChanges      []SessionDelegationChangeSummary `json:"proposed_changes"`
	FinalResponsePreview string                           `json:"final_response_preview,omitempty"`
}

func NewDefaultSessionDelegationChildSummary() SessionDelegationChildSummary {
	return SessionDelegationChildSummary{
		StartedAtUtc:    time.Now().UTC(),
		Status:          SessionDelegationChildSummaryDefaultStatus,
		ToolUsage:       []SessionDelegationToolUsage{},
		ProposedChanges: []SessionDelegationChangeSummary{},
	}
}

// -----------------------------------------------------------------------------

type SessionSummary struct {
	Id                     string       `json:"id"`
	ChannelId              string       `json:"channel_id"`
	SenderId               string       `json:"sender_id"`
	StableSessionId        string       `json:"stable_session_id,omitempty"`
	StableSessionNamespace string       `json:"stable_session_namespace,omitempty"`
	StableSessionOwnerKey  string       `json:"stable_session_owner_key,omitempty"`
	CreatedAt              time.Time    `json:"created_at"`
	LastActiveAt           time.Time    `json:"last_active_at"`
	State                  SessionState `json:"state"`
	HistoryTurns           int          `json:"history_turns"`
	TotalInputTokens       int64        `json:"total_input_tokens"`
	TotalOutputTokens      int64        `json:"total_output_tokens"`
	TotalCacheReadTokens   int64        `json:"total_cache_read_tokens"`
	TotalCacheWriteTokens  int64        `json:"total_cache_write_tokens"`
	IsActive               bool         `json:"is_active"`
}

// -----------------------------------------------------------------------------

type PagedSessionList struct {
	Page          int              `json:"page"`
	PageSize      int              `json:"page_size"`
	HasMore       bool             `json:"has_more"`
	ReturnedCount int              `json:"returned_count"`
	Items         []SessionSummary `json:"items"`
}

func NewDefaultPagedSessionList() PagedSessionList {
	return PagedSessionList{
		Items: []SessionSummary{},
	}
}
func (p *PagedSessionList) GetReturnedCount() int {
	return len(p.Items)
}

type SessionListQuery struct {
	Search    string        `json:"search,omitempty"`
	ChannelId string        `json:"channel_id,omitempty"`
	SenderId  string        `json:"sender_id,omitempty"`
	FromUtc   *time.Time    `json:"from_utc,omitempty"`
	ToUtc     *time.Time    `json:"to_utc,omitempty"`
	State     *SessionState `json:"state,omitempty"`
	Starred   *bool         `json:"starred,omitempty"`
	Tag       string        `json:"tag,omitempty"`
}

type SessionBranch struct {
	BranchId  string     `json:"branch_id"`
	SessionId string     `json:"session_id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	History   []ChatTurn `json:"history"`
}

const (
	SessionSearchQueryDefaultLimit         = 25
	SessionSearchQueryDefaultSnippetLength = 180
)

type SessionSearchQuery struct {
	Text          string     `json:"text"`
	ChannelID     string     `json:"channel_id"`
	SenderID      string     `json:"sender_id"`
	FromUtc       *time.Time `json:"from_utc"`
	ToUtc         *time.Time `json:"to_utc"`
	Limit         int        `json:"limit"`
	SnippetLength int        `json:"snippet_length"`
}

func DefaultSessionSearchQuery() SessionSearchQuery {
	return SessionSearchQuery{
		Limit:         SessionSearchQueryDefaultLimit,
		SnippetLength: SessionSearchQueryDefaultSnippetLength,
	}
}

type SessionTurnsFts struct {
	SessionID string    `json:"session_id"`
	ChannelID string    `json:"channel_id"`
	SenderID  string    `json:"sender_id"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

type SessionSearchHit struct {
	SessionID string    `json:"session_id"`
	ChannelID string    `json:"channel_id"`
	SenderID  string    `json:"sender_id"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	Snippet   string    `json:"snippet"`
	Score     float32   `json:"score"`
	Rank      float32   `json:"rank"`
}

type SessionSearchResult struct {
	Query *SessionSearchQuery `json:"query"`
	Items []SessionSearchHit  `json:"items"`
}

type SessionMetaExecutionCheckpoint struct {
	SkillName        string                  `json:"skill_name"`
	PendingStepId    string                  `json:"pending_step_id"`
	Prompt           string                  `json:"prompt"`
	CreatedAtUtc     time.Time               `json:"created_at_utc"`
	LastUpdatedAtUtc time.Time               `json:"last_updated_at_utc"`
	PendingStepIds   []string                `json:"pending_step_ids"`
	BlockedStepIds   []string                `json:"blocked_step_ids"`
	Outputs          map[string]string       `json:"outputs"`
	FailureAliases   map[string]string       `json:"failure_aliases"`
	StepResults      []SessionMetaStepResult `json:"step_results"`
}

func DefaultSessionMetaExecutionCheckpoint() *SessionMetaExecutionCheckpoint {
	return &SessionMetaExecutionCheckpoint{
		CreatedAtUtc:     time.Now().UTC(),
		LastUpdatedAtUtc: time.Now().UTC(),
	}
}

type SessionMetaRunRecord struct {
	RunId          string                  `json:"run_id"`
	SkillName      string                  `json:"skill_name"`
	Status         string                  `json:"status"`
	FinalText      string                  `json:"final_text"`
	Error          string                  `json:"error"`
	ErrorCode      string                  `json:"error_code"`
	StartedAtUtc   time.Time               `json:"started_at_utc"`
	CompletedAtUtc time.Time               `json:"completed_at_utc"`
	StepResults    []SessionMetaStepResult `json:"step_results"`
}

func DefaultSessionMetaRunRecord() *SessionMetaRunRecord {
	return &SessionMetaRunRecord{
		Status:         "completed",
		StartedAtUtc:   time.Now().UTC(),
		CompletedAtUtc: time.Now().UTC(),
	}
}

type SessionMetaStepResult struct {
	Id                string                            `json:"id"`
	Kind              string                            `json:"kind"`
	Status            string                            `json:"status"`
	FailureCode       string                            `json:"failure_code"`
	DurationMs        float64                           `json:"duration_ms"`
	Continued         bool                              `json:"continued"`
	ExecutionEvidence *SessionMetaStepExecutionEvidence `json:"execution_evidence"`
}

type SessionMetaStepExecutionEvidence struct {
	CommandPreview string `json:"command_preview"`
	InputMode      string `json:"input_mode"`
	StdinBytes     int    `json:"stdin_bytes"`
	ParseMode      string `json:"parse_mode"`
}

func DefaultSessionMetaStepExecutionEvidence() *SessionMetaStepExecutionEvidence {
	return &SessionMetaStepExecutionEvidence{
		InputMode: "none",
		ParseMode: "text",
	}
}

type MetaRunReplayPreviewResponse struct {
	SessionId           string                            `json:"session_id"`
	RunId               string                            `json:"run_id"`
	SkillName           string                            `json:"skill_name"`
	ReplayAvailable     bool                              `json:"replay_available"`
	Reason              string                            `json:"reason"`
	AvailableArtifacts  []string                          `json:"available_artifacts"`
	RetainedSteps       []MetaRunReplayStepPreview        `json:"retained_steps"`
	Plan                *MetaRunReplayPlanPreview         `json:"plan"`
	MissingRequirements []MetaRunReplayRequirementPreview `json:"missing_requirements"`
	OperatorSummary     *MetaRunReplayOperatorSummary     `json:"operator_summary"`
	TriageHints         []MetaRunReplayTriageHint         `json:"triage_hints"`
}

type MetaRunReplayStepPreview struct {
	Id          string  `json:"id"`
	Kind        string  `json:"kind"`
	Status      string  `json:"status"`
	FailureCode string  `json:"failure_code"`
	DurationMs  float64 `json:"duration_ms"`
	Continued   bool    `json:"continued"`
}

type MetaRunReplayRequirementPreview struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

type MetaRunReplayPlanPreview struct {
	Summary               string                              `json:"summary"`
	Mode                  string                              `json:"mode"`
	Executable            bool                                `json:"executable"`
	ReplayableSteps       []MetaRunReplayStepReadinessPreview `json:"replayable_steps"`
	BlockedByRequirements []MetaRunReplayRequirementPreview   `json:"blocked_by_requirements"`
}

func DefaultMetaRunReplayPlanPreview() *MetaRunReplayPlanPreview {
	return &MetaRunReplayPlanPreview{
		Summary: "auditable_not_replayable",
		Mode:    "preview_only",
	}
}

type MetaRunReplayStepReadinessPreview struct {
	Id        string `json:"id"`
	Readiness string `json:"readiness"`
	Reason    string `json:"reason"`
}

type MetaRunReplayResultResponse struct {
	SessionId       string                          `json:"session_id"`
	RunId           string                          `json:"run_id"`
	SkillName       string                          `json:"skill_name"`
	Mode            string                          `json:"mode"`
	Status          string                          `json:"status"`
	Source          string                          `json:"source"`
	FinalText       string                          `json:"final_text"`
	Error           string                          `json:"error"`
	ErrorCode       string                          `json:"error_code"`
	Timeline        []MetaRunReplayTimelineItem     `json:"timeline"`
	Checkpoint      *MetaRunReplayCheckpointSummary `json:"checkpoint"`
	ProposalSummary *MetaRunProposalSummary         `json:"proposal_summary"`
	OperatorSummary *MetaRunReplayOperatorSummary   `json:"operator_summary"`
	TriageHints     []MetaRunReplayTriageHint       `json:"triage_hints"`
}

func DefaultMetaRunReplayResultResponse() *MetaRunReplayResultResponse {
	return &MetaRunReplayResultResponse{
		Mode:   "audit_reconstruction",
		Source: "history_only",
	}
}

type MetaRunReplayOperatorSummary struct {
	TotalSteps                    int                        `json:"total_steps"`
	FailedSteps                   int                        `json:"failed_steps"`
	ContinuedSteps                int                        `json:"continued_steps"`
	SkillExecSteps                int                        `json:"skill_exec_steps"`
	SkillExecStepsWithoutEvidence int                        `json:"skill_exec_steps_without_evidence"`
	StepKinds                     []MetaRunReplayCountBucket `json:"step_kinds"`
	FailureClusters               []MetaRunReplayCountBucket `json:"failure_clusters"`
}

type MetaRunReplayCountBucket struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type MetaRunReplayTriageHint struct {
	Code             string   `json:"code"`
	Priority         int      `json:"priority"`
	Message          string   `json:"message"`
	StepIds          []string `json:"step_ids"`
	RequirementNames []string `json:"requirement_names"`
}

type MetaRunReplayTimelineItem struct {
	Sequence    int     `json:"sequence"`
	StepId      string  `json:"step_id"`
	Kind        string  `json:"kind"`
	Status      string  `json:"status"`
	FailureCode string  `json:"failure_code"`
	DurationMs  float64 `json:"duration_ms"`
	Continued   bool    `json:"continued"`
	Source      string  `json:"source"`
	Notes       string  `json:"notes"`
}

func DefaultMetaRunReplayTimelineItem() *MetaRunReplayTimelineItem {
	return &MetaRunReplayTimelineItem{
		Source: "run_history",
	}
}

type MetaRunReplayCheckpointSummary struct {
	PendingStepId       string   `json:"pending_step_id"`
	PendingStepIds      []string `json:"pending_step_ids"`
	BlockedStepIds      []string `json:"blocked_step_ids"`
	PromptPresent       bool     `json:"prompt_present"`
	OutputStepIds       []string `json:"output_step_ids"`
	FailureAliasStepIds []string `json:"failure_alias_step_ids"`
}

type MetaRunProposalSummary struct {
	Available bool     `json:"available"`
	Count     int      `json:"count"`
	Kinds     []string `json:"kinds"`
	Reason    string   `json:"reason"`
}

func DefaultMetaRunProposalSummary() *MetaRunProposalSummary {
	return &MetaRunProposalSummary{
		Reason: "proposal_workflow_not_implemented",
	}
}

type MetaRunDerivedProposalListResponse struct {
	SessionId     string                          `json:"session_id"`
	Entrypoint    string                          `json:"entrypoint"`
	ReadOnlyAlias bool                            `json:"read_only_alias"`
	Count         int                             `json:"count"`
	Proposals     []MetaRunDerivedProposalSummary `json:"proposals"`
}

func DefaultMetaRunDerivedProposalListResponse() *MetaRunDerivedProposalListResponse {
	return &MetaRunDerivedProposalListResponse{
		Entrypoint: "skills meta-runs proposals",
	}
}

type MetaRunDerivedProposalSummary struct {
	Id               string     `json:"id"`
	RunId            string     `json:"run_id"`
	SkillName        string     `json:"skill_name"`
	Status           string     `json:"status"`
	Kind             string     `json:"kind"`
	Title            string     `json:"title"`
	Summary          string     `json:"summary"`
	Source           string     `json:"source"`
	AvailableActions []string   `json:"available_actions"`
	ReviewStatus     string     `json:"review_status"`
	ReviewedAtUtc    *time.Time `json:"reviewed_at_utc"`
}

func DefaultMetaRunDerivedProposalSummary() *MetaRunDerivedProposalSummary {
	return &MetaRunDerivedProposalSummary{
		Source:           "derived_meta_run_evidence",
		AvailableActions: []string{"show"},
		ReviewStatus:     "pending",
	}
}

type MetaRunDerivedProposalDetailResponse struct {
	SessionId     string                        `json:"session_id"`
	Entrypoint    string                        `json:"entrypoint"`
	ReadOnlyAlias bool                          `json:"read_only_alias"`
	Proposal      *MetaRunDerivedProposalDetail `json:"proposal"`
}

func DefaultMetaRunDerivedProposalDetailResponse() *MetaRunDerivedProposalDetailResponse {
	return &MetaRunDerivedProposalDetailResponse{
		Entrypoint: "skills meta-runs proposals",
	}
}

type MetaRunDerivedProposalDetail struct {
	Id                string                                  `json:"id"`
	RunId             string                                  `json:"run_id"`
	SkillName         string                                  `json:"skill_name"`
	Status            string                                  `json:"status"`
	Kind              string                                  `json:"kind"`
	Title             string                                  `json:"title"`
	Summary           string                                  `json:"summary"`
	Source            string                                  `json:"source"`
	AvailableActions  []string                                `json:"available_actions"`
	Checkpoint        *MetaRunDerivedProposalCheckpointDetail `json:"checkpoint"`
	Evidence          *MetaRunDerivedProposalEvidenceDetail   `json:"evidence"`
	Provenance        *MetaRunProposalProvenanceDetail        `json:"provenance"`
	Lifecycle         *MetaRunProposalLifecycleDetail         `json:"lifecycle"`
	Audit             *MetaRunProposalAuditDetail             `json:"audit"`
	Workflow          *MetaRunProposalWorkflowDetail          `json:"workflow"`
	ProvenanceHistory []MetaRunProposalProvenanceTransition   `json:"provenance_history"`
	Review            *MetaRunProposalReviewDetail            `json:"review"`
	PendingStepId     string                                  `json:"pending_step_id"`
	PendingStepIds    []string                                `json:"pending_step_ids"`
	BlockedStepIds    []string                                `json:"blocked_step_ids"`
	TimelineStepIds   []string                                `json:"timeline_step_ids"`
	Steps             []MetaRunDerivedProposalStepDetail      `json:"steps"`
	ErrorCode         string                                  `json:"error_code"`
	Error             string                                  `json:"error"`
	FinalText         string                                  `json:"final_text"`
}

func DefaultMetaRunDerivedProposalDetail() *MetaRunDerivedProposalDetail {
	return &MetaRunDerivedProposalDetail{
		Source:           "derived_meta_run_evidence",
		AvailableActions: []string{"show"},
	}
}

type MetaRunProposalLifecycleDetail struct {
	Status          string     `json:"status"`
	RolledBack      bool       `json:"rolled_back"`
	ReviewedAtUtc   *time.Time `json:"reviewed_at_utc"`
	RolledBackAtUtc *time.Time `json:"rolled_back_at_utc"`
	ReviewNotes     string     `json:"review_notes"`
	RollbackReason  string     `json:"rollback_reason"`
}

type MetaRunProposalProvenanceTransition struct {
	Action       string    `json:"action"`
	FromStatus   string    `json:"from_status"`
	ToStatus     string    `json:"to_status"`
	ChangedAtUtc time.Time `json:"changed_at_utc"`
	Reason       string    `json:"reason"`
}

type MetaRunProposalProvenanceDetail struct {
	SnapshotVersion         string    `json:"snapshot_version"`
	CapturedAtUtc           time.Time `json:"captured_at_utc"`
	RunStatus               string    `json:"run_status"`
	RunStartedAtUtc         time.Time `json:"run_started_at_utc"`
	RunCompletedAtUtc       time.Time `json:"run_completed_at_utc"`
	StepCount               int       `json:"step_count"`
	StepIds                 []string  `json:"step_ids"`
	CheckpointPendingStepId string    `json:"checkpoint_pending_step_id"`
	CheckpointPromptPresent bool      `json:"checkpoint_prompt_present"`
}

func DefaultMetaRunProposalProvenanceDetail() *MetaRunProposalProvenanceDetail {
	return &MetaRunProposalProvenanceDetail{
		SnapshotVersion: "v1",
	}
}

type MetaRunProposalReviewRecord struct {
	SessionId     string    `json:"session_id"`
	ProposalId    string    `json:"proposal_id"`
	ReviewStatus  string    `json:"review_status"`
	Reason        string    `json:"reason"`
	ReviewedAtUtc time.Time `json:"reviewed_at_utc"`
	ReviewedBy    string    `json:"reviewed_by"`
}

type MetaRunProposalReviewMutationResponse struct {
	SessionId       string                         `json:"session_id"`
	ProposalId      string                         `json:"proposal_id"`
	ReviewStatus    string                         `json:"review_status"`
	LifecycleStatus string                         `json:"lifecycle_status"`
	AlreadyReviewed bool                           `json:"already_reviewed"`
	ReviewedAtUtc   time.Time                      `json:"reviewed_at_utc"`
	Reason          string                         `json:"reason"`
	Audit           *MetaRunProposalAuditDetail    `json:"audit"`
	Workflow        *MetaRunProposalWorkflowDetail `json:"workflow"`
}

type MetaRunProposalWorkflowDetail struct {
	WorkflowId       string     `json:"workflow_id"`
	Stage            string     `json:"stage"`
	LastAction       string     `json:"last_action"`
	LastActorId      string     `json:"last_actor_id"`
	LastChangedAtUtc *time.Time `json:"last_changed_at_utc"`
	TransitionCount  int        `json:"transition_count"`
}

type MetaRunProposalAuditDetail struct {
	SchemaVersion    string     `json:"schema_version"`
	ActorId          string     `json:"actor_id"`
	ChangedAtUtc     *time.Time `json:"changed_at_utc"`
	TransitionAction string     `json:"transition_action"`
}

func DefaultMetaRunProposalAuditDetail() *MetaRunProposalAuditDetail {
	return &MetaRunProposalAuditDetail{
		SchemaVersion: "v1",
	}
}

type MetaRunProposalReviewDetail struct {
	Status        string    `json:"status"`
	ReviewedAtUtc time.Time `json:"reviewed_at_utc"`
	Reason        string    `json:"reason"`
}

type MetaRunDerivedProposalCheckpointDetail struct {
	PendingStepId       string   `json:"pending_step_id"`
	PendingStepIds      []string `json:"pending_step_ids"`
	BlockedStepIds      []string `json:"blocked_step_ids"`
	PromptPresent       bool     `json:"prompt_present"`
	OutputStepIds       []string `json:"output_step_ids"`
	FailureAliasStepIds []string `json:"failure_alias_step_ids"`
}

type MetaRunDerivedProposalEvidenceDetail struct {
	TimelineStepIds []string `json:"timeline_step_ids"`
	ErrorCode       string   `json:"error_code"`
	Error           string   `json:"error"`
	FinalText       string   `json:"final_text"`
}

type MetaRunDerivedProposalStepDetail struct {
	Id          string  `json:"id"`
	Kind        string  `json:"kind"`
	Status      string  `json:"status"`
	FailureCode string  `json:"failure_code"`
	DurationMs  float64 `json:"duration_ms"`
	Continued   bool    `json:"continued"`
}
