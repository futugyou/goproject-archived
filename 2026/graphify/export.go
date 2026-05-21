package graphify

import "context"

type IGraphExporter interface {
	GetFormat() string
	Export(ctx context.Context, graph *KnowledgeGraph, outputPath string) error
}
