package graphify

import (
	"context"
	"slices"
)

var _ IPipelineStage[KnowledgeGraph, KnowledgeGraph] = (*ClusterEngine)(nil)

type ClusterEngine struct {
	options *ClusterOptions
}

func NewClusterEngine(options *ClusterOptions) *ClusterEngine {
	if options == nil {
		options = DefaultClusterOptions()
	}
	return &ClusterEngine{options: options}
}

func (c *ClusterEngine) Execute(ctx context.Context, graph *KnowledgeGraph) (*KnowledgeGraph, error) {
	if graph.NodeCount() == 0 {
		return graph, nil
	}

	communities := c.detectCommunities(graph)
	if err := graph.AssignCommunities(communities); err != nil {
		return nil, err
	}

	return graph, nil
}

type graphContext struct {
	adjacency            map[string]map[string]float32
	nodeDegree           map[string]float32
	communityTotalDegree map[int]float32
	nodeToCommunity      map[string]int
	m2                   float32
	m2Sq                 float32
}

// Consolidate the logic for extracting and constructing local/global adjacency lists and degree information.
func (c *ClusterEngine) buildGraphContext(graph *KnowledgeGraph, nodeIds []string, filterSet map[string]struct{}) *graphContext {
	adj := map[string]map[string]float32{}
	degrees := map[string]float32{}
	var totalWeightDoubled float32 = 0.0

	for _, nodeId := range nodeIds {
		neighborWeights := map[string]float32{}
		var degree float32 = 0.0

		for _, edge := range graph.GetEdgesById(nodeId) {
			neighborId := edge.Source.Id
			if edge.Source.Id == nodeId {
				neighborId = edge.Target.Id
			}

			// If a restricted set is provided (Split scenario), filter out external nodes.
			if filterSet != nil {
				if _, ok := filterSet[neighborId]; !ok {
					continue
				}
			}

			neighborWeights[neighborId] += float32(edge.Weight)
			degree += float32(edge.Weight)
		}

		adj[nodeId] = neighborWeights
		degrees[nodeId] = degree
		totalWeightDoubled += degree
	}

	m2 := totalWeightDoubled // If it is an undirected graph, the global accumulation of bidirectional edges is 2m.
	nodeToComm := map[string]int{}
	commTotalDegree := map[int]float32{}
	for i, id := range nodeIds {
		nodeToComm[id] = i
		commTotalDegree[i] = degrees[id]
	}

	return &graphContext{
		adjacency:            adj,
		nodeDegree:           degrees,
		communityTotalDegree: commTotalDegree,
		nodeToCommunity:      nodeToComm,
		m2:                   m2,
		m2Sq:                 m2 * m2,
	}
}

// Core Greedy Node Movement Algorithm (Merged Single-Layer Louvain Iteration Loop)
func (c *ClusterEngine) optimizeModularity(nodeIds []string, gCtx *graphContext, maxIter int) {
	if gCtx.m2 == 0.0 {
		return
	}

	improved := true
	iteration := 0
	edgesToCommunity := map[int]float32{}

	for improved && iteration < maxIter {
		improved = false
		iteration++

		for _, nodeId := range nodeIds {
			currentCommunity := gCtx.nodeToCommunity[nodeId]
			nDegree := gCtx.nodeDegree[nodeId]
			neighbors := gCtx.adjacency[nodeId]

			// Clear and recalculate the weights of the current node to each community.
			clear(edgesToCommunity)
			for neighborId, weight := range neighbors {
				if neighborId == nodeId {
					continue
				}
				neighborCommunity := gCtx.nodeToCommunity[neighborId]
				edgesToCommunity[neighborCommunity] += weight
			}

			edgesToCurrent := edgesToCommunity[currentCommunity]
			currentTotal := gCtx.communityTotalDegree[currentCommunity]

			bestCommunity := currentCommunity
			var bestGain float32 = 0.0

			for targetCommunity, edgesToTarget := range edgesToCommunity {
				if targetCommunity == currentCommunity {
					continue
				}

				targetTotal := gCtx.communityTotalDegree[targetCommunity]
				// Incremental Modularity Formula (Delta Q)
				deltaQ := c.options.Resolution * ((edgesToTarget-edgesToCurrent)/gCtx.m2 +
					(currentTotal-targetTotal-nDegree)*nDegree/gCtx.m2Sq)

				if deltaQ > bestGain {
					bestGain = deltaQ
					bestCommunity = targetCommunity
				}
			}

			if bestCommunity != currentCommunity {
				gCtx.communityTotalDegree[currentCommunity] -= nDegree
				gCtx.communityTotalDegree[bestCommunity] += nDegree
				gCtx.nodeToCommunity[nodeId] = bestCommunity
				improved = true
			}
		}
	}
}

// Perform subgraph re-aggregation using the refactored helper functions.
func (c *ClusterEngine) splitCommunity(graph *KnowledgeGraph, nodeIds []string, maxSize int) [][]string {
	if len(nodeIds) <= maxSize {
		return [][]string{nodeIds}
	}

	nodeSet := map[string]struct{}{}
	for _, v := range nodeIds {
		nodeSet[v] = struct{}{}
	}

	gCtx := c.buildGraphContext(graph, nodeIds, nodeSet)
	maxIter := min(50, c.options.MaxIterations)
	c.optimizeModularity(nodeIds, gCtx, maxIter)

	subCommunities := map[int][]string{}
	for nodeId, communityId := range gCtx.nodeToCommunity {
		subCommunities[communityId] = append(subCommunities[communityId], nodeId)
	}

	result := make([][]string, 0, len(subCommunities))
	for _, v := range subCommunities {
		result = append(result, v)
	}
	return result
}

func (c *ClusterEngine) detectCommunities(graph *KnowledgeGraph) map[int][]string {
	nodes := graph.GetNodes()

	// Handling Edgeless Isolated Graphs
	if graph.EdgeCount() == 0 {
		isolatedNodeIds := make([]string, len(nodes))
		for i, v := range nodes {
			isolatedNodeIds[i] = v.Id
		}
		slices.Sort(isolatedNodeIds)

		isolatedCommunities := map[int][]string{}
		for i, id := range isolatedNodeIds {
			isolatedCommunities[i] = []string{id}
		}
		return isolatedCommunities
	}

	nodeIds := make([]string, len(nodes))
	for i, v := range nodes {
		nodeIds[i] = v.Id
	}

	// 1. Global Re-aggregation (Phase 1)
	gCtx := c.buildGraphContext(graph, nodeIds, nil)
	c.optimizeModularity(nodeIds, gCtx, c.options.MaxIterations)

	// Grouped Aggregation Results
	rawCommunities := map[int][]string{}
	for nodeId, communityId := range gCtx.nodeToCommunity {
		rawCommunities[communityId] = append(rawCommunities[communityId], nodeId)
	}

	// 2. Carving Up a Massive Community
	finalCommunities := [][]string{}
	maxSize := max(c.options.MinSplitSize, graph.NodeCount()*int(c.options.MaxCommunityFraction))

	for _, communityNodes := range rawCommunities {
		if len(communityNodes) > maxSize {
			finalCommunities = append(finalCommunities, c.splitCommunity(graph, communityNodes, maxSize)...)
		} else {
			finalCommunities = append(finalCommunities, communityNodes)
		}
	}

	// 3. Sort and Output in the Specified Format
	slices.SortFunc(finalCommunities, func(a, b []string) int {
		return len(b) - len(a)
	})

	result := map[int][]string{}
	for i, t := range finalCommunities {
		slices.Sort(t)
		result[i] = t
	}

	return result
}
