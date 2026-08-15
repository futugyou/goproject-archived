package fractalmemory

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
)

type FractalMemoryHandoffCreateTool struct {
	provider core.IStructuredMemoryProvider
	config   core.FractalMemoryConfig
}

func NewFractalMemoryHandoffCreateTool(provider core.IStructuredMemoryProvider, config core.FractalMemoryConfig) *FractalMemoryHandoffCreateTool {
	return &FractalMemoryHandoffCreateTool{provider: provider, config: config}
}

func (a *FractalMemoryHandoffCreateTool) Name() string {
	return "fractal_memory_handoff_create"
}

func (a *FractalMemoryHandoffCreateTool) Description() string {
	return "Create a Fractal Memory handoff packet for a node. Write-capable and disabled unless Fractal writes are explicitly enabled."
}

func (a *FractalMemoryHandoffCreateTool) ParameterSchema() string {
	return ` {
          "type":"object",
          "properties":{
            "path":{"type":"string","description":"Fractal Memory node path."}
          },
          "required":["path"]
        }
`
}

func (a *FractalMemoryHandoffCreateTool) Execute(ctx context.Context, argumentsJson string) string {
	var model struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return FractalMemoryError(err.Error())
	}

	if model.Path == "" {
		return FractalMemoryError("path is required")
	}

	response, err := a.provider.CreateHandoff(ctx, model.Path)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	d, err := json.Marshal(response)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	return string(d)
}

func (a *FractalMemoryHandoffCreateTool) ResolveActionDescriptor(argumentsJson string) core.ToolActionDescriptor {
	return BuildWriteDescriptor(a.Name(), "create_handoff", argumentsJson, a.config.RequireApprovalForWrites)
}
