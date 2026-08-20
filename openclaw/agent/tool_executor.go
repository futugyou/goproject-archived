package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/extensions_ai/abstractions"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type OpenClawToolExecutor struct {
	toolsByName           map[string]core.ITool
	toolDeclarations      []abstractions.AITool
	toolsMutationLock     sync.RWMutex
	toolTimeout           time.Duration
	requireToolApproval   bool
	approvalRequiredTools map[string]struct{}
	hooks                 []core.IToolHook
	interceptors          []core.IToolResultInterceptor
	metrics               *core.RuntimeMetrics
	logger                *slog.Logger
	config                *core.GatewayConfig
	toolSandbox           core.IToolSandbox
	toolUsageTracker      *core.ToolUsageTracker
	executionRouter       *ToolExecutionRouter
	toolPresetResolver    core.IToolPresetResolver
	auditLog              *core.ToolAuditLog
	redaction             core.IRedactionPipeline
	sentinelSubstitution  core.ISentinelSubstitutionService
	toolGovernance        core.IToolGovernanceService
	planExecuteVerify     core.IPlanExecuteVerifyOrchestrator
	metaInvokeExecutor    func(ctx context.Context, session core.Session, toolName string, payload *string) (string, error)
}

func NewOpenClawToolExecutor(
	tools []core.ITool,
	toolTimeoutSeconds int,
	requireToolApproval bool,
	approvalRequiredTools []string,
	hooks []core.IToolHook,
	metrics *core.RuntimeMetrics,
	logger *slog.Logger,
	config *core.GatewayConfig,
	toolSandbox core.IToolSandbox,
	toolUsageTracker *core.ToolUsageTracker,
	executionRouter *ToolExecutionRouter,
	toolPresetResolver core.IToolPresetResolver,
	auditLog *core.ToolAuditLog,
	redaction core.IRedactionPipeline,
	sentinelSubstitution core.ISentinelSubstitutionService,
	toolGovernance core.IToolGovernanceService,
	planExecuteVerify core.IPlanExecuteVerifyOrchestrator,
	metaInvokeExecutor func(ctx context.Context, session core.Session, toolName string, payload *string) (string, error),
	interceptors []core.IToolResultInterceptor,
) *OpenClawToolExecutor {
	toolsMap := make(map[string]core.ITool, len(tools))
	declarations := make([]abstractions.AITool, 0, len(tools))

	for _, t := range tools {
		toolsMap[t.Name()] = t
		declarations = append(declarations, CreateDeclaration(t))
	}

	approvalSet := make(map[string]struct{})
	for _, item := range approvalRequiredTools {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			approvalSet[NormalizeApprovalToolName(trimmed)] = struct{}{}
		}
	}

	if config == nil {
		config = &core.GatewayConfig{
			Tooling: core.ToolingConfig{
				ToolTimeoutSeconds:    toolTimeoutSeconds,
				RequireToolApproval:   requireToolApproval,
				ApprovalRequiredTools: append([]string(nil), approvalRequiredTools...),
			},
		}
	}

	if logger == nil {
		logger = slog.Default()
	}

	if executionRouter == nil {
		executionRouter = NewToolExecutionRouter(config, toolSandbox, logger)
	}
	if redaction == nil {
		redaction = &core.NoopRedactionPipeline{}
	}
	if sentinelSubstitution == nil {
		sentinelSubstitution = &core.NoopSentinelSubstitutionService{}
	}
	if toolGovernance == nil {
		toolGovernance = &core.NoopToolGovernanceService{}
	}
	if planExecuteVerify == nil {
		planExecuteVerify = core.NoopPlanExecuteVerifyOrchestratorInstance
	}

	return &OpenClawToolExecutor{
		toolsByName:           toolsMap,
		toolDeclarations:      declarations,
		toolTimeout:           time.Duration(toolTimeoutSeconds) * time.Second,
		requireToolApproval:   requireToolApproval,
		approvalRequiredTools: approvalSet,
		hooks:                 hooks,
		interceptors:          interceptors,
		metrics:               metrics,
		logger:                logger,
		config:                config,
		toolSandbox:           toolSandbox,
		toolUsageTracker:      toolUsageTracker,
		executionRouter:       executionRouter,
		toolPresetResolver:    toolPresetResolver,
		auditLog:              auditLog,
		redaction:             redaction,
		sentinelSubstitution:  sentinelSubstitution,
		toolGovernance:        toolGovernance,
		planExecuteVerify:     planExecuteVerify,
		metaInvokeExecutor:    metaInvokeExecutor,
	}
}

