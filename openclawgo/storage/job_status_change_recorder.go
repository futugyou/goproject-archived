package storage

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

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
