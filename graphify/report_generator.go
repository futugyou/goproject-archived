package graphify

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type ReportGenerator struct {
}

func (r *ReportGenerator) appendKnowledgeGaps(sb *strings.Builder, graph *KnowledgeGraph) {
	isolatedNodes := []GraphNode{}
	for _, node := range graph.GetNodes() {
		if graph.GetDegree(node.Id) > 1 {
			continue
		}
		isolatedNodes = append(isolatedNodes, node)
		if len(isolatedNodes) >= 10 {
			break
		}
	}

	var edges = graph.GetEdges()
	var totalEdges = 1
	if len(edges) > 0 {
		totalEdges = len(edges)
	}

	ambiguousCount := len(Where(edges, func(e GraphEdge) bool {
		return e.Confidence == ConfidenceAmbiguous
	}))

	ambiguousPct := ambiguousCount * 100 / totalEdges

	if len(isolatedNodes) > 0 || ambiguousPct > 20 {
		sb.WriteString("## Knowledge Gaps")

		if len(isolatedNodes) > 0 {
			isolatedLabels := []string{}
			for _, n := range isolatedNodes {
				isolatedLabels = append(isolatedLabels, n.LabelOrID())
				if len(isolatedLabels) >= 5 {
					break
				}
			}

			var suffix = ""
			if len(isolatedNodes) > 5 {
				suffix = fmt.Sprintf(" (+%d more)", len(isolatedNodes)-5)
			}
			fmt.Fprintf(sb, "- **%d isolated node(s):** %s%s", len(isolatedNodes), strings.Join(isolatedLabels, ", "), suffix)
			sb.WriteString("  These have ≤1 connection - possible missing edges or undocumented components.")
		}

		if ambiguousPct > 20 {
			fmt.Fprintf(sb, "- **High ambiguity: %d%% of edges are AMBIGUOUS.** Review and refine extraction rules.", ambiguousPct)
		}

		sb.WriteByte('\n')
	}
}

func (r *ReportGenerator) appendSuggestedQuestions(sb *strings.Builder, questions []SuggestedQuestion) {
	sb.WriteString("## Suggested Questions")

	if len(questions) == 0 {
		sb.WriteString("- No questions suggested")
		sb.WriteByte('\n')
		return
	}

	var noSignal = len(questions) == 1 && questions[0].Type == "no_signal"

	if noSignal {
		fmt.Fprintf(sb, "_%s_", questions[0].Why)
	} else {
		sb.WriteString("_Questions this graph is uniquely positioned to answer:_")
		sb.WriteByte('\n')

		for _, q := range questions {
			if len(q.Question) > 0 {
				fmt.Fprintf(sb, "- **%s**", q.Question)
				fmt.Fprintf(sb, "  _%s_", q.Why)
			}
		}
	}

	sb.WriteByte('\n')
}

func (r *ReportGenerator) appendCommunities(sb *strings.Builder, graph *KnowledgeGraph, communityLabels map[int]string, cohesionScores map[int]float32) {
	sb.WriteString("## Communities")

	communitiesById := graph.SortNodeByCommunityDesc()

	for _, item := range communitiesById {
		label := fmt.Sprintf("Community %d", item.CommunityID)
		if l, ok := communityLabels[item.CommunityID]; ok {
			label = l
		}

		var cohesion float32 = 0.0
		if c, ok := cohesionScores[item.CommunityID]; ok {
			cohesion = c
		}

		sb.WriteByte('\n')
		fmt.Fprintf(sb, "### Community %d - \"%s\"", item.CommunityID, label)
		fmt.Fprintf(sb, "Cohesion: %.2f", cohesion)

		nodes := item.Nodes
		sort.Slice(nodes, func(i, j int) bool {
			return graph.GetDegree(nodes[i].Id) > graph.GetDegree(nodes[j].Id)
		})

		// Show top nodes by degree
		displayNodes := []string{}
		for _, n := range nodes {
			displayNodes = append(displayNodes, n.LabelOrID())
			if len(displayNodes) >= 8 {
				break
			}
		}

		var suffix = ""
		if len(nodes) > 8 {
			suffix = fmt.Sprintf(" (+%d more)", len(nodes)-8)
		}
		fmt.Fprintf(sb, "Nodes (%d): %s%s", len(nodes), strings.Join(displayNodes, ", "), suffix)
	}

	sb.WriteByte('\n')
}

