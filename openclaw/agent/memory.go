package agent

import (
	"context"
	"encoding/json"
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

func buildValidationSummary(issues []core.StructuredMemoryValidationIssue) string {
	count := len(issues)
	if count == 0 {
		return "fractal Memory validation completed with no reported issues"
	}

	return fmt.Sprintf("fractal Memory validation reported %d issue(s)", count)
}

func getStringOrDefault(ptr *string, defaultValue string) string {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

func appendList(sb *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		return
	}

	fmt.Fprintf(sb, "%s:", label)
	for _, value := range values {
		fmt.Fprintf(sb, "- %s", value)
	}

	sb.WriteString("\n")
}

func appendField(sb *strings.Builder, label string, value string) {
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

type sourceRefDTO struct {
	Path           string `json:"path"`
	RelativePath   string `json:"relativePath"`
	Title          string `json:"title"`
	SourcePath     string `json:"sourcePath"`
	SectionHeading string `json:"sectionHeading"`
	StartLine      *int   `json:"startLine"`
	EndLine        *int   `json:"endLine"`
	Snippet        string `json:"snippet"`
	Excerpt        string `json:"excerpt"`
}

func (dto *sourceRefDTO) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		dto.Path = str
		dto.Title = str
		return nil
	}

	type plainDTO sourceRefDTO
	var p plainDTO
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*dto = sourceRefDTO(p)
	return nil
}

func (dto sourceRefDTO) toModel() (core.StructuredMemorySourceRef, bool) {
	path := dto.Path
	if path == "" {
		if dto.RelativePath != "" {
			path = dto.RelativePath
		} else if dto.SourcePath != "" {
			path = dto.SourcePath
		}
	}

	snippet := dto.Snippet
	if snippet == "" {
		snippet = dto.Excerpt
	}

	if strings.TrimSpace(path) == "" && strings.TrimSpace(snippet) == "" {
		return core.StructuredMemorySourceRef{}, false
	}

	return core.StructuredMemorySourceRef{
		Path:           path,
		Title:          dto.Title,
		SourcePath:     dto.SourcePath,
		SectionHeading: dto.SectionHeading,
		StartLine:      dto.StartLine,
		EndLine:        dto.EndLine,
		Snippet:        snippet,
	}, true
}

type rootDTO struct {
	RelativePath    *string           `json:"relativePath"`
	Title           *string           `json:"title"`
	Summary         *string           `json:"summary"`
	IndexSummary    *string           `json:"indexSummary"`
	CurrentState    *string           `json:"currentState"`
	Depth           *int              `json:"depth"`
	View            *string           `json:"view"`
	Children        []sourceRefDTO    `json:"children"`
	SuggestedReads  []sourceRefDTO    `json:"suggestedReads"`
	RecentTimeline  []json.RawMessage `json:"recentTimeline"`
	RecentDecisions []json.RawMessage `json:"recentDecisions"`
}

func parseRootDTO(data []byte) (*rootDTO, bool) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, false
	}

	if strings.HasPrefix(trimmed, "[") {
		var list []json.RawMessage
		if err := json.Unmarshal(data, &list); err != nil || len(list) == 0 {
			return nil, false
		}
		data = list[0]
	} else if !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}

	var root rootDTO
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false
	}

	return &root, true
}

