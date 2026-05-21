package graphify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var _ IGraphExporter = (*Neo4jExporter)(nil)

type Neo4jExporter struct {
}

// Export implements [IGraphExporter].
func (n *Neo4jExporter) Export(ctx context.Context, graph *KnowledgeGraph, outputPath string) error {
	var cypher = n.generateCypher(graph)

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

// GetFormat implements [IGraphExporter].
func (n *Neo4jExporter) GetFormat() string {
	return "neo4j"
}

func (n *Neo4jExporter) generateVariableName(nodeId string) string {
	var sb strings.Builder
	sb.WriteRune('n')

	for _, c := range nodeId {
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('_')
		}
	}

	result := sb.String()
	if len(result) > 50 {
		hash := n.absHash(nodeId)
		hashStr := fmt.Sprintf("%03d", hash%1000)
		result = result[:47] + hashStr
	}

	return result
}

func (n *Neo4jExporter) generateCypher(graph *KnowledgeGraph) string {
	var sb strings.Builder

	// Header comment
	sb.WriteString("// Knowledge Graph Export to Neo4j")
	sb.WriteString("// Generated: {DateTimeOffset.Now:yyyy-MM-dd HH:mm:ss}")
	sb.WriteString("// Nodes: {graph.NodeCount}, Edges: {graph.EdgeCount}")
	sb.WriteByte('\n')
	sb.WriteString("// Clear existing data (optional - uncomment if needed)")
	sb.WriteString("// MATCH (n) DETACH DELETE n;")
	sb.WriteByte('\n')
	sb.WriteString("// Create nodes")
	sb.WriteByte('\n')

	var nodes = graph.GetNodes()
	var nodeIdMap = map[string]string{}

	for _, node := range nodes {
		var varName = n.generateVariableName(node.Id)
		nodeIdMap[node.Id] = varName

		l := node.Label
		if len(l) == 0 {
			l = node.Id
		}
		var label = n.escapeCypher(l)
		var nodeType = n.sanitizeNodeType(node.Type)
		var community = fmt.Sprintf("%d", node.Community)

		fmt.Fprintf(&sb, "CREATE (%s:%s {{", varName, nodeType)
		fmt.Fprintf(&sb, "id: \"%s\", ", n.escapeCypher(node.Id))
		fmt.Fprintf(&sb, "label: \"%s\"", label)

		if node.Community != -1 {
			fmt.Fprintf(&sb, ", community: %s", community)
		}

		// Add metadata properties
		ForEachSorted(node.Metadata, func(key string, value string) {
			if len(value) > 0 && key != "label" {
				var safeKey = n.sanitizePropertyName(key)
				var safeValue = n.escapeCypher(value)
				fmt.Fprintf(&sb, ", %s: \"%s\"", safeKey, safeValue)
			}
		})

		sb.WriteString("});")
	}

	sb.WriteByte('\n')
	sb.WriteString("// Create relationships")
	sb.WriteByte('\n')

	// Generate CREATE statements for edges
	var edgeCount = 0
	var edgesBatch = []string{}

	for _, edge := range graph.GetEdges() {
		sourceVar, ok := nodeIdMap[edge.Source.Id]
		targetVar, ok2 := nodeIdMap[edge.Target.Id]
		if !ok || !ok2 {
			continue
		}
		relationship := edge.Relationship
		if len(relationship) == 0 {
			relationship = "RELATED_TO"
		}
		var relationshipType = n.sanitizeRelationshipType(relationship)
		var weight = edge.Weight
		var confidence = strings.ToUpper(string(edge.Confidence))

		var edgeStmt = fmt.Sprintf("CREATE (%s)-[:%s {{weight: %d, confidence: \"%s\"}}]->(%s);", sourceVar, relationshipType, weight, confidence, targetVar)
		edgesBatch = append(edgesBatch, edgeStmt)
		edgeCount++

		// Write in batches of 100 for better readability
		if edgeCount%100 == 0 {
			for i := 0; i < len(edgesBatch); i++ {
				sb.WriteString(edgesBatch[i])
			}
			sb.WriteByte('\n')
			edgesBatch = []string{}
		}
	}

	// Write remaining edges
	for i := 0; i < len(edgesBatch); i++ {
		sb.WriteString(edgesBatch[i])
	}

	sb.WriteByte('\n')
	sb.WriteString("// Create indexes for better query performance")
	sb.WriteByte('\n')

	// Get unique node types for index creation
	nodeTypes := map[string]struct{}{}
	for _, no := range nodes {
		t := n.sanitizeNodeType(no.Type)
		nodeTypes[t] = struct{}{}
	}
	ForEachSorted(nodeTypes, func(key string, value struct{}) {
		fmt.Fprintf(&sb, "CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.id);", nodeTypes)
		fmt.Fprintf(&sb, "CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.label);", nodeTypes)
	})

	for _, no := range nodes {
		if no.Community != -1 {
			sb.WriteByte('\n')
			sb.WriteString("// Index for community-based queries")
			ForEachSorted(nodeTypes, func(key string, value struct{}) {
				fmt.Fprintf(&sb, "CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.community);", nodeTypes)
			})
			break
		}
	}

	sb.WriteByte('\n')
	sb.WriteString("// Query examples:")
	sb.WriteString("// - Find all nodes: MATCH (n) RETURN n LIMIT 25;")
	sb.WriteString("// - Find nodes by type: MATCH (n:Class) RETURN n LIMIT 25;")
	sb.WriteString("// - Find nodes in a community: MATCH (n) WHERE n.community = 1 RETURN n;")
	sb.WriteString("// - Find highly connected nodes: MATCH (n) RETURN n, size((n)--()) as degree ORDER BY degree DESC LIMIT 10;")
	sb.WriteString("// - Find paths: MATCH p=shortestPath((a)-[*]-(b)) WHERE a.id='Node1' AND b.id='Node2' RETURN p;")

	return sb.String()
}

