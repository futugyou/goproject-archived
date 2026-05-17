package graphify

import (
	"regexp"
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

func Truncate(value string, maxLength int) string {
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
