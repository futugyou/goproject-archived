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
	"go.opentelemetry.io/otel/attribute"
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
	decision core.GovernanceDecision) {
	if o.auditLog == nil {
		return
	}
	o.auditLog.Record(&core.ToolAuditEntry{
		TimestampUtc:           time.Now().UTC(),
		ToolName:               tool.Name(),
		SessionId:              session.Id,
		ChannelId:              session.ChannelId,
		SenderId:               session.SenderId,
		CorrelationId:          turnCtx.CorrelationId,
		Failed:                 true,
		ArgumentsBytes:         len(argumentsJson),
		ResultBytes:            len(result),
		GovernanceAllowed:      decision.Allowed,
		GovernanceAction:       decision.Action.String(),
		GovernanceReason:       decision.Reason,
		GovernancePolicyId:     decision.PolicyId,
		GovernanceRuleId:       decision.RuleId,
		GovernanceTrustScore:   util.Deref(decision.TrustScore),
		GovernanceEvaluationMs: util.Deref(decision.EvaluationMs),
		GovernanceUnavailable:  decision.IsUnavailable,
	})
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

func ApplyGovernanceActivityTags(span trace.Span, decision core.GovernanceDecision) {

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
