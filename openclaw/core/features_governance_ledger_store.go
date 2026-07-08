package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresGovernanceLedgerStore struct {
	db *gorm.DB
}

// Get implements [IGovernanceLedgerStore].
func (s *PostgresGovernanceLedgerStore) Get(ctx context.Context, id string) (*GovernanceLedgerEntry, error) {
	ad, err := gorm.G[GovernanceLedgerEntry](s.db).Where("id = ?", id).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// List implements [IGovernanceLedgerStore].
func (s *PostgresGovernanceLedgerStore) List(ctx context.Context, query *GovernanceLedgerListQuery) ([]GovernanceLedgerEntry, error) {
	tx := gorm.G[GovernanceLedgerEntry](s.db).Where("1=1")

	if query != nil {
		if query.Decision != nil && *query.Decision != "" {
			tx = tx.Where("decision = ?", *query.Decision)
		}

		if query.Status != nil && *query.Status != "" {
			tx = tx.Where("status = ?", *query.Status)
		}

		if query.ToolName != nil && *query.ToolName != "" {
			tx = tx.Where("tool_name = ?", *query.ToolName)
		}

		if query.ActionType != nil && *query.ActionType != "" {
			tx = tx.Where("action_type = ?", *query.ActionType)
		}

		if query.RiskLevel != nil && *query.RiskLevel != "" {
			tx = tx.Where("risk_level = ?", *query.RiskLevel)
		}

		if query.Scope != nil && *query.Scope != "" {
			tx = tx.Where("scope = ?", *query.Scope)
		}

		if query.SessionId != nil && *query.SessionId != "" {
			tx = tx.Where("session_id = ?", *query.SessionId)
		}

		if query.ActorId != nil && *query.ActorId != "" {
			tx = tx.Where("actor_id = ?", *query.ActorId)
		}

		if query.ChannelId != nil && *query.ChannelId != "" {
			tx = tx.Where("channel_id = ?", *query.ChannelId)
		}

		if query.DecidedBy != nil && *query.DecidedBy != "" {
			tx = tx.Where("decided_by = ?", *query.DecidedBy)
		}

		if query.CreatedFromUtc != nil {
			tx = tx.Where("created_at_utc >= ?", *query.CreatedFromUtc)
		}

		if query.CreatedToUtc != nil {
			tx = tx.Where("created_at_utc <= ?", *query.CreatedToUtc)
		}
		if query.Tag != nil && *query.Tag != "" {
			qTag := strings.TrimSpace(*query.Tag)
			tx = tx.Where("tags in  ?", qTag)
		}
	}

	return tx.Order("updated_at_utc DESC, created_at_utc DESC").
		Limit(query.Limit).
		Find(ctx)
}

// Revoke implements [IGovernanceLedgerStore].
func (s *PostgresGovernanceLedgerStore) Revoke(ctx context.Context, id string, revokedBy string, reason string) (*GovernanceLedgerEntry, error) {
	now := time.Now().UTC()

	var ad GovernanceLedgerEntry
	db := s.db.WithContext(ctx).
		Model(&ad).
		Where("id = ?", id).
		Clauses(clause.Returning{}).
		Updates(map[string]any{
			"updated_at_utc":    now,
			"status":            "revoked",
			"revoked_at_utc":    now,
			"revoked_by":        revokedBy,
			"revocation_reason": reason,
		})

	if db.Error != nil {
		return nil, db.Error
	}

	if db.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &ad, nil
}

// Save implements [IGovernanceLedgerStore].
func (s *PostgresGovernanceLedgerStore) Save(ctx context.Context, entry *GovernanceLedgerEntry) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).
		Create(entry).Error
}

func NewPostgresGovernanceLedgerStore(db *gorm.DB) *PostgresGovernanceLedgerStore {
	store := &PostgresGovernanceLedgerStore{db: db}
	if err := store.initialize(); err != nil {
		return nil
	}
	return store
}

func (s *PostgresGovernanceLedgerStore) initialize() error {
	return s.db.AutoMigrate(
		&GovernanceLedgerEntry{},
	)
}

var _ IGovernanceLedgerStore = (*PostgresGovernanceLedgerStore)(nil)
