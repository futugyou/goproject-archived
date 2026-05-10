package graphify

import "time"

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
	Weight       float32
	Confidence   Confidence
	Metadata     map[string]string
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
