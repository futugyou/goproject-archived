package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type UpdateGoalTool struct {
	goalService core.IGoalService
}

func NewUpdateGoalTool(goalService core.IGoalService) *UpdateGoalTool {
	return &UpdateGoalTool{goalService: goalService}
}

func (a *UpdateGoalTool) Name() string {
	return "update_goal	"
}

func (a *UpdateGoalTool) Description() string {
	return "Update the goal status. Only 'complete' (goal achieved) or 'blocked' (genuinely stuck) are allowed. Cannot pause, resume, or clear the goal."
}

func (a *UpdateGoalTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["complete", "blocked"],
			"description": "New status: 'complete' when fully achieved, 'blocked' when genuinely stuck after 3+ attempts."
		},
		"note": {
			"type": "string",
			"description": "Optional note explaining the status change."
		}
	},
	"required": ["status"]
}`
}

func (a *UpdateGoalTool) Execute(ctx context.Context, argumentsJson string) string {
	return "Error: update_goal requires session context."
}

type UpdateGoal struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func tryVerifyCompletion(toolContext core.ToolExecutionContext) bool {
	if toolContext.TurnContext.ToolCallCount() > 0 {
		return false
	}

	var latestAssistantTurn *core.ChatTurn
	for i := len(toolContext.Session.History) - 1; i >= 0; i-- {
		if toolContext.Session.History[i].Role == "assistant" {
			latestAssistantTurn = &toolContext.Session.History[i]
			break
		}
	}

	if latestAssistantTurn == nil {
		return false
	}

	var content = strings.TrimSpace(latestAssistantTurn.Content)
	if content == "" {
		return false
	}

	return !strings.EqualFold(content, "[tool_use]")
}

func (a *UpdateGoalTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	if toolContext.Session == nil {
		return "Error: ToolExecutionContext Session is empty."
	}

	var data UpdateGoal
	if err := json.Unmarshal([]byte(argumentsJson), &data); err != nil {
		return "Error: arguments must be valid JSON."
	}

	if data.Status == "" {
		return "Error: status is required."
	}

	goal, err := a.goalService.GetGoal(ctx, toolContext.Session.Id)
	if err != nil || goal == nil {
		return "Error: No active goal for this session."
	}

	if !goal.Status.IsPursuable() {
		return fmt.Sprintf("Error: Goal is not active (current status: %s).", goal.Status.ToDisplayName())
	}

	switch data.Status {
	case "complete":
		if !tryVerifyCompletion(toolContext) {
			return "Warning: Cannot verify completion. The goal may not be fully achieved yet. " +
				"Please continue working toward the objective and verify all requirements before declaring completion."
		}
		if err := a.goalService.UpdateStatus(ctx, toolContext.Session.Id, core.GoalStatus_Complete, data.Note); err != nil {
			return err.Error()
		}
		return "Goal marked as complete. Well done!"
	case "blocked":
		if err := a.goalService.UpdateStatus(ctx, toolContext.Session.Id, core.GoalStatus_Blocked, data.Note); err != nil {
			return err.Error()
		}
		return "Goal marked as blocked. The user can resume it with /goal resume."
	default:
		return fmt.Sprintf("Error: Invalid status '%s'. Use 'complete' or 'blocked'.", data.Status)
	}
}
