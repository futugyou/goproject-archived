package graphify

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
)

var _ IGraphExporter = (*SvgExporter)(nil)

type SvgExporter struct {
	width      int
	height     int
	nodeRadius int
	padding    int
}

func NewSvgExporter() *SvgExporter {
	return &SvgExporter{
		width:      1600,
		height:     1200,
		nodeRadius: 8,
		padding:    50,
	}
}

// Export implements [IGraphExporter].
func (s *SvgExporter) Export(ctx context.Context, graph *KnowledgeGraph, outputPath string) error {
	nodes := graph.GetNodes()
	var svg string

	if len(nodes) == 0 {
		svg = s.generateEmptySvg()
	} else {
		positions := s.calculateLayout(graph, nodes)
		svg = s.generateSvg(graph, nodes, positions)
	}

	return os.WriteFile(outputPath, []byte(svg), 0644)
}

func (s *SvgExporter) calculateLayout(graph *KnowledgeGraph, nodes []GraphNode) map[string][2]float64 {
	positions := make(map[string][2]float64)
	r := rand.New(rand.NewSource(42)) // Fixed seed for reproducibility
	// Initialize with random positions
	for _, node := range nodes {
		x := r.Float64()*float64(s.width-2*s.padding) + float64(s.padding)
		y := r.Float64()*float64(s.height-2*s.padding) + float64(s.padding)
		positions[node.Id] = [2]float64{x, y}
	}
	// Simple force-directed iterations
	const iterations = 100
	const k = 50.0 // Ideal spring length
	const damping = 0.9

	for iter := 0; iter < iterations; iter++ {
		forces := make(map[string][2]float64)
		// Initialize forces
		for _, node := range nodes {
			forces[node.Id] = [2]float64{0, 0}
		}

		// Repulsive forces between all nodes
		for i := range nodes {
			for j := i + 1; j < len(nodes); j++ {
				n1, n2 := nodes[i], nodes[j]
				p1, p2 := positions[n1.Id], positions[n2.Id]

				dx := p2[0] - p1[0]
				dy := p2[1] - p1[1]
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist < 1.0 {
					dist = 1.0
				}

				force := (k * k) / dist
				fx := force * dx / dist
				fy := force * dy / dist

				f1, f2 := forces[n1.Id], forces[n2.Id]
				forces[n1.Id] = [2]float64{f1[0] - fx, f1[1] - fy}
				forces[n2.Id] = [2]float64{f2[0] + fx, f2[1] + fy}
			}
		}

		// Attractive forces along edges
		for _, edge := range graph.GetEdges() {
			p1, p2 := positions[edge.Source.Id], positions[edge.Target.Id]

			dx := p2[0] - p1[0]
			dy := p2[1] - p1[1]
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < 1.0 {
				dist = 1.0
			}

			force := (dist * dist) / k
			fx := force * dx / dist
			fy := force * dy / dist

			f1, f2 := forces[edge.Source.Id], forces[edge.Target.Id]
			forces[edge.Source.Id] = [2]float64{f1[0] + fx, f1[1] + fy}
			forces[edge.Target.Id] = [2]float64{f2[0] - fx, f2[1] - fy}
		}

		// Apply forces with damping
		var maxForceSq float64
		for _, node := range nodes {
			f := forces[node.Id]
			fMagSq := f[0]*f[0] + f[1]*f[1]
			if fMagSq > maxForceSq {
				maxForceSq = fMagSq
			}

			pos := positions[node.Id]
			newX := pos[0] + f[0]*damping
			newY := pos[1] + f[1]*damping

			newX = math.Max(float64(s.padding), math.Min(float64(s.width-s.padding), newX))
			newY = math.Max(float64(s.padding), math.Min(float64(s.height-s.padding), newY))

			positions[node.Id] = [2]float64{newX, newY}
		}

		if math.Sqrt(maxForceSq) < 0.1 {
			break
		}
	}

	return positions
}

