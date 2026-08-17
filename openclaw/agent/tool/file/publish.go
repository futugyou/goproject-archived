package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type PublishFileTool struct {
	config *core.ToolingConfig
}

func NewPublishFileTool(config *core.ToolingConfig) *PublishFileTool {
	if config == nil {
		config = &core.ToolingConfig{}
	}
	return &PublishFileTool{config: config}
}

func (a *PublishFileTool) Name() string {
	return "publish_file"
}

func (a *PublishFileTool) Description() string {
	return "Publish a file so the user can download it via a download link. " +
		"Pass the absolute path of any file that already exists on the filesystem — " +
		"including files created by 'shell' in /tmp or other temporary directories. " +
		"If the file is outside the workspace the tool will automatically copy it into the workspace downloads folder. " +
		"The system will then register the file and deliver a download link to the user automatically."
}

func (a *PublishFileTool) ParameterSchema() string {
	return `{"type":"object","properties":{"path":{"type":"string","description":"Absolute path of the file to publish for download"}},"required":["path"]}`
}

func (a *PublishFileTool) Execute(ctx context.Context, argumentsJson string) string {
	var root map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &root); err != nil {
		return err.Error()
	}

	var path = util.GetString(root, "path")
	if path == nil || strings.TrimSpace(*path) == "" {
		return "Error: 'path' is required."
	}
	var resolvedPath = pathpolicy.ResolveRealPath(*path)

	if !pathpolicy.IsReadAllowed(*a.config, resolvedPath) {
		return fmt.Sprintf("Error: Read access denied for path: %s", *path)
	}

	if !util.FileExists(resolvedPath) {
		return fmt.Sprintf("Error: File not found: %s", *path)
	}

	// If the file is already inside an AllowedWriteRoot, GatewayWorkers can
	// pick it up directly — no copy needed.
	publishPath := resolvedPath
	if !pathpolicy.IsWriteAllowed(*a.config, resolvedPath) {
		publishPath = a.copyToDownloadsFolder(ctx, resolvedPath)
	}

	if publishPath == "" {
		return "Error: Could not copy file to a publishable location. Ensure WorkspaceRoot or an AllowedWriteRoot is configured."
	}

	fileinfo, err := os.Stat(publishPath)
	if err != nil {
		return err.Error()
	}

	fileName := filepath.Base(publishPath)
	sizeLabel := ""
	size := fileinfo.Size()
	if size < 1024 {
		sizeLabel = fmt.Sprintf("%d B", size)
	} else if size < 1_048_576 {
		sizeLabel = fmt.Sprintf("%d KB", size/1024)
	} else {
		sizeLabel = fmt.Sprintf("%d MB", size/1_048_576)
	}

	return fmt.Sprintf("File ready for download: %s (%s)\n[FILE_PATH:%s]", fileName, sizeLabel, publishPath)
}

func (a *PublishFileTool) copyToDownloadsFolder(ctx context.Context, sourcePath string) string {
	var downloadsDir = a.resolveDownloadsDirectory()
	if downloadsDir == "" {
		return ""
	}

	os.MkdirAll(downloadsDir, 0755)

	var fileName = filepath.Base(sourcePath)
	var destPath = filepath.Join(downloadsDir, fileName)

	so, err := filepath.Abs(sourcePath)
	de, err1 := filepath.Abs(destPath)
	if err != nil || err1 != nil {
		return ""
	}
	// Avoid overwriting with a suffix if a file with the same name already exists.
	if util.FileExists(destPath) && so != de {
		var stem = util.GetFileNameWithoutExtension(fileName)
		var ext = filepath.Ext(fileName)
		destPath = filepath.Join(downloadsDir, fmt.Sprintf("%s_%s%s", stem, util.CleanUUID()[8:], ext))
	}

	if err := util.CopyFileWithContext(ctx, sourcePath, destPath); err != nil {
		return ""
	}

	return destPath
}

func (a *PublishFileTool) resolveDownloadsDirectory() string {
	var workspaceRaw = core.SecretResolverInstance.Resolve(a.config.WorkspaceRoot)
	workspaceBase := workspaceRaw
	if workspaceRaw == "" {
		dir, err := os.Getwd()
		if err != nil {

			return ""
		}
		workspaceBase = dir
	}

	p, err := filepath.Abs(workspaceBase)
	if err != nil {
		return ""
	}
	var downloadsDir = filepath.Join(p, ".downloads")
	// Verify the destination would be writable according to policy.
	if pathpolicy.IsWriteAllowed(*a.config, downloadsDir) {
		return downloadsDir
	}
	return ""
}
