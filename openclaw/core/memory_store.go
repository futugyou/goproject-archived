package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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

// ListBackgroundRunnableSessions implements [IBackgroundSessionStore].
func (s *PostgresMemoryStore) ListBackgroundRunnableSessions(ctx context.Context, limit int) ([]Session, error) {
	limit = max(min(limit, 500), 1)
	sessions, err := gorm.G[Session](s.db).Order("updated_at DESC").Limit(4 * limit).Find(ctx)
	if err != nil {
		return nil, err
	}

	results := []Session{}
	for _, session := range sessions {
		if session.BackgroundRun != nil && (session.RunState == SessionRunState_Running || session.RunState == SessionRunState_Continuing) {
			results = append(results, session)
		}

		if len(results) > limit {
			break
		}
	}

	slices.SortFunc(results, func(a, b Session) int {
		aTime := a.LastActiveAt
		bTime := b.LastActiveAt

		if a.BackgroundRun != nil && a.BackgroundRun.LastContinuedAtUtc != nil {
			aTime = *a.BackgroundRun.LastContinuedAtUtc
		}
		if b.BackgroundRun != nil && b.BackgroundRun.LastContinuedAtUtc != nil {
			bTime = *b.BackgroundRun.LastContinuedAtUtc
		}

		if aTime.Before(bTime) {
			return -1
		}
		if aTime.After(bTime) {
			return 1
		}

		return 0
	})

	return results, nil
}

// SearchSessions implements [ISessionSearchStore].
func (s *PostgresMemoryStore) SearchSessions(ctx context.Context, query *SessionSearchQuery) (*SessionSearchResult, error) {
	if query == nil || isBlank(query.Text) {
		return &SessionSearchResult{Query: query, Items: []SessionSearchHit{}}, nil
	}

	limit := max(min(query.Limit, 200), 1)

	// 1. 全文检索分支 (PostgreSQL tsvector)
	if s.ftsEnabled {
		var ftsResults []SessionSearchHit

		// websearch_to_tsquery 类似 Google 搜索语法
		// 如果需要严格的布尔语法，可以换成 to_tsquery
		tx := s.db.WithContext(ctx).Table("session_turns_fts").
			Select(`
				session_id, 
				channel_id, 
				sender_id, 
				role, 
				timestamp, 
				ts_headline('simplified', content, websearch_to_tsquery('simplified', ?), 'StartSel=<<, StopSel=>>, MaxWords=16, MinWords=8') as snippet,
				ts_rank(search_vector, websearch_to_tsquery('simplified', ?)) as rank
			`, query.Text, query.Text)

		// 动态拼接过滤条件
		tx = tx.Where("search_vector @@ websearch_to_tsquery('simplified', ?)", query.Text)

		if query.ChannelID != nil && *query.ChannelID != "" {
			tx = tx.Where("channel_id = ?", *query.ChannelID)
		}
		if query.SenderID != nil && *query.SenderID != "" {
			tx = tx.Where("sender_id = ?", *query.SenderID)
		}
		if query.FromUtc != nil {
			tx = tx.Where("timestamp >= ?", *query.FromUtc)
		}
		if query.ToUtc != nil {
			tx = tx.Where("timestamp <= ?", *query.ToUtc)
		}

		err := tx.Order("rank DESC").Limit(limit).Find(&ftsResults).Error
		if err != nil {
			return &SessionSearchResult{Query: query, Items: []SessionSearchHit{}}, nil
		}

		hits := make([]SessionSearchHit, len(ftsResults))
		for i, res := range ftsResults {
			hits[i] = SessionSearchHit{
				SessionID: res.SessionID,
				ChannelID: res.ChannelID,
				SenderID:  res.SenderID,
				Role:      res.Role,
				Timestamp: res.Timestamp,
				Snippet:   res.Snippet,
				Score:     res.Rank, //
			}
		}

		return &SessionSearchResult{Query: query, Items: hits}, nil
	}

	// 2. fallback
	fallback, err := s.ListSessions(ctx, 1, 200, &SessionListQuery{
		ChannelId: query.ChannelID,
		SenderId:  query.SenderID,
		FromUtc:   query.FromUtc,
		ToUtc:     query.ToUtc,
	})
	if err != nil {
		return nil, err
	}

	var itemsFallback []SessionSearchHit
	searchTextLower := strings.ToLower(query.Text)

	for _, summary := range fallback.Items {
		session, err := s.GetSession(ctx, summary.Id)
		if err != nil || session == nil {
			continue
		}

		for _, turn := range session.History {
			if isBlank(turn.Content) {
				continue
			}

			contentLower := strings.ToLower(turn.Content)
			idx := strings.Index(contentLower, searchTextLower)
			if idx < 0 {
				continue
			}

			// 计算分数
			bonus := float32(100-idx) / 100.0
			if bonus < 0 {
				bonus = 0
			}
			score := 1.0 + bonus

			itemsFallback = append(itemsFallback, SessionSearchHit{
				SessionID: session.Id,
				ChannelID: session.ChannelId,
				SenderID:  session.SenderId,
				Role:      turn.Role,
				Timestamp: turn.Timestamp,
				Snippet:   s.buildSnippet(turn.Content, idx, query.SnippetLength),
				Score:     score,
			})
		}
	}

	sort.Slice(itemsFallback, func(i, j int) bool {
		return itemsFallback[i].Score > itemsFallback[j].Score
	})

	if len(itemsFallback) > limit {
		itemsFallback = itemsFallback[:limit]
	}

	return &SessionSearchResult{
		Query: query,
		Items: itemsFallback,
	}, nil
}

