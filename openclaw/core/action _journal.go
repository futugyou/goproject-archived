package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
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

func (l *Lease) AcknowledgeHistory(session *Session) error {
	calls := DurableActionJournalRecordedCallIds(session)
	for _, record := range l.records {
		if record.State == "completed" && !record.HistoryPersisted && slices.Contains(calls, record.CallId) {
			(&record).HistoryPersisted = true
			if err := l.save(&record); err != nil {
				return err
			}
		}
	}
	return nil
}

type DurableActionJournal struct {
	root string
}

func NewDurableActionJournal(storagePath string) *DurableActionJournal {
	root := filepath.Join(storagePath, "action-journal")
	return &DurableActionJournal{root: root}
}

func (j *DurableActionJournal) Open(ctx context.Context, sessionId string) (*Lease, error) {
	if err := os.MkdirAll(j.root, 0755); err != nil {
		return nil, err
	}

	stem := filepath.Join(j.root, util.ComputeTurnHash(sessionId))
	lockPath := stem + ".lock"
	jsonPath := stem + ".json"

	var lockFile *os.File
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 使用 O_CREATE | O_EXCL 实现排他性创建，模拟独占锁
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err == nil {
			lockFile = f
			defer func() {
				if lockFile != nil && err != nil {
					lockFile.Close()
					os.Remove(lockPath)
				}
			}()
			break
		}

		// 如果文件已存在（说明被其他进程锁定），则等待后重试
		if os.IsExist(err) {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		return nil, err
	}

	// 读取并解析现有记录
	var records []ActionRecord
	if _, err := os.Stat(jsonPath); err == nil {
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			lockFile.Close()
			os.Remove(lockPath)
			return nil, err
		}
		if err := json.Unmarshal(data, &records); err != nil {
			lockFile.Close()
			os.Remove(lockPath)
			return nil, errors.New("invalid action journal")
		}
	} else {
		records = []ActionRecord{}
	}

	// 验证记录合法性
	for _, r := range records {
		if r.SessionId != sessionId || r.Id != util.ComputeTurnHash(sessionId+"\n"+r.CallId) ||
			(r.State != "started" && r.State != "completed" && r.State != "not_executed") ||
			r.Revision < 1 || (r.State == "completed" && r.Result == "") ||
			(r.State != "started" && strings.TrimSpace(r.Evidence) == "") {
			lockFile.Close()
			os.Remove(lockPath)
			return nil, errors.New("invalid action journal records")
		}
	}

	return &Lease{
		path:      jsonPath,
		sessionId: sessionId,
		lockFile:  lockFile,
		records:   records,
	}, nil
}

func DurableActionJournalRecordedCallIds(session *Session) []string {
	result := []string{}
	for _, v := range session.History {
		for _, vv := range v.ToolCalls {
			if vv.CallId != "" {
				result = append(result, vv.CallId)
			}
		}
	}

	for _, v := range session.MetaRunHistory {
		for _, vv := range v.StepResults {
			if vv.ExecutionEvidence != nil && vv.ExecutionEvidence.CapabilityInvocation != nil && vv.ExecutionEvidence.CapabilityInvocation.CallId != "" {
				result = append(result, vv.ExecutionEvidence.CapabilityInvocation.CallId)
			}
		}
	}

	return result
}

func (j *DurableActionJournal) AcknowledgePersistedHistory(ctx context.Context, session *Session) error {
	subctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	lease, err := j.Open(subctx, session.Id)
	if err != nil {
		return err
	}

	return lease.AcknowledgeHistory(session)
}
