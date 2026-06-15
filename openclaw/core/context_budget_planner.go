package core

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

const TokenCharEstimate = 4

type ContextBudgetPlanner struct {
	config   *GatewayConfig
	provider IStructuredMemoryProvider
}

func NewContextBudgetPlanner(config *GatewayConfig, provider IStructuredMemoryProvider) *ContextBudgetPlanner {
	return &ContextBudgetPlanner{
		config:   config,
		provider: provider,
	}
}

func (cb *ContextBudgetPlanner) BuildContext(
	ctx context.Context,
	request *StructuredMemoryContextRequest,
) (*StructuredMemoryContextResult, error) {

	fractal := cb.config.Memory.Fractal
	if !fractal.Enabled {
		return cb.fail("Fractal Memory is disabled.", ""), nil
	}

	autoMode := cb.normalizeAutoContextMode(fractal.AutoContextMode)
	requestedMode := cb.normalizeAutoContextMode(request.Mode)

	if requestedMode != "off" && requestedMode != "manual" && requestedMode != "pulse" && requestedMode != "auto" {
		return cb.fail(fmt.Sprintf("Unsupported Fractal Memory context mode '%s'.", request.Mode), ""), nil
	}
	if requestedMode == "off" {
		return cb.fail("Fractal Memory context request mode is off.", ""), nil
	}
	if autoMode == "off" && requestedMode != "manual" {
		return cb.fail("Fractal Memory automatic context is disabled.", ""), nil
	}
	if autoMode == "manual" && requestedMode != "manual" {
		return cb.fail("Fractal Memory automatic context is set to manual only.", ""), nil
	}
	if requestedMode == "auto" && autoMode != "auto" {
		return cb.fail(fmt.Sprintf("Fractal Memory auto context is configured for '%s', not 'auto'.", autoMode), ""), nil
	}
	if requestedMode == "pulse" && autoMode != "pulse" && autoMode != "auto" {
		return cb.fail(fmt.Sprintf("Fractal Memory pulse context is configured for '%s', not 'pulse' or 'auto'.", autoMode), ""), nil
	}

	mode := cb.normalizeExportMode(fractal.DefaultExportMode)
	var export *StructuredMemoryExportResult
	var err error
	sourcePath := cb.normalizePath(request.PathHint)

	if sourcePath != "" {
		export, err = cb.provider.Export(ctx, sourcePath, mode)
	} else {
		sourcePath, err = cb.resolveBestPath(ctx, request)
		if err != nil {
			return nil, err
		}
		if sourcePath == "" {
			return cb.fail("No Fractal Memory node matched the context request.", ""), nil
		}

		export, err = cb.provider.Export(ctx, sourcePath, mode)
	}

	if err != nil {
		return nil, err
	}

	if !export.Success {
		errMsg := export.Error
		if errMsg == "" {
			errMsg = "Fractal Memory export failed."
		}
		return cb.fail(errMsg, sourcePath), nil
	}

	contextStr := cb.buildContextBlock(export, fractal.DefaultDepth)
	maxChars := cb.resolveMaxChars(request, fractal)
	truncated := export.Truncated

	// Go 使用 rune 切片来确保字符级安全截取，避免字节截断导致乱码
	contextRunes := []rune(contextStr)
	if len(contextRunes) > maxChars {
		marker := "\n...[truncated]\n</fractal_memory_context>"
		markerRunes := []rune(marker)

		contentBudget := maxChars - len(markerRunes)
		if contentBudget < 0 {
			contentBudget = 0
		}

		if contentBudget == 0 {
			limit := len(markerRunes)
			if maxChars < limit {
				limit = maxChars
			}
			contextStr = string(markerRunes[:limit])
		} else {
			limit := len(contextRunes)
			if limit > contentBudget {
				limit = contentBudget
			}
			trimmed := strings.TrimRight(string(contextRunes[:limit]), " \t\n\r")
			contextStr = trimmed + marker
		}
		truncated = true
	}

	return &StructuredMemoryContextResult{
		Success:    true,
		Context:    &contextStr,
		SourcePath: &sourcePath,
		Mode:       mode,
		Truncated:  truncated,
		Sources:    export.Sources,
	}, nil
}

