package graphify

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
)

type PipelineRunner struct {
	output     io.Writer
	verbose    bool
	chatClient chatcompletion.IChatClient
}

func NewPipelineRunner(output io.Writer, verbose *bool, chatClient chatcompletion.IChatClient) *PipelineRunner {
	v := false
	if verbose != nil {
		v = *verbose
	}
	return &PipelineRunner{
		output:     output,
		verbose:    v,
		chatClient: chatClient,
	}
}

func (p *PipelineRunner) calculateCohesion(graph *KnowledgeGraph, nodeIds []string) float32 {
	if len(nodeIds) < 2 {
		return 0
	}

	nodeMap := make(map[string]struct{}, len(nodeIds))
	for _, id := range nodeIds {
		nodeMap[id] = struct{}{}
	}

	internalEdges := 0
	for _, nodeId := range nodeIds {
		edges := graph.GetEdgesById(nodeId)
		for _, edge := range edges {
			_, sourceExists := nodeMap[edge.Source.Id]
			_, targetExists := nodeMap[edge.Target.Id]
			if sourceExists || targetExists {
				internalEdges++
			}
		}
	}

	possibleEdges := len(nodeIds) * (len(nodeIds) - 1)
	return float32(internalEdges) / float32(possibleEdges)
}

func (p *PipelineRunner) calculateCohesionScores(graph *KnowledgeGraph) map[int]float32 {
	communities := make(map[int][]string)
	for _, node := range graph.GetNodes() {
		if node.Community != -1 {
			communities[node.Community] = append(communities[node.Community], node.Id)
		}
	}

	result := map[int]float32{}
	for commId, nodeIds := range communities {
		result[commId] = p.calculateCohesion(graph, nodeIds)
	}

	return result
}

func (p *PipelineRunner) buildCommunityLabels(graph *KnowledgeGraph) map[int]string {
	result := make(map[int]string)
	communities := make(map[int][]GraphNode)
	for _, node := range graph.GetNodes() {
		if node.Community != -1 {
			communities[node.Community] = append(communities[node.Community], node)
		}
	}

	for commId, nodes := range communities {
		ts := make(map[string][]GraphNode)
		for _, v := range nodes {
			ts[v.Type] = append(ts[v.Type], v)
		}
		tss := []commonType{}
		for k, v := range ts {
			tss = append(tss, commonType{
				t:     k,
				nodes: v,
			})
		}

		slices.SortFunc(tss, func(a, b commonType) int {
			return cmp.Compare(len(b.nodes), len(a.nodes))
		})

		commonType := "Mixed"
		if len(tss) > 0 && len(tss[0].t) > 0 {
			commonType = tss[0].t
		}

		result[commId] = fmt.Sprintf("%s (Community %d)", commonType, commId)
	}

	return result
}

type commonType struct {
	nodes []GraphNode
	t     string
}

func (p *PipelineRunner) writeLine(m string) {
	p.output.Write([]byte(m + "\n"))
}

func (p *PipelineRunner) writeErrorLine(err error) {
	p.writeLine("")
	p.writeLine("Error" + err.Error())
}

