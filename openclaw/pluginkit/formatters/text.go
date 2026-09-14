package formatters

import (
	"fmt"
	"regexp"
	"strings"
)

// ansiPattern 匹配 ANSI 逃逸序列的正则表达式
var ansiPattern = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)

// StripAnsi 移除字符串中的 ANSI 逃逸字符
func StripAnsi(text string) string {
	return ansiPattern.ReplaceAllString(text, "")
}

// TrimEmptyEdges 移除切片开头和结尾的空白字符串（包含仅含空格的行）
func TrimEmptyEdges(lines []string) []string {
	start := 0
	end := len(lines)

	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	return append([]string(nil), lines[start:end]...)
}

// DedupeAdjacent 去除连续重复的行
func DedupeAdjacent(lines []string) []string {
	if len(lines) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(lines))
	var last *string

	for _, line := range lines {
		if last == nil || line != *last {
			result = append(result, line)
			l := line
			last = &l
		}
	}

	return result
}

// HeadTail 保留前 head 行和后 tail 行，中间超过的部分以省略提示替换
func HeadTail(lines []string, head, tail int) []string {
	if len(lines) <= head+tail {
		return append([]string(nil), lines...)
	}

	omitted := len(lines) - head - tail
	result := make([]string, 0, head+1+tail)

	result = append(result, lines[:head]...)
	result = append(result, fmt.Sprintf("... omitted %d lines ...", omitted))
	result = append(result, lines[len(lines)-tail:]...)

	return result
}

// CountPattern 统计符合正则匹配的行数
func CountPattern(lines []string, pattern *regexp.Regexp) int {
	count := 0
	for _, line := range lines {
		if pattern.MatchString(line) {
			count++
		}
	}
	return count
}

// CompilePattern 编译正则表达式（支持 'i' 忽略大小写 和 'm' 多行模式标志）
func CompilePattern(pattern string, flags string) (*regexp.Regexp, error) {
	var prefix strings.Builder
	if strings.Contains(flags, "i") || strings.Contains(flags, "m") {
		prefix.WriteString("(?")
		if strings.Contains(flags, "i") {
			prefix.WriteString("i")
		}
		if strings.Contains(flags, "m") {
			prefix.WriteString("m")
		}
		prefix.WriteString(")")
	}

	expr := prefix.String() + pattern
	return regexp.Compile(expr)
}
