package loopcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/futugyou/openclaw/core"
)

type LoopControlTool struct {
	detector *core.LoopTerminationDetector
}

func New(detector *core.LoopTerminationDetector) *LoopControlTool {
	return &LoopControlTool{detector: detector}
}

func (a *LoopControlTool) Name() string {
	return "loop_control"
}

func (a *LoopControlTool) Description() string {
	return "Control the active /loop recurring task. Use status='complete' when the loop task is fully done. Do NOT use this for ongoing progress — only for final completion."
}

func (a *LoopControlTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["complete"],
			"description": "Set to 'complete' when the loop task is finished."
		}
	},
	"required": ["status"]
}
	`
}

func (a *LoopControlTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	return "", errors.New("Error: loop_control requires session context.")
}

func (a *LoopControlTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) (string, error) {
	if argumentsJson == "" {
		return "", errors.New("Error: arguments payload is empty.")
	}

	var model struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return "", err
	}

	if model.Status == "" {
		return "", errors.New("Error: status is required.")
	}

	if model.Status != "complete" {
		return "", fmt.Errorf("Error: unsupported status '%s'. Only 'complete' is allowed.", model.Status)
	}

	if err := a.detector.OnToolComplete(ctx, toolContext.Session.Id); err != nil {
		return "", err
	}

	return "Loop marked as complete. The recurring task has been stopped.", nil
}
