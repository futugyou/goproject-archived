package agent

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/extensions_ai/abstractions/contents"
	"github.com/futugyou/openclaw/circuitbreaker"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type AgentRuntime struct {
	chatClient                      chatcompletion.IChatClient
	tools                           []core.ITool
	toolExecutor                    *OpenClawToolExecutor
	memory                          core.IMemoryStore
	logger                          *slog.Logger
	systemPrompt                    string
	maxTokens                       int
	maxIterations                   int
	temperature                     float32
	maxHistoryTurns                 int
	llmTimeoutSeconds               int
	retryCount                      int
	toolTimeoutSeconds              int
	parallelToolExecution           bool
	enableCompaction                bool
	compactionThreshold             int
	compactionKeepRecent            int
	requireToolApproval             bool
	approvalRequiredTools           map[string]struct{}
	hooks                           []core.IToolHook
	circuitBreaker                  *circuitbreaker.CircuitBreaker
	metrics                         *core.RuntimeMetrics
	providerUsage                   *core.ProviderUsageTracker
	turnTokenUsageObserver          core.ITurnTokenUsageObserver
	llmExecutionService             ILlmExecutionService
	goalService                     core.IGoalService
	goalIntegration                 *AgentRuntimeGoalIntegration
	sessionTokenBudget              int64
	estimateTokenBudgetAdmission    bool
	config                          core.LlmProviderConfig
	recall                          *core.MemoryRecallConfig
	profileStore                    core.IUserProfileStore
	profilesConfig                  *core.ProfilesConfig
	isContractTokenBudgetExceeded   func(session core.Session) bool
	isContractRuntimeBudgetExceeded func(session core.Session) bool
	recordContractTurnUsage         func(session core.Session, arg1, arg2 string, arg3, arg4 int64)
	appendContractSnapshot          func(session core.Session, arg string)
	skillsConfig                    *core.SkillsConfig
	metaSkillsEnabled               bool
	skillWorkspacePath              *string
	pluginSkillDirs                 []string
	redaction                       core.IRedactionPipeline
	sentinelSubstitution            core.ISentinelSubstitutionService
	memoryRecallPrefix              *string
	contextBudgetPlanner            *core.ContextBudgetPlanner
	fractalMemory                   *core.FractalMemoryConfig
	backgroundExecutionEnabled      bool
	turnRoutingPolicy               ITurnRoutingPolicy

	skillGate         sync.RWMutex
	loadedSkillNames  []string
	loadedSkills      []core.SkillDefinition
	skillPromptLength int
}

type AgentRuntimeOptions struct {
	Skills                          []core.SkillDefinition
	SkillsConfig                    *core.SkillsConfig
	SkillWorkspacePath              *string
	PluginSkillDirs                 []string
	Logger                          *slog.Logger
	ToolTimeoutSeconds              int
	Metrics                         *core.RuntimeMetrics
	ProviderUsage                   *core.ProviderUsageTracker
	TurnTokenUsageObserver          core.ITurnTokenUsageObserver
	LlmExecutionService             ILlmExecutionService
	ParallelToolExecution           bool
	EnableCompaction                bool
	CompactionThreshold             int
	CompactionKeepRecent            int
	RequireToolApproval             bool
	ApprovalRequiredTools           []string
	MaxIterations                   int
	Hooks                           []core.IToolHook
	SessionTokenBudget              int64
	Recall                          *core.MemoryRecallConfig
	ProfileStore                    core.IUserProfileStore
	ProfilesConfig                  *core.ProfilesConfig
	ToolSandbox                     core.IToolSandbox
	GatewayConfig                   *core.GatewayConfig
	ToolUsageTracker                *core.ToolUsageTracker
	ExecutionRouter                 *ToolExecutionRouter
	ToolPresetResolver              core.IToolPresetResolver
	IsContractTokenBudgetExceeded   func(session core.Session) bool
	IsContractRuntimeBudgetExceeded func(session core.Session) bool
	RecordContractTurnUsage         func(session core.Session, arg1, arg2 string, arg3, arg4 int64)
	AppendContractSnapshot          func(session core.Session, arg string)
	ToolAuditLog                    *core.ToolAuditLog
	Redaction                       core.IRedactionPipeline
	SentinelSubstitution            core.ISentinelSubstitutionService
	ToolGovernance                  core.IToolGovernanceService
	PlanExecuteVerify               core.IPlanExecuteVerifyOrchestrator
	ContextBudgetPlanner            *core.ContextBudgetPlanner
	TurnRoutingPolicy               ITurnRoutingPolicy
	GoalService                     core.IGoalService
	Interceptors                    []core.IToolResultInterceptor
}

