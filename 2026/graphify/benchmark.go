package graphify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

type GraphJsonDto struct {
	Nodes []NodeDto
	Edges []EdgeDto
}

const CharsPerToken int = 4

var SampleQuestions = []string{"how does authentication work",
	"what is the main entry point",
	"how are errors handled",
	"what connects the data layer to the api",
	"what are the core abstractions"}

type BenchmarkResult struct {
	Error          string
	CorpusTokens   int
	CorpusWords    int
	NodeCount      int
	EdgeCount      int
	AvgQueryTokens int
	ReductionRatio float64
	PerQuestion    []QuestionBenchmark
}

type QuestionBenchmark struct {
	Question    string
	QueryTokens int
	Reduction   float32
}

func EstimateQueryTokens(graph KnowledgeGraph, question string, depth *int) int {
	depths := 3
	if depth != nil {
		depths = *depth
	}

	words := strings.Fields(question)
	var terms []string
	for _, w := range words {
		if len(w) > 2 {
			terms = append(terms, strings.ToLower(w))
		}
	}

	if len(terms) == 0 {
		return 0
	}

	var scored []ScoredNode
	for _, node := range graph.GetNodes() {
		label := strings.ToLower(node.Label)
		score := 0
		for _, term := range terms {
			if strings.Contains(label, term) {
				score++
			}
		}
		if score > 0 {
			scored = append(scored, ScoredNode{Score: score, Id: node.Id})
		}
	}

	if len(scored) == 0 {
		return 0
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	limit := min(len(scored), 3)

	visited := make(map[string]struct{})
	frontier := make(map[string]struct{})
	var startNodes []string

	for i := range limit {
		id := scored[i].Id
		startNodes = append(startNodes, id)
		visited[id] = struct{}{}
		frontier[id] = struct{}{}
	}

	var edgesSeen []GraphEdge

	for i := 0; i < depths; i++ {
		nextFrontier := make(map[string]struct{})
		for nodeId := range frontier {
			for _, neighbor := range graph.GetNeighbors(nodeId) {
				if _, exists := visited[neighbor.Id]; !exists {
					nextFrontier[neighbor.Id] = struct{}{}

					for _, edge := range graph.GetEdgesById(nodeId) {
						if edge.Target.Id == neighbor.Id || edge.Source.Id == neighbor.Id {
							edgesSeen = append(edgesSeen, edge)
						}
					}
				}
			}
		}

		for id := range nextFrontier {
			visited[id] = struct{}{}
		}
		frontier = nextFrontier
	}

	var lines []string

	for nodeId := range visited {
		node, err := graph.GetNodesById(nodeId)
		if err == nil {
			filePath := node.FilePath
			var location string
			if node.Metadata != nil {
				location = node.Metadata["source_location"]
			}
			lines = append(lines, fmt.Sprintf("NODE %s src=%s loc=%s", node.Label, filePath, location))
		}
	}

	edgeSeenMap := make(map[string]struct{})

	for _, edge := range edgesSeen {
		edgeKey := fmt.Sprintf("%s-%s-%s", edge.Source.Id, edge.Relationship, edge.Target.Id)

		if _, seen := edgeSeenMap[edgeKey]; !seen {
			edgeSeenMap[edgeKey] = struct{}{}

			_, sourceVisited := visited[edge.Source.Id]
			_, targetVisited := visited[edge.Target.Id]

			if sourceVisited && targetVisited {
				lines = append(lines, fmt.Sprintf("EDGE %s --%s--> %s", edge.Source.Label, edge.Relationship, edge.Target.Label))
			}
		}
	}

	contextText := strings.Join(lines, "\n")
	return CharactersToTokens(contextText)
}

func CharactersToTokens(text string) int {
	return max(1, len(text)/CharsPerToken)
}
func EstimateCorpusWords(graph KnowledgeGraph) int {
	// Rough estimate: each node label is ~3 words, plus source context
	return graph.NodeCount() * 50
}

func WordsToTokens(words int) int {
	// Approximate conversion: 100 words ≈ 133 tokens
	return words * 100 / 75
}

type ScoredNode struct {
	Score int
	Id    string
}

func LoadGraphFromJson(data GraphJsonDto) KnowledgeGraph {
	var graph = KnowledgeGraph{}

	for _, nodeDto := range data.Nodes {
		var node = GraphNode{
			Id:        nodeDto.Id,
			Label:     nodeDto.Label,
			Type:      nodeDto.Type,
			FilePath:  nodeDto.FilePath,
			Community: nodeDto.Community,
			Metadata:  nodeDto.Metadata,
		}
		if len(node.Type) == 0 {
			node.Type = "Entity"
		}
		graph.AddNode(node)
	}

	for _, edgeDto := range data.Edges {
		sourceNode, err := graph.GetNodesById(edgeDto.Source)
		targetNode, err1 := graph.GetNodesById(edgeDto.Target)

		if err != nil && err1 != nil {
			var edge = GraphEdge{
				Source:       sourceNode,
				Target:       targetNode,
				Relationship: edgeDto.Relationship,
				Weight:       edgeDto.Weight,
				Metadata:     edgeDto.Metadata,
			}
			graph.AddEdge(edge)
		}
	}

	return graph
}

func PrintBenchmark(result *BenchmarkResult, output io.Writer) error {
	if result == nil || output == nil {
		return fmt.Errorf("nil argument passed to PrintBenchmark")
	}

	if result.Error != "" {
		fmt.Fprintf(output, "Benchmark error: %s\n", result.Error)
		return fmt.Errorf("Benchmark error: %s\n", result.Error)
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "graphify token reduction benchmark")
	fmt.Fprintln(output, strings.Repeat("─", 50))
	fmt.Fprintf(output, "  Corpus:          %d words → ~%d tokens (naive)\n", result.CorpusWords, result.CorpusTokens)
	fmt.Fprintf(output, "  Graph:           %d nodes, %d edges\n", result.NodeCount, result.EdgeCount)
	fmt.Fprintf(output, "  Avg query cost:  ~%d tokens\n", result.AvgQueryTokens)
	fmt.Fprintf(output, "  Reduction:       %.1fx fewer tokens per query\n", result.ReductionRatio)
	fmt.Fprintln(output)

	fmt.Fprintln(output, "  Per question:")
	for _, q := range result.PerQuestion {
		runes := []rune(q.Question)
		truncated := q.Question
		if len(runes) > 55 {
			truncated = string(runes[:52]) + "..."
		}

		fmt.Fprintf(output, "    [%.1fx] %s\n", q.Reduction, truncated)
	}
	fmt.Fprintln(output)
	return nil
}

func BenchmarkRun(graphPath string, corpusWords int, questions []string) (*BenchmarkResult, error) {
	if _, err := os.Stat(graphPath); errors.Is(err, os.ErrNotExist) {
		return &BenchmarkResult{
			Error: "Graph file not found:  " + graphPath,
		}, nil
	}

	// Load graph
	var graph KnowledgeGraph

	data, err := os.ReadFile(graphPath)
	if err != nil {
		return &BenchmarkResult{
			Error: "Failed to load graph:  " + err.Error(),
		}, nil
	}

	var dto GraphJsonDto
	err = json.Unmarshal(data, &dto)
	if err != nil {
		return &BenchmarkResult{
			Error: "Invalid graph JSON format",
		}, nil
	}
	graph = LoadGraphFromJson(dto)

	// Estimate corpus size
	estimatedCorpusWords := corpusWords
	if estimatedCorpusWords == -1 {
		estimatedCorpusWords = EstimateCorpusWords(graph)
	}
	corpusTokens := WordsToTokens(estimatedCorpusWords)

	// Run queries
	var questionList = questions
	if len(questionList) == 0 {
		questionList = SampleQuestions
	}
	perQuestionResults := []QuestionBenchmark{}

	for _, question := range questionList {
		queryTokens := EstimateQueryTokens(graph, question, nil)
		if queryTokens > 0 {
			reduction := float32(corpusTokens) / float32(queryTokens)
			perQuestionResults = append(perQuestionResults, QuestionBenchmark{
				Question:    question,
				QueryTokens: queryTokens,
				Reduction:   reduction,
			})

		}
	}

	if len(perQuestionResults) == 0 {
		return &BenchmarkResult{
			Error: "No matching nodes found for sample questions. Build the graph first.",
		}, nil
	}
	queryTokens := 0
	for _, v := range perQuestionResults {
		queryTokens += v.QueryTokens
	}
	avgQueryTokens := queryTokens / len(perQuestionResults)
	reductionRatio := corpusTokens / avgQueryTokens

	return &BenchmarkResult{
		CorpusTokens:   corpusTokens,
		CorpusWords:    estimatedCorpusWords,
		NodeCount:      graph.NodeCount(),
		EdgeCount:      graph.EdgeCount(),
		AvgQueryTokens: avgQueryTokens,
		ReductionRatio: math.Round(float64(reductionRatio*10)) / 10,
		PerQuestion:    perQuestionResults,
	}, nil
}
