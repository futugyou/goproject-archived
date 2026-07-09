package core

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type RuntimeMetrics struct {
	// ── Counters (monotonically increasing) ───────────────────────────────
	totalRequests                  atomic.Int64
	totalLlmCalls                  atomic.Int64
	totalInputTokens               atomic.Int64
	totalOutputTokens              atomic.Int64
	totalToolCalls                 atomic.Int64
	totalToolFailures              atomic.Int64
	totalToolTimeouts              atomic.Int64
	totalLlmRetries                atomic.Int64
	totalLlmErrors                 atomic.Int64
	approvalDecisionsRecorded      atomic.Int64
	approvalDecisionsRejected      atomic.Int64
	sessionEvictions               atomic.Int64
	sessionCapacityRejects         atomic.Int64
	estimatedTokenAdmissionRejects atomic.Int64
	browserCancellationResets      atomic.Int64
	pluginBridgeAuthFailures       atomic.Int64
	pluginBridgeRestartAttempts    atomic.Int64
	pluginBridgeRestartFailures    atomic.Int64
	processStarts                  atomic.Int64
	processCompletions             atomic.Int64
	processFailures                atomic.Int64
	processKills                   atomic.Int64
	processTimeouts                atomic.Int64
	processHistoryEvictions        atomic.Int64
	sandboxLeaseCreates            atomic.Int64
	sandboxLeaseReuses             atomic.Int64
	sandboxLeaseRecoveries         atomic.Int64
	retentionSweepRuns             atomic.Int64
	retentionSweepFailures         atomic.Int64
	retentionArchivedItems         atomic.Int64
	retentionDeletedItems          atomic.Int64
	retentionSkippedProtectedSess  atomic.Int64
	operatorAuditWriteFailures     atomic.Int64
	runtimeEventWriteFailures      atomic.Int64
	sessionCacheHits               atomic.Int64
	sessionCacheMisses             atomic.Int64
	memoryRecallSearches           atomic.Int64
	memoryRecallHits               atomic.Int64
	memoryCompactions              atomic.Int64
	promptCacheReads               atomic.Int64
	promptCacheWrites              atomic.Int64
	promptCacheWarmRuns            atomic.Int64
	promptCacheWarmSkips           atomic.Int64
	promptCacheWarmFailures        atomic.Int64
	pulseRuns                      atomic.Int64
	pulseSkips                     atomic.Int64
	pulseAlerts                    atomic.Int64
	pulseOkSuppressed              atomic.Int64
	pulseErrors                    atomic.Int64

	// ── Gauges ────────────────────────────────────────────────────────────
	activeSessions             atomic.Int32
	circuitBreakerState        atomic.Int32 // 0=Closed, 1=Open, 2=HalfOpen
	retainedProcesses          atomic.Int32
	retentionLastRunAtUnixSec  atomic.Int64
	retentionLastRunDurationMs atomic.Int64
	retentionLastRunSucceeded  atomic.Int32
	pulseLastRunDurationMs     atomic.Int64
}

// ── Getters (Load) ────────────────────────────────────────────────────

