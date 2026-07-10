package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var _ IGoalService = (*InMemoryGoalService)(nil)

type InMemoryGoalService struct {
	historyFilePath string
	goals           sync.Map //map[string]*SessionGoal
	logger          *slog.Logger
	mu              sync.Mutex
}

func NewInMemoryGoalService(historyFilePath string, logger *slog.Logger) *InMemoryGoalService {
	return &InMemoryGoalService{
		historyFilePath: historyFilePath,
		logger:          logger,
	}
}

// ClearGoal implements [IGoalService].
func (i *InMemoryGoalService) ClearGoal(sessionId string) error {
	val, exists := i.goals.LoadAndDelete(sessionId)
	if !exists {
		return nil
	}

	goal := val.(*SessionGoal)

	goal.mu.Lock()
	defer goal.mu.Unlock()

	if !goal.Status.IsTerminal() {
		return i.RecordGoalHistory(goal)
	}

	return nil
}

// CreateGoal implements [IGoalService].
func (i *InMemoryGoalService) CreateGoal(sessionId string, objective string, tokenBudget int64, tokensAtStart int64) (*SessionGoal, error) {
	if isBlank(sessionId) || isBlank(objective) {
		return nil, errors.New("invalid parameter")
	}

	if len(objective) > 4000 {
		return nil, errors.New("objective exceeds max length of 4000 characters")
	}

	if tokenBudget < 0 {
		return nil, errors.New("token budget cannot be negative")
	}

	if tokensAtStart < 0 {
		return nil, errors.New("token baseline cannot be negative")
	}

	var goal = &SessionGoal{
		SessionId:     sessionId,
		Objective:     objective,
		TokenBudget:   tokenBudget,
		TokensAtStart: tokensAtStart,
	}

	_, loaded := i.goals.LoadOrStore(sessionId, goal)
	if loaded {
		return nil, fmt.Errorf("goal already exists for session '%s'. clear it first", sessionId)
	}

	return goal, nil
}

// GetGoal implements [IGoalService].
func (i *InMemoryGoalService) GetGoal(sessionId string) (*SessionGoal, error) {
	val, exists := i.goals.Load(sessionId)
	if !exists {
		return nil, fmt.Errorf("no goal found for session '%s'", sessionId)
	}

	goal := val.(*SessionGoal)

	return goal, nil
}

// HasActiveGoal implements [IGoalService].
func (i *InMemoryGoalService) HasActiveGoal(sessionId string) bool {
	val, exists := i.goals.Load(sessionId)
	if !exists {
		return false
	}

	goal := val.(*SessionGoal)

	goal.mu.Lock()
	defer goal.mu.Unlock()

	return goal.Status.IsPursuable()
}

// IncrementContinuationCount implements [IGoalService].
func (i *InMemoryGoalService) IncrementContinuationCount(sessionId string) int {
	val, exists := i.goals.Load(sessionId)
	if !exists {
		return 0
	}

	goal := val.(*SessionGoal)

	goal.mu.Lock()
	defer goal.mu.Unlock()

	goal.ContinuationCount++
	goal.UpdatedAt = time.Now().UTC()
	return goal.ContinuationCount

}

// RecordGoalHistory implements [IGoalService].
func (i *InMemoryGoalService) RecordGoalHistory(goal *SessionGoal) error {
	if isBlank(i.historyFilePath) {
		return nil
	}

	dir := filepath.Dir(i.historyFilePath)
	if !isBlank(dir) {
		os.MkdirAll(dir, 0755)
	}

	record := GoalHistoryRecord{
		Timestamp:         time.Now().UTC().Format(time.RFC3339Nano),
		SessionId:         goal.SessionId,
		Objective:         goal.Objective,
		Status:            goal.Status.ToDisplayName(),
		TokenBudget:       goal.TokenBudget,
		TokensUsed:        goal.TokensUsed,
		ContinuationCount: goal.ContinuationCount,
		CreatedAt:         goal.CreatedAt.Format(time.RFC3339Nano),
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return AppendAllText(i.historyFilePath, string(data))
}

// RecordTurnHash implements [IGoalService].
func (i *InMemoryGoalService) RecordTurnHash(sessionId string, normalizedText string) bool {
	val, exists := i.goals.Load(sessionId)
	if !exists {
		return false
	}

	goal := val.(*SessionGoal)

	goal.mu.Lock()
	defer goal.mu.Unlock()

	hash := ComputeTurnHash(normalizedText)

	if isBlank(hash) {
		goal.LastBlockerHash = ""
		goal.ConsecutiveBlockerCount = 0
		return false
	}

	if hash == goal.LastBlockerHash {
		goal.ConsecutiveBlockerCount++
		return goal.ConsecutiveBlockerCount >= 3
	}

	goal.LastBlockerHash = hash
	goal.ConsecutiveBlockerCount = 1
	return false
}

func isValidTransition(current, next GoalStatus) bool {
	if current == next {
		return true
	}

	switch struct{ cur, nxt GoalStatus }{current, next} {
	case
		struct{ cur, nxt GoalStatus }{GoalStatus_Active, GoalStatus_Paused},
		struct{ cur, nxt GoalStatus }{GoalStatus_Active, GoalStatus_Blocked},
		struct{ cur, nxt GoalStatus }{GoalStatus_Active, GoalStatus_BudgetLimited},
		struct{ cur, nxt GoalStatus }{GoalStatus_Active, GoalStatus_UsageLimited},
		struct{ cur, nxt GoalStatus }{GoalStatus_Active, GoalStatus_Complete},
		struct{ cur, nxt GoalStatus }{GoalStatus_Paused, GoalStatus_Active},
		struct{ cur, nxt GoalStatus }{GoalStatus_Blocked, GoalStatus_Active},
		struct{ cur, nxt GoalStatus }{GoalStatus_BudgetLimited, GoalStatus_Active},
		struct{ cur, nxt GoalStatus }{GoalStatus_UsageLimited, GoalStatus_Active}:
		return true
	default:
		return false
	}
}

// UpdateStatus implements [IGoalService].
func (i *InMemoryGoalService) UpdateStatus(sessionId string, newStatus GoalStatus, note *string) error {
	val, exists := i.goals.Load(sessionId)
	if !exists {
		return fmt.Errorf("no goal found for session '%s'", sessionId)
	}

	goal := val.(*SessionGoal)

	goal.mu.Lock()
	defer goal.mu.Unlock()

	if goal.Status.IsTerminal() {
		return fmt.Errorf("cannot transition from terminal state '%s'", goal.Status.ToDisplayName())
	}

	if !isValidTransition(goal.Status, newStatus) {
		return fmt.Errorf("invalid transition: %s -> %s", goal.Status.ToDisplayName(), newStatus.ToDisplayName())
	}

	goal.Status = newStatus
	goal.UpdatedAt = time.Now().UTC()
	if note != nil {
		goal.StatusNote = *note
	}

	if newStatus.IsTerminal() || (newStatus == GoalStatus_Blocked || newStatus == GoalStatus_BudgetLimited) {
		return i.RecordGoalHistory(goal)
	}

	return nil
}

// UpdateTokenUsage implements [IGoalService].
func (i *InMemoryGoalService) UpdateTokenUsage(sessionId string, sessionTotalTokens int64) error {
	val, exists := i.goals.Load(sessionId)
	if !exists {
		return nil
	}

	goal := val.(*SessionGoal)

	goal.mu.Lock()
	defer goal.mu.Unlock()

	goal.TokensUsed = max(0, sessionTotalTokens-goal.TokensAtStart)
	goal.UpdatedAt = time.Now().UTC()

	return nil
}