func parseOpenResult(structured any, path string, depth int, view string, text string) core.StructuredMemoryOpenResult {
	rawJSON, err := toJSONBytes(structured)

	if err == nil && len(rawJSON) > 0 {
		if root, ok := parseRootDTO(rawJSON); ok {
			children := parseSourceRefs(root.Children)
			suggestedReads := parseSourceRefs(root.SuggestedReads)

			timeline := parseStringRefs(root.RecentTimeline, path)
			decisions := parseStringRefs(root.RecentDecisions, path)

			finalPath := path
			if root.RelativePath != nil && *root.RelativePath != "" {
				finalPath = *root.RelativePath
			}

			finalDepth := depth
			if root.Depth != nil {
				finalDepth = *root.Depth
			}

			finalView := view
			if root.View != nil && *root.View != "" {
				finalView = strings.ToLower(*root.View)
			}

			var title, summary string
			if root.Title != nil {
				title = *root.Title
			}
			if root.Summary != nil {
				summary = *root.Summary
			}

			sources := make([]core.StructuredMemorySourceRef, 0, len(children)+len(suggestedReads)+len(timeline)+len(decisions))
			sources = append(sources, children...)
			sources = append(sources, suggestedReads...)
			sources = append(sources, timeline...)
			sources = append(sources, decisions...)

			return core.StructuredMemoryOpenResult{
				Success:         true,
				Path:            finalPath,
				Title:           title,
				Summary:         summary,
				Depth:           finalDepth,
				View:            finalView,
				Content:         buildOpenContent(root, text),
				Children:        children,
				SuggestedReads:  suggestedReads,
				RecentTimeline:  timeline,
				RecentDecisions: decisions,
				Sources:         sources,
			}
		}
	}

	return core.StructuredMemoryOpenResult{
		Success: true,
		Path:    path,
		Depth:   depth,
		View:    view,
		Content: text,
		Sources: []core.StructuredMemorySourceRef{
			{
				Path:    path,
				Snippet: util.Truncate(text, 500),
			},
		},
	}
}

func buildOpenContent(root *rootDTO, fallback string) string {
	if root == nil {
		return fallback
	}

	var sb strings.Builder

	// 解析 string 数组供 Timeline 和 Decisions 使用
	timelineStrs := parseStringArray(root.RecentTimeline)
	decisionStrs := parseStringArray(root.RecentDecisions)

	appendField(&sb, "Title", getString(root.Title))
	appendField(&sb, "Summary", getString(root.Summary))
	appendField(&sb, "Index", getString(root.IndexSummary))
	appendField(&sb, "Current state", getString(root.CurrentState))
	appendList(&sb, "Recent timeline", timelineStrs)
	appendList(&sb, "Recent decisions", decisionStrs)

	text := strings.TrimSpace(sb.String())
	if len(text) == 0 {
		return fallback
	}
	return text
}

func getString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
func parseStringArray(rawList []json.RawMessage) []string {
	res := make([]string, 0, len(rawList))
	for _, raw := range rawList {
		var str string
		if json.Unmarshal(raw, &str) != nil {
			str = string(raw)
		}
		str = strings.TrimSpace(str)
		if str != "" {
			res = append(res, str)
		}
	}
	return res
}
func parseSourceRefs(dtos []sourceRefDTO) []core.StructuredMemorySourceRef {
	res := make([]core.StructuredMemorySourceRef, 0, len(dtos))
	for _, dto := range dtos {
		if ref, valid := dto.toModel(); valid {
			res = append(res, ref)
		}
	}
	return res
}

func parseStringRefs(rawList []json.RawMessage, path string) []core.StructuredMemorySourceRef {
	res := make([]core.StructuredMemorySourceRef, 0, len(rawList))
	for _, itemRaw := range rawList {
		var str string
		if json.Unmarshal(itemRaw, &str) != nil {
			str = string(itemRaw)
		}

		if strings.TrimSpace(str) != "" {
			res = append(res, core.StructuredMemorySourceRef{
				Path:    path,
				Snippet: str,
			})
		}
	}
	return res
}

func toJSONBytes(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	switch val := v.(type) {
	case []byte:
		return val, nil
	case string:
		return []byte(val), nil
	default:
		return json.Marshal(v)
	}
}

func (p *FractalMemoryMcpProvider) Open(ctx context.Context, path string, depth int, view string) (*core.StructuredMemoryOpenResult, error) {
	path, err := p.requirePath(path)
	if err != nil {
		return nil, err
	}
	view = p.normalizeView(view)
	var depthName = p.depthName(depth)

	result, err := p.callTool(ctx, "memory_open", map[string]any{
		"path":  path,
		"depth": depthName,
		"view":  util.ToPascal(view),
	})

	if err != nil {
		return &core.StructuredMemoryOpenResult{Path: path, Depth: depth, View: view, Error: err.Error()}, nil
	}

	if !result.Success {
		return &core.StructuredMemoryOpenResult{Path: path, Depth: depth, View: view, Error: result.Error}, nil
	}

	res := parseOpenResult(result.StructuredContent, path, depth, view, result.Text)
	return &res, nil
}

