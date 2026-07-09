package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/futugyou/extensions_ai/abstractions"
)

type TurnContext struct {
	CorrelationID string `json:"correlation_id"`
	SessionID     string `json:"session_id"`
	ChannelID     string `json:"channel_id"`

	// ── LLM metrics (内部私有字段，全小写，避免外部误操作) ──
	llmCallCount         atomic.Int32
	totalInputTokens     atomic.Int64
	totalOutputTokens    atomic.Int64
	totalLlmLatencyTicks atomic.Int64
	retryCount           atomic.Int32

	// ── Tool metrics ──
	toolCallCount          atomic.Int32
	totalToolDurationTicks atomic.Int64
	toolFailureCount       atomic.Int32
	toolTimeoutCount       atomic.Int32
}

// NewTurnContext 接收 Go 标准的 context.Context，用于提取 TraceID
func NewTurnContext(ctx context.Context, sessionID, channelID string) *TurnContext {
	var correlationID string

	// 1. 从 Go 标准的 Context 中获取 OpenTelemetry 的 SpanContext
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		// 拿到标准的 32 位 Hex TraceID 字符串
		fullTraceID := spanContext.TraceID().String()
		if len(fullTraceID) >= 16 {
			correlationID = fullTraceID[:16]
		} else {
			correlationID = fullTraceID
		}
	}

	// 2. 兜底策略：如果 Context 里没有合法的 TraceID，生成 16 位随机 Hex 字符串
	if correlationID == "" {
		bytes := make([]byte, 8) // 8字节 = 16位 hex 字符
		if _, err := rand.Read(bytes); err == nil {
			correlationID = hex.EncodeToString(bytes)
		} else {
			correlationID = "fallback_trace_id"
		}
	}

	return &TurnContext{
		CorrelationID: correlationID,
		SessionID:     sessionID,
		ChannelID:     channelID,
	}
}

// ── LLM Getters ──

func (c *TurnContext) LlmCallCount() int32 {
	return c.llmCallCount.Load()
}

func (c *TurnContext) TotalInputTokens() int64 {
	return c.totalInputTokens.Load()
}

func (c *TurnContext) TotalOutputTokens() int64 {
	return c.totalOutputTokens.Load()
}

func (c *TurnContext) TotalLlmLatency() time.Duration {
	// time.Duration 底层就是 int64 纳秒数，直接转换即可
	return time.Duration(c.totalLlmLatencyTicks.Load())
}

func (c *TurnContext) RetryCount() int32 {
	return c.retryCount.Load()
}

// ── Tool Getters ──

func (c *TurnContext) ToolCallCount() int32 {
	return c.toolCallCount.Load()
}

func (c *TurnContext) TotalToolDuration() time.Duration {
	return time.Duration(c.totalToolDurationTicks.Load())
}

func (c *TurnContext) ToolFailureCount() int32 {
	return c.toolFailureCount.Load()
}

func (c *TurnContext) ToolTimeoutCount() int32 {
	return c.toolTimeoutCount.Load()
}

// ── Recorder Methods ──

func (c *TurnContext) RecordLlmCall(latency time.Duration, inputTokens, outputTokens int64) {
	c.llmCallCount.Add(1)
	c.totalLlmLatencyTicks.Add(int64(latency))
	c.totalInputTokens.Add(inputTokens)
	c.totalOutputTokens.Add(outputTokens)
}

func (c *TurnContext) RecordRetry() {
	c.retryCount.Add(1)
}

func (c *TurnContext) RecordToolCall(duration time.Duration, failed, timedOut bool) {
	c.toolCallCount.Add(1)
	c.totalToolDurationTicks.Add(int64(duration))
	if failed {
		c.toolFailureCount.Add(1)
	}
	if timedOut {
		c.toolTimeoutCount.Add(1)
	}
}

