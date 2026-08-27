package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
	"github.com/google/uuid"
)

type RuntimeFactoryHandler func([]core.ITool, core.LlmProviderConfig, core.AgentProfile) IAgentRuntime

type DelegateTool struct {
	chatClient       chatcompletion.IChatClient
	allTools         []core.ITool
	memory           core.IMemoryStore
	llmConfig        core.LlmProviderConfig
	delegationConfig core.DelegationConfig
	currentDepth     int
	metrics          *core.RuntimeMetrics
	logger           *slog.Logger
	recall           *core.MemoryRecallConfig
	runtimeFactory   RuntimeFactoryHandler
}

func NewDelegateTool(
	chatClient chatcompletion.IChatClient,
	allTools []core.ITool,
	memory core.IMemoryStore,
	llmConfig core.LlmProviderConfig,
	delegationConfig core.DelegationConfig,
	currentDepth int,
	metrics *core.RuntimeMetrics,
	logger *slog.Logger,
	recall *core.MemoryRecallConfig,
	runtimeFactory RuntimeFactoryHandler) *DelegateTool {
	if logger == nil {
		logger = slog.Default()
	}

	if runtimeFactory == nil {
		runtimeFactory = func(tools []core.ITool, subConfig core.LlmProviderConfig, profile core.AgentProfile) IAgentRuntime {
			return NewAgentRuntime(
				chatClient,
				tools,
				memory,
				subConfig,
				profile.MaxHistoryTurns,
				&AgentRuntimeOptions{
					Logger:  logger,
					Metrics: metrics,
					Recall:  recall,
				},
			)
		}
	}

	return &DelegateTool{
		chatClient:       chatClient,
		allTools:         allTools,
		memory:           memory,
		llmConfig:        llmConfig,
		delegationConfig: delegationConfig,
		currentDepth:     currentDepth,
		metrics:          metrics,
		logger:           logger,
		recall:           recall,
		runtimeFactory:   runtimeFactory,
	}
}

func (a *DelegateTool) Name() string {
	return "delegate_agent"
}

func (a *DelegateTool) Description() string {
	keys := []string{}
	for k := range a.delegationConfig.Profiles {
		keys = append(keys, k)
	}

	return fmt.Sprintf("Delegate a subtask to a specialized sub-agent.  Available profiles: %s. Use this when a task requires a different expertise or focus area.", strings.Join(keys, ", "))
}

func (a *DelegateTool) ParameterSchema() string {
	return `
	{
          "type": "object",
          "properties": {
            "profile": {
              "type": "string",
              "description": "Name of the agent profile to delegate to"
            },
            "task": {
              "type": "string",
              "description": "The task description for the sub-agent to complete"
            }
          },
          "required": ["profile", "task"]
	}
    `
}

func (a *DelegateTool) Execute(ctx context.Context, argumentsJson string) string {
	return a.executeCore(ctx, argumentsJson, nil)
}

func (a *DelegateTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) string {
	return a.executeCore(ctx, argumentsJson, &toolContext)
}

