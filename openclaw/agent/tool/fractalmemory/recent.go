package fractalmemory

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type FractalMemoryRecentTool struct {
	provider core.IStructuredMemoryProvider
}

func NewFractalMemoryRecentTool(provider core.IStructuredMemoryProvider) *FractalMemoryRecentTool {
	return &FractalMemoryRecentTool{provider: provider}
}

func (a *FractalMemoryRecentTool) Name() string {
	return "fractal_memory_recent"
}

func (a *FractalMemoryRecentTool) Description() string {
	return "List recently changed Fractal Memory nodes. Read-only."
}

func (a *FractalMemoryRecentTool) ParameterSchema() string {
	return ` {
          "type":"object",
          "properties":{
            "days":{"type":"integer","default":30,"minimum":1,"maximum":3650},
            "limit":{"type":"integer","default":10,"minimum":1,"maximum":100},
            "scope":{"type":"string","description":"Optional Fractal Memory path scope."}
          }
        }
`
}

func (a *FractalMemoryRecentTool) Execute(ctx context.Context, argumentsJson string) string {
	var model struct {
		Days  int    `json:"days"`
		Limit int    `json:"limit"`
		Scope string `json:"scope"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return FractalMemoryError(err.Error())
	}

	if model.Days == 0 {
		model.Days = 30
	}
	model.Days = util.Clamp(model.Days, 1, 3650)

	if model.Limit == 0 {
		model.Limit = 10
	}
	model.Limit = util.Clamp(model.Limit, 1, 100)

	response, err := a.provider.Recent(ctx, model.Days, model.Limit, model.Scope)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	d, err := json.Marshal(response)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	return string(d)
}
