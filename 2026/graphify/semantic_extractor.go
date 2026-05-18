package graphify

import (
	"context"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/futugyou/yomawari/extensions_ai/abstractions/chatcompletion"
	"github.com/google/uuid"
)

var _ IPipelineStage[DetectedFile, ExtractionResult] = (*SemanticExtractor)(nil)

type SemanticExtractor struct {
	chatClient chatcompletion.IChatClient
	options    SemanticExtractorOptions
}

func NewSemanticExtractor(options *SemanticExtractorOptions, chatClient chatcompletion.IChatClient) *SemanticExtractor {
	if options == nil {
		options = DefaultSemanticExtractorOptions()
	}

	return &SemanticExtractor{
		options:    *options,
		chatClient: chatClient,
	}
}

// Execute implements [IPipelineStage].
func (s *SemanticExtractor) Execute(ctx context.Context, input DetectedFile) (*ExtractionResult, error) {
	// Graceful degradation: if no AI client configured, return empty results
	if s.chatClient == nil {
		return s.createEmptyResult(input), nil
	}

	// Check file size limit
	if input.SizeBytes > s.options.MaxFileSizeBytes {
		return s.createEmptyResult(input), nil
	}

	// Decide whether to process based on file category and options
	shouldProcess := false
	switch input.Category {
	case FileCategoryCode:
		shouldProcess = s.options.ExtractFromCode
	case FileCategoryDocumentation:
		shouldProcess = s.options.ExtractFromDocs
	case FileCategoryMedia:
		shouldProcess = s.options.ExtractFromMedia
	}

	if !shouldProcess {
		return s.createEmptyResult(input), nil
	}

	extractedData, err := s.extractFromFileAsync(ctx, input)
	if err != nil {
		return s.createEmptyResult(input), nil
	}

	// On any error (API failure, malformed response, rate limits), return empty result
	// This allows the pipeline to continue even if semantic extraction fails
	return extractedData, nil
}

func (s *SemanticExtractor) createEmptyResult(file DetectedFile) *ExtractionResult {
	return &ExtractionResult{
		Nodes:                  []ExtractedNode{},
		Edges:                  []ExtractedEdge{},
		SourceFilePath:         file.FilePath,
		RelativeSourceFilePath: file.RelativePath,
		Method:                 ExtractionMethodSemantic,
	}
}

func (s *SemanticExtractor) convertToExtractionResult(data *LlmExtractionData, sourceFile DetectedFile) *ExtractionResult {
	var nodes = []ExtractedNode{}
	var edges = []ExtractedEdge{}

	// Convert nodes
	for _, node := range data.Nodes {
		fileType := FileTypeCode
		switch strings.ToLower(node.Type) {
		case "document":
			fileType = FileTypeDocument
		case "paper":
			fileType = FileTypePaper
		case "image":
			fileType = FileTypeImage
		}

		var metadata = map[string]string{}
		maps.Copy(metadata, node.Metadata)
		n := ExtractedNode{
			Id:         node.Id,
			Label:      node.Label,
			FileType:   fileType,
			SourceFile: sourceFile.FilePath,
			Metadata:   metadata,
		}

		if n.Id == "" {
			n.Id = uuid.New().String()
		}
		if n.Label == "" {
			n.Label = "Unknown"
		}
		nodes = append(nodes, n)

	}

	// Convert edges
	for _, edge := range data.Edges {
		confidence := ConfidenceInferred
		switch strings.ToUpper(edge.Confidence) {
		case "EXTRACTED":
			confidence = ConfidenceExtracted
		case "AMBIGUOUS":
			confidence = ConfidenceAmbiguous
		}

		e := ExtractedEdge{
			Source:     edge.Source,
			Target:     edge.Target,
			Relation:   edge.Relation,
			Confidence: confidence,
			SourceFile: sourceFile.FilePath,
			Weight:     float32(edge.Weight),
		}
		if e.Relation == "" {
			e.Relation = "related_to"
		}
		if e.Weight == -1 {
			e.Weight = 1.0
		}

		edges = append(edges, e)
	}

	// Add confidence scores
	var confidenceScores = map[string]float32{}
	for _, edge := range data.Edges {
		if edge.Weight != -1 && len(edge.Source) > 0 && len(edge.Target) > 0 {
			confidenceScores[fmt.Sprintf("%s->%s", edge.Source, edge.Target)] = float32(edge.Weight)
		}
	}

	return &ExtractionResult{
		Nodes:                  nodes,
		Edges:                  edges,
		SourceFilePath:         sourceFile.FilePath,
		RelativeSourceFilePath: sourceFile.RelativePath,
		Method:                 ExtractionMethodSemantic,
		ConfidenceScores:       confidenceScores,
	}
}

func (s *SemanticExtractor) buildPrompt(file DetectedFile, fileContent string) string {
	mexNodes := s.options.MaxNodesPerFile
	switch file.Category {
	case FileCategoryCode:
		return ExtractionPromptsCodeSemanticExtraction(file.FileName, fileContent, &mexNodes)
	case FileCategoryDocumentation:
		return ExtractionPromptsDocumentationExtraction(file.FileName, fileContent, &mexNodes)

	case FileCategoryMedia:
		if file.Extension == ".pdf" {
			return ExtractionPromptsPaperExtraction(file.FileName, fileContent, &mexNodes)
		} else {
			return ExtractionPromptsImageVisionExtraction(file.FileName, &mexNodes)
		}
	}

	return ""
}

func (s *SemanticExtractor) extractFromFileAsync(ctx context.Context, file DetectedFile) (*ExtractionResult, error) {
	// Read file content
	fileContent, err := os.ReadFile(file.FilePath)
	if err != nil {
		return nil, err
	}

	// Build the extraction prompt based on file category
	var prompt = s.buildPrompt(file, string(fileContent))

	// Create chat messages
	var messages = []chatcompletion.ChatMessage{*chatcompletion.NewChatMessageWithText(chatcompletion.RoleUser, prompt)}

	// Set up chat options
	temperature := float64(s.options.Temperature)
	maxOutputTokens := int64(s.options.MaxTokens)
	var chatOptions = &chatcompletion.ChatOptions{
		Temperature:     &temperature,
		MaxOutputTokens: &maxOutputTokens,
	}

	if len(s.options.ModelId) > 0 {
		modelid := s.options.ModelId
		chatOptions.ModelId = &modelid
	}

	// Call the LLM
	response, err := s.chatClient.GetResponse(ctx, messages, chatOptions)
	if err != nil {
		return nil, err
	}

	// Validate and sanitize LLM response before it enters the pipeline (FINDING-003)
	var jsonResponse = "{}"

	if response != nil && len(response.Text()) > 0 {
		jsonResponse = response.Text()
	}
	var validated = LlmValidateAndSanitize(jsonResponse, file.FilePath)

	if validated != nil {
		// LLM response failed validation — return empty result (fail safe)
		return s.createEmptyResult(file), nil
	}

	// Convert validated data to ExtractionResult
	return s.convertToExtractionResult(validated, file), nil
}
