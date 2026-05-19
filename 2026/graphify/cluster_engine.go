package graphify

import (
	"context"
	"slices"
)

var _ IPipelineStage[KnowledgeGraph, KnowledgeGraph] = (*ClusterEngine)(nil)

type ClusterEngine struct {
	options ClusterOptions
}

func NewClusterEngine(options *ClusterOptions) *ClusterEngine {
	if options == nil {
		options = DefaultClusterOptions()
	}

	return &ClusterEngine{
		options: *options,
	}
}

// Execute implements [IPipelineStage].
func (c *ClusterEngine) Execute(ctx context.Context, graph KnowledgeGraph) (*KnowledgeGraph, error) {
	if graph.NodeCount() == 0 {
		return &graph, nil
	}

	var communities = c.detectCommunities(graph)
	g := &graph
	if err := g.AssignCommunities(communities); err != nil {
		return nil, err
	}

	return g, nil
}

func (c *ClusterEngine) splitCommunity(graph KnowledgeGraph, nodeIds []string, maxSize int) [][]string {
	if len(nodeIds) <= maxSize {
		return [][]string{nodeIds}
	}
	nodeSet := map[string]struct{}{}
	for _, v := range nodeIds {
		nodeSet[v] = struct{}{}
	}

	// Build sub-adjacency: only edges that stay inside nodeSet.
	subAdjacency := map[string]map[string]float32{}
	subNodeDegree := map[string]float32{}
	var subTotalWeightDoubled float32 = 0.0

	for _, nodeId := range nodeIds {
		neighborWeights := map[string]float32{}
		var degree float32 = 0.0
		for _, edge := range graph.GetEdgesById(nodeId) {
			var neighborId = edge.Source.Id
			if edge.Source.Id == nodeId {
				neighborId = edge.Target.Id
			}

			if _, ok := nodeSet[neighborId]; !ok {
				continue
			}

			if existing, ok := neighborWeights[neighborId]; ok {
				neighborWeights[neighborId] = existing + float32(edge.Weight)
			} else {
				neighborWeights[neighborId] = float32(edge.Weight)
			}
			degree += float32(edge.Weight)
		}
		subAdjacency[nodeId] = neighborWeights
		subNodeDegree[nodeId] = degree
		subTotalWeightDoubled += degree
	}

	var subTotalWeight float32 = subTotalWeightDoubled / 2.0
	if subTotalWeight == 0.0 {
		return [][]string{nodeIds}
	}

	subNodeToCommunity := map[string]int{}
	subCommunityTotalDegree := map[int]float32{}
	for i := range nodeIds {
		id := nodeIds[i]
		subNodeToCommunity[id] = i
		subCommunityTotalDegree[i] = subNodeDegree[id]
	}

	var m2 float32 = 2.0 * subTotalWeight
	var m2Sq float32 = m2 * m2

	improved := true
	iteration := 0
	maxIter := min(50, c.options.MaxIterations)
	edgesToCommunity := map[int]float32{}

	for {
		if !improved || iteration >= maxIter {
			break
		}

		improved = false
		iteration++

		for _, nodeId := range nodeIds {
			var currentCommunity = subNodeToCommunity[nodeId]
			var nDegree = subNodeDegree[nodeId]
			var neighbors = subAdjacency[nodeId]

			edgesToCommunity = map[int]float32{}
			for neighborId, weight := range neighbors {
				if neighborId == nodeId {
					continue
				}
				var neighborCommunity = subNodeToCommunity[neighborId]
				if w, ok := edgesToCommunity[neighborCommunity]; ok {

					edgesToCommunity[neighborCommunity] = w + weight
				} else {
					edgesToCommunity[neighborCommunity] = weight
				}
			}

			var edgesToCurrent float32 = 0
			if ec, ok := edgesToCommunity[currentCommunity]; ok {
				edgesToCurrent = ec
			}
			var currentTotal float32 = subCommunityTotalDegree[currentCommunity]

			bestCommunity := currentCommunity
			var bestGain float32 = 0.0

			for targetCommunity, edgesToTarget := range edgesToCommunity {

				if targetCommunity == currentCommunity {
					continue
				}

				var targetTotal float32 = subCommunityTotalDegree[targetCommunity]
				var deltaQ float32 = c.options.Resolution * ((edgesToTarget-edgesToCurrent)/m2 +
					(currentTotal-targetTotal-nDegree)*nDegree/m2Sq)

				if deltaQ > bestGain {
					bestGain = deltaQ
					bestCommunity = targetCommunity
				}
			}

			if bestCommunity != currentCommunity {
				subCommunityTotalDegree[currentCommunity] -= nDegree
				subCommunityTotalDegree[bestCommunity] += nDegree
				subNodeToCommunity[nodeId] = bestCommunity
				improved = true
			}
		}
	}

	subCommunities := map[int][]string{}
	for nodeId, communityId := range subNodeToCommunity {

		if _, ok := subCommunities[communityId]; !ok {
			subCommunities[communityId] = []string{}
		}
		subCommunities[communityId] = append(subCommunities[communityId], nodeId)
	}

	result := [][]string{}
	for _, v := range subCommunities {
		result = append(result, v)
	}
	return result
}

