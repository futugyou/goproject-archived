package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const ArtifactStorageInlineThresholdBytes int = 64 * 1024

type ArtifactStorageService struct {
	db               *gorm.DB
	notifier         IArtifactCreatedNotifier
	artifactRootPath string
}

func NewArtifactStorageService(db *gorm.DB, notifier IArtifactCreatedNotifier, storageRoot string) *ArtifactStorageService {
	artifactRootPath := filepath.Join(storageRoot, "artifacts")
	os.Mkdir(artifactRootPath, 0755)
	return &ArtifactStorageService{
		db:               db,
		notifier:         notifier,
		artifactRootPath: artifactRootPath,
	}
}

func (a *ArtifactStorageService) determineArtifactType(content string, isError bool) JobRunArtifactKind {
	if isError {
		return JobRunArtifactKindError
	}

	if len(content) == 0 {
		return JobRunArtifactKindText
	}

	var trimmed = strings.TrimSpace(content)

	if strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, "```") {
		return JobRunArtifactKindMarkdown
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return JobRunArtifactKindJson
	}

	return JobRunArtifactKindText
}

func (a *ArtifactStorageService) storeContent(artifact *JobRunArtifact, content string) {
	var bytes = len([]byte(content))
	artifact.ContentSizeBytes = int64(bytes)

	if bytes <= ArtifactStorageInlineThresholdBytes {
		artifact.ContentInline = content
		artifact.ContentPath = ""
	} else {
		var jobIdStr = artifact.JobId
		var runIdStr = artifact.JobRunId

		var relativePath = filepath.Join(jobIdStr, runIdStr, fmt.Sprintf("%d_%s.txt", artifact.Sequence, artifact.Id))
		var fullPath = filepath.Join(a.artifactRootPath, relativePath)
		dir := filepath.Dir(fullPath)
		os.MkdirAll(dir, 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
		artifact.ContentInline = ""
		artifact.ContentPath = relativePath
	}
}

func (a *ArtifactStorageService) CreateArtifactFromJobRun(ctx context.Context, run *JobRun) (*JobRunArtifact, error) {
	content := run.Result
	if len(run.Error) > 0 {
		content = run.Error
	}
	var artifactType = a.determineArtifactType(content, len(run.Error) > 0)
	var title = ""
	if len(run.Error) > 0 {
		title = "Execution Error"
	}

	var artifact = &JobRunArtifact{
		Id:           uuid.NewString(),
		JobRunId:     run.Id,
		JobId:        run.JobId,
		Sequence:     0,
		ArtifactType: artifactType,
		Title:        title,
		CreatedAt:    time.Now(),
	}

	a.storeContent(artifact, content)

	a.db.Save(&artifact)

	if a.notifier != nil {
		a.notifier.NotifyArtifactCreated(artifact.JobId, artifact.JobRunId, artifact.Id)
	}

	return artifact, nil
}

func (a *ArtifactStorageService) CreateArtifact(ctx context.Context, jobId, jobRunId string, kindType JobRunArtifactKind, title, content string, sequence int) (*JobRunArtifact, error) {
	var artifact = &JobRunArtifact{
		Id:           uuid.NewString(),
		JobRunId:     jobRunId,
		JobId:        jobId,
		Sequence:     sequence,
		ArtifactType: kindType,
		Title:        title,
		CreatedAt:    time.Now(),
	}

	a.storeContent(artifact, content)
	a.db.Save(&artifact)
	a.db.Save(&artifact)

	if a.notifier != nil {
		a.notifier.NotifyArtifactCreated(artifact.JobId, artifact.JobRunId, artifact.Id)
	}

	return artifact, nil
}

func (a *ArtifactStorageService) GetArtifactContent(ctx context.Context, artifact *JobRunArtifact) (string, error) {
	if len(artifact.ContentInline) > 0 {
		return artifact.ContentInline, nil
	}

	if len(artifact.ContentPath) > 0 {
		var fullPath = filepath.Join(a.artifactRootPath, artifact.ContentPath)
		_, err := os.Stat(fullPath)
		if err == nil || !os.IsNotExist(err) {
			if d, err := os.ReadFile(fullPath); err != nil {
				return "", err
			} else {
				return string(d), nil
			}
		}
	}

	return "", nil
}

func (s *ArtifactStorageService) CleanupOldArtifacts(ctx context.Context, maxRunsPerJob int, maxAgeDays int) (int64, error) {
	cutoffDate := time.Now().UTC().AddDate(0, 0, -maxAgeDays)
	var deletedCount int64

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subQueryQty := tx.Model(&JobRun{}).
			Select("id").
			Where("id IN (SELECT id FROM (SELECT id, ROW_NUMBER() OVER (PARTITION BY job_id ORDER BY started_at DESC) as row_num FROM job_runs) t WHERE t.row_num > ?)", maxRunsPerJob)

		subQueryAge := tx.Model(&JobRun{}).
			Select("id").
			Where("started_at < ?", cutoffDate)

		var artifactsToDelete []JobRunArtifact
		err := tx.Model(&JobRunArtifact{}).
			Where("job_run_id IN (?) OR job_run_id IN (?)", subQueryQty, subQueryAge).
			Find(&artifactsToDelete).Error
		if err != nil {
			return err
		}

		if len(artifactsToDelete) == 0 {
			return nil
		}

		for _, artifact := range artifactsToDelete {
			if artifact.ContentPath != "" {
				fullPath := filepath.Join(s.artifactRootPath, artifact.ContentPath)
				if _, err := os.Stat(fullPath); err == nil { // 文件存在
					if err := os.Remove(fullPath); err != nil {
						fmt.Printf("Failed to delete artifact file: %s, err: %v\n", fullPath, err)
					}
				}
			}
		}

		var idsToDel []string
		for _, a := range artifactsToDelete {
			idsToDel = append(idsToDel, a.Id)
		}

		result := tx.Delete(&JobRunArtifact{}, idsToDel)
		if result.Error != nil {
			return result.Error
		}

		deletedCount = result.RowsAffected
		return nil
	})

	return deletedCount, err
}
