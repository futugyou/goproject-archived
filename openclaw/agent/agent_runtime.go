package agent

import (
	"context"
	"errors"
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

func IsClarifyInputTimedOut(session core.Session, skillName string, step core.MetaSkillStepDefinition) bool {
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
