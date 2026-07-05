package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	RunId                      string    `json:"run_id"`
	Objective                  string    `json:"objective"`
	StartedAtUtc               time.Time `json:"started_at_utc"`
	LastContinuedAtUtc         time.Time `json:"last_continued_at_utc"`
	LastNotificationAtUtc      time.Time `json:"last_notification_at_utc"`
	ContinuationCount          int       `json:"continuation_count"`
	ContinuationSequence       int       `json:"continuation_sequence"`
	ConsecutiveNoProgressCount int       `json:"consecutive_no_progress_count"`
	ToolCallCount              int64     `json:"tool_call_count"`
	TokenBudget                int64     `json:"token_budget"`
	MaxContinuationTurns       int       `json:"max_continuation_turns"`
	LastCheckpointId           string    `json:"last_checkpoint_id"`
	LastStopReason             string    `json:"last_stop_reason"`
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
	ModelOverride                *string                         `json:"model_override,omitempty"`
	ModelProfileId               *string                         `json:"model_profile_id,omitempty"`
	PreferredModelTags           []string                        `json:"preferred_model_tags" gorm:"type:text[];not null;default:'{}'"`
	FallbackModelProfileIds      []string                        `json:"fallback_model_profile_ids" gorm:"type:text[];not null;default:'{}'"`
	ModelRequirements            ModelSelectionRequirements      `json:"model_requirements"`
	SystemPromptOverride         *string                         `json:"system_prompt_override,omitempty"`
	RoutePresetId                *string                         `json:"route_preset_id,omitempty"`
	RouteAllowedTools            []string                        `json:"route_allowed_tools" gorm:"type:text[];not null;default:'{}'"`
	ReasoningEffort              *string                         `json:"reasoning_effort,omitempty"`
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
		ExternalSessionId: "",
		Namespace:         "",
		OwnerKey:          "",
		BoundAtUtc:        time.Now().UTC(),
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
	CallId                 *string       `json:"call_id,omitempty"`
	ToolName               string        `json:"tool_name"`
	Arguments              string        `json:"arguments"`
	Result                 *string       `json:"result,omitempty"`
	Duration               time.Duration `json:"duration"`
	ResultStatus           *string       `json:"result_status,omitempty"`
	FailureCode            *string       `json:"failure_code,omitempty"`
	FailureMessage         *string       `json:"failure_message,omitempty"`
	NextStep               *string       `json:"next_step,omitempty"`
	GovernanceAllowed      *bool         `json:"governance_allowed,omitempty"`
	GovernanceAction       *string       `json:"governance_action,omitempty"`
	GovernanceReason       *string       `json:"governance_reason,omitempty"`
	GovernancePolicyId     *string       `json:"governance_policy_id,omitempty"`
	GovernanceRuleId       *string       `json:"governance_rule_id,omitempty"`
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
	CorrelationId          *string                     `json:"correlation_id,omitempty"`
	CreatedAtUtc           time.Time                   `json:"created_at_utc"`
	PersistedAtUtc         *time.Time                  `json:"persisted_at_utc,omitempty"`
	LastResumeAttemptAtUtc *time.Time                  `json:"last_resume_attempt_at_utc,omitempty"`
	CompletedAtUtc         *time.Time                  `json:"completed_at_utc,omitempty"`
	CompletionReason       *string                     `json:"completion_reason,omitempty"`
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
	CallId         *string `json:"call_id,omitempty"`
	ToolName       string  `json:"tool_name"`
	ResultStatus   string  `json:"result_status"`
	FailureCode    *string `json:"failure_code,omitempty"`
	DurationMs     int64   `json:"duration_ms"`
	ArgumentsBytes int     `json:"arguments_bytes"`
	ResultBytes    int     `json:"result_bytes"`
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
	ParentSessionId      *string                          `json:"parent_session_id,omitempty"`
	ParentChannelId      *string                          `json:"parent_channel_id,omitempty"`
	ParentSenderId       *string                          `json:"parent_sender_id,omitempty"`
	Profile              string                           `json:"profile"`
	RequestedTask        string                           `json:"requested_task"`
	AllowedTools         []string                         `json:"allowed_tools"`
	Depth                int                              `json:"depth"`
	StartedAtUtc         time.Time                        `json:"started_at_utc"`
	CompletedAtUtc       *time.Time                       `json:"completed_at_utc,omitempty"`
	Status               string                           `json:"status"`
	FinalResponsePreview *string                          `json:"final_response_preview,omitempty"`
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
	FinalResponsePreview *string                          `json:"final_response_preview,omitempty"`
}