func (m *RuntimeMetrics) TotalRequests() int64             { return m.totalRequests.Load() }
func (m *RuntimeMetrics) TotalLlmCalls() int64             { return m.totalLlmCalls.Load() }
func (m *RuntimeMetrics) TotalInputTokens() int64          { return m.totalInputTokens.Load() }
func (m *RuntimeMetrics) TotalOutputTokens() int64         { return m.totalOutputTokens.Load() }
func (m *RuntimeMetrics) TotalToolCalls() int64            { return m.totalToolCalls.Load() }
func (m *RuntimeMetrics) TotalToolFailures() int64         { return m.totalToolFailures.Load() }
func (m *RuntimeMetrics) TotalToolTimeouts() int64         { return m.totalToolTimeouts.Load() }
func (m *RuntimeMetrics) TotalLlmRetries() int64           { return m.totalLlmRetries.Load() }
func (m *RuntimeMetrics) TotalLlmErrors() int64            { return m.totalLlmErrors.Load() }
func (m *RuntimeMetrics) ApprovalDecisionsRecorded() int64 { return m.approvalDecisionsRecorded.Load() }
func (m *RuntimeMetrics) ApprovalDecisionsRejected() int64 { return m.approvalDecisionsRejected.Load() }
func (m *RuntimeMetrics) SessionEvictions() int64          { return m.sessionEvictions.Load() }
func (m *RuntimeMetrics) SessionCapacityRejects() int64    { return m.sessionCapacityRejects.Load() }
func (m *RuntimeMetrics) EstimatedTokenAdmissionRejects() int64 {
	return m.estimatedTokenAdmissionRejects.Load()
}
func (m *RuntimeMetrics) BrowserCancellationResets() int64 { return m.browserCancellationResets.Load() }
func (m *RuntimeMetrics) PluginBridgeAuthFailures() int64  { return m.pluginBridgeAuthFailures.Load() }
func (m *RuntimeMetrics) PluginBridgeRestartAttempts() int64 {
	return m.pluginBridgeRestartAttempts.Load()
}
func (m *RuntimeMetrics) PluginBridgeRestartFailures() int64 {
	return m.pluginBridgeRestartFailures.Load()
}
func (m *RuntimeMetrics) ProcessStarts() int64           { return m.processStarts.Load() }
func (m *RuntimeMetrics) ProcessCompletions() int64      { return m.processCompletions.Load() }
func (m *RuntimeMetrics) ProcessFailures() int64         { return m.processFailures.Load() }
func (m *RuntimeMetrics) ProcessKills() int64            { return m.processKills.Load() }
func (m *RuntimeMetrics) ProcessTimeouts() int64         { return m.processTimeouts.Load() }
func (m *RuntimeMetrics) ProcessHistoryEvictions() int64 { return m.processHistoryEvictions.Load() }
func (m *RuntimeMetrics) SandboxLeaseCreates() int64     { return m.sandboxLeaseCreates.Load() }
func (m *RuntimeMetrics) SandboxLeaseReuses() int64      { return m.sandboxLeaseReuses.Load() }
func (m *RuntimeMetrics) SandboxLeaseRecoveries() int64  { return m.sandboxLeaseRecoveries.Load() }
func (m *RuntimeMetrics) RetentionSweepRuns() int64      { return m.retentionSweepRuns.Load() }
func (m *RuntimeMetrics) RetentionSweepFailures() int64  { return m.retentionSweepFailures.Load() }
func (m *RuntimeMetrics) RetentionArchivedItems() int64  { return m.retentionArchivedItems.Load() }
func (m *RuntimeMetrics) RetentionDeletedItems() int64   { return m.retentionDeletedItems.Load() }
func (m *RuntimeMetrics) RetentionSkippedProtectedSessions() int64 {
	return m.retentionSkippedProtectedSess.Load()
}
func (m *RuntimeMetrics) OperatorAuditWriteFailures() int64 {
	return m.operatorAuditWriteFailures.Load()
}
func (m *RuntimeMetrics) RuntimeEventWriteFailures() int64 { return m.runtimeEventWriteFailures.Load() }
func (m *RuntimeMetrics) SessionCacheHits() int64          { return m.sessionCacheHits.Load() }
func (m *RuntimeMetrics) SessionCacheMisses() int64        { return m.sessionCacheMisses.Load() }
func (m *RuntimeMetrics) MemoryRecallSearches() int64      { return m.memoryRecallSearches.Load() }
func (m *RuntimeMetrics) MemoryRecallHits() int64          { return m.memoryRecallHits.Load() }
func (m *RuntimeMetrics) MemoryCompactions() int64         { return m.memoryCompactions.Load() }
func (m *RuntimeMetrics) PromptCacheReads() int64          { return m.promptCacheReads.Load() }
func (m *RuntimeMetrics) PromptCacheWrites() int64         { return m.promptCacheWrites.Load() }
func (m *RuntimeMetrics) PromptCacheWarmRuns() int64       { return m.promptCacheWarmRuns.Load() }
func (m *RuntimeMetrics) PromptCacheWarmSkips() int64      { return m.promptCacheWarmSkips.Load() }
func (m *RuntimeMetrics) PromptCacheWarmFailures() int64   { return m.promptCacheWarmFailures.Load() }
func (m *RuntimeMetrics) PulseRuns() int64                 { return m.pulseRuns.Load() }
func (m *RuntimeMetrics) PulseSkips() int64                { return m.pulseSkips.Load() }
func (m *RuntimeMetrics) PulseAlerts() int64               { return m.pulseAlerts.Load() }
func (m *RuntimeMetrics) PulseOkSuppressed() int64         { return m.pulseOkSuppressed.Load() }
func (m *RuntimeMetrics) PulseErrors() int64               { return m.pulseErrors.Load() }

