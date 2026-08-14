package creategoal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type CreateGoalTool struct {
	goalService core.IGoalService
}

func New(goalService core.IGoalService) *CreateGoalTool {
	return &CreateGoalTool{goalService: goalService}
}

func (a *CreateGoalTool) Name() string {
	return "create_goal	"
}

func (a *CreateGoalTool) Description() string {
	return "Create a new session goal with an objective and optional token budget. Fails if a goal already exists."
}

func (a *CreateGoalTool) ParameterSchema() string {
	return `
	{"type":"object","properties":{"objective":{"type":"string","description":"The goal objective — what to achieve."},"token_budget":{"type":"integer","description":"Optional token budget (e.g., 500000 for 500k). 0 or omitted means unlimited."}},"required":["objective"]}
    `
}

func (a *CreateGoalTool) Execute(ctx context.Context, argumentsJson string) string {
	return "Error: create_goal requires session context"
}

type Token struct {
	Objective   *string `json:"objective,omitempty"`
	TokenBudget int64   `json:"token_budget,omitempty"`
}

func (a *CreateGoalTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	if toolContext.Session == nil {
		return "Error: ToolExecutionContext Session is empty."
	}

	var data Token
	if err := json.Unmarshal([]byte(argumentsJson), &data); err != nil {
		return "Error: arguments must be valid JSON."
	}

	if data.Objective == nil || strings.TrimSpace(*data.Objective) == "" {
		return "Error: objective is required."
	}

	if data.TokenBudget < 0 {
		return "Error: token_budget cannot be negative."
	}

	goal, err := a.goalService.CreateGoal(ctx, toolContext.Session.Id, *data.Objective, data.TokenBudget, toolContext.Session.GetTotalTokens())
	if err != nil {
		return err.Error()
	}

	return fmt.Sprintf("goal created. Status: %s. Objective: %s", goal.Status.ToDisplayName(), goal.Objective)
}
