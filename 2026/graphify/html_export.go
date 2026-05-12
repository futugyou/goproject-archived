package graphify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var _ IGraphExporter = (*HtmlExporter)(nil)

type HtmlExporter struct {
}

// ExportAsync implements [IGraphExporter].
func (h *HtmlExporter) Export(ctx context.Context, graph KnowledgeGraph, outputPath string) error {
	return h.ExportWIthLabels(ctx, graph, outputPath, nil)
}

func (h *HtmlExporter) ExportWIthLabels(ctx context.Context, graph KnowledgeGraph, outputPath string, communityLabels map[int]string) error {
	if graph.NodeCount() > MaxNodesForVisualization {
		return fmt.Errorf("Graph has %d nodes - too large for HTML visualization.Maximum is %d nodes ", graph.NodeCount(), MaxNodesForVisualization)
	}

	communities := h.buildCommunityMap(graph)
	degrees := ToDictionary(
		graph.GetNodes(),
		func(n GraphNode) string { return n.Id },               // KeySelector
		func(n GraphNode) int { return graph.GetDegree(n.Id) }, // ValueSelector
	)
	maxDegree := 1
	for _, d := range degrees {
		if d > maxDegree {
			maxDegree = d
		}
	}

	visNodes := h.buildVisNodes(graph, degrees, maxDegree, communityLabels)
	visEdges := h.buildVisEdges(graph)
	legendData := h.buildLegend(communities, communityLabels)

	nodesJson, err := json.Marshal(visNodes)
	if err != nil {
		return err
	}

	edgesJson, err := json.Marshal(visEdges)
	if err != nil {
		return err
	}

	legendJson, err := json.Marshal(legendData)
	if err != nil {
		return err
	}

	stats := fmt.Sprintf("%d nodes &middot; %d edges &middot; %d communities", graph.NodeCount(), graph.EdgeCount(), len(communities))
	title := SanitizeLabel(GetFileName(outputPath), -1)

	f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	return HtmlTemplateGenerate(title, string(nodesJson), string(edgesJson), string(legendJson), stats, f)
}

func (h *HtmlExporter) buildCommunityMap(graph KnowledgeGraph) map[int][]string {
	communities := map[int][]string{}
	for _, node := range graph.GetNodes() {
		var communityId = node.Community
		if _, ok := communities[communityId]; !ok {
			communities[communityId] = []string{}
		} else {
			communities[communityId] = append(communities[communityId], node.Id)
		}
	}

	return communities
}

func (h *HtmlExporter) buildVisNodes(graph KnowledgeGraph, degrees map[string]int, maxDegree int, communityLabels map[int]string) []VisNode {
	visNodes := []VisNode{}
	var safeDegree = maxDegree
	for _, node := range graph.GetNodes() {
		var communityId = node.Community
		var color = CommunityColors[communityId%len(CommunityColors)]
		var label = SanitizeLabel(node.Label, -1)
		degree, ok := degrees[node.Id]
		if !ok {
			degree = 1
		}
		// Node size proportional to degree (10-40 range)
		var size = 10 + 30*(degree/safeDegree)

		// Only show label for high-degree nodes by default; others show on hover
		var fontSize = 12
		if (float32)(degree) < ((float32)(maxDegree) * 0.15) {
			fontSize = 0
		}

		var communityName = communityLabels[communityId]
		if len(communityName) == 0 {
			communityName = fmt.Sprintf("Community %d", communityId)
		}
		var sourceFile = ""
		if len(node.RelativePath) > 0 {
			sourceFile = node.RelativePath
		} else if len(node.FilePath) > 0 {
			sourceFile = node.FilePath
		}
		visNodes = append(visNodes, VisNode{
			ID:    node.Id,
			Label: label,
			Color: ColorConfig{
				Background: color,
				Border:     color,
				Highlight: HighlightColor{
					Background: "#ffffff",
					Border:     color,
				},
			},
			Size: float64(size),
			Font: FontConfig{
				Size:  fontSize,
				Color: "#ffffff",
			},
			Title:         label,
			Community:     communityId,
			CommunityName: communityName,
			SourceFile:    SanitizeLabel(sourceFile, -1),
			FileType:      node.Type,
			Degree:        degree,
		})
	}

	return visNodes
}

func (h *HtmlExporter) buildLegend(communities map[int][]string, communityLabels map[int]string) []LegendData {
	legendData := []LegendData{}
	ForEachSorted(communities, func(communityId int, nodeIds []string) {
		color := CommunityColors[communityId%len(CommunityColors)]
		var label = communityLabels[communityId]
		if len(label) == 0 {
			label = fmt.Sprintf("Community %d", communityId)
		}
		var count = len(nodeIds)

		legendData = append(legendData, LegendData{
			Cid:   communityId,
			Color: color,
			Count: count,
			Label: label,
		})
	})
	return legendData
}

func (h *HtmlExporter) buildVisEdges(graph KnowledgeGraph) []ViEdge {
	legendData := []ViEdge{}

	for _, edge := range graph.GetEdges() {
		var confidence = strings.ToUpper((string)(edge.Confidence))
		var relation = edge.Relationship
		d := ViEdge{
			From:       edge.Source.Id,
			To:         edge.Target.Id,
			Label:      relation,
			Title:      fmt.Sprintf("%s [%s]", relation, confidence),
			Dashes:     confidence != "EXTRACTED",
			Confidence: confidence,
			Width:      2,
			Color: EdgeColor{
				Opacity: 0.7,
			},
		}

		if confidence != "EXTRACTED" {
			d.Width = 1
			d.Color.Opacity = 0.35
		}

		legendData = append(legendData, d)

	}

	return legendData
}

// GetFormat implements [IGraphExporter].
func (h *HtmlExporter) GetFormat() string {
	return "html"
}