func (s *PostgresMemoryStore) buildSnippet(content string, index int, snippetLength int) string {
	contentLen := len(content)

	if contentLen == 0 {
		return ""
	}

	if snippetLength > 400 {
		snippetLength = 400
	} else if snippetLength < 40 {
		snippetLength = 40
	}

	if index < 0 {
		index = 0
	} else if index > contentLen {
		index = contentLen
	}

	start := max(0, index-(snippetLength/3))
	start = min(start, contentLen)

	end := min(snippetLength+start, contentLen)

	r := strings.NewReplacer("\r", "", "\n", "")
	snippet := strings.TrimSpace(r.Replace(content[start:end]))

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < contentLen {
		snippet = snippet + "..."
	}

	return snippet
}

// ListSessions implements [ISessionAdminStore].
func (s *PostgresMemoryStore) ListSessions(ctx context.Context, page int, pageSize int, query *SessionListQuery) (*PagedSessionList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	} else if pageSize > 200 {
		pageSize = 200
	}
	tx := gorm.G[SessionSummary](s.db).Where("1=1")

	if query != nil {
		if query.ChannelId != nil && *query.ChannelId != "" {
			tx = tx.Where("channel_id = ?", *query.ChannelId)
		}
		if query.SenderId != nil && *query.SenderId != "" {
			tx = tx.Where("sender_id = ?", *query.SenderId)
		}
		if query.FromUtc != nil {
			tx = tx.Where("last_active_at >= ?", query.FromUtc.Format(time.RFC3339))
		}
		if query.ToUtc != nil {
			tx = tx.Where("last_active_at <= ?", query.ToUtc.Format(time.RFC3339))
		}
		if query.State != nil {
			tx = tx.Where("state = ? OR state = ?", fmt.Sprintf("%d", *query.State), fmt.Sprintf("%v", *query.State))
		}
		if query.Search != nil && *query.Search != "" {
			searchPattern := "%" + *query.Search + "%"
			tx = tx.Where(
				"(id LIKE ? OR channel_id LIKE ? OR sender_id LIKE ?)",
				searchPattern, searchPattern, searchPattern,
			)
		}
	}

	total, err := tx.Count(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	var items []SessionSummary
	skip := (page - 1) * pageSize

	items, err = tx.Order("last_active_at DESC, id ASC").
		Limit(pageSize).
		Offset(skip).
		Find(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch sessions: %w", err)
	}

	hasMore := total > int64(skip+pageSize)

	return &PagedSessionList{
		Page:          page,
		PageSize:      pageSize,
		HasMore:       hasMore,
		ReturnedCount: len(items),
		Items:         items,
	}, nil
}

// GetRetentionStats implements [IMemoryRetentionStore].
func (s *PostgresMemoryStore) GetRetentionStats(ctx context.Context) (*RetentionStoreStats, error) {
	var stats RetentionStoreStats

	err := s.db.Raw(`
        SELECT 
            (SELECT COUNT(*) FROM sessions) AS persisted_sessions,
            (SELECT COUNT(*) FROM session_branch) AS persisted_branches
    `).Scan(&stats).Error

	if err != nil {
		return nil, err
	}
	stats.Backend = "postgre"
	return &stats, nil
}

// Sweep implements [IMemoryRetentionStore].
func (s *PostgresMemoryStore) Sweep(ctx context.Context, request *RetentionSweepRequest, protectedSessionIds map[string]struct{}) (*RetentionSweepResult, error) {
	if request == nil {
		return nil, errors.New("request can not be nil")
	}
	var result = &RetentionSweepResult{
		StartedAtUtc: request.NowUtc,
		DryRun:       request.DryRun,
	}
	var remaining = max(1, request.MaxItems)
	var err error
	remaining, err = s.sweepSessions(ctx, request, protectedSessionIds, result, remaining)
	if err != nil {
		return nil, err
	}

	if remaining > 0 {
		remaining, err = s.sweepBranches(ctx, request, result, remaining)
		if err != nil {
			return nil, err
		}
	}

	if remaining <= 0 {
		result.MaxItemsLimitReached = true
	}

	if request.ArchiveEnabled && !request.DryRun {
		mra := MemoryRetentionArchive{}
		var purgeResult = mra.PurgeExpiredArchives(ctx, request.ArchivePath, request.NowUtc, request.ArchiveRetentionDays)

		result.ArchivePurgedFiles = purgeResult.DeletedFiles
		result.ArchivePurgeErrors = purgeResult.Errors
		for _, errorStr := range purgeResult.ErrorMessages {
			if len(result.Errors) >= 16 {
				break
			}
			result.Errors = append(result.Errors, errorStr)
		}
	}

	result.CompletedAtUtc = time.Now().UTC()
	return result, nil
}

