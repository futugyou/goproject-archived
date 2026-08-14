package editfiletool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/core"
)

type EditFileTool struct {
	config core.ToolingConfig
}

func New(config core.ToolingConfig) *EditFileTool {
	return &EditFileTool{config: config}
}

func (e *EditFileTool) Name() string {
	return "edit_file"
}

func (e *EditFileTool) Description() string {
	return "Edit a file by replacing a specific text string with new text. Safer than write_file for targeted change changes."
}

func (e *EditFileTool) ParameterSchema() string {
	return `
		{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "File path to edit"
    },
    "old_text": {
      "type": "string",
      "description": "Exact text to find and replace"
    },
    "new_text": {
      "type": "string",
      "description": "Replacement text"
    },
    "replace_all": {
      "type": "boolean",
      "description": "Replace all occurrences (default: false)"
    }
  },
  "required": [
    "path",
    "old_text",
    "new_text"
  ]
}
		`
}

type EditFileParams struct {
	Path       string `json:"path"`
	OldText    string `json:"old_text"`
	NewText    string `json:"new_text"`
	ReplaceAll bool   `json:"replace_all"`
}

func (a *EditFileTool) ExecuteExecute(ctx context.Context, argumentsJson string) string {
	if a.config.ReadOnlyMode {
		return "Error: edit_file is disabled because Tooling.ReadOnlyMode is enabled."
	}

	var args EditFileParams
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return fmt.Sprintf("invalid arguments: %v", err)
	}

	if args.Path == "" {
		return "Error: 'path' is required."
	}

	if args.OldText == "" {
		return "Error: 'old_text' is required and must not be empty."
	}

	if args.NewText == "" {
		return "Error: 'new_text' is required."
	}

	replaceAll := args.ReplaceAll

	path, err := filepath.Abs(args.Path)
	if err != nil {
		return fmt.Sprintf("invalid path: %v", err)
	}

	if !pathpolicy.IsWriteAllowed(a.config, path) {
		return fmt.Sprintf("Error: Write access denied for path: %s", path)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Sprintf("Error: File not found: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("failed to read file: %v", err)
	}

	if !strings.Contains(string(content), args.OldText) {
		return "Error: 'old_text' not found in file."
	}

	if !replaceAll {
		firstIdx := strings.Index(string(content), args.OldText)
		lastIdx := strings.LastIndex(string(content), args.OldText)
		if firstIdx != lastIdx {
			return "Error: 'old_text' appears multiple times. Set replace_all=true or provide more context to make it unique."
		}
	}

	updated := ""
	if !replaceAll {
		updated = ReplaceFirst(string(content), args.OldText, args.NewText)
	} else {
		updated = strings.ReplaceAll(string(content), args.OldText, args.NewText)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(updated), 0644); err != nil {
		return fmt.Sprintf("failed to write temporary file: %v", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Sprintf("failed to rename temporary file: %v", err)
	}

	count := 1
	if replaceAll {
		count = ReplaceCount(string(content), args.OldText)
	}
	return fmt.Sprintf("Replaced %d occurrence(s) in %s.", count, path)
}

func ReplaceFirst(source, oldValue, newValue string) string {
	before, after, ok := strings.Cut(source, oldValue)
	if !ok {
		return source
	}
	return before + newValue + after
}

func ReplaceCount(source, value string) int {
	count := 0
	idx := 0
	for {
		idx = strings.Index(source[idx:], value)
		if idx == -1 {
			break
		}
		count++
		idx += len(value)
	}
	return count
}
