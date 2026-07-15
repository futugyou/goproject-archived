package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
func (s *PostgresFeatureStore) ListBackendSessions(ctx context.Context, backendID string) ([]BackendSessionRecord, error) {
	if IsBlank(backendID) {
		return gorm.G[BackendSessionRecord](s.db).Find(ctx)
	} else {
		return gorm.G[BackendSessionRecord](s.db).Where("backend_id = ?", backendID).Find(ctx)
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
func (s *PostgresFeatureStore) ListProposals(ctx context.Context, status string, kind string) ([]LearningProposal, error) {
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

type FileFeatureStore struct {
	automationsPath          string
	automationRunsPath       string
	automationRunHistoryPath string
	accountsPath             string
	backendEventsPath        string
	backendSessionsPath      string
	profilesPath             string
	proposalsPath            string
}

func NewFileFeatureStore(storagePath string) (*FileFeatureStore, error) {
	root, err := filepath.Abs(storagePath)
	if err != nil {
		return nil, err
	}

	store := &FileFeatureStore{
		automationsPath:          filepath.Join(root, "automations"),
		automationRunsPath:       filepath.Join(root, "automation-runs"),
		automationRunHistoryPath: filepath.Join(root, "automation-run-history"),
		accountsPath:             filepath.Join(root, "connected-accounts"),
		backendSessionsPath:      filepath.Join(root, "backend-sessions"),
		backendEventsPath:        filepath.Join(root, "backend-session-events"),
		profilesPath:             filepath.Join(root, "profiles"),
		proposalsPath:            filepath.Join(root, "learning-proposals"),
	}

	// 初始化创建所有目录
	paths := []string{
		store.automationsPath,
		store.automationRunsPath,
		store.automationRunHistoryPath,
		store.accountsPath,
		store.backendSessionsPath,
		store.backendEventsPath,
		store.profilesPath,
		store.proposalsPath,
	}

	for _, path := range paths {
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	return store, nil
}

// ==========================================
// 3. 核心业务方法实现
// ==========================================

// --- Automations ---

func (f *FileFeatureStore) ListAutomations(ctx context.Context) ([]AutomationDefinition, error) {
	return LoadAllFile[AutomationDefinition](ctx, f.automationsPath)
}

func (f *FileFeatureStore) GetAutomation(ctx context.Context, automationId string) (*AutomationDefinition, error) {
	path := filepath.Join(f.automationsPath, EncodeKey(automationId)+".json")
	return LoadOneFile[AutomationDefinition](ctx, path)
}

func (f *FileFeatureStore) SaveAutomation(ctx context.Context, automation AutomationDefinition) error {
	path := filepath.Join(f.automationsPath, EncodeKey(automation.Id)+".json")
	return SaveOneFile(ctx, path, automation)
}

func (f *FileFeatureStore) DeleteAutomation(ctx context.Context, automationId string) error {
	_ = DeleteOneFile(filepath.Join(f.automationsPath, EncodeKey(automationId)+".json"))
	_ = DeleteOneFile(filepath.Join(f.automationRunsPath, EncodeKey(automationId)+".json"))
	DeleteDirectory(filepath.Join(f.automationRunHistoryPath, EncodeKey(automationId)))
	return nil
}

// --- Run States ---

func (f *FileFeatureStore) GetRunState(ctx context.Context, automationId string) (*AutomationRunState, error) {
	path := filepath.Join(f.automationRunsPath, EncodeKey(automationId)+".json")
	return LoadOneFile[AutomationRunState](ctx, path)
}

func (f *FileFeatureStore) SaveRunState(ctx context.Context, runState AutomationRunState) error {
	path := filepath.Join(f.automationRunsPath, EncodeKey(runState.AutomationId)+".json")
	return SaveOneFile(ctx, path, runState)
}

// --- Run Records ---

func (f *FileFeatureStore) ListRunRecords(ctx context.Context, automationId string, limit int) ([]AutomationRunRecord, error) {
	dir := filepath.Join(f.automationRunHistoryPath, EncodeKey(automationId))
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []AutomationRunRecord{}, nil
	}

	items, err := LoadAllFile[AutomationRunRecord](ctx, dir)
	if err != nil {
		return nil, err
	}

	// 排序: StartedAtUtc 倒序, 然后 CompletedAtUtc 倒序
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAtUtc != items[j].StartedAtUtc {
			return items[i].StartedAtUtc.After(items[j].StartedAtUtc)
		}
		var compI, compJ = items[i].StartedAtUtc, items[j].StartedAtUtc
		if items[i].CompletedAtUtc != nil {
			compI = *items[i].CompletedAtUtc
		}
		if items[j].CompletedAtUtc != nil {
			compJ = *items[j].CompletedAtUtc
		}
		return compI.After(compJ)
	})

	if limit < 1 {
		limit = 1
	}
	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

func (f *FileFeatureStore) GetRunRecord(ctx context.Context, automationId, runId string) (*AutomationRunRecord, error) {
	path := filepath.Join(f.automationRunHistoryPath, EncodeKey(automationId), EncodeKey(runId)+".json")
	return LoadOneFile[AutomationRunRecord](ctx, path)
}

func (f *FileFeatureStore) SaveRunRecord(ctx context.Context, runRecord AutomationRunRecord) error {
	dir := filepath.Join(f.automationRunHistoryPath, EncodeKey(runRecord.AutomationId))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, EncodeKey(runRecord.RunId)+".json")
	return SaveOneFile(ctx, path, runRecord)
}

func (f *FileFeatureStore) PruneRunRecords(ctx context.Context, automationId string, retainCount int) error {
	dir := filepath.Join(f.automationRunHistoryPath, EncodeKey(automationId))
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	retain := max(retainCount, 1)

	records, err := LoadAllFile[AutomationRunRecord](ctx, dir)
	if err != nil {
		return err
	}

	// 排序机制相同
	sort.Slice(records, func(i, j int) bool {
		if records[i].StartedAtUtc != records[j].StartedAtUtc {
			return records[i].StartedAtUtc.After(records[j].StartedAtUtc)
		}
		var compI, compJ = records[i].StartedAtUtc, records[j].StartedAtUtc
		if records[i].CompletedAtUtc != nil {
			compI = *records[i].CompletedAtUtc
		}
		if records[j].CompletedAtUtc != nil {
			compJ = *records[j].CompletedAtUtc
		}
		return compI.After(compJ)
	})

	if len(records) <= retain {
		return nil
	}

	toDelete := records[retain:]
	for _, record := range toDelete {
		path := filepath.Join(dir, EncodeKey(record.RunId)+".json")
		_ = DeleteOneFile(path)
	}

	return nil
}

// --- Profiles ---

func (f *FileFeatureStore) ListProfiles(ctx context.Context) ([]UserProfile, error) {
	return LoadAllFile[UserProfile](ctx, f.profilesPath)
}

func (f *FileFeatureStore) GetProfile(ctx context.Context, actorId string) (*UserProfile, error) {
	path := filepath.Join(f.profilesPath, EncodeKey(actorId)+".json")
	return LoadOneFile[UserProfile](ctx, path)
}

func (f *FileFeatureStore) SaveProfile(ctx context.Context, profile UserProfile) error {
	path := filepath.Join(f.profilesPath, EncodeKey(profile.ActorId)+".json")
	return SaveOneFile(ctx, path, profile)
}

func (f *FileFeatureStore) DeleteProfile(ctx context.Context, actorId string) error {
	return DeleteOneFile(filepath.Join(f.profilesPath, EncodeKey(actorId)+".json"))
}

// --- Proposals ---

func (f *FileFeatureStore) ListProposals(ctx context.Context, status string, kind string) ([]LearningProposal, error) {
	all, err := LoadAllFile[LearningProposal](ctx, f.proposalsPath)
	if err != nil {
		return nil, err
	}

	var filtered []LearningProposal
	statusStr := strings.TrimSpace(status)

	kindstr := strings.TrimSpace(kind)

	for _, item := range all {
		if statusStr != "" && !strings.EqualFold(item.Status, statusStr) {
			continue
		}
		if kindstr != "" && !strings.EqualFold(item.Kind, kindstr) {
			continue
		}
		filtered = append(filtered, item)
	}

	// 按照 UpdatedAtUtc 降序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAtUtc.After(filtered[j].UpdatedAtUtc)
	})

	return filtered, nil
}