func (p *FractalMemoryMcpProvider) Recent(ctx context.Context, days int, limit int, scope string) (*core.StructuredMemoryRecentResult, error) {
	result, err := p.callTool(ctx, "memory_recent", map[string]any{
		"days":  util.Clamp(days, 1, 3650),
		"limit": util.Clamp(limit, 1, 100),
		"scope": p.normalizeOptional(scope),
	})

	if err != nil {
		return &core.StructuredMemoryRecentResult{Days: util.Clamp(days, 1, 3650), Scope: p.normalizeOptional(scope), Error: err.Error()}, nil
	}

	if !result.Success {
		return &core.StructuredMemoryRecentResult{Days: util.Clamp(days, 1, 3650), Scope: p.normalizeOptional(scope), Error: result.Error}, nil
	}

	res := &core.StructuredMemoryRecentResult{
		Success: true,
		Days:    util.Clamp(days, 1, 3650),
		Scope:   p.normalizeOptional(scope),
	}
	items := parseRecentItems(result.StructuredContent)
	if len(items) == 0 {
		items = parseSourceRefsFromText(result.Text)
	}
	res.Items = items

	return res, nil
}

func (p *FractalMemoryMcpProvider) Export(ctx context.Context, path string, mode string) (*core.StructuredMemoryExportResult, error) {
	path, err := p.requirePath(path)
	if err != nil {
		return nil, err
	}
	mode = p.normalizeExportMode(mode)

	result, err := p.callTool(ctx, "memory_export", map[string]any{
		"path": path,
		"mode": mode,
	})

	if err != nil {
		return &core.StructuredMemoryExportResult{Path: path, Mode: mode, Error: err.Error()}, nil
	}

	if !result.Success {
		return &core.StructuredMemoryExportResult{Path: path, Mode: mode, Error: result.Error}, nil
	}

	return parseExportResult(result.StructuredContent, path, mode, result.Text), nil
}

func parseExportResult(structured any, path, mode, text string) *core.StructuredMemoryExportResult {
	if root, ok := util.TryGetObject(structured); ok {
		exportPath := getStringOrDefault(util.GetString(root, "relativePath"), path)
		sources := parseAnswerContextSources(root)
		content := buildExportContent(root, text)

		var parsedMode string
		if modeStr := util.GetString(root, "mode"); modeStr != nil {
			parsedMode = strings.ToLower(*modeStr)
		} else {
			parsedMode = mode
		}

		finalSources := sources
		if len(finalSources) == 0 {
			finalSources = []core.StructuredMemorySourceRef{
				{Path: exportPath},
			}
		}

		return &core.StructuredMemoryExportResult{
			Success:   true,
			Path:      exportPath,
			Mode:      parsedMode,
			Title:     getStringOrDefault(util.GetString(root, "title"), ""),
			Content:   content,
			Sources:   finalSources,
			CharCount: len([]rune(content)),
		}
	}

	snippet := util.Truncate(text, 500)
	return &core.StructuredMemoryExportResult{
		Success: true,
		Path:    path,
		Mode:    mode,
		Content: text,
		Sources: []core.StructuredMemorySourceRef{
			{
				Path:    path,
				Snippet: snippet,
			},
		},
		CharCount: len([]rune(text)),
	}
}

func parseAnswerContextSources(root map[string]any) []core.StructuredMemorySourceRef {
	val, ok := util.TryGetProperty(root, "answerContext")
	if !ok {
		return nil
	}

	answerContext, ok := val.(map[string]any)
	if !ok {
		return nil
	}

	return parseSourceArray(answerContext, "supportingSources")
}

