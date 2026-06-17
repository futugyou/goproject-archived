package core

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type MessageContext struct {
	ChannelId string
	SenderId  string
	Text      string
	MessageId string
	SessionId string

	/// <summary>Session-level token counters (input + output accumulated across turns).</summary>
	SessionInputTokens  int64
	SessionOutputTokens int64

	Properties map[string]any

	/// <summary>When set to true by a middleware, the message is dropped and the response text is returned directly.</summary>
	IsShortCircuited     bool
	ShortCircuitResponse string
}

func (m *MessageContext) ShortCircuit(responseText string) {
	m.IsShortCircuited = true
	m.ShortCircuitResponse = responseText
}

type MiddlewarePipeline struct {
	middleware []IMessageMiddleware
}

func NewMiddlewarePipeline(middleware []IMessageMiddleware) *MiddlewarePipeline {
	return &MiddlewarePipeline{middleware: middleware}
}

func (m *MiddlewarePipeline) Execute(ctx context.Context, messageContext *MessageContext) bool {
	if len(m.middleware) == 0 {
		return true
	}

	var index = 0

	var next func(ctx context.Context) error

	next = func(ctx context.Context) error {
		if messageContext.IsShortCircuited {
			return nil
		}
		if index < len(m.middleware) {
			var mw = m.middleware[index]
			index++
			return mw.Invoke(ctx, messageContext, next)
		}

		return nil
	}

	if err := next(ctx); err != nil {
		return false
	}

	return !messageContext.IsShortCircuited
}

type rateWindow struct {
	mu         sync.Mutex
	entries    []time.Time // Go 中使用切片替代 Queue
	lastSeenAt time.Time
}

var _ IMessageMiddleware = (*RateLimitMiddleware)(nil)

type RateLimitMiddleware struct {
	maxMessagesPerMinute int
	logger               *slog.Logger
	idleTtl              time.Duration
	cleanupEvery         int64
	windows              sync.Map // 键为 string (格式: "channelId:senderId")
	requestCount         int64
}

func NewRateLimitMiddleware(maxMessagesPerMinute int, logger *slog.Logger, idleTtl *time.Duration, cleanupEvery int) *RateLimitMiddleware {
	maxMsgs := maxMessagesPerMinute
	if maxMsgs <= 0 {
		maxMsgs = math.MaxInt
	}

	ttl := 10 * time.Minute
	if idleTtl != nil {
		ttl = *idleTtl
	}

	cleanup := max(int64(cleanupEvery), 1)

	if logger == nil {
		logger = slog.Default()
	}

	return &RateLimitMiddleware{
		maxMessagesPerMinute: maxMsgs,
		logger:               logger,
		idleTtl:              ttl,
		cleanupEvery:         cleanup,
	}
}

// GetName implements [IMessageMiddleware].
func (r *RateLimitMiddleware) GetName() string {
	return "RateLimit"
}

// Invoke implements [IMessageMiddleware].
func (r *RateLimitMiddleware) Invoke(ctx context.Context, msgCtx *MessageContext, next func(context.Context) error) error {
	// 用 string 组合作为复合主键
	key := fmt.Sprintf("%s:%s", msgCtx.ChannelId, msgCtx.SenderId)

	// 获取或创建窗口
	actual, _ := r.windows.LoadOrStore(key, &rateWindow{})
	window := actual.(*rateWindow)

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	// 锁定当前用户的窗口（细粒度锁）
	window.mu.Lock()
	// 1. 驱逐过期条目
	for len(window.entries) > 0 && window.entries[0].Before(cutoff) {
		window.entries = window.entries[1:]
	}

	// 2. 检查是否限流
	if len(window.entries) >= r.maxMessagesPerMinute {
		r.logger.Warn("Rate limit exceeded", "key", key, "count", len(window.entries), "max", r.maxMessagesPerMinute)
		msgCtx.ShortCircuit("You're sending messages too quickly. Please wait a moment and try again.")
		window.mu.Unlock()
		return nil // 阻断后续执行，直接返回
	}

	// 3. 追加当前请求
	window.entries = append(window.entries, now)
	window.lastSeenAt = now
	window.mu.Unlock()

	// 4. 定时清理垃圾（原子操作递增）
	if atomic.AddInt64(&r.requestCount, 1)%r.cleanupEvery == 0 {
		r.cleanupStaleWindows(now, cutoff)
	}

	return next(ctx)
}

func (r *RateLimitMiddleware) cleanupStaleWindows(now time.Time, rateWindowCutoff time.Time) {
	idleCutoff := now.Add(-r.idleTtl)

	r.windows.Range(func(key, value any) bool {
		window := value.(*rateWindow)
		remove := false

		window.mu.Lock()
		// 同样先驱逐该窗口的过期数据
		for len(window.entries) > 0 && window.entries[0].Before(rateWindowCutoff) {
			window.entries = window.entries[1:]
		}

		// 如果队列为空且长期不活跃，则标记删除
		if len(window.entries) == 0 && window.lastSeenAt.Before(idleCutoff) {
			remove = true
		}
		window.mu.Unlock()

		if remove {
			r.windows.Delete(key)
		}

		return true // 继续迭代下一个
	})
}
