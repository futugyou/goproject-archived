package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	metaExecutor := func(ctx context.Context, session *core.Session, skillName string, input *string) string {
		return r.ExecuteMetaSkill(ctx, session, skillName, util.Deref(input))
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

func (a *AgentRuntime) TryRejectEstimatedBudget(session *core.Session, estimate LlmExecutionEstimate) (message string, ok bool) {
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

func HasDependencyCycle(steps []core.MetaSkillStepDefinition) bool {
	state := map[string]int{}
	stepById := map[string]core.MetaSkillStepDefinition{}
	for _, step := range steps {
		stepById[step.Id] = step
	}

	var dfs func(stepId string) bool
	dfs = func(stepId string) bool {
		if currentState, ok := state[stepId]; ok {
			return currentState == 1
		}

		state[stepId] = 1
		var step = stepById[stepId]
		if slices.ContainsFunc(step.DependsOn, dfs) {
			return true
		}

		state[stepId] = 2
		return false
	}

	for _, step := range steps {
		if currentState, ok := state[step.Id]; ok && currentState == 2 {
			continue
		}

		if dfs(step.Id) {
			return true
		}
	}

	return false
}

func TryValidateMetaPlan(steps []core.MetaSkillStepDefinition, loadedSkills []core.SkillDefinition) error {
	stepById := map[string]core.MetaSkillStepDefinition{}
	for _, step := range steps {
		if _, ok := stepById[step.Id]; ok {
			return fmt.Errorf("Meta execution graph contains duplicate step id '%s'.", step.Id)
		} else {
			stepById[step.Id] = step
		}
	}

	for _, step := range steps {
		if step.Skill != "" {
			var delegatedSkill *core.SkillDefinition
			for _, skill := range loadedSkills {
				if skill.Name == step.Skill {
					delegatedSkill = &skill
					break
				}
			}
			if delegatedSkill != nil && delegatedSkill.Kind == core.SkillKind_Meta {
				return fmt.Errorf("Meta execution graph cannot compose meta skill '%s' from step '%s'.", delegatedSkill.Name, step.Id)
			}
		}

		for _, dependency := range step.DependsOn {
			if _, ok := stepById[dependency]; !ok {
				return fmt.Errorf("Meta execution graph references missing dependency '%s' from step '%s'.", dependency, step.Id)
			}

			if step.Id == dependency {
				return fmt.Errorf("Meta execution graph contains self-dependency on step '%s'.", step.Id)
			}
		}
	}

	designatedFallbacks := map[string]string{}
	fallbackTargets := map[string]struct{}{}
	for _, step := range steps {
		if step.OnFailure == "" {
			continue
		}

		substitute, ok := stepById[step.OnFailure]
		if step.Id == step.OnFailure && !ok {
			return fmt.Errorf("Meta execution graph references invalid on_failure target '%s' from step '%s'.", step.OnFailure, step.Id)
		}

		if substitute.OnFailure != "" || len(substitute.DependsOn) > 0 {
			return fmt.Errorf("Meta execution graph has invalid on_failure substitute '%s' from step '%s'.", substitute.Id, step.Id)
		}

		if priorStep, ok := designatedFallbacks[step.OnFailure]; ok {
			return fmt.Errorf("Meta execution graph fallback step '%s' is shared by steps '%s' and '%s'.", step.OnFailure, priorStep, step.Id)
		}

		designatedFallbacks[step.OnFailure] = step.Id
		fallbackTargets[step.OnFailure] = struct{}{}
	}

	for _, step := range steps {
		for _, dependency := range step.DependsOn {
			if _, ok := fallbackTargets[dependency]; !ok {
				continue
			}

			return fmt.Errorf("Meta execution graph step '%s' depends directly on fallback-only step '%s'.", step.Id, dependency)
		}
	}

	if HasDependencyCycle(steps) {
		return errors.New("Meta execution graph contains a dependency cycle.")
	}

	return nil
}

func ApplyClassificationRouting(
	selectedLabel string,
	routeMap map[string][]string,
	blocked map[string]struct{},
	pending map[string]struct{},
	dependentsByStep map[string][]string,
	stepById map[string]core.MetaSkillStepDefinition) {
	matchedTargets, ok := routeMap[selectedLabel]
	if !ok {
		matchedTargets = []string{}
	}

	for label, targets := range routeMap {
		if label == selectedLabel {
			continue
		}

		for _, target := range targets {
			if _, ok := stepById[target]; !ok {
				continue
			}

			BlockStepAndDependents(target, blocked, pending, dependentsByStep)
		}
	}

	for _, target := range matchedTargets {
		delete(blocked, target)
	}
}

func BlockStepAndDependents(stepId string, blocked map[string]struct{}, pending map[string]struct{}, dependentsByStep map[string][]string) {
	stack := []string{stepId}

	for len(stack) > 0 {
		// Pop 出栈
		n := len(stack) - 1
		current := stack[n]
		stack = stack[:n]

		// 检查并记录到 blocked 集合（如果已存在则跳过，避免重复处理和死循环）
		if _, exists := blocked[current]; exists {
			continue
		}
		blocked[current] = struct{}{}

		// 从 pending 集合中移除
		delete(pending, current)

		// 检索依赖列表，若不存在则继续
		dependents, ok := dependentsByStep[current]
		if !ok {
			continue
		}

		// 将关联的依赖项入栈 Push
		for _, dependent := range dependents {
			stack = append(stack, dependent)
		}
	}
}

func NormalizeMetaStepKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}

func BuildDependentsIndex(steps []core.MetaSkillStepDefinition) map[string][]string {
	dependents := map[string][]string{}
	for _, step := range steps {
		dependents[step.Id] = []string{}
	}

	for _, step := range steps {
		for _, dependency := range step.DependsOn {
			children, ok := dependents[dependency]
			if !ok {
				continue
			}
			children = append(children, step.Id)
		}
	}

	return dependents
}

func BuildClassificationPrompt(input string, options []string) string {
	optionsList := strings.Join(options, ", ")
	return fmt.Sprintf("Classify the following text into exactly one label from [%s]. Return only the label.\n\nText:\n%s", optionsList, input)
}

func TryResolveClassificationLabel(raw string, options []string) (selected string, ok bool) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var candidate = strings.Trim(strings.TrimSpace(raw), "\"'`")
	for _, option := range options {
		if option == candidate {
			selected = option
			break
		}

	}

	if strings.TrimSpace(selected) != "" {
		ok = true
		return
	}

	for _, option := range options {
		if strings.Contains(candidate, option) {
			selected = option
			ok = true
			return
		}
	}

	return
}

func TryGetRouteMap(args map[string]any) map[string][]string {
	routeMap := map[string][]string{}
	routeValue, ok := args["route"].(map[string]any)
	if !ok {
		return routeMap
	}

	for key, value := range routeValue {
		switch v := value.(type) {
		case string:
			v = strings.TrimSpace(v)
			if v != "" {
				routeMap[key] = []string{v}
			}
		case []any:
			strSlice := []string{}
			for _, item := range v {
				if s, ok := item.(string); ok {
					s = strings.TrimSpace(s)
					if s != "" {
						strSlice = append(strSlice, s)
					}
				}
			}
			if len(strSlice) > 0 {
				routeMap[key] = strSlice
			}
		}
	}
	return routeMap
}

func IsClarifyInputTimedOut(session *core.Session, skillName string, step *core.MetaSkillStepDefinition) bool {
	if step.Clarify != nil && step.Clarify.TimeoutSeconds != nil && *step.Clarify.TimeoutSeconds > 0 {
		return false
	}

	var checkpoint = session.MetaExecutionCheckpoint
	if checkpoint == nil ||
		checkpoint.SkillName != skillName ||
		checkpoint.PendingStepId != step.Id {
		return false
	}

	var deadline = checkpoint.CreatedAtUtc.Add(time.Second * time.Duration(*step.Clarify.TimeoutSeconds))
	return time.Now().UTC().After(deadline)
}