func (c *TurnContext) String() string {
	return fmt.Sprintf(
		"Turn[%s] session=%s llm=%d retries=%d tokens=%din/%dout tools=%d toolFails=%d toolTimeouts=%d llmLatency=%dms toolDuration=%dms",
		c.CorrelationID,
		c.SessionID,
		c.LlmCallCount(),
		c.RetryCount(),
		c.TotalInputTokens(),
		c.TotalOutputTokens(),
		c.ToolCallCount(),
		c.ToolFailureCount(),
		c.ToolTimeoutCount(),
		c.TotalLlmLatency().Milliseconds(),
		c.TotalToolDuration().Milliseconds(),
	)
}

type TurnTokenUsageRecord struct {
	SessionId                       string                      `json:"session_id"`
	ChannelId                       string                      `json:"channel_id"`
	ProviderId                      string                      `json:"provider_id"`
	ModelId                         string                      `json:"model_id"`
	InputTokens                     int64                       `json:"input_tokens"`
	OutputTokens                    int64                       `json:"output_tokens"`
	CacheReadTokens                 int64                       `json:"cache_read_tokens"`
	CacheWriteTokens                int64                       `json:"cache_write_tokens"`
	EstimatedInputTokensByComponent InputTokenComponentEstimate `json:"estimated_input_tokens_by_component"`
	IsEstimated                     bool                        `json:"is_estimated"`
	TimestampUtc                    time.Time                   `json:"timestamp_utc"`
}

// DefaultTurnTokenUsageRecord 创建一个带有默认 UTC 时间的结构体实例
func DefaultTurnTokenUsageRecord() *TurnTokenUsageRecord {
	return &TurnTokenUsageRecord{
		TimestampUtc: time.Now().UTC(),
	}
}

type PromptCacheUsage struct {
	CacheReadTokens  int64
	CacheWriteTokens int64
}

var promptCacheCacheWriteKeys = []string{
	"cache_write_tokens",
	"cacheWriteTokens",
	"cache_creation_input_tokens",
	"cacheCreationInputTokens",
}

type PromptCacheUsageExtractor struct {
}

func (p *PromptCacheUsageExtractor) FromUsage(usage *abstractions.UsageDetails) *PromptCacheUsage {
	if usage == nil {
		return &PromptCacheUsage{}
	}

	var cacheRead = usage.CachedInputTokenCount
	var cacheWrite int64 = 0
	if usage.AdditionalCounts != nil {
		for _, key := range promptCacheCacheWriteKeys {
			if value, ok := usage.AdditionalCounts[key]; ok {
				cacheWrite = value
				break
			}
		}
	}

	return &PromptCacheUsage{
		CacheReadTokens:  *cacheRead,
		CacheWriteTokens: cacheWrite,
	}
}

func (p *PromptCacheUsageExtractor) Merge(items []PromptCacheUsage) *PromptCacheUsage {

	var cacheRead int64 = 0
	var cacheWrite int64 = 0
	for _, item := range items {
		cacheRead += item.CacheReadTokens
		cacheWrite += item.CacheWriteTokens
	}

	return &PromptCacheUsage{
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
	}
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

type ToolAuditEntry struct {
	TimestampUtc           time.Time `json:"timestamp_utc"`
	ToolName               string    `json:"tool_name"`
	SessionId              string    `json:"session_id"`
	ChannelId              string    `json:"channel_id"`
	SenderId               string    `json:"sender_id"`
	CorrelationId          string    `json:"crrelation_id"`
	DurationMs             float64   `json:"duration_ms"`
	Failed                 bool      `json:"failed"`
	TimedOut               bool      `json:"timed_out"`
	ApprovalId             string    `json:"approval_id"`
	ArgumentsBytes         int       `json:"arguments_bytes"`
	ResultBytes            int       `json:"result_bytes"`
	GovernanceAllowed      bool      `json:"governance_allowed"`
	GovernanceAction       string    `json:"governance_action"`
	GovernanceReason       string    `json:"governance_reason"`
	GovernancePolicyId     string    `json:"governance_policy_id"`
	GovernanceRuleId       string    `json:"governance_rule_id"`
	GovernanceTrustScore   float64   `json:"governance_trust_score"`
	GovernanceEvaluationMs float64   `json:"governance_evaluation_ms"`
	GovernanceUnavailable  bool      `json:"governance_unavailable"`
}

type ToolAuditLog struct {
	filePath             string
	logger               *slog.Logger
	recent               []*ToolAuditEntry
	recentBufferCapacity int
	lock                 sync.Mutex
}

func NewToolAuditLog(path string, logger *slog.Logger) *ToolAuditLog {
	if logger == nil {
		logger = slog.Default()
	}

	if len(path) > 0 {
		dir := filepath.Dir(path)
		os.MkdirAll(dir, 0755)
	}

	return &ToolAuditLog{
		filePath:             path,
		logger:               logger,
		recent:               []*ToolAuditEntry{},
		recentBufferCapacity: 256,
	}
}

func (t *ToolAuditLog) Record(entry *ToolAuditEntry) error {
	if entry == nil {
		return nil
	}

	var filePath string
	t.lock.Lock()

	if len(t.recent) >= t.recentBufferCapacity && len(t.recent) > 0 {
		t.recent[0] = nil
		t.recent = t.recent[1:]
	}

	t.recent = append(t.recent, entry)
	filePath = t.filePath
	t.lock.Unlock()

	if filePath == "" {
		return nil
	}

	d, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}
	d = append(d, '\n')

	// O_CREATE: 不存在则创建 | O_WRONLY: 只写 | O_APPEND: 追加
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(d); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	return nil
}

