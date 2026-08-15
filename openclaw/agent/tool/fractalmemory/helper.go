package fractalmemory

import (
	"encoding/json"
	"fmt"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

func buildFingerprint(toolName, action, path string) string {
	var payload = fmt.Sprintf("%s|%s|%s", toolName, action, path)
	return util.ComputeTurnHash(payload)
}

func BuildWriteDescriptor(
	toolName,
	action,
	argumentsJson string,
	requireApproval bool) *core.ToolActionDescriptor {
	var path = ""
	var model struct {
		Path string `json:"path"`
	}

	json.Unmarshal([]byte(argumentsJson), &model)

	summary := fmt.Sprintf("%s updates Fractal Memory state.", toolName)
	if model.Path != "" {
		summary = fmt.Sprintf("%s updates Fractal Memory state for '%s'.", toolName, model.Path)
	}
	return &core.ToolActionDescriptor{
		Action:              action,
		IsMutation:          true,
		RequiresApproval:    requireApproval,
		RiskLevel:           "medium",
		Summary:             summary,
		ApprovalFingerprint: buildFingerprint(toolName, action, path),
	}
}

func FractalMemoryError(message string) string {
	response := core.MutationResponse{
		Error: message,
	}

	d, err := json.Marshal(response)
	if err != nil {
		return message
	}

	return string(d)
}
