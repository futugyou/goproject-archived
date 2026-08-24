package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/futugyou/extensions_ai/abstractions"
	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/extensions_ai/abstractions/contents"
	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/circuitbreaker"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type AgentTurnStopReason string

const (
	AgentTurnCompleted                AgentTurnStopReason = "Completed"
	AgentTurnGoalContinuationRequired AgentTurnStopReason = "GoalContinuationRequired"
	AgentTurnBatchLimitReached        AgentTurnStopReason = "BatchLimitReached"
	AgentTurnBlocked                  AgentTurnStopReason = "CompBlockedleted"
	AgentTurnBudgetLimited            AgentTurnStopReason = "BudgetLimited"
	AgentTurnFailed                   AgentTurnStopReason = "Failed"
)

type AgentTurnResult struct {
	Text           string
	ShouldContinue bool
	StopReason     AgentTurnStopReason
	ContinuePrompt string
}

func CompletedAgentTurnResult(text string) *AgentTurnResult {
	return &AgentTurnResult{
		Text:       text,
		StopReason: AgentTurnCompleted,
	}
}

type AuditLogHook struct {
	logger *slog.Logger
}

func NewAuditLogHook(logger *slog.Logger) *AuditLogHook {
	if logger == nil {
		logger = slog.Default()
	}

	return &AuditLogHook{logger: logger}
}

func (a *AuditLogHook) Name() string {
	return "AuditLog"
}

func (a *AuditLogHook) BeforeExecute(ctx context.Context, toolName string, arguments string) bool {
	return true
}

func (a *AuditLogHook) BeforeExecuteContext(ctx context.Context, context core.ToolHookContext) bool {
	a.logger.Info("[Audit] Tool invoked for session",
		"ToolName", context.ToolName,
		"SessionId", context.SessionId,
		"ChannelId", context.ChannelId,
		"SenderId", context.SenderId,
		"Length", len(context.ArgumentsJson),
	)
	return true
}

func (a *AuditLogHook) AfterExecute(ctx context.Context, toolName string, arguments string, result string, duration time.Duration, failed bool) error {
	return nil
}

func (a *AuditLogHook) AfterExecuteContext(ctx context.Context, context core.ToolHookContext, result string, duration time.Duration, failed bool) error {
	if failed {
		a.logger.Warn("[Audit] Tool FAILED for session ",
			"ToolName", context.ToolName,
			"SessionId", context.SessionId,
			"ChannelId", context.ChannelId,
			"SenderId", context.SenderId,
			"TotalMilliseconds", duration.Milliseconds(),
			"Length", len(result),
		)
	} else {
		a.logger.Info("[Audit] Tool completed for session ",
			"ToolName", context.ToolName,
			"SessionId", context.SessionId,
			"ChannelId", context.ChannelId,
			"SenderId", context.SenderId,
			"TotalMilliseconds", duration.Milliseconds(),
			"Length", len(result),
		)
	}

	return nil
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		if len(path) == 1 {
			return home
		}

		return filepath.Join(home, path[2:])
	}

	return path
}

func resolveWorkspaceRoot(cfg core.ToolingConfig) string {
	if !cfg.WorkspaceOnly {
		return ""
	}

	var resolved = core.SecretResolverInstance.Resolve(cfg.WorkspaceRoot)
	if resolved == "" {
		return ""
	}

	var expanded = expandTilde(resolved)
	full, err := filepath.Abs(expanded)
	if err != nil {
		return ""
	}
	if !util.DirectoryExists(full) {
		return ""
	}

	return pathpolicy.ResolveRealPath(full)
}

func tryExtractPathArgument(toolName, arguments string) (string, bool) {
	prop := "path"
	switch toolName {
	case "git":
		prop = "cwd"
	case "process":
		prop = "working_directory"
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(arguments), &data); err != nil {
		return "", false
	}

	if value, ok := data[prop].(string); ok {
		value = strings.TrimSpace(value)
		return value, value != ""
	}

	return "", false
}

func isUnderWorkspace(path, workspaceRoot string) bool {
	var expanded = expandTilde(path)
	var full = pathpolicy.ResolveRealPath(expanded)

	if strings.EqualFold(full, workspaceRoot) {
		return true
	}

	root := workspaceRoot
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}

	return strings.HasPrefix(full, root)
}

func tryExtractShellCommand(arguments string) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal([]byte(arguments), &data); err != nil {
		return "", false
	}

	if value, ok := data["command"].(string); ok {
		value = strings.TrimSpace(value)
		return value, value != ""
	}

	return "", false
}

