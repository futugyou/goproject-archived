package graphify

import (
	"fmt"
	"sort"
	"time"

	"github.com/dominikbraun/graph"
)

type AnalysisResult struct {
	GodNodes              []GodNode
	SurprisingConnections []SurprisingConnection
	SuggestedQuestions    []SuggestedQuestion
	Statistics            GraphStatistics
}
type GodNode struct {
	Id        string
	Label     string
	EdgeCount int
}

type SurprisingConnection struct {
	Source       string
	Target       string
	SourceFiles  []string
	Relationship string
	Confidence   Confidence
	Why          string
}

type SuggestedQuestion struct {
	Type     string
	Question string
	Why      string
}

type GraphStatistics struct {
	NodeCount         int
	EdgeCount         int
	CommunityCount    int
	AverageDegree     float32
	IsolatedNodeCount int
}

type Confidence string

const (
	ConfidenceExtracted Confidence = "Extracted"
	ConfidenceInferred  Confidence = "Inferred"
	ConfidenceAmbiguous Confidence = "Ambiguous"
)

type DetectedFile struct {
	FilePath     string
	FileName     string
	Extension    string
	Language     string
	Category     FileCategory
	SizeBytes    int64
	RelativePath string
}

type FileCategory string

const (
	FileCategoryCode          FileCategory = "Code"
	FileCategoryDocumentation FileCategory = "Documentation"
	FileCategoryMedia         FileCategory = "Media"
)

type ExtractedEdge struct {
	Source         string
	Target         string
	Relation       string
	Confidence     Confidence
	SourceFile     string
	SourceLocation string
	Weight         float32
}

type ExtractedNode struct {
	Id             string
	Label          string
	FileType       FileType
	SourceFile     string
	SourceLocation string
	Metadata       map[string]any
}

type FileType string

const (
	FileTypeCode     FileType = "Code"
	FileTypeDocument FileType = "Document"
	FileTypePaper    FileType = "Paper"
	FileTypeImage    FileType = "Image"
)

type ExtractionMethod string

const (
	ExtractionMethodAst      ExtractionMethod = "Ast"
	ExtractionMethodSemantic ExtractionMethod = "Semantic"
	ExtractionMethodHybrid   ExtractionMethod = "Hybrid"
)

type ExtractionResult struct {
	Nodes                  []ExtractedNode
	Edges                  []ExtractedEdge
	RawText                string
	SourceFilePath         string
	RelativeSourceFilePath string
	Method                 ExtractionMethod
	Timestamp              time.Time
	ConfidenceScores       map[string]float32
}

type GraphEdge struct {
	Source       GraphNode
	Target       GraphNode
	Relationship string
	Weight       int
	Confidence   Confidence
	Metadata     map[string]string
}

func (g GraphEdge) ToEdgeProperties() []func(*graph.EdgeProperties) {
	result := []func(*graph.EdgeProperties){
		graph.EdgeData(g),
		graph.EdgeAttribute("Relationship", g.Relationship),
		graph.EdgeAttribute("Confidence", string(g.Confidence)),
		graph.EdgeWeight(g.Weight),
	}

	for k, v := range g.Metadata {
		result = append(result, graph.EdgeAttribute(k, v))
	}

	return result
}

type ViEdge struct {
	From       string    `json:"from"`
	To         string    `json:"to"`
	Label      string    `json:"label"`
	Title      string    `json:"title"`
	Dashes     bool      `json:"dashes"`
	Width      int       `json:"width"`
	Color      EdgeColor `json:"color"`
	Confidence string    `json:"confidence"`
}

type EdgeColor struct {
	Opacity float64 `json:"opacity"`
}

type VisNode struct {
	ID            string      `json:"id"`
	Label         string      `json:"label"`
	Color         ColorConfig `json:"color"`
	Size          float64     `json:"size"`
	Font          FontConfig  `json:"font"`
	Title         string      `json:"title"`
	Community     int         `json:"community"`
	CommunityName string      `json:"community_name"`
	SourceFile    string      `json:"source_file"`
	FileType      string      `json:"file_type"`
	Degree        int         `json:"degree"`
}

