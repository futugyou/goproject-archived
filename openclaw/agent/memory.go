package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/shlex"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type ToolCallOutcome struct {
	Success           bool
	Text              string
	StructuredContent any
	Error             string
}

func FailToolCallOutcome(err string) *ToolCallOutcome {
	if err == "" {
		err = "fractal Memory MCP call failed"
	}

	return &ToolCallOutcome{Error: err}
}

type FractalMemoryMcpProvider struct {
	config        *core.GatewayConfig
	workspacePath string
	logger        *slog.Logger

	mu       sync.Mutex
	client   *mcp.Client
	session  *mcp.ClientSession
	disposed atomic.Bool
}

func NewFractalMemoryMcpProvider(config *core.GatewayConfig, workspacePath string, logger *slog.Logger) *FractalMemoryMcpProvider {
	return &FractalMemoryMcpProvider{
		config:        config,
		workspacePath: workspacePath,
		logger:        logger,
	}
}

func (f *FractalMemoryMcpProvider) depthName(depth int) string {
	switch util.Clamp(depth, 0, 3) {
	case 0:
		return "Pointer"
	case 2:
		return "Working"
	case 3:
		return "Deep"
	default:
		return "Orientation"
	}
}

func (*FractalMemoryMcpProvider) normalize(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return strings.TrimSpace(strings.ToLower(value))
}

func (f *FractalMemoryMcpProvider) normalizeExportMode(mode string) string {
	switch f.normalize(mode, "compact") {
	case "standard":
		return "standard"
	case "verbose":
		return "verbose"
	default:
		return "compact"
	}
}

func (f *FractalMemoryMcpProvider) normalizeView(view string) string {
	switch f.normalize(view, "index") {
	case "state":
		return "state"
	case "timeline":
		return "timeline"
	case "decisions":
		return "decisions"
	case "children":
		return "children"
	default:
		return "index"
	}
}

func (f *FractalMemoryMcpProvider) normalizeOptional(value string) string {
	return strings.TrimSpace(value)
}

func (f *FractalMemoryMcpProvider) requirePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is required")
	}

	return path, nil
}

func (f *FractalMemoryMcpProvider) buildValidationSummary(issues []core.StructuredMemoryValidationIssue) string {
	count := len(issues)
	if count == 0 {
		return "fractal Memory validation completed with no reported issues"
	}

	return fmt.Sprintf("fractal Memory validation reported %d issue(s)", count)
}

func (f *FractalMemoryMcpProvider) appendList(sb *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		return
	}

	fmt.Fprintf(sb, "%s:", label)
	for _, value := range values {
		fmt.Fprintf(sb, "- %s", value)
	}

	sb.WriteString("\n")
}

func (f *FractalMemoryMcpProvider) appendField(sb *strings.Builder, label string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(sb, "%s:", label)
	sb.WriteString(value)
	sb.WriteString("\n")
}

func (f *FractalMemoryMcpProvider) friendlyError(err error) string {
	if err == nil {
		return ""
	}
	return "fractal Memory MCP provider is unavailable, err: " + err.Error()
}

func (p *FractalMemoryMcpProvider) GetStatus(ctx context.Context) (*core.StructuredMemoryStatusResponse, error) {
	fractal := p.config.Memory.Fractal
	resolvedRoot := p.resolveRepositoryRoot(fractal)
	warnings := p.buildRepositoryWarnings(resolvedRoot)

	response := &core.StructuredMemoryStatusResponse{
		Enabled:                fractal.Enabled,
		Mode:                   p.normalize(fractal.Mode, "mcp"),
		RepositoryRoot:         fractal.RepositoryRoot,
		ResolvedRepositoryRoot: resolvedRoot,
		McpCommand:             fractal.McpCommand,
		AutoContextMode:        p.normalize(fractal.AutoContextMode, "off"),
		AllowWrites:            fractal.AllowWrites,
		WriteToolsAvailable:    fractal.Enabled && fractal.AllowWrites,
		Available:              false,
		Status:                 "disabled",
		Warnings:               warnings,
	}

	if !fractal.Enabled {
		return response, nil
	}

	response.Status = "unavailable"

	_, err := p.ensureSession(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		response.Error = p.friendlyError(err)
		response.Status = "unavailable"
		return response, nil
	}

	response.Available = true
	if len(warnings) == 0 {
		response.Status = "available"
	} else {
		response.Status = "available_with_warnings"
	}

	return response, nil
}

