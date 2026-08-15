package fractalmemory

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type FractalMemorySearchTool struct {
	provider core.IStructuredMemoryProvider
}

func NewFractalMemorySearchTool(provider core.IStructuredMemoryProvider) *FractalMemorySearchTool {
	return &FractalMemorySearchTool{provider: provider}
}

func (a *FractalMemorySearchTool) Name() string {
	return "fractal_memory_search"
}

func (a *FractalMemorySearchTool) Description() string {
	return "Search optional Fractal Memory structured project memory. Read-only."
}

func (a *FractalMemorySearchTool) ParameterSchema() string {
	return ` {
          "type":"object",
          "properties":{
            "query":{"type":"string","description":"Search query."},
            "limit":{"type":"integer","default":10,"minimum":1,"maximum":50},
            "scope":{"type":"string","description":"Optional Fractal Memory path scope."}
          },
          "required":["query"]
        }
`
}

func (a *FractalMemorySearchTool) Execute(ctx context.Context, argumentsJson string) string {
	var model struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
		Scope string `json:"scope"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return FractalMemoryError(err.Error())
	}

	if model.Query == "" {
		return FractalMemoryError("query is required")
	}

	if model.Limit == 0 {
		model.Limit = 10
	}

	model.Limit = util.Clamp(model.Limit, 1, 50)
	response, err := a.provider.Search(ctx, model.Query, model.Limit, model.Scope)
	if err != nil {
		return FractalMemoryError(err.Error())
	}
	d, err := json.Marshal(response)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	return string(d)
}