func (s *PostgresMemoryStore) sweepBranches(ctx context.Context, request *RetentionSweepRequest, result *RetentionSweepResult, remaining int) (int, error) {
	if remaining <= 0 {
		return 0, nil
	}
	var cutoff = request.BranchExpiresBeforeUtc
	var scanLimit = min(max(remaining*4, remaining), 20_000)
	sessionBranchs, err := gorm.G[SessionBranch](s.db).Where("updated_at < ?", cutoff).Order("updated_at asc").Limit(scanLimit).Find(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return -1, err
	}

	pendingDeletes := []string{}

	for i := range sessionBranchs {
		if remaining <= 0 {
			result.MaxItemsLimitReached = true
			break
		}

		if err := ctx.Err(); err != nil {
			return -1, err
		}

		sessionBranch := sessionBranchs[i]
		if sessionBranch.CreatedAt.After(request.BranchExpiresBeforeUtc) {
			continue
		}
		result.EligibleBranches++
		remaining--
		if request.DryRun {
			continue
		}

		if request.ArchiveEnabled {
			mra := MemoryRetentionArchive{}
			data, err := json.Marshal(sessionBranch)
			if err != nil {
				if len(result.Errors) < 16 {
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to archive branch '%s': %s", sessionBranch.BranchId, err.Error()))
				}
				continue
			}
			if err := mra.ArchivePayload(ctx, request.ArchivePath, request.NowUtc, "branches", sessionBranch.BranchId, request.BranchExpiresBeforeUtc, "postgres", string(data)); err != nil {
				if len(result.Errors) < 16 {
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to archive branch '%s': %s", sessionBranch.BranchId, err.Error()))
				}
				continue
			}
			result.ArchivedBranches++
		}
		pendingDeletes = append(pendingDeletes, sessionBranch.BranchId)
	}

	if len(pendingDeletes) > 0 {
		deletedBranches, err := s.deleteBranchesById(ctx, pendingDeletes)
		if err == nil {
			result.DeletedBranches += deletedBranches
		}
	}
	return remaining, nil
}

func (s *PostgresMemoryStore) sweepSessions(ctx context.Context, request *RetentionSweepRequest, protectedSessionIds map[string]struct{}, result *RetentionSweepResult, remaining int) (int, error) {
	if remaining <= 0 {
		return 0, nil
	}
	var cutoff = request.SessionExpiresBeforeUtc
	var scanLimit = min(max(remaining*4, remaining), 20_000)
	sessions, err := gorm.G[Session](s.db).Where("updated_at < ?", cutoff).Order("updated_at asc").Limit(scanLimit).Find(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return -1, err
	}

	pendingDeletes := []string{}

	for i := range sessions {
		if remaining <= 0 {
			result.MaxItemsLimitReached = true
			break
		}

		if err := ctx.Err(); err != nil {
			return -1, err
		}

		session := sessions[i]
		if _, ok := protectedSessionIds[session.Id]; ok {
			result.SkippedProtectedSessions++
			continue
		}
		if session.LastActiveAt.After(request.SessionExpiresBeforeUtc) {
			continue
		}
		result.EligibleSessions++
		remaining--
		if request.DryRun {
			continue
		}

		if request.ArchiveEnabled {
			mra := MemoryRetentionArchive{}
			data, err := json.Marshal(session)
			if err != nil {
				if len(result.Errors) < 16 {
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to archive session '%s': %s", session.Id, err.Error()))
				}
				continue
			}
			if err := mra.ArchivePayload(ctx, request.ArchivePath, request.NowUtc, "session", session.Id, request.SessionExpiresBeforeUtc, "postgres", string(data)); err != nil {
				if len(result.Errors) < 16 {
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to archive session '%s': %s", session.Id, err.Error()))
				}
				continue
			}
			result.ArchivedSessions++
		}
		pendingDeletes = append(pendingDeletes, session.Id)
	}

	if len(pendingDeletes) > 0 {
		deletedSessions, err := s.deleteSessionsById(ctx, pendingDeletes)
		if err == nil {
			result.DeletedSessions += deletedSessions
		}
	}
	return remaining, nil
}

func (s *PostgresMemoryStore) deleteSessionsById(ctx context.Context, pendingDeletes []string) (int, error) {
	return gorm.G[Session](s.db).Where("id in ?", pendingDeletes).Delete(ctx)
}

func (s *PostgresMemoryStore) deleteBranchesById(ctx context.Context, pendingDeletes []string) (int, error) {
	return gorm.G[SessionBranch](s.db).Where("branch_id in ?", pendingDeletes).Delete(ctx)
}

// GetNoteEntry implements [IMemoryNoteCatalog].
func (s *PostgresMemoryStore) GetNoteEntry(ctx context.Context, key string) (*MemoryNoteCatalogEntry, error) {
	ad, err := gorm.G[MemoryNoteCatalogEntry](s.db).Where("key = ?", key).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if len(ad.PreviewContent) > 4096 {
		ad.PreviewContent = ad.PreviewContent[0:4096] + "..."
	}
	return &ad, err
}