func (f *FileFeatureStore) GetProposal(ctx context.Context, proposalId string) (*LearningProposal, error) {
	path := filepath.Join(f.proposalsPath, EncodeKey(proposalId)+".json")
	return LoadOneFile[LearningProposal](ctx, path)
}

func (f *FileFeatureStore) SaveProposal(ctx context.Context, proposal *LearningProposal) error {
	path := filepath.Join(f.proposalsPath, EncodeKey(proposal.Id)+".json")
	return SaveOneFile(ctx, path, proposal)
}

// --- Connected Accounts ---

func (f *FileFeatureStore) ListAccounts(ctx context.Context) ([]ConnectedAccount, error) {
	return LoadAllFile[ConnectedAccount](ctx, f.accountsPath)
}

func (f *FileFeatureStore) GetAccount(ctx context.Context, accountId string) (*ConnectedAccount, error) {
	path := filepath.Join(f.accountsPath, EncodeKey(accountId)+".json")
	return LoadOneFile[ConnectedAccount](ctx, path)
}

func (f *FileFeatureStore) SaveAccount(ctx context.Context, account ConnectedAccount) error {
	path := filepath.Join(f.accountsPath, EncodeKey(account.Id)+".json")
	return SaveOneFile(ctx, path, account)
}

func (f *FileFeatureStore) DeleteAccount(ctx context.Context, accountId string) error {
	return DeleteOneFile(filepath.Join(f.accountsPath, EncodeKey(accountId)+".json"))
}

