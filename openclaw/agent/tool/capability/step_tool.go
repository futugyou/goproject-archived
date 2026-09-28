package capability

import "context"

type GovernedStepTool struct {
	run func(ctx context.Context) (string, error)
}

func NewGovernedStepTool(run func(ctx context.Context) (string, error)) *GovernedStepTool {
	return &GovernedStepTool{run: run}
}

func (g *GovernedStepTool) Name() string { return "resolve_capability" }

func (g *GovernedStepTool) Description() string {
	return "Resolve an authorized capability workflow step"
}

func (g *GovernedStepTool) ParameterSchema() string { return "{\"type\":\"object\"}" }

func (g *GovernedStepTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	return g.run(ctx)
}