func parseSourceArray(root map[string]any, propertyName string) []core.StructuredMemorySourceRef {
	val, ok := util.TryGetProperty(root, propertyName)
	if !ok {
		return nil
	}

	array, ok := val.([]any)
	if !ok {
		return nil
	}

	var result []core.StructuredMemorySourceRef

	for _, item := range array {
		if item == nil {
			continue
		}

		if strVal, isStr := item.(string); isStr {
			result = append(result, core.StructuredMemorySourceRef{
				Path:  strVal,
				Title: strVal,
			})
			continue
		}

		itemObj, isObj := item.(map[string]any)
		if !isObj {
			continue
		}

		var path string
		if p := util.GetString(itemObj, "relativePath"); p != nil {
			path = *p
		} else if p := util.GetString(itemObj, "path"); p != nil {
			path = *p
		} else if p := util.GetString(itemObj, "sourcePath"); p != nil {
			path = *p
		}

		var snippet *string
		if s := util.GetString(itemObj, "excerpt"); s != nil {
			snippet = s
		} else {
			snippet = util.GetString(itemObj, "snippet")
		}

		sourceRef := core.StructuredMemorySourceRef{
			Path:           path,
			Title:          getStringOrDefault(util.GetString(itemObj, "title"), ""),
			SourcePath:     getStringOrDefault(util.GetString(itemObj, "sourcePath"), ""),
			SectionHeading: getStringOrDefault(util.GetString(itemObj, "sectionHeading"), ""),
			StartLine:      util.GetInt(itemObj, "startLine"),
			EndLine:        util.GetInt(itemObj, "endLine"),
			Snippet:        *snippet,
		}

		hasPath := strings.TrimSpace(sourceRef.Path) != ""
		hasSnippet := strings.TrimSpace(sourceRef.Snippet) != ""

		if hasPath || hasSnippet {
			result = append(result, sourceRef)
		}
	}

	return result
}

func buildExportContent(root map[string]any, fallback string) string {
	var sb strings.Builder

	appendField(&sb, "Title", getStringOrDefault(util.GetString(root, "title"), ""))
	appendField(&sb, "Summary", getStringOrDefault(util.GetString(root, "summary"), ""))
	appendField(&sb, "Current state", getStringOrDefault(util.GetString(root, "currentState"), ""))
	appendList(&sb, "Children", util.ParseStringArray(root, "children"))
	appendList(&sb, "Timeline highlights", util.ParseStringArray(root, "timelineHighlights"))
	appendList(&sb, "Decision highlights", util.ParseStringArray(root, "decisionHighlights"))

	if val, ok := util.TryGetProperty(root, "answerContext"); ok {
		if answerContext, isObj := val.(map[string]any); isObj {
			appendField(&sb, "Project branch", getStringOrDefault(util.GetString(answerContext, "projectBranch"), ""))
			appendField(&sb, "Current objective", getStringOrDefault(util.GetString(answerContext, "currentObjective"), ""))
			appendList(&sb, "Key prior decisions", util.ParseStringArray(answerContext, "keyPriorDecisions"))
			appendList(&sb, "Active constraints", util.ParseStringArray(answerContext, "activeConstraints"))
			appendList(&sb, "Next best actions", util.ParseStringArray(answerContext, "nextBestActions"))
			appendList(&sb, "Missing information", util.ParseStringArray(answerContext, "missingInformation"))
		}
	}

	text := strings.TrimSpace(sb.String())
	if len(text) == 0 {
		return fallback
	}
	return text
}

func parseHandoffResult(structured any, path, text string) *core.StructuredMemoryHandoffResult {
	if root, ok := util.TryGetObject(structured); ok {
		var handoffPath = getStringOrDefault(util.GetString(root, "handoffFilePath"), "")
		var content = getStringOrDefault(util.GetString(root, "renderedContent"), text)
		return &core.StructuredMemoryHandoffResult{
			Success:         true,
			Path:            getStringOrDefault(util.GetString(root, "relativePath"), path),
			HandoffFilePath: handoffPath,
			Content:         content,
			Sources:         parseSourceArray(root, "sourceReferences"),
		}
	}

	return &core.StructuredMemoryHandoffResult{
		Success: true,
		Path:    path,
		Content: text,
		Sources: []core.StructuredMemorySourceRef{
			{Path: path, Snippet: util.Truncate(text, 500)},
		},
	}
}

