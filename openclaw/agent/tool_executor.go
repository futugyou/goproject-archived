package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/extensions_ai/abstractions"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
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
		preset = e.toolPresetResolver.Resolve(session, toolNames)
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

func GetLocalExecutionUnavailableFailureCode(tool core.ITool) string {
	if policy, ok := tool.(core.IToolLocalExecutionPolicy); ok && !policy.LocalExecutionSupported() {
		return policy.LocalExecutionUnavailableFailureCode()
	}

	return core.ToolFailureCodesRuntimeCapabilityUnavailable
}

func GetLocalExecutionUnavailableMessage(tool core.ITool) string {
	if policy, ok := tool.(core.IToolLocalExecutionPolicy); ok && !policy.LocalExecutionSupported() {
		return policy.LocalExecutionUnavailableMessage()
	}

	return fmt.Sprintf("Error: Tool '%s' requires a configured execution backend or sandbox in this runtime. Local execution is unavailable.", tool.Name())
}

func CreateLocalExecutionUnavailableException(tool core.ITool) []string {
	return []string{
		GetLocalExecutionUnavailableMessage(tool),
		GetLocalExecutionUnavailableFailureCode(tool),
	}
}

func CreateImmediateResult(
	toolName string,
	argsJson string,
	result string,
	callId string,
	resultStatus string,
	failureCode string,
	failureMessage string,
	nextStep string,
	governanceDecision *core.GovernanceDecision) *ToolExecutionResult {
	var invocation = core.ToolInvocation{
		CallId:         callId,
		ToolName:       toolName,
		Arguments:      argsJson,
		Result:         result,
		ResultStatus:   resultStatus,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
		NextStep:       nextStep,
	}

	if governanceDecision != nil {
		invocation.GovernanceAllowed = &governanceDecision.Allowed
		invocation.GovernanceAction = governanceDecision.Action.String()
		invocation.GovernanceReason = governanceDecision.Reason
		invocation.GovernancePolicyId = governanceDecision.PolicyId
		invocation.GovernanceRuleId = governanceDecision.RuleId
		invocation.GovernanceTrustScore = governanceDecision.TrustScore
		invocation.GovernanceEvaluationMs = governanceDecision.EvaluationMs
		invocation.GovernanceUnavailable = &governanceDecision.IsUnavailable
	}

	return &ToolExecutionResult{
		Invocation:     invocation,
		ResultText:     result,
		ResultStatus:   resultStatus,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
		NextStep:       nextStep,
	}
}

func (o *OpenClawToolExecutor) ExecuteSkillEntrypoint(
	ctx context.Context,
	skill core.SkillDefinition,
	entrypoint string,
	arguments []string,
	workingDirectory string,
	parseMode string,
	stdin string) *ToolExecutionResult {
	var script = ResolveSkillScript(skill, entrypoint)
	if script == nil {
		return CreateImmediateResult(
			"skill_exec",
			"{}",
			fmt.Sprintf("Meta step skill_exec entrypoint '%s' was not found in skill '%s'.", entrypoint, skill.Name),
			"",
			core.ToolResultStatusesFailed,
			"skill_exec_entrypoint_not_found",
			fmt.Sprintf("Entrypoint '%s' was not found.", entrypoint), "", nil)
	}

	if !IsPathWithinSkillRoot(script.AbsolutePath, skill) || ResourcePathContainsReparsePoint(skill.Location, script.AbsolutePath) {
		return CreateImmediateResult(
			"skill_exec",
			"{}",
			fmt.Sprintf("Meta step skill_exec entrypoint '%s' was rejected because it resolves outside the skill root or through a reparse point.", entrypoint),
			"",
			core.ToolResultStatusesBlocked,
			"skill_exec_entrypoint_denied",
			fmt.Sprintf("Entrypoint '%s' failed skill root validation.", entrypoint),
			"", nil)
	}

	command, commandArguments := ResolveScriptCommand(script.AbsolutePath)
	allArguments := append(commandArguments, command)
	resolvedWorkingDirectory, err := ResolveSkillWorkingDirectory(skill, workingDirectory)
	if err != nil {
		return CreateImmediateResult(
			"skill_exec",
			"{}",
			fmt.Sprintf("Meta step skill_exec failed: %s", err.Error()),
			"",
			core.ToolResultStatusesFailed,
			"skill_exec_failed",
			err.Error(), "", nil)
	}

	executionResult, err := o.executionRouter.Execute(ctx, &core.ExecutionRequest{
		ToolName:           "skill_exec",
		BackendName:        o.config.Execution.DefaultBackend,
		Command:            command,
		Arguments:          allArguments,
		StandardInput:      stdin,
		WorkingDirectory:   resolvedWorkingDirectory,
		Environment:        map[string]string{},
		AllowLocalFallback: true,
	}, "")
	if err != nil {
		return CreateImmediateResult(
			"skill_exec",
			"{}",
			fmt.Sprintf("Meta step skill_exec failed: %s", err.Error()),
			"",
			core.ToolResultStatusesFailed,
			"skill_exec_failed",
			err.Error(), "", nil)
	}

	output, err := NormalizeSkillExecOutput(parseMode, executionResult.Stdout, executionResult.Stderr)
	if err != nil {
		return CreateImmediateResult(
			"skill_exec",
			"{}",
			fmt.Sprintf("Meta step skill_exec failed: %s", err.Error()),
			"",
			core.ToolResultStatusesFailed,
			"skill_exec_failed",
			err.Error(), "", nil)
	}
	if executionResult.TimedOut {
		return CreateImmediateResult(
			"skill_exec",
			"{}",
			output,
			"",
			core.ToolResultStatusesFailed,
			"step_timeout",
			"skill_exec timed out.", "", nil)
	}

	if executionResult.ExitCode != 0 {
		return CreateImmediateResult(
			"skill_exec",
			"{}",
			output,
			"",
			core.ToolResultStatusesFailed,
			"skill_exec_failed",
			fmt.Sprintf("skill_exec exited with code %d.", executionResult.ExitCode),
			"", nil)
	}

	return CreateImmediateResult("skill_exec", "{}", output, "", "", "", "", "", nil)
}