// ListNotes implements [IMemoryNoteCatalog].
func (s *PostgresMemoryStore) ListNotes(ctx context.Context, prefix string, limit int) ([]MemoryNoteCatalogEntry, error) {
	return gorm.G[MemoryNoteCatalogEntry](s.db).Find(ctx)
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
		&Note{},
		&SessionSummary{},
		&SessionTurnsFts{},
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
				Where("(to_tsvector('english', content) @@ plainto_tsquery('english', ?) OR (1.0 - (embedding <=> ?)) > 0.6)", query, queryEmbedding).
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
var _ IMemoryNoteCatalog = (*PostgresMemoryStore)(nil)
var _ IMemoryRetentionStore = (*PostgresMemoryStore)(nil)
var _ ISessionAdminStore = (*PostgresMemoryStore)(nil)
var _ ISessionSearchStore = (*PostgresMemoryStore)(nil)
var _ IBackgroundSessionStore = (*PostgresMemoryStore)(nil)

// SqliteMemoryStore 实现
type SqliteMemoryStore struct {
	db                 *sql.DB
	dbPath             string
	enableFtsRequested bool
	ftsEnabled         bool
	embeddingGenerator embeddings.IEmbeddingGenerator[string, embeddings.EmbeddingT[float64]]
	enableVectors      bool
	logger             *slog.Logger
	redaction          IRedactionPipeline
}

func NewSqliteMemoryStore(
	dbPath string,
	enableFts bool,
	embeddingGenerator embeddings.IEmbeddingGenerator[string, embeddings.EmbeddingT[float64]],
	enableVectors bool,
	logger *slog.Logger,
	redaction IRedactionPipeline,
) (*SqliteMemoryStore, error) {
	if dbPath == "" {
		return nil, errors.New("dbPath cannot be empty")
	}

	dir := filepath.Dir(filepath.Clean(dbPath))
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	store := &SqliteMemoryStore{
		dbPath:             dbPath,
		enableFtsRequested: enableFts,
		embeddingGenerator: embeddingGenerator,
		enableVectors:      enableVectors && embeddingGenerator != nil,
		logger:             logger,
		redaction:          redaction,
	}

	if err := store.initialize(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *SqliteMemoryStore) connectionString() string {
	return fmt.Sprintf("file:%s?cache=shared&mode=rwc", s.dbPath)
}

func (s *SqliteMemoryStore) initialize() error {
	db, err := sql.Open("sqlite3", s.connectionString())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	s.db = db

	// 1. 初始化基础表和 PRAGMA 设置
	baseSchema := `
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA foreign_keys=ON;

		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			json TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS notes (
			key TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS branches (
			branch_id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			name TEXT NOT NULL,
			json TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_sessions_updated_at ON sessions(updated_at);
		CREATE INDEX IF NOT EXISTS idx_branches_updated_at ON branches(updated_at);
	`
	if _, err := s.db.Exec(baseSchema); err != nil {
		return fmt.Errorf("failed to initialize base schema: %w", err)
	}

	// 2. 初始化 FTS (全文检索)
	if s.enableFtsRequested {
		err := s.initializeFts()
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("FTS initialization failed, falling back to disabled", "error", err)
			}
			s.ftsEnabled = false
		} else {
			s.ftsEnabled = true
		}
	}

	// 3. 初始化向量列
	if s.enableVectors {
		// SQLite 中如果列已存在，ALTER TABLE 会报错，捕获并忽略即可
		_, _ = s.db.Exec("ALTER TABLE notes ADD COLUMN embedding BLOB;")
	}

	return nil
}

func (s *SqliteMemoryStore) initializeFts() error {
	ftsSchema := `
		CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(key, content);
		CREATE VIRTUAL TABLE IF NOT EXISTS session_turns_fts USING fts5(session_id, channel_id, sender_id, role, content, timestamp UNINDEXED);

		CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN
			INSERT INTO notes_fts(key, content) VALUES (new.key, new.content);
		END;

		CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN
			INSERT INTO notes_fts(notes_fts, key, content) VALUES ('delete', old.key, old.content);
		END;

		CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN
			INSERT INTO notes_fts(notes_fts, key, content) VALUES ('delete', old.key, old.content);
			INSERT INTO notes_fts(key, content) VALUES (new.key, new.content);
		END;
	`
	if _, err := s.db.Exec(ftsSchema); err != nil {
		return err
	}

	// Backfill notes
	backfillNotes := `
		INSERT INTO notes_fts(key, content)
		SELECT key, content FROM notes
		WHERE key NOT IN (SELECT key FROM notes_fts);
	`
	if _, err := s.db.Exec(backfillNotes); err != nil {
		return err
	}

	return s.backfillSessionSearchIndex()
}

func (s *SqliteMemoryStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SqliteMemoryStore) GetSession(ctx context.Context, sessionId string) (*Session, error) {
	if sessionId == "" {
		return nil, nil
	}

	var jsonStr string
	err := s.db.QueryRowContext(ctx, "SELECT json FROM sessions WHERE id = ? LIMIT 1;", sessionId).Scan(&jsonStr)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal([]byte(jsonStr), &session); err != nil {
		if s.logger != nil {
			s.logger.Error("Persisted sqlite session row is corrupt or unreadable", "sessionId", sessionId, "error", err)
		}
		return nil, fmt.Errorf("session '%s' could not be loaded because its persisted sqlite state is corrupt: %w", sessionId, err)
	}

	return &session, nil
}

func (s *SqliteMemoryStore) SaveSession(ctx context.Context, session Session) error {
	persistedSession := &session
	if s.redaction != nil {
		persistedSession = s.redaction.RedactSession(&session)
	}

	jsonData, err := json.Marshal(persistedSession)
	if err != nil {
		return err
	}

	updatedAt := time.Now().Unix()

	// SQLite 的 UPSERT (INSERT ... ON CONFLICT)
	query := `
		INSERT INTO sessions(id, json, updated_at)
		VALUES(?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			json=excluded.json,
			updated_at=excluded.updated_at;`

	_, err = s.db.ExecContext(ctx, query, session.Id, string(jsonData), updatedAt)
	if err != nil {
		return err
	}

	return s.syncSessionSearchIndex(ctx, persistedSession)
}

