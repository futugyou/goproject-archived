package graphify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var _ IGraphExporter = (*LadybugExporter)(nil)

type LadybugExporter struct {
}

// Export implements [IGraphExporter].
func (l *LadybugExporter) Export(ctx context.Context, graph KnowledgeGraph, outputPath string) error {
	var cypher = l.generateLadybugCypher(graph)

	data, err := json.Marshal(cypher)
	if err != nil {
		return err
	}
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

func (l *LadybugExporter) generateLadybugCypher(graph KnowledgeGraph) string {
	var sb strings.Builder
	sb.WriteString("// Ladybug Knowledge Graph Export")
	sb.WriteString("// Generated: {DateTimeOffset.Now:yyyy-MM-dd HH:mm:ss}")
	sb.WriteString("// Nodes: {graph.NodeCount}, Edges: {graph.EdgeCount}")
	sb.WriteByte('\n')

	// DDL — Node table
	sb.WriteString("// Create node table")
	sb.WriteString("CREATE NODE TABLE GraphNode (")
	sb.WriteString("    id STRING PRIMARY KEY,")
	sb.WriteString("    label STRING,")
	sb.WriteString("    nodeType STRING,")
	sb.WriteString("    filePath STRING,")
	sb.WriteString("    relativePath STRING,")
	sb.WriteString("    language STRING,")
	sb.WriteString("    community INT64,")
	sb.WriteString("    confidence STRING,")
	sb.WriteString("    metadata MAP(STRING, STRING)")
	sb.WriteString(");")
	sb.WriteByte('\n')

	// DDL — Relationship table
	sb.WriteString("// Create relationship table")
	sb.WriteString("CREATE REL TABLE GraphEdge (")
	sb.WriteString("    FROM GraphNode TO GraphNode,")
	sb.WriteString("    relationship STRING,")
	sb.WriteString("    metadata MAP(STRING, STRING),")
	sb.WriteString("    weight DOUBLE,")
	sb.WriteString("    confidence STRING,")
	sb.WriteString("    MANY_MANY")
	sb.WriteString(");")
	sb.WriteByte('\n')

	// DML — Nodes
	sb.WriteString("// Create nodes")
	nodeIds := map[string]struct{}{}
	for _, node := range graph.GetNodes() {
		l.appendCreateNode(&sb, node)
		nodeIds[node.Id] = struct{}{}
	}

	sb.WriteByte('\n')

	// DML — Edges (using MATCH to link existing nodes)
	sb.WriteString("// Create edges")

	for _, edge := range graph.GetEdges() {
		_, ok := nodeIds[edge.Source.Id]
		_, ok1 := nodeIds[edge.Target.Id]
		if ok && ok1 {
			l.appendCreateEdge(&sb, edge)
		}
	}

	return sb.String()
}

// GetFormat implements [IGraphExporter].
func (l *LadybugExporter) GetFormat() string {
	return "ladybug"
}

func (l *LadybugExporter) escapeLadybugString(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"'", "\\'",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	)
	return replacer.Replace(value)
}

func (l *LadybugExporter) formatMetadataMap(metadata map[string]string) string {
	var keys strings.Builder
	var values strings.Builder
	first := true
	ForEachSorted(metadata, func(key string, value string) {
		if !first {
			keys.WriteString(", ")
			values.WriteString(", ")
		}

		fmt.Fprintf(&keys, "\"%s\"", l.escapeLadybugString(key))
		fmt.Fprintf(&values, "\"%s\"", l.escapeLadybugString(value))
		first = false
	})
	return fmt.Sprintf("map([%s], [%s])", keys.String(), values.String())
}

func (l *LadybugExporter) appendCreateEdge(sb *strings.Builder, edge GraphEdge) {
	relationship := edge.Relationship
	if len(relationship) == 0 {
		relationship = "RELATED_TO"
	}
	relType := l.escapeLadybugString(relationship)
	confidence := l.escapeLadybugString((string)(edge.Confidence))
	sourceId := l.escapeLadybugString(edge.Source.Id)
	targetId := l.escapeLadybugString(edge.Target.Id)

	fmt.Fprintf(sb, "MATCH (s:GraphNode {{id: \"%s\"}}), (t:GraphNode {{id: \"%s\"}}) ", sourceId, targetId)
	fmt.Fprintf(sb, "CREATE (s)-[:GraphEdge {{relationship: \"%s\", weight: %d, confidence: \"%s\"", relType, edge.Weight, confidence)

	if len(edge.Metadata) > 0 {
		fmt.Fprintf(sb, ", metadata: %s", l.formatMetadataMap(edge.Metadata))
	}

	sb.WriteString("}]->(t);")
}

func (l *LadybugExporter) appendCreateNode(sb *strings.Builder, node GraphNode) {
	fmt.Fprintf(sb, "CREATE (:GraphNode %s\"", l.escapeLadybugString(node.Id))
	fmt.Fprintf(sb, ", label: \"%s\"", l.escapeLadybugString(node.Label))
	fmt.Fprintf(sb, ", nodeType: \"%s\"", l.escapeLadybugString(node.Type))

	if len(node.FilePath) > 0 {
		fmt.Fprintf(sb, ", filePath: \"%s\"", l.escapeLadybugString(node.FilePath))
	}

	if len(node.RelativePath) > 0 {
		fmt.Fprintf(sb, ", relativePath: \"%s\"", l.escapeLadybugString(node.RelativePath))
	}

	if len(node.Language) > 0 {
		fmt.Fprintf(sb, ", language: \"%s\"", l.escapeLadybugString(node.Language))
	}

	if node.Community != -1 {
		fmt.Fprintf(sb, ", community: %d", node.Community)
	}

	fmt.Fprintf(sb, ", confidence: \"%s\"", l.escapeLadybugString(string(node.Confidence)))

	// Metadata as MAP(STRING, STRING)
	if len(node.Metadata) > 0 {
		fmt.Fprintf(sb, ", metadata: %s", l.formatMetadataMap(node.Metadata))
	}

	sb.WriteString("});")
}