func NewAgentRuntime(
	chatClient chatcompletion.IChatClient,
	tools []core.ITool,
	memory core.IMemoryStore,
	config core.LlmProviderConfig,
	maxHistoryTurns int,
	opts *AgentRuntimeOptions,
) *AgentRuntime {
	if opts == nil {
		opts = &AgentRuntimeOptions{}
	}

	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	// Set default values for optional parameters
	toolTimeoutSeconds := 30
	if opts.ToolTimeoutSeconds != 0 {
		toolTimeoutSeconds = opts.ToolTimeoutSeconds
	}

	parallelToolExecution := true
	if opts.ParallelToolExecution != false || opts.ToolTimeoutSeconds != 0 {
		parallelToolExecution = opts.ParallelToolExecution
	}

	compactionThreshold := 40
	if opts.CompactionThreshold != 0 {
		compactionThreshold = opts.CompactionThreshold
	}

	compactionKeepRecent := 10
	if opts.CompactionKeepRecent != 0 {
		compactionKeepRecent = opts.CompactionKeepRecent
	}

	maxIterations := 10
	if opts.MaxIterations != 0 {
		maxIterations = opts.MaxIterations
	}

	var hooks []core.IToolHook
	if opts.Hooks != nil {
		hooks = opts.Hooks
	}

	var pluginSkillDirs []string
	if opts.PluginSkillDirs != nil {
		pluginSkillDirs = opts.PluginSkillDirs
	}

	redaction := opts.Redaction
	if redaction == nil {
		redaction = &core.NoopRedactionPipeline{}
	}

	sentinelSubstitution := opts.SentinelSubstitution
	if sentinelSubstitution == nil {
		sentinelSubstitution = &core.NoopSentinelSubstitutionService{}
	}

	metaSkillsEnabled := true
	if opts.SkillsConfig != nil {
		metaSkillsEnabled = opts.SkillsConfig.MetaSkill.Enabled
	}

	approvalSet := normalizeApprovalRequiredTools(opts.ApprovalRequiredTools)

	var goalIntegration *AgentRuntimeGoalIntegration
	if opts.GoalService != nil {
		goalIntegration = NewAgentRuntimeGoalIntegration(opts.GoalService, opts.Logger)
	}

	circuitBreaker := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(config.CircuitBreakerThreshold),
		circuitbreaker.WithCooldown(time.Duration(config.CircuitBreakerCooldownSeconds)*time.Second),
		circuitbreaker.WithLogger(opts.Logger),
	)

	estimateTokenBudgetAdmission := false
	var fractalMemory *core.FractalMemoryConfig
	backgroundExecutionEnabled := false
	var memoryProjectId string

	if opts.GatewayConfig == nil {
		opts.GatewayConfig = core.DefaultGatewayConfig()
	}
	estimateTokenBudgetAdmission = opts.GatewayConfig.EnableEstimatedTokenAdmissionControl
	fractalMemory = opts.GatewayConfig.Memory.Fractal
	backgroundExecutionEnabled = opts.GatewayConfig.BackgroundExecution.Enabled
	memoryProjectId = opts.GatewayConfig.Memory.ProjectId

	if strings.TrimSpace(memoryProjectId) == "" {
		memoryProjectId = os.Getenv("OPENCLAW_PROJECT")
	}

	var memoryRecallPrefix *string
	trimmedProject := strings.TrimSpace(memoryProjectId)
	if trimmedProject != "" {
		prefix := "project:" + trimmedProject + ":"
		memoryRecallPrefix = &prefix
	}

	turnRoutingPolicy := opts.TurnRoutingPolicy
	if turnRoutingPolicy == nil {
		turnRoutingPolicy = &NoopTurnRoutingPolicy{}
	}

	r := &AgentRuntime{
		chatClient:                      chatClient,
		tools:                           tools,
		memory:                          memory,
		logger:                          opts.Logger,
		config:                          config,
		maxTokens:                       config.MaxTokens,
		maxIterations:                   int(math.Max(1, float64(maxIterations))),
		temperature:                     config.Temperature,
		maxHistoryTurns:                 int(math.Max(1, float64(maxHistoryTurns))),
		llmTimeoutSeconds:               config.TimeoutSeconds,
		retryCount:                      config.RetryCount,
		toolTimeoutSeconds:              toolTimeoutSeconds,
		parallelToolExecution:           parallelToolExecution,
		enableCompaction:                opts.EnableCompaction,
		compactionThreshold:             int(math.Max(4, float64(compactionThreshold))),
		compactionKeepRecent:            int(math.Max(2, float64(compactionKeepRecent))),
		requireToolApproval:             opts.RequireToolApproval,
		approvalRequiredTools:           approvalSet,
		hooks:                           hooks,
		metrics:                         opts.Metrics,
		providerUsage:                   opts.ProviderUsage,
		turnTokenUsageObserver:          opts.TurnTokenUsageObserver,
		llmExecutionService:             opts.LlmExecutionService,
		goalService:                     opts.GoalService,
		goalIntegration:                 goalIntegration,
		skillsConfig:                    opts.SkillsConfig,
		metaSkillsEnabled:               metaSkillsEnabled,
		skillWorkspacePath:              opts.SkillWorkspacePath,
		pluginSkillDirs:                 pluginSkillDirs,
		redaction:                       redaction,
		sentinelSubstitution:            sentinelSubstitution,
		circuitBreaker:                  circuitBreaker,
		sessionTokenBudget:              opts.SessionTokenBudget,
		estimateTokenBudgetAdmission:    estimateTokenBudgetAdmission,
		recall:                          opts.Recall,
		profileStore:                    opts.ProfileStore,
		profilesConfig:                  opts.ProfilesConfig,
		contextBudgetPlanner:            opts.ContextBudgetPlanner,
		fractalMemory:                   fractalMemory,
		backgroundExecutionEnabled:      backgroundExecutionEnabled,
		turnRoutingPolicy:               turnRoutingPolicy,
		isContractTokenBudgetExceeded:   opts.IsContractTokenBudgetExceeded,
		isContractRuntimeBudgetExceeded: opts.IsContractRuntimeBudgetExceeded,
		recordContractTurnUsage:         opts.RecordContractTurnUsage,
		appendContractSnapshot:          opts.AppendContractSnapshot,
		memoryRecallPrefix:              memoryRecallPrefix,
		loadedSkillNames:                []string{},
		loadedSkills:                    []core.SkillDefinition{},
	}

	// Closure for openClawToolExecutor meta invoke handler
	metaExecutor := func(ctx context.Context, session core.Session, skillName string, input *string) (string, error) {
		return r.ExecuteMetaSkill(ctx, session, skillName, input)
	}

	r.toolExecutor = NewOpenClawToolExecutor(
		tools,
		toolTimeoutSeconds,
		opts.RequireToolApproval,
		mapKeysToSlice(approvalSet),
		hooks,
		opts.Metrics,
		opts.Logger,
		opts.GatewayConfig,
		opts.ToolSandbox,
		opts.ToolUsageTracker,
		opts.ExecutionRouter,
		opts.ToolPresetResolver,
		opts.ToolAuditLog,
		redaction,
		sentinelSubstitution,
		opts.ToolGovernance,
		opts.PlanExecuteVerify,
		metaExecutor,
		opts.Interceptors,
	)

	initialSkills := opts.Skills
	if initialSkills == nil {
		initialSkills = []core.SkillDefinition{}
	}
	r.ApplySkills(initialSkills)

	return r
}

