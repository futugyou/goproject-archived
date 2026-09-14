package reduction

import (
	"strings"
	"unicode"
)

type SemanticDensityCalculator struct {
	threshold float64
}

func NewSemanticDensityCalculator(threshold ...float64) *SemanticDensityCalculator {
	t := 0.3
	if len(threshold) > 0 {
		t = threshold[0]
	}
	return &SemanticDensityCalculator{threshold: t}
}

// ShouldReduce 计算文本的语义密度并判断是否需要化简/规约
func (c *SemanticDensityCalculator) ShouldReduce(text string) bool {
	// 按照回车符 \r 和换行符 \n 切分字符串
	rawLines := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\r' || r == '\n'
	})

	// 移除空行（对应 StringSplitOptions.RemoveEmptyEntries）
	var lines []string
	for _, line := range rawLines {
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}

	totalLines := len(lines)
	if totalLines == 0 {
		return false
	}

	// 计算唯一行数量（去重）
	lineMap := make(map[string]struct{}, totalLines)
	for _, line := range lines {
		lineMap[line] = struct{}{}
	}
	uniqueLines := len(lineMap)

	totalChars := len(text)
	if totalChars == 0 {
		return false
	}

	// 统计非空白字符数量
	nonWhitespaceChars := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			nonWhitespaceChars++
		}
	}

	// 计算语义密度
	density := (float64(uniqueLines) / float64(max(totalLines, 1))) *
		(float64(nonWhitespaceChars) / float64(max(totalChars, 1)))

	return density < c.threshold
}