type ColorConfig struct {
	Background string         `json:"background"`
	Border     string         `json:"border"`
	Highlight  HighlightColor `json:"highlight"`
}

type FontConfig struct {
	Size  int    `json:"size"`
	Color string `json:"color"`
}

type HighlightColor struct {
	Background string `json:"background"`
	Border     string `json:"border"`
}

type LegendData struct {
	Cid   int    `json:"cid"`
	Color string `json:"color"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type GraphNode struct {
	Id           string
	Label        string
	Type         string
	FilePath     string
	RelativePath string
	Language     string
	Confidence   Confidence
	Community    int
	Metadata     map[string]string
}

func (k *GraphNode) LabelOrID() string {
	if len(k.Label) > 0 {
		return k.Label
	}

	return k.Id
}

func (g *GraphNode) ToVertexProperties() []func(*graph.VertexProperties) {
	result := []func(*graph.VertexProperties){
		graph.VertexAttribute("Id", g.Id),
		graph.VertexAttribute("Label", g.Label),
		graph.VertexAttribute("Type", g.Type),
		graph.VertexAttribute("FilePath", g.FilePath),
		graph.VertexAttribute("RelativePath", g.RelativePath),
		graph.VertexAttribute("Language", g.Language),
		graph.VertexAttribute("Confidence", string(g.Confidence)),
		graph.VertexAttribute("Community", fmt.Sprintf("%d", g.Community)),
	}

	for k, v := range g.Metadata {
		result = append(result, graph.VertexAttribute(k, v))
	}

	return result
}

type GraphReport struct {
	Title           string
	Summary         string
	Communities     []Community
	GodNodes        []GodNode
	SurprisingEdges []SurprisingConnection
	GeneratedAt     time.Time
	Statistics      *GraphStatistics
}

type Community struct {
	Id            int
	Label         string
	Members       []string
	CohesionScore float32
}

type KnowledgeGraph struct {
	graph     graph.Graph[string, GraphNode]
	nodeIndex map[string]GraphNode
}

type NodeDegree struct {
	Node   GraphNode
	Degree int
}

func (k *KnowledgeGraph) GetHighestDegreeNodes(topN int) []NodeDegree {
	amap, err := k.graph.AdjacencyMap()
	if err != nil {
		return []NodeDegree{}
	}

	results := make([]NodeDegree, 0, len(amap))

	for id := range amap {
		d := k.GetDegree(id)

		node, err := k.graph.Vertex(id)
		if err != nil {
			continue
		}

		results = append(results, NodeDegree{
			Node:   node,
			Degree: d,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Degree > results[j].Degree
	})

	if len(results) > topN {
		results = results[:topN]
	}

	return results
}

var nodeHash = func(n GraphNode) string { return n.Id }

func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		graph:     graph.New(nodeHash, graph.Directed()),
		nodeIndex: map[string]GraphNode{},
	}
}

func (k *KnowledgeGraph) NodeCount() int {
	count, _ := k.graph.Order()
	return count
}

func (k *KnowledgeGraph) EdgeCount() int {
	count, _ := k.graph.Size()
	return count
}

func (k *KnowledgeGraph) AddNode(node GraphNode) {
	k.graph.AddVertex(node, node.ToVertexProperties()...)
}

func (k *KnowledgeGraph) AddEdge(edge GraphEdge) {
	k.graph.AddEdge(edge.Source.Id, edge.Target.Id, edge.ToEdgeProperties()...)
}

func (k *KnowledgeGraph) GetNeighbors(id string) []GraphNode {
	result := []GraphNode{}
	amap, _ := k.graph.AdjacencyMap()
	if outEdges, ok := amap[id]; ok {
		for targetID := range outEdges {
			n, _ := k.graph.Vertex(targetID)
			result = append(result, n)
		}
	}

	return result
}

func (k *KnowledgeGraph) GetEdges() []GraphEdge {
	result := []GraphEdge{}
	edges, _ := k.graph.Edges()

	for _, v := range edges {
		if e, ok := v.Properties.Data.(GraphEdge); ok {
			result = append(result, e)
		}
	}

	return result
}

func (k *KnowledgeGraph) GetEdgesById(id string) []GraphEdge {
	result := []GraphEdge{}
	targets, _ := k.graph.AdjacencyMap()
	for _, target := range targets[id] {
		if e, ok := target.Properties.Data.(GraphEdge); ok {
			result = append(result, e)
		}
	}

	sources, _ := k.graph.PredecessorMap()
	for _, source := range sources[id] {
		if e, ok := source.Properties.Data.(GraphEdge); ok {
			result = append(result, e)
		}
	}

	return result
}

func (k *KnowledgeGraph) GetNodes() []GraphNode {
	amap, err := k.graph.AdjacencyMap()
	if err != nil {
		return []GraphNode{}
	}

	nodes := make([]GraphNode, 0, len(amap))

	for id := range amap {
		node, err := k.graph.Vertex(id)
		if err == nil {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func (k *KnowledgeGraph) GetNodesById(id string) (GraphNode, error) {
	return k.graph.Vertex(id)
}

func (k *KnowledgeGraph) GetDegree(id string) int {
	amap, _ := k.graph.AdjacencyMap()
	pmap, _ := k.graph.PredecessorMap()

	return len(amap[id]) + len(pmap[id])
}

func (k *KnowledgeGraph) AssignCommunities(communities map[int][]string) error {
	if communities == nil {
		return fmt.Errorf("communities map is nil")
	}

	for communityID, nodeIDs := range communities {
		for _, nodeID := range nodeIDs {
			oldNode, err := k.graph.Vertex(nodeID)
			if err != nil {
				continue
			}

			updatedNode := oldNode
			updatedNode.Community = communityID

			err = k.graph.AddVertex(updatedNode)
			if err != nil {
				return fmt.Errorf("failed to update node %s: %w", nodeID, err)
			}

			if k.nodeIndex != nil {
				k.nodeIndex[nodeID] = updatedNode
			}
		}
	}

	return nil
}

func (k *KnowledgeGraph) GetNodesByCommunity(communityId int) []GraphNode {
	amap, err := k.graph.AdjacencyMap()
	if err != nil {
		return []GraphNode{}
	}

	nodes := make([]GraphNode, 0, len(amap))

	for id := range amap {
		node, err := k.graph.Vertex(id)
		if err == nil || node.Community == communityId {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func (k *KnowledgeGraph) MergeGraph(other KnowledgeGraph) error {
	for _, node := range other.GetNodes() {
		k.AddNode(node)
	}
	for _, edge := range other.GetEdges() {
		k.AddEdge(edge)
	}

	return nil
}

type GraphExportDto struct {
	Nodes    []NodeDto         `json:"nodes"`
	Edges    []EdgeDto         `json:"edges"`
	Metadata ExportMetadataDto `json:"metadata"`
}

type NodeDto struct {
	Id         string            `json:"id"`
	Label      string            `json:"label"`
	Type       string            `json:"type"`
	Community  int               `json:"community"`
	FilePath   string            `json:"file_path"`
	Language   string            `json:"language"`
	Confidence string            `json:"confidence"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type EdgeDto struct {
	Source       string            `json:"source"`
	Target       string            `json:"target"`
	Relationship string            `json:"relationship"`
	Weight       float64           `json:"weight"`
	Confidence   string            `json:"confidence"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type ExportMetadataDto struct {
	NodeCount      int       `json:"node_count"`
	EdgeCount      int       `json:"edge_count"`
	CommunityCount int       `json:"community_count"`
	GeneratedAt    time.Time `json:"generated_at"`
}

type ConnectionEntry struct {
	LinkedNode GraphNode
	Confidence Confidence
}
