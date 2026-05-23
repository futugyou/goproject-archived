package graphify

import (
	"context"
	"fmt"
	"maps"
	"strings"
)

var _ IPipelineStage[GraphExtractionInput, KnowledgeGraph] = (*GraphBuilder)(nil)

type GraphBuilder struct {
	options *GraphBuilderOptions
}

func NewGraphBuilder(options *GraphBuilderOptions) *GraphBuilder {
	if options == nil {
		options = DefaultGraphBuilderOptions()
	}

	return &GraphBuilder{
		options: options,
	}
}

// Execute implements [IPipelineStage].
func (g *GraphBuilder) Execute(ctx context.Context, input *GraphExtractionInput) (*KnowledgeGraph, error) {
	graph := &KnowledgeGraph{}
	nodeMetadataAggregator := map[string][]ExtractedNode{}
	edgeWeightTracker := map[edgeKey]edgeData{}
	fileNodes := []string{}
	// Map absolute file paths to relative paths for portability
	relativePathMap := map[string]string{}

	// Phase 1: Collect all nodes and track duplicates for merging
	for _, extraction := range input.Datas {

		// Track file for file-level node creation
		if g.options.CreateFileNodes && len(extraction.SourceFilePath) > 0 {
			fileNodes = append(fileNodes, extraction.SourceFilePath)
		}

		// Build mapping of absolute to relative paths
		if len(extraction.SourceFilePath) > 0 && len(extraction.RelativeSourceFilePath) > 0 {
			relativePathMap[extraction.SourceFilePath] = extraction.RelativeSourceFilePath
		}

		for _, node := range extraction.Nodes {
			if _, ok := nodeMetadataAggregator[node.Id]; !ok {
				nodeMetadataAggregator[node.Id] = []ExtractedNode{}
			}

			nodeMetadataAggregator[node.Id] = append(nodeMetadataAggregator[node.Id], node)
		}
	}

	// Phase 2: Merge nodes according to strategy
	for nodeId, duplicates := range nodeMetadataAggregator {

		var mergedNode = g.mergeNodes(nodeId, duplicates, relativePathMap)
		graph.AddNode(*mergedNode)
	}

	// Phase 3: Create file-level nodes if enabled
	if g.options.CreateFileNodes {
		for _, filePath := range fileNodes {

			var fileNodeId = "file:" + filePath
			_, err := graph.GetNodesById(fileNodeId)
			if err == nil {
				// Resolve relative path from mapping
				relativePath := relativePathMap[filePath]
				// Normalize path separators to forward slashes for cross-platform compatibility
				if len(relativePath) > 0 {
					relativePath = strings.ReplaceAll(relativePath, "\\", "/")
				}

				var fileNode = GraphNode{
					Id:           fileNodeId,
					Label:        GetFileName(filePath),
					Type:         "File",
					FilePath:     filePath,
					RelativePath: relativePath,
					Confidence:   ConfidenceExtracted,
					Metadata:     map[string]string{"full_path": filePath},
				}
				graph.AddNode(fileNode)
			}
		}
	}

	// Phase 4: Collect and merge edges
	for _, extraction := range input.Datas {
		for _, edge := range extraction.Edges {
			// Skip edges to nodes that don't exist (external/stdlib imports)
			_, err1 := graph.GetNodesById(edge.Source)
			_, err2 := graph.GetNodesById(edge.Target)
			if err1 != nil || err2 != nil {
				continue
			}

			key := edgeKey{
				Source:       edge.Source,
				Target:       edge.Target,
				Relationship: edge.Relation,
			}

			if _, ok := edgeWeightTracker[key]; !ok {
				edgeWeightTracker[key] = edgeData{
					Weight:         edge.Weight,
					Confidence:     edge.Confidence,
					SourceFile:     edge.SourceFile,
					SourceLocation: edge.SourceLocation,
					Count:          1,
				}
			} else {
				// Merge edges: increment weight and keep highest confidence
				var existing = edgeWeightTracker[key]
				existing.Weight += edge.Weight
				existing.Count++
				if edge.Confidence < existing.Confidence {
					existing.Confidence = edge.Confidence
				}
			}
		}

		// Create "contains" edges from file nodes to entities in that file
		if g.options.CreateFileNodes && len(extraction.SourceFilePath) > 0 {
			var fileNodeId = "file:" + extraction.SourceFilePath
			_, err := graph.GetNodesById(fileNodeId)

			if err == nil {
				for _, node := range extraction.Nodes {
					var entityNode, err = graph.GetNodesById(node.Id)
					if err == nil && entityNode.Id != fileNodeId {
						var containsKey = edgeKey{
							Source:       fileNodeId,
							Target:       node.Id,
							Relationship: "contains",
						}
						if _, ok := edgeWeightTracker[containsKey]; !ok {
							edgeWeightTracker[containsKey] = edgeData{
								Weight:     1.0,
								Confidence: ConfidenceExtracted,
								SourceFile: extraction.SourceFilePath,
								Count:      1,
							}
						}
					}
				}
			}
		}
	}

	// Phase 5: Add edges to graph (with weight filtering)
	for key, data := range edgeWeightTracker {
		if data.Weight < g.options.MinEdgeWeight {
			continue
		}

		var sourceNode, err1 = graph.GetNodesById(key.Source)
		var targetNode, err2 = graph.GetNodesById(key.Target)

		if err1 == nil && err2 == nil {
			var metadata = map[string]string{"merge_count": fmt.Sprintf("%d", data.Count)}
			if len(data.SourceFile) > 0 {
				metadata["source_file"] = data.SourceFile
			}

			if len(data.SourceLocation) > 0 {
				metadata["source_location"] = data.SourceLocation
			}

			var graphEdge = GraphEdge{
				Source:       sourceNode,
				Target:       targetNode,
				Relationship: key.Relationship,
				Weight:       int(data.Weight),
				Confidence:   data.Confidence,
				Metadata:     metadata,
			}

			graph.AddEdge(graphEdge)
		}
	}

	return graph, nil
}