func (p *FractalMemoryMcpProvider) ensureSession(ctx context.Context) (*mcp.ClientSession, error) {
	if p.disposed.Load() {
		return nil, errors.New("FractalMemoryMcpProvider has been disposed")
	}

	fractal := p.config.Memory.Fractal
	if !fractal.Enabled {
		return nil, errors.New("Fractal Memory is disabled")
	}
	if !strings.EqualFold(p.normalize(fractal.Mode, "mcp"), "mcp") {
		return nil, fmt.Errorf("unsupported Fractal Memory mode '%s'", fractal.Mode)
	}
	if strings.TrimSpace(fractal.McpCommand) == "" {
		return nil, errors.New("Memory.Fractal.McpCommand is not configured")
	}

	root := p.resolveRepositoryRoot(fractal)
	if strings.TrimSpace(fractal.RepositoryRoot) != "" {
		if info, err := os.Stat(root); os.IsNotExist(err) || !info.IsDir() {
			return nil, fmt.Errorf("Fractal Memory repository root was not found: %s", root)
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-checking 保证 session 只初始化一次
	if p.session != nil {
		return p.session, nil
	}

	// 拆分命令行指令（建议使用 shell 解析库如 shlex 避免路径带空格切割错误）
	cmdParts, err := shlex.Split(fractal.McpCommand)
	if err != nil || len(cmdParts) == 0 {
		return nil, fmt.Errorf("invalid McpCommand configuration: %w", err)
	}

	// 创建底层子进程 exec.Cmd
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	if root != "" {
		cmd.Dir = root
		// 继承当前环境变量并注入特定的环境变量
		cmd.Env = append(os.Environ(), fmt.Sprintf("FRACTALMEM_REPOSITORY_ROOT=%s", root))
	}

	// 组装 CommandTransport
	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	// 15 秒超时控制
	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "fractal-memory",
		Version: "1.0.0",
	}, nil)

	// 连接 Transport，获取控制会话 ClientSession
	session, err := client.Connect(initCtx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("Fractal Memory MCP command '%s' could not be started. Install the MCP server or adjust config: %w", fractal.McpCommand, err)
	}

	p.client = client
	p.session = session
	return p.session, nil
}

func (p *FractalMemoryMcpProvider) resolveRepositoryRoot(fractal *core.FractalMemoryConfig) string {
	root := ""
	if fractal != nil {
		root = fractal.RepositoryRoot
	}

	if strings.TrimSpace(root) == "" {
		if strings.TrimSpace(p.workspacePath) != "" {
			root = p.workspacePath
		} else {
			pwd, err := os.Getwd()
			if err != nil {
				root = "."
			} else {
				root = pwd
			}
		}
	}

	absPath, err := filepath.Abs(root)
	if err != nil {
		return root
	}

	return absPath
}

func (p *FractalMemoryMcpProvider) buildRepositoryWarnings(resolvedRoot string) []string {
	var warnings []string
	if info, err := os.Stat(resolvedRoot); os.IsNotExist(err) || !info.IsDir() {
		warnings = append(warnings, fmt.Sprintf("Repository root directory does not exist: %s", resolvedRoot))
	}
	return warnings
}