func IsLocalExecutionDisabled(tool core.ITool) bool {
	if policy, ok := tool.(core.IToolLocalExecutionPolicy); ok && !policy.LocalExecutionSupported() {
		return true
	}
	return false
}

func (o *OpenClawToolExecutor) ExecuteToolWithRouting(
	ctx context.Context,
	tool core.ITool,
	argsJson string,
	session core.Session,
	turnCtx *core.TurnContext,
) (string, error) {
	route, template, legacySandboxRoute, sandboxMode, ok := o.executionRouter.TryResolveRoute(tool)
	if !ok {
		if IsLocalExecutionDisabled(tool) {
			return "", errors.New(strings.Join(CreateLocalExecutionUnavailableException(tool), "\n"))
		}

		return o.ExecuteToolWithTimeout(ctx, tool, argsJson, session, turnCtx), nil
	}

	sandboxCapableTool, ok := tool.(core.ISandboxCapableTool)
	if !ok {
		return o.ExecuteToolWithTimeout(ctx, tool, argsJson, session, turnCtx), nil
	}

	backendName := o.config.Execution.DefaultBackend
	fallbackBackend := ""
	requireWorkspace := false
	if route != nil {
		requireWorkspace = route.RequireWorkspace
		if route.Backend != "" {
			backendName = route.Backend
		}
		if route.FallbackBackend != "" {
			fallbackBackend = route.FallbackBackend
		}
	}

	if sandboxMode == core.ToolSandboxMode_Require && !legacySandboxRoute && route == nil {
		return "", fmt.Errorf("Error: Tool '%s' requires sandboxing but no sandbox provider is configured.", tool.Name())
	}

	if backendName == "local" && IsLocalExecutionDisabled(tool) {
		return "", errors.New(strings.Join(CreateLocalExecutionUnavailableException(tool), "\n"))
	}

	if backendName == "local" && !legacySandboxRoute {
		return o.ExecuteToolWithTimeout(ctx, tool, argsJson, session, turnCtx), nil
	}

	if o.executionRouter.RequiresWorkspace(backendName) && o.config.Tooling.WorkspaceRoot != "" {
		return "", fmt.Errorf("Error: Tool '%s' is configured to use execution backend '%s' but Tooling.WorkspaceRoot is not set.", tool.Name(), backendName)
	}

	if legacySandboxRoute && template != nil && *template != "" && fallbackBackend != "" {
		return "", fmt.Errorf("Error: Tool '%s' requires sandboxing but no sandbox template is configured.", tool.Name())
	}

	if legacySandboxRoute && o.toolSandbox == nil {
		return "", fmt.Errorf("Error: Tool '%s' requires sandboxing but no sandbox provider is configured.", tool.Name())
	}

	sandboxRequest, err := sandboxCapableTool.CreateSandboxRequest(argsJson)
	if err != nil {
		return handleToolExecutorError(ctx, legacySandboxRoute, route, tool, backendName, sandboxMode, o, argsJson, session, turnCtx, err)
	}
	if sandboxRequest.LeaseKey == "" {
		sandboxRequest.LeaseKey = fmt.Sprintf("%s:%s", session.Id, tool.Name())
	}

	if sandboxRequest.Template == "" && template != nil {
		sandboxRequest.Template = *template
	}

	sandboxRequest.TimeToLiveSeconds = core.ResolveTimeToLiveSeconds(
		o.config,
		tool.Name(),
		&sandboxRequest.TimeToLiveSeconds)

	executionResult, err := o.executionRouter.Execute(ctx, &core.ExecutionRequest{
		ToolName:           tool.Name(),
		BackendName:        backendName,
		Command:            sandboxRequest.Command,
		Arguments:          sandboxRequest.Arguments,
		LeaseKey:           sandboxRequest.LeaseKey,
		Environment:        map[string]string{},
		WorkingDirectory:   sandboxRequest.WorkingDirectory,
		Template:           sandboxRequest.Template,
		TimeToLiveSeconds:  &sandboxRequest.TimeToLiveSeconds,
		RequireWorkspace:   requireWorkspace,
		AllowLocalFallback: !IsLocalExecutionDisabled(tool),
	}, fallbackBackend)
	if err != nil {
		return handleToolExecutorError(ctx, legacySandboxRoute, route, tool, backendName, sandboxMode, o, argsJson, session, turnCtx, err)
	}

	var sandboxResult = core.SandboxResult{
		ExitCode: executionResult.ExitCode,
		Stdout:   executionResult.Stdout,
		Stderr:   executionResult.Stderr,
	}
	return sandboxCapableTool.FormatSandboxResult(argsJson, sandboxResult), nil
}

