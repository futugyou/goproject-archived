package graphify

import (
	"cmp"
	"path/filepath"
	"regexp"
	"slices"
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

func ForEachSorted[K cmp.Ordered, V any](m map[K]V, action func(key K, value V)) {
	if len(m) == 0 {
		return
	}

	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	for _, k := range keys {
		action(k, m[k])
	}
}

func ForEachOrderBy[K comparable, V any, T cmp.Ordered](m map[K]V, action func(K, V), keySelector func(K) T) {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(a, b K) int {
		return cmp.Compare(keySelector(a), keySelector(b))
	})

	for _, k := range keys {
		action(k, m[k])
	}
}

func GetFileName(outputPath string) string {
	fileName := filepath.Base(outputPath)
	ext := filepath.Ext(fileName)
	return strings.TrimSuffix(fileName, ext)
}