func (s *SvgExporter) generateSvg(graph *KnowledgeGraph, nodes []GraphNode, positions map[string][2]float64) string {
	var sb strings.Builder

	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&sb, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", s.width, s.height, s.width, s.height)
	fmt.Fprintf(&sb, "  <rect width=\"%d\" height=\"%d\" fill=\"#ffffff\"/>\n", s.width, s.height)
	sb.WriteString("  <style>\n    .edge { stroke: #999; stroke-width: 1; stroke-opacity: 0.6; }\n    .node { stroke: #fff; stroke-width: 2; }\n    .label { font-family: Arial, sans-serif; font-size: 10px; fill: #333; }\n  </style>\n")

	sb.WriteString("  <g id=\"edges\">\n")
	for _, edge := range graph.GetEdges() {
		p1, ok1 := positions[edge.Source.Id]
		p2, ok2 := positions[edge.Target.Id]
		if ok1 && ok2 {
			fmt.Fprintf(&sb, "    <line class=\"edge\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\"/>\n", p1[0], p1[1], p2[0], p2[1])
		}
	}
	sb.WriteString("  </g>\n")

	// Draw edges first (so they appear behind nodes)
	sb.WriteString("  <g id=\"nodes\">\n")
	for _, node := range nodes {
		pos, ok := positions[node.Id]
		if !ok {
			continue
		}

		color := s.getCommunityColor(node.Community)
		label := node.Label
		if label == "" {
			label = node.Id
		}
		degree := graph.GetDegree(node.Id)
		radius := float64(s.nodeRadius) + math.Min(float64(degree)/5.0, 10.0)

		fmt.Fprintf(&sb, "    <circle class=\"node\" cx=\"%.2f\" cy=\"%.2f\" r=\"%.2f\" fill=\"%s\">\n", pos[0], pos[1], radius, color)
		fmt.Fprintf(&sb, "      <title>%s (%d connections)</title>\n", s.escapeXml(label), degree)
		sb.WriteString("    </circle>\n")

		if degree > 10 {
			labelY := pos[1] + radius + 12
			fmt.Fprintf(&sb, "    <text class=\"label\" x=\"%.2f\" y=\"%.2f\" text-anchor=\"middle\">%s</text>\n", pos[0], labelY, s.escapeXml(s.truncateLabel(label, 20)))
		}
	}
	sb.WriteString("  </g>\n")

	// Legend
	sb.WriteString("  <g id=\"legend\">\n")
	fmt.Fprintf(&sb, "    <text x=\"20\" y=\"30\" font-family=\"Arial\" font-size=\"14\" font-weight=\"bold\">Knowledge Graph</text>\n")
	fmt.Fprintf(&sb, "    <text x=\"20\" y=\"50\" font-family=\"Arial\" font-size=\"12\" fill=\"#666\">%d nodes · %d edges</text>\n", len(nodes), graph.EdgeCount())
	sb.WriteString("  </g>\n")

	sb.WriteString("</svg>")
	return sb.String()
}

func (s *SvgExporter) generateEmptySvg() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="%d" height="%d" fill="#ffffff"/>
  <text x="%d" y="%d" text-anchor="middle" font-family="Arial" font-size="24" fill="#999">
    Empty Graph
  </text>
</svg>`, s.width, s.height, s.width, s.height, s.width, s.height, s.width/2, s.height/2)
}

func (s *SvgExporter) getCommunityColor(communityId int) string {
	if communityId == -1 {
		return "#cccccc"
	}
	colors := []string{
		"#4285F4", "#EA4335", "#FBBC04", "#34A853", "#FF6D00",
		"#9C27B0", "#00BCD4", "#8BC34A", "#FF5722", "#795548",
		"#607D8B", "#E91E63", "#3F51B5", "#009688", "#FFC107",
	}
	return colors[communityId%len(colors)]
}

func (s *SvgExporter) truncateLabel(label string, maxLength int) string {
	runes := []rune(label)
	if len(runes) <= maxLength {
		return label
	}
	return string(runes[:maxLength-3]) + "..."
}

func (s *SvgExporter) escapeXml(text string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(text)
}

// GetFormat implements [IGraphExporter].
func (s *SvgExporter) GetFormat() string {
	return "svg"
}
