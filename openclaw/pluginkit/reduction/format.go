package reduction

import (
	"fmt"
	"sort"
	"strings"
)

func Format(summary string, facts map[string]int, exitCode int, maxInlineChars *int) string {
	var parts []string

	// 1. 处理退出码
	if exitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit %d", exitCode))
	}

	// 2. 过滤非 0 的 facts 键值对
	var nonZeroFacts []string
	var keys []string
	for k := range facts {
		keys = append(keys, k)
	}
	// 对 key 进行排序以保证输出结果顺序确定（解决 Go map 无序问题）
	sort.Strings(keys)

	for _, k := range keys {
		if v := facts[k]; v != 0 {
			nonZeroFacts = append(nonZeroFacts, fmt.Sprintf("%s: %d", k, v))
		}
	}

	if len(nonZeroFacts) > 0 {
		parts = append(parts, strings.Join(nonZeroFacts, "; "))
	}

	// 3. 处理 summary 摘要
	trimmedSummary := strings.TrimSpace(summary)
	if len(trimmedSummary) > 0 {
		parts = append(parts, trimmedSummary)
	}

	// 4. 拼接最终字符串并 TrimSpace
	result := strings.TrimSpace(strings.Join(parts, "\n"))

	// 5. 超长截断处理
	if maxInlineChars != nil && *maxInlineChars > 0 {
		// 转为 []rune 以正确计算 Unicode 字符数（避免切分中文等多字节字符时产生乱码）
		runes := []rune(result)
		if len(runes) > *maxInlineChars {
			half := (*maxInlineChars - 32) / 2
			if half < 1 {
				half = 1
			}

			head := string(runes[:half])
			tail := string(runes[len(runes)-half:])
			result = fmt.Sprintf("%s\n... omitted chars ...\n%s", head, tail)
		}
	}

	return result
}