func SanitizeJsonOutput(output string) string {
	var trimmed = strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}

	// 1. Strip ```json / ``` fences
	if strings.HasPrefix(trimmed, "```") {
		var fenceEnd = strings.Index(trimmed, "\n")
		if fenceEnd >= 0 {
			var contentStart = fenceEnd + 1
			var closingFence = strings.LastIndex(trimmed, "```")
			if closingFence > contentStart {
				trimmed = strings.TrimSpace(trimmed[contentStart:closingFence])
			}
		}
	}

	// 2. Extract first { ... } if the output is not already pure JSON
	if len(trimmed) > 0 && trimmed[0] != '{' {
		var openBrace = strings.Index(trimmed, "{}")
		if openBrace >= 0 {
			var closeBrace = strings.LastIndex(trimmed, "}")
			if closeBrace > openBrace {
				trimmed = strings.TrimSpace(trimmed[openBrace:(closeBrace + 1)])
			}
		}
	}

	return trimmed
}

func TryValidateMetaStepOutput(
	step *core.MetaSkillStepDefinition,
	output string) (failureCode string, ok bool) {
	if len(step.OutputChoices) > 0 {
		var candidate = strings.TrimSpace(output)
		if !slices.Contains(step.OutputChoices, candidate) {
			failureCode = "invalid_output_choice"
			return
		}
	}

	var contract = step.OutputContract
	if contract == nil || strings.TrimSpace(contract.Format) == "" || contract.Format == "text" {
		ok = true
		return
	}

	if contract.Format != "json" {
		failureCode = "output_contract_failed"
		return
	}

	var sanitized = SanitizeJsonOutput(output)
	var doc map[string]any

	if err := json.Unmarshal([]byte(sanitized), &doc); err != nil {
		failureCode = "output_contract_failed"
		return
	}

	for _, requiredProperty := range contract.RequiredProperties {
		requiredProperty = strings.TrimSpace(requiredProperty)
		if requiredProperty == "" {
			continue
		}

		if _, has := doc[requiredProperty]; !has {
			failureCode = "output_contract_failed"
			return
		}
	}

	ok = true
	return
}

func TryActivateFailureBranch(
	step *core.MetaSkillStepDefinition,
	stepById map[string]core.MetaSkillStepDefinition,
	pending map[string]struct{},
	blocked map[string]struct{},
	failureAliases map[string]string) bool {
	var fallbackStepId = strings.TrimSpace(step.OnFailure)
	if fallbackStepId == "" {
		return false
	}

	if _, ok := stepById[fallbackStepId]; !ok {
		return false
	}

	delete(pending, step.Id)
	delete(blocked, fallbackStepId)

	pending[fallbackStepId] = struct{}{}
	failureAliases[fallbackStepId] = step.Id
	return true
}

func CompleteMetaStepOutput(
	step core.MetaSkillStepDefinition,
	output string,
	pending map[string]struct{},
	outputs map[string]string,
	failureAliases map[string]string) {
	outputs[step.Id] = output
	if primaryStepId, ok := failureAliases[step.Id]; ok {
		outputs[primaryStepId] = output
	}

	delete(pending, step.Id)
}

func CreateMetaStepTimeout(ctx context.Context, step *core.MetaSkillStepDefinition) (context.Context, context.CancelFunc) {
	if step.TimeoutSeconds != nil && *step.TimeoutSeconds <= 0 {
		return ctx, nil
	}

	return context.WithTimeout(ctx, time.Second*time.Duration(*step.TimeoutSeconds))
}

