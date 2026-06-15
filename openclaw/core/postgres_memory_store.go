package core

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresMemoryStore struct {
	db            *gorm.DB
	enableVectors bool
}

func NewPostgresMemoryStore(db *gorm.DB, enableVectors bool) (*PostgresMemoryStore, error) {
	store := &PostgresMemoryStore{db: db, enableVectors: enableVectors}
	if err := store.initialize(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PostgresMemoryStore) initialize() error {
	return s.db.AutoMigrate(
		&Session{},
		&SessionBranch{},
		&MemoryNoteHit{},
	)
}

// DeleteBranch implements [IMemoryStore].
func (s *PostgresMemoryStore) DeleteBranch(ctx context.Context, branchId string) error {
	_, err := gorm.G[SessionBranch](s.db).Where("branch_id = ?", branchId).Delete(ctx)
	return err
}

// DeleteNote implements [IMemoryStore].
func (s *PostgresMemoryStore) DeleteNote(ctx context.Context, key string) error {
	_, err := gorm.G[MemoryNoteHit](s.db).Where("key = ?", key).Delete(ctx)
	return err
}

// GetSession implements [IMemoryStore].
func (s *PostgresMemoryStore) GetSession(ctx context.Context, sessionId string) (*Session, error) {
	ad, err := gorm.G[Session](s.db).Where("id = ?", sessionId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// ListBranches implements [IMemoryStore].
func (s *PostgresMemoryStore) ListBranches(ctx context.Context, sessionId string) ([]SessionBranch, error) {
	return gorm.G[SessionBranch](s.db).Find(ctx)
}

// ListNotesWithPrefix implements [IMemoryStore].
func (s *PostgresMemoryStore) ListNotesWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	var keys []string

	likePattern := prefix + "%"
	err := s.db.WithContext(ctx).
		Model(&MemoryNoteHit{}).
		Where("key ILIKE ?", likePattern).
		Order("key ASC").
		Limit(500).
		Pluck("key", &keys).
		Error

	if err != nil {
		return nil, err
	}

	return keys, nil
}

// LoadBranch implements [IMemoryStore].
func (s *PostgresMemoryStore) LoadBranch(ctx context.Context, branchId string) (*SessionBranch, error) {
	ad, err := gorm.G[SessionBranch](s.db).Where("branch_id = ?", branchId).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// LoadNote implements [IMemoryStore].
func (s *PostgresMemoryStore) LoadNote(ctx context.Context, key string) (string, error) {
	var keys []string

	err := s.db.WithContext(ctx).
		Model(&MemoryNoteHit{}).
		Where("key = ?", key).
		Limit(1).
		Pluck("key", &keys).
		Error

	if err != nil {
		return "", err
	}

	if len(keys) == 0 {
		return "", errors.New("no data found")
	}
	return keys[0], nil
}

// SaveBranch implements [IMemoryStore].
func (s *PostgresMemoryStore) SaveBranch(ctx context.Context, branch SessionBranch) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "branch_id"}},
			UpdateAll: true,
		}).
		Create(&branch).Error
}

// SaveNote implements [IMemoryStore].
func (s *PostgresMemoryStore) SaveNote(ctx context.Context, key string, content string) error {
	node := &MemoryNoteHit{
		Key:       key,
		Content:   content,
		UpdatedAt: time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			UpdateAll: true,
		}).
		Create(node).Error
}

// SaveSession implements [IMemoryStore].
func (s *PostgresMemoryStore) SaveSession(ctx context.Context, session Session) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).
		Create(&session).Error
}

var _ IMemoryStore = (*PostgresMemoryStore)(nil)