func (a *AgentRuntime) LoadedSkillNames() []string {
	a.skillGate.RLock()
	defer a.skillGate.RUnlock()

	result := make([]string, len(a.loadedSkillNames))
	copy(result, a.loadedSkillNames)
	return result
}

func (a *AgentRuntime) LoadedSkills() []core.SkillDefinition {
	a.skillGate.RLock()
	defer a.skillGate.RUnlock()

	result := make([]core.SkillDefinition, len(a.loadedSkills))
	copy(result, a.loadedSkills)
	return result
}
func normalizeApprovalRequiredTools(tools []string) map[string]struct{} {
	res := make(map[string]struct{})
	for _, t := range tools {
		res[t] = struct{}{}
	}
	return res
}

func mapKeysToSlice(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (a *AgentRuntime) ExecuteMetaSkill(ctx context.Context, session core.Session, skillName string, input *string) (string, error) {
	// Implementation placeholder
	return "", nil
}

func (a *AgentRuntime) ApplySkills(skills []core.SkillDefinition) {
	a.skillGate.Lock()
	defer a.skillGate.Unlock()

	promptVisibleSkills := skills

	if !a.metaSkillsEnabled {
		promptVisibleSkills = []core.SkillDefinition{}
		for _, skill := range skills {
			if skill.Kind != core.SkillKind_Meta {
				promptVisibleSkills = append(promptVisibleSkills, skill)
			}
		}
	}

	// Progressive disclosure: only the metadata index lives in the system prompt.
	// The full SKILL.md body for any single skill is fetched on demand via the
	// `load_skill` tool, which reads from LoadedSkills (this same snapshot).
	instructionPrompt := ""
	if a.skillsConfig != nil {
		instructionPrompt = a.skillsConfig.InstructionPrompt
	}
	var skillSection = core.SkillPromptBuilderInstance.BuildIndex(promptVisibleSkills, instructionPrompt)
	var basePrompt = BuildBaseSystemPrompt(a.requireToolApproval)
	a.skillPromptLength = len(skillSection)
	if len(skillSection) == 0 {
		a.systemPrompt = basePrompt
	} else {
		a.systemPrompt = basePrompt + "\n" + skillSection
	}

	a.loadedSkills = skills
	names := []string{}
	for _, skill := range skills {
		names = append(names, skill.Name)
	}

	slices.Sort(names)
	a.loadedSkillNames = names
}

func (a *AgentRuntime) AppendContractSnapshot(session core.Session, status string) {
	if session.ContractPolicy == nil {
		return
	}

	if a.appendContractSnapshot != nil {
		a.appendContractSnapshot(session, status)
	}
}

func (a *AgentRuntime) TryRejectContractBudget(session core.Session) (message string, ok bool) {
	if session.ContractPolicy == nil {
		return
	}

	if a.isContractRuntimeBudgetExceeded != nil && a.isContractRuntimeBudgetExceeded(session) {
		message = "This contract has expired and can no longer execute new work."
		ok = true
		return
	}

	if a.isContractTokenBudgetExceeded != nil && a.isContractTokenBudgetExceeded(session) {
		message = "This contract has reached its token budget and cannot continue."
		ok = true
		return
	}

	return
}

func (a *AgentRuntime) CircuitBreakerState() circuitbreaker.CircuitState {
	var state circuitbreaker.CircuitState = 0
	if a.llmExecutionService != nil {
		state = a.llmExecutionService.DefaultCircuitState()
	}
	if state == 0 {
		return a.circuitBreaker.State()
	}

	return state
}

func (a *AgentRuntime) LogTurnComplete(turnCtx *core.TurnContext) {
	if a.metrics != nil {
		a.metrics.SetCircuitBreakerState(int32(a.CircuitBreakerState()))
	}

	a.logger.Info("Turn complete", "CorrelationId", turnCtx.CorrelationId, "Summary", turnCtx.String())
}

func (a *AgentRuntime) TryRejectEstimatedBudget(session core.Session, estimate LlmExecutionEstimate) (message string, ok bool) {
	if !a.estimateTokenBudgetAdmission || a.sessionTokenBudget <= 0 {
		return
	}

	var remaining = a.sessionTokenBudget - session.GetTotalTokens()
	if remaining <= 0 || estimate.EstimatedInputTokens < remaining {
		return
	}

	message =
		fmt.Sprintf("This session is close to its token budget. Estimated prompt tokens (%d) ", estimate.EstimatedInputTokens) +
			fmt.Sprintf("meet or exceed the remaining budget (%d). Please start a new conversation.", remaining)
	if a.metrics != nil {
		a.metrics.IncrementEstimatedTokenAdmissionRejects()
	}
	a.logger.Info(
		"Estimated token admission control rejected session",
		"SessionId", session.Id,
		"EstimatedInputTokens", estimate.EstimatedInputTokens,
		"RemainingBudget", remaining)

	ok = true
	return
}

func (a *AgentRuntime) TrimHistory(session *core.Session) {
	if len(session.History) < a.maxHistoryTurns {
		return
	}

	var toRemove = len(session.History) - a.maxHistoryTurns
	session.History = session.History[toRemove:]
}

func BuildTurnContents(content string) []contents.IAIContent {
	markers, remainingText := core.MediaMarkerExtract(content)
	aicontents := []contents.IAIContent{}
	if remainingText != "" {
		aicontents = append(aicontents, contents.NewTextContent(remainingText))
	}

	for _, marker := range markers {
		mediaType := "application/octet-stream"

		switch marker.Kind {
		case core.MediaMarkerImageUrl, core.MediaMarkerImagePath, core.MediaMarkerTelegramImageFileId:
			mediaType = "image/*"
		case core.MediaMarkerAudioUrl, core.MediaMarkerTelegramAudioFileId:
			mediaType = "audio/*"
		case core.MediaMarkerVideoUrl, core.MediaMarkerTelegramVideoFileId:
			mediaType = "video/*"
		}

		switch marker.Kind {
		case core.MediaMarkerImagePath, core.MediaMarkerFilePath:
			aicontents = append(aicontents, contents.NewUriContent(marker.Value, mediaType))
		default:

			baseURL, err := url.Parse(marker.Value)
			if err == nil && (baseURL != nil && (baseURL.IsAbs())) {
				aicontents = append(aicontents, contents.NewUriContent(marker.Value, mediaType))
			} else {
				aicontents = append(aicontents, contents.NewTextContent(marker.Value))
			}
		}

	}

	if len(aicontents) == 0 {
		aicontents = append(aicontents, contents.NewTextContent(content))
	}
	return aicontents
}

func CombineSystemPromptSuffixes(first, second string) string {
	if first == "" {
		return second
	}
	if second == "" {
		return first
	}

	return strings.TrimSpace(first) + "\n" + strings.TrimSpace(second)
}

func RewriteMetaTemplateJson(withJson, rootInput string, outputs map[string]string) string {
	var resolved = ResolveMetaTemplate(withJson, &rootInput, outputs)
	if util.IsValidJson(resolved) {
		return resolved
	}
	return withJson
}

var metaTemplateRegex = regexp.MustCompile(`{{\s*([^{}]+?)\s*}}`)

func ResolveMetaTemplate(template string, rootInput *string, outputs map[string]string) string {
	return metaTemplateRegex.ReplaceAllStringFunc(template, func(match string) string {
		// 提取分组捕获的内容（即 token）
		submatches := metaTemplateRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return ""
		}
		token := strings.TrimSpace(submatches[1])

		// 判断 input 匹配（忽略大小写）
		if strings.EqualFold(token, "input") || strings.EqualFold(token, "inputs.user_message") {
			if rootInput != nil {
				return *rootInput
			}
			return ""
		}

		// 判断 outputs. 前缀匹配（忽略大小写）
		outputPrefix := "outputs."
		if len(token) >= len(outputPrefix) && strings.EqualFold(token[:len(outputPrefix)], outputPrefix) {
			key := token[len(outputPrefix):]
			if strings.TrimSpace(key) != "" {
				if val, ok := outputs[key]; ok {
					return val
				}
			}
		}

		return ""
	})
}