type AutonomyHook struct {
	config        core.ToolingConfig
	logger        *slog.Logger
	workspaceRoot string
}

func NewAutonomyHook(config core.ToolingConfig, logger *slog.Logger) *AutonomyHook {
	if logger != nil {
		logger = slog.Default()
	}

	return &AutonomyHook{
		config:        config,
		logger:        logger,
		workspaceRoot: resolveWorkspaceRoot(config),
	}
}

func (a *AutonomyHook) Name() string {
	return "Autonomy"
}

func (a *AutonomyHook) isForbiddenPath(path string) bool {
	var expanded = expandTilde(path)
	var full = pathpolicy.ResolveRealPath(expanded)
	for _, pat := range a.config.ForbiddenPathGlobs {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			continue
		}

		var p = expandTilde(pat)
		if core.GlobMatcherInstance.IsMatch(p, expanded) || core.GlobMatcherInstance.IsMatch(p, full) {
			return true
		}
	}

	return false
}

func (a *AutonomyHook) isForbiddenCommand(command string) bool {
	for _, pat := range a.config.ForbiddenPathGlobs {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			continue
		}

		envelope := pat
		if !strings.HasPrefix(envelope, "*") {
			envelope = "*" + envelope
		}
		if !strings.HasSuffix(envelope, "*") {
			envelope = envelope + "*"
		}

		if core.GlobMatcherInstance.IsMatch(envelope, command) {
			return true
		}
	}

	return false
}

func (a *AutonomyHook) isShellCommandAllowed(command string) bool {
	var allow = a.config.AllowedShellCommandGlobs
	if len(allow) == 0 {
		return false
	}

	// Special-case: ["*"] means allow all
	if len(allow) == 1 && allow[0] == "*" {
		return true
	}

	for _, pat := range allow {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			continue
		}

		if core.GlobMatcherInstance.IsMatch(pat, command) {
			return true
		}
	}

	return false
}

func (a *AutonomyHook) BeforeExecute(ctx context.Context, toolName string, arguments string) bool {
	mode := "full"
	if strings.TrimSpace(a.config.AutonomyMode) != "" {
		mode = strings.ToLower(strings.TrimSpace(a.config.AutonomyMode))
	}

	if mode == "readonly" {
		if core.ToolActionPolicyResolverInstance.IsMutationCapable(toolName, arguments) {
			a.logger.Info("autonomy readonly: denied tool", "ToolName", toolName)
			return false
		}
	}

	if a.config.WorkspaceOnly && a.config.WorkspaceRoot != "" {
		path, ok := tryExtractPathArgument(toolName, arguments)
		if ok && !isUnderWorkspace(path, a.workspaceRoot) {
			a.logger.Info("workspace only: denied tool", "ToolName", toolName, "Path", path)
			return false
		}
	}
	if len(a.config.ForbiddenPathGlobs) > 0 {
		path, ok := tryExtractPathArgument(toolName, arguments)
		if ok && a.isForbiddenPath(path) {
			a.logger.Info("ForbiddenPathGlobs: denied tool", "ToolName", toolName, "Path", path)
			return false
		}

		cmd, ok := tryExtractShellCommand(arguments)
		if toolName == "shell" && ok && a.isForbiddenCommand(cmd) {
			a.logger.Info("ForbiddenPathGlobs: denied shell command")
			return false
		}
	}

	if toolName == "shell" {
		if !a.config.AllowShell {
			return false
		}

		cmd, ok := tryExtractShellCommand(arguments)
		if ok && !a.isShellCommandAllowed(cmd) {
			a.logger.Info("AllowedShellCommandGlobs: denied shell command")
			return false
		}
	}

	return true
}

func (a *AutonomyHook) AfterExecute(ctx context.Context, toolName string, arguments string, result string, duration time.Duration, failed bool) error {
	return nil
}

type ContractResolver func(string) *core.ContractPolicy
type ToolCallCounter func(string) int
type ContractScopeHook struct {
	contractResolver ContractResolver
	toolCallCounter  ToolCallCounter
	logger           *slog.Logger
}

func NewContractScopeHook(
	contractResolver ContractResolver,
	toolCallCounter ToolCallCounter,
	logger *slog.Logger,
) *ContractScopeHook {

	if logger == nil {
		logger = slog.Default()
	}
	return &ContractScopeHook{
		logger:           logger,
		contractResolver: contractResolver,
		toolCallCounter:  toolCallCounter,
	}
}

