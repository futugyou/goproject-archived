package fileread

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type FileReadTool struct {
	config *core.ToolingConfig
}

func New(config *core.ToolingConfig) *FileReadTool {
	if config == nil {
		config = &core.ToolingConfig{}
	}
	return &FileReadTool{config: config}
}

func (a *FileReadTool) Name() string {
	return "read_file	"
}

func (a *FileReadTool) Description() string {
	return "Read the contents of a file from the local filesystem. For large files, use start_line and max_lines to read in chunks."
}

func (a *FileReadTool) ParameterSchema() string {
	return `
	{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Absolute or relative file path"
    },
    "start_line": {
      "type": "integer",
      "description": "1-based line number to start reading from (default: 1)",
      "default": 1
    },
    "max_lines": {
      "type": "integer",
      "description": "Maximum number of lines to read (default: 500, max: 5000)",
      "default": 500
    }
  },
  "required": [
    "path"
  ]
}
`
}

type FileReadModel struct {
	Path      string `json:"path"`
	StartLine int32  `json:"start_line"`
	MaxLines  int32  `json:"max_lines"`
}

func (a *FileReadTool) Execute(ctx context.Context, argumentsJson string) string {
	var args FileReadModel

	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return err.Error()
	}

	if args.StartLine == 0 {
		args.StartLine = 1
	}
	if args.MaxLines == 0 {
		args.MaxLines = 500
	}

	args.StartLine = max(args.StartLine, 1)
	args.MaxLines = util.Clamp(args.MaxLines, 1, 5000)

	var resolvedPath = pathpolicy.ResolveRealPath(args.Path)

	if !pathpolicy.IsReadAllowed(*a.config, resolvedPath) {
		return fmt.Sprintf("Error: Read access denied for path: %s", args.Path)
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		return err.Error()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var sb strings.Builder
	var totalLines int32 = 0
	var read int32 = 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err().Error()
		default:
		}

		line := scanner.Text()
		totalLines++

		if totalLines < args.StartLine {
			continue
		}

		if read >= args.MaxLines {
			for scanner.Scan() {
				totalLines++
			}
			break
		}

		if read > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(line)
		read++
	}

	if err := scanner.Err(); err != nil {
		return err.Error()
	}

	if totalLines > args.StartLine-1+read {
		nextLine := args.StartLine + read
		fmt.Fprintf(&sb, "\n[Showing lines %d-%d of %d total. Use start_line=%d to read more.]",
			args.StartLine, args.StartLine+read-1, totalLines, nextLine)
	} else if args.StartLine > 1 {
		fmt.Fprintf(&sb, "\n[End of file. Showed lines %d-%d of %d total.]",
			args.StartLine, args.StartLine+read-1, totalLines)
	}

	return sb.String()
}
