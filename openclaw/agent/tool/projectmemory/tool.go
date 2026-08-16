package projectmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type ProjectMemoryTool struct {
	memory    core.IMemoryStore
	projectid string
}

func New(memory core.IMemoryStore, projectid string) *ProjectMemoryTool {
	return &ProjectMemoryTool{memory: memory, projectid: projectid}
}

func (a *ProjectMemoryTool) Name() string {
	return "project_memory"
}

func (a *ProjectMemoryTool) Description() string {
	return "Save or recall persistent project-level context. Use 'save' to store architecture decisions, conventions, and preferences. Use 'load' to recall them. Use 'list' to see all saved keys. Use 'delete' to remove a key. This memory persists across conversations."
}

func (a *ProjectMemoryTool) ParameterSchema() string {
	return `
	{
          "type": "object",
          "properties": {
            "action": {
              "type": "string",
              "enum": ["save", "load", "list", "delete"],
              "description": "The action to perform"
            },
            "key": {
              "type": "string",
              "description": "The memory key (required for save/load/delete)"
            },
            "content": {
              "type": "string",
              "description": "The content to save (required for save)"
            }
          },
          "required": ["action"]
        }
    `
}

type ProjectMemoryModel struct {
	Action  string `json:"action"`
	Key     string `json:"key"`
	Content string `json:"content"`
}

func (a *ProjectMemoryTool) Execute(ctx context.Context, argumentsJson string) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	var model ProjectMemoryModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if model.Action == "" {
		return "Error: action is empty."
	}

	switch model.Action {
	case "save":
		return a.save(ctx, model)
	case "load":
		return a.load(ctx, model)
	case "list":
		return a.list(ctx, model)
	case "delete":
		return a.delete(ctx, model)
	default:
		return "Error: Unknown action. Use 'save', 'load', 'list', or 'delete'."
	}
}

func (a *ProjectMemoryTool) save(ctx context.Context, model ProjectMemoryModel) string {
	if model.Key == "" {
		return "Error: 'key' is required for save."
	}
	if model.Content == "" {
		return "Error: 'content' is required for save."
	}
	fullKey := a.projectKey(model.Key)
	if err := a.memory.SaveNote(ctx, fullKey, model.Content); err != nil {
		return err.Error()
	}

	return fmt.Sprintf("Saved project memory: %s", model.Key)
}

func (a *ProjectMemoryTool) load(ctx context.Context, model ProjectMemoryModel) string {
	if model.Key == "" {
		return "Error: 'key' is required"
	}
	fullKey := a.projectKey(model.Key)
	result, err := a.memory.LoadNote(ctx, fullKey)
	if err != nil {
		return err.Error()
	}
	if result != "" {
		return result
	}
	return fmt.Sprintf("No project memory found for key: %s", model.Key)
}

func (a *ProjectMemoryTool) list(ctx context.Context, _ ProjectMemoryModel) string {
	var prefix = fmt.Sprintf("project:%s:", a.projectid)
	notes, err := a.memory.ListNotesWithPrefix(ctx, prefix)
	if err != nil {
		return err.Error()
	}
	if len(notes) == 0 {
		return "No project memory saved yet."
	}

	sb := strings.Builder{}
	sb.WriteString("Project memory keys:\n")

	for _, noteKey := range notes {
		// Strip prefix to show clean key names
		cleanKey := noteKey
		if !strings.HasPrefix(noteKey, prefix) {
			cleanKey = noteKey[len(prefix):]
		}
		fmt.Fprintf(&sb, "  - %s", cleanKey)
	}
	return sb.String()
}

func (a *ProjectMemoryTool) delete(ctx context.Context, model ProjectMemoryModel) string {
	if model.Key == "" {
		return "Error: 'key' is required for delete."
	}
	fullKey := a.projectKey(model.Key)
	if err := a.memory.DeleteNote(ctx, fullKey); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("delete project memory: %s", model.Key)
}

func (a *ProjectMemoryTool) projectKey(key string) string {
	return fmt.Sprintf("project:%s:%s", a.projectid, key)
}