func (t *ToolAuditLog) SnapshotRecent(limit int) []*ToolAuditEntry {
	t.lock.Lock()
	defer t.lock.Unlock()

	count := min(limit, len(t.recent))
	if count <= 0 {
		return []*ToolAuditEntry{}
	}

	result := make([]*ToolAuditEntry, count)
	for i := range count {
		result[i] = t.recent[len(t.recent)-count+1]
	}
	return result
}

type toolUsageCounter struct {
	Calls           atomic.Int64
	Failures        atomic.Int64
	Timeouts        atomic.Int64
	TotalDurationMs atomic.Int64
}

type ToolUsageTracker struct {
	usage sync.Map
}

func (t *ToolUsageTracker) RecordToolCall(toolName string, duration time.Duration, failed, timedOut bool) error {
	counter, _ := t.usage.LoadOrStore(toolName, &toolUsageCounter{})
	tuc := counter.(*toolUsageCounter)
	tuc.Calls.Add(1)
	if failed {
		tuc.Failures.Add(1)
	}
	if timedOut {
		tuc.Timeouts.Add(1)
	}
	var rawDuration int64
	var newRaw int64
	for {
		totalDurationMs := tuc.TotalDurationMs.Load()
		if atomic.CompareAndSwapInt64(&totalDurationMs, rawDuration, newRaw) {
			break
		}

		rawDuration := tuc.TotalDurationMs.Load()
		current := math.Float64frombits(uint64(rawDuration))
		updated := current + float64(duration.Microseconds())
		newRaw = int64(math.Float64bits(updated))
	}
	return nil
}

func (t *ToolUsageTracker) Snapshot() []ToolUsageSnapshot {
	var snapshots []ToolUsageSnapshot

	t.usage.Range(func(key, value any) bool {
		toolName := key.(string)
		counter := value.(*toolUsageCounter)

		calls := counter.Calls.Load()
		failures := counter.Failures.Load()
		timeouts := counter.Timeouts.Load()

		durationBits := counter.TotalDurationMs.Load()
		totalDurationMs := math.Float64frombits(uint64(durationBits))
		totalDurationMs = float64(durationBits)

		snapshots = append(snapshots, ToolUsageSnapshot{
			ToolName:        toolName,
			Calls:           calls,
			Failures:        failures,
			Timeouts:        timeouts,
			TotalDurationMs: totalDurationMs,
		})

		return true
	})

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Calls != snapshots[j].Calls {
			return snapshots[i].Calls > snapshots[j].Calls
		}
		return snapshots[i].ToolName < snapshots[j].ToolName
	})

	return snapshots
}