func (c *ContractScopeHook) Name() string {
	return "ContractScope"
}

func (c *ContractScopeHook) BeforeExecute(ctx context.Context, toolName string, arguments string) bool {
	return true
}

func (c *ContractScopeHook) AfterExecute(ctx context.Context, toolName string, arguments string, result string, duration time.Duration, failed bool) error {
	return nil
}

func (c *ContractScopeHook) AfterExecuteContext(ctx context.Context, context core.ToolHookContext, result string, duration time.Duration, failed bool) error {
	return nil
}

func findToolScope(policy *core.ContractPolicy, toolName string) *core.ScopedCapability {
	for _, scope := range policy.ScopedCapabilities {
		if strings.EqualFold(scope.ToolName, toolName) {
			return scope
		}
	}

	return nil
}

func hasScopedFilesystemCapability(policy *core.ContractPolicy) bool {
	for _, scope := range policy.ScopedCapabilities {
		if len(scope.AllowedPaths) > 0 {
			return true
		}
	}
	return false
}

func isFilesystemAffectingTool(toolName string) bool {
	switch toolName {
	case "shell", "code_exec", "git", "process", "file_read", "file_write", "edit_file", "apply_patch":
		return true
	default:
		return false
	}
}

func tryReadStringList(root map[string]any, propertyName string) ([]string, bool) {
	str := util.GetString(root, propertyName)
	if str == nil || strings.TrimSpace(*str) == "" {
		return []string{}, false
	}

	return []string{strings.TrimSpace(*str)}, true
}

func tryReadProcessPaths(root map[string]any) ([]string, bool) {
	var action = util.GetString(root, "action")

	if action == nil || strings.TrimSpace(*action) != "start" {
		return []string{}, false
	}

	return tryReadStringList(root, "working_directory")
}

func tryExtractScopedPaths(toolName, arguments string) ([]string, bool) {
	var root map[string]any
	if err := json.Unmarshal([]byte(arguments), &root); err != nil {
		return []string{}, false
	}

	switch toolName {
	case "git":
		return tryReadStringList(root, "cwd")
	case "process":
		return tryReadProcessPaths(root)
	case "file_read", "file_write", "edit_file", "apply_patch":
		return tryReadStringList(root, "path")
	case "shell":
		return []string{}, false
	default:
		return tryReadStringList(root, "path")
	}
}

func isPathAllowed(path string, allowedPaths []string) bool {
	var expanded = expandTilde(path)
	var full = pathpolicy.ResolveRealPath(expanded)
	for _, allowed := range allowedPaths {
		allowed = strings.TrimSpace(allowed)
		var allowedExpanded = expandTilde(allowed)
		var allowedFull = pathpolicy.ResolveRealPath(allowedExpanded)

		if strings.EqualFold(full, allowedFull) {
			return true
		}

		root := allowedFull
		if !strings.HasSuffix(root, "/") {
			root += "/"
		}

		if strings.HasPrefix(full, root) {
			return true
		}
	}

	return false
}

func (c *ContractScopeHook) BeforeExecuteContext(ctx context.Context, context core.ToolHookContext) bool {
	if c.contractResolver == nil || c.toolCallCounter == nil {
		return true
	}
	var policy = c.contractResolver(context.SessionId)
	if policy == nil {
		return true
	}

	if policy.MaxToolCalls > 0 {
		var count = c.toolCallCounter(context.SessionId)
		if count >= policy.MaxToolCalls {
			c.logger.Info("ContractScope: denied tool", "Tool", context.ToolName, "SessionId", context.SessionId, "MaxToolCalls", policy.MaxToolCalls)
			return false
		}
	}

	// Check scoped capabilities (path restrictions)
	var scope = findToolScope(policy, context.ToolName)
	var hasScopedFilesystemCapability = hasScopedFilesystemCapability(policy)
	if scope == nil && hasScopedFilesystemCapability && isFilesystemAffectingTool(context.ToolName) {
		// Shell is always denied under scoped contracts unless explicitly granted
		// with an unscoped shell capability (AllowedPaths empty). Other filesystem
		// tools are denied because they lack a matching scope entry.
		if context.ToolName == "shell" || context.ToolName == "code_exec" {
			c.logger.Info(
				"ContractScope: denied tool — shell/exec tools require an explicit grant under scoped filesystem contracts",
				"ToolName", context.ToolName,
				"SessionId", context.SessionId)
		} else {
			c.logger.Info(
				"ContractScope: denied tool — tool is unscoped under a scoped filesystem contract",
				"ToolName", context.ToolName,
				"SessionId", context.SessionId)
		}
		return false
	}

	if scope != nil && len(scope.AllowedPaths) > 0 {
		paths, ok := tryExtractScopedPaths(context.ToolName, context.ArgumentsJson)
		if ok {
			for _, path := range paths {
				path = strings.TrimSpace(path)
				if path == "" {
					continue
				}

				if !isPathAllowed(path, scope.AllowedPaths) {
					c.logger.Info(
						"ContractScope: denied tool — outside scoped paths",
						"ToolName", context.ToolName,
						"SessionId", context.SessionId)
					return false
				}
			}
		} else if context.ToolName == "git" || context.ToolName == "process" || context.ToolName == "shell" {
			c.logger.Info(
				"ContractScope: denied tool — scoped path could not be resolved safely",
				"ToolName", context.ToolName,
				"SessionId", context.SessionId)
			return false
		}
	}

	return true
}