func (p *PipelineRunner) Run(ctx context.Context, inputPath, outputDir string, formats []string, useCache bool) (*KnowledgeGraph, error) {
	p.writeLine("graphify-dotnet: Transform codebases into knowledge graphs")
	p.writeLine(strings.Repeat("─", 60))
	p.writeLine("")

	// Stage 1: Detect files
	p.writeLine("[1/6] Detecting files...")
	fileDetector := &FileDetector{}
	detectorOptions := FileDetectorOptions{
		RootPath:         inputPath,
		MaxFileSizeBytes: 1024 * 1024,
		RespectGitIgnore: true,
	}

	dfs, err := fileDetector.Execute(ctx, detectorOptions)
	if err != nil {
		p.writeErrorLine(err)
		return nil, err
	}
	detectedFiles := *dfs
	p.writeLine(fmt.Sprintf("      Found %d files to process", len(detectedFiles)))

	if p.verbose {
		for i := 0; i < min(5, len(detectedFiles)); i++ {
			p.writeLine(fmt.Sprintf("        - %s (%s)", detectedFiles[i].RelativePath, detectedFiles[i].Language))
		}

		if len(detectedFiles) > 5 {
			p.writeLine(fmt.Sprintf("        ... and %d more", len(detectedFiles)-5))
		}
	}
	p.writeLine("")

	// Stage 2: Extract nodes and edges
	p.writeLine("[2/6] Extracting code structure...")
	extractor := NewSourceExtractor()
	var processed int32 = 0
	var skipped int32 = 0
	var extractionResults []ExtractionResult
	var bagMu sync.Mutex

	var verboseWarnings []string
	var warningsMu sync.Mutex

	maxDegreeOfParallelism := runtime.NumCPU()
	sem := make(chan struct{}, maxDegreeOfParallelism)
	var wg sync.WaitGroup

	for _, file := range detectedFiles {
		if ctx.Err() != nil {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(f DetectedFile) {
			defer func() {
				<-sem
				wg.Done()
			}()

			result, err := extractor.Execute(ctx, f)
			if err != nil {
				atomic.AddInt32(&skipped, 1)
				if p.verbose {
					warningsMu.Lock()
					verboseWarnings = append(verboseWarnings, fmt.Sprintf("      Warning: Failed to extract %s: %v", f.RelativePath, err))
					warningsMu.Unlock()
				}
				return
			}

			if len(result.Nodes) > 0 || len(result.Edges) > 0 {
				bagMu.Lock()
				extractionResults = append(extractionResults, *result)
				bagMu.Unlock()

				atomic.AddInt32(&processed, 1)
			} else {
				atomic.AddInt32(&skipped, 1)
			}
		}(file)
	}

	wg.Wait()

	p.writeLine(fmt.Sprintf("      Processed %d files, skipped %d", processed, skipped))
	totalNodes := 0
	totalEdges := 0
	for _, v := range extractionResults {
		totalNodes += len(v.Nodes)
		totalEdges += len(v.Edges)
	}
	p.writeLine(fmt.Sprintf("      Extracted %d nodes, %d edges", totalNodes, totalEdges))
	p.writeLine("")

	// Stage 2b: AI-enhanced semantic extraction (if provider configured)
	if p.chatClient != nil {
		p.writeLine("[2b/6] Running AI-enhanced semantic extraction...")
		semanticExtractor := NewSemanticExtractor(nil, p.chatClient)
		semanticProcessed := 0

		for _, file := range detectedFiles {
			if ctx.Err() != nil {
				p.writeLine(ctx.Err().Error())
				return nil, ctx.Err()
			}

			if result, err := semanticExtractor.Execute(ctx, file); err != nil {
				if p.verbose {
					p.writeLine(fmt.Sprintf("      Warning: Semantic extraction failed for %s: %s", file.RelativePath, err.Error()))
				}
			} else if len(result.Nodes) > 0 || len(result.Edges) > 0 {
				extractionResults = append(extractionResults, *result)
				semanticProcessed++
			}
		}

		p.writeLine("      AI extracted from {semanticProcessed} files")
		for _, v := range extractionResults {
			totalNodes += len(v.Nodes)
			totalEdges += len(v.Edges)
		}
		p.writeLine(fmt.Sprintf("      Total: %d nodes, %d edges (AST + AI)", totalNodes, totalEdges))
		p.writeLine("")
	} else {
		p.writeLine("      \u2139 No AI provider configured. Using AST-only extraction.")
		p.writeLine("        Use --provider to enable AI-enhanced semantic extraction.")
		p.writeLine("")
	}

	// Stage 3: Build graph
	p.writeLine("[3/6] Building knowledge graph...")
	graphBuilder := NewGraphBuilder(&GraphBuilderOptions{
		CreateFileNodes: true,
		MinEdgeWeight:   0.1,
		MergeStrategy:   MergeStrategyMostRecent,
	})
	graph, err := graphBuilder.Execute(ctx, extractionResults)
	if err != nil {
		p.writeErrorLine(err)
		return nil, err
	}
	p.writeLine(fmt.Sprintf("      Graph: %d nodes, %d edges", graph.NodeCount(), graph.EdgeCount()))
	p.writeLine("")

	// Stage 4: Detect communities (clustering)
	p.writeLine("[4/6] Detecting communities...")
	var clusterEngine = NewClusterEngine(&ClusterOptions{
		MaxIterations:        100,
		Resolution:           1.0,
		MinSplitSize:         5,
		MaxCommunityFraction: 0.2,
	})
	graph, err = clusterEngine.Execute(ctx, graph)
	if err != nil {
		p.writeErrorLine(err)
		return nil, err
	}

	communityCount := 0
	communitySet := map[int]struct{}{}
	for _, v := range graph.GetNodes() {
		if v.Community != -1 {
			communitySet[v.Community] = struct{}{}
		}
	}

	communityCount = len(communitySet)
	p.writeLine(fmt.Sprintf("      Found %d communities", communityCount))
	p.writeLine("")

	// Stage 5: Analyze graph
	p.writeLine("[5/6] Analyzing graph structure...")
	var analyzer = NewAnalyzer(&AnalyzerOptions{
		TopGodNodesCount:         10,
		TopSurprisingConnections: 5,
		MaxSuggestedQuestions:    10,
	})
	analysis, err := analyzer.Execute(ctx, *graph)
	if err != nil {
		p.writeErrorLine(err)
		return nil, err
	}
	p.writeLine(fmt.Sprintf("      God nodes: %d", len(analysis.GodNodes)))
	p.writeLine(fmt.Sprintf("      Surprising connections: %d", len(analysis.SurprisingConnections)))
	p.writeLine(fmt.Sprintf("      Suggested questions: %d", len(analysis.SuggestedQuestions)))
	p.writeLine("")

	// Prepare community labels and cohesion scores for report and exports
	var communityLabels = p.buildCommunityLabels(graph)
	var cohesionScores = p.calculateCohesionScores(graph)

	// Stage 6: Export
	p.writeLine("[6/6] Exporting results...")

	// Validate output directory to prevent path traversal
	var validator = NewInputValidator()
	var outputValidation = validator.ValidatePath(outputDir, "")
	if !outputValidation.IsValid {
		p.writeLine("Invalid output directory: " + strings.Join(outputValidation.Errors, " "))
		return nil, err
	}

	if _, err := os.Stat(outputDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			p.writeErrorLine(err)
			return nil, err
		}
	}

	for _, format := range formats {
		var normalizedFormat = strings.ToLower(format)
		switch normalizedFormat {
		case "json":
			var jsonExporter = &JsonExporter{}
			var jsonPath = filepath.Join(outputDir, "graph.json")
			if err := jsonExporter.Export(ctx, graph, jsonPath); err != nil {
				p.writeErrorLine(err)
				return nil, err

			}
			p.writeLine(fmt.Sprintf("      Exported JSON: %s", jsonPath))

		case "html":
			var htmlExporter = &HtmlExporter{}
			var htmlPath = filepath.Join(outputDir, "graph.html")
			if err := htmlExporter.ExportWIthLabels(ctx, graph, htmlPath, communityLabels); err != nil {
				p.writeErrorLine(err)
				return nil, err

			}
			p.writeLine(fmt.Sprintf("      Exported HTML: %s", htmlPath))
		case "svg":
			var svgExporter = NewSvgExporter()
			var svgPath = filepath.Join(outputDir, "graph.svg")
			if err := svgExporter.Export(ctx, graph, svgPath); err != nil {
				p.writeErrorLine(err)
				return nil, err
			}
			p.writeLine(fmt.Sprintf("      Exported SVG: %s", svgPath))
		case "neo4j":
			var neo4jExporter = &Neo4jExporter{}
			var cypherPath = filepath.Join(outputDir, "graph.cypher")
			if err := neo4jExporter.Export(ctx, graph, cypherPath); err != nil {
				p.writeErrorLine(err)
				return nil, err
			}
			p.writeLine(fmt.Sprintf("      Exported Neo4j Cypher: %s", cypherPath))
		case "ladybug":
			var ladybugExporter = &LadybugExporter{}
			var ladybugPath = filepath.Join(outputDir, "graph.ladybug.cypher")
			if err := ladybugExporter.Export(ctx, graph, ladybugPath); err != nil {
				p.writeErrorLine(err)
				return nil, err
			}
			p.writeLine(fmt.Sprintf("      Exported Ladybug Cypher: %s", ladybugPath))
		case "obsidian":
			var obsidianExporter = &ObsidianExporter{}
			var obsidianPath = filepath.Join(outputDir, "obsidian")
			if err := obsidianExporter.Export(ctx, graph, obsidianPath); err != nil {
				p.writeErrorLine(err)
				return nil, err
			}
			p.writeLine(fmt.Sprintf("      Exported Obsidian vault: %s/", obsidianPath))
		case "wiki":
			var wikiExporter = &WikiExporter{}
			var wikiPath = filepath.Join(outputDir, "wiki")
			if err := wikiExporter.Export(ctx, graph, wikiPath); err != nil {
				p.writeErrorLine(err)
				return nil, err
			}
			p.writeLine(fmt.Sprintf("      Exported Wiki: %s/", wikiPath))

		case "report":
			var reportGenerator = &ReportGenerator{}
			var projectName = GetFileName(inputPath)
			var reportMarkdown = reportGenerator.Generate(graph, *analysis, communityLabels, cohesionScores, projectName)
			var reportPath = filepath.Join(outputDir, "GRAPH_REPORT.md")
			if err := os.WriteFile(reportPath, []byte(reportMarkdown), 0644); err != nil {
				p.writeErrorLine(err)
				return nil, err
			}
			p.writeLine(fmt.Sprintf("      Exported Report: %s", reportPath))

		default:
			p.writeLine(fmt.Sprintf("      Warning: Unknown format '%s' - skipped", normalizedFormat))
		}
	}

	p.writeLine("")
	p.writeLine("✓ Pipeline completed successfully")
	p.writeLine("")

	// Print summary
	p.writeLine("Summary:")
	p.writeLine(fmt.Sprintf("  Nodes:         %d", analysis.Statistics.NodeCount))
	p.writeLine(fmt.Sprintf("  Edges:         %d", analysis.Statistics.EdgeCount))
	p.writeLine(fmt.Sprintf("  Communities:   %d", analysis.Statistics.CommunityCount))
	p.writeLine(fmt.Sprintf("  Avg Degree:    %f", analysis.Statistics.AverageDegree))
	p.writeLine(fmt.Sprintf("  Isolated:      %d", analysis.Statistics.IsolatedNodeCount))

	if len(analysis.GodNodes) > 0 {
		p.writeLine("")
		p.writeLine("Top God Nodes:")
		for i := 0; i < min(5, len(analysis.GodNodes)); i++ {
			godNode := analysis.GodNodes[i]
			p.writeLine(fmt.Sprintf("  [%d] %s", godNode.EdgeCount, godNode.Label))
		}
	}

	return graph, nil
}
