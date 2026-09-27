package capability

import "context"

type governedStepTool struct {
	run func(ctx context.Context) string
}

func (g *governedStepTool) Name() string { return "resolve_capability" }
func (g *governedStepTool) Description() string {
	return "Resolve an authorized capability workflow step"
}
func (g *governedStepTool) ParameterSchema() string { return "{\"type\":\"object\"}" }
func (g *governedStepTool) Execute(ctx context.Context, argumentsJson string) string {
	return g.run(ctx)
}
