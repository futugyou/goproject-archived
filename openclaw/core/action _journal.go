package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/futugyou/openclaw/util"
)

type IReconcilableTool interface {
	ITool
	Reconcile(ctx context.Context, idempotencyKey string) (*ActionOutcome, error)
	ExecuteWithIdempotency(ctx context.Context, argumentsJson, idempotencyKey string, executionContext ToolExecutionContext) (string, error)
}

type ActionOutcome struct {
	State  string
	Result string
}

type ActionRecord struct {
	Id               string
	SessionId        string
	CallId           string
	ToolName         string
	ArgumentsHash    string
	State            string
	Result           string
	HistoryPersisted bool
	Evidence         string
	Revision         int64
	UpdatedAtUtc     time.Time
}

type Lease struct {
	path      string
	sessionId string
	lockFile  *os.File // 用于保持独占锁的文件句柄
	records   []ActionRecord
}

func (l *Lease) Close() error {
	if l.lockFile != nil {
		lockPath := strings.TrimSuffix(l.path, ".json") + ".lock"
		l.lockFile.Close()
		os.Remove(lockPath) // 删除锁文件，允许其他进程进入
	}
	return nil
}

func (l *Lease) Begin(callId, toolName, arguments string) (*ActionRecord, error) {
	argHash := util.ComputeTurnHash(arguments)
	for i := range l.records {
		if l.records[i].CallId == callId {
			if l.records[i].ToolName != toolName || l.records[i].ArgumentsHash != argHash {
				return nil, errors.New("action identity was reused with different arguments")
			}
			return &l.records[i], nil
		}
	}

	record := ActionRecord{
		Id:            util.ComputeTurnHash(l.sessionId + "\n" + callId),
		SessionId:     l.sessionId,
		CallId:        callId,
		ToolName:      toolName,
		ArgumentsHash: argHash,
		State:         "started",
	}
	l.records = append(l.records, record)
	if err := l.save(&l.records[len(l.records)-1]); err != nil {
		return nil, err
	}
	return &l.records[len(l.records)-1], nil
}

func (l *Lease) save(record *ActionRecord) error {
	record.Revision++
	record.UpdatedAtUtc = time.Now().UTC()

	tempPath := l.path + "." + util.ComputeTurnHash(time.Now().String()) + ".tmp"
	data, err := json.Marshal(l.records)
	if err != nil {
		return err
	}

	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tempPath, l.path)
}
