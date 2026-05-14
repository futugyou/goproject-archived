package graphify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
)

var _ IGraphExporter = (*WikiExporter)(nil)

type WikiExporter struct{}

// Export implements [IGraphExporter].
func (e *WikiExporter) Export(ctx context.Context, graph KnowledgeGraph, outputPath string) error {
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("outputPath is empty")
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return err
	}

	communities := make(map[int][]string)
	for _, n := range graph.GetNodes() {
		if n.Community != -1 {
			communities[n.Community] = append(communities[n.Community], n.Id)
		}
	}

	communityLabels := make(map[int]string)
	for id := range communities {
		communityLabels[id] = fmt.Sprintf("Community %d", id)
	}

	cohesionScores := make(map[int]float64)
	for id, nodeIDs := range communities {
		cohesionScores[id] = e.calculateCohesion(graph, nodeIDs)
	}

	godNodes := e.getGodNodes(graph, 10)

	sortedCommunityIDs := make([]int, 0, len(communities))
	for id := range communities {
		sortedCommunityIDs = append(sortedCommunityIDs, id)
	}
	sort.Slice(sortedCommunityIDs, func(i, j int) bool {
		return len(communities[sortedCommunityIDs[i]]) > len(communities[sortedCommunityIDs[j]])
	})

	for _, communityID := range sortedCommunityIDs {
		nodeIDs := communities[communityID]
		label := communityLabels[communityID]
		cohesion := cohesionScores[communityID]

		content := e.generateCommunityArticle(graph, communityID, nodeIDs, label, communityLabels, cohesion)
		filename := e.safeFilename(label) + ".md"

		if err := os.WriteFile(filepath.Join(outputPath, filename), []byte(content), 0644); err != nil {
			return err
		}
	}

	for _, gn := range godNodes {
		node, err := graph.GetNodesById(gn.Id)
		if err != nil {
			return err
		}
		content := e.generateGodNodeArticle(graph, node, communityLabels)
		filename := e.safeFilename(gn.Label) + ".md"
		if err := os.WriteFile(filepath.Join(outputPath, filename), []byte(content), 0644); err != nil {
			return err
		}
	}

	indexContent := e.generateIndex(communities, communityLabels, godNodes, graph.NodeCount(), graph.EdgeCount())
	return os.WriteFile(filepath.Join(outputPath, "index.md"), []byte(indexContent), 0644)
}

func (e *WikiExporter) calculateCohesion(graph KnowledgeGraph, nodeIDs []string) float64 {
	if len(nodeIDs) < 2 {
		return 0.0
	}

	nodeSet := make(map[string]struct{})
	for _, id := range nodeIDs {
		nodeSet[id] = struct{}{}
	}

	internalEdges := 0
	for _, id := range nodeIDs {
		edges := graph.GetEdgesById(id)
		for _, edge := range edges {
			_, srcExists := nodeSet[edge.Source.Id]
			_, tgtExists := nodeSet[edge.Target.Id]
			if srcExists && tgtExists {
				internalEdges++
			}
		}
	}

	possibleEdges := len(nodeIDs) * (len(nodeIDs) - 1)
	if possibleEdges > 0 {
		return float64(internalEdges) / float64(possibleEdges)
	}
	return 0.0
}

func (e *WikiExporter) generateCommunityArticle(
	graph KnowledgeGraph,
	communityID int,
	nodeIDs []string,
	label string,
	communityLabels map[int]string,
	cohesion float64) string {

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", label)
	fmt.Fprintf(&sb, "> %d nodes · cohesion %.2f\n\n", len(nodeIDs), cohesion)

	// Key Concepts
	sb.WriteString("## Key Concepts\n\n")

	type nodeInfo struct {
		ID     string
		Label  string
		Degree int
		Source string
	}
	var topNodes []nodeInfo
	for _, id := range nodeIDs {
		n, err := graph.GetNodesById(id)
		if err == nil {
			source := ""
			if s, ok := n.Metadata["source_file"]; ok {
				source = s
			}
			lbl := n.Id
			if n.Label != "" {
				lbl = n.Label
			}
			topNodes = append(topNodes, nodeInfo{id, lbl, graph.GetDegree(id), source})
		}
	}

	sort.Slice(topNodes, func(i, j int) bool {
		return topNodes[i].Degree > topNodes[j].Degree
	})

	limit := min(len(topNodes), 25)

	for i := range limit {
		n := topNodes[i]
		sourceStr := ""
		if n.Source != "" {
			sourceStr = fmt.Sprintf(" — `%s`", n.Source)
		}
		fmt.Fprintf(&sb, "- **%s** (%d connections)%s\n", n.Label, n.Degree, sourceStr)
	}

	if len(nodeIDs) > limit {
		fmt.Fprintf(&sb, "- *... and %d more nodes in this community*\n", len(nodeIDs)-limit)
	}
	sb.WriteString("\n")

	// Relationships
	sb.WriteString("## Relationships\n\n")
	crossLinks := e.getCrossCommunityLinks(graph, nodeIDs, communityID, communityLabels)
	if len(crossLinks) > 0 {
		for i, link := range crossLinks {
			if i >= 12 {
				break
			}
			fmt.Fprintf(&sb, "- [[%s]] (%d shared connections)\n", link.Label, link.Count)
		}
	} else {
		sb.WriteString("- No strong cross-community connections detected\n")
	}
	sb.WriteString("\n")

	// Audit Trail
	sb.WriteString("## Audit Trail\n\n")
	edgeCounts := map[string]int{"EXTRACTED": 0, "INFERRED": 0, "AMBIGUOUS": 0}
	totalEdges := 0
	for _, id := range nodeIDs {
		for _, edge := range graph.GetEdgesById(id) {
			edgeCounts[strings.ToUpper(string(edge.Confidence))]++
			totalEdges++
		}
	}

	if totalEdges > 0 {
		confs := []string{"EXTRACTED", "INFERRED", "AMBIGUOUS"}
		for _, c := range confs {
			count := edgeCounts[c]
			pct := float64(count) * 100.0 / float64(totalEdges)
			fmt.Fprintf(&sb, "- %s: %d (%.0f%%)\n", c, count, pct)
		}
	}
	sb.WriteString("\n---\n\n*Part of the graphify knowledge wiki. See [[index]] to navigate.*\n")

	return sb.String()
}

