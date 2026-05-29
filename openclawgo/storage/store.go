package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/futugyou/openclawgo/models"
	"gorm.io/gorm"
)

type AgentInvocationLogger struct {
	db *gorm.DB
}

func NewAgentInvocationLogger(db *gorm.DB) *AgentInvocationLogger {
	return &AgentInvocationLogger{
		db: db,
	}
}

func (p *AgentInvocationLogger) Record(ctx context.Context, entry *AgentInvocationLog) error {
	return gorm.G[AgentInvocationLog](p.db).Create(ctx, entry)
}

type JobStatusChangeRecorder struct {
	db *gorm.DB
}

func NewJobStatusChangeRecorder(db *gorm.DB) *JobStatusChangeRecorder {
	return &JobStatusChangeRecorder{
		db: db,
	}
}

func (p *JobStatusChangeRecorder) RecordTransition(ctx context.Context, job ScheduledJob, to JobStatus, reason, changedBy string) error {
	var from = job.Status
	if from == to {
		return nil
	}

	if !IsJobStatusTransitionAllowed(from, to) {
		return fmt.Errorf("Job state transition '%d' → '%d' is not allowed.", from, to)
	}

	job.Status = to

	return gorm.G[JobDefinitionStateChange](p.db).Create(ctx, &JobDefinitionStateChange{
		JobId:      job.Id,
		FromStatus: from,
		ToStatus:   to,
		Reason:     reason,
		ChangedBy:  changedBy,
		ChangedAt:  time.Now(),
	})
}

type AgentProfileStore struct {
	db *gorm.DB
}

func NewAgentProfileStore(db *gorm.DB) *AgentProfileStore {
	return &AgentProfileStore{
		db: db,
	}
}

func (p *AgentProfileStore) Get(ctx context.Context, name string) (*models.AgentProfile, error) {
	entity, err := gorm.G[*AgentProfileEntity](p.db).Where("name = ?", name).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return p.toModel(entity), nil
}

func (p *AgentProfileStore) Save(ctx context.Context, profile *models.AgentProfile) error {
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if profile.IsDefault {
			if _, err := gorm.G[AgentProfileEntity](tx).
				Where("is_default = ? AND name != ?", true, profile.Name).
				Update(ctx, "is_default", false); err != nil {
				return err
			}
		}

		entity := p.toEntity(profile)
		updateData := *entity
		updateData.UpdatedAt = time.Now().UTC()

		return tx.Where(AgentProfileEntity{Name: entity.Name}).
			Attrs(models.AgentProfile{UpdatedAt: entity.UpdatedAt}).
			Assign(updateData).
			FirstOrCreate(entity).Error
	})
}

func (p *AgentProfileStore) toModel(entity *AgentProfileEntity) *models.AgentProfile {
	return &models.AgentProfile{
		Name:                entity.Name,
		DisplayName:         entity.DisplayName,
		Provider:            entity.Provider,
		Model:               entity.Model,
		Endpoint:            entity.Endpoint,
		ApiKey:              entity.ApiKey,
		DeploymentName:      entity.DeploymentName,
		AuthMode:            entity.AuthMode,
		Instructions:        entity.Instructions,
		EnabledTools:        entity.EnabledTools,
		Temperature:         entity.Temperature,
		MaxTokens:           entity.MaxTokens,
		IsDefault:           entity.IsDefault,
		RetrievalLevel:      entity.RetrievalLevel,
		Kind:                entity.Kind,
		RequireToolApproval: entity.RequireToolApproval,
		IsEnabled:           entity.IsEnabled,
		LastTestedAt:        entity.LastTestedAt,
		LastTestSucceeded:   entity.LastTestSucceeded,
		LastTestError:       entity.LastTestError,
		CreatedAt:           entity.CreatedAt,
		UpdatedAt:           entity.UpdatedAt,
	}
}

func (p *AgentProfileStore) toEntity(entity *models.AgentProfile) *AgentProfileEntity {
	return &AgentProfileEntity{
		Name:                entity.Name,
		DisplayName:         entity.DisplayName,
		Provider:            entity.Provider,
		Model:               entity.Model,
		Endpoint:            entity.Endpoint,
		ApiKey:              entity.ApiKey,
		DeploymentName:      entity.DeploymentName,
		AuthMode:            entity.AuthMode,
		Instructions:        entity.Instructions,
		EnabledTools:        entity.EnabledTools,
		Temperature:         entity.Temperature,
		MaxTokens:           entity.MaxTokens,
		IsDefault:           entity.IsDefault,
		RetrievalLevel:      entity.RetrievalLevel,
		Kind:                entity.Kind,
		RequireToolApproval: entity.RequireToolApproval,
		IsEnabled:           entity.IsEnabled,
		LastTestedAt:        entity.LastTestedAt,
		LastTestSucceeded:   entity.LastTestSucceeded,
		LastTestError:       entity.LastTestError,
		CreatedAt:           entity.CreatedAt,
		UpdatedAt:           entity.UpdatedAt,
	}
}