func (r *ReportGenerator) appendSurprisingConnections(sb *strings.Builder, connections []SurprisingConnection) {
	sb.WriteString("## Surprising Connections (you probably didn't know these)")

	if len(connections) == 0 {
		sb.WriteString("- None detected - all connections are within the same source files.")
	} else {
		for _, conn := range connections {
			var confTag = strings.ToUpper(string(conn.Confidence))
			var semanticTag = ""
			if conn.Relationship == "semantically_similar_to" {
				semanticTag = " [semantically similar]"
			}

			fmt.Fprintf(sb, "- `%s` --%s--> `%s`  [%s]%s", conn.Source, conn.Relationship, conn.Target, confTag, semanticTag)

			if len(conn.SourceFiles) >= 2 {
				fmt.Fprintf(sb, "  %s → %s", conn.SourceFiles[0], conn.SourceFiles[1])
			}

			if len(conn.Why) > 0 {
				fmt.Fprintf(sb, "  _%s_", conn.Why)
			}

			sb.WriteByte('\n')
		}
	}

	sb.WriteByte('\n')
}

func (r *ReportGenerator) appendGodNodes(sb *strings.Builder, godNodes []GodNode) {
	sb.WriteString("## God Nodes (most connected - your core abstractions)")

	if len(godNodes) == 0 {
		sb.WriteString("- No highly connected nodes detected")
	} else {
		for i := range godNodes {
			var node = godNodes[i]
			fmt.Fprintf(sb, "%d. `%s` - %d edges", i+1, node.Label, node.EdgeCount)
		}
	}

	sb.WriteByte('\n')
}

func (r *ReportGenerator) appendSummary(sb *strings.Builder, graph *KnowledgeGraph, analysis AnalysisResult) {
	sb.WriteString("## Summary")

	// Calculate confidence distribution
	var edges = graph.GetEdges()
	var totalEdges = 1
	if len(edges) > 0 {
		totalEdges = len(edges)
	}

	var extractedCount = len(Where(edges, func(e GraphEdge) bool {
		return e.Confidence == ConfidenceExtracted
	}))

	inferredEdges := Where(edges, func(e GraphEdge) bool {
		return e.Confidence == ConfidenceInferred
	})
	var inferredCount = len(inferredEdges)
	var ambiguousCount = len(Where(edges, func(e GraphEdge) bool {
		return e.Confidence == ConfidenceAmbiguous
	}))

	var extractedPct = math.Round(float64(extractedCount*100.0) / float64(totalEdges))
	var inferredPct = math.Round(float64(inferredCount*100.0) / float64(totalEdges))
	var ambiguousPct = math.Round(float64(ambiguousCount*100.0) / float64(totalEdges))

	fmt.Fprintf(sb, "- %d nodes · %d edges · %d communities detected", analysis.Statistics.NodeCount, analysis.Statistics.EdgeCount, analysis.Statistics.CommunityCount)
	fmt.Fprintf(sb, "- Extraction: %.2f%% EXTRACTED · %.2f%% INFERRED · %.2f%% AMBIGUOUS", extractedPct, inferredPct, ambiguousPct)

	if inferredCount > 0 {
		sumConfidence := 0.0
		for _, v := range inferredEdges {
			sumConfidence += float64(v.Weight)
		}
		var avgConfidence = sumConfidence / float64(inferredCount)
		fmt.Fprintf(sb, "  INFERRED: %d edges (avg confidence: %.2f)", inferredCount, avgConfidence)
	}

	sb.WriteByte('\n')
}

func (r *ReportGenerator) Generate(graph *KnowledgeGraph, analysis AnalysisResult, communityLabels map[int]string, cohesionScores map[int]float32, projectName string) string {
	var sb strings.Builder
	var today = time.Now().Format("2006-01-02 15:04:05")

	// Header
	fmt.Fprintf(&sb, "# Graph Report - %s  (%s)", projectName, today)
	sb.WriteByte('\n')

	// Summary
	r.appendSummary(&sb, graph, analysis)

	// God Nodes
	r.appendGodNodes(&sb, analysis.GodNodes)

	// Surprising Connections
	r.appendSurprisingConnections(&sb, analysis.SurprisingConnections)

	// Communities
	r.appendCommunities(&sb, graph, communityLabels, cohesionScores)

	// Suggested Questions
	r.appendSuggestedQuestions(&sb, analysis.SuggestedQuestions)

	// Knowledge Gaps
	r.appendKnowledgeGaps(&sb, graph)

	return sb.String()
}
