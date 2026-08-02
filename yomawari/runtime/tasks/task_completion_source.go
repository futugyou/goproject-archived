package tasks

import (
	"context"
	"sync"
)

type TaskCompletionSource[T any] struct {
	ctx        context.Context
	cancelFunc context.CancelFunc

	done chan struct{} // 仅用于通知完成（完成时 close(done)）
	once sync.Once

	result T
	err    error
}

func NewTaskCompletionSource[T any](ctx context.Context, cancelFunc context.CancelFunc) *TaskCompletionSource[T] {
	return &TaskCompletionSource[T]{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		done:       make(chan struct{}),
	}
}

func NewFuture[T any](ctx context.Context) (Future[T], Resolver[T]) {
	ctx, cancel := context.WithCancel(ctx)
	tcs := &TaskCompletionSource[T]{
		ctx:        ctx,
		cancelFunc: cancel,
		done:       make(chan struct{}),
	}
	return tcs, tcs
}

func (tcs *TaskCompletionSource[T]) Done() <-chan struct{} {
	return tcs.done
}

func (tcs *TaskCompletionSource[T]) Context() context.Context {
	return tcs.ctx
}

func (tcs *TaskCompletionSource[T]) SetResult(result T) {
	if !tcs.TrySetResult(result) {
		panic("TaskCompletionSource has already been completed")
	}
}

func (tcs *TaskCompletionSource[T]) SetError(err error) {
	if !tcs.TrySetError(err) {
		panic("TaskCompletionSource has already been completed")
	}
}

func (tcs *TaskCompletionSource[T]) SetCanceled() {
	if !tcs.TrySetCanceled() {
		panic("TaskCompletionSource has already been completed")
	}
}

func (tcs *TaskCompletionSource[T]) TrySetResult(result T) bool {
	completed := false
	tcs.once.Do(func() {
		tcs.result = result
		close(tcs.done)
		completed = true
	})
	return completed
}

func (tcs *TaskCompletionSource[T]) TrySetError(err error) bool {
	completed := false
	tcs.once.Do(func() {
		tcs.err = err
		close(tcs.done)
		completed = true
	})
	return completed
}

func (tcs *TaskCompletionSource[T]) TrySetCanceled() bool {
	completed := false
	tcs.once.Do(func() {
		tcs.err = context.Canceled
		if tcs.cancelFunc != nil {
			tcs.cancelFunc()
		}
		close(tcs.done)
		completed = true
	})
	return completed
}

func (tcs *TaskCompletionSource[T]) Result() (T, error) {
	var zero T
	select {
	case <-tcs.done:
		if tcs.err != nil {
			return zero, tcs.err
		}
		return tcs.result, nil
	case <-tcs.ctx.Done():
		return zero, tcs.ctx.Err()
	}
}

func (tcs *TaskCompletionSource[T]) IsCompleted() bool {
	select {
	case <-tcs.done:
		return true
	default:
		return false
	}
}
