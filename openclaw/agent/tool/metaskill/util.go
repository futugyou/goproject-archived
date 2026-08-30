package metaskill

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/futugyou/openclaw/util"
)

var triggerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)触发(?:短语|词)?(?:要|应|必须)?(?:包含|包括)\s*[:：]\s*([^\n。；;]+)`),
	regexp.MustCompile(`(?i)trigger phrases?\s+(?:must\s+)?(?:include|contain)\s*[:：]\s*([^\n.;]+)`),
}

func ExtractRequiredTriggersFromIntent(userIntent string) []string {
	var captured string

	for _, pattern := range triggerPatterns {
		match := pattern.FindStringSubmatch(userIntent)
		if len(match) > 1 {
			captured = match[1]
			break
		}
	}

	if strings.TrimSpace(captured) == "" {
		return []string{}
	}

	var output []string
	seen := make(map[string]struct{})

	splitFn := func(r rune) bool {
		return r == ',' || r == '，' || r == '、'
	}

	raws := strings.FieldsFunc(captured, splitFn)
	cutset := "`'\"“”‘’[]()"

	for _, raw := range raws {
		phrase := strings.TrimSpace(raw)
		phrase = strings.Trim(phrase, cutset)
		phrase = strings.TrimSpace(phrase)

		if phrase == "" || len(phrase) > 80 {
			continue
		}

		if strings.ContainsAny(phrase, `"\\r\n`) {
			continue
		}

		if _, exists := seen[phrase]; !exists {
			seen[phrase] = struct{}{}
			output = append(output, phrase)
		}
	}

	return output
}

var (
	nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9_-]`)
	startsWithLetter = regexp.MustCompile(`^[a-z]`)
)

func BuildNameFromIntent(intent string, patternID string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(intent) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}

	normalized := strings.TrimSpace(builder.String())

	if len(normalized) == 0 {
		normalized = "generated"
	}

	normalized = strings.ReplaceAll(normalized, " ", "-")

	if len(normalized) > 40 {
		normalized = strings.TrimRight(normalized[:40], "-")
	}

	name := "meta-" + patternID + "-" + normalized

	name = nonAlphaNumRegex.ReplaceAllString(name, "")

	if !startsWithLetter.MatchString(name) {
		name = "meta-" + name
	}

	if len(name) > 64 {
		return name[:64]
	}

	return name
}

func BuildDescription(userIntent string) (string, error) {
	var compact = strings.TrimSpace(strings.Join(strings.Split(userIntent, "\r\n"), " "))
	if len(compact) < 30 {
		compact = fmt.Sprintf("Generated meta skill for workflow: %s", compact)
	}

	compact = util.Truncate(compact, 200)

	return SanitizeYamlText(compact, "description")
}

func SanitizeYamlText(value, fieldName string) (string, error) {
	if strings.ContainsAny(value, "\"\r\n\\") {
		return "", fmt.Errorf("%s may not contain double quotes, newlines, or backslashes.", fieldName)
	}
	return value, nil
}

func BuildTask(prefix, userIntent string) string {
	var compact = strings.TrimSpace(strings.Join(strings.Split(userIntent, "\r\n"), " "))

	compact = util.Truncate(compact, 200)

	var task = util.Truncate(fmt.Sprintf("%s: %s", prefix, compact), 400)
	re, _ := SanitizeYamlText(task, "description")
	return re
}
