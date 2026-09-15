package reduction

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/pluginkit/matching"
	"github.com/futugyou/openclaw/pluginkit/rules"
)

var _ core.IToolResultInterceptor = (*TokenJuiceInterceptor)(nil)

type TokenJuiceInterceptor struct {
	rules          []rules.TokenJuiceRule
	density        *SemanticDensityCalculator
	maxInlineChars *int
}

// NewTokenJuiceInterceptor 构造函数
func NewTokenJuiceInterceptor(
	rules []rules.TokenJuiceRule,
	density *SemanticDensityCalculator,
	maxInlineChars *int,
) *TokenJuiceInterceptor {
	if density == nil {
		density = NewSemanticDensityCalculator()
	}
	return &TokenJuiceInterceptor{
		rules:          rules,
		density:        density,
		maxInlineChars: maxInlineChars,
	}
}

func (t *TokenJuiceInterceptor) GetOrder() int {
	return 100
}

func (t *TokenJuiceInterceptor) GetName() string {
	return "TokenJuice"
}

// Intercept 实现异步/管道拦截逻辑 (Go 中通过 context 传递取消信号)
func (t *TokenJuiceInterceptor) Intercept(ctx context.Context, reqContext core.ReductionContext) (string, error) {
	// 校验 context 是否已被取消
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	rawOutput := reqContext.RawOutput

	if reqContext.BypassReduction {
		return rawOutput, nil
	}

	// Escape hatch: --raw / --full
	argsJson := reqContext.ArgumentsJSON
	if strings.Contains(argsJson, "--raw") || strings.Contains(argsJson, "--full") {
		return rawOutput, nil
	}

	command := extractCommand(argsJson)
	argv := matching.ParseCommandArgv(command, nil)
	exitCode := reqContext.ExitCode

	// Rule matching
	rule := matching.SelectRule(t.rules, reqContext.ToolName, command, argv, rawOutput, exitCode)

	if rule != nil {
		// rule.Failure?.PreserveOnFailure ?? false
		preserveOnFailure := false
		if rule.Failure != nil {
			preserveOnFailure = rule.Failure.PreserveOnFailure
		}

		if reqContext.IsError && !preserveOnFailure {
			return rawOutput, nil
		}

		summary, facts := Reduce(*rule, rawOutput, exitCode)
		if summary != "" {
			formatted := Format(summary, facts, exitCode, t.maxInlineChars)
			if len(formatted) < len(rawOutput) {
				return formatted, nil
			}
		}
	} else if !reqContext.IsError && t.density.ShouldReduce(rawOutput) {
		var fallback *rules.TokenJuiceRule
		for i := range t.rules {
			if t.rules[i].ID == "generic/fallback" {
				fallback = &t.rules[i]
				break
			}
		}

		if fallback != nil {
			summary, facts := Reduce(*fallback, rawOutput, exitCode)
			if summary != "" {
				formatted := Format(summary, facts, exitCode, t.maxInlineChars)
				if len(formatted) < len(rawOutput) {
					return formatted, nil
				}
			}
		}
	}

	return rawOutput, nil
}

func extractCommand(argumentsJson string) *string {
	if !strings.Contains(argumentsJson, `"command"`) {
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJson), &data); err != nil {
		return nil
	}

	if val, ok := data["command"]; ok {
		if cmdStr, isString := val.(string); isString {
			return &cmdStr
		}
	}

	return nil
}
