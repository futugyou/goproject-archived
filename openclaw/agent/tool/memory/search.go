package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type MemorySearchTool struct {
	store core.IMemoryNoteSearch
}

func NewMemorySearchTool(store core.IMemoryNoteSearch) *MemorySearchTool {
	return &MemorySearchTool{store: store}
}

func (a *MemorySearchTool) Name() string {
	return "memory_search"
}

func (a *MemorySearchTool) Description() string {
	return "Search persistent memory notes by keyword (SQLite FTS when enabled). Useful for recalling prior decisions and preferences."
}

func (a *MemorySearchTool) ParameterSchema() string {
	return `
{
          "type": "object",
          "properties": {
            "query": { "type": "string", "description": "Search query" },
            "prefix": { "type": "string", "description": "Optional key prefix filter (e.g. 'project:myproj:')" },
            "limit": { "type": "integer", "default": 10, "minimum": 1, "maximum": 50 },
            "format": { "type": "string", "enum": ["text","json"], "default": "text" }
          },
          "required": ["query"]
        } `
}

func indent(s string) string {
	return " " + strings.ReplaceAll(s, "\n", "\n  ")
}

func (a *MemorySearchTool) Execute(ctx context.Context, argumentsJson string) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	var model struct {
		Query  string `json:"query"`
		Prefix string `json:"prefix"`
		Format string `json:"format"`
		Limit  int    `json:"limit"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if model.Query == "" {
		return "Error: query is required."
	}

	if model.Limit <= 0 {
		model.Limit = 10
	}

	model.Limit = util.Clamp(model.Limit, 1, 50)

	if model.Format == "" {
		model.Format = "text"
	}

	hits, err := a.store.SearchNotes(ctx, model.Query, model.Prefix, model.Limit)
	if err != nil {
		return err.Error()
	}
	if len(hits) == 0 {
		return "No matching memory notes found."
	}

	if model.Format == "json" {
		data, err := json.Marshal(hits)
		if err != nil {
			return err.Error()
		}
		return string(data)
	}

	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "Matches: %d\n", len(hits))
	for _, hit := range hits {
		fmt.Fprintf(&sb, "- %s (updated %s, score %f)\n", hit.Key, hit.UpdatedAt.Format(time.RFC3339Nano), hit.Score)
		sb.WriteString(indent(util.Truncate(hit.Content, 400)))
		sb.WriteString("\n")
	}

	return sb.String()
}