func (c *ClusterEngine) detectCommunities(graph KnowledgeGraph) map[int][]string {
	if graph.EdgeCount() == 0 {
		// No edges - each node is its own community
		isolatedCommunities := map[int][]string{}
		isolatedNodeIds := []string{}
		for _, v := range graph.GetNodes() {
			isolatedNodeIds = append(isolatedNodeIds, v.Id)
		}

		slices.Sort(isolatedNodeIds)

		for i := 0; i < len(isolatedNodeIds); i++ {
			isolatedCommunities[i] = []string{isolatedNodeIds[i]}
		}
		return isolatedCommunities
	}

	// Phase 1: Initialize - each node is its own community
	nodes := graph.GetNodes()
	nodeCount := len(nodes)

	// Precompute weighted adjacency (neighbor -> aggregated edge weight) and per-node degree.
	// This collapses parallel edges and lets the Louvain main loop run in O(E) per iteration
	// instead of re-walking the whole graph for each gain evaluation.
	adjacency := map[string]map[string]float32{}
	nodeDegree := map[string]float32{}
	var totalEdgeWeight float32 = 0
	for _, node := range nodes {
		neighborWeights := map[string]float32{}
		var degree float32 = 0.0
		for _, edge := range graph.GetEdgesById(node.Id) {
			var neighborId = edge.Source.Id
			if edge.Source.Id == node.Id {
				neighborId = edge.Target.Id
			}

			if existing, ok := neighborWeights[neighborId]; ok {
				neighborWeights[neighborId] = existing + float32(edge.Weight)
			} else {
				neighborWeights[neighborId] = float32(edge.Weight)
			}
		}
		adjacency[node.Id] = neighborWeights
		nodeDegree[node.Id] = degree
		totalEdgeWeight += degree
	}

	var m2 float32 = totalEdgeWeight
	var m2Sq float32 = m2 * m2

	nodeToCommunity := map[string]int{}
	communityTotalDegree := map[int]float32{}
	for i := range nodeCount {
		id := nodes[i].Id
		nodeToCommunity[id] = i
		communityTotalDegree[i] = nodeDegree[id]
	}

	improved := true
	iteration := 0
	edgesToCommunity := map[int]float32{}

	for {
		if !improved || iteration >= c.options.MaxIterations {
			break
		}
		improved = false
		iteration++

		for _, node := range nodes {
			nodeId := node.Id
			currentCommunity := nodeToCommunity[nodeId]
			nDegree := nodeDegree[nodeId]
			neighbors := adjacency[nodeId]

			// Aggregate edge weight from this node into each neighboring community in one pass.
			edgesToCommunity = map[int]float32{}
			for neighborId, weight := range neighbors {
				if neighborId == nodeId {
					continue
				}
				var neighborCommunity = nodeToCommunity[neighborId]
				if w, ok := edgesToCommunity[neighborCommunity]; ok {

					edgesToCommunity[neighborCommunity] = w + weight
				} else {
					edgesToCommunity[neighborCommunity] = weight
				}
			}

			var edgesToCurrent float32 = 0
			if ec, ok := edgesToCommunity[currentCommunity]; ok {
				edgesToCurrent = ec
			}
			var currentTotal float32 = edgesToCommunity[currentCommunity]

			bestCommunity := currentCommunity
			var bestGain float32 = 0.0

			for targetCommunity, edgesToTarget := range edgesToCommunity {
				if targetCommunity == currentCommunity {
					continue
				}

				var targetTotal float32 = communityTotalDegree[targetCommunity]
				var deltaQ float32 = c.options.Resolution * ((edgesToTarget-edgesToCurrent)/m2 +
					(currentTotal-targetTotal-nDegree)*nDegree/m2Sq)

				if deltaQ > bestGain {
					bestGain = deltaQ
					bestCommunity = targetCommunity
				}
			}

			if bestCommunity != currentCommunity {
				communityTotalDegree[currentCommunity] -= nDegree
				communityTotalDegree[bestCommunity] += nDegree
				nodeToCommunity[nodeId] = bestCommunity
				improved = true
			}
		}
	}

	// Group nodes by community
	rawCommunities := map[int][]string{}
	for nodeId, communityId := range nodeToCommunity {
		if _, ok := rawCommunities[communityId]; !ok {
			rawCommunities[communityId] = []string{}
		}
		rawCommunities[communityId] = append(rawCommunities[communityId], nodeId)
	}

	// Split oversized communities
	finalCommunities := [][]string{}

	maxSize := max(c.options.MinSplitSize, graph.NodeCount()*int(c.options.MaxCommunityFraction))

	for _, communityNodes := range rawCommunities {
		if len(communityNodes) > maxSize {
			a := c.splitCommunity(graph, communityNodes, maxSize)
			finalCommunities = append(finalCommunities, a...)
		} else {
			finalCommunities = append(finalCommunities, communityNodes)
		}
	}

	// Sort by size descending and re-index
	slices.SortFunc(finalCommunities, func(a []string, b []string) int {
		return len(b) - len(a)
	})

	result := map[int][]string{}
	for i := 0; i < len(finalCommunities); i++ {
		t := finalCommunities[i]
		slices.Sort(t)
		result[i] = t
	}

	return result
}
