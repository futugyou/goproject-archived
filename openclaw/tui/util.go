package tui

import (
	"context"
	"fmt"

	"github.com/futugyou/openclaw/core"
)

func FormatGoalFooterLine(ctx context.Context, goalService core.IGoalService, sessionId string) string {
	if goalService == nil {
		return ""
	}
	goal, err := goalService.GetGoal(ctx, sessionId)
	if err != nil {
		return ""
	}

	return goal.FormatGoalFooterLine()
}

func FormatGoalProgressBar(ctx context.Context, goalService core.IGoalService, sessionId string) string {
	if goalService == nil {
		return ""
	}
	goal, err := goalService.GetGoal(ctx, sessionId)
	if err != nil || !goal.Status.IsPursuable() {
		return ""
	}

	return goal.FormatGoalProgressBar()
}

func FormatGoalStatusLine(ctx context.Context, goalService core.IGoalService, sessionId string) string {
	if goalService == nil {
		return ""
	}
	goal, err := goalService.GetGoal(ctx, sessionId)
	if err != nil {
		return ""
	}
	statusText := goal.FormatGoalFooterLine()
	progressBar := goal.FormatGoalProgressBar()
	if progressBar != "" {
		return fmt.Sprintf("%s %s", statusText, progressBar)
	}
	return statusText
}
