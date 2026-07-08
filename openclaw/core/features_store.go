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

// AppendBackendEvent implements [IBackendSessionStore].
func (s *PostgresFeatureStore) AppendBackendEvent(ctx context.Context, evt BackendEvent) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "session_id"}, {Name: "sequence"}},
			UpdateAll: true,
		}).
		Create(&evt).Error
}

// DeleteBackendSession implements [IBackendSessionStore].
func (s *PostgresFeatureStore) DeleteBackendSession(ctx context.Context, sessionID string) error {
	_, err := gorm.G[BackendSessionRecord](s.db).Where("session_id = ?", sessionID).Delete(ctx)
	return err
}

// GetBackendSession implements [IBackendSessionStore].
func (s *PostgresFeatureStore) GetBackendSession(ctx context.Context, sessionID string) (*BackendSessionRecord, error) {
	ad, err := gorm.G[BackendSessionRecord](s.db).Where("session_id = ?", sessionID).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// ListBackendEvents implements [IBackendSessionStore].
func (s *PostgresFeatureStore) ListBackendEvents(ctx context.Context, sessionID string, afterSequence int64, limit int) ([]BackendEvent, error) {
	return gorm.G[BackendEvent](s.db).Where("session_id = ?", sessionID).Where("sequence >= ?", afterSequence).Limit(limit).Find(ctx)
}

// ListBackendSessions implements [IBackendSessionStore].
func (s *PostgresFeatureStore) ListBackendSessions(ctx context.Context, backendID *string) ([]BackendSessionRecord, error) {
	if isBlankP(backendID) {
		return gorm.G[BackendSessionRecord](s.db).Find(ctx)
	} else {
		return gorm.G[BackendSessionRecord](s.db).Where("backend_id = ?", *backendID).Find(ctx)
	}
}

// SaveBackendSession implements [IBackendSessionStore].
func (s *PostgresFeatureStore) SaveBackendSession(ctx context.Context, session BackendSessionRecord) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "session_id"}},
			UpdateAll: true,
		}).
		Create(&session).Error
}

// DeleteAccount implements [IConnectedAccountStore].
func (s *PostgresFeatureStore) DeleteAccount(ctx context.Context, accountID string) error {
	_, err := gorm.G[ConnectedAccount](s.db).Where("id = ?", accountID).Delete(ctx)
	return err
}

// GetAccount implements [IConnectedAccountStore].
func (s *PostgresFeatureStore) GetAccount(ctx context.Context, accountID string) (*ConnectedAccount, error) {
	ad, err := gorm.G[ConnectedAccount](s.db).Where("id = ?", accountID).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// ListAccounts implements [IConnectedAccountStore].
func (s *PostgresFeatureStore) ListAccounts(ctx context.Context) ([]ConnectedAccount, error) {
	return gorm.G[ConnectedAccount](s.db).Find(ctx)
}

// SaveAccount implements [IConnectedAccountStore].
func (s *PostgresFeatureStore) SaveAccount(ctx context.Context, account ConnectedAccount) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).
		Create(&account).Error
}

// GetProposal implements [ILearningProposalStore].
func (s *PostgresFeatureStore) GetProposal(ctx context.Context, proposalId string) (*LearningProposal, error) {
	ad, err := gorm.G[LearningProposal](s.db).Where("id = ?", proposalId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// ListProposals implements [ILearningProposalStore].
func (s *PostgresFeatureStore) ListProposals(ctx context.Context, status *string, kind *string) ([]LearningProposal, error) {
	return gorm.G[LearningProposal](s.db).Find(ctx)
}

// SaveProposal implements [ILearningProposalStore].
func (s *PostgresFeatureStore) SaveProposal(ctx context.Context, proposal *LearningProposal) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).
		Create(proposal).Error
}

// DeleteProfile implements [IUserProfileStore].
func (s *PostgresFeatureStore) DeleteProfile(ctx context.Context, actorId string) error {
	_, err := gorm.G[UserProfile](s.db).Where("actor_id = ?", actorId).Delete(ctx)
	return err
}

// GetProfile implements [IUserProfileStore].
func (s *PostgresFeatureStore) GetProfile(ctx context.Context, actorId string) (*UserProfile, error) {
	ad, err := gorm.G[UserProfile](s.db).Where("actor_id = ?", actorId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// ListProfiles implements [IUserProfileStore].
func (s *PostgresFeatureStore) ListProfiles(ctx context.Context) ([]UserProfile, error) {
	return gorm.G[UserProfile](s.db).Find(ctx)
}

// SaveProfile implements [IUserProfileStore].
func (s *PostgresFeatureStore) SaveProfile(ctx context.Context, profile UserProfile) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "actor_id"}},
			UpdateAll: true,
		}).
		Create(&profile).Error
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
		&UserProfile{},
		&LearningProposal{},
		&ConnectedAccount{},
		&BackendSessionRecord{},
		&BackendEvent{},
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
var _ IUserProfileStore = (*PostgresFeatureStore)(nil)
var _ ILearningProposalStore = (*PostgresFeatureStore)(nil)
var _ IConnectedAccountStore = (*PostgresFeatureStore)(nil)
var _ IBackendSessionStore = (*PostgresFeatureStore)(nil)
