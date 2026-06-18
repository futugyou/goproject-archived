package core

import (
	"slices"
	"sync"
	"sync/atomic"
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
