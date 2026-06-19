package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type ProviderUsageSnapshot struct {
	ProviderID       string `json:"provider_id"`
	ModelID          string `json:"model_id"`
	Requests         int64  `json:"requests"`
	Retries          int64  `json:"retries"`
	Errors           int64  `json:"errors"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
}

type ToolUsageSnapshot struct {
	ToolName        string  `json:"tool_name"`
	Calls           int64   `json:"calls"`
	Failures        int64   `json:"failures"`
	Timeouts        int64   `json:"timeouts"`
	TotalDurationMs float64 `json:"total_duration_ms"`
}

type usageCounter struct {
	requests         atomic.Int64
	retries          atomic.Int64
	errors           atomic.Int64
	inputTokens      atomic.Int64
	outputTokens     atomic.Int64
	cacheReadTokens  atomic.Int64
	cacheWriteTokens atomic.Int64
}

func (u *usageCounter) Snapshot(providerId, modelId string) *ProviderUsageSnapshot {
	return &ProviderUsageSnapshot{
		ProviderID:       providerId,
		ModelID:          modelId,
		Requests:         u.requests.Load(),
		Retries:          u.retries.Load(),
		Errors:           u.errors.Load(),
		InputTokens:      u.inputTokens.Load(),
		OutputTokens:     u.outputTokens.Load(),
		CacheReadTokens:  u.cacheReadTokens.Load(),
		CacheWriteTokens: u.cacheWriteTokens.Load(),
	}
}

type ProviderUsageTracker struct {
	usage          sync.Map
	recentTurns    []ProviderTurnUsageEntry
	maxRecentTurns int
}

func NewProviderUsageTracker() *ProviderUsageTracker {
	return &ProviderUsageTracker{maxRecentTurns: 256, recentTurns: []ProviderTurnUsageEntry{}}
}

func (p *ProviderUsageTracker) getCounter(providerId, modelId string) *usageCounter {
	if len(providerId) == 0 {
		providerId = "unknown"
	}
	if len(modelId) == 0 {
		modelId = "default"
	}

	key := providerId + ":" + modelId

	// 先读，读到了直接返回（无内存分配）
	if u, ok := p.usage.Load(key); ok {
		return u.(*usageCounter)
	}

	// 没读到再创建
	u, _ := p.usage.LoadOrStore(key, &usageCounter{})
	return u.(*usageCounter)
}

func (p *ProviderUsageTracker) GetLatestSessionCacheTotals(sessionId string) (int64, int64) {
	slices.SortFunc(p.recentTurns, func(a, b ProviderTurnUsageEntry) int {
		return b.TimestampUtc.Compare(a.TimestampUtc)
	})

	var latest ProviderTurnUsageEntry
	for _, item := range p.recentTurns {
		if item.SessionId == sessionId && (item.CacheReadTokens > 0 || item.CacheWriteTokens > 0) {
			latest = item
			break
		}
	}

	return latest.CacheReadTokens, latest.CacheWriteTokens
}

func (p *ProviderUsageTracker) RecentTurns(sessionId string, limit int) []ProviderTurnUsageEntry {
	if limit < 1 {
		limit = 1
	}
	if limit >= p.maxRecentTurns {
		limit = p.maxRecentTurns
	}
	slices.SortFunc(p.recentTurns, func(a, b ProviderTurnUsageEntry) int {
		return b.TimestampUtc.Compare(a.TimestampUtc)
	})

	result := []ProviderTurnUsageEntry{}
	for _, item := range p.recentTurns {
		if item.SessionId == sessionId && (item.CacheReadTokens > 0 || item.CacheWriteTokens > 0) {
			if len(result) > limit {
				break
			}
			result = append(result, item)
		}
	}

	return result
}

func (p *ProviderUsageTracker) RecordTurn(sessionId, channelId, providerId, modelId string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64, estimatedInputTokensByComponent *InputTokenComponentEstimate) {
	if len(sessionId) == 0 {
		sessionId = "unknown"
	}
	if len(channelId) == 0 {
		channelId = "unknown"
	}
	if len(providerId) == 0 {
		providerId = "unknown"
	}
	if len(modelId) == 0 {
		modelId = "default"
	}
	p.recentTurns = append(p.recentTurns, ProviderTurnUsageEntry{
		SessionId:                       sessionId,
		ChannelId:                       channelId,
		ProviderId:                      providerId,
		ModelId:                         modelId,
		InputTokens:                     inputTokens,
		OutputTokens:                    outputTokens,
		CacheReadTokens:                 cacheReadTokens,
		CacheWriteTokens:                cacheWriteTokens,
		EstimatedInputTokensByComponent: estimatedInputTokensByComponent,
	})

	for {
		if len(p.recentTurns) <= p.maxRecentTurns {
			break
		}
		p.recentTurns = p.recentTurns[1:]
	}
}

func (p *ProviderUsageTracker) AddCacheTokens(providerId, modelId string, cacheReadTokens, cacheWriteTokens int64) {
	var counter = p.getCounter(providerId, modelId)
	if cacheReadTokens > 0 {
		counter.cacheReadTokens.Add(cacheReadTokens)
	}

	if cacheWriteTokens > 0 {
		counter.cacheWriteTokens.Add(cacheWriteTokens)
	}
}

func (p *ProviderUsageTracker) RecordRequest(providerId, modelId string) {
	p.getCounter(providerId, modelId).requests.Add(1)

}
func (p *ProviderUsageTracker) RecordRetry(providerId, modelId string) {
	p.getCounter(providerId, modelId).retries.Add(1)

}

func (p *ProviderUsageTracker) RecordError(providerId, modelId string) {
	p.getCounter(providerId, modelId).errors.Add(1)

}

func (p *ProviderUsageTracker) AddTokens(providerId, modelId string, inputTokens, outputTokens int64) {
	var counter = p.getCounter(providerId, modelId)
	if inputTokens > 0 {
		counter.inputTokens.Add(inputTokens)
	}
	if outputTokens > 0 {
		counter.outputTokens.Add(outputTokens)
	}
}

var _ ITurnTokenUsageObserver = (*ProviderUsageTurnTokenUsageObserver)(nil)

type ProviderUsageTurnTokenUsageObserver struct {
	providerUsage *ProviderUsageTracker
}

// RecordTurn implements [ITurnTokenUsageObserver].
func (p *ProviderUsageTurnTokenUsageObserver) RecordTurn(record TurnTokenUsageRecord) {
	p.providerUsage.RecordTurn(record.SessionId, record.ChannelId, record.ProviderId, record.ModelId, record.InputTokens, record.OutputTokens, record.CacheReadTokens, record.CacheWriteTokens, &record.EstimatedInputTokensByComponent)
}

func NewProviderUsageTurnTokenUsageObserver(providerUsage *ProviderUsageTracker) *ProviderUsageTurnTokenUsageObserver {
	return &ProviderUsageTurnTokenUsageObserver{providerUsage: providerUsage}
}

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

var _ ITurnTokenUsageObserver = (*TurnTokenUsageAuditLog)(nil)

type TurnTokenUsageAuditLog struct {
	defaultAuditQueueCapacity int
	filePath                  string
	lineQueue                 chan string
	wg                        sync.WaitGroup
	disposed                  atomic.Int32
}

func NewTurnTokenUsageAuditLog(filePath string, auditQueueCapacity int) *TurnTokenUsageAuditLog {
	if filePath == "" {
		return &TurnTokenUsageAuditLog{}
	}

	if auditQueueCapacity <= 0 {
		auditQueueCapacity = 4096
	}

	// 1. 解析并创建文件夹路径
	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("Failed to get absolute path for %s: %v; file logging will be disabled\n", filePath, err)
		return &TurnTokenUsageAuditLog{}
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create directory %s: %v; file logging will be disabled\n", dir, err)
		return &TurnTokenUsageAuditLog{}
	}

	logger := &TurnTokenUsageAuditLog{
		filePath:  fullPath,
		lineQueue: make(chan string, auditQueueCapacity),
	}

	logger.wg.Add(1)
	go logger.writeLoop()

	return logger
}

// RecordTurn implements [ITurnTokenUsageObserver].
func (l *TurnTokenUsageAuditLog) RecordTurn(record TurnTokenUsageRecord) {
	// 检查是否已被释放/关闭
	if l.disposed.Load() != 0 {
		return
	}

	// 如果未初始化成功（路径为空），则直接跳过
	if l.filePath == "" || l.lineQueue == nil {
		return
	}

	// 序列化
	jsonData, err := json.Marshal(record)
	if err != nil {
		log.Printf("Failed to serialize turn token usage record: %v\n", err)
		return
	}

	// 为防止关闭 channel 时的 panic 风险，配合 atomic 安全写入
	defer func() {
		if recover() != nil {
			// 捕获可能在极罕见并发下向已关闭 channel 发送数据引发的 panic
		}
	}()

	if l.disposed.Load() == 0 {
		l.lineQueue <- string(jsonData)
	}
}

func (l *TurnTokenUsageAuditLog) Close() {
	// 使用原子操作确保只执行一次关闭
	if !l.disposed.CompareAndSwap(0, 1) {
		return
	}

	if l.lineQueue == nil {
		return
	}

	// 关闭 channel，通知 writeLoop 结束
	close(l.lineQueue)

	l.wg.Wait()
}

// writeLoop 后台写文件循环
func (l *TurnTokenUsageAuditLog) writeLoop() {
	defer l.wg.Done()

	// 以 Append 模式打开文件
	file, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open turn token usage audit file %s: %v\n", l.filePath, err)
		// 即使文件打开失败，也要排空 channel，防止 RecordTurn 端阻塞
		for range l.lineQueue {
		}
		return
	}
	defer file.Close()

	// Go 的 range channel 会持续消费，直到 channel 被 close 且数据被读完
	for line := range l.lineQueue {
		_, err := fmt.Fprintln(file, line)
		if err != nil {
			log.Printf("Failed to append turn token usage entry to %s: %v\n", l.filePath, err)
		}
	}
}

var _ ITurnTokenUsageObserver = (*CompositeTurnTokenUsageObserver)(nil)

type CompositeTurnTokenUsageObserver struct {
	observers []ITurnTokenUsageObserver
}

// RecordTurn implements [ITurnTokenUsageObserver].
func (c *CompositeTurnTokenUsageObserver) RecordTurn(record TurnTokenUsageRecord) {
	for _, observer := range c.observers {
		observer.RecordTurn(record)
	}
}
