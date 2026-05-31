package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

var _ ISecretsStore = (*SecretsStore)(nil)

type SecretsStore struct {
	db *gorm.DB
}

func NewSecretsStore(db *gorm.DB) *SecretsStore {
	return &SecretsStore{
		db: db,
	}
}

// Delete implements [ISecretsStore].
func (s *SecretsStore) Delete(ctx context.Context, name string) (bool, error) {
	now := time.Now().UTC()
	purgeAfter := now.AddDate(0, 0, 30)
	c, err := gorm.G[SecretEntity](s.db).Where("deleted_at is not null AND name = ?", name).
		Updates(ctx, SecretEntity{
			DeletedAt:  &now,
			PurgeAfter: &purgeAfter,
			UpdatedAt:  now,
		})
	return c > 0, err
}

// Get implements [ISecretsStore].
func (s *SecretsStore) Get(ctx context.Context, name string, version int) (string, error) {
	var entity SecretEntity
	var err error
	if version == -1 {
		entity, err = gorm.G[SecretEntity](s.db).Where("deleted_at is null AND name = ?", name).Order("version desc").First(ctx)
	} else {
		entity, err = gorm.G[SecretEntity](s.db).Where("deleted_at is null AND version = ? AND name = ?", version, name).First(ctx)
	}

	if err != nil {
		return "", err
	}
	if len(entity.Name) == 0 {
		return "", errors.New("no secret found")
	}
	return entity.EncryptedValue, nil
}

// List implements [ISecretsStore].
func (s *SecretsStore) List(ctx context.Context) ([]SecretSummary, error) {
	result := []SecretSummary{}
	err := s.db.WithContext(ctx).
		Model(&SecretEntity{}).
		Select("name", "description", "update_at").
		Find(&result).Error
	return result, err
}

// ListVersions implements [ISecretsStore].
func (s *SecretsStore) ListVersions(ctx context.Context, name string) ([]int, error) {
	result := []int{}
	err := s.db.WithContext(ctx).
		Model(&SecretEntity{}).
		Select("version").
		Find(&result).Error
	return result, err
}

// Purge implements [ISecretsStore].
func (s *SecretsStore) Purge(ctx context.Context, name string) (bool, error) {
	return true, s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := gorm.G[SecretEntity](tx).Where("name = ?", name).Delete(ctx); err != nil {
			return err
		}
		if _, err := gorm.G[SecretVersionEntity](tx).Where("secret_name = ?", name).Delete(ctx); err != nil {
			return err
		}
		return nil
	})
}

// Recover implements [ISecretsStore].
func (s *SecretsStore) Recover(ctx context.Context, name string) (bool, error) {
	c, err := gorm.G[SecretEntity](s.db).Where("deleted_at is not null AND name = ?", name).
		Updates(ctx, SecretEntity{
			DeletedAt:  nil,
			PurgeAfter: nil,
			UpdatedAt:  time.Now().UTC(),
		})
	return c > 0, err
}

// Rotate implements [ISecretsStore].
func (s *SecretsStore) Rotate(ctx context.Context, name string, newValue string) error {
	existing, err := gorm.G[SecretEntity](s.db).Where("name = ?", name).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if len(existing.Name) == 0 {
		return s.Set(ctx, name, newValue, "")
	}

	if existing.DeletedAt != nil {
		return errors.New("cannot rotate a soft-deleted secret. recover it first")
	}

	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		secret := SecretEntity{
			EncryptedValue: newValue,
			UpdatedAt:      now,
		}

		_, err := gorm.G[SecretEntity](tx).Where("id = ?", 111).Updates(ctx, secret)
		if err != nil {
			return err
		}

		version := 1
		err = tx.Model(&SecretVersionEntity{}).
			Where("secret_name = ?", name).
			Select("max(version)").
			First(&version).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else {
			version = version + 1
		}

		secretVersion := &SecretVersionEntity{
			Id:             uuid.NewString(),
			SecretName:     name,
			EncryptedValue: newValue,
			CreatedAt:      now,
			IsCurrent:      true,
			Version:        version,
		}

		if err := tx.Save(secretVersion).Error; err != nil {
			return err
		}

		if _, err := gorm.G[SecretVersionEntity](tx).
			Where("secret_name = ? AND version < ?", name, version).
			Updates(ctx, SecretVersionEntity{IsCurrent: false, SupersededAt: nil}); err != nil {
			return err
		}
		return nil
	})
}

// Set implements [ISecretsStore].
func (s *SecretsStore) Set(ctx context.Context, name string, value string, description string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		secret := &SecretEntity{
			Name:           name,
			EncryptedValue: value,
			Description:    description,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		updateData := map[string]any{}
		updateData["updated_at"] = time.Now().UTC()
		updateData["deleted_at"] = nil
		updateData["purge_after"] = nil
		if len(description) > 0 {
			updateData["description"] = description
		}

		err := tx.Where(SecretEntity{Name: name}).
			Assign(updateData).
			FirstOrCreate(secret).Error

		version := 1
		err = tx.Model(&SecretVersionEntity{}).
			Where("secret_name = ?", name).
			Select("max(version)").
			First(&version).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else {
			version = version + 1
		}

		secretVersion := &SecretVersionEntity{
			Id:             uuid.NewString(),
			SecretName:     name,
			EncryptedValue: value,
			CreatedAt:      now,
			IsCurrent:      true,
			Version:        version,
		}
		if err := tx.Save(secretVersion).Error; err != nil {
			return err
		}

		if _, err := gorm.G[SecretVersionEntity](tx).
			Where("secret_name = ? AND version < ?", name, version).
			Updates(ctx, SecretVersionEntity{IsCurrent: false, SupersededAt: nil}); err != nil {
			return err
		}
		return nil
	})

}
