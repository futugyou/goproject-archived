package fractalmemory

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type FractalMemoryOpenTool struct {
	config   core.FractalMemoryConfig
	provider core.IStructuredMemoryProvider
}

func NewFractalMemoryOpenTool(provider core.IStructuredMemoryProvider, config core.FractalMemoryConfig) *FractalMemoryOpenTool {
	return &FractalMemoryOpenTool{provider: provider, config: config}
}

func (a *FractalMemoryOpenTool) Name() string {
	return "fractal_memory_open"
}

func (a *FractalMemoryOpenTool) Description() string {
	return "Open a Fractal Memory node as structured project memory. Read-only."
}

func (a *FractalMemoryOpenTool) ParameterSchema() string {
	return `{
          "type":"object",
          "properties":{
            "path":{"type":"string","description":"Fractal Memory node path."},
            "depth":{"type":"integer","default":1,"minimum":0,"maximum":3},
            "view":{"type":"string","enum":["index","state","timeline","decisions","children"],"default":"index"}
          },
          "required":["path"]
        }
`
}

func (a *FractalMemoryOpenTool) Execute(ctx context.Context, argumentsJson string) string {
	var model struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
		View  string `json:"view"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return FractalMemoryError(err.Error())
	}

	if model.Path == "" {
		return FractalMemoryError("path is required")
	}

	if model.Depth == 0 {
		model.Depth = a.config.DefaultDepth
	}

	model.Depth = util.Clamp(model.Depth, 0, 3)
	if model.View == "" {
		model.View = a.config.DefaultView
	}

	response, err := a.provider.Open(ctx, model.Path, model.Depth, model.View)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	d, err := json.Marshal(response)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	return string(d)
}
