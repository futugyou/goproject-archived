package graphify

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

var _ IPipelineStage[KnowledgeGraph, AnalysisResult] = (*Analyzer)(nil)

type Analyzer struct {
	options *AnalyzerOptions
}

func NewAnalyzer(options *AnalyzerOptions) *Analyzer {
	if options == nil {
		options = DefaultAnalyzerOptions()
	}
	return &Analyzer{
		options: options,
	}
}

// Execute implements [IPipelineStage].
func (a *Analyzer) Execute(ctx context.Context, graph KnowledgeGraph) (*AnalysisResult, error) {
	var godNodes = a.findGodNodes(graph)
	var surprisingConnections = a.findSurprisingConnections(graph)
	var suggestedQuestions = a.generateSuggestedQuestions(graph)
	var statistics = a.calculateStatistics(graph)

	return &AnalysisResult{
		GodNodes:              godNodes,
		SurprisingConnections: surprisingConnections,
		SuggestedQuestions:    suggestedQuestions,
		Statistics:            statistics,
	}, nil
}

func (a *Analyzer) getTopLevelDir(path string) string {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return path
}

func (a *Analyzer) getFileCategory(path string) string {
	fileName := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(fileName))

	if slices.Contains(AnalyzerCodeExtensions, ext) {
		return "code"
	}

	if ext == "pdf" {
		return "paper"

	}
	if slices.Contains(AnalyzerImageExtensions, ext) {
		return "image"
	}
	return "doc"
}

func (a *Analyzer) isConceptNode(node GraphNode) bool {
	// Concept nodes have no source file
	if len(node.FilePath) == 0 {
		return true
	}

	// Or source file has no extension (not a real file)
	fileName := filepath.Base(node.FilePath)
	return strings.Contains(fileName, ".")
}

func (a *Analyzer) isFileNode(node GraphNode) bool {
	var label = node.Label
	if len(label) == 0 {
		return false
	}

	// File-level hub: label is a filename with code extension
	var parts = strings.Split(label, ".")
	if len(parts) > 1 && slices.Contains(AnalyzerCodeExtensions, parts[len(parts)-1]) {
		return true
	}

	// Method stub: .method_name()
	if strings.HasPrefix(label, ".") && strings.HasSuffix(label, "()") {
		return true
	}

	// Module-level function stub: function_name() with degree <= 1
	if strings.HasPrefix(label, ".") && strings.HasSuffix(label, "()") {
		return true
	}

	if strings.HasSuffix(label, "()") && !strings.Contains(label, ".") {
		return true
	}

	return false
}

func (a *Analyzer) buildCommunityLabels(graph KnowledgeGraph) map[int]string {
	result := make(map[int]string)

	nodesWithCommunity := Where(graph.GetNodes(), func(n GraphNode) bool {
		return n.Community != -1
	})

	communities := GroupBy(nodesWithCommunity, func(n GraphNode) int {
		return n.Community
	})

	for commId, nodes := range communities {
		typeGroups := GroupBy(nodes, func(n GraphNode) string {
			return n.Type
		})

		commonType := "Mixed"
		maxCount := 0

		for t, group := range typeGroups {
			if len(group) > maxCount {
				maxCount = len(group)
				commonType = t
			}
		}

		result[commId] = fmt.Sprintf("%s (Community %d)", commonType, commId)
	}

	return result
}

func (a *Analyzer) calculateStatistics(graph KnowledgeGraph) GraphStatistics {
	var nodes = graph.GetNodes()
	var edges = graph.GetEdges()

	nodeCount := len(nodes)
	edgeCount := len(edges)

	communities := map[int]struct{}{}
	sumDegree := 0
	isolatedCount := 0
	for _, node := range nodes {
		d := graph.GetDegree(node.Id)
		if d <= 1 {
			isolatedCount++
		}
		sumDegree += d
		if _, ok := communities[node.Community]; !ok && node.Community != -1 {
			communities[node.Community] = struct{}{}
		}
	}

	communityCount := len(communities)
	var averageDegree float32 = 0.0
	if nodeCount > 0 {
		averageDegree = (float32)(sumDegree) / (float32)(nodeCount)
	}

	return GraphStatistics{
		NodeCount:         nodeCount,
		EdgeCount:         edgeCount,
		CommunityCount:    communityCount,
		AverageDegree:     averageDegree,
		IsolatedNodeCount: isolatedCount,
	}
}

