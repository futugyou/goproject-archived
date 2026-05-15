package graphify

import "context"

type IPipelineStage[TInput, TOutput any] interface {
	Execute(ctx context.Context, input TInput) (*TOutput, error)
}

var AnalyzerCodeExtensions = []string{"cs", "py", "ts", "tsx", "js", "go", "rs", "java", "rb", "cpp", "c", "h", "kt", "scala", "php"}
var AnalyzerStructuralRelations = []string{"imports", "imports_from", "contains", "method"}

type AnalyzerOptions struct {
	TopGodNodesCount         int
	MinSurpriseWeight        float32
	MaxSuggestedQuestions    int
	TopSurprisingConnections int
}

func DefaultAnalyzerOptions() *AnalyzerOptions {
	return &AnalyzerOptions{
		TopGodNodesCount:         10,
		MinSurpriseWeight:        0.5,
		MaxSuggestedQuestions:    10,
		TopSurprisingConnections: 5,
	}
}

type ClusterOptions struct {
	Resolution           float32
	MaxIterations        int
	MinCommunitySize     int
	MaxCommunityFraction float32
	MinSplitSize         int
}

func DefaultClusterOptions() *ClusterOptions {
	return &ClusterOptions{
		Resolution:           1.0,
		MaxIterations:        100,
		MinCommunitySize:     2,
		MaxCommunityFraction: 0.25,
		MinSplitSize:         10,
	}
}