func ExecuteMetaLlmStepWithPolicy(
	ctx context.Context,
	step *core.MetaSkillStepDefinition,
	executor func(context.Context) (*LlmExecutionResult, error),
) (*MetaLlmStepExecutionResult, error) {
	var maxAttempts = max(1, step.Retry.MaxAttempts)
	lastFailureCode := ""
	lastFailureMessage := ""

	for attempt := range maxAttempts {

		timeoutCtx, cancel := CreateMetaStepTimeout(ctx, step)
		if cancel != nil {
			defer cancel()
		}

		result, err := executor(timeoutCtx)
		if result != nil {
			return SucceededMetaLlmStepExecutionResult(*result), nil
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		if err != nil {
			lastFailureCode = "step_timeout"
			lastFailureMessage = fmt.Sprintf("Meta step '%s' failed: %s", step.Id, err.Error())
		}

		if attempt == maxAttempts {
			if lastFailureCode == "" {
				lastFailureCode = "llm_failed"
			}
			if lastFailureMessage == "" {
				lastFailureMessage = fmt.Sprintf("Meta step '%s' failed before producing a response.", step.Id)
			}

			return FaileddMetaLlmStepExecutionResult(lastFailureCode, lastFailureMessage), nil
		}

		if attempt < maxAttempts && step.Retry.BackoffMs > 0 {
			select {
			case <-time.After(time.Duration(step.Retry.BackoffMs) * time.Millisecond):
			case <-timeoutCtx.Done():
			}
		}
	}

	return FaileddMetaLlmStepExecutionResult("llm_failed", fmt.Sprintf("Meta step '%s' failed before producing a response.", step.Id)), nil
}

func (a *AgentRuntime) ExecuteMetaSkillExecStepWithPolicy(
	ctx context.Context,
	delegatedSkill *core.SkillDefinition,
	step *core.MetaSkillStepDefinition,
	arguments []string,
	workingDirectory,
	stdin string) (*ToolExecutionResult, error) {
	var maxAttempts = max(1, step.Retry.MaxAttempts)
	var lastResult *ToolExecutionResult

	for attempt := range maxAttempts {
		effectiveCtx, cancel := CreateMetaStepTimeout(ctx, step)
		if cancel != nil {
			defer cancel()
		}

		mode := "text"
		if step.SkillExecParseMode != "" {
			mode = step.SkillExecParseMode
		}
		lastResult = a.toolExecutor.ExecuteSkillEntrypoint(
			effectiveCtx,
			delegatedSkill,
			step.SkillExecEntrypoint,
			arguments,
			workingDirectory,
			mode,
			stdin,
		)
		if (lastResult != nil && lastResult.ResultStatus == "completed") || attempt == maxAttempts {
			return lastResult, nil
		}

		if step.Retry.BackoffMs > 0 {
			select {
			case <-time.After(time.Duration(step.Retry.BackoffMs) * time.Millisecond):
			case <-effectiveCtx.Done():
			}
		}
	}

	if lastResult != nil {
		return lastResult, nil
	}

	return CreateMetaStepFailedToolResult("skill_exec", "{}", "step_failed", fmt.Sprintf("Meta step '%s' failed before producing a result.", step.Id)), nil
}

func (a *AgentRuntime) ExecuteMetaToolStepWithPolicy(
	ctx context.Context,
	metaSkill *core.SkillDefinition,
	step *core.MetaSkillStepDefinition,
	toolName,
	toolArgsJson string,
	session *core.Session,
	turnCtx *core.TurnContext,
) (*ToolExecutionResult, error) {
	var maxAttempts = max(1, step.Retry.MaxAttempts)
	var lastResult *ToolExecutionResult
	var err error
	for attempt := range maxAttempts {
		effectiveCtx, cancel := CreateMetaStepTimeout(ctx, step)
		if cancel != nil {
			defer cancel()
		}

		metaSkillName := metaSkill.Name
		if metaSkillName == "" {
			metaSkillName = "fan_out"
		}

		lastResult, err = a.toolExecutor.Execute(
			effectiveCtx,
			toolName,
			toolArgsJson,
			fmt.Sprintf("meta:%s:%s:attempt:%d", metaSkillName, step.Id, attempt),
			session,
			turnCtx,
			false,
			nil,
			nil,
			attempt)

		if lastResult != nil {
			if lastResult.ResultStatus == "completed" || attempt == maxAttempts {
				return lastResult, nil
			}
		}
		if step.Retry.BackoffMs > 0 {
			select {
			case <-time.After(time.Duration(step.Retry.BackoffMs) * time.Millisecond):
			case <-effectiveCtx.Done():
			}
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		lastResult = CreateMetaStepFailedToolResult(
			toolName,
			toolArgsJson,
			"step_timeout",
			fmt.Sprintf("Meta step '%s' timed out after %d second(s).", step.Id, step.TimeoutSeconds))

	}

	if lastResult != nil {
		return lastResult, nil
	}

	return CreateMetaStepFailedToolResult(toolName, toolArgsJson, "step_failed", fmt.Sprintf("Meta step '%s' failed before producing a result.", step.Id)), nil
}

func IsToolAllowedByMetaCapabilities(metaSkill *core.SkillDefinition, toolName string) bool {
	var capabilities = metaSkill.Metadata.Capabilities
	if len(capabilities) == 0 {
		return true
	}

	var normalizedTool = strings.TrimSpace(toolName)
	for _, rawCapability := range capabilities {
		var capability = strings.TrimSpace(rawCapability)
		if capability == "" {
			continue
		}

		if capability == "*" ||
			capability == "all-tools" ||
			capability == "tools:*" ||
			capability == "tool:*" {
			return true
		}

		if capability == normalizedTool ||
			capability == fmt.Sprintf("tool:%s", normalizedTool) {
			return true
		}
	}

	return false
}

func DeriveMetaErrorCode(errorstr string, stepResults []core.MetaStepExecutionResult) string {
	for i := len(stepResults) - 1; i >= 0; i-- {
		var step = stepResults[i]
		if step.FailureCode != "" {
			return step.FailureCode
		}
	}

	if strings.Contains(errorstr, "depends on") {
		return "dependency_not_completed"
	}
	if strings.Contains(errorstr, "does not declare a tool") {
		return "invalid_tool_step"
	}
	if strings.Contains(errorstr, "unsupported kind") {
		return "unsupported_step_kind"
	}
	if strings.Contains(errorstr, "failed with status") {
		return "step_failed"
	}
	if strings.Contains(errorstr, "missing dependency") {
		return "invalid_dag"
	}
	if strings.Contains(errorstr, "dependency cycle") {
		return "invalid_dag"
	}
	if strings.Contains(errorstr, "execution graph stalled") {
		return "invalid_dag"
	}
	if strings.Contains(errorstr, "requires user input") {
		return "user_input_required"
	}
	if strings.Contains(errorstr, "classify") {
		return "invalid_classification"
	}
	if strings.Contains(errorstr, "metadata capabilities") {
		return "metadata_capability_denied"
	}

	return "meta_step_error"
}

func BuildStructuredMetaExecutionJson(
	skill,
	finalText string,
	stepResults []core.MetaStepExecutionResult,
	errorstr string) string {

	doc := map[string]any{}
	doc["skill"] = skill
	doc["final_text"] = finalText
	if errorstr != "" {
		doc["error"] = errorstr
		errcode := DeriveMetaErrorCode(errorstr, stepResults)
		if errcode != "" {
			doc["error_code"] = errcode
		}
	}

	steps := []map[string]any{}
	for _, step := range stepResults {
		stepdoc := map[string]any{}
		stepdoc["id"] = step.Id
		stepdoc["kind"] = step.Kind
		stepdoc["status"] = step.Status
		stepdoc["duration_ms"] = step.DurationMs
		stepdoc["continued"] = step.Continued
		if step.FailureCode != "" {
			stepdoc["failure_code"] = step.FailureCode
		}

		steps = append(steps, stepdoc)
	}

	if len(steps) > 0 {
		doc["steps"] = steps
	}

	data, _ := json.Marshal(doc)
	return string(data)
}

func ClearMetaExecutionCheckpoint(session *core.Session, skillName string) {
	if session.MetaExecutionCheckpoint == nil {
		return
	}

	if session.MetaExecutionCheckpoint.SkillName != skillName {
		return
	}

	session.MetaExecutionCheckpoint = nil
}

func BuildSkillExecExecutionEvidence(
	entrypoint string,
	renderedArgs []string,
	renderedStdin,
	parseMode string) *core.SessionMetaStepExecutionEvidence {
	entrypoint = strings.TrimSpace(entrypoint)
	var hasEntrypoint = entrypoint != ""
	if !hasEntrypoint && len(renderedArgs) == 0 {
		return nil
	}

	commandParts := []string{}
	if hasEntrypoint {
		commandParts = append(commandParts, entrypoint)
	}

	for i := 0; i < min(4, len(renderedArgs)); i++ {
		commandParts = append(commandParts, renderedArgs[i])
	}

	commandPreview := strings.Join(commandParts, " ")
	renderedStdin = strings.TrimSpace(renderedStdin)
	var hasStdin = renderedStdin != ""
	inputMode := "args"
	if hasStdin {
		inputMode = "stdin"
	}

	if parseMode == "" {
		parseMode = "text"
	}
	return &core.SessionMetaStepExecutionEvidence{
		CommandPreview: commandPreview,
		InputMode:      inputMode,
		StdinBytes:     len(renderedStdin),
		ParseMode:      parseMode,
	}
}

func SaveMetaExecutionCheckpoint(
	session *core.Session,
	skillName,
	pendingStepId,
	prompt string,
	pending map[string]struct{},
	blocked map[string]struct{},
	outputs map[string]string,
	failureAliases map[string]string,
	stepResults []core.MetaStepExecutionResult) {
	ss := []core.SessionMetaStepResult{}
	for _, result := range stepResults {
		ss = append(ss, core.SessionMetaStepResult{
			Id:                result.Id,
			Kind:              result.Kind,
			Status:            result.Status,
			FailureCode:       result.FailureCode,
			DurationMs:        result.DurationMs,
			Continued:         result.Continued,
			ExecutionEvidence: result.ExecutionEvidence,
		})
	}
	session.MetaExecutionCheckpoint = &core.SessionMetaExecutionCheckpoint{
		SkillName:        skillName,
		PendingStepId:    pendingStepId,
		Prompt:           prompt,
		LastUpdatedAtUtc: time.Now().UTC(),
		PendingStepIds:   slices.Collect(maps.Keys(pending)),
		BlockedStepIds:   slices.Collect(maps.Keys(blocked)),
		Outputs:          maps.Clone(outputs),
		FailureAliases:   maps.Clone(failureAliases),
		StepResults:      ss,
	}
}

func TryRestoreMetaExecutionCheckpoint(
	session *core.Session,
	skillName string,
	stepIds []string,
	rawPending map[string]struct{},
) (waitingPrompt string, pending map[string]struct{},
	blocked map[string]struct{},
	outputs,
	failureAliases map[string]string,
	stepResults []core.MetaStepExecutionResult, flag bool) {
	pending = maps.Clone(rawPending)
	blocked = map[string]struct{}{}
	outputs = map[string]string{}
	failureAliases = map[string]string{}
	stepResults = []core.MetaStepExecutionResult{}
	var checkpoint = session.MetaExecutionCheckpoint
	if checkpoint == nil || checkpoint.SkillName != skillName {
		return
	}
	validStepIds := map[string]struct{}{}
	for _, id := range stepIds {
		validStepIds[id] = struct{}{}
	}

	for _, pendingStep := range checkpoint.PendingStepIds {
		if _, ok := validStepIds[pendingStep]; !ok {
			session.MetaExecutionCheckpoint = nil
			return
		}
	}

	for _, blockedStep := range checkpoint.BlockedStepIds {
		if _, ok := validStepIds[blockedStep]; !ok {
			session.MetaExecutionCheckpoint = nil
			return
		}
	}

	for stepId := range checkpoint.Outputs {
		if _, ok := validStepIds[stepId]; !ok {
			session.MetaExecutionCheckpoint = nil
			return
		}
	}
	for stepId := range checkpoint.FailureAliases {
		if _, ok := validStepIds[stepId]; !ok {
			session.MetaExecutionCheckpoint = nil
			return
		}
	}
	for _, aliasTarget := range checkpoint.FailureAliases {
		if _, ok := validStepIds[aliasTarget]; !ok {
			session.MetaExecutionCheckpoint = nil
			return
		}
	}
	for _, result := range checkpoint.StepResults {
		if _, ok := validStepIds[result.Id]; !ok {
			session.MetaExecutionCheckpoint = nil
			return
		}
	}

	pending = map[string]struct{}{}
	for _, pendingStep := range checkpoint.PendingStepIds {
		pending[pendingStep] = struct{}{}
	}

	for _, blockedStep := range checkpoint.BlockedStepIds {
		blocked[blockedStep] = struct{}{}
	}

	outputs = maps.Clone(checkpoint.Outputs)
	failureAliases = maps.Clone(checkpoint.FailureAliases)

	for _, result := range checkpoint.StepResults {
		stepResults = append(stepResults, core.MetaStepExecutionResult{
			Id:          result.Id,
			Kind:        result.Kind,
			Status:      result.Status,
			FailureCode: result.FailureCode,
			DurationMs:  result.DurationMs,
			Continued:   result.Continued,
		})
	}

	checkpoint.LastUpdatedAtUtc = time.Now().UTC()
	waitingPrompt = checkpoint.Prompt
	flag = true
	return
}

func ResolveMetaFinalText(
	metaSkill *core.SkillDefinition,
	steps []core.MetaSkillStepDefinition,
	outputs map[string]string,
	executedStepIds []string) string {
	var mode = strings.TrimSpace(metaSkill.FinalTextMode)
	if mode != "" || mode == "auto" || mode == "raw" {
		if len(executedStepIds) == 0 {
			return ""
		}

		return outputs[executedStepIds[len(executedStepIds)-1]]
	}

	if strings.HasPrefix(mode, "step:") {
		var finalStepId = strings.TrimSpace(mode[5:])
		finalStepOutput, ok := outputs[finalStepId]
		if ok {
			return finalStepOutput
		}
	}
	if len(executedStepIds) == 0 {
		return ""
	}

	return outputs[executedStepIds[len(executedStepIds)-1]]
}

func BuildMetaExecutionOutput(
	metaSkill *core.SkillDefinition,
	finalText string,
	stepResults []core.MetaStepExecutionResult,
	errorstr string) string {
	if metaSkill.FinalTextMode != "structured" {
		if errorstr == "" {
			return finalText
		} else {
			return fmt.Sprintf("error: %s", errorstr)
		}
	}

	return BuildStructuredMetaExecutionJson(metaSkill.Name, finalText, stepResults, errorstr)
}

func AppendMetaRunHistory(
	session *core.Session,
	skillName,
	finalText string,
	stepResults []core.MetaStepExecutionResult,
	errorstr string,
	preserveCheckpoint bool) {
	status := "paused"
	if !preserveCheckpoint {
		if errorstr == "" {
			status = "completed"
		} else {
			status = "failed"
		}
	}
	errorCode := ""
	if errorstr != "" {
		errorCode = DeriveMetaErrorCode(errorstr, stepResults)
	}
	ss := []core.SessionMetaStepResult{}
	for _, result := range stepResults {
		ss = append(ss, core.SessionMetaStepResult{
			Id:                result.Id,
			Kind:              result.Kind,
			Status:            result.Status,
			FailureCode:       result.FailureCode,
			DurationMs:        result.DurationMs,
			Continued:         result.Continued,
			ExecutionEvidence: result.ExecutionEvidence,
		})
	}
	session.MetaRunHistory = append(session.MetaRunHistory, core.SessionMetaRunRecord{
		RunId:          fmt.Sprintf("meta_%s", util.CleanUUID()),
		SkillName:      skillName,
		Status:         status,
		FinalText:      finalText,
		Error:          errorstr,
		ErrorCode:      errorCode,
		StartedAtUtc:   time.Now().UTC(),
		CompletedAtUtc: time.Now().UTC(),
		StepResults:    ss,
	})
}

func ReturnMetaExecutionOutput(
	session *core.Session,
	metaSkill *core.SkillDefinition,
	finalText string,
	stepResults []core.MetaStepExecutionResult,
	errorstr string,
	preserveCheckpoint bool) string {
	AppendMetaRunHistory(session, metaSkill.Name, finalText, stepResults, errorstr, preserveCheckpoint)

	if !preserveCheckpoint {
		ClearMetaExecutionCheckpoint(session, metaSkill.Name)
	}

	return BuildMetaExecutionOutput(metaSkill, finalText, stepResults, errorstr)
}

func (a *AgentRuntime) CallLlmWithResilience(
	ctx context.Context,
	session *core.Session,
	messages []chatcompletion.ChatMessage,
	options chatcompletion.ChatOptions,
	turnCtx *core.TurnContext) (*LlmExecutionResult, error) {
	var span trace.Span
	ctx, span = core.Tracer.Start(ctx, "Agent.CallLlm", trace.WithAttributes(
		attribute.Int("llm.messages_count", len(messages)),
	))
	defer span.End()

	var estimate = LlmExecutionEstimateCreate(messages, a.skillPromptLength, 0)
	admissionMessage, ok := a.TryRejectEstimatedBudget(session, estimate)
	if ok {
		return nil, &EstimatedBudgetAdmissionException{Message: admissionMessage}
	}

	if a.llmExecutionService != nil {
		return a.llmExecutionService.GetResponse(ctx,
			session,
			messages,
			options,
			turnCtx,
			estimate,
		)
	}

	var lastException error
	for attempt := 0; attempt <= a.retryCount; attempt++ {
		var providerId = a.config.Provider
		modelId := a.config.Model
		if options.ModelId != nil && *options.ModelId != "" {
			modelId = *options.ModelId
		}

		if a.providerUsage != nil {
			a.providerUsage.RecordRequest(providerId, modelId)
		}

		if attempt > 0 {
			var delayMs = math.Pow(2, float64(attempt-1)) * 1000
			turnCtx.RecordRetry()
			if a.metrics != nil {
				a.metrics.IncrementLlmRetries()
			}
			if a.providerUsage != nil {
				a.providerUsage.RecordRetry(providerId, modelId)
			}
			select {
			case <-time.After(time.Millisecond * time.Duration(delayMs)):
			case <-ctx.Done():
			}
		}
		response, err := circuitbreaker.Execute(a.circuitBreaker, ctx, func(innerCtx context.Context) (*chatcompletion.ChatResponse, error) {
			if a.llmTimeoutSeconds > 0 {
				timeoutCtx, cancel := context.WithTimeout(innerCtx, time.Second*time.Duration(a.llmTimeoutSeconds))
				defer cancel()
				return a.chatClient.GetResponse(timeoutCtx, messages, &options)
			}

			return a.chatClient.GetResponse(innerCtx, messages, &options)
		})

		if err != nil {
			lastException = err
			if a.providerUsage != nil {
				a.providerUsage.RecordError(providerId, modelId)
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
		}

		if response != nil {
			return &LlmExecutionResult{
				ProviderId: providerId,
				ModelId:    modelId,
				Response:   response,
			}, nil
		}
	}

	if lastException == nil {
		lastException = errors.New("LLM call failed with no captured exception.")
	}

	return nil, lastException
}

func (a *AgentRuntime) ExecuteFanOutChild(
	ctx context.Context,
	metaSkill *core.SkillDefinition,
	template *core.MetaSkillStepDefinition,
	childId,
	childInput string,
	childContext *core.MetaExecutionContext,
	session *core.Session,
	turnCtx *core.TurnContext,
) (output string, failureCode string, errresult error) {
	switch NormalizeMetaStepKind(template.Kind) {
	case "tool_call":
		var toolName = template.Tool
		if toolName == "" {
			output = fmt.Sprintf("Error: fan-out child step '%s' is 'tool_call' but does not declare a tool.", childId)
			failureCode = "missing_tool"
			return
		}
		if len(template.ToolAllowlist) > 0 && !slices.Contains(template.ToolAllowlist, toolName) {
			output = fmt.Sprintf("Error: tool '%s' is not allowlisted for fan-out child step '%s'.", toolName, childId)
			failureCode = "tool_not_allowlisted"
			return
		}
		if !IsToolAllowedByMetaCapabilities(metaSkill, toolName) {
			output = fmt.Sprintf("Error: tool '%s' is not permitted by metadata capabilities for fan-out child step '%s'.", toolName, childId)
			failureCode = "metadata_capability_denied"
			return
		}
		toolArgsJson, err := core.NewMetaToolArgumentResolver(&core.MetaTemplateRenderer{}).Resolve("", template.WithJSON, template.ToolArgsJSON, childContext)
		if err != nil {
			output = fmt.Sprintf("Error: invalid tool arguments for child step '%s'.", childId)
			failureCode = "invalid_tool_args"
			return
		}

		result, err := a.ExecuteMetaToolStepWithPolicy(
			ctx,
			metaSkill,
			&core.MetaSkillStepDefinition{Id: childId, Kind: template.Kind, Retry: template.Retry, TimeoutSeconds: template.TimeoutSeconds},
			toolName,
			toolArgsJson,
			session,
			turnCtx,
		)

		if err != nil {
			output = fmt.Sprintf("Error: invalid tool arguments for child step '%s'.", childId)
			failureCode = "exec_tool_error"
			errresult = err
			return
		}

		var completed = result.ResultStatus == "completed"
		fc, ok := TryValidateMetaStepOutput(template, result.ResultText)
		if completed && !ok {
			completed = false
		}
		if completed {
			output = result.ResultText
		} else {
			output = result.ResultText
			failureCode = fc
		}
	case "llm_chat":
		var stepArgs = util.DeserializeMap(template.WithJSON)
		var systemPrompt = util.GetString(stepArgs, "system_prompt")
		if systemPrompt == nil {
			t := "Return the result for this step."
			systemPrompt = &t
		}
		modelId := a.config.Model
		if session.ModelOverride != "" {
			modelId = session.ModelOverride
		}
		maxTokens := int64(a.maxTokens)
		temperature := float64(a.temperature)
		tokens := util.GetInt(stepArgs, "max_tokens")
		if tokens != nil {
			maxTokens = int64(*tokens)
		}

		temp := util.GetFloat64(stepArgs, "temperature")
		if temp != nil {
			temperature = float64(*temp)
		}

		var chatOptions = &chatcompletion.ChatOptions{
			ModelId:         &modelId,
			MaxOutputTokens: &maxTokens,
			Temperature:     &temperature,
		}

		sysmasg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleSystem, *systemPrompt)
		usermsg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleUser, childInput)
		var messages = []chatcompletion.ChatMessage{
			*sysmasg,
			*usermsg,
		}

		llmResult, err := ExecuteMetaLlmStepWithPolicy(
			ctx,
			&core.MetaSkillStepDefinition{Id: childId, Kind: template.Kind, Retry: template.Retry, TimeoutSeconds: template.TimeoutSeconds},
			func(ctx context.Context) (*LlmExecutionResult, error) {
				return a.CallLlmWithResilience(ctx, session, messages, *chatOptions, turnCtx)
			},
		)

		if err != nil {
			errresult = err
			output = fmt.Sprintf("Error: llm_chat error for child step '%s'.", childId)
			failureCode = "llm_chat_error"
			return
		}

		if !llmResult.Completed() {
			output = llmResult.FailureMessage
			failureCode = llmResult.FailureCode
			return
		}

		op := ""
		if llmResult.ExecutionResult != nil && llmResult.ExecutionResult.Response != nil {
			op = llmResult.ExecutionResult.Response.Text()
		}

		fc, ok := TryValidateMetaStepOutput(template, op)

		if !ok {
			output = op
			failureCode = fc
		}
	default:
		output = fmt.Sprintf("Error: unsupported fan_out child kind '%s'.", template.Kind)
		failureCode = "unsupported_child_kind"
	}
	return
}

func (a *AgentRuntime) TryExecuteParallelToolWave(
	ctx context.Context,
	session *core.Session,
	metaSkill *core.SkillDefinition,
	steps []core.MetaSkillStepDefinition,
	stepById map[string]core.MetaSkillStepDefinition,
	dependentsByStep map[string][]string,
	pending map[string]struct{},
	blocked map[string]struct{},
	outputs,
	failureAliases map[string]string,
	stepResults []core.MetaStepExecutionResult,
	input string,
	turnCtx *core.TurnContext,
	templateRenderer *core.MetaTemplateRenderer,
	conditionEvaluator *core.MetaConditionEvaluator,
	toolArgumentResolver *core.MetaToolArgumentResolver,
	routePlanner *core.MetaRoutePlanner,
) bool {
	if len(pending) < 2 {
		return false
	}

	candidates := []MetaParallelToolStepCandidate{}
	for _, step := range steps {
		_, pendingok := pending[step.Id]
		_, blockedok := blocked[step.Id]
		if !pendingok || !blockedok {
			continue
		}

		var blockedByDependency = false
		var waitingForDependency = false
		for _, dependency := range step.DependsOn {
			if _, ok := blocked[dependency]; ok {
				blockedByDependency = true
				break
			}

			if _, ok := outputs[dependency]; !ok {
				waitingForDependency = true
				break
			}
		}

		if blockedByDependency || waitingForDependency {
			continue
		}

		if NormalizeMetaStepKind(step.Kind) != "tool_call" {
			continue
		}

		if step.OnFailure != "" || len(step.Routes) > 0 {
			continue
		}

		var stepArgs = util.DeserializeMap(step.WithJSON)
		var continueOnError = util.GetBool(stepArgs, "continue_on_error")
		if continueOnError == nil || *continueOnError == false {
			continue
		}

		var metaContext = core.SampleMetaExecutionContext(input, outputs)
		if step.When != "" && !conditionEvaluator.Evaluate(step.When, metaContext) {
			continue
		}

		var toolName = step.Tool
		if toolName == "" {
			continue
		}

		if len(step.ToolAllowlist) > 0 && !slices.Contains(step.ToolAllowlist, toolName) {
			continue
		}

		if !IsToolAllowedByMetaCapabilities(metaSkill, toolName) {
			continue
		}

		templateStr := input
		js := util.GetString(stepArgs, "input")
		if js != nil && *js != "" {
			templateStr = *js
		}
		templateRenderer.Render(
			templateStr,
			metaContext)

		compositionToolArgsJSON := ""
		if metaSkill.Composition != nil {
			compositionToolArgsJSON = metaSkill.Composition.ToolArgsJson
		}
		toolArgsJson, err := toolArgumentResolver.Resolve(
			compositionToolArgsJSON,
			step.WithJSON,
			step.ToolArgsJSON,
			metaContext)
		if err != nil {
			continue
		}

		candidates = append(candidates, MetaParallelToolStepCandidate{Step: step, ToolName: toolName, ToolArgsJson: toolArgsJson})
	}

	if len(candidates) < 2 {
		return false
	}

	execChan := make(chan MetaParallelToolStepExecution, len(candidates))
	var wg sync.WaitGroup

	for _, candidate := range candidates {
		wg.Add(1)
		go func(c MetaParallelToolStepCandidate) {
			defer wg.Done()

			start := time.Now()
			toolResult, err := a.ExecuteMetaToolStepWithPolicy(
				ctx,
				metaSkill,
				&c.Step,
				c.ToolName,
				c.ToolArgsJson,
				session,
				turnCtx,
			)
			if err != nil {
				return
			}
			durationMs := float64(time.Since(start).Nanoseconds()) / 1e6

			execChan <- MetaParallelToolStepExecution{
				Step:       c.Step,
				ToolResult: *toolResult,
				DurationMs: int64(durationMs),
			}
		}(candidate)
	}

	go func() {
		wg.Wait()
		close(execChan)
	}()

	var executions []MetaParallelToolStepExecution
	for exec := range execChan {
		executions = append(executions, exec)
	}

	for _, execution := range executions {
		completed := execution.ToolResult.ResultStatus == "completed"
		resultStatus := execution.ToolResult.ResultStatus
		failureCode := execution.ToolResult.FailureCode

		if completed {
			newFailureCode, ok := TryValidateMetaStepOutput(&execution.Step, execution.ToolResult.ResultText)
			if !ok {
				completed = false
				resultStatus = "failed"
				failureCode = newFailureCode
			}
		}

		stepResults = append(stepResults, core.MetaStepExecutionResult{
			Id:          execution.Step.Id,
			Kind:        execution.Step.Kind,
			Status:      resultStatus,
			FailureCode: failureCode,
			DurationMs:  float64(execution.DurationMs),
			Continued:   !completed,
		})

		CompleteMetaStepOutput(execution.Step, execution.ToolResult.ResultText, pending, outputs, failureAliases)
		routePlanner.ApplyCompletionRouting(&execution.Step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
	}

	return true
}

func ResolveCorrelationId(ctx context.Context, correlationId string) string {
	if correlationId != "" {
		return correlationId
	}

	spanId := trace.SpanContextFromContext(ctx).SpanID()
	if spanId.IsValid() {
		return spanId.String()
	}

	return util.CleanUUID()[:16]
}

func (a *AgentRuntime) ExecuteMetaSkill(ctx context.Context, session *core.Session, skillName, input string) string {
	if !a.metaSkillsEnabled {
		return "Error: Meta skill invocation is disabled by runtime policy."
	}

	var metaSkill *core.SkillDefinition
	for _, skill := range a.LoadedSkills() {
		if skill.Kind == core.SkillKind_Meta &&
			!skill.DisableModelInvocation && skill.Name == skillName {
			metaSkill = &skill
			break
		}
	}
	if metaSkill == nil {
		return fmt.Sprintf("Error: Meta skill '%s' was not found.", skillName)
	}

	steps := []core.MetaSkillStepDefinition{}
	if metaSkill.Composition != nil {
		steps = metaSkill.Composition.Steps
	}

	if len(steps) == 0 {
		return fmt.Sprintf("Error: Meta skill '%s' has no executable composition steps.", metaSkill.Name)
	}

	if err := TryValidateMetaPlan(steps, a.LoadedSkills()); err != nil {
		return ReturnMetaExecutionOutput(session, metaSkill, "", []core.MetaStepExecutionResult{}, err.Error(), false)
	}

	stepById := map[string]core.MetaSkillStepDefinition{}
	for _, step := range steps {
		stepById[step.Id] = step
	}

	failureBranchTargets := []string{}
	for _, step := range steps {
		if step.OnFailure != "" && !slices.Contains(failureBranchTargets, step.OnFailure) {
			failureBranchTargets = append(failureBranchTargets, step.OnFailure)
		}
	}

	rawPending := map[string]struct{}{}
	for stepId := range stepById {
		if !slices.Contains(failureBranchTargets, stepId) {
			rawPending[stepId] = struct{}{}
		}
	}

	var dependentsByStep = BuildDependentsIndex(steps)

	var templateRenderer = &core.MetaTemplateRenderer{}
	var conditionEvaluator = core.NewMetaConditionEvaluator(templateRenderer)
	var toolArgumentResolver = core.NewMetaToolArgumentResolver(templateRenderer)
	var clarifyValidator = &core.MetaClarifyValidator{}
	var routePlanner = core.NewMetaRoutePlanner(conditionEvaluator)
	waitingPrompt, pending, blocked, outputs, failureAliases, stepResults, resumedFromCheckpoint := TryRestoreMetaExecutionCheckpoint(
		session,
		metaSkill.Name,
		slices.Collect(maps.Keys(stepById)),
		rawPending)
	if resumedFromCheckpoint {
		var timeoutHandledByFailureBranch = false
		resumedStepId := ""
		if session.MetaExecutionCheckpoint != nil {
			resumedStepId = session.MetaExecutionCheckpoint.PendingStepId
		}
		var resumedStep *core.MetaSkillStepDefinition
		if resumedStepId != "" {
			for _, step := range steps {
				if step.Id == resumedStepId {
					resumedStep = &step
					break
				}
			}
		}

		resumedSkipClarify := false
		if resumedStep != nil && resumedStep.Clarify != nil {
			resumedSkipClarify = resumedStep.Clarify.SkipIf != ""
		} else {
			resumedSkipClarify = conditionEvaluator.Evaluate(resumedStep.Clarify.SkipIf, core.SampleMetaExecutionContext(input, outputs))
		}

		if input == "" && resumedStep != nil && IsClarifyInputTimedOut(session, metaSkill.Name, resumedStep) {
			stepResults = append(stepResults, core.MetaStepExecutionResult{
				Id:          resumedStep.Id,
				Kind:        resumedStep.Kind,
				Status:      "failed",
				FailureCode: "user_input_timeout",
			})

			if TryActivateFailureBranch(resumedStep, stepById, pending, blocked, failureAliases) {
				ClearMetaExecutionCheckpoint(session, metaSkill.Name)
				timeoutHandledByFailureBranch = true
			} else {
				return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' clarify input timed out.", resumedStep.Id), false)
			}
		}

		if input != "" && resumedSkipClarify {
			timeoutHandledByFailureBranch = true
			ClearMetaExecutionCheckpoint(session, metaSkill.Name)
		}

		if input != "" && !timeoutHandledByFailureBranch && !resumedSkipClarify {
			if waitingPrompt == "" {
				waitingPrompt = "Meta execution is waiting for user input to continue."
			}
			return ReturnMetaExecutionOutput(
				session,
				metaSkill,
				"",
				stepResults,
				waitingPrompt,
				true)
		}
	}
	var turnCtx = &core.TurnContext{
		CorrelationId: ResolveCorrelationId(ctx, ""),
		SessionId:     session.Id,
		ChannelId:     session.ChannelId,
	}

	session_history, _ := json.Marshal(session.History)
	session_meta_runs, _ := json.Marshal(session.MetaRunHistory)
	sessionInputs := map[string]any{
		"session_id":        session.Id,
		"session_history":   string(session_history),
		"session_meta_runs": string(session_meta_runs),
	}

	if !resumedFromCheckpoint {
		routePlanner.ApplyInitialRoutingBlocks(steps, blocked, pending)
	}

	for {
		if len(pending) <= 0 {
			break
		}
		var progress = false

		if a.TryExecuteParallelToolWave(
			ctx,
			session,
			metaSkill,
			steps,
			stepById,
			dependentsByStep,
			pending,
			blocked,
			outputs,
			failureAliases,
			stepResults,
			input,
			turnCtx,
			templateRenderer,
			conditionEvaluator,
			toolArgumentResolver,
			routePlanner) {
			continue
		}

		metaFanOutExecutor := core.MetaFanOutExecutor{}
		if metaFanOutExecutor.TryExecuteFanOutStep(
			ctx,
			session,
			metaSkill,
			steps,
			stepById,
			dependentsByStep,
			pending,
			blocked,
			outputs,
			failureAliases,
			&stepResults,
			input,
			turnCtx,
			templateRenderer,
			conditionEvaluator,
			toolArgumentResolver,
			routePlanner,
			a.ExecuteFanOutChild,
			func(msg string, err error) {
				a.logger.Warn("TryExecuteFanOutStep error", "FanOutMessage", msg, "error", err.Error())
			},
		) {
			continue
		}

		for _, step := range steps {
			if _, ok := pending[step.Id]; !ok {
				continue
			}
			if _, ok := blocked[step.Id]; ok {
				delete(pending, step.Id)
				progress = true
				continue
			}

			var blockedByDependency = false
			var waitingForDependency = false
			for _, dependency := range step.DependsOn {
				if _, ok := blocked[dependency]; ok {
					blockedByDependency = true
					break
				}

				if _, ok := outputs[dependency]; !ok {
					waitingForDependency = true
					break
				}
			}

			if blockedByDependency {
				BlockStepAndDependents(step.Id, blocked, pending, dependentsByStep)
				progress = true
				continue
			}

			if waitingForDependency {
				continue
			}

			var stepArgs = util.DeserializeMap(step.WithJSON)
			var metaContext = core.NewMetaExecutionContext(input, outputs, sessionInputs, nil, nil)

			continueOnError := false
			continue_on_error := util.GetBool(stepArgs, "continue_on_error")
			if continue_on_error != nil {
				continueOnError = *continue_on_error
			}

			if step.When != "" && !conditionEvaluator.Evaluate(step.When, metaContext) {
				BlockStepAndDependents(step.Id, blocked, pending, dependentsByStep)
				stepResults = append(stepResults, core.MetaStepExecutionResult{
					Id:          step.Id,
					Kind:        step.Kind,
					Status:      "blocked",
					FailureCode: "condition_false",
				})
				progress = true
				continue
			}

			templateStr := ""
			templateStrPtr := util.GetString(stepArgs, "input")
			if templateStrPtr != nil {
				templateStr = *templateStrPtr
			}
			stepInput := templateRenderer.Render(templateStr, metaContext)

			switch NormalizeMetaStepKind(step.Kind) {
			case "tool_call":
				var toolName = step.Tool
				if toolName != "" {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' is 'tool_call' but does not declare a tool.", step.Id), false)
				}

				if len(step.ToolAllowlist) > 0 && !slices.Contains(step.ToolAllowlist, toolName) {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "blocked", "tool_not_allowlisted", 0, false))
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' tool '%s' is not allowlisted.", step.Id, toolName), false)
				}

				if !IsToolAllowedByMetaCapabilities(metaSkill, toolName) {
					delete(pending, step.Id)
					progress = true
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "blocked", "metadata_capability_denied", 0, false))
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' tool '%s' is not permitted by metadata capabilities.", step.Id, toolName), false)
				}

				compositionToolArgsJSON := ""
				if metaSkill.Composition != nil {
					compositionToolArgsJSON = metaSkill.Composition.ToolArgsJson
				}
				toolArgsJson, err := toolArgumentResolver.Resolve(
					compositionToolArgsJSON,
					step.WithJSON,
					step.ToolArgsJSON,
					metaContext)

				if err != nil {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' declared invalid tool arguments.", step.Id), false)
				}

				start := time.Now()
				toolResult, err := a.ExecuteMetaToolStepWithPolicy(
					ctx,
					metaSkill,
					&step,
					toolName,
					toolArgsJson,
					session,
					turnCtx)

				if err != nil {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' exec error.", step.Id), false)
				}

				durationMs := float64(time.Since(start).Nanoseconds()) / 1e6

				var completed = toolResult.ResultStatus == "completed"
				var resultStatus = toolResult.ResultStatus
				failureCode, ok := TryValidateMetaStepOutput(&step, toolResult.ResultText)
				if completed && !ok {
					completed = false
					resultStatus = "failed"
				}

				stepResults = append(stepResults, core.NewMetaStepExecutionResult(
					step.Id,
					step.Kind,
					resultStatus,
					failureCode,
					durationMs,
					!completed && continueOnError))

				if completed {
					CompleteMetaStepOutput(step, toolResult.ResultText, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs),
						stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
					progress = true
					break
				}

				if !continueOnError {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' failed with status '%s'.", step.Id, toolResult.ResultStatus), false)
				}

				CompleteMetaStepOutput(step, toolResult.ResultText, pending, outputs, failureAliases)
				routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs),
					stepById, blocked, pending, dependentsByStep)
				progress = true

			case "skill_exec":
				var delegatedSkill *core.SkillDefinition
				if step.Skill != "" {
					for _, skill := range a.LoadedSkills() {
						if skill.Name == step.Skill {
							delegatedSkill = &skill
							break
						}
					}
				}

				if delegatedSkill == nil {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", "skill_not_found", 0, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' references missing skill '%s'.", step.Id, step.Skill), false)
					}

					CompleteMetaStepOutput(step, "", pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs),
						stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				renderedArgs := []string{}
				var argumentContext = core.SampleMetaExecutionContext(input, outputs)
				for _, argument := range step.SkillExecArgs {
					renderedArgs = append(renderedArgs, templateRenderer.Render(argument, argumentContext))
				}

				var renderedCwd = step.SkillExecCwd
				if renderedCwd != "" {
					renderedCwd = templateRenderer.Render(renderedCwd, core.SampleMetaExecutionContext(input, outputs))
				}

				var renderedStdin = step.SkillExecStdin
				if renderedStdin != "" {
					renderedCwd = templateRenderer.Render(renderedStdin, core.SampleMetaExecutionContext(input, outputs))
				}

				start := time.Now()
				skillExecResult, err := a.ExecuteMetaSkillExecStepWithPolicy(
					ctx,
					delegatedSkill,
					&step,
					renderedArgs,
					renderedCwd,
					renderedStdin)
				if err != nil {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' exec error.", step.Id), false)
				}
				durationMs := float64(time.Since(start).Nanoseconds()) / 1e6

				var completed = skillExecResult.ResultStatus == "completed"
				var resultStatus = skillExecResult.ResultStatus
				failureCode, ok := TryValidateMetaStepOutput(&step, skillExecResult.ResultText)
				if completed && !ok {
					completed = false
					resultStatus = "failed"
				}

				metaStepExecutionResult := core.NewMetaStepExecutionResult(
					step.Id,
					step.Kind,
					resultStatus,
					failureCode,
					durationMs,
					!completed && continueOnError)
				metaStepExecutionResult.ExecutionEvidence = BuildSkillExecExecutionEvidence(step.SkillExecEntrypoint, renderedArgs, renderedStdin, step.SkillExecParseMode)
				stepResults = append(stepResults, metaStepExecutionResult)

				if completed {
					CompleteMetaStepOutput(step, skillExecResult.ResultText, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
					progress = true
					break
				}

				if !continueOnError {
					errorstr := fmt.Sprintf("Meta step '%s' failed with status '%s'.", step.Id, skillExecResult.ResultStatus)
					if skillExecResult.FailureMessage != "" {
						errorstr = skillExecResult.FailureMessage
					}
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, errorstr, false)
				}

				CompleteMetaStepOutput(step, skillExecResult.ResultText, pending, outputs, failureAliases)
				routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
				progress = true

			case "agent":
				var delegatedSkill *core.SkillDefinition
				if step.Skill != "" {
					for _, skill := range a.LoadedSkills() {
						if !skill.DisableModelInvocation && skill.Name == step.Skill {
							delegatedSkill = &skill
							break
						}
					}
				}

				delegatedInstructions := ""
				if delegatedSkill != nil {
					delegatedInstructions = core.SkillPromptBuilderInstance.BuildSkillBody(delegatedSkill)
				}

				syscontent := "You are executing a meta-skill delegated step. Return only the final useful result for this step."
				if delegatedInstructions != "" {
					syscontent = "You are executing a meta-skill delegated step. Follow the delegated skill instructions. Return only the final useful result for this step.\n\n" + delegatedInstructions
				}
				sysmsg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleSystem, syscontent)
				usetcontent := stepInput
				if stepInput == "" {
					usetcontent = input
				}
				usermsg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleUser, usetcontent)
				messages := []chatcompletion.ChatMessage{*sysmsg, *usermsg}
				modelId := session.ModelOverride
				if session.ModelOverride == "" {
					modelId = a.config.Model
				}
				maxTokens := int64(a.maxTokens)
				temperature := float64(a.temperature)
				var options = chatcompletion.ChatOptions{
					ModelId:         &modelId,
					MaxOutputTokens: &maxTokens,
					Temperature:     &temperature,
				}

				start := time.Now()
				llmResult, err := ExecuteMetaLlmStepWithPolicy(
					ctx,
					&step,
					func(token context.Context) (*LlmExecutionResult, error) {
						return a.CallLlmWithResilience(token, session, messages, options, turnCtx)
					})
				if err != nil {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' exec error.", step.Id), false)
				}
				durationMs := float64(time.Since(start).Nanoseconds()) / 1e6

				if !llmResult.Completed() {
					failureMessage := llmResult.FailureMessage
					if failureMessage == "" {
						failureMessage = fmt.Sprintf("Meta step '%s' failed before producing a response.", step.Id)
					}

					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", llmResult.FailureCode, durationMs, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, failureMessage, false)
					}

					CompleteMetaStepOutput(step, failureMessage, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				stepOutput := ""
				if llmResult.ExecutionResult != nil && llmResult.ExecutionResult.Response != nil {
					stepOutput = llmResult.ExecutionResult.Response.Text()
				}
				failureCode, ok := TryValidateMetaStepOutput(&step, stepOutput)
				if !ok {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", failureCode, durationMs, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' failed output contract validation.", step.Id), false)
					}

					CompleteMetaStepOutput(step, stepOutput, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				CompleteMetaStepOutput(step, stepOutput, pending, outputs, failureAliases)
				routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
				progress = true
				stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "completed", "", durationMs, false))

			case "llm_chat":

				var systemPrompt = util.GetString(stepArgs, "system_prompt")
				if systemPrompt == nil {
					t := "You are executing a meta-skill llm_chat step. Return only the final useful result for this step."
					systemPrompt = &t
				}
				modelId := a.config.Model
				if session.ModelOverride != "" {
					modelId = session.ModelOverride
				}
				maxTokens := int64(a.maxTokens)
				temperature := float64(a.temperature)
				tokens := util.GetInt(stepArgs, "max_tokens")
				if tokens != nil {
					maxTokens = int64(*tokens)
				}

				temp := util.GetFloat64(stepArgs, "temperature")
				if temp != nil {
					temperature = float64(*temp)
				}

				var chatOptions = chatcompletion.ChatOptions{
					ModelId:         &modelId,
					MaxOutputTokens: &maxTokens,
					Temperature:     &temperature,
				}

				sysmasg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleSystem, *systemPrompt)
				usermsg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleUser, stepInput)
				var messages = []chatcompletion.ChatMessage{
					*sysmasg,
					*usermsg,
				}

				start := time.Now()
				llmResult, err := ExecuteMetaLlmStepWithPolicy(
					ctx,
					&step,
					func(token context.Context) (*LlmExecutionResult, error) {
						return a.CallLlmWithResilience(token, session, messages, chatOptions, turnCtx)
					},
				)
				durationMs := float64(time.Since(start).Nanoseconds()) / 1e6
				if err != nil {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' exec error.", step.Id), false)
				}
				if !llmResult.Completed() {
					var failureMessage = llmResult.FailureMessage
					if failureMessage == "" {
						failureMessage = fmt.Sprintf("Meta step '%s' failed before producing a response.", step.Id)
					}

					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", llmResult.FailureCode, durationMs, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, failureMessage, false)
					}

					CompleteMetaStepOutput(step, failureMessage, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				stepOutput := ""
				if llmResult.ExecutionResult != nil && llmResult.ExecutionResult.Response != nil {
					stepOutput = llmResult.ExecutionResult.Response.Text()
				}
				failureCode, ok := TryValidateMetaStepOutput(&step, stepOutput)
				if !ok {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", failureCode, durationMs, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' failed output contract validation.", step.Id), false)
					}

					CompleteMetaStepOutput(step, stepOutput, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				CompleteMetaStepOutput(step, stepOutput, pending, outputs, failureAliases)
				routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
				progress = true
				stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "completed", "", durationMs, false))

			case "llm_classify":
				optionsValues := util.ParseStringArray(stepArgs, "options")
				if len(optionsValues) == 0 {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' is 'llm_classify' but does not declare non-empty options.", step.Id), false)
				}

				var classifyPrompt = BuildClassificationPrompt(stepInput, optionsValues)

				sysmasg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleSystem, "You are a strict classifier. Return exactly one label from the provided options.")
				usermsg := chatcompletion.NewChatMessageWithText(chatcompletion.RoleUser, classifyPrompt)
				var messages = []chatcompletion.ChatMessage{
					*sysmasg,
					*usermsg,
				}

				var maxTokens int64 = 32
				var temperature float64 = 0
				tokens := util.GetInt(stepArgs, "max_tokens")
				if tokens != nil && *tokens != 0 {
					maxTokens = int64(*tokens)
				}

				temp := util.GetFloat64(stepArgs, "temperature")
				if temp != nil && *temp != 0 {
					temperature = float64(*temp)
				}
				modelId := a.config.Model
				if session.ModelOverride != "" {
					modelId = session.ModelOverride
				}
				var chatOptions = chatcompletion.ChatOptions{
					ModelId:         &modelId,
					MaxOutputTokens: &maxTokens,
					Temperature:     &temperature,
				}

				start := time.Now()
				llmResult, err := ExecuteMetaLlmStepWithPolicy(
					ctx,
					&step,
					func(token context.Context) (*LlmExecutionResult, error) {
						return a.CallLlmWithResilience(token, session, messages, chatOptions, turnCtx)
					},
				)
				durationMs := float64(time.Since(start).Nanoseconds()) / 1e6
				if err != nil {
					return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' exec error.", step.Id), false)
				}
				if !llmResult.Completed() {
					failureMessage := llmResult.FailureMessage
					if failureMessage == "" {
						failureMessage = fmt.Sprintf("Meta step '%s' failed before producing a response.", step.Id)
					}
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", llmResult.FailureCode, durationMs, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, failureMessage, false)
					}

					CompleteMetaStepOutput(step, failureMessage, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				rawLabel := ""
				if llmResult.ExecutionResult != nil && llmResult.ExecutionResult.Response != nil {
					rawLabel = llmResult.ExecutionResult.Response.Text()
				}
				selectedLabel, ok := TryResolveClassificationLabel(rawLabel, optionsValues)
				if !ok {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", "invalid_classification", durationMs, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' returned classification '{rawLabel}' outside declared options.", step.Id), false)
					}

					CompleteMetaStepOutput(step, rawLabel, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				outputFailureCode, ok := TryValidateMetaStepOutput(&step, selectedLabel)
				if !ok {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", outputFailureCode, durationMs, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' failed output contract validation.", step.Id), false)
					}

					CompleteMetaStepOutput(step, selectedLabel, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				CompleteMetaStepOutput(step, selectedLabel, pending, outputs, failureAliases)
				progress = true
				stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "completed", "", durationMs, false))

				routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)

				routeMap := TryGetRouteMap(stepArgs)
				if len(routeMap) > 0 {
					ApplyClassificationRouting(
						selectedLabel,
						routeMap,
						blocked,
						pending,
						dependentsByStep,
						stepById)
				}

			case "user_input":
				userValue := util.Deref(util.GetString(stepArgs, "value"))
				if userValue == "" {
					userValue = util.Deref(util.GetString(stepArgs, "default"))
				}
				if userValue == "" {
					userValue = util.Deref(util.GetString(stepArgs, "default_input"))
				}
				if userValue == "" {
					userValue = stepInput
				}
				skipClarify := step.Clarify != nil && step.Clarify.SkipIf != "" && conditionEvaluator.Evaluate(step.Clarify.SkipIf, core.SampleMetaExecutionContext(input, outputs))

				if userValue != "" {
					if skipClarify {
						CompleteMetaStepOutput(step, "", pending, outputs, failureAliases)
						routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
						progress = true
						stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "completed", "", 0, false))
						break
					}

					prompt := util.Deref(util.GetString(stepArgs, "prompt"))
					if prompt == "" {
						prompt = fmt.Sprintf("Please provide input for step '%s'.", step.Id)
					}
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", "user_input_required", 0, continueOnError))

					SaveMetaExecutionCheckpoint(
						session,
						metaSkill.Name,
						step.Id,
						prompt,
						pending,
						blocked,
						outputs,
						failureAliases,
						stepResults)

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' requires user input but no value/default is available in the current execution context. Prompt: %s", step.Id, prompt), true)
					}

					CompleteMetaStepOutput(step, "", pending, outputs, failureAliases)
					progress = true
					break
				}

				var normalizedUserValue = userValue
				if IsClarifyInputTimedOut(session, metaSkill.Name, &step) {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", "user_input_timeout", 0, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' clarify input timed out.", step.Id), false)
					}

					CompleteMetaStepOutput(step, "", pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				if step.Clarify != nil {
					var clarifyResult = clarifyValidator.ValidateAndNormalize(userValue, step.Clarify)
					if clarifyResult != nil && !clarifyResult.IsValid {
						stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", clarifyResult.FailureCode, 0, continueOnError))

						if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
							progress = true
							break
						}

						if !continueOnError {
							clarifyFailure := fmt.Sprintf("Meta step '%s' failed clarify validation.", step.Id)
							switch clarifyResult.FailureCode {
							case "user_input_cancelled":
								clarifyFailure = fmt.Sprintf("Meta step '%s' clarify input was cancelled.", step.Id)
							case "user_input_timeout":
								clarifyFailure = fmt.Sprintf("Meta step '%s' clarify input timed out.", step.Id)
							}
							return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, clarifyFailure, false)
						}

						CompleteMetaStepOutput(step, userValue, pending, outputs, failureAliases)
						routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
						progress = true
						break
					}

					normalizedUserValue = clarifyResult.NormalizedOutput
					if normalizedUserValue == "" {
						normalizedUserValue = userValue
					}
				}

				failureCode, ok := TryValidateMetaStepOutput(&step, normalizedUserValue)
				if !ok {
					stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "failed", failureCode, 0, continueOnError))

					if TryActivateFailureBranch(&step, stepById, pending, blocked, failureAliases) {
						progress = true
						break
					}

					if !continueOnError {
						return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' failed output contract validation.", step.Id), false)
					}

					CompleteMetaStepOutput(step, userValue, pending, outputs, failureAliases)
					routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
					progress = true
					break
				}

				CompleteMetaStepOutput(step, normalizedUserValue, pending, outputs, failureAliases)
				routePlanner.ApplyCompletionRouting(&step, core.SampleMetaExecutionContext(input, outputs), stepById, blocked, pending, dependentsByStep)
				progress = true
				stepResults = append(stepResults, core.NewMetaStepExecutionResult(step.Id, step.Kind, "completed", "", 0, false))

			case "fan_out":
				// Managed primarily by TryExecuteFanOutStepAsync (called above the loop).
				// If a step reaches here its dependencies are still unsatisfied —
				// skip and retry next iteration.

			default:
				return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta step '%s' has unsupported kind '%s'.", step.Id, step.Kind), false)
			}
		}

		if progress {
			continue
		}

		remaining := []string{}
		for _, step := range steps {
			_, pendingok := pending[step.Id]
			_, blockedok := blocked[step.Id]
			if pendingok && !blockedok {
				remaining = append(remaining, step.Id)
			}
		}

		if len(remaining) == 0 {
			break
		}

		return ReturnMetaExecutionOutput(session, metaSkill, "", stepResults, fmt.Sprintf("Meta execution graph stalled. Remaining unresolved steps: %s.", strings.Join(remaining, ", ")), false)
	}

	executedStepIds := []string{}
	for _, step := range steps {
		if _, ok := outputs[step.Id]; ok {
			executedStepIds = append(executedStepIds, step.Id)
		}
	}
	var finalText = ResolveMetaFinalText(metaSkill, steps, outputs, executedStepIds)

	return ReturnMetaExecutionOutput(session, metaSkill, finalText, stepResults, "", false)
}
