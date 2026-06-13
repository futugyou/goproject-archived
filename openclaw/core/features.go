package core

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresFeatureStore struct {
	db *gorm.DB
}

func NewPostgresFeatureStore(db *gorm.DB) (*PostgresFeatureStore, error) {
	store := &PostgresFeatureStore{db: db}
	if err := store.initialize(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PostgresFeatureStore) initialize() error {
	return s.db.AutoMigrate(
		&AutomationDefinition{},
		&AutomationRunRecord{},
	)
}

// DeleteAutomation implements [IAutomationStore].
func (p *PostgresFeatureStore) DeleteAutomation(ctx context.Context, automationId string) error {
	_, err := gorm.G[AutomationDefinition](p.db).Where("id = ?", automationId).Delete(ctx)
	return err
}

// GetAutomation implements [IAutomationStore].
func (p *PostgresFeatureStore) GetAutomation(ctx context.Context, automationId string) (*AutomationDefinition, error) {
	ad, err := gorm.G[AutomationDefinition](p.db).Where("id = ?", automationId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// GetRunRecord implements [IAutomationStore].
func (p *PostgresFeatureStore) GetRunRecord(ctx context.Context, automationId string, runId string) (*AutomationRunRecord, error) {
	ad, err := gorm.G[AutomationRunRecord](p.db).Where("automation_id = ? AND run_id = ?", automationId, runId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// GetRunState implements [IAutomationStore].
func (p *PostgresFeatureStore) GetRunState(ctx context.Context, automationId string) (*AutomationRunState, error) {
	ad, err := gorm.G[AutomationRunState](p.db).Where("automation_id = ? ", automationId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// ListAutomations implements [IAutomationStore].
func (p *PostgresFeatureStore) ListAutomations(ctx context.Context) ([]AutomationDefinition, error) {
	return gorm.G[AutomationDefinition](p.db).Find(ctx)
}

// ListRunRecords implements [IAutomationStore].
func (p *PostgresFeatureStore) ListRunRecords(ctx context.Context, automationId string, limit int) ([]AutomationRunRecord, error) {
	return gorm.G[AutomationRunRecord](p.db).Where("automation_id = ?", automationId).Order("started_at_utc desc").Limit(limit).Find(ctx)
}

// PruneRunRecords implements [IAutomationStore].
func (p *PostgresFeatureStore) PruneRunRecords(ctx context.Context, automationId string, retainCount int) error {
	if retainCount < 1 {
		retainCount = 1
	}
	sql := `DELETE FROM automation_run_record 
	        WHERE automation_id = ? 
	          AND run_id IN (
	              SELECT run_id 
	              FROM automation_run_record 
	              WHERE automation_id = ? 
	              ORDER BY started_at DESC, updated_at DESC 
	              OFFSET ?
	          );`
	return p.db.WithContext(ctx).Exec(sql, automationId, automationId, retainCount).Error
}

// SaveAutomation implements [IAutomationStore].
func (p *PostgresFeatureStore) SaveAutomation(ctx context.Context, automation AutomationDefinition) error {
	automation.UpdatedAtUtc = time.Now().UTC()
	if automation.CreatedAtUtc.IsZero() {
		automation.CreatedAtUtc = automation.UpdatedAtUtc
	}

	return p.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}}, // 冲突的约束列（主键）
			UpdateAll: true,                          // 发生冲突时，更新所有字段
		}).
		Create(&automation).Error // 传入指针
}

// SaveRunRecord implements [IAutomationStore].
func (p *PostgresFeatureStore) SaveRunRecord(ctx context.Context, runRecord AutomationRunRecord) error {
	return p.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "run_id"}, {Name: "automation_id"}},
			UpdateAll: true,
		}).
		Create(&runRecord).Error
}

// SaveRunState implements [IAutomationStore].
func (p *PostgresFeatureStore) SaveRunState(ctx context.Context, runState AutomationRunState) error {
	return p.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "automation_id"}},
			UpdateAll: true,
		}).
		Create(&runState).Error
}

var _ IAutomationStore = (*PostgresFeatureStore)(nil)
