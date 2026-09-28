package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/futugyou/openclaw/core"
)

type MemoryNoteTool struct {
	store core.IMemoryStore
}

func NewMemoryNoteTool(store core.IMemoryStore) *MemoryNoteTool {
	return &MemoryNoteTool{store: store}
}

func (a *MemoryNoteTool) Name() string {
	return "memory"
}

func (a *MemoryNoteTool) Description() string {
	return "Read or write persistent memory notes. Use to remember user preferences, project context, and important information across sessions."
}

func (a *MemoryNoteTool) ParameterSchema() string {
	return `
{
	"type": "object",
	"properties": {
		"action": {
			"type": "string",
			"enum": ["read", "write"]
		},
		"key": {
			"type": "string",
			"description": "Note identifier"
		},
		"content": {
			"type": "string",
			"description": "Content to write (only for write action)"
		}
	},
	"required": ["action", "key"]
} `
}

func (a *MemoryNoteTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	if argumentsJson == "" {
		return "", errors.New("Error: arguments payload is empty.")
	}

	var model struct {
		Key     string `json:"key"`
		Action  string `json:"action"`
		Content string `json:"content"`
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

	switch model.Action {
	case "read":
		content, err := a.store.LoadNote(ctx, model.Key)
		if err != nil {
			return "", err
		}
		return content, nil
	case "write":
		if err := a.store.SaveNote(ctx, model.Key, model.Content); err != nil {
			return "", err
		}
		return "", fmt.Errorf("Saved note: %s", model.Key)
	default:
		return "", fmt.Errorf("Unknown action: %s", model.Action)
	}

}