func NormalizeApprovalToolName(toolName string) string {
	if toolName == "file_write" {
		return "write_file"
	}
	return toolName
}

func CreateDeclaration(t core.ITool) abstractions.AITool {
	// TODO: extensions_ai
	panic("unimplemented")
}

func (e *OpenClawToolExecutor) ToolDeclarations() []abstractions.AITool {
	e.toolsMutationLock.RLock()
	defer e.toolsMutationLock.RUnlock()

	res := make([]abstractions.AITool, len(e.toolDeclarations))
	copy(res, e.toolDeclarations)
	return res
}

func (e *OpenClawToolExecutor) GetToolDeclarations(session core.Session) []abstractions.AITool {
	e.toolsMutationLock.RLock()
	declarations := make([]abstractions.AITool, len(e.toolDeclarations))
	copy(declarations, e.toolDeclarations)

	toolNames := make([]string, 0, len(e.toolsByName))
	for name := range e.toolsByName {
		toolNames = append(toolNames, name)
	}
	e.toolsMutationLock.RUnlock()

	var preset *core.ResolvedToolPreset
	if e.toolPresetResolver != nil {
		preset = new(core.ResolvedToolPreset)
		*preset = e.toolPresetResolver.Resolve(session, toolNames)
	}

	var filtered []abstractions.AITool
	for _, decl := range declarations {
		if IsToolAllowedForSession(session, decl.GetName(), preset) {
			filtered = append(filtered, decl)
		}
	}
	return filtered
}

func IsToolAllowedForSession(session core.Session, toolName string, preset *core.ResolvedToolPreset) bool {
	// DisableTools routing decisions intentionally expose no tools to the model.
	if session.RouteToolsDisabled {
		return false
	}

	if preset != nil && !preset.AllowedTools.Contains(toolName) {
		return false
	}

	if len(session.RouteAllowedTools) > 0 {
		return slices.Contains(session.RouteAllowedTools, toolName)
	}

	return true
}

func (e *OpenClawToolExecutor) SupportsStreaming(toolName string) bool {
	e.toolsMutationLock.RLock()
	defer e.toolsMutationLock.RUnlock()

	tool, exists := e.toolsByName[toolName]
	if !exists {
		return false
	}
	_, ok := tool.(core.IStreamingTool)
	return ok
}

func LooksLikeOperatorAuthFailure(message string) bool {
	return strings.Contains(message, "operator auth") ||
		strings.Contains(message, "operator authentication") ||
		strings.Contains(message, "operator token") ||
		strings.Contains(message, "browser-session") ||
		strings.Contains(message, "account-token") ||
		strings.Contains(message, "bootstrap token") ||
		strings.Contains(message, "current surface")
}

func BuildFailureNextStep(toolName, failureCode string) string {
	switch failureCode {
	case core.ToolFailureCodesOperatorAuthRequired:
		return "Authenticate with a browser session or operator token on this surface before retrying the tool."
	case core.ToolFailureCodesBrowserBackendMissing:
		return "Configure a browser execution backend or sandbox, or disable the browser tool in this runtime."
	case core.ToolFailureCodesRuntimeCapabilityUnavailable:
		if toolName == "shell" {
			return "Configure the required sandbox or execution backend for shell, or relax the tool policy for trusted local sessions."
		} else {
			return "Configure the required execution backend or sandbox for this tool, or disable the tool in this runtime."
		}
	default:
		return ""
	}
}

func ResourcePathContainsReparsePoint(skillLocation, resourceAbsolutePath string) bool {
	if skillLocation == "" {
		return false
	}

	skillRoot, err := filepath.Abs(skillLocation)
	if err != nil {
		return true
	}

	resolved, err := filepath.Abs(resourceAbsolutePath)
	if err != nil {
		return true
	}

	relative, err := filepath.Rel(skillRoot, resolved)
	if err != nil {
		return true
	}

	if relative == ".." ||
		strings.HasPrefix(relative, ".."+string(os.PathSeparator)) ||
		strings.HasPrefix(relative, "../") ||
		filepath.IsAbs(relative) {
		return true
	}

	var current = skillRoot
	for _, segment := range strings.FieldsFunc(relative, func(r rune) bool {
		return r == os.PathSeparator || r == '\\'
	}) {
		current = filepath.Join(current, segment)
		flag, err := util.IsReparsePoint(current)
		if flag || err != nil {
			return true
		}
	}

	return false
}

