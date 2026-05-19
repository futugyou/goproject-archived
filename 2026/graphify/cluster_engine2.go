package graphify

import (
	"context"
	"slices"
)

type louvainAdapter struct {
	graph KnowledgeGraph
}

func (a louvainAdapter) Nodes() []string {
	nodes := a.graph.GetNodes()
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.Id
	}
	return ids
}

func (a louvainAdapter) EdgesFrom(id string) []LouvainEdge[string] {
	edges := a.graph.GetEdgesById(id)
	res := make([]LouvainEdge[string], len(edges))
	for i, e := range edges {
		res[i] = LouvainEdge[string]{
			Source: e.Source.Id,
			Target: e.Target.Id,
			Weight: float32(e.Weight),
		}
	}
	return res
}

var _ IPipelineStage[KnowledgeGraph, KnowledgeGraph] = (*ClusterEngine2)(nil)

type ClusterEngine2 struct {
	options ClusterOptions
}

func NewClusterEngine2(options *ClusterOptions) *ClusterEngine2 {
	if options == nil {
		options = DefaultClusterOptions()
	}
	return &ClusterEngine2{options: *options}
}

func (c *ClusterEngine2) Execute(ctx context.Context, graph KnowledgeGraph) (*KnowledgeGraph, error) {
	if graph.NodeCount() == 0 {
		return &graph, nil
	}

	communities := c.detectCommunities(graph)
	g := &graph
	if err := g.AssignCommunities(communities); err != nil {
		return nil, err
	}

	return g, nil
}
func (c *ClusterEngine2) detectCommunities(graph KnowledgeGraph) map[int][]string {
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

	adapter := louvainAdapter{graph: graph}
	engine := NewLouvainEngine[string](LouvainConfig{
		Resolution:    c.options.Resolution,
		MaxIterations: c.options.MaxIterations,
	})

	// Global Clustering
	raw := engine.Execute(adapter, nil, nil)

	// Splitting Oversized Communities
	finalCommunities := [][]string{}
	maxSize := max(c.options.MinSplitSize, graph.NodeCount()*int(c.options.MaxCommunityFraction))

	for _, communityNodes := range raw {
		if len(communityNodes) > maxSize {
			// Subgraph splitting also directly reuses the core engine.
			filterSet := map[string]struct{}{}
			for _, v := range communityNodes {
				filterSet[v] = struct{}{}
			}
			// Pass in the local subset and filterSet to perform local optimization.
			subRaw := engine.Execute(adapter, communityNodes, filterSet)
			for _, subNodes := range subRaw {
				finalCommunities = append(finalCommunities, subNodes)
			}
		} else {
			finalCommunities = append(finalCommunities, communityNodes)
		}
	}

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
