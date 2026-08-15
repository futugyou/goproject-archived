package fractalmemory

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
)

type FractalMemoryExportTool struct {
	provider core.IStructuredMemoryProvider
	config   core.FractalMemoryConfig
}

func NewFractalMemoryExportTool(provider core.IStructuredMemoryProvider, config core.FractalMemoryConfig) *FractalMemoryExportTool {
	return &FractalMemoryExportTool{provider: provider, config: config}
}

func (a *FractalMemoryExportTool) Name() string {
	return "fractal_memory_export"
}

func (a *FractalMemoryExportTool) Description() string {
	return "Export compact Fractal Memory context for a node. Read-only."
}

func (a *FractalMemoryExportTool) ParameterSchema() string {
	return `{
          "type":"object",
          "properties":{
            "path":{"type":"string","description":"Fractal Memory node path."},
            "mode":{"type":"string","enum":["compact","standard","verbose"],"default":"compact"}
          },
          "required":["path"]
        }
`
}

func (a *FractalMemoryExportTool) Execute(ctx context.Context, argumentsJson string) string {
	var model struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return FractalMemoryError(err.Error())
	}

	if model.Path == "" {
		return FractalMemoryError("path is required")
	}

	if model.Mode == "" {
		model.Mode = a.config.DefaultExportMode
	}

	response, err := a.provider.Export(ctx, model.Path, model.Mode)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	d, err := json.Marshal(response)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	return string(d)
}
