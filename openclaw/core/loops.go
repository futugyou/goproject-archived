package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
)

type AgentLoopRequestPayload struct {
	SessionId string `json:"session_id"`
	Prompt    string `json:"prompt"`
}

type IAgentLoopDispatcher interface {
	Dispatch(ctx context.Context, sessionId, prompt string) bool
}

type NoopAgentLoopDispatcher struct{}

// Dispatch implements [IAgentLoopDispatcher].
func (n *NoopAgentLoopDispatcher) Dispatch(ctx context.Context, sessionId string, prompt string) bool {
	return true
}

var _ IAgentLoopDispatcher = (*NoopAgentLoopDispatcher)(nil)

type ILoopControlService interface {
	SignalComplete(ctx context.Context, sessionId string) error
}
type LoopAction uint8

const (
	LoopAction_Schedule LoopAction = iota
	LoopAction_Cancel
	LoopAction_Status
	LoopAction_Invalid
)

type LoopCommand struct {
	Action   LoopAction `json:"action"`
	Interval string     `json:"interval"`
	Prompt   string     `json:"prompt"`
}

type LoopCommandParser struct{}

const (
	CancelCommand string = "cancel"
	StatusCommand string = "status"
)

var LoopCommandRegex = regexp.MustCompile(`(?i)^/loop\s+(?P<value>\d+)\s*(?P<unit>s|m|h)\s+(?P<prompt>.+)$`)
var (
	valueIdx  = LoopCommandRegex.SubexpIndex("value")
	unitIdx   = LoopCommandRegex.SubexpIndex("unit")
	promptIdx = LoopCommandRegex.SubexpIndex("prompt")
)

func (l *LoopCommandParser) TryParse(text string) *LoopCommand {
	if isBlank(text) || !strings.HasPrefix(text, "/loop") {
		return nil
	}

	var trimmed = strings.TrimSpace(text)

	if trimmed == "/loop cancel" || trimmed == "/loop stop" {
		return &LoopCommand{
			Action: LoopAction_Cancel,
		}
	}

	// /loop status
	if trimmed == "/loop status" {
		return &LoopCommand{
			Action: LoopAction_Status,
		}
	}

	// /loop <value><unit> <prompt>
	match := LoopCommandRegex.FindStringSubmatch(trimmed)
	if match == nil {
		return &LoopCommand{
			Action: LoopAction_Invalid,
		}
	}
	interval := match[valueIdx] + match[unitIdx]
	prompt := strings.TrimSpace(match[promptIdx])

	return &LoopCommand{
		Action:   LoopAction_Schedule,
		Interval: interval,
		Prompt:   prompt,
	}
}

type LoopTerminationDetector struct {
	loopControl ILoopControlService
}

var TerminationKeywords = map[string]struct{}{
	"LOOP_TERMINATE": {},
	"DONE":           {},
	"WORK_COMPLETE":  {},
}

func NewLoopTerminationDetector(loopControl ILoopControlService) *LoopTerminationDetector {
	return &LoopTerminationDetector{
		loopControl: loopControl,
	}
}

func (l *LoopTerminationDetector) isKeywordCharacter(b byte) bool {
	return isLetterOrDigit(b) || b == '_'
}

func (l *LoopTerminationDetector) cntainsWholeKeyword(text, keyword string) bool {
	if len(keyword) == 0 {
		return false
	}

	lowerText := strings.ToLower(text)
	lowerKeyword := strings.ToLower(keyword)

	startIndex := 0
	textLen := len(lowerText)
	keywordLen := len(lowerKeyword)

	for startIndex < textLen {
		relIndex := strings.Index(lowerText[startIndex:], lowerKeyword)
		if relIndex == -1 {
			return false
		}

		// 算出在原字符串中的绝对字节索引
		index := startIndex + relIndex

		// 2. 检查前边界
		before := index == 0 || !l.isKeywordCharacter(lowerText[index-1])

		// 3. 检查后边界
		afterIndex := index + keywordLen
		after := afterIndex == textLen || !l.isKeywordCharacter(lowerText[afterIndex])

		// 4. 两者都符合，说明是独立单词
		if before && after {
			return true
		}

		startIndex = index + 1
	}

	return false
}

func (l *LoopTerminationDetector) OnToolComplete(ctx context.Context, sessionId string) error {
	return l.loopControl.SignalComplete(ctx, sessionId)
}

func (l *LoopTerminationDetector) ScanText(ctx context.Context, sessionId, text string) bool {
	if isBlank(text) {
		return false
	}

	for keyword := range TerminationKeywords {
		if !l.cntainsWholeKeyword(text, keyword) {
			continue
		}

		if err := l.loopControl.SignalComplete(ctx, sessionId); err != nil {
			return false
		}
		return true
	}

	return false
}

type LoopEntry struct {
	SessionId      string
	Prompt         string
	CronExpression string
	ScheduledAt    time.Time

	mu             sync.Mutex
	schedule       cron.Schedule
	nextOccurrence time.Time
}

func NewLoopEntry(sessionId, prompt, cronExpression string, schedule cron.Schedule) *LoopEntry {
	now := time.Now().UTC()
	return &LoopEntry{
		SessionId:      sessionId,
		Prompt:         prompt,
		CronExpression: cronExpression,
		schedule:       schedule,
		ScheduledAt:    now,
		nextOccurrence: schedule.Next(now),
	}
}

