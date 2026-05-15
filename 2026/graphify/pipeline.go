package graphify

import "context"

type IPipelineStage[TInput, TOutput any] interface {
	Execute(ctx context.Context, input TInput) (*TOutput, error)
}

var AnalyzerCodeExtensions = []string{"cs", "py", "ts", "tsx", "js", "go", "rs", "java", "rb", "cpp", "c", "h", "kt", "scala", "php"}
var AnalyzerImageExtensions = []string{"png", "jpg", "jpeg", "webp", "gif", "svg"}
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

type FileDetectorOptions struct {
	RootPath          string
	MaxFileSizeBytes  int64
	ExcludePatterns   []string
	IncludeExtensions []string
	RespectGitIgnore  bool
}

func DefaultFileDetectorOptions() *FileDetectorOptions {
	return &FileDetectorOptions{
		MaxFileSizeBytes: 1_048_576,
		RespectGitIgnore: true,
	}
}

type MergeStrategy string

const MergeStrategyHighestConfidence MergeStrategy = "HighestConfidence"
const MergeStrategyMostRecent MergeStrategy = "MostRecent"
const MergeStrategyAggregate MergeStrategy = "Aggregate"

type GraphBuilderOptions struct {
	MergeStrategy   MergeStrategy
	CreateFileNodes bool
	MinEdgeWeight   float32
}

func DefaultGraphBuilderOptions() *GraphBuilderOptions {
	return &GraphBuilderOptions{
		MergeStrategy:   MergeStrategyHighestConfidence,
		CreateFileNodes: true,
		MinEdgeWeight:   0,
	}
}

type SemanticExtractorOptions struct {
	ModelId          string
	MaxTokens        int
	Temperature      float32
	ExtractFromCode  bool
	ExtractFromDocs  bool
	ExtractFromMedia bool
	MaxNodesPerFile  int
	MaxFileSizeBytes int64
}

func DefaultSemanticExtractorOptions() *SemanticExtractorOptions {
	return &SemanticExtractorOptions{
		MaxTokens:        4096,
		Temperature:      0.1,
		ExtractFromCode:  true,
		ExtractFromDocs:  true,
		ExtractFromMedia: true,
		MaxNodesPerFile:  15,
		MaxFileSizeBytes: 1024 * 1024,
	}
}