func handleToolExecutorError(ctx context.Context, legacySandboxRoute bool, route *core.ExecutionToolRouteConfig, tool core.ITool, backendName string, sandboxMode core.ToolSandboxMode, o *OpenClawToolExecutor, argsJson string, session core.Session, turnCtx *core.TurnContext, err error) (string, error) {
	if legacySandboxRoute || (route != nil && route.FallbackBackend != "") {
		if IsLocalExecutionDisabled(tool) {
			if legacySandboxRoute {
				return "", fmt.Errorf("Error: Tool '%s' requires sandboxing but the sandbox provider is unavailable.", tool.Name())
			} else {
				return "", fmt.Errorf("Error: Tool '%s' requires execution backend '%s' but the provider is unavailable.", tool.Name(), backendName)
			}
		}
		if sandboxMode == core.ToolSandboxMode_Require {
			return "", fmt.Errorf("Error: Tool '%s' requires sandboxing but the sandbox provider is unavailable.", tool.Name())
		}
		return o.ExecuteToolWithTimeout(ctx, tool, argsJson, session, turnCtx), nil
	}
	return "", err
}

func (o *OpenClawToolExecutor) ExecuteSandboxWithTimeout(ctx context.Context, request core.SandboxExecutionRequest) (*core.SandboxResult, error) {
	if o.toolSandbox == nil {
		return nil, fmt.Errorf("Error: Tool requires sandboxing but no sandbox provider is configured.")
	}

	if o.toolTimeout <= 0 {
		return o.toolSandbox.Execute(ctx, request)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, o.toolTimeout)
	defer cancel()

	return o.toolSandbox.Execute(timeoutCtx, request)
}

func ResolveToolActionDescriptor(tool core.ITool, argsJson string) *core.ToolActionDescriptor {
	descriptorProvider, ok := tool.(core.IToolActionDescriptorProvider)
	if ok {
		r, _ := descriptorProvider.ResolveActionDescriptor(argsJson)
		return r
	}
	return core.ToolActionPolicyResolverInstance.Resolve(tool.Name(), argsJson)
}

func (o *OpenClawToolExecutor) ExecuteStreamingToolCollect(
	ctx context.Context,
	tool core.IStreamingTool,
	argsJson string,
	onDelta func(string) error,
) (string, error) {
	if o.toolTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.toolTimeout)
		defer cancel()
	}
	MaxChars := 1_000_000
	var sb = strings.Builder{}
	chunks := []string{}

	stm, err := tool.ExecuteStreaming(ctx, argsJson)
	if err != nil {
		return "", err
	}

