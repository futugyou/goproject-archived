package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/futugyou/extensions_ai/abstractions/embeddings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresMemoryStore struct {
	db                 *gorm.DB
	enableVectors      bool
	ftsEnabled         bool
	embeddingGenerator embeddings.IEmbeddingGenerator[string, embeddings.EmbeddingT[float64]]
}

func NewPostgresMemoryStore(db *gorm.DB, enableVectors, ftsEnabled bool, embeddingGenerator embeddings.IEmbeddingGenerator[string, embeddings.EmbeddingT[float64]]) (*PostgresMemoryStore, error) {
	store := &PostgresMemoryStore{
		db:                 db,
		enableVectors:      enableVectors,
		ftsEnabled:         ftsEnabled,
		embeddingGenerator: embeddingGenerator,
	}
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

// SearchNotes implements [IMemoryNoteSearch].
func (s *PostgresMemoryStore) SearchNotes(ctx context.Context, query string, prefix *string, limit int) ([]MemoryNoteHit, error) {
	prefixstr := ""
	if prefix != nil {
		prefixstr = *prefix
	}
	// Clamp 到 [1, 50]
	if limit < 1 {
		limit = 1
	} else if limit > 50 {
		limit = 50
	}
	var hits []MemoryNoteHit
	tx := s.db.WithContext(ctx).Model(&Note{})

	// 模式 1: 混合检索 (FTS + Vector)
	if s.ftsEnabled && s.enableVectors {
		emb, err := s.embeddingGenerator.Generate(ctx, []string{query}, nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		var queryEmbedding []float64
		if err == nil && emb.Count() > 0 {
			queryEmbedding = emb.Get(0).Vector
		}

		if len(queryEmbedding) > 0 {
			// Postgres 混合得分 SQL:
			// 1. ts_rank 计算文本相关性 (0 到 1 之间)
			// 2. (1 - (n.embedding <=> $1)) 计算余弦相似度 (1 减去余弦距离)
			err = tx.Select(`
            key, content, updated_at,
            (ts_rank(to_tsvector('english', content), plainto_tsquery('english', ?)) * 0.4 + 
            (1.0 - (embedding <=> ?)) * 0.6) AS score`, query, queryEmbedding).
				Where("key LIKE ?", prefixstr+"%").
				Where("to_tsvector('english', content) @@ plainto_tsquery('english', ?)", query).
				Order("score DESC").
				Limit(limit).
				Find(&hits).Error

			return hits, err
		}
	}

	// 模式 2: 纯 Postgres FTS 全文检索
	if s.ftsEnabled {
		err := tx.Select("key, content, updated_at, ts_rank(to_tsvector('english', content), plainto_tsquery('english', ?)) AS score", query).
			Where("key LIKE ?", prefixstr+"%").
			Where("to_tsvector('english', content) @@ plainto_tsquery('english', ?)", query).
			Order("score DESC, updated_at DESC").
			Limit(limit).
			Find(&hits).Error
		return hits, err
	}

	// 模式 3: 基础模糊搜索 (LIKE)
	err := tx.Select("key, content, updated_at, 1.0 AS score").
		Where("key LIKE ?", prefixstr+"%").
		Where("(key ILIKE ? OR content ILIKE ?)", "%"+query+"%", "%"+query+"%").
		Order("updated_at DESC").
		Limit(limit).
		Find(&hits).Error

	return hits, err
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
var _ IMemoryNoteSearch = (*PostgresMemoryStore)(nil)