func (a *Analyzer) generateSuggestedQuestions(graph KnowledgeGraph) []SuggestedQuestion {
	var questions = []SuggestedQuestion{}

	// 1. AMBIGUOUS edges
	for _, edge := range graph.GetEdges() {
		if edge.Confidence == ConfidenceAmbiguous {
			questions = append(questions, SuggestedQuestion{
				Type:     "ambiguous_edge",
				Question: fmt.Sprintf("What is the exact relationship between `%s` and `%s`?", edge.Source.Label, edge.Target.Label),
				Why:      fmt.Sprintf("Edge tagged AMBIGUOUS (relation: %s) - confidence is low.", edge.Relationship),
			})
		}
	}

	// 2. Bridge nodes (nodes connecting multiple communities)
	nodeCommunityMap := ToDictionary(
		graph.GetNodes(),
		func(n GraphNode) string { return n.Id },     // KeySelector
		func(n GraphNode) int { return n.Community }, // ValueSelector
	)
	var communityLabels = a.buildCommunityLabels(graph)

	for _, node := range graph.GetNodes() {
		if a.isFileNode(node) || a.isConceptNode(node) {
			continue
		}

		nodeCommunity, commLabel, otherLabel, ok := 0, "", "", false
		if nodeCommunity, ok = nodeCommunityMap[node.Id]; !ok {
			continue
		}

		var neighbors = graph.GetNeighbors(node.Id)

		neighborCommunities := []int{}
		for _, n := range neighbors {
			if _, ok := nodeCommunityMap[n.Id]; ok && nodeCommunityMap[n.Id] != nodeCommunity {
				if !slices.Contains(neighborCommunities, nodeCommunityMap[n.Id]) {
					neighborCommunities = append(neighborCommunities, nodeCommunityMap[n.Id])
				}
			}
		}
		bridge_nodeQ := Where(questions, func(n SuggestedQuestion) bool {
			return n.Type == "bridge_node"
		})

		if len(neighborCommunities) >= 2 && len(bridge_nodeQ) < 3 {
			if commLabel, ok = communityLabels[nodeCommunity]; !ok {
				commLabel = fmt.Sprintf("Community %d", nodeCommunity)
			}
			otherLabels := []string{}
			for _, c := range neighborCommunities {
				if otherLabel, ok = communityLabels[c]; !ok {
					otherLabel = fmt.Sprintf("`Community %d`", c)
				}
				otherLabels = append(otherLabels, otherLabel)
			}

			questions = append(questions, SuggestedQuestion{
				Type:     "bridge_node",
				Question: fmt.Sprintf("Why does `%s` connect `%s` to %s?", node.Label, commLabel, strings.Join(otherLabels, ", ")),
				Why:      "This node bridges multiple communities - it's a cross-cutting concern.",
			})
		}
	}

	// 3. God nodes with many INFERRED edges
	nodesWithEdges := make([]inferredEdgesNode, 0, len(graph.GetNodes())/2)
	for _, n := range graph.GetNodes() {
		if a.isFileNode(n) {
			continue
		}

		infCount := 0
		for _, e := range graph.GetEdgesById(n.Id) {
			if e.Confidence == ConfidenceInferred {
				infCount++
			}
		}

		if infCount >= 2 {
			nodesWithEdges = append(nodesWithEdges, inferredEdgesNode{
				Node:          n,
				InferredEdges: infCount,
			})
		}
	}

	slices.SortFunc(nodesWithEdges, func(a, b inferredEdgesNode) int {
		return cmp.Compare(b.InferredEdges, a.InferredEdges)
	})

	godNodesWithInferred := nodesWithEdges[:min(len(nodesWithEdges), 3)]

	for _, item := range godNodesWithInferred {
		inferredEdges := Where(graph.GetEdgesById(item.Node.Id), func(n GraphEdge) bool {
			return n.Confidence == ConfidenceInferred
		})
		if len(inferredEdges) > 2 {
			var other1 = inferredEdges[0].Source.Label
			if inferredEdges[0].Source.Id == item.Node.Id {
				other1 = inferredEdges[0].Target.Label
			}

			var other2 = inferredEdges[1].Source.Label
			if inferredEdges[1].Source.Id == item.Node.Id {
				other2 = inferredEdges[1].Target.Label
			}

			questions = append(questions, SuggestedQuestion{
				Type:     "verify_inferred",
				Question: fmt.Sprintf("Are the %d inferred relationships involving `%s` (e.g. with `%s` and `%s`) actually correct?", item.InferredEdges, item.Node.Label, other1, other2),
				Why:      fmt.Sprintf("`%s` has %d INFERRED edges - model-reasoned connections that need verification.", item.Node.Label, item.InferredEdges),
			})
		}
	}

	// 4. Isolated nodes
	isolated := Where(graph.GetNodes(), func(n GraphNode) bool {
		return !a.isFileNode(n) && !a.isConceptNode(n) && graph.GetDegree(n.Id) <= 1
	})

	if len(isolated) > 0 {
		labels := Take(Select(isolated, func(n GraphNode) string {
			return fmt.Sprintf("`%s`", n.Label)
		}), 3)

		totalIsolated := len(Where(graph.GetNodes(), func(n GraphNode) bool {
			return !a.isFileNode(n) && !a.isConceptNode(n) && graph.GetDegree(n.Id) <= 1
		}))

		questions = append(questions, SuggestedQuestion{
			Type:     "isolated_nodes",
			Question: fmt.Sprintf("What connects %s to the rest of the system?", strings.Join(labels, ", ")),
			Why:      fmt.Sprintf("%d weakly-connected nodes found - possible documentation gaps or missing edges.", totalIsolated),
		})
	}

	// If no questions generated
	if len(questions) == 0 {
		questions = append(questions, SuggestedQuestion{
			Type: "no_signal",
			Why:  "Not enough signal to generate questions. The graph has no ambiguous edges, no bridge nodes, and all communities are well-connected.",
		})
	}

	return questions[:min(len(questions), a.options.MaxSuggestedQuestions)]
}