func (cb *ContextBudgetPlanner) resolveBestPath(
	ctx context.Context,
	request *StructuredMemoryContextRequest,
) (string, error) {

	query := strings.TrimSpace(request.Query)
	if query == "" && request.SessionId != nil {
		query = *request.SessionId
	}

	if query != "" {
		search, err := cb.provider.Search(ctx, query, 3, request.Scope)
		if err != nil {
			return "", err
		}
		if search.Success {
			for _, item := range search.Items {
				if strings.TrimSpace(item.Path) != "" {
					return item.Path, nil
				}
			}
		}
	}

	recent, err := cb.provider.Recent(ctx, 14, 1, request.Scope)
	if err != nil {
		return "", err
	}
	if recent.Success {
		for _, item := range recent.Items {
			if strings.TrimSpace(item.Path) != "" {
				return item.Path, nil
			}
		}
	}

	return "", nil
}

func (cb *ContextBudgetPlanner) buildContextBlock(export *StructuredMemoryExportResult, depth int) string {
	generatedAt := time.Now().UTC()
	var sb strings.Builder

	sb.WriteString("<fractal_memory_context>\n")
	sb.WriteString(fmt.Sprintf("Source: %s\n", export.Path))
	sb.WriteString(fmt.Sprintf("Mode: %s\n", export.Mode))
	sb.WriteString(fmt.Sprintf("Depth: %d\n", depth))
	sb.WriteString(fmt.Sprintf("GeneratedAtUtc: %s\n", generatedAt.Format(time.RFC3339Nano)))
	sb.WriteString("Trust: untrusted_reference_data\n\n")

	if len(export.Sources) > 0 {
		sb.WriteString("Source labels:\n")
		limit := len(export.Sources)
		if limit > 20 {
			limit = 20
		}
		for _, source := range export.Sources[:limit] {
			label := source.SourcePath
			if strings.TrimSpace(label) == "" {
				label = source.Path
			}

			line := ""
			if source.StartLine != nil && source.EndLine != nil {
				line = fmt.Sprintf(":%d-%d", *source.StartLine, *source.EndLine)
			}
			sb.WriteString(fmt.Sprintf("- %s%s\n", label, line))
		}
		sb.WriteString("\n")
	}

	if strings.TrimSpace(export.Content) != "" {
		sb.WriteString(strings.TrimSpace(export.Content))
		sb.WriteString("\n")
	}

	sb.WriteString("</fractal_memory_context>\n")
	return sb.String()
}

func (cb *ContextBudgetPlanner) resolveMaxChars(request *StructuredMemoryContextRequest, config *FractalMemoryConfig) int {
	safeTokenChars := func(tokens int) int64 {
		t := int64(tokens)
		if t < 1 {
			t = 1
		}
		return t * TokenCharEstimate
	}

	reqMaxChars := int64(config.MaxContextChars)
	if request.MaxChars != nil {
		reqMaxChars = int64(*request.MaxChars)
	}
	if reqMaxChars < 1 {
		reqMaxChars = 1
	}

	reqMaxTokens := config.MaxContextTokens
	if request.MaxTokens != nil {
		reqMaxTokens = *request.MaxTokens
	}
	maxTokenChars := safeTokenChars(reqMaxTokens)

	configMaxChars := int64(config.MaxContextChars)
	if configMaxChars < 1 {
		configMaxChars = 1
	}
	configTokenChars := safeTokenChars(config.MaxContextTokens)

	// Math.Min 逻辑
	min := reqMaxChars
	if maxTokenChars < min {
		min = maxTokenChars
	}
	if configMaxChars < min {
		min = configMaxChars
	}
	if configTokenChars < min {
		min = configTokenChars
	}

	// Clamp 到 [1, math.MaxInt32]
	if min < 1 {
		min = 1
	} else if min > math.MaxInt32 {
		min = math.MaxInt32
	}

	return int(min)
}

func (cb *ContextBudgetPlanner) normalizeExportMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return "compact"
	}
	return strings.ToLower(strings.TrimSpace(mode))
}

func (cb *ContextBudgetPlanner) normalizeAutoContextMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return "off"
	}
	return strings.ToLower(strings.TrimSpace(mode))
}

func (cb *ContextBudgetPlanner) normalizePath(path *string) string {
	if path == nil || strings.TrimSpace(*path) == "" {
		return ""
	}
	return strings.TrimSpace(*path)
}

func (cb *ContextBudgetPlanner) fail(errorMsg string, sourcePath string) *StructuredMemoryContextResult {
	return &StructuredMemoryContextResult{
		Success:    false,
		SourcePath: &sourcePath,
		Error:      &errorMsg,
	}
}
