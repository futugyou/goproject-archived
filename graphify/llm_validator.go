package graphify

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

const (
	LlmResponseValidatorMaxNodeLabelLength    = 200
	LlmResponseValidatorMaxEdgeRelationLength = 100
	LlmResponseValidatorMaxFilePathLength     = 500
	LlmResponseValidatorMaxIdLength           = 200
	LlmResponseValidatorMaxNodesAllowed       = 50
	LlmResponseValidatorMaxEdgesAllowed       = 100
)

var LlmResponseValidator *InputValidator = NewInputValidator()
var scriptPattern = regexp.MustCompile(`(?i)<script[^>]*>|</script>|javascript:|on\w+\s*=`)

func ContainsSuspiciousContent(src string) bool {
	return scriptPattern.MatchString(src)
}

func LlmResponseValidatorTruncate(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}

	return value[:(maxLength-3)] + "..."
}

func ExtractJsonFromMarkdown(text string) string {
	var trimmed = strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```json") {
		trimmed = trimmed[7:]
	} else if strings.HasPrefix(trimmed, "```") {
		trimmed = trimmed[3:]
	}

	trimmed = strings.TrimSuffix(trimmed, "```")

	return strings.TrimSpace(trimmed)
}

type LlmExtractionData struct {
	Nodes []LlmNodeData
	Edges []LlmEdgeData
}

type LlmNodeData struct {
	Id       string
	Label    string
	Type     string
	Metadata map[string]string
}

type LlmEdgeData struct {
	Source     string
	Target     string
	Relation   string
	Confidence string
	Weight     int
}

func llmValidatorSanitizeEdge(edge LlmEdgeData, validNodeIds []string) *LlmEdgeData {
	if len(edge.Source) == 0 || len(edge.Target) == 0 {
		return nil
	}

	// Only allow edges that reference known nodes
	if !slices.Contains(validNodeIds, edge.Source) || !slices.Contains(validNodeIds, edge.Target) {
		return nil
	}

	// Reject edges with suspicious content
	if ContainsSuspiciousContent(edge.Relation) {
		return nil
	}

	// Sanitize relation through InputValidator
	rel := edge.Relation
	if len(rel) == 0 {
		rel = "related_to"
	}
	var relationResult = LlmResponseValidator.SanitizeLabel(rel, LlmResponseValidatorMaxEdgeRelationLength)
	sanitizedRelation := "related_to"
	if relationResult.IsValid {
		if len(relationResult.SanitizedValue) > 0 {
			sanitizedRelation = relationResult.SanitizedValue
		} else {
			sanitizedRelation = rel
		}
	}

	// Clamp weight to valid range
	weight := MathClamp(edge.Weight, 0.0, 1.0)

	return &LlmEdgeData{
		Source:     edge.Source,
		Target:     edge.Target,
		Relation:   sanitizedRelation,
		Confidence: edge.Confidence,
		Weight:     weight,
	}
}

func llmValidatorSanitizeNode(node LlmNodeData) *LlmNodeData {
	// Must have id and label
	if len(node.Id) == 0 || len(node.Label) == 0 {
		return nil
	}

	// Reject nodes with script/HTML injection in id or label
	if ContainsSuspiciousContent(node.Id) || ContainsSuspiciousContent(node.Label) {
		return nil
	}

	// Sanitize label through InputValidator
	var labelResult = LlmResponseValidator.SanitizeLabel(node.Label, LlmResponseValidatorMaxNodeLabelLength)

	sanitizedLabel := node.Label
	if labelResult.IsValid && len(labelResult.SanitizedValue) > 0 {
		sanitizedLabel = labelResult.SanitizedValue
	}

	// Truncate id
	sanitizedId := LlmResponseValidatorTruncate(node.Id, LlmResponseValidatorMaxIdLength)

	// Sanitize metadata values
	sanitizedMetadata := map[string]string{}
	for key, value := range node.Metadata {
		if len(key) == 0 || ContainsSuspiciousContent(value) {
			continue
		}

		var metaResult = LlmResponseValidator.SanitizeLabel(value, LlmResponseValidatorMaxNodeLabelLength)
		sanitizedMetadata[key] = value

		if metaResult.IsValid && len(metaResult.SanitizedValue) > 0 {
			sanitizedMetadata[key] = metaResult.SanitizedValue
		}
	}

	nodeType := node.Type
	if len(nodeType) == 0 {
		nodeType = "Code"
	}

	return &LlmNodeData{
		Id:       sanitizedId,
		Label:    sanitizedLabel,
		Type:     LlmResponseValidatorTruncate(nodeType, LlmResponseValidatorMaxEdgeRelationLength),
		Metadata: sanitizedMetadata,
	}
}

func LlmValidateAndSanitize(rawJson, filePath string) *LlmExtractionData {
	if len(rawJson) == 0 {
		return nil
	}

	// Parse JSON
	var data LlmExtractionData
	var cleanJson = ExtractJsonFromMarkdown(rawJson)
	if err := json.Unmarshal([]byte(cleanJson), &data); err != nil {
		return nil
	}

	sanitizedNodes := []LlmNodeData{}

	// Validate and sanitize nodes
	for _, node := range data.Nodes {
		var sanitized = llmValidatorSanitizeNode(node)
		if sanitized != nil {
			sanitizedNodes = append(sanitizedNodes, *sanitized)
		}
	}

	// Enforce max count
	if len(sanitizedNodes) > LlmResponseValidatorMaxNodesAllowed {
		sanitizedNodes = sanitizedNodes[:LlmResponseValidatorMaxNodesAllowed]
	}

	nodeIds := []string{}
	for _, v := range sanitizedNodes {
		if !slices.Contains(nodeIds, v.Id) {
			nodeIds = append(nodeIds, v.Id)
		}
	}
	// Validate and sanitize edges — only keep edges referencing known nodes
	sanitizedEdges := []LlmEdgeData{}
	for _, edge := range data.Edges {
		var sanitized = llmValidatorSanitizeEdge(edge, nodeIds)
		if sanitized != nil {
			sanitizedEdges = append(sanitizedEdges, *sanitized)
		}
	}

	if len(sanitizedEdges) > LlmResponseValidatorMaxEdgesAllowed {
		sanitizedEdges = sanitizedEdges[:LlmResponseValidatorMaxEdgesAllowed]
	}

	return &LlmExtractionData{
		Nodes: sanitizedNodes,
		Edges: sanitizedEdges,
	}
}