func IsPathWithinSkillRoot(resourceAbsolutePath string, skill core.SkillDefinition) bool {
	if skill.Location == "" {
		return true
	}

	skillRoot, err := filepath.Abs(skill.Location)
	if err != nil {
		return true
	}

	resolved, err := filepath.Abs(resourceAbsolutePath)
	if err != nil {
		return true
	}

	rootWithSep := skillRoot
	if !strings.HasSuffix(skillRoot, string(os.PathSeparator)) {
		rootWithSep += string(os.PathSeparator)
	}

	return strings.HasPrefix(resolved, rootWithSep)
}

func NormalizeSkillExecOutput(parseMode, stdout, stderr string) (string, error) {
	var output = stdout
	if stdout == "" {
		output = stderr
	}

	var trimmed = strings.TrimSpace(output)

	if parseMode == "json" {
		var data map[string]any
		if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
			return trimmed, err
		}
	}

	return trimmed, nil
}

func ResolveScriptCommand(scriptAbsolutePath string) (string, []string) {
	var extension = filepath.Ext(scriptAbsolutePath)
	if extension == ".ps1" {
		prefixArguments := []string{"-NoProfile", "-File", scriptAbsolutePath}
		o := "pwsh"
		if runtime.GOOS != "windows" {
			o = "pwsh"
		}
		return o, prefixArguments
	}

	prefixArguments := []string{scriptAbsolutePath}
	return scriptAbsolutePath, prefixArguments
}

func ResolveSkillWorkingDirectory(skill core.SkillDefinition, workingDirectory string) (string, error) {
	skillRoot, err := filepath.Abs(skill.Location)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(skillRoot, workingDirectory))
	if err != nil {
		return "", err
	}

	rootWithSep := skillRoot
	if !strings.HasSuffix(rootWithSep, string(os.PathSeparator)) {
		rootWithSep += string(os.PathSeparator)
	}

	if candidate != skillRoot && !strings.HasPrefix(candidate, rootWithSep) {
		return "", errors.New("skill_exec working directory must remain inside the skill root.")
	}

	return candidate, nil
}

func ResolveSkillScript(skill core.SkillDefinition, entrypoint string) *core.SkillResource {
	for _, resource := range skill.Resources {
		if resource.Kind == core.SkillResourceKind_Script &&
			(resource.Name == entrypoint ||
				resource.RelativePath == fmt.Sprintf("scripts/%s", entrypoint) ||
				resource.RelativePath == strings.ReplaceAll(entrypoint, "\\", "/")) {
			return &resource
		}
	}

	return nil
}

func ClassifyToolFailureCode(tool core.ITool, message string) string {
	if LooksLikeOperatorAuthFailure(message) {
		return core.ToolFailureCodesOperatorAuthRequired
	}

	if policy, ok := tool.(core.IToolLocalExecutionPolicy); ok &&
		!policy.LocalExecutionSupported() &&
		message == policy.LocalExecutionUnavailableMessage() {
		return policy.LocalExecutionUnavailableFailureCode()
	}

	var toolName = tool.Name()
	if toolName == "browser" {
		if strings.Contains(message, "execution backend") || strings.Contains(message, "Local Playwright execution is unavailable") {
			return core.ToolFailureCodesBrowserBackendMissing
		} else {
			return core.ToolFailureCodesRuntimeCapabilityUnavailable
		}
	}

	if strings.Contains(message, "sandbox") || strings.Contains(message, "execution backend") {
		return core.ToolFailureCodesRuntimeCapabilityUnavailable
	} else {
		return core.ToolFailureCodesToolFailed
	}
}

func BlocksPlanExecuteVerifyDecision(decision string) bool {
	return decision != core.PlanExecuteVerifyDecisionKindsProceed && decision != core.PlanExecuteVerifyDecisionKindsRequireApproval
}

func InvokeTool(ctx context.Context, tool core.ITool, argsJson string, toolContext *core.ToolExecutionContext) string {
	if contextualTool, ok := tool.(core.IToolWithContext); ok && toolContext != nil {
		return contextualTool.ExecuteContext(ctx, argsJson, *toolContext)
	}

	return tool.Execute(ctx, argsJson)
}

func (o *OpenClawToolExecutor) ExecuteToolWithTimeout(ctx context.Context, tool core.ITool, argsJson string, session core.Session, turnCtx *core.TurnContext) string {
	var execContext = &core.ToolExecutionContext{
		Session:     &session,
		TurnContext: turnCtx,
	}

	if o.toolTimeout <= 0 {
		return InvokeTool(ctx, tool, argsJson, execContext)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, o.toolTimeout)
	defer cancel()

	return InvokeTool(timeoutCtx, tool, argsJson, execContext)
}
