package storage

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const (
	ProviderTypeDefaultsOllamaProviderType = "ollama"
	ProviderTypeDefaultsOllamaEndpoint     = "http://localhost:11434"
	ProviderTypeDefaultsOllamaModel        = "gemma4:e2b"
	ProviderTypeDefaultsOllamaDisplayName  = "Ollama (Local)"
	ProviderTypeDefaultsOllamaTemperature  = 0.7
	ProviderTypeDefaultsOllamaMaxTokens    = 4096

	ProviderTypeDefaultsAzureOpenAIProviderType   = "azure-openai"
	ProviderTypeDefaultsAzureOpenAIDeploymentName = "gpt-5-mini"
	ProviderTypeDefaultsAzureOpenAIAuthMode       = "api-key"
	ProviderTypeDefaultsAzureOpenAIDisplayName    = "Azure OpenAI"

	ProviderTypeDefaultsGitHubCopilotProviderType = "github-copilot"
	ProviderTypeDefaultsGitHubCopilotModel        = "gpt-5-mini"
	ProviderTypeDefaultsGitHubCopilotDisplayName  = "GitHub Copilot SDK"

	ProviderTypeDefaultsFoundryProviderType = "foundry"
	ProviderTypeDefaultsFoundryModel        = "gpt-4o-mini"
	ProviderTypeDefaultsFoundryAuthMode     = "api-key"
	ProviderTypeDefaultsFoundryDisplayName  = "Microsoft Foundry"

	ProviderTypeDefaultsFoundryLocalProviderType = "foundry-local"
	ProviderTypeDefaultsFoundryLocalModel        = "phi-4"
	ProviderTypeDefaultsFoundryLocalDisplayName  = "Foundry Local"

	ProviderTypeDefaultsLMStudioProviderType = "lm-studio"
	ProviderTypeDefaultsLMStudioEndpoint     = "http://localhost:1234"
	ProviderTypeDefaultsLMStudioDisplayName  = "LM Studio"
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
			ProviderType: ProviderTypeDefaultsOllamaProviderType,
			DisplayName:  ProviderTypeDefaultsOllamaDisplayName,
			Endpoint:     ProviderTypeDefaultsOllamaEndpoint,
			Model:        ProviderTypeDefaultsOllamaModel,
			IsSupported:  false,
		},
		{
			Name:           "azure-openai-default",
			ProviderType:   ProviderTypeDefaultsAzureOpenAIProviderType,
			DisplayName:    ProviderTypeDefaultsAzureOpenAIDisplayName,
			DeploymentName: ProviderTypeDefaultsAzureOpenAIDeploymentName,
			AuthMode:       ProviderTypeDefaultsAzureOpenAIAuthMode,
			IsSupported:    false,
		},
		{
			Name:         "github-copilot-default",
			ProviderType: ProviderTypeDefaultsGitHubCopilotProviderType,
			DisplayName:  ProviderTypeDefaultsGitHubCopilotDisplayName,
			IsSupported:  false,
		},
		{
			Name:         "foundry-default",
			ProviderType: ProviderTypeDefaultsFoundryProviderType,
			DisplayName:  ProviderTypeDefaultsFoundryDisplayName,
			Model:        ProviderTypeDefaultsFoundryModel,
			AuthMode:     ProviderTypeDefaultsFoundryAuthMode,
			IsSupported:  false,
		},
		{
			Name:         "foundry-local-default",
			ProviderType: ProviderTypeDefaultsFoundryLocalProviderType,
			DisplayName:  ProviderTypeDefaultsFoundryLocalDisplayName,
			Model:        ProviderTypeDefaultsFoundryLocalModel,
			IsSupported:  false,
		},
		{
			Name:         "lm-studio-default",
			ProviderType: ProviderTypeDefaultsLMStudioProviderType,
			DisplayName:  ProviderTypeDefaultsLMStudioDisplayName,
			Endpoint:     ProviderTypeDefaultsLMStudioEndpoint,
			IsSupported:  false,
		},
	}

	return m.db.CreateInBatches(defaults, len(defaults)).Error
}
