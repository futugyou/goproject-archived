package matching

import (
	"slices"
	"strings"
	"unicode"
)

// ParseCommandArgv 解析指令参数列表
func ParseCommandArgv(command *string, argv []string) []string {
	if len(argv) > 0 {
		return argv
	}
	if command == nil || strings.TrimSpace(*command) == "" {
		return []string{}
	}

	var tokens []string
	inQuotes := false
	var current strings.Builder

	for _, ch := range *command {
		if ch == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes && unicode.IsSpace(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// 辅助函数：判断字符串切片中是否包含指定字符串（区分大小写）
func containsString(slice []string, needle string) bool {
	return slices.Contains(slice, needle)
}
