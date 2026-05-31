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

var _ ISecretsStore = (*ChainedSecretsStore)(nil)

type ChainedSecretsStore struct {
	stores []ISecretsStore
}

// Delete implements [ISecretsStore].
func (c *ChainedSecretsStore) Delete(ctx context.Context, name string) (bool, error) {
	for _, store := range c.stores {
		if _, err := store.Delete(ctx, name); err != nil {
			return false, err
		}
	}

	return true, nil
}

// Get implements [ISecretsStore].
func (c *ChainedSecretsStore) Get(ctx context.Context, name string, version int) (string, error) {
	for _, store := range c.stores {
		if s, err := store.Get(ctx, name, version); err == nil {
			return s, err
		}
	}

	return "", nil
}

// List implements [ISecretsStore].
func (c *ChainedSecretsStore) List(ctx context.Context) ([]SecretSummary, error) {
	result := []SecretSummary{}
	for _, store := range c.stores {
		if s, err := store.List(ctx); err != nil {
			result = append(result, s...)
		}
	}

	return result, nil
}

// ListVersions implements [ISecretsStore].
func (c *ChainedSecretsStore) ListVersions(ctx context.Context, name string) ([]int, error) {
	for _, store := range c.stores {
		if s, err := store.ListVersions(ctx, name); err == nil {
			return s, err
		}
	}

	return []int{}, nil
}

// Purge implements [ISecretsStore].
func (c *ChainedSecretsStore) Purge(ctx context.Context, name string) (bool, error) {
	for _, store := range c.stores {
		if _, err := store.Purge(ctx, name); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Recover implements [ISecretsStore].
func (c *ChainedSecretsStore) Recover(ctx context.Context, name string) (bool, error) {
	for _, store := range c.stores {
		if _, err := store.Recover(ctx, name); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Rotate implements [ISecretsStore].
func (c *ChainedSecretsStore) Rotate(ctx context.Context, name string, newValue string) error {
	for _, store := range c.stores {
		if err := store.Rotate(ctx, name, newValue); err != nil {
			return err
		}
	}
	return nil
}

// Set implements [ISecretsStore].
func (c *ChainedSecretsStore) Set(ctx context.Context, name string, value string, description string) error {
	for _, store := range c.stores {
		if err := store.Set(ctx, name, value, description); err != nil {
			return err
		}
	}
	return nil
}