func (s *SqliteMemoryStore) backfillSessionSearchIndex() error {
	// 使用 Background 上下文，因为这是初始化阶段的后台任务
	ctx := context.Background()

	rows, err := s.db.QueryContext(ctx, "SELECT json FROM sessions;")
	if err != nil {
		return fmt.Errorf("failed to select sessions for backfill: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var jsonStr string
		if err := rows.Scan(&jsonStr); err != nil {
			return fmt.Errorf("failed to scan session json row: %w", err)
		}

		var session Session
		if err := json.Unmarshal([]byte(jsonStr), &session); err != nil {
			// 如果某一行数据损坏，记录日志并继续处理下一行，不中断整个 backfill
			if s.logger != nil {
				s.logger.Warn("Skipping corrupt session during FTS backfill", "error", err)
			}
			continue
		}

		// 复用已经写好的同步索引逻辑（内部带有事务）
		if err := s.syncSessionSearchIndex(ctx, &session); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to sync session search index during backfill", "sessionId", session.Id, "error", err)
			}
			return err
		}
	}

	return rows.Err()
}

func (s *SqliteMemoryStore) syncSessionSearchIndex(ctx context.Context, session *Session) error {
	if s.ftsEnabled {
		return s._syncSessionSearchIndex(ctx, session)
	}
	return nil
}

func (s *SqliteMemoryStore) _syncSessionSearchIndex(ctx context.Context, session *Session) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // 如果没有 commit，则回滚

	_, err = tx.ExecContext(ctx, "DELETE FROM session_turns_fts WHERE session_id = ?;", session.Id)
	if err != nil {
		return err
	}

	for _, turn := range session.History {
		err = s.insertSessionTurn(ctx, tx, session, turn.Role, turn.Content, turn.Timestamp)
		if err != nil {
			return err
		}

		for _, toolCall := range turn.ToolCalls {
			toolText := toolCall.Result
			if isBlankP(toolText) {
				toolText = &toolCall.Arguments
			}
			if !isBlankP(toolText) {
				err = s.insertSessionTurn(ctx, tx, session, "tool", *toolText, turn.Timestamp)
				if err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

func (s *SqliteMemoryStore) insertSessionTurn(ctx context.Context, tx *sql.Tx, session *Session, role, content string, timestamp time.Time) error {
	if content == "" {
		return nil
	}

	query := `
		INSERT INTO session_turns_fts(session_id, channel_id, sender_id, role, content, timestamp)
		VALUES(?, ?, ?, ?, ?, ?);`

	_, err := tx.ExecContext(ctx, query, session.Id, session.ChannelId, session.SenderId, role, content, timestamp.Unix())
	return err
}

func (s *SqliteMemoryStore) LoadNote(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", nil
	}

	var content string
	err := s.db.QueryRowContext(ctx, "SELECT content FROM notes WHERE key = ? LIMIT 1;", key).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

func (s *SqliteMemoryStore) SaveNote(ctx context.Context, key, content string) error {
	if key == "" {
		return errors.New("key must be set")
	}

	if s.redaction != nil {
		content = s.redaction.Redact(content)
	}

	updatedAt := time.Now().Unix()

	query := `
		INSERT INTO notes(key, content, updated_at)
		VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			content=excluded.content,
			updated_at=excluded.updated_at;`

	_, err := s.db.ExecContext(ctx, query, key, content, updatedAt)
	if err != nil {
		return err
	}

	if s.enableVectors && s.embeddingGenerator != nil {
		res, err := s.embeddingGenerator.Generate(ctx, []string{content}, nil)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("Failed to generate embedding for note", "key", key, "error", err)
			}
			return err
		}

		var queryEmbedding []float64
		if res.Count() > 0 {
			queryEmbedding = res.Get(0).Vector
		}

		if len(queryEmbedding) > 0 {
			blob := serializeEmbedding(queryEmbedding, false)
			_, _ = s.db.ExecContext(ctx, "UPDATE notes SET embedding = ? WHERE key = ?;", blob, key)
		}
	}

	return nil
}

func (s *SqliteMemoryStore) DeleteNote(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM notes WHERE key = ?;", key)
	return err
}

func (s *SqliteMemoryStore) ListNotesWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	query := "SELECT key FROM notes WHERE key LIKE ? || '%' ORDER BY key LIMIT 500;"
	rows, err := s.db.QueryContext(ctx, query, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		results = append(results, key)
	}
	return results, rows.Err()
}

func (s *SqliteMemoryStore) SaveBranch(ctx context.Context, branch SessionBranch) error {
	persistedBranch := &branch
	if s.redaction != nil {
		persistedBranch = s.redaction.RedactBranch(&branch)
	}

	jsonData, err := json.Marshal(persistedBranch)
	if err != nil {
		return err
	}

	updatedAt := time.Now().Unix()

	query := `
		INSERT INTO branches(branch_id, session_id, name, json, updated_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(branch_id) DO UPDATE SET
			session_id=excluded.session_id,
			name=excluded.name,
			json=excluded.json,
			updated_at=excluded.updated_at;`

	_, err = s.db.ExecContext(ctx, query, branch.BranchId, branch.SessionId, branch.Name, string(jsonData), updatedAt)
	return err
}

