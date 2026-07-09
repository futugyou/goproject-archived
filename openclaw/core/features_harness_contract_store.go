package core

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ IHarnessContractStore = (*PostgresHarnessContractStore)(nil)

type PostgresHarnessContractStore struct {
	db *gorm.DB
}

func NewPostgresHarnessContractStore(db *gorm.DB) *PostgresHarnessContractStore {
	store := &PostgresHarnessContractStore{db: db}
	if err := store.initialize(); err != nil {
		return nil
	}
	return store
}

func (s *PostgresHarnessContractStore) initialize() error {
	return s.db.AutoMigrate(
		&HarnessContract{},
	)
}

// Delete implements [IHarnessContractStore].
func (p *PostgresHarnessContractStore) Delete(ctx context.Context, id string) error {
	_, err := gorm.G[HarnessContract](p.db).Where("id = ?", id).Delete(ctx)
	return err
}

// Get implements [IHarnessContractStore].
func (p *PostgresHarnessContractStore) Get(ctx context.Context, id string) (*HarnessContract, error) {
	ad, err := gorm.G[HarnessContract](p.db).Where("id = ?", id).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ad, err
}

// List implements [IHarnessContractStore].
func (p *PostgresHarnessContractStore) List(ctx context.Context, query *HarnessContractListQuery) ([]HarnessContract, error) {
	tx := gorm.G[HarnessContract](p.db).Where("1=1")

	if query != nil {
		if query.Status != nil && *query.Status != "" {
			tx = tx.Where("status = ?", *query.Status)
		}

		if query.RiskLevel != nil && *query.RiskLevel != "" {
			tx = tx.Where("risk_level = ?", *query.RiskLevel)
		}

		if query.SourceSessionID != nil && *query.SourceSessionID != "" {
			tx = tx.Where("source_session_id = ?", *query.SourceSessionID)
		}

		if query.ActorID != nil && *query.ActorID != "" {
			tx = tx.Where("actor_id = ?", *query.ActorID)
		}

		if query.ChannelID != nil && *query.ChannelID != "" {
			tx = tx.Where("channel_id = ?", *query.ChannelID)
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

// Save implements [IHarnessContractStore].
func (p *PostgresHarnessContractStore) Save(ctx context.Context, contract *HarnessContract) error {
	return p.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).
		Create(contract).Error
}
