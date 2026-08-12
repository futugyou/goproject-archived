package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateOpen:
		return "Open"
	case StateHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

// 当熔断器处于 Open 状态或 HalfOpen 拒绝新探针时抛出
type ErrCircuitOpen struct {
	RetryAfter time.Duration
}

func (e *ErrCircuitOpen) Error() string {
	return fmt.Sprintf("circuit breaker is open, please try again after %v", e.RetryAfter)
}

type CircuitBreaker struct {
	mu                  sync.Mutex
	failureThreshold    int
	baseCooldown        time.Duration
	maxCooldown         time.Duration
	logger              *slog.Logger
	consecutiveFailures int
	probeFailures       int
	openedAt            time.Time
	currentCooldown     time.Duration
	state               CircuitState
	halfOpenProbeActive bool
}

type Option func(*CircuitBreaker)

func WithFailureThreshold(threshold int) Option {
	return func(cb *CircuitBreaker) {
		if threshold < 1 {
			threshold = 1
		}
		cb.failureThreshold = threshold
	}
}

func WithCooldown(cooldown time.Duration) Option {
	return func(cb *CircuitBreaker) {
		cb.baseCooldown = cooldown
		cb.currentCooldown = cooldown
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(cb *CircuitBreaker) {
		cb.logger = logger
	}
}

func New(opts ...Option) *CircuitBreaker {
	cb := &CircuitBreaker{
		failureThreshold: 5,
		baseCooldown:     30 * time.Second,
		maxCooldown:      5 * time.Minute,
		state:            StateClosed,
	}
	cb.currentCooldown = cb.baseCooldown

	for _, opt := range opts {
		opt(cb)
	}
	return cb
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.onSuccess()
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.onFailure()
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures = 0
	cb.probeFailures = 0
	cb.currentCooldown = cb.baseCooldown
	cb.state = StateClosed
	cb.halfOpenProbeActive = false
}

// 针对流式场景/无需 Execute 包裹场景的检查
func (cb *CircuitBreaker) ThrowIfOpen() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen {
		if time.Since(cb.openedAt) >= cb.currentCooldown {
			cb.state = StateHalfOpen
			if cb.logger != nil {
				cb.logger.Info("Circuit breaker transitioning to HalfOpen")
			}
		} else {
			retryAfter := time.Until(cb.openedAt.Add(cb.currentCooldown))
			return &ErrCircuitOpen{RetryAfter: retryAfter}
		}
	}
	return nil
}

func Execute[T any](cb *CircuitBreaker, ctx context.Context, action func(ctx context.Context) (T, error)) (T, error) {
	var zero T

	// 1. 状态判断与探针锁抢占
	err := func() error {
		cb.mu.Lock()
		defer cb.mu.Unlock()

		switch cb.state {
		case StateOpen:
			if time.Since(cb.openedAt) >= cb.currentCooldown {
				cb.state = StateHalfOpen
				cb.halfOpenProbeActive = true
				if cb.logger != nil {
					cb.logger.Info("Circuit breaker transitioning to HalfOpen")
				}
			} else {
				retryAfter := time.Until(cb.openedAt.Add(cb.currentCooldown))
				return &ErrCircuitOpen{RetryAfter: retryAfter}
			}

		case StateHalfOpen:
			if cb.halfOpenProbeActive {
				retryAfter := time.Until(cb.openedAt.Add(cb.currentCooldown))
				return &ErrCircuitOpen{RetryAfter: retryAfter}
			}
			cb.halfOpenProbeActive = true
		}

		return nil
	}()

	if err != nil {
		return zero, err
	}

	// 2. 执行业务逻辑
	result, err := action(ctx)

	// 3. 拦截 Context 取消（不计入服务失败）
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return zero, err
	}

	if err != nil {
		cb.onFailure()
		return zero, err
	}

	cb.onSuccess()
	return result, nil
}

func (cb *CircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateClosed && cb.consecutiveFailures == 0 {
		return
	}

	if cb.state == StateHalfOpen && cb.logger != nil {
		cb.logger.Info("Circuit breaker closing (probe succeeded)")
	}

	cb.consecutiveFailures = 0
	cb.probeFailures = 0
	cb.currentCooldown = cb.baseCooldown
	cb.state = StateClosed
	cb.halfOpenProbeActive = false
}

func (cb *CircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++

	if cb.state == StateHalfOpen || (cb.state == StateClosed && cb.consecutiveFailures >= cb.failureThreshold) {
		if cb.state == StateHalfOpen {
			cb.probeFailures++
			cb.currentCooldown = cb.computeBackoffCooldown(cb.probeFailures)
		} else {
			cb.probeFailures = 0
			cb.currentCooldown = cb.baseCooldown
		}

		cb.state = StateOpen
		cb.halfOpenProbeActive = false
		cb.openedAt = time.Now()

		if cb.logger != nil {
			cb.logger.Warn(fmt.Sprintf("Circuit breaker opened after %d consecutive failures. Will retry after %v.", cb.consecutiveFailures, cb.currentCooldown))
		}
	}
}

func (cb *CircuitBreaker) computeBackoffCooldown(probeFailures int) time.Duration {
	if probeFailures < 1 {
		probeFailures = 1
	}
	multiplier := math.Pow(2, float64(probeFailures))
	cooldown := time.Duration(float64(cb.baseCooldown) * multiplier)
	if cooldown > cb.maxCooldown {
		return cb.maxCooldown
	}
	return cooldown
}
