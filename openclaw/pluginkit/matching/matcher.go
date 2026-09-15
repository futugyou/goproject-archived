package matching

import (
	"regexp"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/pluginkit/rules"
)

// SelectRule 遍历规则列表，返回第一个匹配成功的目标规则
func SelectRule(
	rules []rules.TokenJuiceRule,
	toolName string,
	command *string,
	argv []string,
	content string,
	exitCode int,
) *rules.TokenJuiceRule {
	for i := range rules {
		if RuleMatches(&rules[i], toolName, command, argv, content, exitCode) {
			return &rules[i]
		}
	}
	return nil
}

// RuleMatches 检查单条规则是否匹配当前上下文
func RuleMatches(
	rule *rules.TokenJuiceRule,
	toolName string,
	command *string,
	argv []string,
	content string,
	exitCode int,
) bool {
	match := rule.Match
	if match == nil {
		return true
	}

	normalizedTool := toolName
	if command != nil {
		normalizedTool = "exec"
	}

	if len(match.ToolNames) > 0 {
		found := false
		for _, name := range match.ToolNames {
			if name == normalizedTool || name == toolName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	tokens := ParseCommandArgv(command, argv)

	if len(match.Argv0) > 0 {
		if len(tokens) == 0 {
			return false
		}
		found := slices.Contains(match.Argv0, tokens[0])
		if !found {
			return false
		}
	}

	if len(match.ArgvIncludes) > 0 {
		matchedAnyGroup := false
		for _, entry := range match.ArgvIncludes {
			allFound := true
			for _, needle := range entry {
				if !containsString(tokens, needle) {
					allFound = false
					break
				}
			}
			if allFound {
				matchedAnyGroup = true
				break
			}
		}
		if !matchedAnyGroup {
			return false
		}
	}

	if len(match.ArgvIncludesAny) > 0 {
		matchedAnyGroup := false
		for _, entry := range match.ArgvIncludesAny {
			anyFound := false
			for _, needle := range entry {
				if containsString(tokens, needle) {
					anyFound = true
					break
				}
			}
			if anyFound {
				matchedAnyGroup = true
				break
			}
		}
		if !matchedAnyGroup {
			return false
		}
	}

	cmdText := ""
	if command != nil {
		cmdText = *command
	} else {
		cmdText = strings.Join(tokens, " ")
	}
	cmdLower := strings.ToLower(cmdText)

	if len(match.CommandIncludes) > 0 {
		for _, needle := range match.CommandIncludes {
			if !strings.Contains(cmdLower, strings.ToLower(needle)) {
				return false
			}
		}
	}

	if len(match.CommandIncludesAny) > 0 {
		found := false
		for _, needle := range match.CommandIncludesAny {
			if strings.Contains(cmdLower, strings.ToLower(needle)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if match.CommandRegex != nil && len(*match.CommandRegex) > 0 {
		re, err := regexp.Compile(*match.CommandRegex)
		if err != nil || !re.MatchString(cmdText) {
			return false
		}
	}

	if len(match.ExitCodes) > 0 {
		found := slices.Contains(match.ExitCodes, exitCode)
		if !found {
			return false
		}
	}

	if match.OutputRegex != nil && len(*match.OutputRegex) > 0 {
		// Go 正则表达多行匹配模式 (?m)
		re, err := regexp.Compile("(?m)" + *match.OutputRegex)
		if err != nil || !re.MatchString(content) {
			return false
		}
	}

	return true
}