Loop:
	for {
		select {
		case chunk, ok := <-stm:
			if !ok {
				break Loop
			}
			chunks = append(chunks, chunk)
			if sb.Len() < MaxChars {
				var remaining = MaxChars - sb.Len()
				if len(chunk) <= remaining {
					sb.WriteString(chunk)
				} else {
					sb.WriteString(chunk[:remaining])
				}
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if sb.Len() >= MaxChars {
		sb.WriteString("…")
	}

	var result = sb.String()
	var redactedResult = o.redaction.Redact(result)
	if redactedResult == result {
		for _, chunk := range chunks {
			onDelta(chunk)
		}
	} else if redactedResult != "" {
		onDelta(redactedResult)
	}

	return result, nil
}

func (o *OpenClawToolExecutor) RecordImmediateGovernanceAudit(
	tool core.ITool,
	session core.Session,
	turnCtx *core.TurnContext,
	argumentsJson,
	result string,
	decision *core.GovernanceDecision) {
	if o.auditLog == nil {
		return
	}

	entry := &core.ToolAuditEntry{
		TimestampUtc:   time.Now().UTC(),
		ToolName:       tool.Name(),
		SessionId:      session.Id,
		ChannelId:      session.ChannelId,
		SenderId:       session.SenderId,
		CorrelationId:  turnCtx.CorrelationId,
		Failed:         true,
		ArgumentsBytes: len(argumentsJson),
		ResultBytes:    len(result),
	}

	if decision != nil {
		entry.GovernanceAllowed = decision.Allowed
		entry.GovernanceAction = decision.Action.String()
		entry.GovernanceReason = decision.Reason
		entry.GovernancePolicyId = decision.PolicyId
		entry.GovernanceRuleId = decision.RuleId
		entry.GovernanceTrustScore = util.Deref(decision.TrustScore)
		entry.GovernanceEvaluationMs = util.Deref(decision.EvaluationMs)
		entry.GovernanceUnavailable = decision.IsUnavailable
	}
	o.auditLog.Record(entry)
}

func (o *OpenClawToolExecutor) RecordGovernanceResult(
	ctx context.Context,
	toolContext core.ToolGovernanceContext,
	decision core.GovernanceDecision,
	resultStatus,
	failureCode,
	failureMessage string,
	failed,
	timedOut bool,
	duration time.Duration,
	resultBytes int,
) error {
	err := o.toolGovernance.RecordResult(
		ctx,
		toolContext,
		decision,
		core.ToolGovernanceExecutionResult{
			ResultStatus:   resultStatus,
			FailureCode:    failureCode,
			FailureMessage: failureMessage,
			Failed:         failed,
			TimedOut:       timedOut,
			DurationMs:     float64(duration.Milliseconds()),
			ResultBytes:    resultBytes,
		})

	if err != nil {
		o.logger.Warn("Governance result audit failed for tool",
			"CorrelationId", toolContext.CorrelationId,
			"Tool", toolContext.ToolName,
		)
	}

	return err
}

func TryGetMetaInvokeArguments(argsJson string) (skill string, input string, ok bool) {
	if argsJson == "" {
		return
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(argsJson), &doc); err != nil {
		return
	}

	skill, ok = doc["skill"].(string)
	if !ok || skill == "" {
		return
	}

	input, ok = doc["input"].(string)
	if !ok || input == "" {
		return
	}

	ok = true
	return
}

func ApplyGovernanceActivityTags(span trace.Span, decision *core.GovernanceDecision) {
	if span == nil || decision == nil {
		return
	}
	tags := []attribute.KeyValue{
		attribute.Bool("tool.governance.allowed", decision.Allowed),
		attribute.String("tool.governance.action", decision.Action.String()),
		attribute.Bool("tool.governance.unavailable", decision.IsUnavailable),
	}
	if decision.PolicyId != "" {
		tags = append(tags, attribute.String("tool.governance.policy_id", decision.PolicyId))
	}
	if decision.RuleId != "" {
		tags = append(tags, attribute.String("tool.governance.rule_id", decision.RuleId))
	}
	if decision.TrustScore != nil {
		tags = append(tags, attribute.Float64("tool.governance.trust_score", *decision.TrustScore))
	}
	if decision.EvaluationMs != nil {
		tags = append(tags, attribute.Float64("tool.governance.evaluation_ms", *decision.EvaluationMs))
	}
	span.SetAttributes(
		tags...,
	)
}

func (o *OpenClawToolExecutor) Execute(
	ctx context.Context,
	toolName,
	argsJson,
	callId string,
	session core.Session,
	turnCtx *core.TurnContext,
	isStreaming bool,
	approvalCallback ToolApprovalCallback,
	onDelta func(string) error,
	toolCallCount int) (*ToolExecutionResult, error) {
	var span trace.Span
	ctx, span = core.Tracer.Start(ctx, "Agent.ExecuteTool", trace.WithAttributes(
		attribute.String("tool.name", toolName),
	))
	defer span.End()

	var persistedArgsJson = o.redaction.Redact(argsJson)

	var tool core.ITool
	o.toolsMutationLock.Lock()
	if t, ok := o.toolsByName[toolName]; ok {
		tool = t
	}
	o.toolsMutationLock.Unlock()

	if tool == nil {
		return CreateImmediateResult(
			toolName,
			persistedArgsJson,
			"Error: Unknown tool",
			callId,
			core.ToolResultStatusesFailed,
			core.ToolFailureCodesToolFailed,
			"Unknown tool.",
			"Use one of the tools declared for this session.", nil), nil
	}

	if session.RouteToolsDisabled {
		var disabledMessage = fmt.Sprintf("Tool '%s' is disabled for this routed turn.", tool.Name())
		return CreateImmediateResult(
			toolName,
			persistedArgsJson,
			disabledMessage,
			callId,
			core.ToolResultStatusesBlocked,
			core.ToolFailureCodesPresetBlocked,
			disabledMessage,
			"Continue without tools for this routed turn.", nil), nil
	}

	var preset *core.ResolvedToolPreset
	if o.toolPresetResolver != nil {
		preset = o.toolPresetResolver.Resolve(session, slices.Collect(maps.Keys(o.toolsByName)))
	}

	if !IsToolAllowedForSession(session, tool.Name(), preset) {
		deniedByPreset := fmt.Sprintf("Tool '%s' is not allowed for this session.", tool.Name())
		if preset != nil {
			deniedByPreset = fmt.Sprintf("Tool '%s' is not allowed for preset '%s'.", tool.Name(), preset.PresetId)
		}

		return CreateImmediateResult(
			toolName,
			persistedArgsJson,
			deniedByPreset,
			callId,
			core.ToolResultStatusesBlocked,
			core.ToolFailureCodesPresetBlocked,
			deniedByPreset,
			"Use a broader preset on this surface, or change the session preset if that access is intentional.", nil), nil
	}

	toolGovernanceDescriptorCatalog := core.NewToolGovernanceDescriptorCatalog()
	var approvalDescriptor = ResolveToolActionDescriptor(tool, persistedArgsJson)
	var governanceDescriptor = toolGovernanceDescriptorCatalog.Resolve(tool.Name(), tool.Description(), approvalDescriptor)
	var governanceContext = core.ToolGovernanceContext{
		AgentId:          session.Id,
		SessionId:        session.Id,
		ChannelId:        session.ChannelId,
		SenderId:         session.SenderId,
		CorrelationId:    turnCtx.CorrelationId,
		CallId:           callId,
		ToolName:         tool.Name(),
		ArgumentsJson:    persistedArgsJson,
		ActionDescriptor: approvalDescriptor,
		Descriptor:       governanceDescriptor,
		IsStreaming:      isStreaming,
	}
	governanceDecision, err := o.toolGovernance.Authorize(ctx, governanceContext)
	if err != nil {
		return nil, err
	}
	ApplyGovernanceActivityTags(span, governanceDecision)

	if governanceDecision.RedactedArgumentsJson != "" {
		if util.IsValidJson(governanceDecision.RedactedArgumentsJson) {
			persistedArgsJson = o.redaction.Redact(governanceDecision.RedactedArgumentsJson)
		} else {
			o.logger.Warn(
				"Governance returned invalid redacted tool arguments. Keeping existing redacted arguments.",
				"CorrelationId", turnCtx.CorrelationId,
				"Tool", tool.Name(),
			)
		}
	}

	if governanceDecision.Action == core.GovernanceActionRedact && governanceDecision.ReplacementArgumentsJson != "" {
		if !util.IsValidJson(governanceDecision.ReplacementArgumentsJson) {
			var invalidReplacementMessage = "Governance returned invalid replacement tool arguments."
			o.RecordImmediateGovernanceAudit(
				tool,
				session,
				turnCtx,
				persistedArgsJson,
				invalidReplacementMessage,
				governanceDecision)
			return CreateImmediateResult(
				toolName,
				persistedArgsJson,
				invalidReplacementMessage,
				callId,
				core.ToolResultStatusesBlocked,
				core.ToolFailureCodesGovernanceDenied,
				invalidReplacementMessage,
				"Review the governance sidecar redaction response.",
				governanceDecision), nil
		}

		argsJson = governanceDecision.ReplacementArgumentsJson
		persistedArgsJson = o.redaction.Redact(governanceDecision.ReplacementArgumentsJson)
		approvalDescriptor = ResolveToolActionDescriptor(tool, persistedArgsJson)
		governanceDescriptor = toolGovernanceDescriptorCatalog.Resolve(tool.Name(), tool.Description(), approvalDescriptor)
	}

	governanceContext.ArgumentsJson = persistedArgsJson
	governanceContext.ActionDescriptor = approvalDescriptor
	governanceContext.Descriptor = governanceDescriptor

	if governanceDecision.Action != core.GovernanceActionRequireApproval && !governanceDecision.Allowed {
		var deniedByGovernance = governanceDecision.Reason
		if deniedByGovernance == "" {
			deniedByGovernance = "Tool invocation denied by governance policy."
		}
		governanceFailureCode := core.ToolFailureCodesGovernanceDenied
		if governanceDecision.IsUnavailable {
			governanceFailureCode = core.ToolFailureCodesGovernanceUnavailable
		}

		o.logger.Warn(
			"Tool invocation denied by governance.",
			"CorrelationId", turnCtx.CorrelationId,
			"Tool", tool.Name,
			"Reason", deniedByGovernance)
		o.RecordImmediateGovernanceAudit(
			tool,
			session,
			turnCtx,
			persistedArgsJson,
			deniedByGovernance,
			governanceDecision)

		nextStep := "Adjust the request or governance policy before retrying."
		if governanceFailureCode == core.ToolFailureCodesGovernanceUnavailable {
			nextStep = "Check governance sidecar availability or adjust fail-open/fail-closed policy before retrying."
		}
		return CreateImmediateResult(
			toolName,
			persistedArgsJson,
			deniedByGovernance,
			callId,
			core.ToolResultStatusesBlocked,
			governanceFailureCode,
			deniedByGovernance,
			nextStep,
			governanceDecision), nil
	}

	var hookCtx = core.ToolHookContext{
		SessionId:     session.Id,
		ChannelId:     session.ChannelId,
		SenderId:      session.SenderId,
		CorrelationId: turnCtx.CorrelationId,
		ToolName:      tool.Name(),
		ArgumentsJson: persistedArgsJson,
		IsStreaming:   isStreaming,
	}

	for _, hook := range o.hooks {
		allowed := false
		if ctxHook, ok := hook.(core.IToolHookWithContext); ok {
			allowed = ctxHook.BeforeExecuteContext(ctx, hookCtx)
		} else {
			allowed = hook.BeforeExecute(ctx, tool.Name(), persistedArgsJson)
		}

		if !allowed {
			var deniedByHook = fmt.Sprintf("Tool execution denied by hook: %s", hook.Name())
			return CreateImmediateResult(
				toolName,
				persistedArgsJson,
				deniedByHook,
				callId,
				core.ToolResultStatusesBlocked,
				core.ToolFailureCodesToolFailed,
				deniedByHook,
				"",
				governanceDecision), nil
		}
	}

	var normalizedToolName = NormalizeApprovalToolName(tool.Name())
	explicitlyConfiguredApproval := false
	for _, item := range o.config.Tooling.ApprovalRequiredTools {
		if NormalizeApprovalToolName(item) == normalizedToolName {
			explicitlyConfiguredApproval = true
			break
		}
	}

	presetRequiresApproval := false
	if preset != nil {
		presetRequiresApproval = preset.ApprovalRequiredTools.Contains(tool.Name())
	}

	var defaultActionAwareApproval = o.requireToolApproval &&
		core.ToolActionPolicyResolverInstance.SupportsActionAwareApproval(tool.Name()) &&
		(approvalDescriptor.IsMutation || approvalDescriptor.RequiresApproval)
	var listedApproval = o.requireToolApproval && (slices.Contains(slices.Collect(maps.Keys(o.approvalRequiredTools)), normalizedToolName) || presetRequiresApproval)
	var governanceRequiresApproval = governanceDecision.Action == core.GovernanceActionRequireApproval

	var innerApproval = defaultActionAwareApproval
	if core.ToolActionPolicyResolverInstance.SupportsActionAwareApproval(tool.Name()) && !explicitlyConfiguredApproval && !presetRequiresApproval {
		innerApproval = listedApproval || defaultActionAwareApproval
	}
	var requiresApproval = governanceRequiresApproval || approvalDescriptor.RequiresApproval || innerApproval
	pevDecision, err := o.planExecuteVerify.EvaluateTool(ctx, &core.PlanExecuteVerifyToolContext{
		Session:                  session,
		CorrelationID:            turnCtx.CorrelationId,
		CallID:                   callId,
		ToolName:                 tool.Name(),
		ArgumentsJSON:            persistedArgsJson,
		ActionDescriptor:         approvalDescriptor,
		GovernanceDescriptor:     governanceDescriptor,
		ExistingApprovalRequired: requiresApproval,
		IsStreaming:              isStreaming,
		ToolCallCount:            toolCallCount,
	})
	if err != nil {
		return nil, err
	}
	if BlocksPlanExecuteVerifyDecision(pevDecision.Decision) {
		var blocked = fmt.Sprintf("Plan-Execute-Verify decision '%s' blocked tool execution: %s", pevDecision.Decision, pevDecision.Summary)
		return CreateImmediateResult(
			toolName,
			persistedArgsJson,
			o.redaction.Redact(blocked),
			callId,
			core.ToolResultStatusesBlocked,
			core.ToolFailureCodesApprovalRequired,
			blocked,
			"Review the linked Plan-Execute-Verify run before retrying.",
			governanceDecision), nil
	}
	requiresApproval = requiresApproval || pevDecision.RequiresApproval

	if requiresApproval {
		if approvalCallback != nil {
			var approved = approvalCallback(ctx, tool.Name(), persistedArgsJson)
			o.planExecuteVerify.RecordApprovalDecision(ctx, pevDecision.Run, approved)
			if !approved {
				var deniedResult = CreateImmediateResult(
					toolName,
					persistedArgsJson,
					"Tool execution denied by user.",
					callId,
					core.ToolResultStatusesBlocked,
					core.ToolFailureCodesApprovalRequired,
					"Tool execution was denied by the reviewer.",
					"Approve the tool request to allow this action.",
					governanceDecision)
				return deniedResult, nil
			}
		} else {
			o.logger.Warn(
				"Tool requires approval but no approval channel is available — denied",
				"CorrelationId", turnCtx.CorrelationId,
				"Tool", tool.Name())
			var approvalMessage = fmt.Sprintf("Tool '%s' requires approval but this session has no approval channel — auto-denied. "+
				"To enable this tool: connect through the browser chat at /chat (it supports interactive approvals) "+
				"or set OpenClaw:Tooling:RequireToolApproval=false for trusted local sessions.", tool.Name())
			var deniedResult = CreateImmediateResult(
				toolName,
				persistedArgsJson,
				o.redaction.Redact(approvalMessage),
				callId,
				core.ToolResultStatusesBlocked,
				core.ToolFailureCodesApprovalRequired,
				approvalMessage,
				"Use an approval-capable surface such as /chat, or disable approval requirements for trusted local sessions.",
				governanceDecision)
			o.planExecuteVerify.RecordApprovalDecision(ctx, pevDecision.Run, false)
			return deniedResult, nil
		}
	}

	if requiresApproval && approvalDescriptor.ApprovalFingerprint != "" {
		var currentDescriptor = ResolveToolActionDescriptor(tool, persistedArgsJson)
		if currentDescriptor.ApprovalFingerprint != approvalDescriptor.ApprovalFingerprint {
			var message = fmt.Sprintf("Tool '%s' changed after approval was requested; execution blocked.", tool.Name())
			return CreateImmediateResult(
				toolName,
				persistedArgsJson,
				message,
				callId,
				core.ToolResultStatusesBlocked,
				core.ToolFailureCodesApprovalRequired,
				message,
				"Preview the command again and request approval for the updated fingerprint.",
				governanceDecision), nil
		}
	}

	start := time.Now()

	var (
		result         string
		resultStatus   = core.ToolResultStatusesCompleted
		failureCode    string
		failureMessage string
		nextStep       string
		toolFailed     = false
		toolTimedOut   = false
		persistedArgs  string
		afterHookCtx   = hookCtx
	)

	// 1. Tool Execution Block
	sub, err := o.sentinelSubstitution.Substitute(ctx, &core.SentinelSubstitutionContext{
		ToolName:      tool.Name(),
		ArgumentsJson: argsJson,
		SessionId:     session.Id,
		ChannelId:     session.ChannelId,
		SenderId:      session.SenderId,
		CorrelationId: turnCtx.ChannelId,
	})

	if err != nil {
		toolFailed = true
		fCode := ClassifyToolFailureCode(tool, err.Error())
		failureCode = fCode
		fMsg := err.Error()
		failureMessage = fMsg
		result = "Error: Tool execution failed."
		resultStatus = core.ToolResultStatusesFailed
	} else {
		executionArgsJson := sub.ExecutionArgumentsJson
		persistedArgs = o.redaction.Redact(sub.PersistedArgumentsJson)
		afterHookCtx.ArgumentsJson = persistedArgs

		// Handle cancellation / timeout / errors during execution
		streamingTool, ok := tool.(core.IStreamingTool)
		requestedSkill, requestedInput, ook := TryGetMetaInvokeArguments(executionArgsJson)
		if onDelta != nil && ok {
			result, err = o.ExecuteStreamingToolCollect(ctx, streamingTool, executionArgsJson, onDelta)
		} else if o.metaInvokeExecutor != nil && tool.Name() == "meta_invoke" && ook {
			result, err = o.metaInvokeExecutor(ctx, session, requestedSkill, &requestedInput)
			if strings.Contains(result, "disabled by runtime policy") {
				toolFailed = true
				resultStatus = core.ToolResultStatusesBlocked
				failureCode = core.ToolFailureCodesRuntimeCapabilityUnavailable
				failureMessage = result
				nextStep = "Use a non-meta skill or enable meta invocation in runtime policy."
			}
		} else {
			result, err = o.ExecuteToolWithRouting(ctx, tool, executionArgsJson, session, turnCtx)
		}

		if err != nil {
			// Check if caller/parent context was canceled
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, ctx.Err()
			}

			toolFailed = true
			fMsg := err.Error()
			failureMessage = fMsg

			if errors.Is(err, context.DeadlineExceeded) { // Timeouts
				result = "Error: Tool execution timed out."
				toolTimedOut = true
				resultStatus = core.ToolResultStatusesFailed
				fCode := core.ToolFailureCodesTimeout
				failureCode = fCode
				step := "Retry the tool call or increase Tooling.ToolTimeoutSeconds."
				nextStep = step

				if o.metrics != nil {
					o.metrics.IncrementToolTimeouts()
				}
			} else { // General Exceptions
				fCode := ClassifyToolFailureCode(tool, err.Error())
				failureCode = fCode

				if fCode == core.ToolFailureCodesOperatorAuthRequired ||
					fCode == core.ToolFailureCodesBrowserBackendMissing ||
					fCode == core.ToolFailureCodesRuntimeCapabilityUnavailable {

					if strings.HasPrefix(strings.ToLower(err.Error()), "error:") {
						result = err.Error()
					} else {
						result = "Error: " + err.Error()
					}
					resultStatus = core.ToolResultStatusesBlocked
					step := BuildFailureNextStep(tool.Name(), fCode)
					nextStep = step
				} else {
					result = "Error: Tool execution failed."
					resultStatus = core.ToolResultStatusesFailed
				}

				if o.metrics != nil {
					o.metrics.IncrementToolFailures()
				}
			}
		} else {
			if o.metaInvokeExecutor != nil &&
				tool.Name() == "meta_invoke" &&
				strings.Contains(strings.ToLower(result), "disabled by runtime policy") {

				toolFailed = true
				resultStatus = core.ToolResultStatusesBlocked
				fCode := core.ToolFailureCodesRuntimeCapabilityUnavailable
				failureCode = fCode
				fMsg := result
				failureMessage = fMsg
				step := "Use a non-meta skill or enable meta invocation in runtime policy."
				nextStep = step
			}
		}
	}

	elapsed := time.Since(start)

	// 2. Redaction
	result = o.redaction.Redact(result)
	if failureMessage != "" {
		redactedMsg := o.redaction.Redact(failureMessage)
		failureMessage = redactedMsg
	}
	if nextStep != "" {
		redactedStep := o.redaction.Redact(nextStep)
		nextStep = redactedStep
	}

	// 3. Interceptors Execution
	if len(o.interceptors) > 0 {
		// Sort interceptors by order
		sortedInterceptors := make([]core.IToolResultInterceptor, len(o.interceptors))
		copy(sortedInterceptors, o.interceptors)
		sort.Slice(sortedInterceptors, func(i, j int) bool {
			return sortedInterceptors[i].GetOrder() < sortedInterceptors[j].GetOrder()
		})

		exitCode := 0
		if toolFailed {
			exitCode = 1
		}

		for _, interceptor := range sortedInterceptors {
			interceptedRes, err := interceptor.Intercept(ctx, core.ReductionContext{
				ToolName:      tool.Name(),
				ArgumentsJSON: persistedArgs,
				RawOutput:     result,
				IsError:       toolFailed,
				ExitCode:      exitCode,
			})
			if err == nil {
				result = interceptedRes
			}
		}
	}

	// 4. Metrics & Telemetry Record
	if o.metrics != nil {
		o.metrics.IncrementToolCalls()
	}
	core.ToolExecutionDuration.Record(ctx, float64(elapsed.Milliseconds()), metric.WithAttributes([]attribute.KeyValue{
		attribute.String("tool.name", tool.Name()),
		attribute.Bool("tool.success", !toolFailed),
	}...))

	turnCtx.RecordToolCall(elapsed, toolFailed, toolTimedOut)

	if o.toolUsageTracker != nil {
		o.toolUsageTracker.RecordToolCall(tool.Name(), elapsed, toolFailed, toolTimedOut)
	}

	argsBytes := len([]byte(persistedArgs))
	resultBytes := len([]byte(result))

	if governanceDecision != nil {
		o.RecordGovernanceResult(
			ctx,
			governanceContext,
			*governanceDecision,
			resultStatus,
			failureCode,
			failureMessage,
			toolFailed,
			toolTimedOut,
			elapsed,
			resultBytes,
		)
	}

	// 5. Audit Logging
	if o.auditLog != nil {
		o.auditLog.Record(&core.ToolAuditEntry{
			TimestampUtc:           time.Now().UTC(),
			ToolName:               tool.Name(),
			SessionId:              session.Id,
			ChannelId:              session.ChannelId,
			SenderId:               session.SenderId,
			CorrelationId:          turnCtx.CorrelationId,
			DurationMs:             float64(elapsed.Milliseconds()),
			Failed:                 toolFailed,
			TimedOut:               toolTimedOut,
			ArgumentsBytes:         argsBytes,
			ResultBytes:            resultBytes,
			GovernanceAllowed:      governanceDecision.Allowed,
			GovernanceAction:       governanceDecision.Action.String(),
			GovernanceReason:       governanceDecision.Reason,
			GovernancePolicyId:     governanceDecision.PolicyId,
			GovernanceRuleId:       governanceDecision.RuleId,
			GovernanceTrustScore:   util.Deref(governanceDecision.TrustScore),
			GovernanceEvaluationMs: util.Deref(governanceDecision.EvaluationMs),
			GovernanceUnavailable:  governanceDecision.IsUnavailable,
		})
	}

	// 6. Post Hooks Execution
	for _, hook := range o.hooks {
		var hookErr error
		if ctxHook, ok := hook.(core.IToolHookWithContext); ok {
			hookErr = ctxHook.AfterExecuteContext(ctx, afterHookCtx, result, elapsed, toolFailed)
		} else {
			hookErr = hook.AfterExecute(ctx, tool.Name(), persistedArgs, result, elapsed, toolFailed)
		}

		if hookErr != nil && o.logger != nil {
			o.logger.Warn(fmt.Sprintf("[%s] Hook %s AfterExecute threw: %v", turnCtx.CorrelationId, hook.Name(), hookErr))
		}
	}

	// 7. Complete Invocation & Build Return Payload
	invocation := core.ToolInvocation{
		CallId:                 callId,
		ToolName:               toolName,
		Arguments:              persistedArgs,
		Result:                 result,
		Duration:               elapsed,
		ResultStatus:           resultStatus,
		FailureCode:            failureCode,
		FailureMessage:         failureMessage,
		NextStep:               nextStep,
		GovernanceAllowed:      &governanceDecision.Allowed,
		GovernanceAction:       governanceDecision.Action.String(),
		GovernanceReason:       governanceDecision.Reason,
		GovernancePolicyId:     governanceDecision.PolicyId,
		GovernanceRuleId:       governanceDecision.RuleId,
		GovernanceTrustScore:   governanceDecision.TrustScore,
		GovernanceEvaluationMs: governanceDecision.EvaluationMs,
		GovernanceUnavailable:  &governanceDecision.IsUnavailable,
	}

	_, err = o.planExecuteVerify.CompleteTool(ctx, pevDecision.Run, invocation)
	if err != nil {
		if o.logger != nil {
			o.logger.Warn(fmt.Sprintf("[%s] CompleteTool failed: %v", turnCtx.CorrelationId, err))
		}
	}

	return &ToolExecutionResult{
		Invocation:     invocation,
		ResultText:     result,
		ResultStatus:   resultStatus,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
		NextStep:       nextStep,
	}, nil
}
