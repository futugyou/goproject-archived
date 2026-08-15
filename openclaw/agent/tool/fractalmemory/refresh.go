package fractalmemory

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
)

type FractalMemoryIndexRefreshTool struct {
	provider core.IStructuredMemoryProvider
	config   core.FractalMemoryConfig
}

func NewFractalMemoryIndexRefreshTool(provider core.IStructuredMemoryProvider, config core.FractalMemoryConfig) *FractalMemoryIndexRefreshTool {
	return &FractalMemoryIndexRefreshTool{provider: provider, config: config}
}

func (a *FractalMemoryIndexRefreshTool) Name() string {
	return "fractal_memory_index_refresh"
}

func (a *FractalMemoryIndexRefreshTool) Description() string {
	return "Refresh Fractal Memory indexes. Write/update-capable and disabled unless Fractal writes are explicitly enabled."
}

func (a *FractalMemoryIndexRefreshTool) ParameterSchema() string {
	return `{"type":"object","properties":{}}`
}

func (a *FractalMemoryIndexRefreshTool) Execute(ctx context.Context, argumentsJson string) string {
	response, err := a.provider.RefreshIndex(ctx)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	d, err := json.Marshal(response)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	return string(d)
}

func (a *FractalMemoryIndexRefreshTool) ResolveActionDescriptor(argumentsJson string) (*core.ToolActionDescriptor, error) {
	return BuildWriteDescriptor(a.Name(), "refresh_index", argumentsJson, a.config.RequireApprovalForWrites), nil
}
