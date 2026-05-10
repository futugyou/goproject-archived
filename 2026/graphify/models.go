package graphify

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