func (m *RuntimeMetrics) ActiveSessions() int32      { return m.activeSessions.Load() }
func (m *RuntimeMetrics) CircuitBreakerState() int32 { return m.circuitBreakerState.Load() }
func (m *RuntimeMetrics) RetainedProcesses() int32   { return m.retainedProcesses.Load() }
func (m *RuntimeMetrics) RetentionLastRunAtUnixSeconds() int64 {
	return m.retentionLastRunAtUnixSec.Load()
}
func (m *RuntimeMetrics) RetentionLastRunDurationMs() int64 {
	return m.retentionLastRunDurationMs.Load()
}
func (m *RuntimeMetrics) RetentionLastRunSucceeded() int32 { return m.retentionLastRunSucceeded.Load() }
func (m *RuntimeMetrics) PulseLastRunDurationMs() int64    { return m.pulseLastRunDurationMs.Load() }

// ── Mutators ──────────────────────────────────────────────────────────

func (m *RuntimeMetrics) IncrementRequests()      { m.totalRequests.Add(1) }
func (m *RuntimeMetrics) IncrementLlmCalls()      { m.totalLlmCalls.Add(1) }
func (m *RuntimeMetrics) AddInputTokens(n int64)  { m.totalInputTokens.Add(n) }
func (m *RuntimeMetrics) AddOutputTokens(n int64) { m.totalOutputTokens.Add(n) }
func (m *RuntimeMetrics) IncrementToolCalls()     { m.totalToolCalls.Add(1) }
func (m *RuntimeMetrics) IncrementToolFailures()  { m.totalToolFailures.Add(1) }
func (m *RuntimeMetrics) IncrementToolTimeouts()  { m.totalToolTimeouts.Add(1) }
func (m *RuntimeMetrics) IncrementLlmRetries()    { m.totalLlmRetries.Add(1) }
func (m *RuntimeMetrics) IncrementLlmErrors()     { m.totalLlmErrors.Add(1) }
func (m *RuntimeMetrics) IncrementApprovalDecisionsRecorded() {
	m.approvalDecisionsRecorded.Add(1)
}
func (m *RuntimeMetrics) IncrementApprovalDecisionsRejected() {
	m.approvalDecisionsRejected.Add(1)
}
func (m *RuntimeMetrics) IncrementSessionEvictions()       { m.sessionEvictions.Add(1) }
func (m *RuntimeMetrics) IncrementSessionCapacityRejects() { m.sessionCapacityRejects.Add(1) }
func (m *RuntimeMetrics) IncrementEstimatedTokenAdmissionRejects() {
	m.estimatedTokenAdmissionRejects.Add(1)
}
func (m *RuntimeMetrics) IncrementBrowserCancellationResets() { m.browserCancellationResets.Add(1) }
func (m *RuntimeMetrics) IncrementPluginBridgeAuthFailures()  { m.pluginBridgeAuthFailures.Add(1) }
func (m *RuntimeMetrics) IncrementPluginBridgeRestartAttempts() {
	m.pluginBridgeRestartAttempts.Add(1)
}
func (m *RuntimeMetrics) IncrementPluginBridgeRestartFailures() {
	m.pluginBridgeRestartFailures.Add(1)
}
func (m *RuntimeMetrics) IncrementProcessStarts()           { m.processStarts.Add(1) }
func (m *RuntimeMetrics) IncrementProcessCompletions()      { m.processCompletions.Add(1) }
func (m *RuntimeMetrics) IncrementProcessFailures()         { m.processFailures.Add(1) }
func (m *RuntimeMetrics) IncrementProcessKills()            { m.processKills.Add(1) }
func (m *RuntimeMetrics) IncrementProcessTimeouts()         { m.processTimeouts.Add(1) }
func (m *RuntimeMetrics) IncrementProcessHistoryEvictions() { m.processHistoryEvictions.Add(1) }
func (m *RuntimeMetrics) IncrementSandboxLeaseCreates()     { m.sandboxLeaseCreates.Add(1) }
func (m *RuntimeMetrics) IncrementSandboxLeaseReuses()      { m.sandboxLeaseReuses.Add(1) }
func (m *RuntimeMetrics) IncrementSandboxLeaseRecoveries()  { m.sandboxLeaseRecoveries.Add(1) }
func (m *RuntimeMetrics) IncrementRetentionSweepRuns()      { m.retentionSweepRuns.Add(1) }
func (m *RuntimeMetrics) IncrementRetentionSweepFailures()  { m.retentionSweepFailures.Add(1) }
func (m *RuntimeMetrics) AddRetentionArchivedItems(n int64) { m.retentionArchivedItems.Add(n) }
func (m *RuntimeMetrics) AddRetentionDeletedItems(n int64)  { m.retentionDeletedItems.Add(n) }
func (m *RuntimeMetrics) AddRetentionSkippedProtectedSessions(n int64) {
	m.retentionSkippedProtectedSess.Add(n)
}
func (m *RuntimeMetrics) IncrementOperatorAuditWriteFailures() { m.operatorAuditWriteFailures.Add(1) }
func (m *RuntimeMetrics) IncrementRuntimeEventWriteFailures()  { m.runtimeEventWriteFailures.Add(1) }
func (m *RuntimeMetrics) IncrementSessionCacheHits()           { m.sessionCacheHits.Add(1) }
func (m *RuntimeMetrics) IncrementSessionCacheMisses()         { m.sessionCacheMisses.Add(1) }
func (m *RuntimeMetrics) IncrementMemoryRecallSearches()       { m.memoryRecallSearches.Add(1) }
func (m *RuntimeMetrics) AddMemoryRecallHits(n int64)          { m.memoryRecallHits.Add(n) }
func (m *RuntimeMetrics) IncrementMemoryCompactions()          { m.memoryCompactions.Add(1) }
func (m *RuntimeMetrics) AddPromptCacheReads(n int64)          { m.promptCacheReads.Add(n) }
func (m *RuntimeMetrics) AddPromptCacheWrites(n int64)         { m.promptCacheWrites.Add(n) }
func (m *RuntimeMetrics) IncrementPromptCacheWarmRuns()        { m.promptCacheWarmRuns.Add(1) }
func (m *RuntimeMetrics) IncrementPromptCacheWarmSkips()       { m.promptCacheWarmSkips.Add(1) }
func (m *RuntimeMetrics) IncrementPromptCacheWarmFailures()    { m.promptCacheWarmFailures.Add(1) }
func (m *RuntimeMetrics) IncrementPulseRuns()                  { m.pulseRuns.Add(1) }
func (m *RuntimeMetrics) IncrementPulseSkips()                 { m.pulseSkips.Add(1) }
func (m *RuntimeMetrics) IncrementPulseAlerts()                { m.pulseAlerts.Add(1) }
func (m *RuntimeMetrics) IncrementPulseOkSuppressed()          { m.pulseOkSuppressed.Add(1) }
func (m *RuntimeMetrics) IncrementPulseErrors()                { m.pulseErrors.Add(1) }