// IsDue 检查任务是否到期，并原子化更新下一次执行时间
func (e *LoopEntry) IsDue(now time.Time) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	nowUTC := now.UTC()
	if nowUTC.Before(e.nextOccurrence) {
		return false
	}

	e.nextOccurrence = e.schedule.Next(nowUTC)
	return true
}

var _ ILoopControlService = (*ClawLoopScheduler)(nil)

type ClawLoopScheduler struct {
	logger  *slog.Logger
	entries sync.Map //map[string]*LoopEntry
}

func NewClawLoopScheduler(logger *slog.Logger) *ClawLoopScheduler {
	return &ClawLoopScheduler{
		logger: logger,
	}
}

func (s *ClawLoopScheduler) ScheduleLoop(ctx context.Context, sessionId, cronExpression, prompt string) error {
	schedule, ok := parseCronExpression(cronExpression)
	if !ok {
		return fmt.Errorf("invalid cron expression %s", cronExpression)
	}

	entry := NewLoopEntry(sessionId, prompt, cronExpression, schedule)

	s.entries.Store(strings.ToLower(sessionId), entry)
	s.logger.Info("Loop scheduled", "sessionId", sessionId, "cron", cronExpression)
	return nil
}

func (s *ClawLoopScheduler) CancelLoop(ctx context.Context, sessionId string) error {
	key := strings.ToLower(sessionId)
	if _, loaded := s.entries.LoadAndDelete(key); loaded {
		s.logger.Info("Loop canceled", "sessionId", sessionId)
	}
	return nil
}

func (s *ClawLoopScheduler) GetLoopStatus(ctx context.Context, sessionId string) (string, error) {
	key := strings.ToLower(sessionId)
	if val, ok := s.entries.Load(key); ok {
		entry := val.(*LoopEntry)
		status := fmt.Sprintf("Loop active — cron: %s, prompt: \"%s\", scheduled at: %s",
			entry.CronExpression, entry.Prompt, entry.ScheduledAt.Format(time.RFC3339))
		return status, nil
	}
	return "", nil
}

func (s *ClawLoopScheduler) SignalComplete(ctx context.Context, sessionId string) error {
	s.logger.Info("Loop termination signal received", "sessionId", sessionId)
	return s.CancelLoop(ctx, sessionId)
}

func (s *ClawLoopScheduler) GetDueEntries(now time.Time) []*LoopEntry {
	var results []*LoopEntry

	s.entries.Range(func(key, value any) bool {
		entry := value.(*LoopEntry)
		if entry.IsDue(now) {
			results = append(results, entry)
		}
		return true
	})

	return results
}

const TypeAgentLoopTask = "agent:loop_task"

type AgentLoopPayload struct {
	SessionId string `json:"session_id"`
	Prompt    string `json:"prompt"`
}

func NewAgentLoopTask(sessionId, prompt string) (*asynq.Task, error) {
	payload, err := json.Marshal(AgentLoopPayload{SessionId: sessionId, Prompt: prompt})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeAgentLoopTask, payload), nil
}

type AgentLoopJob struct {
	scheduler   *ClawLoopScheduler
	asynqClient *asynq.Client
	logger      *slog.Logger
}

func NewAgentLoopJob(sched *ClawLoopScheduler, client *asynq.Client, logger *slog.Logger) *AgentLoopJob {
	return &AgentLoopJob{
		scheduler:   sched,
		asynqClient: client,
		logger:      logger,
	}
}

func (j *AgentLoopJob) Execute(ctx context.Context) {
	now := time.Now().UTC()
	dueEntries := j.scheduler.GetDueEntries(now)

	for _, entry := range dueEntries {
		if ctx.Err() != nil {
			break
		}

		j.logger.Info("Loop tick: dispatching prompt for session", "sessionId", entry.SessionId)

		// 打包发给, 准备 asynq 分布式队列
		task, err := NewAgentLoopTask(entry.SessionId, entry.Prompt)
		if err != nil {
			j.logger.Error("Failed to serialize task", "sessionId", entry.SessionId, "err", err)
			continue
		}

		// 投递到asynq(比如 Redis)，让集群中的 Worker 去异步执行
		_, err = j.asynqClient.EnqueueContext(ctx, task)
		if err != nil {
			j.logger.Error("Loop dispatch (enqueue) failed for session", "sessionId", entry.SessionId, "err", err)
		}
	}
}

type AgentTaskHandler struct {
	dispatcher IAgentLoopDispatcher
	logger     *slog.Logger
}

var _ asynq.Handler = (*AgentTaskHandler)(nil)

func NewAgentTaskHandler(dispatcher IAgentLoopDispatcher, logger *slog.Logger) *AgentTaskHandler {
	return &AgentTaskHandler{
		dispatcher: dispatcher,
		logger:     logger,
	}
}

func (h *AgentTaskHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p AgentLoopPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}

	h.logger.Info("Executing business logic via dispatcher", "sessionId", p.SessionId)

	ok := h.dispatcher.Dispatch(ctx, p.SessionId, p.Prompt)
	if !ok {
		h.logger.Error("Dispatcher execution failed")
		return errors.New("Dispatcher execution failed")
	}

	return nil
}
