package graphify

// GraphView defines the minimal topological information interface required for the execution of the Louvain algorithm.
type LouvainGraphView[ID comparable] interface {
	// Nodes returns the unique identifiers of all nodes in the graph.
	Nodes() []ID
	// EdgesFrom returns information on all outgoing/adjacent edges of the specified node.
	EdgesFrom(id ID) []LouvainEdge[ID]
}

// Edge: Abstract Edge Information
type LouvainEdge[ID comparable] struct {
	Source ID
	Target ID
	Weight float32
}

// Configure Algorithm Hyperparameters
type LouvainConfig struct {
	Resolution    float32
	MaxIterations int
}

// Engine is a general-purpose Louvain community detection engine.
type LouvainEngine[ID comparable] struct {
	config LouvainConfig
}

func NewLouvainEngine[ID comparable](cfg LouvainConfig) *LouvainEngine[ID] {
	if cfg.Resolution <= 0 {
		cfg.Resolution = 1.0
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 20
	}
	return &LouvainEngine[ID]{config: cfg}
}

// ctx: Internal computation context
type ctx[ID comparable] struct {
	adjacency            map[ID]map[ID]float32
	nodeDegree           map[ID]float32
	communityTotalDegree map[int]float32
	nodeToCommunity      map[ID]int
	m2                   float32
	m2Sq                 float32
}

// Execute the first-stage single-layer modularity optimization.
func (e *LouvainEngine[ID]) Execute(g LouvainGraphView[ID], subset []ID, filter map[ID]struct{}) map[int][]ID {
	nodes := subset
	if len(nodes) == 0 {
		nodes = g.Nodes()
	}

	// 1. Initialize Topology Context
	c := e.buildContext(g, nodes, filter)
	if c.m2 == 0.0 {
		// With no edges, it degenerates into a set of isolated communities.
		return e.fallbackIsolated(nodes)
	}

	// 2. Greedy Iterative Modularity Optimization
	improved := true
	iteration := 0
	edgesToCommunity := map[int]float32{}

	for improved && iteration < e.config.MaxIterations {
		improved = false
		iteration++

		for _, nodeId := range nodes {
			currentCommunity := c.nodeToCommunity[nodeId]
			nDegree := c.nodeDegree[nodeId]
			neighbors := c.adjacency[nodeId]

			clear(edgesToCommunity)
			for neighborId, weight := range neighbors {
				if neighborId == nodeId {
					continue
				}
				edgesToCommunity[c.nodeToCommunity[neighborId]] += weight
			}

			edgesToCurrent := edgesToCommunity[currentCommunity]
			currentTotal := c.communityTotalDegree[currentCommunity]

			bestCommunity := currentCommunity
			var bestGain float32 = 0.0

			for targetCommunity, edgesToTarget := range edgesToCommunity {
				if targetCommunity == currentCommunity {
					continue
				}

				targetTotal := c.communityTotalDegree[targetCommunity]
				deltaQ := e.config.Resolution * ((edgesToTarget-edgesToCurrent)/c.m2 +
					(currentTotal-targetTotal-nDegree)*nDegree/c.m2Sq)

				if deltaQ > bestGain {
					bestGain = deltaQ
					bestCommunity = targetCommunity
				}
			}

			if bestCommunity != currentCommunity {
				c.communityTotalDegree[currentCommunity] -= nDegree
				c.communityTotalDegree[bestCommunity] += nDegree
				c.nodeToCommunity[nodeId] = bestCommunity
				improved = true
			}
		}
	}

	// 3. Collect and Output Clustering Results
	rawCommunities := map[int][]ID{}
	for nodeId, communityId := range c.nodeToCommunity {
		rawCommunities[communityId] = append(rawCommunities[communityId], nodeId)
	}
	return rawCommunities
}

func (e *LouvainEngine[ID]) buildContext(g LouvainGraphView[ID], nodes []ID, filter map[ID]struct{}) *ctx[ID] {
	adj := map[ID]map[ID]float32{}
	degrees := map[ID]float32{}
	var totalWeight float32 = 0.0

	for _, nodeId := range nodes {
		neighborWeights := map[ID]float32{}
		var degree float32 = 0.0

		for _, edge := range g.EdgesFrom(nodeId) {
			neighborId := edge.Source
			if edge.Source == nodeId {
				neighborId = edge.Target
			}

			if filter != nil {
				if _, ok := filter[neighborId]; !ok {
					continue
				}
			}

			neighborWeights[neighborId] += edge.Weight
			degree += edge.Weight
		}

		adj[nodeId] = neighborWeights
		degrees[nodeId] = degree
		totalWeight += degree
	}

	nodeToComm := map[ID]int{}
	commTotalDegree := map[int]float32{}
	for i, id := range nodes {
		nodeToComm[id] = i
		commTotalDegree[i] = degrees[id]
	}

	return &ctx[ID]{
		adjacency:            adj,
		nodeDegree:           degrees,
		communityTotalDegree: commTotalDegree,
		nodeToCommunity:      nodeToComm,
		m2:                   totalWeight,
		m2Sq:                 totalWeight * totalWeight,
	}
}

func (e *LouvainEngine[ID]) fallbackIsolated(nodes []ID) map[int][]ID {
	res := map[int][]ID{}
	for i, id := range nodes {
		res[i] = []ID{id}
	}
	return res
}
