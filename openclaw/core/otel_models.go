package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
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
