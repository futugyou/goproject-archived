package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

var _ IGoalService = (*PostgresGoalService)(nil)

type PostgresGoalService struct {
	db *gorm.DB
}

func NewPostgresGoalService(db *gorm.DB) (*PostgresGoalService, error) {
	store := &PostgresGoalService{db: db}
	if err := store.initialize(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PostgresGoalService) initialize() error {
	return s.db.AutoMigrate(
		&SessionGoal{},
		&GoalHistoryRecord{},
	)
}

// ClearGoal implements [IGoalService].
func (p *PostgresGoalService) ClearGoal(sessionId string) error {
	ctx := context.Background()
	_, err := gorm.G[UserProfile](p.db).Where("session_id = ?", sessionId).Delete(ctx)
	return err
}

// CreateGoal implements [IGoalService].
func (p *PostgresGoalService) CreateGoal(sessionId string, objective string, tokenBudget int64, tokensAtStart int64) (*SessionGoal, error) {

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

	ctx := context.Background()
	err := p.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "session_id"}},
			UpdateAll: true,
		}).
		Create(goal).Error
	if err != nil {
		return nil, err
	}

	return goal, nil
}

// GetGoal implements [IGoalService].
func (p *PostgresGoalService) GetGoal(sessionId string) (*SessionGoal, error) {
	ctx := context.Background()
	ad, err := gorm.G[SessionGoal](p.db).Where("session_id = ?", sessionId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// HasActiveGoal implements [IGoalService].
func (p *PostgresGoalService) HasActiveGoal(sessionId string) bool {
	ctx := context.Background()
	ad, _ := gorm.G[SessionGoal](p.db).Where("session_id = ?", sessionId).First(ctx)
	if isBlank(ad.SessionId) {
		return false
	}

	return true
}

// IncrementContinuationCount implements [IGoalService].
func (p *PostgresGoalService) IncrementContinuationCount(sessionId string) int {
	ctx := context.Background()
	var updatedCount int

	// 1. 使用 gorm.Expr 让数据库自增：continuation_count = continuation_count + 1
	// 2. 使用 Clauses(clause.Returning{}) 让 Postgres 返回更新后的值
	err := p.db.WithContext(ctx).
		Model(&SessionGoal{}).
		Where("session_id = ?", sessionId).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "continuation_count"}}}).
		Updates(map[string]any{
			"continuation_count": gorm.Expr("continuation_count + 1"),
			"updated_at":         time.Now().UTC(),
		}).
		Scan(&updatedCount). // 将返回的最新值直接注入到 updatedCount 变量中
		Error

	if err != nil {
		return 0
	}

	return updatedCount
}

// RecordGoalHistory implements [IGoalService].
func (p *PostgresGoalService) RecordGoalHistory(goal *SessionGoal) error {
	record := &GoalHistoryRecord{
		Timestamp:         time.Now().UTC().Format(time.RFC3339Nano),
		SessionId:         goal.SessionId,
		Objective:         goal.Objective,
		Status:            goal.Status.ToDisplayName(),
		TokenBudget:       goal.TokenBudget,
		TokensUsed:        goal.TokensUsed,
		ContinuationCount: goal.ContinuationCount,
		CreatedAt:         goal.CreatedAt.Format(time.RFC3339Nano),
	}
	return p.db.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "timestamp"}, {Name: "session_id"}},
			UpdateAll: true,
		}).
		Create(record).Error // 传入指针
}

// RecordTurnHash implements [IGoalService].
func (p *PostgresGoalService) RecordTurnHash(sessionId string, normalizedText string) bool {
	ctx := context.Background()

	result := false
	err := p.db.Transaction(func(tx *gorm.DB) error {
		goal, err := gorm.G[SessionGoal](tx).Where("session_id = ? ", sessionId).First(ctx)
		if err != nil {
			return err
		}

		hash := ComputeTurnHash(normalizedText)

		updatedFields := make(map[string]any)

		if isBlank(hash) {
			updatedFields["last_blocker_hash"] = ""
			updatedFields["consecutive_blocker_count"] = 0
		} else if hash == goal.LastBlockerHash {
			updatedFields["consecutive_blocker_count"] = goal.ConsecutiveBlockerCount + 1
			result = goal.ConsecutiveBlockerCount >= 3
		} else {
			updatedFields["last_blocker_hash"] = hash
			updatedFields["consecutive_blocker_count"] = 1
		}

		err = tx.WithContext(ctx).
			Model(&SessionGoal{}).
			Where("session_id = ? ", sessionId).
			Updates(updatedFields).
			Error

		return err
	})

	if err != nil {
		return false
	}

	return result
}

// UpdateStatus implements [IGoalService].
func (p *PostgresGoalService) UpdateStatus(sessionId string, newStatus GoalStatus, note *string) error {
	ctx := context.Background()

	return p.db.Transaction(func(tx *gorm.DB) error {
		goal, err := gorm.G[SessionGoal](tx).Where("session_id = ? ", sessionId).First(ctx)
		if err != nil {
			return err
		}

		if goal.Status.IsTerminal() {
			return fmt.Errorf("cannot transition from terminal state '%s'", goal.Status.ToDisplayName())
		}

		if !isValidTransition(goal.Status, newStatus) {
			return fmt.Errorf("invalid transition: %s -> %s", goal.Status.ToDisplayName(), newStatus.ToDisplayName())
		}

		updatedFields := make(map[string]any)
		updatedFields["status"] = newStatus
		updatedFields["updated_at"] = time.Now().UTC()

		if note != nil {
			updatedFields["status_note"] = *note
		}

		if err = tx.WithContext(ctx).
			Model(&SessionGoal{}).
			Where("session_id = ? ", sessionId).
			Updates(updatedFields).
			Error; err != nil {
			return err
		}

		if newStatus.IsTerminal() || (newStatus == GoalStatus_Blocked || newStatus == GoalStatus_BudgetLimited) {
			record := &GoalHistoryRecord{
				Timestamp:         time.Now().UTC().Format(time.RFC3339Nano),
				SessionId:         goal.SessionId,
				Objective:         goal.Objective,
				Status:            goal.Status.ToDisplayName(),
				TokenBudget:       goal.TokenBudget,
				TokensUsed:        goal.TokensUsed,
				ContinuationCount: goal.ContinuationCount,
				CreatedAt:         goal.CreatedAt.Format(time.RFC3339Nano),
			}
			return tx.
				Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "timestamp"}, {Name: "session_id"}},
					UpdateAll: true,
				}).
				Create(record).Error
		}

		return nil
	})
}

// UpdateTokenUsage implements [IGoalService].
func (p *PostgresGoalService) UpdateTokenUsage(sessionId string, sessionTotalTokens int64) error {
	ctx := context.Background()

	// 使用 GREATEST(0, ? - tokens_at_start) 在数据库层面直接计算
	return p.db.WithContext(ctx).
		Model(&SessionGoal{}).
		Where("session_id = ?", sessionId).
		Updates(map[string]any{
			"tokens_used": gorm.Expr("GREATEST(0, ? - tokens_at_start)", sessionTotalTokens),
			"updated_at":  time.Now().UTC(),
		}).
		Error
}
