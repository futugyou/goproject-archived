package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

var GoalPromptTemplatesInstance = &GoalPromptTemplates{}

type GoalPromptTemplates struct{}

func (g *GoalPromptTemplates) BuildActivationPrompt(goal *core.SessionGoal) string {
	if goal == nil {
		return ""
	}
	return fmt.Sprintf(`
            **Active Goal**
            A session-scoped goal is now active with the following objective:
            <objective>%s</objective>

            **Your Behavior**
            - Treat the objective itself as your directive. Do NOT pause to ask the user what to do.
            - The system will automatically continue you if you stop before the goal is achieved.
            - When the goal is fully achieved, use the update_goal tool with status='complete'.
            - If you're genuinely blocked after repeated attempts, use update_goal with status='blocked'.

            **Completion Audit**
            Before declaring the goal complete, derive concrete requirements from the objective.
            For each requirement, identify authoritative evidence. Uncertain evidence means NOT achieved.
            `, goal.Objective)
}

func (g *GoalPromptTemplates) BuildCheckPrompt(goal *core.SessionGoal, iteration, maxIterations int) string {
	if goal == nil {
		return ""
	}
	var budgetLine = "**Budget**: No limit set."
	if goal.TokenBudget > 0 {
		budgetLine = fmt.Sprintf("**Budget**: Used %d / Budget %d / Remaining %d", goal.TokensUsed, goal.TokenBudget, goal.RemainingBudget())
	}

	return fmt.Sprintf(`
            **Goal Check — Continue Working**
            You were working toward this objective: <objective>{goal.Objective}</objective>

            1. REVIEW all work done so far
            2. DETERMINE whether the objective has been FULLY achieved
            3. If ACHIEVED → use update_goal tool with status='complete'
            4. If NOT ACHIEVED → CONTINUE working without asking the user

            %s
            **Fidelity**: Optimize for movement toward the requested end state. Do NOT substitute easier solutions.
            **Blocked Audit**: Only mark blocked after 3+ consecutive turns with the same blocker.
            Iteration: %d/%d
            `, budgetLine, iteration, maxIterations)
}

func (g *GoalPromptTemplates) FormatGoalFooterLine(goal *core.SessionGoal) string {
	return goal.FormatGoalFooterLine()
}

func (g *GoalPromptTemplates) FormatProgressBar(goal *core.SessionGoal) string {
	return goal.FormatGoalProgressBar()
}

var InteractiveChannelIds = map[string]struct{}{
	"cli": {}, "tui": {}, "terminal": {}, "console": {}, "companion": {},
	"websocket": {},
	"discord":   {}, "slack": {}, "teams": {}, "telegram": {}, "signal": {}, "whatsapp": {}, "sms": {},
}

type AgentRuntimeGoalIntegration struct {
	goalService core.IGoalService
	logger      *slog.Logger
}

func NewAgentRuntimeGoalIntegration(goalService core.IGoalService, logger *slog.Logger) *AgentRuntimeGoalIntegration {
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentRuntimeGoalIntegration{
		goalService: goalService,
		logger:      logger,
	}
}

func (a *AgentRuntimeGoalIntegration) BuildGoalSystemPrompt(sessionId string) string {
	goal, err := a.goalService.GetGoal(context.Background(), sessionId)
	if err != nil || !goal.Status.IsPursuable() {
		return ""
	}

	return GoalPromptTemplatesInstance.BuildActivationPrompt(goal)
}

func (a *AgentRuntimeGoalIntegration) UpdateGoalTokenUsage(session *core.Session) error {
	return a.goalService.UpdateTokenUsage(context.Background(), session.Id, session.GetTotalTokens())
}

func (a *AgentRuntimeGoalIntegration) isInteractiveChannel(channelId string) bool {
	if channelId == "" {
		return true
	}

	_, ok := InteractiveChannelIds[channelId]
	return ok
}

func (a *AgentRuntimeGoalIntegration) EvaluateGoalContinuation(session *core.Session, iteration, maxIterations int, modelResponseText string) string {
	if session == nil {
		return ""
	}
	goal, err := a.goalService.GetGoal(context.Background(), session.Id)
	if err != nil || !goal.Status.IsPursuable() {
		return ""
	}

	if !a.isInteractiveChannel(session.ChannelId) {
		a.logger.Debug("Goal auto-continuation skipped for non-interactive channel", "channel_id", session.ChannelId)
		return ""
	}

	// Check budget
	if err := a.goalService.UpdateTokenUsage(context.Background(), session.Id, session.GetTotalTokens()); err != nil {
		return ""
	}
	goal, err = a.goalService.GetGoal(context.Background(), session.Id)
	if err != nil || !goal.Status.IsPursuable() {
		return ""
	}
	if goal.IsBudgetExceeded() {
		if err := a.goalService.UpdateStatus(context.Background(), session.Id, core.GoalStatus_BudgetLimited, "Token budget exceeded"); err != nil {
			return ""
		}
		a.logger.Info("Goal budget exceeded",
			"session_id", session.Id,
			"used", goal.TokensUsed,
			"budget", goal.TokenBudget,
		)
		return ""
	}

	// Check per-turn continuation limit
	var continuationCount = a.goalService.IncrementContinuationCount(context.Background(), session.Id)
	if continuationCount == 0 {
		return ""
	}
	if continuationCount > 10 {
		if err := a.goalService.UpdateStatus(context.Background(), session.Id, core.GoalStatus_Paused, "Auto-paused after 10 continuations"); err != nil {
			return ""
		}
		a.logger.Info("Goal auto-paused: exceeded continuation limit",
			"session_id", session.Id,
		)
		return ""
	}

	// Record turn hash for blocker detection
	var normalized = util.NormalizeForComparison(modelResponseText)
	var isBlocked = a.goalService.RecordTurnHash(context.Background(), session.Id, normalized)
	if isBlocked {
		if err := a.goalService.UpdateStatus(context.Background(), session.Id, core.GoalStatus_Blocked, "Same blocker repeated 3+ consecutive turns"); err != nil {
			return ""
		}
		a.logger.Info("Goal blocked after 3+ same-blocker turns", "session_id", session.Id)
		return ""
	}

	// Check iteration limit
	if iteration >= maxIterations {
		a.logger.Info("Goal reached max iterations",
			"session_id", session.Id,
			"max", maxIterations,
		)
		return ""
	}

	// Build continuation prompt
	a.logger.Info("Goal auto-continuing",
		"session_id", session.Id,
		"iter", iteration,
		"max", maxIterations,
		"cont", continuationCount,
	)

	return GoalPromptTemplatesInstance.BuildCheckPrompt(goal, iteration, maxIterations)
}
