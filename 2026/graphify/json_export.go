package graphify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var _ IGraphExporter = (*JsonExporter)(nil)

type JsonExporter struct {
}

// Export implements [IGraphExporter].
func (j *JsonExporter) Export(ctx context.Context, graph KnowledgeGraph, outputPath string) error {
	nodes := []NodeDto{}
	communities := map[int]struct{}{}
	for _, node := range graph.GetNodes() {
		n := NodeDto{
			Id:         node.Id,
			Label:      node.Label,
			Type:       node.Type,
			Community:  node.Community,
			FilePath:   node.RelativePath,
			Confidence: strings.ToUpper(string(node.Confidence)),
			Metadata:   node.Metadata,
		}
		if len(n.FilePath) == 0 {
			n.FilePath = node.FilePath
		}
		if _, ok := communities[node.Community]; !ok {
			communities[node.Community] = struct{}{}
		}
		nodes = append(nodes, n)
	}

	edges := []EdgeDto{}
	for _, edge := range graph.GetEdges() {
		e := EdgeDto{
			Source:       edge.Source.Id,
			Target:       edge.Target.Id,
			Relationship: edge.Relationship,
			Weight:       float64(edge.Weight),
			Confidence:   strings.ToUpper(string(edge.Confidence)),
			Metadata:     edge.Metadata,
		}
		if e.Weight == 0 {
			e.Weight = 1
		}
		edges = append(edges, e)
	}

	exportData := GraphExportDto{
		Nodes: nodes,
		Edges: edges,
		Metadata: ExportMetadataDto{
			NodeCount:      len(nodes),
			EdgeCount:      len(edges),
			CommunityCount: len(communities),
			GeneratedAt:    time.Now(),
		},
	}

	data, err := json.Marshal(exportData)
	if err != nil {
		return err
	}
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

// GetFormat implements [IGraphExporter].
func (j *JsonExporter) GetFormat() string {
	return "json"
}