// Close 用于清理并释放 MCP 进程连接
func (p *FractalMemoryMcpProvider) Close() error {
	if !p.disposed.CompareAndSwap(false, true) {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.session != nil {
		err := p.session.Close()
		p.session = nil
		p.client = nil
		return err
	}
	return nil
}

func (p *FractalMemoryMcpProvider) callTool(ctx context.Context, toolName string, arguments map[string]any) (*ToolCallOutcome, error) {
	client, err := p.ensureSession(ctx)
	if err != nil {
		return nil, err
	}
	initCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	response, err := client.CallTool(initCtx, p.compactArguments(toolName, arguments))
	if err != nil {
		return nil, err
	}
	var text = p.formatResponseContent(response)
	if response.IsError {
		if text == "" {
			text = fmt.Sprintf("fractal Memory MCP tool '%s' returned an error", toolName)
		}
		return FailToolCallOutcome(text), nil
	}

	return &ToolCallOutcome{Success: true, Text: text, StructuredContent: response.StructuredContent}, nil
}

func (f *FractalMemoryMcpProvider) formatResponseContent(response *mcp.CallToolResult) string {
	parts := []string{}
	for _, v := range response.Content {
		if d, ok := v.(*mcp.TextContent); ok {
			parts = append(parts, d.Text)
		}
		if d, ok := v.(*mcp.EmbeddedResource); ok {
			if d.Resource != nil && d.Resource.Text != "" {
				parts = append(parts, d.Resource.Text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func (f *FractalMemoryMcpProvider) compactArguments(toolName string, arguments map[string]any) *mcp.CallToolParams {
	return &mcp.CallToolParams{Name: toolName, Arguments: arguments}
}

func parseSourceRefsFromText(text string) []core.StructuredMemorySourceRef {
	if text == "" {
		return []core.StructuredMemorySourceRef{}
	}

	return []core.StructuredMemorySourceRef{
		{
			Path:    "unknown",
			Snippet: util.Truncate(text, 500),
		},
	}
}

func parseRecentItems(data any) []core.StructuredMemorySourceRef {
	items, ok1 := util.TryGetArrayOrObjectArray(data, "items")
	results, ok2 := util.TryGetArrayOrObjectArray(data, "results")
	if !ok1 && !ok2 {
		return nil
	}

	result := []core.StructuredMemorySourceRef{}

	for _, item := range append(items, results...) {
		obj, ok := util.TryGetObject(item)
		if !ok {
			continue
		}

		path := util.GetString(obj, "relativePath")
		if path == nil {
			path = util.GetString(obj, "path")
		}

		title := util.GetString(obj, "title")
		fileName := util.GetString(obj, "fileName")
		lastModified := util.GetDateTimeOffset(obj, "lastModified")
		ref := core.StructuredMemorySourceRef{
			LastModifiedUtc: lastModified,
		}
		if path != nil {
			ref.Path = *path
		}

		if title != nil {
			ref.Title = *title
		}

		if fileName != nil {
			ref.FileName = *fileName
		}

		result = append(result, ref)
	}

	return result
}

func (p *FractalMemoryMcpProvider) Search(ctx context.Context, query string, limit int, scope string) *core.StructuredMemorySearchResult {
	if query == "" {
		return &core.StructuredMemorySearchResult{
			Error: "query is required",
		}
	}

	result, err := p.callTool(ctx, "memory_search", map[string]any{
		"query": query,
		"limit": util.Clamp(limit, 1, 50),
		"scope": p.normalizeOptional(scope),
	})

	if err != nil {
		return &core.StructuredMemorySearchResult{
			Error: err.Error(),
		}
	}

	if !result.Success {
		return &core.StructuredMemorySearchResult{
			Query: query,
			Error: result.Error,
			Scope: p.normalizeOptional(scope),
		}
	}

	res := &core.StructuredMemorySearchResult{
		Query:   query,
		Success: true,
		Scope:   p.normalizeOptional(scope),
	}
	items := parseRecentItems(result.StructuredContent)
	if len(items) == 0 {
		items = parseSourceRefsFromText(result.Text)
	}
	res.Items = items

	return res
}