func (m *RuntimeMetrics) SetPulseLastRunDuration(durationMs int64) {
	if durationMs < 0 {
		durationMs = 0
	}
	m.pulseLastRunDurationMs.Store(durationMs)
}

func (m *RuntimeMetrics) SetActiveSessions(count int32) {
	m.activeSessions.Store(count)
}

func (m *RuntimeMetrics) SetCircuitBreakerState(state int32) {
	m.circuitBreakerState.Store(state)
}

func (m *RuntimeMetrics) SetRetainedProcesses(count int32) {
	m.retainedProcesses.Store(count)
}

func (m *RuntimeMetrics) SetRetentionLastRun(runAtUtc time.Time, durationMs int64, succeeded bool) {
	m.retentionLastRunAtUnixSec.Store(runAtUtc.Unix())
	m.retentionLastRunDurationMs.Store(durationMs)
	var succ int32
	if succeeded {
		succ = 1
	}
	m.retentionLastRunSucceeded.Store(succ)
}

// Snapshot 返回当前指标的快照用于 JSON 序列化。
// 返回值类型保证了高并发下的零内存分配（Allocation-free）。
func (m *RuntimeMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		TotalRequests:                     m.TotalRequests(),
		TotalLlmCalls:                     m.TotalLlmCalls(),
		TotalInputTokens:                  m.TotalInputTokens(),
		TotalOutputTokens:                 m.TotalOutputTokens(),
		TotalToolCalls:                    m.TotalToolCalls(),
		TotalToolFailures:                 m.TotalToolFailures(),
		TotalToolTimeouts:                 m.TotalToolTimeouts(),
		TotalLlmRetries:                   m.TotalLlmRetries(),
		TotalLlmErrors:                    m.TotalLlmErrors(),
		ApprovalDecisionsRecorded:         m.ApprovalDecisionsRecorded(),
		ApprovalDecisionsRejected:         m.ApprovalDecisionsRejected(),
		SessionEvictions:                  m.SessionEvictions(),
		SessionCapacityRejects:            m.SessionCapacityRejects(),
		EstimatedTokenAdmissionRejects:    m.EstimatedTokenAdmissionRejects(),
		BrowserCancellationResets:         m.BrowserCancellationResets(),
		PluginBridgeAuthFailures:          m.PluginBridgeAuthFailures(),
		PluginBridgeRestartAttempts:       m.PluginBridgeRestartAttempts(),
		PluginBridgeRestartFailures:       m.PluginBridgeRestartFailures(),
		ProcessStarts:                     m.ProcessStarts(),
		ProcessCompletions:                m.ProcessCompletions(),
		ProcessFailures:                   m.ProcessFailures(),
		ProcessKills:                      m.ProcessKills(),
		ProcessTimeouts:                   m.ProcessTimeouts(),
		ProcessHistoryEvictions:           m.ProcessHistoryEvictions(),
		SandboxLeaseCreates:               m.SandboxLeaseCreates(),
		SandboxLeaseReuses:                m.SandboxLeaseReuses(),
		SandboxLeaseRecoveries:            m.SandboxLeaseRecoveries(),
		RetentionSweepRuns:                m.RetentionSweepRuns(),
		RetentionSweepFailures:            m.RetentionSweepFailures(),
		RetentionArchivedItems:            m.RetentionArchivedItems(),
		RetentionDeletedItems:             m.RetentionDeletedItems(),
		RetentionSkippedProtectedSessions: m.RetentionSkippedProtectedSessions(),
		OperatorAuditWriteFailures:        m.OperatorAuditWriteFailures(),
		RuntimeEventWriteFailures:         m.RuntimeEventWriteFailures(),
		SessionCacheHits:                  m.SessionCacheHits(),
		SessionCacheMisses:                m.SessionCacheMisses(),
		MemoryRecallSearches:              m.MemoryRecallSearches(),
		MemoryRecallHits:                  m.MemoryRecallHits(),
		MemoryCompactions:                 m.MemoryCompactions(),
		PromptCacheReads:                  m.PromptCacheReads(),
		PromptCacheWrites:                 m.PromptCacheWrites(),
		PromptCacheWarmRuns:               m.PromptCacheWarmRuns(),
		PromptCacheWarmSkips:              m.PromptCacheWarmSkips(),
		PromptCacheWarmFailures:           m.PromptCacheWarmFailures(),
		PulseRuns:                         m.PulseRuns(),
		PulseSkips:                        m.PulseSkips(),
		PulseAlerts:                       m.PulseAlerts(),
		PulseOkSuppressed:                 m.PulseOkSuppressed(),
		PulseErrors:                       m.PulseErrors(),
		RetentionLastRunAtUnixSeconds:     m.RetentionLastRunAtUnixSeconds(),
		RetentionLastRunDurationMs:        m.RetentionLastRunDurationMs(),
		RetentionLastRunSucceeded:         m.RetentionLastRunSucceeded(),
		PulseLastRunDurationMs:            m.PulseLastRunDurationMs(),
		ActiveSessions:                    m.ActiveSessions(),
		CircuitBreakerState:               m.CircuitBreakerState(),
		RetainedProcesses:                 m.RetainedProcesses(),
	}
}

