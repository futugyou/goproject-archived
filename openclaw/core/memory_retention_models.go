package core

import (
	"time"
)

const (
	RetentionSweepRequestDefaultArchivePath          = "./memory/archive"
	RetentionSweepRequestDefaultArchiveRetentionDays = 30
	RetentionSweepRequestDefaultMaxItems             = 1000
)

type RetentionSweepRequest struct {
	NowUtc                  time.Time `json:"now_utc"`
	SessionExpiresBeforeUtc time.Time `json:"session_expires_before_utc"`
	BranchExpiresBeforeUtc  time.Time `json:"branch_expires_before_utc"`
	ArchiveEnabled          bool      `json:"archive_enabled"`
	ArchivePath             string    `json:"archive_path"`
	ArchiveRetentionDays    int       `json:"archive_retention_days"`
	MaxItems                int       `json:"max_items"`
	DryRun                  bool      `json:"dry_run"`
}

func DefaultRetentionSweepRequest() RetentionSweepRequest {
	return RetentionSweepRequest{
		NowUtc:               time.Now().UTC(),
		ArchiveEnabled:       true,
		ArchivePath:          RetentionSweepRequestDefaultArchivePath,
		ArchiveRetentionDays: RetentionSweepRequestDefaultArchiveRetentionDays,
		MaxItems:             RetentionSweepRequestDefaultMaxItems,
		DryRun:               false,
	}
}

// ---

type RetentionSweepResult struct {
	StartedAtUtc               time.Time `json:"started_at_utc"`
	CompletedAtUtc             time.Time `json:"completed_at_utc"`
	DryRun                     bool      `json:"dry_run"`
	MaxItemsLimitReached       bool      `json:"max_items_limit_reached"`
	EligibleSessions           int       `json:"eligible_sessions"`
	EligibleBranches           int       `json:"eligible_branches"`
	ArchivedSessions           int       `json:"archived_sessions"`
	ArchivedBranches           int       `json:"archived_branches"`
	DeletedSessions            int       `json:"deleted_sessions"`
	DeletedBranches            int       `json:"deleted_branches"`
	SkippedProtectedSessions   int       `json:"skipped_protected_sessions"`
	SkippedCorruptSessionItems int       `json:"skipped_corrupt_session_items"`
	SkippedCorruptBranchItems  int       `json:"skipped_corrupt_branch_items"`
	ArchivePurgedFiles         int       `json:"archive_purged_files"`
	ArchivePurgeErrors         int       `json:"archive_purge_errors"`
	Errors                     []string  `json:"errors"`
}

func DefaultRetentionSweepResult() RetentionSweepResult {
	now := time.Now().UTC()
	return RetentionSweepResult{
		StartedAtUtc:   now,
		CompletedAtUtc: now,
		Errors:         []string{},
	}
}

func (r RetentionSweepResult) TotalArchived() int {
	return r.ArchivedSessions + r.ArchivedBranches
}

func (r RetentionSweepResult) TotalDeleted() int {
	return r.DeletedSessions + r.DeletedBranches
}

func (r RetentionSweepResult) TotalEligible() int {
	return r.EligibleSessions + r.EligibleBranches
}

func (r RetentionSweepResult) DurationMs() int64 {
	duration := r.CompletedAtUtc.Sub(r.StartedAtUtc)
	ms := duration.Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

// ---

type RetentionStoreStats struct {
	Backend           string `json:"backend"`
	PersistedSessions int64  `json:"persisted_sessions"`
	PersistedBranches int64  `json:"persisted_branches"`
}

// ---

type RetentionRunStatus struct {
	Enabled                bool                  `json:"enabled"`
	StoreSupportsRetention bool                  `json:"store_supports_retention"`
	IsRunning              bool                  `json:"is_running"`
	LastRunStartedAtUtc    *time.Time            `json:"last_run_started_at_utc,omitempty"`
	LastRunCompletedAtUtc  *time.Time            `json:"last_run_completed_at_utc,omitempty"`
	LastRunDurationMs      int64                 `json:"last_run_duration_ms"`
	LastRunSucceeded       bool                  `json:"last_run_succeeded"`
	LastError              string                `json:"last_error,omitempty"`
	TotalRuns              int64                 `json:"total_runs"`
	TotalSweepErrors       int64                 `json:"total_sweep_errors"`
	TotalArchivedItems     int64                 `json:"total_archived_items"`
	TotalDeletedItems      int64                 `json:"total_deleted_items"`
	LastResult             *RetentionSweepResult `json:"last_result,omitempty"`
	StoreStats             *RetentionStoreStats  `json:"store_stats,omitempty"`
}

type MemoryNoteHit struct {
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
	Score     float64   `json:"score"`
}

type MemoryNoteCatalogEntry struct {
	Key            string    `json:"key"`
	PreviewContent string    `json:"preview_content"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Note struct {
	Key       string `gorm:"primaryKey"`
	Content   string `gorm:"type:text"`
	UpdatedAt int64  `gorm:"index"`
	Embedding string `gorm:"type:vector(1536)"`
}
