package graphify

import (
	"fmt"
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
