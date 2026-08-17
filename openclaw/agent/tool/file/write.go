package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type FileWriteTool struct {
	config *core.ToolingConfig
}

func NewFileWriteTool(config *core.ToolingConfig) *FileWriteTool {
	if config == nil {
		config = &core.ToolingConfig{}
	}
	return &FileWriteTool{config: config}
}

func (a *FileWriteTool) Name() string {
	return "write_file"
}

func (a *FileWriteTool) Description() string {
	return "Write content to a file on the local filesystem. Creates parent directories if needed."
}

func (a *FileWriteTool) ParameterSchema() string {
	return `
{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "File path to write to"
    },
    "content": {
      "type": "string",
      "description": "Content to write"
    }
  },
  "required": [
    "path",
    "content"
  ]
}
`
}

type WriteReadModel struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (a *FileWriteTool) Execute(ctx context.Context, argumentsJson string) string {
	if a.config.ReadOnlyMode {
		return "Error: write_file is disabled because Tooling.ReadOnlyMode is enabled."
	}

	var args WriteReadModel
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return err.Error()
	}

	if args.Path == "" {
		return "Error: 'path' is required."
	}

	resolvedPath := pathpolicy.ResolveRealPath(args.Path)
	if !pathpolicy.IsReadAllowed(*a.config, resolvedPath) {
		return fmt.Sprintf("Error: Write access denied for path: %s", args.Path)
	}

	dir := filepath.Dir(resolvedPath)
	if dir != "" {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err.Error()
		}
	}

	if err := util.SaveFile(ctx, resolvedPath, args.Content); err != nil {
		return err.Error()
	}

	return fmt.Sprintf("Written %d characters to %s", len(args.Content), args.Path)
}
