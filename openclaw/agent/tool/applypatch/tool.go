package applypatch

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type ApplyPatchTool struct {
	config *core.ToolingConfig
}

func New(config *core.ToolingConfig) *ApplyPatchTool {
	if config == nil {
		config = &core.ToolingConfig{}
	}
	return &ApplyPatchTool{config: config}
}

func (a *ApplyPatchTool) Name() string {
	return "apply_patch	"
}

func (a *ApplyPatchTool) Description() string {
	return "Apply a unified diff patch to a file. Supports multi-hunk patches for complex edits."
}

func (a *ApplyPatchTool) ParameterSchema() string {
	return `{"type":"object","properties":{"path":{"type":"string","description":"File path to patch"},"patch":{"type":"string","description":"Unified diff patch content (lines starting with +/- and @@ hunk headers)"}},"required":["path","patch"]}`
}

func (a *ApplyPatchTool) Execute(ctx context.Context, argumentsJson string) string {
	if a.config.ReadOnlyMode {
		return "Error: apply_patch is disabled because Tooling.ReadOnlyMode is enabled."
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &root); err != nil {
		return err.Error()
	}

	var path = util.GetString(root, "path")
	if path == nil || strings.TrimSpace(*path) == "" {
		return "Error: 'path' is required."
	}

	var patch = util.GetString(root, "patch")
	if patch == nil || strings.TrimSpace(*patch) == "" {
		return "Error: 'patch' is required."
	}

	var resolvedPath = pathpolicy.ResolveRealPath(*path)

	if !pathpolicy.IsWriteAllowed(*a.config, resolvedPath) {
		return fmt.Sprintf("Error: Write access denied for path: %s", *path)
	}

	if !util.FileExists(resolvedPath) {
		return fmt.Sprintf("Error: File not found: %s", *path)
	}

	originalLines, err := util.ReadAllLines(ctx, resolvedPath)
	if err != nil {
		return err.Error()
	}
	var hunks = parseHunks(*patch)

	if len(hunks) == 0 {
		return "Error: No valid hunks found in patch. Use @@ -start,count +start,count @@ headers."
	}

	result := slices.Clone(originalLines)
	var offset = 0

	for _, hunk := range hunks {
		var startLine = hunk.OriginalStart - 1 + offset
		if startLine < 0 || startLine > len(result) {
			return fmt.Sprintf("Error: Hunk at line %d is out of range (file has %d lines).", hunk.OriginalStart, len(result))
		}

		// Validate removed lines match file content
		if startLine+len(hunk.RemoveLines) > len(result) {
			return fmt.Sprintf("Error: Hunk at line %d expects %d lines to remove, but only %d lines remain.", hunk.OriginalStart, len(hunk.RemoveLines), len(result)-startLine)
		}

		for i := 0; i < len(hunk.RemoveLines); i++ {
			var expected = strings.TrimSpace(hunk.RemoveLines[i])
			var actual = strings.TrimSpace(result[startLine+i])
			if !strings.EqualFold(expected, actual) {
				return fmt.Sprintf("Error: Hunk at line %d mismatch. Expected: \"%s\" Got: \"%s\"", hunk.OriginalStart+i, util.Truncate(expected, 60), util.Truncate(actual, 60))
			}
		}

		// Remove old lines (validated above)
		for i := 0; i < len(hunk.RemoveLines); i++ {
			result = slices.Delete(result, startLine, startLine+1)
		}

		// Insert new lines
		for i := len(hunk.AddLines) - 1; i >= 0; i-- {
			result = slices.Insert(result, startLine, hunk.AddLines[i])
		}

		offset += len(hunk.AddLines) - len(hunk.RemoveLines)
	}

	if err := util.SaveOneFile(ctx, resolvedPath, result); err != nil {
		return err.Error()
	}

	return fmt.Sprintf("Applied %d hunk(s) to %s.", len(hunks), *path)
}

type Hunk struct {
	OriginalStart int
	RemoveLines   []string
	AddLines      []string
}

func parseHunks(patch string) []Hunk {
	hunks := []Hunk{}
	var lines = strings.Split(patch, "\n")
	var current *Hunk

	for _, rawLine := range lines {
		var line = strings.TrimRight(rawLine, "\r")

		if strings.HasPrefix(line, "@@") {
			if current != nil {
				hunks = append(hunks, *current)
			}

			var origStart = parseHunkStart(line)
			current = &Hunk{OriginalStart: origStart}
		} else if current != nil {

			if strings.HasPrefix(line, "-") {
				current.RemoveLines = append(current.RemoveLines, line[1:])
			} else if strings.HasPrefix(line, "+") {
				current.AddLines = append(current.AddLines, line[1:])
			}
		}
	}

	if current != nil {
		hunks = append(hunks, *current)
	}

	return hunks
}

func parseHunkStart(header string) int {
	// Parse @@ -start,count +start,count @@
	var idx = util.IndexOf(header, "-", 3)
	if idx < 0 {
		return 1
	}
	var comma = util.IndexOf(header, ",", idx)
	var end = comma
	if comma <= 0 {
		end = util.IndexOf(header, " ", idx+1)
	}
	if end < 0 {
		end = len(header)
	}
	if idx+1 < 0 || end > len(header) || idx+1 > end {
		return 1
	}

	start, err := strconv.Atoi(header[idx+1 : end])
	if err != nil {
		return 1
	}
	return start
}
