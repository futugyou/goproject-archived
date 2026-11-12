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

const (
	MetaRunReplayPlanSummariesAuditableNotReplayable             = "auditable_not_replayable"
	MetaRunReplayPlanSummariesMetadataOnlyNotReplayable          = "metadata_only_not_replayable"
	MetaRunReplayModesPreviewOnly                                = "preview_only"
	MetaRunReplayArtifactNamesFinalText                          = "final_text"
	MetaRunReplayArtifactNamesErrorCode                          = "error_code"
	MetaRunReplayArtifactNamesErrorMessage                       = "error_message"
	MetaRunReplayArtifactNamesStepResults                        = "step_results"
	MetaRunReplayRequirementNamesPromptContext                   = "prompt_context"
	MetaRunReplayRequirementNamesStepInputs                      = "step_inputs"
	MetaRunReplayRequirementNamesToolArguments                   = "tool_arguments"
	MetaRunReplayRequirementNamesStepResults                     = "step_results"
	MetaRunReplayRequirementNamesSkillExecInputs                 = "skill_exec_inputs"
	MetaRunReplayRequirementKindsNotPersisted                    = "not_persisted"
	MetaRunReplayRequirementKindsNotRetained                     = "not_retained"
	MetaRunReplayReasonsNotEnoughInputsForExecutableReplay       = "Persisted meta run history does not yet include enough inputs to build an executable replay plan."
	MetaRunReplayRequirementReasonsPromptContextNotPersisted     = "Persisted meta run history does not retain the originating prompt context needed to rebuild the replay request."
	MetaRunReplayRequirementReasonsStepInputsNotPersisted        = "Persisted meta run history records step outcomes but not the step-level inputs needed to re-run the graph deterministically."
	MetaRunReplayRequirementReasonsToolArgumentsNotPersisted     = "Persisted meta run history does not retain the original tool arguments required to reconstruct tool calls."
	MetaRunReplayRequirementReasonsStepResultsNotRetained        = "This run did not retain any step results, so replay preview cannot show even a step-level execution trace."
	MetaRunReplayRequirementReasonsSkillExecInputsNotPersisted   = "skill_exec_inputs_not_persisted"
	MetaRunReplayStepReadinessReasonsTraceOnly                   = "Persisted meta run history retains a step summary only; replay inputs for this step were not recorded."
	MetaRunReplayStepReadinessReasonsFailureTraceOnly            = "Persisted meta run history retains only a failed step summary; replay inputs for this step were not recorded."
	MetaRunReplayStepReadinessReasonsContinuationTraceOnly       = "Persisted meta run history retains only a continued step summary; replay inputs for this step were not recorded."
	MetaRunReplayStepReadinessReasonsFailureTraceContinued       = "Persisted meta run history retains only a failed step summary and shows execution continued after the failure; replay inputs for this step were not recorded."
	MetaRunReplayStepReadinessKindsTraceOnly                     = "trace_only"
	MetaRunReplayStepReadinessKindsFailureTraceOnly              = "failure_trace_only"
	MetaRunReplayStepReadinessKindsContinuationTraceOnly         = "continuation_trace_only"
	MetaRunReplayStepReadinessKindsFailureTraceContinued         = "failure_trace_continued"
	MetaRunReplayTriageHintCodesSkillExecInputsNotPersisted      = "skill_exec_inputs_not_persisted"
	MetaRunReplayTriageHintCodesSkillExecParseModeAnomaly        = "skill_exec_parse_mode_anomaly"
	MetaRunReplayTriageHintCodesSkillExecCommandPreviewTruncated = "skill_exec_command_preview_possibly_truncated"
	MetaRunReplayTriageHintCodesDominantFailureCluster           = "dominant_failure_cluster"
	MetaRunReplayExecutionModesAuditReconstruction               = "audit_reconstruction"
	MetaRunReplayExecutionSourcesHistoryOnly                     = "history_only"
	MetaRunReplayExecutionSourcesHistoryPlusCheckpoint           = "history_plus_checkpoint"
	MetaRunReplayTimelineSourcesRunHistory                       = "run_history"
	MetaRunReplayTimelineSourcesCheckpoint                       = "checkpoint"
	MetaRunProposalReasonsNotImplemented                         = "proposal_workflow_not_implemented"
	MetaRunProposalSourcesDerivedMetaRunEvidence                 = "derived_meta_run_evidence"
	MetaRunProposalActionsShow                                   = "show"
	MetaRunProposalActionsAccept                                 = "accept"
	MetaRunProposalActionsDismiss                                = "dismiss"
	MetaRunProposalActionsRollback                               = "rollback"
	MetaRunProposalActionsChange                                 = "change"
	MetaRunProposalReviewStatusesPending                         = "pending"
	MetaRunProposalReviewStatusesAccepted                        = "accepted"
	MetaRunProposalReviewStatusesDismissed                       = "dismissed"
	MetaRunProposalReviewStatusesRolledBack                      = "rolled_back"
	MetaRunProposalKindsPausedRunFollowup                        = "paused_run_followup"
	MetaRunProposalKindsFailedRunReview                          = "failed_run_review"
	MetaRunProposalEntrypointsMetaRuns                           = "skills meta-runs proposals"
	MetaRunProposalEntrypointsReadOnlyAlias                      = "skills proposals"
	MetaRunProposalMetadataSessionId                             = "meta_run_proposal_session_id"
	MetaRunProposalMetadataProposalId                            = "meta_run_proposal_id"
	MetaRunProposalMetadataRunId                                 = "meta_run_proposal_run_id"
	MetaRunProposalMetadataProposalStatus                        = "meta_run_proposal_status"
	MetaRunProposalMetadataProposalKind                          = "meta_run_proposal_kind"
	MetaRunProposalMetadataReason                                = "meta_run_proposal_reason"
	MetaRunProposalMetadataSource                                = "meta_run_proposal_source"
	MetaRunProposalMetadataProvenanceSnapshotVersion             = "meta_run_proposal_provenance_snapshot_version"
	MetaRunProposalMetadataProvenanceCapturedAtUtc               = "meta_run_proposal_provenance_captured_at_utc"
	MetaRunProposalMetadataProvenanceRunStatus                   = "meta_run_proposal_provenance_run_status"
	MetaRunProposalMetadataProvenanceRunStartedAtUtc             = "meta_run_proposal_provenance_run_started_at_utc"
	MetaRunProposalMetadataProvenanceRunCompletedAtUtc           = "meta_run_proposal_provenance_run_completed_at_utc"
	MetaRunProposalMetadataProvenanceStepCount                   = "meta_run_proposal_provenance_step_count"
	MetaRunProposalMetadataProvenanceStepIds                     = "meta_run_proposal_provenance_step_ids"
	MetaRunProposalMetadataProvenanceCheckpointPendingStepId     = "meta_run_proposal_provenance_checkpoint_pending_step_id"
	MetaRunProposalMetadataProvenanceCheckpointPromptPresent     = "meta_run_proposal_provenance_checkpoint_prompt_present"
	MetaRunProposalMetadataTransitionCount                       = "meta_run_proposal_transition_count"
	MetaRunProposalMetadataTransitionEntryPrefix                 = "meta_run_proposal_transition"
	MetaRunProposalMetadataTransitionFieldAction                 = "action"
	MetaRunProposalMetadataTransitionFieldFromStatus             = "from_status"
	MetaRunProposalMetadataTransitionFieldToStatus               = "to_status"
	MetaRunProposalMetadataTransitionFieldChangedAtUtc           = "changed_at_utc"
	MetaRunProposalMetadataTransitionFieldActorId                = "actor_id"
	MetaRunProposalMetadataTransitionFieldReason                 = "reason"
	MetaRunProposalMetadataAuditSchemaVersion                    = "meta_run_proposal_audit_schema_version"
	MetaRunProposalMetadataLastTransitionAction                  = "meta_run_proposal_last_transition_action"
	MetaRunProposalMetadataLastTransitionChangedAtUtc            = "meta_run_proposal_last_transition_changed_at_utc"
	MetaRunProposalMetadataLastTransitionActorId                 = "meta_run_proposal_last_transition_actor_id"
	MetaRunProposalMetadataAcceptGateProfile                     = "meta_run_proposal_accept_gate_profile"
	MetaRunProposalMetadataAcceptGatePassed                      = "meta_run_proposal_accept_gate_passed"
	MetaRunProposalMetadataAcceptGateFailedChecks                = "meta_run_proposal_accept_gate_failed_checks"
	MetaRunProposalMetadataAcceptGateCheckedAtUtc                = "meta_run_proposal_accept_gate_checked_at_utc"
	MetaRunWorkflowMetadataSessionId                             = "meta_run_workflow_session_id"
	MetaRunWorkflowMetadataProposalId                            = "meta_run_workflow_proposal_id"
	MetaRunWorkflowMetadataWorkflowId                            = "meta_run_workflow_id"
	MetaRunWorkflowMetadataStage                                 = "meta_run_workflow_stage"
	MetaRunWorkflowMetadataLastAction                            = "meta_run_workflow_last_action"
	MetaRunWorkflowMetadataLastActorId                           = "meta_run_workflow_last_actor_id"
	MetaRunWorkflowMetadataLastChangedAtUtc                      = "meta_run_workflow_last_changed_at_utc"
	MetaRunWorkflowMetadataTransitionCount                       = "meta_run_workflow_transition_count"
	MetaRunWorkflowMetadataTransitionEntryPrefix                 = "meta_run_workflow_transition"
	MetaRunWorkflowMetadataTransitionFieldAction                 = "action"
	MetaRunWorkflowMetadataTransitionFieldStage                  = "stage"
	MetaRunWorkflowMetadataTransitionFieldChangedAtUtc           = "changed_at_utc"
	MetaRunWorkflowMetadataTransitionFieldActorId                = "actor_id"
	MetaRunWorkflowMetadataTransitionFieldReason                 = "reason"
)

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
		BranchName:               branch.Name,
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