type ToolApprovalCallback func(ctx context.Context, toolName, arguments string) bool

type IAgentRuntime interface {
	CircuitBreakerState() circuitbreaker.CircuitState
	LoadedSkillNames() []string
	LoadedSkills() []core.SkillDefinition
	LoadedTools() []abstractions.AITool
	Run(ctx context.Context, session core.Session, userMessage string, approvalCallback ToolApprovalCallback, responseSchema any, correlationId string) string
	RunTurn(ctx context.Context, session core.Session, userMessage string, approvalCallback ToolApprovalCallback, responseSchema any, correlationId string) (*AgentTurnResult, error)
	ReloadSkills(ctx context.Context) []string
	RunStreaming(ctx context.Context, session core.Session, userMessage string, approvalCallback ToolApprovalCallback, correlationId string) chan core.AgentStreamEvent
	ApplyMcpToolChanges(ctx context.Context, toAdd []core.ITool, toRemove []string) error
}

type LlmExecutionResult struct {
	ProfileId            string
	ProviderId           string
	ModelId              string
	PolicyRuleId         string
	SelectionExplanation string
	Response             *chatcompletion.ChatResponse
}

type LlmExecutionEstimate struct {
	EstimatedInputTokens            int64
	EstimatedInputTokensByComponent core.InputTokenComponentEstimate
}

type LlmStreamingExecutionResult struct {
	ProfileId            string
	ProviderId           string
	ModelId              string
	PolicyRuleId         string
	SelectionExplanation string
	Updates              chan chatcompletion.ChatResponseUpdate
}

type ILlmExecutionService interface {
	DefaultCircuitState() circuitbreaker.CircuitState
	GetResponse(ctx context.Context, session core.Session, messages []chatcompletion.ChatMessage, options chatcompletion.ChatOptions, turnContext *core.TurnContext, estimate LlmExecutionEstimate) (*LlmExecutionResult, error)
	StartStreaming(ctx context.Context, session core.Session, messages []chatcompletion.ChatMessage, options chatcompletion.ChatOptions, turnContext *core.TurnContext, estimate LlmExecutionEstimate) (*LlmStreamingExecutionResult, error)
}

type AgentRuntimeFactoryContext struct {
	Config                core.GatewayConfig
	ChatClient            chatcompletion.IChatClient
	Tools                 []abstractions.AITool
	MemoryStore           core.IMemoryStore
	RuntimeMetrics        *core.RuntimeMetrics
	ProviderUsage         core.ProviderUsageTracker
	LlmExecutionService   ILlmExecutionService
	Skills                []core.SkillDefinition
	SkillsConfig          core.SkillsConfig
	WorkspacePath         *string
	PluginSkillDirs       []string
	Logger                *slog.Logger
	Hooks                 []core.IToolHook
	RequireToolApproval   bool
	ApprovalRequiredTools []string

	TurnTokenUsageObserver core.ITurnTokenUsageObserver
	ToolSandbox            core.IToolSandbox
	ToolGovernance         core.IToolGovernanceService
	PlanExecuteVerify      core.IPlanExecuteVerifyOrchestrator
	ToolUsageTracker       *core.ToolUsageTracker
	ToolAuditLog           *core.ToolAuditLog
	Interceptors           []core.IToolResultInterceptor

	IsContractTokenBudgetExceeded   func(session *core.Session) bool
	IsContractRuntimeBudgetExceeded func(session *core.Session) bool

	RecordContractTurnUsage func(session *core.Session, provider, model string, promptTokens, completionTokens int64)

	AppendContractSnapshot func(session *core.Session, snapshot string)
}

