package core

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var _ IStartupNoticeSink = (*NullStartupNoticeSink)(nil)

var NullStartupNoticeSinkInstance = &NullStartupNoticeSink{}

type NullStartupNoticeSink struct{}

// Record implements [IStartupNoticeSink].
func (n *NullStartupNoticeSink) Record(message string) error {
	return nil
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
