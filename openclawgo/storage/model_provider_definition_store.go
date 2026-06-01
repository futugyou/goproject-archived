package storage

import (
	"context"
	"errors"

	"github.com/futugyou/openclawgo/models"
	"gorm.io/gorm"
)

type IModelProviderDefinitionStore interface {
	Get(ctx context.Context, name string) (*ModelProviderDefinition, error)
	List(ctx context.Context) ([]ModelProviderDefinition, error)
	ListByType(ctx context.Context, providerType string) ([]ModelProviderDefinition, error)
	Save(ctx context.Context, definition *ModelProviderDefinition) error
	Delete(ctx context.Context, name string) error
	SeedDefaults(ctx context.Context) error
}

var _ IModelProviderDefinitionStore = (*ModelProviderDefinitionStore)(nil)

type ModelProviderDefinitionStore struct {
	db *gorm.DB
}

func NewModelProviderDefinitionStore(db *gorm.DB) *ModelProviderDefinitionStore {
	return &ModelProviderDefinitionStore{db: db}
}

// Delete implements [IModelProviderDefinitionStore].
func (m *ModelProviderDefinitionStore) Delete(ctx context.Context, name string) error {
	_, err := gorm.G[ModelProviderDefinition](m.db).Where("name = ?", name).Delete(ctx)
	return err
}

// Get implements [IModelProviderDefinitionStore].
func (m *ModelProviderDefinitionStore) Get(ctx context.Context, name string) (*ModelProviderDefinition, error) {
	d, err := gorm.G[ModelProviderDefinition](m.db).Where("name = ?", name).First(ctx)
	if err != nil {
		return nil, err
	}
	if len(d.Name) == 0 {
		return nil, errors.New("no data found")
	}
	return &d, nil
}

// List implements [IModelProviderDefinitionStore].
func (m *ModelProviderDefinitionStore) List(ctx context.Context) ([]ModelProviderDefinition, error) {
	return gorm.G[ModelProviderDefinition](m.db).Find(ctx)
}

// ListByType implements [IModelProviderDefinitionStore].
func (m *ModelProviderDefinitionStore) ListByType(ctx context.Context, providerType string) ([]ModelProviderDefinition, error) {
	return gorm.G[ModelProviderDefinition](m.db).Where("provider_type = ?", providerType).Find(ctx)
}

// Save implements [IModelProviderDefinitionStore].
func (m *ModelProviderDefinitionStore) Save(ctx context.Context, definition *ModelProviderDefinition) error {
	return m.db.Where(ModelProviderDefinition{Name: definition.Name}).
		Assign(definition).
		FirstOrCreate(definition).Error
}

// SeedDefaults implements [IModelProviderDefinitionStore].
func (m *ModelProviderDefinitionStore) SeedDefaults(ctx context.Context) error {
	datas, _ := gorm.G[ModelProviderDefinition](m.db).Find(ctx)
	if len(datas) > 0 {
		return nil
	}

	var defaults = []ModelProviderDefinition{
		{
			Name:         "ollama-default",
			ProviderType: models.OllamaProviderType,
			DisplayName:  models.OllamaDisplayName,
			Endpoint:     models.OllamaEndpoint,
			Model:        models.OllamaModel,
			IsSupported:  false,
		},
		{
			Name:         "openai-default",
			ProviderType: models.OpenAIProviderType,
			DisplayName:  models.OpenAIDisplayName,
			AuthMode:     models.OpenAIAuthMode,
			IsSupported:  false,
		},
	}

	return m.db.CreateInBatches(defaults, len(defaults)).Error
}