func (p *FractalMemoryMcpProvider) CreateHandoff(ctx context.Context, path string) (*core.StructuredMemoryHandoffResult, error) {
	path, err := p.requirePath(path)
	if err != nil {
		return nil, err
	}
	if p.config.Memory.Fractal == nil || !p.config.Memory.Fractal.AllowWrites {
		return &core.StructuredMemoryHandoffResult{Path: path, Error: "fractal Memory writes are disabled by configuration"}, nil
	}

	result, err := p.callTool(ctx, "memory_handoff_create", map[string]any{
		"path": path,
	})

	if err != nil {
		return &core.StructuredMemoryHandoffResult{Path: path, Error: err.Error()}, nil
	}

	if !result.Success {
		return &core.StructuredMemoryHandoffResult{Path: path, Error: result.Error}, nil
	}

	return parseHandoffResult(result.StructuredContent, path, result.Text), nil
}

func (p *FractalMemoryMcpProvider) Validate(ctx context.Context) (*core.StructuredMemoryValidationResult, error) {
	result, err := p.callTool(ctx, "memory_validate", map[string]any{})
	if err != nil {
		return &core.StructuredMemoryValidationResult{Error: err.Error()}, nil
	}

	if result.Success {
		return parseValidationResult(result.StructuredContent, result.Text, nil), nil
	}

	return &core.StructuredMemoryValidationResult{Error: result.Error}, nil
}

func parseValidationResult(structured any, text string, successSummary *string) *core.StructuredMemoryValidationResult {
	if root, ok := util.TryGetObject(structured); ok {
		issues := parseValidationIssues(root)

		var hasErrors bool
		if boolPtr := util.GetBool(root, "hasErrors"); boolPtr != nil {
			hasErrors = *boolPtr
		} else {
			for _, issue := range issues {
				if strings.EqualFold(issue.Severity, "error") {
					hasErrors = true
					break
				}
			}
		}

		var summary string
		if successSummary != nil {
			summary = *successSummary
		} else {
			summary = buildValidationSummary(issues)
		}

		return &core.StructuredMemoryValidationResult{
			Success:   true,
			HasErrors: hasErrors,
			Issues:    issues,
			Summary:   summary,
		}
	}

	var summary string
	if successSummary != nil && strings.TrimSpace(*successSummary) != "" {
		summary = *successSummary
	} else {
		summary = text
	}

	return &core.StructuredMemoryValidationResult{
		Success:   true,
		HasErrors: strings.Contains(strings.ToLower(text), "error"),
		Summary:   summary,
	}
}

func parseValidationIssues(root map[string]any) []core.StructuredMemoryValidationIssue {
	rawArray, ok := util.TryGetArrayOrObjectArray(root, "issues")
	if !ok || len(rawArray) == 0 {
		return []core.StructuredMemoryValidationIssue{}
	}

	var result []core.StructuredMemoryValidationIssue

	for _, item := range rawArray {
		itemObj, ok := item.(map[string]any)
		if !ok {
			continue
		}

		var path string
		if relPath := util.GetString(itemObj, "relativePath"); relPath != nil {
			path = *relPath
		} else if p := util.GetString(itemObj, "path"); p != nil {
			path = *p
		}

		severity := getStringOrDefault(util.GetString(itemObj, "severity"), "")
		message := getStringOrDefault(util.GetString(itemObj, "message"), "")

		if strings.TrimSpace(message) != "" {
			result = append(result, core.StructuredMemoryValidationIssue{
				Severity: severity,
				Path:     path,
				Message:  message,
			})
		}
	}

	return result
}

func (p *FractalMemoryMcpProvider) RefreshIndex(ctx context.Context) (*core.StructuredMemoryValidationResult, error) {
	if p.config.Memory.Fractal == nil || !p.config.Memory.Fractal.AllowWrites {
		return &core.StructuredMemoryValidationResult{Error: "fractal Memory index refresh is disabled by configuration"}, nil
	}

	result, err := p.callTool(ctx, "memory_index_refresh", map[string]any{})
	if err != nil {
		return &core.StructuredMemoryValidationResult{Error: err.Error()}, nil
	}

	if result.Success {
		successSummary := "fractal Memory index refresh completed"
		return parseValidationResult(result.StructuredContent, result.Text, &successSummary), nil
	}

	return &core.StructuredMemoryValidationResult{Error: result.Error}, nil
}