func (s *SqliteMemoryStore) LoadBranch(ctx context.Context, branchId string) (*SessionBranch, error) {
	if branchId == "" {
		return nil, nil
	}

	var jsonStr string
	err := s.db.QueryRowContext(ctx, "SELECT json FROM branches WHERE branch_id = ? LIMIT 1;", branchId).Scan(&jsonStr)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var branch SessionBranch
	if err := json.Unmarshal([]byte(jsonStr), &branch); err != nil {
		if s.logger != nil {
			s.logger.Error("Persisted sqlite branch row is corrupt or unreadable", "branchId", branchId, "error", err)
		}
		return nil, fmt.Errorf("branch '%s' could not be loaded because its persisted sqlite state is corrupt: %w", branchId, err)
	}

	return &branch, nil
}

func (s *SqliteMemoryStore) ListBranches(ctx context.Context, sessionId string) ([]SessionBranch, error) {
	if sessionId == "" {
		return []SessionBranch{}, nil
	}

	query := "SELECT json FROM branches WHERE session_id = ? ORDER BY updated_at DESC LIMIT 200;"
	rows, err := s.db.QueryContext(ctx, query, sessionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SessionBranch
	for rows.Next() {
		var jsonStr string
		if err := rows.Scan(&jsonStr); err != nil {
			return nil, err
		}

		var b SessionBranch
		if err := json.Unmarshal([]byte(jsonStr), &b); err == nil {
			list = append(list, b)
		}
	}
	return list, rows.Err()
}

func (s *SqliteMemoryStore) DeleteBranch(ctx context.Context, branchId string) error {
	if branchId == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM branches WHERE branch_id = ?;", branchId)
	return err
}

var _ IMemoryStore = (*SqliteMemoryStore)(nil)

const sessionLoadStripeCount = 64

type NoteIndexEntry struct {
	Key            string
	PreviewContent string
	SearchText     string
	UpdatedAt      time.Time
}
type FileMemoryStore struct {
	basePath     string
	sessionsPath string
	notesPath    string
	branchesPath string

	cacheMu      sync.RWMutex
	sessionCache map[string]*Session

	sessionLoadStripes [sessionLoadStripeCount]sync.Mutex
	noteIndexGate      sync.Mutex
	noteIndex          map[string]NoteIndexEntry
	noteIndexMu        sync.RWMutex
	noteIndexInit      int32 // 0=未初始化, 1=已初始化

	logger    *slog.Logger
	metrics   *RuntimeMetrics
	redaction IRedactionPipeline
}

// NewFileMemoryStore 构造函数
func NewFileMemoryStore(basePath string, maxCachedSessions int, logger *slog.Logger, metrics *RuntimeMetrics, redaction IRedactionPipeline) (*FileMemoryStore, error) {
	if basePath == "" {
		return nil, errors.New("basePath cannot be empty")
	}

	sessionsPath := filepath.Join(basePath, "sessions")
	notesPath := filepath.Join(basePath, "notes")
	branchesPath := filepath.Join(basePath, "branches")

	// 确保目录存在
	for _, p := range []string{sessionsPath, notesPath, branchesPath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", p, err)
		}
	}

	return &FileMemoryStore{
		basePath:     basePath,
		sessionsPath: sessionsPath,
		notesPath:    notesPath,
		branchesPath: branchesPath,
		sessionCache: make(map[string]*Session),
		noteIndex:    make(map[string]NoteIndexEntry),
		logger:       logger,
		metrics:      metrics,
		redaction:    redaction,
	}, nil
}

// ── Session 读写管理 ─────────────────────────────────────────

func (f *FileMemoryStore) GetSession(ctx context.Context, sessionId string) (*Session, error) {
	if strings.TrimSpace(sessionId) == "" {
		return nil, nil
	}

	// 1. 尝试从缓存读取
	f.cacheMu.RLock()
	cached, found := f.sessionCache[sessionId]
	f.cacheMu.RUnlock()

	if found {
		if f.metrics != nil {
			f.metrics.IncrementSessionCacheHits()
		}
		return cached, nil
	}
	if f.metrics != nil {
		f.metrics.IncrementSessionCacheMisses()
	}

	// 2. 分段锁控制并发加载
	stripe := f.resolveSessionLoadStripe(sessionId)
	stripe.Lock()
	defer stripe.Unlock()

	// 双重检查锁定 (DCL)
	f.cacheMu.RLock()
	cached, found = f.sessionCache[sessionId]
	f.cacheMu.RUnlock()
	if found {
		return cached, nil
	}

	encodedId := f.encodeKey(sessionId)
	filePath := filepath.Join(f.sessionsPath, encodedId+".json")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil
	}

	// 3. 读取并反序列化文件
	session, err := f.loadSessionFromFile(ctx, filePath)
	if err != nil {
		return nil, f.quarantineCorruptSessionFile(filePath, sessionId, err)
	}

	if session != nil {
		// 再次检查防止覆盖更新的值
		f.cacheMu.Lock()
		if canonical, exists := f.sessionCache[sessionId]; exists {
			f.cacheMu.Unlock()
			return canonical, nil
		}
		f.sessionCache[sessionId] = session
		f.cacheMu.Unlock()
	}

	return session, nil
}

func (f *FileMemoryStore) SaveSession(ctx context.Context, session Session) error {
	persistedSession := &session
	if f.redaction != nil {
		persistedSession = f.redaction.RedactSession(&session)
	}

	encodedId := f.encodeKey(session.Id)
	filePath := filepath.Join(f.sessionsPath, encodedId+".json")
	tempPath := filePath + ".tmp"

	// 原子写入安全处理
	err := func() error {
		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer file.Close()

		if func() bool {
			select {
			case <-ctx.Done():
				return true
			default:
				return false
			}
		}() {
			return ctx.Err()
		}

		encoder := json.NewEncoder(file)
		if err := encoder.Encode(persistedSession); err != nil {
			return err
		}

		return file.Sync()
	}()

	if err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	// 原子重命名
	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	// 更新缓存
	f.cacheMu.Lock()
	f.sessionCache[session.Id] = persistedSession
	f.cacheMu.Unlock()

	return nil
}

