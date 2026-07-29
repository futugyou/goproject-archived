package shared

import (
	"context"
	"fmt"
	"sync"

	"github.com/futugyou/mcp/protocol"
)

type HandlerFunc func(ctx context.Context, notif protocol.JsonRpcNotification) error

// contextKey 用于在 Context 中传递“正在执行的 handler 标记”，防止死锁
type contextKey struct{}

var invokingKey = contextKey{}

type Registration struct {
	handlers  *NotificationHandlers
	method    string
	handler   HandlerFunc
	temporary bool

	mu            sync.Mutex
	refCount      int
	disposeCalled bool
	doneChan      chan struct{}

	Next *Registration
	Prev *Registration
}

// NotificationHandlers 对应外部主类
type NotificationHandlers struct {
	mu       sync.Mutex
	handlers map[string]*Registration
}

func NewNotificationHandlers() *NotificationHandlers {
	return &NotificationHandlers{
		handlers: make(map[string]*Registration),
	}
}

// RegisterRange 批量注册永久 handler
func (nh *NotificationHandlers) RegisterRange(handlers map[string]HandlerFunc) {
	for method, h := range handlers {
		_ = nh.Register(method, h, false)
	}
}

// Register 注册一个 handler
func (nh *NotificationHandlers) Register(method string, handler HandlerFunc, temporary bool) *Registration {
	reg := &Registration{
		handlers:  nh,
		method:    method,
		handler:   handler,
		temporary: temporary,
		refCount:  1, // 初始引用计数为 1，代表注册自身
		doneChan:  make(chan struct{}),
	}

	nh.mu.Lock()
	defer nh.mu.Unlock()

	// 插入链表头部（新注册的优先）
	if existingHead, ok := nh.handlers[method]; ok {
		reg.Next = existingHead
		existingHead.Prev = reg
	}
	nh.handlers[method] = reg

	return reg
}

func (r *Registration) Close(ctx context.Context) error {
	if !r.temporary {
		return nil
	}

	r.handlers.mu.Lock()

	r.mu.Lock()
	if !r.disposeCalled {
		r.disposeCalled = true

		// 1. 如果自己是头节点，更新 map 的指向
		if head, ok := r.handlers.handlers[r.method]; ok && head == r {
			if r.Next != nil {
				r.handlers.handlers[r.method] = r.Next
			} else {
				delete(r.handlers.handlers, r.method)
			}
		}

		// 2. 将当前节点从链表中解绑（但不清空 r.Next/r.Prev，允许已有迭代器顺利遍历）
		if r.Prev != nil {
			r.Prev.Next = r.Next
		}
		if r.Next != nil {
			r.Next.Prev = r.Prev
		}

		// 3. 减少引用计数
		r.refCount--
		if r.refCount == 0 {
			close(r.doneChan) // 没有正在执行的异步任务，直接触发完成
		}
	}
	r.mu.Unlock()

	r.handlers.mu.Unlock()

	// 死锁防护：如果 Close 动作是在同一个 Handler 执行上下文中调用的，跳过等待
	if isInvokingAncestor(ctx, r) {
		return nil
	}

	// 等待所有正在执行的在途调用（in-flight invocations）结束
	select {
	case <-r.doneChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Registration) invoke(ctx context.Context, notif protocol.JsonRpcNotification) error {
	if !r.temporary {
		return r.handler(ctx, notif)
	}
	return r.invokeTemporary(ctx, notif)
}

func (r *Registration) invokeTemporary(ctx context.Context, notif protocol.JsonRpcNotification) error {
	r.handlers.mu.Lock()
	r.mu.Lock()

	if r.disposeCalled {
		r.mu.Unlock()
		r.handlers.mu.Unlock()
		return nil // 已被 Dispose，忽略新的触发
	}

	r.refCount++
	r.mu.Unlock()
	r.handlers.mu.Unlock()

	// 在 Context 中标记“正在执行当前 registration”，防止 handler 内部 Dispose 导致死锁
	execCtx := context.WithValue(ctx, invokingKey, r)

	var err error
	defer func() {
		r.handlers.mu.Lock()
		r.mu.Lock()

		r.refCount--
		if r.refCount == 0 {
			// 最后一个在途调用结束，唤醒正在 Close 中等待的协程
			select {
			case <-r.doneChan:
			default:
				close(r.doneChan)
			}
		}

		r.mu.Unlock()
		r.handlers.mu.Unlock()
	}()

	err = r.handler(execCtx, notif)
	return err
}

// InvokeHandlers 触发特定方法的所有 Handler，按照逆序（最新注册优先）顺序调用
func (nh *NotificationHandlers) InvokeHandlers(ctx context.Context, method string, notif protocol.JsonRpcNotification) error {
	nh.mu.Lock()
	reg, ok := nh.handlers[method]
	nh.mu.Unlock()

	if !ok {
		return nil
	}

	var errs []error

	// 顺序遍历链表（从头到尾即为逆序调用）
	for reg != nil {
		if err := reg.invoke(ctx, notif); err != nil {
			errs = append(errs, err)
		}

		nh.mu.Lock()
		reg = reg.Next // 即使当前 reg 被 Close 了，Next 指针依然保留，保证遍历安全
		nh.mu.Unlock()
	}

	if len(errs) > 0 {
		return fmt.Errorf("invoke handlers encountered %d errors: %v", len(errs), errs)
	}

	return nil
}

// 辅助函数：判断 Context 中是否包含当前 Registration（死锁检查）
func isInvokingAncestor(ctx context.Context, target *Registration) bool {
	val := ctx.Value(invokingKey)
	if val == nil {
		return false
	}
	activeReg, ok := val.(*Registration)
	return ok && activeReg == target
}
