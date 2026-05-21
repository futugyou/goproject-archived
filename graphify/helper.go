package graphify

import (
	"cmp"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

func Where[T any](slice []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

func Select[T any, P any](slice []T, predicate func(T) P) []P {
	var result []P
	for _, v := range slice {
		result = append(result, predicate(v))
	}
	return result
}

func GroupBy[T any, K comparable](slice []T, keySelector func(T) K) map[K][]T {
	groups := make(map[K][]T)
	for _, v := range slice {
		key := keySelector(v)
		groups[key] = append(groups[key], v)
	}
	return groups
}

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

func Take[T any](s []T, n int) []T {
	if n > len(s) {
		return s
	}
	return s[:n]
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

func ForEachOrderByDescending[K comparable, V any, T cmp.Ordered](m map[K]V, action func(K, V), keySelector func(K) T) {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(a, b K) int {
		return cmp.Compare(keySelector(b), keySelector(a))
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

func MathClamp[T int | float32 | float64](val, min, max T) T {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func CalculateCohesion(graph KnowledgeGraph, communityId int) float32 {
	var nodes = graph.GetNodesByCommunity(communityId)
	n := len(nodes)

	if n <= 1 {
		return 1.0
	}

	nodeSet := map[string]struct{}{}
	for _, n := range nodes {
		nodeSet[n.Id] = struct{}{}
	}

	var actualEdges float32 = 0

	for _, node := range nodes {
		for _, edge := range graph.GetEdgesById(node.Id) {
			var otherId = edge.Source.Id
			if edge.Source.Id == node.Id {
				otherId = edge.Target.Id
			}
			if _, ok := nodeSet[otherId]; ok && cmp.Compare(edge.Source.Id, edge.Target.Id) < 0 {
				actualEdges++
			}
		}
	}

	possibleEdges := float32(n*(n-1)) / 2.0
	if possibleEdges > 0 {
		return float32(math.Round(float64(actualEdges / possibleEdges)))
	}
	return 0.0
}