var (
	Tracer                trace.Tracer
	Meter                 metric.Meter
	ToolExecutionDuration metric.Float64Histogram
	RateLimitExceeded     metric.Int64Counter
)

func init() {
	// 初始化 Tracer 和 Meter
	Tracer = otel.Tracer("OpenClaw.Gateway", trace.WithInstrumentationVersion("1.0.0"))
	Meter = otel.Meter("OpenClaw.Gateway", metric.WithInstrumentationVersion("1.0.0"))

	var err error

	// 初始化直方图 (Histogram)
	ToolExecutionDuration, err = Meter.Float64Histogram(
		"openclaw.tool.execution.duration",
		metric.WithDescription("Duration of tool executions in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		// Go 的最佳实践：通常在启动初始化失败时 panic，或者记录日志
		panic(err)
	}

	// 初始化计数器 (Counter)
	RateLimitExceeded, err = Meter.Int64Counter(
		"openclaw.ratelimit.exceeded",
		metric.WithDescription("Number of requests blocked by rate limiting"),
	)
	if err != nil {
		panic(err)
	}
}

// RegisterApprovalQueueGauge 注册一个观察指针（Gauge），用于报告当前审批队列的深度。
// 在启动时、ToolApprovalService 构造完成后调用一次。
func RegisterApprovalQueueGauge(observeFunc func() int) error {
	_, err := Meter.Int64ObservableGauge(
		"openclaw.approval.queue.depth",
		metric.WithDescription("Number of pending tool approval requests"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			// 调用传入的闭包函数获取最新的队列深度并上报
			obs.Observe(int64(observeFunc()))
			return nil
		}),
	)
	return err
}