// ── Note 笔记管理 ───────────────────────────────────────────

func (f *FileMemoryStore) LoadNote(ctx context.Context, key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", nil
	}

	encodedKey := f.encodeKey(key)
	filePath := filepath.Join(f.notesPath, encodedKey+".md")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", nil
	}
	return string(content), nil
}

func (f *FileMemoryStore) SaveNote(ctx context.Context, key string, content string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("note key cannot be empty")
	}

	encodedKey := f.encodeKey(key)
	filePath := filepath.Join(f.notesPath, encodedKey+".md")
	tempPath := filePath + ".tmp"
	keyPath := filepath.Join(f.notesPath, encodedKey+".key")
	keyTempPath := keyPath + ".tmp"
	nowUtc := time.Now().UTC()

	safeContent := content
	if f.redaction != nil {
		safeContent = f.redaction.Redact(content)
	}

	// 写入 MD 临时文件
	if err := os.WriteFile(tempPath, []byte(safeContent), 0644); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	// 移动成正式文件
	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	// 写入 Key 旁路文件
	if err := f.persistOriginalNoteKey(key, keyPath, keyTempPath); err != nil {
		_ = os.Remove(keyTempPath)
		return err
	}

	f.upsertNoteIndexEntry(key, safeContent, nowUtc)
	return nil
}

func (f *FileMemoryStore) DeleteNote(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}

	encodedKey := f.encodeKey(key)
	filePath := filepath.Join(f.notesPath, encodedKey+".md")
	keyPath := filepath.Join(f.notesPath, encodedKey+".key")

	_ = os.Remove(filePath)
	_ = os.Remove(keyPath)

	f.noteIndexMu.Lock()
	delete(f.noteIndex, key)
	f.noteIndexMu.Unlock()

	return nil
}

func (f *FileMemoryStore) ListNotesWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	if err := f.ensureNoteIndexLoaded(ctx); err != nil {
		return []string{}, nil
	}

	f.noteIndexMu.RLock()
	var keys []string
	for k := range f.noteIndex {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	f.noteIndexMu.RUnlock()

	sort.Strings(keys)
	return keys, nil
}

// ── Conversation Branching 分支管理 ─────────────────────────

func (f *FileMemoryStore) SaveBranch(ctx context.Context, branch SessionBranch) error {
	persistedBranch := &branch
	if f.redaction != nil {
		persistedBranch = f.redaction.RedactBranch(&branch)
	}

	encodedId := f.encodeKey(branch.BranchId)
	filePath := filepath.Join(f.branchesPath, encodedId+".json")
	tempPath := filePath + ".tmp"

	err := func() error {
		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		if err := encoder.Encode(persistedBranch); err != nil {
			return err
		}
		return file.Sync()
	}()

	if err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

func (f *FileMemoryStore) LoadBranch(ctx context.Context, branchId string) (*SessionBranch, error) {
	if strings.TrimSpace(branchId) == "" {
		return nil, nil
	}

	encodedId := f.encodeKey(branchId)
	filePath := filepath.Join(f.branchesPath, encodedId+".json")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil
	}
	defer file.Close()

	var branch SessionBranch
	if err := json.NewDecoder(file).Decode(&branch); err != nil {
		return nil, nil
	}

	return &branch, nil
}

func (f *FileMemoryStore) ListBranches(ctx context.Context, sessionId string) ([]SessionBranch, error) {
	var results []SessionBranch

	files, err := os.ReadDir(f.branchesPath)
	if err != nil {
		return []SessionBranch{}, nil
	}

	for _, fileInfo := range files {
		if fileInfo.IsDir() || !strings.HasSuffix(fileInfo.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(f.branchesPath, fileInfo.Name())
		branch, err := f.loadBranchFromFile(filePath)
		if err != nil {
			continue // 跳过损坏的分支文件
		}

		if branch != nil && branch.SessionId == sessionId {
			results = append(results, *branch)
		}
	}

	return results, nil
}

func (f *FileMemoryStore) DeleteBranch(ctx context.Context, branchId string) error {
	if strings.TrimSpace(branchId) == "" {
		return nil
	}

	encodedId := f.encodeKey(branchId)
	filePath := filepath.Join(f.branchesPath, encodedId+".json")
	_ = os.Remove(filePath)
	return nil
}

// ── 内部辅助私有函数 (Private Helpers) ───────────────────────

func (f *FileMemoryStore) resolveSessionLoadStripe(sessionId string) *sync.Mutex {
	hash := uint32(2166136261)
	for i := 0; i < len(sessionId); i++ {
		hash ^= uint32(sessionId[i])
		hash *= 16777619
	}
	index := (hash & 0x7FFFFFFF) % sessionLoadStripeCount
	return &f.sessionLoadStripes[index]
}

func (f *FileMemoryStore) encodeKey(key string) string {
	// 大于 200 长度使用 SHA256 压缩避免超出文件系统限制
	if len(key) > 200 {
		hash := sha256.Sum256([]byte(key))
		return strings.NewReplacer("+", "-", "/", "_").Replace(strings.TrimRight(base64.StdEncoding.EncodeToString(hash[:]), "="))
	}

	return strings.NewReplacer("+", "-", "/", "_").Replace(strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(key)), "="))
}

