package graphify

import (
	"regexp"
	"strings"
	"unicode"
)

func ToDictionary[T any, K comparable, V any](
	source []T,
	keySelector func(T) K,
	valueSelector func(T) V,
) map[K]V {
	result := make(map[K]V, len(source))
	for _, item := range source {
		result[keySelector(item)] = valueSelector(item)
	}
	return result
}

func SanitizeLabel(input string, maxLength int) string {
	if input == "" {
		return ""
	}

	var sb strings.Builder
	for _, r := range input {
		if !unicode.IsControl(r) {
			sb.WriteRune(r)
		}
	}
	result := sb.String()

	reScript := regexp.MustCompile(`(?i)(?s)<script[^>]*>.*?</script>`)
	result = reScript.ReplaceAllString(result, "")

	reHtml := regexp.MustCompile(`<[^>]+>`)
	result = reHtml.ReplaceAllString(result, "")

	result = strings.TrimSpace(result)

	runes := []rune(result)
	if len(runes) > maxLength {
		result = string(runes[:maxLength])
	}

	return result
}