type IAgentRuntimeFactory interface {
	OrchestratorId() string
	Create(facContext *AgentRuntimeFactoryContext) IAgentRuntime
}

func AgentRuntimeFactorySelect(factories []IAgentRuntimeFactory, orchestratorId string) (IAgentRuntimeFactory, error) {
	normalizedOrchestrator := core.RuntimeOrchestratorNormalize(orchestratorId)
	for _, candidate := range factories {
		if strings.EqualFold(candidate.OrchestratorId(), normalizedOrchestrator) {
			return candidate, nil
		}
	}

	msg := "no agent runtime factory is registered for orchestrator " + normalizedOrchestrator

	return nil, errors.New(msg)
}

func LlmExecutionEstimateInputTokens(messages []chatcompletion.ChatMessage, additionalSystemPromptChars int) int {
	var charCount = max(0, additionalSystemPromptChars)
	for _, message := range messages {
		value := contents.ConcatTextContents(message.Contents)
		value = strings.TrimSpace(value)
		if value != "" {
			charCount += len(value)
		}
	}
	return LlmExecutionEstimateTokenCount(charCount)
}

func LlmExecutionEstimateTokenCount(charCount int) int {
	if charCount <= 0 {
		return 0
	}

	return max(1, (charCount+3)/4)
}

func LlmExecutionEstimateCreate(
	messages []chatcompletion.ChatMessage,
	skillPromptLength int,
	additionalSystemPromptChars int) LlmExecutionEstimate {
	var estimatedInputTokens = LlmExecutionEstimateInputTokens(messages, additionalSystemPromptChars)
	return LlmExecutionEstimate{
		EstimatedInputTokens: int64(estimatedInputTokens),
		EstimatedInputTokensByComponent: BuildInputTokenEstimate(
			messages,
			estimatedInputTokens,
			skillPromptLength,
			additionalSystemPromptChars),
	}
}

func BuildInputTokenEstimate(messages []chatcompletion.ChatMessage, totalInputTokens, skillPromptLength, additionalSystemPromptChars int) core.InputTokenComponentEstimate {
	var systemChars int64 = int64(max(0, additionalSystemPromptChars))
	var historyChars int64 = 0
	var toolChars int64 = 0
	var userChars int64 = 0

	for i := 0; i < len(messages); i++ {
		var message = messages[i]
		var chars int64 = 0
		value := contents.ConcatTextContents(message.Contents)
		value = strings.TrimSpace(value)
		if value != "" {
			chars += int64(len(value))
		}

		if i == 0 && message.Role == chatcompletion.RoleSystem {
			systemChars += chars
			continue
		}

		if message.Role == chatcompletion.RoleTool {
			toolChars += chars
			continue
		}

		if message.Role == chatcompletion.RoleUser && i == len(messages)-1 {
			userChars += chars
			continue
		}

		historyChars += chars
	}

	var skillChars = min(systemChars, int64(skillPromptLength))
	var systemPromptChars = max(0, systemChars-skillChars)

	return distributeEstimatedTokens(
		int64(totalInputTokens),
		systemPromptChars,
		skillChars,
		historyChars,
		toolChars,
		userChars)
}

func distributeEstimatedTokens(
	totalTokens,
	systemPromptChars,
	skillChars,
	historyChars,
	toolChars,
	userChars int64) core.InputTokenComponentEstimate {
	var totalChars = systemPromptChars + skillChars + historyChars + toolChars + userChars
	if totalTokens <= 0 || totalChars <= 0 {
		return core.InputTokenComponentEstimate{}
	}

	var systemTokens = math.Round((float64)(totalTokens * systemPromptChars / totalChars))
	var skillTokens = math.Round((float64)(totalTokens * skillChars / totalChars))
	var historyTokens = math.Round((float64)(totalTokens * historyChars / totalChars))
	var toolTokens = math.Round((float64)(totalTokens * toolChars / totalChars))
	var userTokens = max(0, (float64)(totalTokens)-systemTokens-skillTokens-historyTokens-toolTokens)

	return core.InputTokenComponentEstimate{
		SystemPrompt: int64(systemTokens),
		Skills:       int64(skillTokens),
		History:      int64(historyTokens),
		ToolOutputs:  int64(toolTokens),
		UserInput:    int64(userTokens),
	}
}