func (n *Neo4jExporter) sanitizeNodeType(nodeType string) string {
	if strings.TrimSpace(nodeType) == "" {
		return "Node"
	}

	var sb strings.Builder
	firstChar := true

	for _, c := range nodeType {
		if unicode.IsLetter(c) {
			sb.WriteRune(c)
			firstChar = false
		} else if !firstChar && unicode.IsDigit(c) {
			sb.WriteRune(c)
		} else if !firstChar && c == '_' {
			sb.WriteRune(c)
		} else if !firstChar {
			sb.WriteRune('_')
		}
	}

	result := sb.String()
	if result == "" {
		return "Node"
	}
	return result
}

func (n *Neo4jExporter) sanitizeRelationshipType(rel string) string {
	if strings.TrimSpace(rel) == "" {
		return "RELATED_TO"
	}

	var sb strings.Builder
	for _, c := range rel {
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			sb.WriteRune(unicode.ToUpper(c))
		} else {
			sb.WriteRune('_')
		}
	}

	result := sb.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")

	if result == "" {
		return "RELATED_TO"
	}
	return result
}

func (n *Neo4jExporter) sanitizePropertyName(prop string) string {
	if strings.TrimSpace(prop) == "" {
		return "property"
	}

	var sb strings.Builder
	firstChar := true

	for _, c := range prop {
		if unicode.IsLetter(c) {
			if firstChar {
				sb.WriteRune(unicode.ToLower(c))
				firstChar = false
			} else {
				sb.WriteRune(c)
			}
		} else if !firstChar && unicode.IsDigit(c) {
			sb.WriteRune(c)
		} else if !firstChar && c == '_' {
			sb.WriteRune(c)
		} else if !firstChar {
			sb.WriteRune('_')
		}
	}

	result := sb.String()
	if result == "" {
		return "property"
	}
	return result
}

func (n *Neo4jExporter) absHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = 31*hash + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

func (n *Neo4jExporter) escapeCypher(value string) string {
	if len(value) == 0 {
		return ""
	}
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
