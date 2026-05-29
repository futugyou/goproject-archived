package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ArtifactRetentionWork struct {
	artifactStorage *ArtifactStorageService
	maxRunsPerJob   int
	maxAgeDays      int
}

func NewArtifactRetentionWork(artifactStorage *ArtifactStorageService, maxRunsPerJob int, maxAgeDays int) *ArtifactRetentionWork {
	return &ArtifactRetentionWork{artifactStorage: artifactStorage, maxRunsPerJob: maxRunsPerJob, maxAgeDays: maxAgeDays}
}

func (w *ArtifactRetentionWork) Start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Go(func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fmt.Println("ArtifactRetentionWork stopped")
				return
			case <-ticker.C:
				fmt.Println("ArtifactRetentionWork working")
				deleted, err := w.artifactStorage.CleanupOldArtifacts(ctx, w.maxRunsPerJob, w.maxAgeDays)
				if err != nil {
					fmt.Println(err.Error())
				} else if deleted > 0 {
					fmt.Printf("retention cleanup deleted %d artifacts", deleted)
				}
			}
		}
	})
}
