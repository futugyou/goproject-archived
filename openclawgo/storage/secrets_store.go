package storage

import (
	"context"
	"errors"
	"time"
)

type SecretSummary struct {
	Name        string
	Description string
	UpdatedAt   time.Time
}

func SecretsStoreSetBundle(ctx context.Context, store ISecretsStore, secrets map[string]string) error {
	if len(secrets) == 0 {
		return errors.New("secret bundle cannot be empty")
	}

	for name, value := range secrets {
		if err := store.Set(ctx, name, value, ""); err != nil {
			return err
		}
	}

	return nil
}

type ISecretsStore interface {
	Get(ctx context.Context, name string, version int) (string, error)
	List(ctx context.Context) ([]SecretSummary, error)
	Set(ctx context.Context, name, value, description string) error
	ListVersions(ctx context.Context, name string) ([]int, error)
	Rotate(ctx context.Context, name, newValue string) error
	Delete(ctx context.Context, name string) (bool, error)
	Recover(ctx context.Context, name string) (bool, error)
	Purge(ctx context.Context, name string) (bool, error)
}