func (e *WikiExporter) generateGodNodeArticle(
	graph KnowledgeGraph,
	node GraphNode,
	communityLabels map[int]string) string {

	var sb strings.Builder
	label := node.Id
	if node.Label != "" {
		label = node.Label
	}
	degree := graph.GetDegree(node.Id)

	fmt.Fprintf(&sb, "# %s\n\n", label)

	sourceStr := ""
	if s, ok := node.Metadata["source_file"]; ok {
		sourceStr = fmt.Sprintf(" · `%s`", s)
	}
	fmt.Fprintf(&sb, "> God node · %d connections%s\n\n", degree, sourceStr)

	if node.Community != -1 {
		if lbl, ok := communityLabels[node.Community]; ok {
			fmt.Fprintf(&sb, "**Community:** [[%s]]\n\n", lbl)
		}
	}

	sb.WriteString("## Connections by Relation\n\n")
	byRelation := make(map[string][]string)
	neighbors := graph.GetNeighbors(node.Id)

	for _, neighbor := range neighbors {
		edges := graph.GetEdgesById(node.Id)
		for _, edge := range edges {
			if edge.Source.Id == neighbor.Id || edge.Target.Id == neighbor.Id {
				rel := "related"
				if edge.Relationship != "" {
					rel = edge.Relationship
				}
				nLabel := neighbor.Id
				if neighbor.Label != "" {
					nLabel = neighbor.Label
				}
				confStr := ""
				if strings.ToUpper(string(edge.Confidence)) != "EXTRACTED" {
					confStr = fmt.Sprintf(" `%s`", edge.Confidence)
				}
				entry := fmt.Sprintf("[[%s]]%s", nLabel, confStr)

				exists := slices.Contains(byRelation[rel], entry)
				if !exists {
					byRelation[rel] = append(byRelation[rel], entry)
				}
			}
		}
	}

	for rel, targets := range byRelation {
		fmt.Fprintf(&sb, "### %s\n", rel)
		for i, t := range targets {
			if i >= 20 {
				break
			}
			fmt.Fprintf(&sb, "- %s\n", t)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n*Part of the graphify knowledge wiki. See [[index]] to navigate.*\n")
	return sb.String()
}

func (e *WikiExporter) generateIndex(
	communities map[int][]string,
	communityLabels map[int]string,
	godNodes []GodNode,
	totalNodes, totalEdges int) string {

	var sb strings.Builder
	sb.WriteString("# Knowledge Graph Index\n\n")
	sb.WriteString("> Auto-generated by graphify. Start here — read community articles for context, then drill into god nodes for detail.\n\n")
	fmt.Fprintf(&sb, "**%d nodes · %d edges · %d communities**\n\n---\n\n", totalNodes, totalEdges, len(communities))

	sb.WriteString("## Communities\n(sorted by size, largest first)\n\n")

	ids := make([]int, 0, len(communities))
	for id := range communities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return len(communities[ids[i]]) > len(communities[ids[j]])
	})

	for _, id := range ids {
		fmt.Fprintf(&sb, "- [[%s]] — %d nodes\n", communityLabels[id], len(communities[id]))
	}

	if len(godNodes) > 0 {
		sb.WriteString("\n## God Nodes\n(most connected concepts — the load-bearing abstractions)\n\n")
		for _, gn := range godNodes {
			fmt.Fprintf(&sb, "- [[%s]] — %d connections\n", gn.Label, gn.EdgeCount)
		}
	}

	sb.WriteString("\n---\n\n*Generated by [graphify](https://github.com/safishamsi/graphify)*\n")
	return sb.String()
}

type crossLink struct {
	Label string
	Count int
}

func (e *WikiExporter) getCrossCommunityLinks(
	graph KnowledgeGraph,
	nodeIDs []string,
	ownCommunityID int,
	communityLabels map[int]string) []crossLink {

	counts := make(map[string]int)
	for _, id := range nodeIDs {
		neighbors := graph.GetNeighbors(id)
		for _, n := range neighbors {
			if n.Community != -1 && n.Community != ownCommunityID {
				if lbl, ok := communityLabels[n.Community]; ok {
					counts[lbl]++
				}
			}
		}
	}

	var results []crossLink
	for lbl, count := range counts {
		results = append(results, crossLink{lbl, count})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})
	return results
}

func (e *WikiExporter) safeFilename(name string) string {
	r := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		" ", "_",
		":", "-",
	)
	safe := r.Replace(name)

	var final strings.Builder
	for _, ch := range safe {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '-' || ch == '_' || ch == '.' {
			final.WriteRune(ch)
		} else {
			final.WriteRune('_')
		}
	}
	return final.String()
}

func (e *WikiExporter) getGodNodes(graph KnowledgeGraph, limit int) []GodNode {
	nodes := graph.GetHighestDegreeNodes(limit)
	result := make([]GodNode, 0, len(nodes))
	for _, v := range nodes {
		result = append(result, GodNode{
			Id:        v.Node.Id,
			Label:     v.Node.Label,
			EdgeCount: v.Degree,
		})
	}
	return result
}

// GetFormat implements [IGraphExporter].
func (w *WikiExporter) GetFormat() string {
	return "wiki"
}
