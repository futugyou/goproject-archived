package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type GetGoalTool struct {
	goalService core.IGoalService
}

func NewGetGoalTool(goalService core.IGoalService) *GetGoalTool {
	return &GetGoalTool{goalService: goalService}
}

func (a *GetGoalTool) Name() string {
	return "get_goal"
}

func (a *GetGoalTool) Description() string {
	return "Read the current session goal: status, objective, token usage, and budget."
}

func (a *GetGoalTool) ParameterSchema() string {
	return `{"type":"object","properties":{},"required":[]}`
}

func (a *GetGoalTool) Execute(ctx context.Context, argumentsJson string) string {
	return "Error: get_goal requires session context. Use the default parameterless call."
}

func (a *GetGoalTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) string {
	var sessionId = toolContext.Session.Id
	goal, err := a.goalService.GetGoal(ctx, sessionId)
	if err != nil {
		return err.Error()
	}

	var sb = strings.Builder{}

	sb.WriteString(fmt.Sprintf(`
            Status: %s
            Objective: %s
            Tokens Used: %d
            `, goal.Status.ToDisplayName(), goal.Objective, goal.TokensUsed))

	if goal.TokenBudget > 0 {
		fmt.Fprintf(&sb, "\nToken Budget: %d\nRemaining: %d", goal.TokenBudget, goal.RemainingBudget())
	}

	if goal.StatusNote != "" {
		fmt.Fprintf(&sb, "\nNote: %s", goal.StatusNote)
	}

	return sb.String()
}
