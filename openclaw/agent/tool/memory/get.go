package memory

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/futugyou/openclaw/core"
)

type MemoryGetTool struct {
	store core.IMemoryStore
}

func NewMemoryGetTool(store core.IMemoryStore) *MemoryGetTool {
	return &MemoryGetTool{store: store}
}

func (a *MemoryGetTool) Name() string {
	return "memory_get"
}

func (a *MemoryGetTool) Description() string {
	return "Retrieve a specific memory note by its key. Returns the note content or an error if not found."
}

func (a *MemoryGetTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"key": {
			"type": "string",
			"description": "The memory note key to retrieve"
		}
	},
	"required": ["key"]
}
	`
}

func (a *MemoryGetTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	if argumentsJson == "" {
		return "", errors.New("Error: arguments payload is empty.")
	}

	var model struct {
		Key string `json:"key"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return "", err
	}

	if model.Key == "" {
		return "", errors.New("Error: key is required.")
	}

	if err := core.Sanitizer.CheckMemoryKey(model.Key); err != nil {
		return "", err
	}

	content, err := a.store.LoadNote(ctx, model.Key)
	if err != nil {
		return "", err
	}
	return content, nil
}