// --- Backend Sessions ---

func (f *FileFeatureStore) ListBackendSessions(ctx context.Context, backendId string) ([]BackendSessionRecord, error) {
	all, err := LoadAllFile[BackendSessionRecord](ctx, f.backendSessionsPath)
	if err != nil {
		return nil, err
	}

	backendIdStr := strings.TrimSpace(backendId)
	if backendIdStr == "" {
		return all, nil
	}

	var filtered []BackendSessionRecord
	for _, item := range all {
		if strings.EqualFold(item.BackendId, backendIdStr) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (f *FileFeatureStore) GetBackendSession(ctx context.Context, sessionId string) (*BackendSessionRecord, error) {
	path := filepath.Join(f.backendSessionsPath, EncodeKey(sessionId)+".json")
	return LoadOneFile[BackendSessionRecord](ctx, path)
}

func (f *FileFeatureStore) SaveBackendSession(ctx context.Context, session BackendSessionRecord) error {
	path := filepath.Join(f.backendSessionsPath, EncodeKey(session.SessionId)+".json")
	return SaveOneFile(ctx, path, session)
}

func (f *FileFeatureStore) DeleteBackendSession(ctx context.Context, sessionId string) error {
	return DeleteOneFile(filepath.Join(f.backendSessionsPath, EncodeKey(sessionId)+".json"))
}

// --- Backend Events ---

func (f *FileFeatureStore) AppendBackendEvent(ctx context.Context, evt BackendEvent) error {
	path := filepath.Join(f.backendEventsPath, EncodeKey(evt.SessionID)+".json")

	// 如果文件存在，载入已有的 events 数组；若不存在则新建
	var events []BackendEvent
	if _, err := os.Stat(path); err == nil {
		ptr, err := LoadOneFile[[]BackendEvent](ctx, path)
		if err == nil && ptr != nil {
			events = *ptr
		}
	}

	events = append(events, evt)
	return SaveOneFile(ctx, path, events)
}

func (f *FileFeatureStore) ListBackendEvents(ctx context.Context, sessionId string, afterSequence int64, limit int) ([]BackendEvent, error) {
	path := filepath.Join(f.backendEventsPath, EncodeKey(sessionId)+".json")
	ptr, err := LoadOneFile[[]BackendEvent](ctx, path)
	if err != nil || ptr == nil {
		return []BackendEvent{}, nil
	}

	events := *ptr
	var filtered []BackendEvent
	for _, item := range events {
		if item.Sequence > afterSequence {
			filtered = append(filtered, item)
		}
	}

	// 按照 Sequence 升序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Sequence < filtered[j].Sequence
	})

	if limit < 1 {
		limit = 1
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

var _ IAutomationStore = (*FileFeatureStore)(nil)
var _ IUserProfileStore = (*FileFeatureStore)(nil)
var _ ILearningProposalStore = (*FileFeatureStore)(nil)
var _ IConnectedAccountStore = (*FileFeatureStore)(nil)
var _ IBackendSessionStore = (*FileFeatureStore)(nil)

type SqliteFeatureStore struct {
	db *sql.DB
}

// NewSqliteFeatureStore 初始化数据库并自动创建表和索引
func NewSqliteFeatureStore(dataSourceName string) (*SqliteFeatureStore, error) {
	// "sqlite3" (github.com/mattn/go-sqlite3)
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	// 配置连接池优化并发
	db.SetMaxOpenConns(1) // SQLite 通常推荐单写连接以避免锁冲突
	db.SetConnMaxLifetime(time.Hour)

	store := &SqliteFeatureStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SqliteFeatureStore) Close() error {
	return s.db.Close()
}

func (s *SqliteFeatureStore) initSchema() error {
	// 创建通用的 KV 存储表，按 category 隔离不同业务实体
	const schema = `
	CREATE TABLE IF NOT EXISTS kv_store (
		category TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		PRIMARY KEY (category, key)
	);

	CREATE TABLE IF NOT EXISTS automation_history (
		automation_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		started_at_utc INTEGER NOT NULL,
		completed_at_utc INTEGER,
		value TEXT NOT NULL,
		PRIMARY KEY (automation_id, run_id)
	);
	CREATE INDEX IF NOT EXISTS idx_automation_history_sort ON automation_history(automation_id, started_at_utc DESC, completed_at_utc DESC);

	CREATE TABLE IF NOT EXISTS backend_events (
		session_id TEXT NOT NULL,
		sequence INTEGER NOT NULL,
		value TEXT NOT NULL,
		PRIMARY KEY (session_id, sequence)
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// ==========================================
// 3. 核心业务方法实现
// ==========================================

// --- Automations ---

func (s *SqliteFeatureStore) ListAutomations(ctx context.Context) ([]AutomationDefinition, error) {
	return loadAllSql[AutomationDefinition](ctx, s.db, "automations")
}

func (s *SqliteFeatureStore) GetAutomation(ctx context.Context, automationId string) (*AutomationDefinition, error) {
	return loadOneSql[AutomationDefinition](ctx, s.db, "automations", automationId)
}

func (s *SqliteFeatureStore) SaveAutomation(ctx context.Context, automation AutomationDefinition) error {
	return saveOneSql(ctx, s.db, "automations", automation.Id, automation)
}

func (s *SqliteFeatureStore) DeleteAutomation(ctx context.Context, automationId string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除定义
	if _, err := tx.ExecContext(ctx, "DELETE FROM kv_store WHERE category = 'automations' AND key = ?", automationId); err != nil {
		return err
	}
	// 删除运行状态
	if _, err := tx.ExecContext(ctx, "DELETE FROM kv_store WHERE category = 'automation_runs' AND key = ?", automationId); err != nil {
		return err
	}
	// 删除历史记录
	if _, err := tx.ExecContext(ctx, "DELETE FROM automation_history WHERE automation_id = ?", automationId); err != nil {
		return err
	}

	return tx.Commit()
}

// --- Run States ---

func (s *SqliteFeatureStore) GetRunState(ctx context.Context, automationId string) (*AutomationRunState, error) {
	return loadOneSql[AutomationRunState](ctx, s.db, "automation_runs", automationId)
}

func (s *SqliteFeatureStore) SaveRunState(ctx context.Context, runState AutomationRunState) error {
	return saveOneSql(ctx, s.db, "automation_runs", runState.AutomationId, runState)
}

// --- Run Records ---

func (s *SqliteFeatureStore) ListRunRecords(ctx context.Context, automationId string, limit int) ([]AutomationRunRecord, error) {
	if limit < 1 {
		limit = 1
	}

	// 依靠 SQLite 索引完成高性能排序
	const query = `
		SELECT value FROM automation_history 
		WHERE automation_id = ? 
		ORDER BY started_at_utc DESC, COALESCE(completed_at_utc, started_at_utc) DESC 
		LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, automationId, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AutomationRunRecord
	for rows.Next() {
		var valStr string
		if err := rows.Scan(&valStr); err != nil {
			return nil, err
		}
		var item AutomationRunRecord
		if err := json.Unmarshal([]byte(valStr), &item); err == nil {
			results = append(results, item)
		}
	}
	return results, nil
}

func (s *SqliteFeatureStore) GetRunRecord(ctx context.Context, automationId, runId string) (*AutomationRunRecord, error) {
	const query = "SELECT value FROM automation_history WHERE automation_id = ? AND run_id = ?"
	var valStr string
	err := s.db.QueryRowContext(ctx, query, automationId, runId).Scan(&valStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var item AutomationRunRecord
	if err := json.Unmarshal([]byte(valStr), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *SqliteFeatureStore) SaveRunRecord(ctx context.Context, runRecord AutomationRunRecord) error {
	data, err := json.Marshal(runRecord)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO automation_history (automation_id, run_id, started_at_utc, completed_at_utc, value) 
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(automation_id, run_id) DO UPDATE SET 
			started_at_utc = excluded.started_at_utc,
			completed_at_utc = excluded.completed_at_utc,
			value = excluded.value`

	_, err = s.db.ExecContext(ctx, query, runRecord.AutomationId, runRecord.RunId, runRecord.StartedAtUtc, runRecord.CompletedAtUtc, string(data))
	return err
}

func (s *SqliteFeatureStore) PruneRunRecords(ctx context.Context, automationId string, retainCount int) error {
	if retainCount < 1 {
		retainCount = 1
	}

	// 使用 SQLite 子查询直接删除 retainCount 之外的历史记录
	const query = `
		DELETE FROM automation_history 
		WHERE automation_id = ? 
		AND run_id NOT IN (
			SELECT run_id FROM automation_history 
			WHERE automation_id = ? 
			ORDER BY started_at_utc DESC, COALESCE(completed_at_utc, started_at_utc) DESC 
			LIMIT ?
		)`

	_, err := s.db.ExecContext(ctx, query, automationId, automationId, retainCount)
	return err
}

// --- Profiles ---

func (s *SqliteFeatureStore) ListProfiles(ctx context.Context) ([]UserProfile, error) {
	return loadAllSql[UserProfile](ctx, s.db, "profiles")
}

func (s *SqliteFeatureStore) GetProfile(ctx context.Context, actorId string) (*UserProfile, error) {
	return loadOneSql[UserProfile](ctx, s.db, "profiles", actorId)
}

func (s *SqliteFeatureStore) SaveProfile(ctx context.Context, profile UserProfile) error {
	return saveOneSql(ctx, s.db, "profiles", profile.ActorId, profile)
}

func (s *SqliteFeatureStore) DeleteProfile(ctx context.Context, actorId string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM kv_store WHERE category = 'profiles' AND key = ?", actorId)
	return err
}

// --- Proposals ---

func (s *SqliteFeatureStore) ListProposals(ctx context.Context, status string, kind string) ([]LearningProposal, error) {
	all, err := loadAllSql[LearningProposal](ctx, s.db, "proposals")
	if err != nil {
		return nil, err
	}

	var filtered []LearningProposal
	statusStr := strings.TrimSpace(status)
	kindstr := strings.TrimSpace(kind)

	for _, item := range all {
		if statusStr != "" && !strings.EqualFold(item.Status, statusStr) {
			continue
		}
		if kindstr != "" && !strings.EqualFold(item.Kind, kindstr) {
			continue
		}
		filtered = append(filtered, item)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAtUtc.After(filtered[j].UpdatedAtUtc)
	})

	return filtered, nil
}

func (s *SqliteFeatureStore) GetProposal(ctx context.Context, proposalId string) (*LearningProposal, error) {
	return loadOneSql[LearningProposal](ctx, s.db, "proposals", proposalId)
}

func (s *SqliteFeatureStore) SaveProposal(ctx context.Context, proposal *LearningProposal) error {
	return saveOneSql(ctx, s.db, "proposals", proposal.Id, proposal)
}

// --- Connected Accounts ---

func (s *SqliteFeatureStore) ListAccounts(ctx context.Context) ([]ConnectedAccount, error) {
	return loadAllSql[ConnectedAccount](ctx, s.db, "accounts")
}

func (s *SqliteFeatureStore) GetAccount(ctx context.Context, accountId string) (*ConnectedAccount, error) {
	return loadOneSql[ConnectedAccount](ctx, s.db, "accounts", accountId)
}

func (s *SqliteFeatureStore) SaveAccount(ctx context.Context, account ConnectedAccount) error {
	return saveOneSql(ctx, s.db, "accounts", account.Id, account)
}

func (s *SqliteFeatureStore) DeleteAccount(ctx context.Context, accountId string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM kv_store WHERE category = 'accounts' AND key = ?", accountId)
	return err
}

// --- Backend Sessions ---

func (s *SqliteFeatureStore) ListBackendSessions(ctx context.Context, backendId string) ([]BackendSessionRecord, error) {
	all, err := loadAllSql[BackendSessionRecord](ctx, s.db, "backend_sessions")
	if err != nil {
		return nil, err
	}

	backendIdStr := strings.TrimSpace(backendId)
	if backendIdStr == "" {
		return all, nil
	}

	var filtered []BackendSessionRecord
	for _, item := range all {
		if strings.EqualFold(item.BackendId, backendIdStr) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *SqliteFeatureStore) GetBackendSession(ctx context.Context, sessionId string) (*BackendSessionRecord, error) {
	return loadOneSql[BackendSessionRecord](ctx, s.db, "backend_sessions", sessionId)
}

func (s *SqliteFeatureStore) SaveBackendSession(ctx context.Context, session BackendSessionRecord) error {
	return saveOneSql(ctx, s.db, "backend_sessions", session.SessionId, session)
}

func (s *SqliteFeatureStore) DeleteBackendSession(ctx context.Context, sessionId string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM kv_store WHERE category = 'backend_sessions' AND key = ?", sessionId)
	return err
}

// --- Backend Events ---

func (s *SqliteFeatureStore) AppendBackendEvent(ctx context.Context, evt BackendEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO backend_events (session_id, sequence, value) 
		VALUES (?, ?, ?)
		ON CONFLICT(session_id, sequence) DO UPDATE SET value = excluded.value`

	_, err = s.db.ExecContext(ctx, query, evt.SessionID, evt.Sequence, string(data))
	return err
}

func (s *SqliteFeatureStore) ListBackendEvents(ctx context.Context, sessionId string, afterSequence int64, limit int) ([]BackendEvent, error) {
	if limit < 1 {
		limit = 1
	}

	const query = `
		SELECT value FROM backend_events 
		WHERE session_id = ? AND sequence > ? 
		ORDER BY sequence ASC 
		LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, sessionId, afterSequence, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BackendEvent
	for rows.Next() {
		var valStr string
		if err := rows.Scan(&valStr); err != nil {
			return nil, err
		}
		var evt BackendEvent
		if err := json.Unmarshal([]byte(valStr), &evt); err == nil {
			results = append(results, evt)
		}
	}
	return results, nil
}

// ==========================================
//  通用私有数据库泛型辅助函数
// ==========================================

func loadAllSql[T any](ctx context.Context, db *sql.DB, category string) ([]T, error) {
	rows, err := db.QueryContext(ctx, "SELECT value FROM kv_store WHERE category = ?", category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var valStr string
		if err := rows.Scan(&valStr); err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal([]byte(valStr), &item); err == nil {
			results = append(results, item)
		}
	}
	return results, nil
}

func loadOneSql[T any](ctx context.Context, db *sql.DB, category, key string) (*T, error) {
	var valStr string
	err := db.QueryRowContext(ctx, "SELECT value FROM kv_store WHERE category = ? AND key = ?", category, key).Scan(&valStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var item T
	if err := json.Unmarshal([]byte(valStr), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func saveOneSql(ctx context.Context, db *sql.DB, category, key string, item any) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO kv_store (category, key, value) 
		VALUES (?, ?, ?) 
		ON CONFLICT(category, key) DO UPDATE SET value = excluded.value`

	_, err = db.ExecContext(ctx, query, category, key, string(data))
	return err
}

var _ IAutomationStore = (*SqliteFeatureStore)(nil)
var _ IUserProfileStore = (*SqliteFeatureStore)(nil)
var _ ILearningProposalStore = (*SqliteFeatureStore)(nil)
var _ IConnectedAccountStore = (*SqliteFeatureStore)(nil)
var _ IBackendSessionStore = (*SqliteFeatureStore)(nil)
