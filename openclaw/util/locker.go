package util

import (
	"context"
	"sync"
)

type NamedLockManager struct {
	mu    sync.Mutex
	gates map[string]*countedLock
}

type countedLock struct {
	ch       chan struct{}
	refCount int // 计数器：记录当前有多少个协程在引用这把锁
}

func NewNamedLockManager() *NamedLockManager {
	return &NamedLockManager{
		gates: make(map[string]*countedLock),
	}
}

// Lock 获取并锁定指定的 key，返回一个释放锁的函数
func (m *NamedLockManager) Lock(ctx context.Context, key string) (unlock func(), err error) {
	m.mu.Lock()

	// 1. 获取或创建锁，并将引用计数 +1
	lock, exists := m.gates[key]
	if !exists {
		lock = &countedLock{
			ch:       make(chan struct{}, 1),
			refCount: 0,
		}
		m.gates[key] = lock
	}
	lock.refCount++

	m.mu.Unlock()

	// 2. 尝试抢锁
	select {
	case lock.ch <- struct{}{}:
		// 抢锁成功
	case <-ctx.Done():
		// 抢锁超时/取消，需要把刚刚加的引用计数扣掉
		m.mu.Lock()
		lock.refCount--
		if lock.refCount == 0 {
			delete(m.gates, key)
		}
		m.mu.Unlock()
		return nil, ctx.Err()
	}

	// 3. 返回一个闭包用于释放锁
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		// 释放通道锁
		<-lock.ch

		// 引用计数 -1
		lock.refCount--
		// 如果没有任何协程在引用它了，安全地从 map 中剔除
		if lock.refCount == 0 {
			delete(m.gates, key)
		}
	}, nil
}