func NewDefaultSessionDelegationChildSummary() SessionDelegationChildSummary {
	return SessionDelegationChildSummary{
		Profile:         "",
		TaskPreview:     "",
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
	StableSessionId        *string      `json:"stable_session_id,omitempty"`
	StableSessionNamespace *string      `json:"stable_session_namespace,omitempty"`
	StableSessionOwnerKey  *string      `json:"stable_session_owner_key,omitempty"`
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
	Search    *string       `json:"search,omitempty"`
	ChannelId *string       `json:"channel_id,omitempty"`
	SenderId  *string       `json:"sender_id,omitempty"`
	FromUtc   *time.Time    `json:"from_utc,omitempty"`
	ToUtc     *time.Time    `json:"to_utc,omitempty"`
	State     *SessionState `json:"state,omitempty"`
	Starred   *bool         `json:"starred,omitempty"`
	Tag       *string       `json:"tag,omitempty"`
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
	ChannelID     *string    `json:"channel_id"`
	SenderID      *string    `json:"sender_id"`
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

type SessionManager struct {
	active                    sync.Map
	lockLastUsed              sync.Map
	sessionLocks              sync.Map
	admissionGate             sync.Mutex
	activeCount               atomic.Int64
	store                     IMemoryStore
	timeout                   time.Duration
	metrics                   *RuntimeMetrics
	backgroundPersists        sync.Map
	backgroundPersistSequence atomic.Int64
	maxSessions               int
	disposeStarted            atomic.Int32
}

func (s *SessionManager) SweepExpiredActiveSessions() int {
	var removedCount = 0
	s.active.Range(func(key, value any) bool {
		session := value.(*Session)
		if session.LastActiveAt.Add(s.timeout).Before(time.Now().UTC()) {
			session.State = SessionStateExpired
			s.active.Delete(key)
			s.activeCount.Add(-1)
			removedCount++
			s.metrics.IncrementSessionEvictions()
			s.queueBestEffortPersist(session)
		}
		return true
	})

	return removedCount
}

func (s *SessionManager) evictLeastRecentlyActive() {
	if s.maxSessions <= 0 {
		return
	}
	var maxAttempts = s.maxSessions + 1
	var attempts = 0

	for {
		if s.activeCount.Load() < int64(s.maxSessions) {
			break
		}
		attempts++
		if attempts > maxAttempts {
			return
		}

		oldestKey := ""
		oldestAt := time.Date(9999, time.December, 31, 23, 59, 59, 999999999, time.UTC)

		s.active.Range(func(key, value any) bool {
			session := value.(*Session)
			if session.LastActiveAt.Before(oldestAt) {
				oldestAt = session.LastActiveAt
				oldestKey = key.(string)
			}
			return true
		})

		if oldestKey == "" {
			return
		}

		if actual, ok := s.active.Load(oldestKey); ok {
			s.active.Delete(oldestKey)
			session := actual.(*Session)
			session.State = SessionStateExpired
			s.activeCount.Add(-1)
			s.metrics.IncrementSessionEvictions()
			s.queueBestEffortPersist(session)
		} else {
			return
		}
	}
}

func (s *SessionManager) queueBestEffortPersist(session *Session) {
	opId := s.backgroundPersistSequence.Add(1)
	taskDone := make(chan struct{})
	s.backgroundPersists.Store(opId, taskDone)

	go func() {
		defer func() {
			close(taskDone)
			s.backgroundPersists.Delete(opId)
		}()

		s.Persist(context.Background(), session, false)
	}()
}

func (sm *SessionManager) Close() {
	if !sm.disposeStarted.CompareAndSwap(0, 1) {
		return
	}

	// 遍历所有还在后台运行的任务，并等待它们结束
	sm.backgroundPersists.Range(func(key, value any) bool {
		taskDone := value.(chan struct{})
		<-taskDone // 阻塞等待该后台任务结束
		return true
	})

	sm.DisposeSessionLocks()
}

func (s *SessionManager) ensureCapacityForAdmission() error {
	if s.maxSessions <= 0 {
		return nil
	}

	if s.activeCount.Load() >= (int64)(s.maxSessions) {
		s.SweepExpiredActiveSessions()
	}

	if s.activeCount.Load() >= (int64)(s.maxSessions) {
		s.evictLeastRecentlyActive()
	}

	if s.activeCount.Load() >= (int64)(s.maxSessions) {
		s.metrics.IncrementSessionCapacityRejects()
		return fmt.Errorf("maximum concurrent sessions limit (%d) has been reached.", s.maxSessions)
	}
	return nil
}

func (s *SessionManager) GetOrCreateById(ctx context.Context, sessionId, channelId, senderId string) (*Session, error) {
	if len(sessionId) == 0 {
		return nil, fmt.Errorf("sessionId must be set")
	}

	key := sessionId
	now := time.Now().UTC()

	// 1. 第一阶段：快路径（无锁检查 TryGetValue）
	if actual, ok := s.active.Load(key); ok {
		session := actual.(*Session)
		session.LastActiveAt = now
		return session, nil
	}

	// 2. 第二阶段：慢路径（加锁，防止缓存击穿）
	s.admissionGate.Lock()
	defer s.admissionGate.Unlock()

	// 二次检查（Double-check）：防止在等待锁期间，别的线程已经把数据放进去了
	if actual, ok := s.active.Load(key); ok {
		session := actual.(*Session)
		session.LastActiveAt = now
		return session, nil
	}

	// 3. 从底层存储加载
	session, err := s.store.GetSession(ctx, key)
	if err != nil {
		return nil, err
	}

	if session != nil {
		session.LastActiveAt = now
		session.State = SessionStateActive
		s.ensureCapacityForAdmission()

		// LoadOrStore 如果返回 loaded == true，说明在我们读库的空窗期，别人捷足先登了
		actual, loaded := s.active.LoadOrStore(key, session)
		if !loaded {
			// loaded == false，说明 TryAdd 成功！我们读到的 session 变成了正统实例
			s.activeCount.Add(1)
			return session, nil
		}

		// loaded == true，说明 TryAdd 失败，有人抢先占坑了。
		canonical := actual.(*Session)
		canonical.LastActiveAt = now
		return canonical, nil
	}

	// 4. 数据库中没有，创建新 Session
	s.ensureCapacityForAdmission()

	created := &Session{
		Id:           key,
		ChannelId:    channelId,
		SenderId:     senderId,
		LastActiveAt: now,
	}

	actual, loaded := s.active.LoadOrStore(key, created)
	if !loaded {
		// TryAdd 成功
		s.activeCount.Add(1)
		return created, nil
	}

	// TryAdd 失败，有人在我们创建期间塞进去了，用别人的
	canonical := actual.(*Session)
	canonical.LastActiveAt = now
	return canonical, nil
}

func (s *SessionManager) AcquireSessionLock(ctx context.Context, sessionId string) (*SessionLockLease, error) {
	if len(sessionId) == 0 {
		return nil, errors.New("sessionId must be set")
	}

	actual, _ := s.sessionLocks.LoadOrStore(sessionId, make(chan struct{}, 1))
	gate := actual.(chan struct{})

	select {
	case gate <- struct{}{}:
		// 成功把数据塞进去了，代表成功拿到锁
	case <-ctx.Done():
		// 如果外面取消了上下文（超时或取消），直接返回错误
		return nil, ctx.Err()
	}

	s.lockLastUsed.Store(sessionId, time.Now().UTC())

	lease := &SessionLockLease{
		owner:     s,
		sessionID: sessionId,
		gate:      gate,
	}
	return lease, nil
}

func (s *SessionManager) Persist(ctx context.Context, session *Session, sessionLockHeld bool) error {
	if session == nil {
		return errors.New("session can not be nil")
	}

	if sessionLockHeld {
		if l, err := s.AcquireSessionLock(ctx, session.Id); err == nil {
			l.Dispose()
		}
	}

	MaxRetries := 3
	delay := time.Duration(100) * time.Millisecond

	for i := 0; i <= MaxRetries; i++ {
		if err := s.store.SaveSession(ctx, *session); err != nil {
			if i < MaxRetries {
				time.Sleep(delay)
				delay *= 2
				continue
			} else {
				return err
			}
		}

		return nil
	}

	return nil
}

func (s *SessionManager) Branch(ctx context.Context, session *Session, branchName string) (string, error) {
	sessionLock, err := s.AcquireSessionLock(ctx, session.Id)
	if err != nil {
		return "", err
	}
	defer sessionLock.Dispose()

	var branchId = fmt.Sprintf("%s:branch:%s:%d", session.Id, branchName, time.Now().UTC().Unix())
	var branch = SessionBranch{
		BranchId:  branchId,
		SessionId: session.Id,
		Name:      branchName,
		History:   session.History,
	}
	return branchId, s.store.SaveBranch(ctx, branch)
}

func (s *SessionManager) RestoreBranch(ctx context.Context, session *Session, branchId string) bool {
	sessionLock, err := s.AcquireSessionLock(ctx, session.Id)
	if err != nil {
		return false
	}
	defer sessionLock.Dispose()

	branch, err := s.store.LoadBranch(ctx, branchId)
	if err != nil {
		return false
	}
	if branch.SessionId != session.Id {
		return false
	}

	session.History = branch.History
	session.LastActiveAt = time.Now().UTC()
	return true
}

func (s *SessionManager) ListBranches(ctx context.Context, sessionId string) ([]SessionBranch, error) {
	return s.store.ListBranches(ctx, sessionId)
}

func (s *SessionManager) BuildBranchDiff(ctx context.Context, session *Session, branchId string, metadata *SessionMetadataSnapshot) (*SessionDiffResponse, error) {
	sessionLock, err := s.AcquireSessionLock(ctx, session.Id)
	if err != nil {
		return nil, err
	}
	defer sessionLock.Dispose()

	branch, err := s.store.LoadBranch(ctx, branchId)
	if err != nil {
		return nil, err
	}
	if branch.SessionId != session.Id {
		return nil, errors.New("session data error")
	}

	var sharedPrefix = 0
	maxPrefix := min(len(session.History), len(branch.History))

	for {
		if sharedPrefix >= maxPrefix || !s.turnsEqual(session.History[sharedPrefix], branch.History[sharedPrefix]) {
			break
		}
		sharedPrefix++
	}

	currentOnlyTurnSummaries := []string{}
	branchOnlyTurnSummaries := []string{}
	for i := 0; i < len(session.History); i++ {
		if i < sharedPrefix {
			continue
		}
		currentOnlyTurnSummaries = append(currentOnlyTurnSummaries, s.summarizeTurn(session.History[i]))
	}
	for i := 0; i < len(branch.History); i++ {
		if i < sharedPrefix {
			continue
		}
		branchOnlyTurnSummaries = append(branchOnlyTurnSummaries, s.summarizeTurn(branch.History[i]))
	}
	return &SessionDiffResponse{
		SessionId:                session.Id,
		BranchId:                 branch.BranchId,
		BranchName:               &branch.Name,
		SharedPrefixTurns:        sharedPrefix,
		CurrentTurnCount:         len(session.History),
		BranchTurnCount:          len(branch.History),
		CurrentOnlyTurnSummaries: currentOnlyTurnSummaries,
		BranchOnlyTurnSummaries:  branchOnlyTurnSummaries,
		Metadata:                 metadata,
	}, nil
}

func (s *SessionManager) summarizeTurn(turn ChatTurn) string {
	content := strings.TrimSpace(turn.Content)
	if len(content) == 0 {
		content = turn.Role
	}

	if len(content) > 180 {
		content = content[:180] + "…"
	}

	return fmt.Sprintf("%s: %s", turn.Role, content)
}

func (s *SessionManager) turnsEqual(left ChatTurn, right ChatTurn) bool {
	if left.Role != right.Role || left.Content != right.Content {
		return false
	}

	leftCalls := left.ToolCalls
	rightCalls := right.ToolCalls

	if len(leftCalls) == 0 && len(rightCalls) == 0 {
		return true
	}

	if len(leftCalls) != len(rightCalls) {
		return false
	}

	for i := range leftCalls {
		l := leftCalls[i]
		r := rightCalls[i]

		if l.ToolName != r.ToolName || l.Arguments != r.Arguments || l.Result != r.Result {
			return false
		}
	}

	return true
}

func (s *SessionManager) ListActive(ctx context.Context) ([]Session, error) {
	sessions := []Session{}
	s.active.Range(func(key, value any) bool {
		session := value.(*Session)
		sessions = append(sessions, *session)
		return true
	})
	return sessions, nil
}

func (s *SessionManager) TryGetActive(channelId, senderId string) (*Session, error) {
	var key = fmt.Sprintf("%s:%s", channelId, senderId)
	value, ok := s.active.Load(key)
	if !ok {
		return nil, errors.New("data no exists")
	}
	return value.(*Session), nil
}

func (s *SessionManager) TryGetActiveById(sessionId string) (*Session, error) {
	sessions := []*Session{}
	s.active.Range(func(key, value any) bool {
		session := value.(*Session)
		if session.Id == sessionId {
			sessions = append(sessions, session)
			return false
		}
		return true
	})

	if len(sessions) > 0 {
		return sessions[0], nil
	}
	return nil, errors.New("data no exists")
}

func (s *SessionManager) TryGetActiveByContractId(contractId string) (*Session, error) {
	sessions := []*Session{}
	s.active.Range(func(key, value any) bool {
		session := value.(*Session)
		if session.ContractPolicy != nil && session.ContractPolicy.ID == contractId {
			sessions = append(sessions, session)
			return false
		}
		return true
	})

	if len(sessions) > 0 {
		return sessions[0], nil
	}
	return nil, errors.New("data no exists")
}

func (s *SessionManager) Load(ctx context.Context, sessionId string) (*Session, error) {
	value, ok := s.active.Load(sessionId)
	if ok {
		return value.(*Session), nil
	}

	return s.store.GetSession(ctx, sessionId)
}

func (s *SessionManager) RemoveActive(sessionId string) bool {
	if len(sessionId) == 0 {
		return false
	}

	if _, ok := s.active.Load(sessionId); ok {
		s.active.Delete(sessionId)
		s.activeCount.Add(-1)
		return true
	}

	return false
}

func (s *SessionManager) IsActive(sessionKey string) bool {
	_, ok := s.active.Load(sessionKey)
	return ok
}

func (s *SessionManager) ActiveCount() int64 {
	return s.activeCount.Load()
}

func (s *SessionManager) CleanupSessionLocksOnce(now time.Time, orphanThreshold time.Duration) {
	s.sessionLocks.Range(func(key, value any) bool {
		sessionKey := key.(string)
		ch := value.(chan struct{})
		s.lockLastUsed.Store(sessionKey, now)

		if s.IsActive(sessionKey) {
			s.lockLastUsed.Store(sessionKey, now)
			return true
		}
		var lastUsed time.Time
		if val, ok := s.lockLastUsed.Load(sessionKey); ok {
			lastUsed = val.(time.Time)
		} else {
			lastUsed = now
		}

		isOrphaned := now.Sub(lastUsed) > orphanThreshold
		if !isOrphaned {
			return true
		}

		select {
		case ch <- struct{}{}:
		default:
			return true
		}

		removed := false
		defer func() {
			if !removed {
				select {
				case <-ch:
				default:
				}
			}
		}()

		if s.IsActive(sessionKey) {
			s.lockLastUsed.Store(sessionKey, now)
			return true
		}

		if actualVal, loaded := s.sessionLocks.LoadAndDelete(sessionKey); loaded {
			removed = true
			s.lockLastUsed.Delete(sessionKey)
			close(actualVal.(chan struct{}))
		}

		return true
	})
}

func (s *SessionManager) DisposeSessionLocks() {
	s.sessionLocks.Range(func(key, value any) bool {
		sessionKey := key.(string)
		if actualVal, loaded := s.sessionLocks.LoadAndDelete(sessionKey); loaded {
			ch := actualVal.(chan struct{})
			func() {
				defer func() {
					if r := recover(); r != nil {
					}
				}()
				close(ch)
			}()
		}

		return true
	})

	s.lockLastUsed.Clear()
}

type SessionLockLease struct {
	owner     *SessionManager
	sessionID string
	gate      chan struct{}
	once      sync.Once
}

func (s *SessionLockLease) Dispose() {
	s.once.Do(func() {
		s.owner.lockLastUsed.Store(s.sessionID, time.Now().UTC())
		select {
		case <-s.gate:
		default:
		}
	})
}