type inferredEdgesNode struct {
	Node          GraphNode
	InferredEdges int
}

func (a *Analyzer) findCrossCommunityBridges(graph KnowledgeGraph) []SurprisingConnection {
	var result = []SurprisingConnection{}

	// Build community map
	nodeCommunityMap := ToDictionary(
		graph.GetNodes(),
		func(n GraphNode) string { return n.Id },
		func(n GraphNode) int { return n.Community },
	)
	if len(nodeCommunityMap) == 0 {
		return result
	}

	for _, edge := range graph.GetEdges() {
		commSource, commTarget, ok := -1, -1, false
		if commSource, ok = nodeCommunityMap[edge.Source.Id]; !ok {
			continue
		}
		if commTarget, ok = nodeCommunityMap[edge.Target.Id]; !ok {
			continue
		}

		if commSource == commTarget {
			continue
		}

		// Skip file nodes and structural edges
		if a.isFileNode(edge.Source) || a.isFileNode(edge.Target) {
			continue
		}

		if slices.Contains(AnalyzerStructuralRelations, edge.Relationship) {
			continue
		}

		// Deduplicate by community pair

		var seenPairs = map[string]struct{}{}
		pair := fmt.Sprintf("%d, %d", commSource, commTarget)
		if commSource >= commTarget {
			pair = fmt.Sprintf("%d, %d", commTarget, commSource)
		}

		if _, ok := seenPairs[pair]; ok {
			continue
		}

		seenPairs[pair] = struct{}{}

		result = append(result, SurprisingConnection{
			Source:       edge.Source.Label,
			Target:       edge.Target.Label,
			SourceFiles:  []string{edge.Source.FilePath, edge.Target.FilePath},
			Relationship: edge.Relationship,
			Confidence:   edge.Confidence,
			Why:          fmt.Sprintf("Bridges community %d → community %d", commSource, commTarget),
		})
	}

	// Sort by confidence: AMBIGUOUS, INFERRED, EXTRACTED
	slices.SortFunc(result, func(a, b SurprisingConnection) int {
		getVal := func(c Confidence) int {
			switch c {
			case ConfidenceExtracted:
				return 2
			case ConfidenceInferred:
				return 1
			default:
				return 0 // Ambiguous
			}
		}
		return cmp.Compare(getVal(a.Confidence), getVal(b.Confidence))
	})

	return result[:min(len(result), a.options.TopSurprisingConnections)]
}

func (a *Analyzer) calculateSurpriseScore(graph KnowledgeGraph, edge GraphEdge, sourceFile, targetFile string) calculateSurpriseScoreType {
	score := 0
	var reasons = []string{}

	// 1. Confidence weight
	getVal := func(c Confidence) int {
		switch c {
		case ConfidenceAmbiguous:
			return 3
		case ConfidenceInferred:
			return 2
		default:
			return 1 // Ambiguous
		}
	}

	var confBonus = getVal(edge.Confidence)
	score += confBonus
	if edge.Confidence == ConfidenceAmbiguous || edge.Confidence == ConfidenceInferred {
		reasons = append(reasons, fmt.Sprintf("%s connection - not explicitly stated in source", strings.ToLower(string(edge.Confidence))))
	}

	// 2. Cross file-type bonus
	var catSource = a.getFileCategory(sourceFile)
	var catTarget = a.getFileCategory(targetFile)
	if catSource != catTarget {
		score += 2
		reasons = append(reasons, fmt.Sprintf("crosses file types (%s ↔ %s)", catSource, catTarget))
	}

	// 3. Cross-directory bonus
	if a.getTopLevelDir(sourceFile) != a.getTopLevelDir(targetFile) {
		score += 2
		reasons = append(reasons, "connects across different repos/directories")
	}

	// 4. Cross-community bonus
	if edge.Source.Community != -1 && edge.Target.Community != -1 &&
		edge.Source.Community != edge.Target.Community {
		score += 1
		reasons = append(reasons, "bridges separate communities")
	}

	// 5. Peripheral to hub connection
	var degSource = graph.GetDegree(edge.Source.Id)
	var degTarget = graph.GetDegree(edge.Target.Id)
	if min(degSource, degTarget) <= 2 && max(degSource, degTarget) >= 5 {
		score += 1
		var peripheral = edge.Target.Label
		if degSource <= 2 {
			peripheral = edge.Source.Label
		}
		var hub = edge.Source.Label
		if degSource <= 2 {
			hub = edge.Target.Label
		}
		reasons = append(reasons, fmt.Sprintf("peripheral node `%s` unexpectedly reaches hub `%s`", peripheral, hub))
	}

	return calculateSurpriseScoreType{
		Score:   score,
		Reasons: reasons,
	}
}