func (d *DelegateTool) executeCore(ctx context.Context, argumentsJson string, toolContext *core.ToolExecutionContext) string {
	// Parse JSON arguments
	var args struct {
		Profile string `json:"profile"`
		Task    string `json:"task"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return "Error: Invalid JSON arguments."
	}

	if strings.TrimSpace(args.Profile) == "" {
		return "Error: 'profile' parameter is required."
	}
	if strings.TrimSpace(args.Task) == "" {
		return "Error: 'task' parameter is required."
	}

	// Lookup profile configuration
	profile, exists := d.delegationConfig.Profiles[args.Profile]
	if !exists {
		var availableProfiles []string
		for k := range d.delegationConfig.Profiles {
			availableProfiles = append(availableProfiles, k)
		}
		return fmt.Sprintf("Error: Unknown agent profile '%s'. Available: %s", args.Profile, strings.Join(availableProfiles, ", "))
	}

	// Check max delegation depth
	if d.currentDepth >= d.delegationConfig.MaxDepth {
		return fmt.Sprintf("Error: Maximum delegation depth (%d) reached. Cannot delegate further.", d.delegationConfig.MaxDepth)
	}

	// Log execution
	taskLog := args.Task
	if len(taskLog) > 100 {
		taskLog = taskLog[:100] + "…"
	}
	if d.logger != nil {
		d.logger.Info("Delegating to sub-agent",
			slog.String("profile", args.Profile),
			slog.Int("depth", d.currentDepth+1),
			slog.String("task", taskLog),
		)
	}

	// Build tool subset for the sub-agent
	var toolSubset []core.ITool
	if len(profile.AllowedTools) > 0 {
		allowedMap := make(map[string]bool, len(profile.AllowedTools))
		for _, name := range profile.AllowedTools {
			allowedMap[name] = true
		}
		for _, tool := range d.allTools {
			if allowedMap[tool.Name()] {
				toolSubset = append(toolSubset, tool)
			}
		}
	} else {
		// Default: Exclude self to prevent trivial direct loops
		for _, tool := range d.allTools {
			if tool.Name() != "delegate_agent" {
				toolSubset = append(toolSubset, tool)
			}
		}
	}

	// Create child DelegateTool at depth + 1 if depth permits
	if d.currentDepth+1 < d.delegationConfig.MaxDepth {
		childDelegate := NewDelegateTool(
			d.chatClient, d.allTools, d.memory, d.llmConfig, d.delegationConfig,
			d.currentDepth+1, d.metrics, d.logger, d.recall, d.runtimeFactory,
		)
		toolSubset = append(toolSubset, childDelegate)
	}

	// Copy LLM Provider Config
	subConfig := d.llmConfig // Struct copy (value pass)

	subAgent := d.runtimeFactory(toolSubset, subConfig, profile)
	now := time.Now().UTC()

	// Extract sorted, distinct allowed tool names
	toolNameMap := make(map[string]struct{})
	for _, tool := range toolSubset {
		toolNameMap[strings.ToLower(tool.Name())] = struct{}{}
	}

	allowedTools := make([]string, 0, len(toolNameMap))
	for name := range toolNameMap {
		allowedTools = append(allowedTools, name)
	}
	sort.Strings(allowedTools)

	// Create a persisted child session for the sub-agent
	var parentSessionID, parentChannelID, parentSenderID string
	if toolContext != nil && toolContext.Session != nil {
		parentSessionID = toolContext.Session.Id
		parentChannelID = toolContext.Session.ChannelId
		parentSenderID = toolContext.Session.SenderId
	}

	subSession := &core.Session{
		Id:        fmt.Sprintf("delegate:%s:%s", args.Profile, strings.ReplaceAll(uuid.New().String(), "-", "")),
		ChannelId: "delegation",
		SenderId:  args.Profile,
		Delegation: &core.SessionDelegationMetadata{
			ParentSessionId: parentSessionID,
			ParentChannelId: parentChannelID,
			ParentSenderId:  parentSenderID,
			Profile:         args.Profile,
			RequestedTask:   args.Task,
			AllowedTools:    allowedTools,
			Depth:           d.currentDepth + 1,
			StartedAtUtc:    now,
			Status:          "running",
		},
	}

	var parentSummary *core.SessionDelegationChildSummary
	if toolContext != nil && toolContext.Session != nil {
		parentSummary = UpsertParentDelegationSummary(toolContext.Session, subSession.Id, args.Profile, args.Task, now)
	}

	// Prefix the task with the profile's system context
	fullTask := args.Task
	if strings.TrimSpace(profile.SystemPrompt) != "" {
		fullTask = fmt.Sprintf("[Context: %s]\n\n%s", profile.SystemPrompt, args.Task)
	}

	// Execute sub-agent with context & recover from potential errors
	result, err := subAgent.Run(ctx, subSession, fullTask, nil, nil, "")
	if err != nil {
		errMsg := fmt.Sprintf("Error: Sub-agent '%s' failed: %v", args.Profile, err)
		FinalizeDelegateToolDelegation(subSession, parentSummary, "failed", "", errMsg)
		_ = d.memory.SaveSession(ctx, *subSession)

		if d.logger != nil {
			d.logger.Error("Sub-agent failed",
				slog.String("profile", args.Profile),
				slog.Int("depth", d.currentDepth+1),
				slog.Any("error", err),
			)
		}
		return errMsg
	}

	FinalizeDelegateToolDelegation(subSession, parentSummary, "completed", result, "")
	_ = d.memory.SaveSession(ctx, *subSession)

	if d.logger != nil {
		d.logger.Info("Sub-agent completed",
			slog.String("profile", args.Profile),
			slog.Int("depth", d.currentDepth+1),
			slog.Int("length", len(result)),
		)
	}

	return result
}

type toolUsageKey struct {
	ToolName   string
	Action     string
	Summary    string
	IsMutation bool
}

func UpsertParentDelegationSummary(
	parentSession *core.Session,
	childSessionId,
	profileName,
	task string,
	startedAtUtc time.Time) *core.SessionDelegationChildSummary {
	var existing *core.SessionDelegationChildSummary
	for _, item := range parentSession.DelegatedSessions {
		if item.SessionId == childSessionId {
			existing = &item
			break
		}

	}
	if existing != nil {
		return existing
	}

	created := core.SessionDelegationChildSummary{
		SessionId:    childSessionId,
		Profile:      profileName,
		TaskPreview:  util.Truncate(task, 200),
		StartedAtUtc: startedAtUtc,
		Status:       "running",
	}

	parentSession.DelegatedSessions = append(parentSession.DelegatedSessions, created)
	return &created
}

func FinalizeDelegateToolDelegation(
	subSession *core.Session,
	parentSummary *core.SessionDelegationChildSummary,
	status,
	result,
	errorstr string) {
	var toolUsage = BuildDelegateToolUsage(subSession)
	proposedChanges := []core.SessionDelegationChangeSummary{}
	for _, item := range toolUsage {
		if item.IsMutation {
			proposedChanges = append(proposedChanges, core.SessionDelegationChangeSummary{
				ToolName: item.ToolName,
				Action:   item.Action,
				Summary:  item.Summary,
			})
		}
	}

	if errorstr == "" {
		errorstr = result
	}

	var preview = util.Truncate(errorstr, 240)
	var completedAtUtc = time.Now().UTC()

	if subSession.Delegation != nil {
		subSession.Delegation.Status = status
		subSession.Delegation.CompletedAtUtc = new(completedAtUtc)
		subSession.Delegation.FinalResponsePreview = preview
		subSession.Delegation.ToolUsage = toolUsage
		subSession.Delegation.ProposedChanges = proposedChanges
	}

	if parentSummary != nil {
		parentSummary.Status = status
		parentSummary.CompletedAtUtc = new(completedAtUtc)
		parentSummary.FinalResponsePreview = preview
		parentSummary.ToolUsage = toolUsage
		parentSummary.ProposedChanges = proposedChanges
	}
}

func BuildDelegateToolUsage(session *core.Session) []core.SessionDelegationToolUsage {
	grouped := make(map[toolUsageKey]int)

	// 1. SelectMany & Select: Flatten tool calls and resolve descriptors
	for _, turn := range session.History {
		if turn.ToolCalls == nil {
			continue
		}

		for _, call := range turn.ToolCalls {
			descriptor := core.ToolActionPolicyResolverInstance.Resolve(call.ToolName, call.Arguments)

			summary := descriptor.Summary
			if strings.TrimSpace(summary) == "" {
				summary = fmt.Sprintf("Execute tool '%s'.", call.ToolName)
			}

			isMutation := descriptor.IsMutation || core.ToolActionPolicyResolverInstance.IsMutationCapable(call.ToolName, call.Arguments)

			key := toolUsageKey{
				ToolName:   call.ToolName,
				Action:     descriptor.Action,
				Summary:    summary,
				IsMutation: isMutation,
			}

			// Group and accumulate Count
			grouped[key]++
		}
	}

	// 2. Map grouped results back into a slice
	results := make([]core.SessionDelegationToolUsage, 0, len(grouped))
	for key, count := range grouped {
		results = append(results, core.SessionDelegationToolUsage{
			ToolName:   key.ToolName,
			Action:     key.Action,
			Summary:    key.Summary,
			IsMutation: key.IsMutation,
			Count:      count,
		})
	}

	// 3. OrderByDescending (Count) -> ThenBy (ToolName case-insensitive)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Count != results[j].Count {
			return results[i].Count > results[j].Count // Descending by Count
		}
		// Case-insensitive comparison for ToolName
		return strings.ToLower(results[i].ToolName) < strings.ToLower(results[j].ToolName)
	})

	return results
}