func (f *FileMemoryStore) persistOriginalNoteKey(key, keyPath, keyTempPath string) error {
	if !f.requiresKeySidecar(key) {
		_ = os.Remove(keyPath)
		return nil
	}

	if err := os.WriteFile(keyTempPath, []byte(key), 0644); err != nil {
		return err
	}
	return os.Rename(keyTempPath, keyPath)
}

func (f *FileMemoryStore) requiresKeySidecar(key string) bool {
	// Base64 编码是可逆的，但如果长度过长使用了 SHA256 摘要（如 encodeKey 里的逻辑），则需要旁路文件存原 Key
	return len(key) > 200
}

func (f *FileMemoryStore) upsertNoteIndexEntry(key, content string, updatedAt time.Time) {
	if atomic.LoadInt32(&f.noteIndexInit) == 0 {
		return
	}

	f.noteIndexMu.Lock()
	f.noteIndex[key] = f.createNoteIndexEntry(key, content, updatedAt)
	f.noteIndexMu.Unlock()
}

func (f *FileMemoryStore) ensureNoteIndexLoaded(ctx context.Context) error {
	if atomic.LoadInt32(&f.noteIndexInit) != 0 {
		return nil
	}

	f.noteIndexGate.Lock()
	defer f.noteIndexGate.Unlock()

	if f.noteIndexInit != 0 {
		return nil
	}

	f.noteIndexMu.Lock()
	clear(f.noteIndex)
	f.noteIndexMu.Unlock()

	err := filepath.WalkDir(f.notesPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		encodedKey := strings.TrimSuffix(d.Name(), ".md")
		key := f.resolveNoteKey(encodedKey, path)

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		info, err := d.Info()
		var updatedAt time.Time
		if err == nil {
			updatedAt = info.ModTime().UTC()
		}

		f.noteIndexMu.Lock()
		f.noteIndex[key] = f.createNoteIndexEntry(key, string(contentBytes), updatedAt)
		f.noteIndexMu.Unlock()

		return nil
	})

	if err != nil {
		return err
	}

	atomic.StoreInt32(&f.noteIndexInit, 1)
	return nil
}

func (f *FileMemoryStore) createNoteIndexEntry(key, content string, updatedAt time.Time) NoteIndexEntry {
	preview := content
	if len(content) > 4096 {
		// 防止截断时切到非 UTF-8 字符的字节（Go 字符串切片是按字节的）
		runes := []rune(content)
		if len(runes) > 4096 {
			preview = string(runes[:4096]) + "…"
		}
	}

	return NoteIndexEntry{
		Key:            key,
		PreviewContent: preview,
		SearchText:     f.normalizeSearchText(fmt.Sprintf("%s\n%s", key, content)),
		UpdatedAt:      updatedAt,
	}
}

func (f *FileMemoryStore) resolveNoteKey(encodedKey, mdPath string) string {
	if len(encodedKey) <= 200 {
		// 尝试 Base64 反解
		// 由于转码时替换了符号，这里换回来
		base64Str := strings.NewReplacer("-", "+", "_", "/").Replace(encodedKey)
		// 补齐等号
		if rem := len(base64Str) % 4; rem != 0 {
			base64Str += strings.Repeat("=", 4-rem)
		}
		if decoded, err := base64.StdEncoding.DecodeString(base64Str); err == nil {
			return string(decoded)
		}
	}

	// 如果反解失败或属于长 key，尝试去读同名 .key 旁路文件
	keyPath := strings.TrimSuffix(mdPath, ".md") + ".key"
	if content, err := os.ReadFile(keyPath); err == nil {
		return string(content)
	}

	return encodedKey
}

func (f *FileMemoryStore) normalizeSearchText(text string) string {
	return strings.ToLower(text)
}

func (f *FileMemoryStore) loadSessionFromFile(_ context.Context, filePath string) (*Session, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var session Session
	if err := json.NewDecoder(file).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (f *FileMemoryStore) loadBranchFromFile(filePath string) (*SessionBranch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var branch SessionBranch
	if err := json.NewDecoder(file).Decode(&branch); err != nil {
		return nil, err
	}
	return &branch, nil
}

// 隔离被损坏的 Session 文件逻辑
func (f *FileMemoryStore) quarantineCorruptSessionFile(filePath, sessionId string, originalErr error) error {
	quarantinePath := ""
	if _, err := os.Stat(filePath); err == nil {
		timestamp := time.Now().UTC().Format("20060102150405000")
		quarantinePath = fmt.Sprintf("%s.corrupt-%s", filePath, timestamp)
		if err := os.Rename(filePath, quarantinePath); err != nil {
			if f.logger != nil {
				f.logger.Warn("Failed to quarantine corrupt session", "filePath", filePath, "sessionId", sessionId, "err", err.Error())
			}
		}
	}

	effectivePath := filePath
	if quarantinePath != "" {
		effectivePath = quarantinePath
	}

	if f.logger != nil {
		f.logger.Error("Session file is corrupt or unreadable and was quarantined", "sessionId", sessionId, "effectivePath", effectivePath, "err", originalErr)
	}

	return fmt.Errorf("session '%s' could not be loaded because its persisted state is corrupt (Path: %s). Original error: %w", sessionId, effectivePath, originalErr)
}

var _ IMemoryStore = (*FileMemoryStore)(nil)
