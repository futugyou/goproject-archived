package goal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type CreateGoalTool struct {
	goalService core.IGoalService
}

func NewCreateGoalTool(goalService core.IGoalService) *CreateGoalTool {
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
	{
	"type": "object",
	"properties": {
		"objective": {
			"type": "string",
			"description": "The goal objective — what to achieve."
		},
		"token_budget": {
			"type": "integer",
			"description": "Optional token budget (e.g., 500000 for 500k). 0 or omitted means unlimited."
		}
	},
	"required": ["objective"]
}`
}

func (a *CreateGoalTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	return "", errors.New("Error: create_goal requires session context")
}

type CreateGoal struct {
	Objective   *string `json:"objective,omitempty"`
	TokenBudget int64   `json:"token_budget,omitempty"`
}

func (a *CreateGoalTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) (string, error) {
	if argumentsJson == "" {
		return "", errors.New("Error: arguments payload is empty.")
	}

	if toolContext.Session == nil {
		return "", errors.New("Error: ToolExecutionContext Session is empty.")
	}

	var data CreateGoal
	if err := json.Unmarshal([]byte(argumentsJson), &data); err != nil {
		return "", errors.New("Error: arguments must be valid JSON.")
	}

	if data.Objective == nil || strings.TrimSpace(*data.Objective) == "" {
		return "", errors.New("Error: objective is required.")
	}

	if data.TokenBudget < 0 {
		return "", errors.New("Error: token_budget cannot be negative.")
	}

	goal, err := a.goalService.CreateGoal(ctx, toolContext.Session.Id, *data.Objective, data.TokenBudget, toolContext.Session.GetTotalTokens())
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("goal created. Status: %s. Objective: %s", goal.Status.ToDisplayName(), goal.Objective), nil
}