type calculateSurpriseScoreType struct {
	Score   int
	Reasons []string
}

func (a *Analyzer) findCrossFileSurprises(graph KnowledgeGraph) []SurprisingConnection {
	var candidates = []findCrossFileSurprises{}

	for _, edge := range graph.GetEdges() {
		// Skip structural edges
		if slices.Contains(AnalyzerStructuralRelations, edge.Relationship) {
			continue
		}

		// Skip concept and file nodes
		if a.isConceptNode(edge.Source) || a.isConceptNode(edge.Target) {
			continue
		}
		if a.isFileNode(edge.Source) || a.isFileNode(edge.Target) {
			continue
		}

		var sourceFile = edge.Source.FilePath
		var targetFile = edge.Target.FilePath

		// Only cross-file connections
		if len(sourceFile) == 0 || len(targetFile) == 0 || sourceFile == targetFile {
			continue
		}

		var scoreResult = a.calculateSurpriseScore(graph, edge, sourceFile, targetFile)

		c := SurprisingConnection{
			Source:       edge.Source.Label,
			Target:       edge.Target.Label,
			SourceFiles:  []string{sourceFile, targetFile},
			Relationship: edge.Relationship,
			Confidence:   edge.Confidence,
			Why:          "cross-file semantic connection",
		}

		if len(scoreResult.Reasons) > 0 {
			c.Why = strings.Join(scoreResult.Reasons, "; ")
		}
		candidates = append(candidates, findCrossFileSurprises{
			Score:      scoreResult.Score,
			Connection: c,
		})

	}

	slices.SortFunc(candidates, func(a, b findCrossFileSurprises) int {
		return cmp.Compare(b.Score, a.Score)
	})

	resultCount := min(len(candidates), a.options.TopSurprisingConnections)
	result := make([]SurprisingConnection, resultCount)
	for i := 0; i < resultCount; i++ {
		result[i] = candidates[i].Connection
	}

	return result
}

type findCrossFileSurprises struct {
	Score      int
	Connection SurprisingConnection
}

func (a *Analyzer) findSurprisingConnections(graph KnowledgeGraph) []SurprisingConnection {
	// Identify unique source files
	sourceFiles := []string{}
	for _, node := range graph.GetNodes() {
		if len(node.FilePath) == 0 {
			continue
		}
		if !slices.Contains(sourceFiles, node.FilePath) {
			sourceFiles = append(sourceFiles, node.FilePath)
		}
	}

	isMultiSource := len(sourceFiles) > 1

	if isMultiSource {
		return a.findCrossFileSurprises(graph)
	} else {
		return a.findCrossCommunityBridges(graph)
	}
}

func (a *Analyzer) findGodNodes(graph KnowledgeGraph) []GodNode {
	var result = []GodNode{}
	var allNodes = graph.GetNodes()

	var nodesByDegree = []findGodNodesType{}
	for _, node := range allNodes {
		nodesByDegree = append(nodesByDegree, findGodNodesType{
			Node:   node,
			Degree: graph.GetDegree(node.Id),
		})
	}

	slices.SortFunc(nodesByDegree, func(a, b findGodNodesType) int {
		return cmp.Compare(b.Degree, a.Degree)
	})

	for _, item := range nodesByDegree {

		if a.isFileNode(item.Node) || a.isConceptNode(item.Node) {
			continue
		}

		result = append(result, GodNode{
			Id:        item.Node.Id,
			Label:     item.Node.Label,
			EdgeCount: item.Degree,
		})

		if len(result) >= a.options.TopGodNodesCount {
			break
		}
	}

	return result
}

type findGodNodesType struct {
	Node   GraphNode
	Degree int
}