type edgeData struct {
	Weight         float32
	Confidence     Confidence
	SourceFile     string
	SourceLocation string
	Count          int
}

type edgeKey struct {
	Source       string
	Target       string
	Relationship string
}

func (g *GraphBuilder) mergeNodes(nodeId string, duplicates []ExtractedNode, relativePathMap map[string]string) *GraphNode {
	var selected ExtractedNode = duplicates[len(duplicates)-1]

	// Build metadata dictionary
	metadata := map[string]string{}

	if g.options.MergeStrategy == MergeStrategyAggregate {
		// Merge metadata from all duplicates
		for _, dup := range duplicates {
			maps.Copy(metadata, dup.Metadata)
		}
	} else {
		maps.Copy(metadata, selected.Metadata)
	}

	// Add source location if present
	if len(selected.SourceLocation) > 0 {
		metadata["source_location"] = selected.SourceLocation
	}

	// Add merge count if there were duplicates
	if len(duplicates) > 1 {
		metadata["merge_count"] = fmt.Sprintf("%d", len(duplicates))
	}

	// Determine type from FileType enum
	graphNodeType := "Unknown"
	switch selected.FileType {
	case FileTypeCode:
		graphNodeType = "Entity"
	case FileTypeDocument:
		graphNodeType = "Document"
	case FileTypePaper:
		graphNodeType = "Paper"
	case FileTypeImage:
		graphNodeType = "Image"
	}

	// Override type if metadata has more specific information
	if s, ok := metadata["type"]; ok {
		graphNodeType = s
	}

	// Resolve relative path from mapping
	relativePath := relativePathMap[selected.SourceFile]
	// Normalize path separators to forward slashes for cross-platform compatibility
	if len(relativePath) > 0 {
		relativePath = strings.ReplaceAll(relativePath, "\\", "/")
	}

	return &GraphNode{
		Id:           nodeId,
		Label:        selected.Label,
		Type:         graphNodeType,
		FilePath:     selected.SourceFile,
		RelativePath: relativePath,
		Confidence:   ConfidenceExtracted, // Default to Extracted (AST-based)
		Metadata:     metadata,
	}
}
