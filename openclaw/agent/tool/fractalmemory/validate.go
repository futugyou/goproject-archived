package fractalmemory

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
)

type FractalMemoryValidateTool struct {
	provider core.IStructuredMemoryProvider
}

func NewFractalMemoryValidateTool(provider core.IStructuredMemoryProvider) *FractalMemoryValidateTool {
	return &FractalMemoryValidateTool{provider: provider}
}

func (a *FractalMemoryValidateTool) Name() string {
	return "fractal_memory_validate"
}

func (a *FractalMemoryValidateTool) Description() string {
	return "Validate the configured Fractal Memory repository. Read-only."
}

func (a *FractalMemoryValidateTool) ParameterSchema() string {
	return `{"type":"object","properties":{}}`
}

func (a *FractalMemoryValidateTool) Execute(ctx context.Context, argumentsJson string) string {
	response, err := a.provider.Validate(ctx)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	d, err := json.Marshal(response)
	if err != nil {
		return FractalMemoryError(err.Error())
	}

	return string(d)
}
