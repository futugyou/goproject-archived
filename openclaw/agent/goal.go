package agent

import (
	"fmt"

	"github.com/futugyou/openclaw/core"
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