type MetricsSnapshot struct {
	TotalRequests                     int64 `json:"total_requests"`
	TotalLlmCalls                     int64 `json:"total_llm_calls"`
	TotalInputTokens                  int64 `json:"total_input_tokens"`
	TotalOutputTokens                 int64 `json:"total_output_tokens"`
	TotalToolCalls                    int64 `json:"total_tool_calls"`
	TotalToolFailures                 int64 `json:"total_tool_failures"`
	TotalToolTimeouts                 int64 `json:"total_tool_timeouts"`
	TotalLlmRetries                   int64 `json:"total_llm_retries"`
	TotalLlmErrors                    int64 `json:"total_llm_errors"`
	ApprovalDecisionsRecorded         int64 `json:"approval_decisions_recorded"`
	ApprovalDecisionsRejected         int64 `json:"approval_decisions_rejected"`
	SessionEvictions                  int64 `json:"session_evictions"`
	SessionCapacityRejects            int64 `json:"session_capacity_rejects"`
	EstimatedTokenAdmissionRejects    int64 `json:"estimated_token_admission_rejects"`
	BrowserCancellationResets         int64 `json:"browser_cancellation_resets"`
	PluginBridgeAuthFailures          int64 `json:"plugin_bridge_auth_failures"`
	PluginBridgeRestartAttempts       int64 `json:"plugin_bridge_restart_attempts"`
	PluginBridgeRestartFailures       int64 `json:"plugin_bridge_restart_failures"`
	ProcessStarts                     int64 `json:"process_starts"`
	ProcessCompletions                int64 `json:"process_completions"`
	ProcessFailures                   int64 `json:"process_failures"`
	ProcessKills                      int64 `json:"process_kills"`
	ProcessTimeouts                   int64 `json:"process_timeouts"`
	ProcessHistoryEvictions           int64 `json:"process_history_evictions"`
	SandboxLeaseCreates               int64 `json:"sandbox_lease_creates"`
	SandboxLeaseReuses                int64 `json:"sandbox_lease_reuses"`
	SandboxLeaseRecoveries            int64 `json:"sandbox_lease_recoveries"`
	RetentionSweepRuns                int64 `json:"retention_sweep_runs"`
	RetentionSweepFailures            int64 `json:"retention_sweep_failures"`
	RetentionArchivedItems            int64 `json:"retention_archived_items"`
	RetentionDeletedItems             int64 `json:"retention_deleted_items"`
	RetentionSkippedProtectedSessions int64 `json:"retention_skipped_protected_sessions"`
	OperatorAuditWriteFailures        int64 `json:"operator_audit_write_failures"`
	RuntimeEventWriteFailures         int64 `json:"runtime_event_write_failures"`
	SessionCacheHits                  int64 `json:"session_cache_hits"`
	SessionCacheMisses                int64 `json:"session_cache_misses"`
	MemoryRecallSearches              int64 `json:"memory_recall_searches"`
	MemoryRecallHits                  int64 `json:"memory_recall_hits"`
	MemoryCompactions                 int64 `json:"memory_compactions"`
	PromptCacheReads                  int64 `json:"prompt_cache_reads"`
	PromptCacheWrites                 int64 `json:"prompt_cache_writes"`
	PromptCacheWarmRuns               int64 `json:"prompt_cache_warm_runs"`
	PromptCacheWarmSkips              int64 `json:"prompt_cache_warm_skips"`
	PromptCacheWarmFailures           int64 `json:"prompt_cache_warm_failures"`
	PulseRuns                         int64 `json:"pulse_runs"`
	PulseSkips                        int64 `json:"pulse_skips"`
	PulseAlerts                       int64 `json:"pulse_alerts"`
	PulseOkSuppressed                 int64 `json:"pulse_ok_suppressed"`
	PulseErrors                       int64 `json:"pulse_errors"`
	RetentionLastRunAtUnixSeconds     int64 `json:"retention_last_run_at_unix_seconds"`
	RetentionLastRunDurationMs        int64 `json:"retention_last_run_duration_ms"`
	RetentionLastRunSucceeded         int32 `json:"retention_last_run_succeeded"`
	PulseLastRunDurationMs            int64 `json:"pulse_last_run_duration_ms"`
	ActiveSessions                    int32 `json:"active_sessions"`
	CircuitBreakerState               int32 `json:"circuit_breaker_state"`
	RetainedProcesses                 int32 `json:"retained_processes"`
}
