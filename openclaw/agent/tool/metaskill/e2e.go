package metaskill

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/util"
)

type MetaSkillRuntimeE2ERunTool struct {
}

func (e *MetaSkillRuntimeE2ERunTool) Name() string {
	return "meta_skill_runtime_e2e_run"
}

func (e *MetaSkillRuntimeE2ERunTool) Description() string {
	return "Run candidate meta skill against no-meta baseline and return gate result."
}

func (e *MetaSkillRuntimeE2ERunTool) ParameterSchema() string {
	return `
{
	"type": "object",
	"properties": {
		"skill_md": {
			"type": "string"
		},
		"eval_prompts": {
			"type": "string"
		},
		"baseline_model": {
			"type": "string"
		}
	},
	"required": ["skill_md"]
}
"`
}

type RunnerFunc func(ctx context.Context, mode, prompt, skillMd, baselineModel string) (map[string]string, error)

type JudgeFunc func(ctx context.Context, prompt string, meta, baseline map[string]string) (map[string]string, error)

type MetaSkillRuntimeE2EContext struct {
	Runner RunnerFunc
	Judge  JudgeFunc
}

type contextKey struct{}

func PushContext(parent context.Context, runtimeCtx *MetaSkillRuntimeE2EContext) context.Context {
	return context.WithValue(parent, contextKey{}, runtimeCtx)
}

func FromContext(ctx context.Context) (*MetaSkillRuntimeE2EContext, bool) {
	runtimeCtx, ok := ctx.Value(contextKey{}).(*MetaSkillRuntimeE2EContext)
	return runtimeCtx, ok
}

type RuntimeCase struct {
	Prompt     string            `json:"prompt"`
	Winner     string            `json:"winner"`
	Regression string            `json:"regression"`
	Reason     string            `json:"reason"`
	Meta       map[string]string `json:"meta"`
	Baseline   map[string]string `json:"baseline"`
}

func (a *MetaSkillRuntimeE2ERunTool) Execute(ctx context.Context, argumentsJson string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}

	if strings.TrimSpace(argumentsJson) == "" {
		argumentsJson = "{}"
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return err.Error()
	}

	skillMd := util.Deref(util.GetString(args, "skill_md"))
	if strings.TrimSpace(skillMd) == "" {
		return SerializeError("invalid_arguments", "'skill_md' is required.")
	}

	evalPromptsRaw := util.Deref(util.GetString(args, "eval_prompts"))
	baselineModel := util.Deref(util.GetString(args, "baseline_model"))

	runtimeCtx, ok := FromContext(ctx)
	if !ok || runtimeCtx == nil {
		return `{"status":"unavailable","passed":false,"winner":"","reason":"runtime_e2e_context_unavailable","cases":[]}`
	}

	prompts := normalisePrompts(evalPromptsRaw, skillMd)
	cases := make([]RuntimeCase, 0, len(prompts))
	winners := make([]string, 0, len(prompts))

	for _, prompt := range prompts {
		meta, err := runtimeCtx.Runner(ctx, "meta", prompt, skillMd, baselineModel)
		if err != nil {
			return err.Error()
		}

		baseline, err := runtimeCtx.Runner(ctx, "baseline", prompt, skillMd, baselineModel)
		if err != nil {
			return err.Error()
		}

		baselineInvalidReason := checkBaselineInvalidReason(baseline)
		if baselineInvalidReason != "" {
			winners = append(winners, "invalid")
			cases = append(cases, RuntimeCase{
				Prompt:     prompt,
				Winner:     "invalid",
				Regression: baselineInvalidReason,
				Reason:     "Baseline comparison was invalid because the no-meta route returned an error/refusal instead of its strongest standalone answer.",
				Meta:       meta,
				Baseline:   baseline,
			})
			continue
		}

		verdict, err := runtimeCtx.Judge(ctx, prompt, meta, baseline)
		if err != nil {
			return err.Error()
		}

		winner := normaliseWinner(getDictString(verdict, "winner"))
		winners = append(winners, winner)

		regression := getDictString(verdict, "regression")
		if strings.TrimSpace(regression) == "" {
			regression = getDictString(verdict, "required_improvements")
		}

		cases = append(cases, RuntimeCase{
			Prompt:     prompt,
			Winner:     winner,
			Regression: regression,
			Reason:     getDictString(verdict, "reason"),
			Meta:       meta,
			Baseline:   baseline,
		})
	}

	blocked := false
	for _, c := range cases {
		if (c.Winner != "meta" && c.Winner != "tie") || strings.TrimSpace(c.Regression) != "" {
			blocked = true
			break
		}
	}

	aggregateWinner := "tie"
	if slices.Contains(winners, "invalid") {
		aggregateWinner = "invalid"
	} else if slices.Contains(winners, "baseline") {
		aggregateWinner = "baseline"
	} else if slices.Contains(winners, "meta") {
		aggregateWinner = "meta"
	}

	resultMap := map[string]any{
		"status":         "ok",
		"passed":         !blocked,
		"winner":         aggregateWinner,
		"baseline_model": baselineModel,
		"cases":          cases,
	}

	resBytes, err := json.Marshal(resultMap)
	if err != nil {
		return err.Error()
	}

	return string(resBytes)
}

func checkBaselineInvalidReason(baseline map[string]string) string {
	text := strings.ToLower(strings.TrimSpace(getDictString(baseline, "text")))
	errStr := strings.TrimSpace(getDictString(baseline, "error"))
	if errStr != "" {
		return "baseline_error"
	}

	refusalMarkers := []string{
		"runtime e2e baseline mode",
		"meta-skill creator tools are disabled",
		"meta_skill creator tools are disabled",
		"meta_skill_* creator tools are disabled",
		"i cannot complete this request",
		"i can’t complete this request",
	}

	for _, marker := range refusalMarkers {
		if strings.Contains(text, marker) {
			return "baseline_invalid_or_blocked"
		}
	}

	return ""
}

func normaliseWinner(value string) string {
	val := strings.ToLower(strings.TrimSpace(value))
	switch val {
	case "orchestrated", "meta-skill", "metaskill":
		return "meta"
	case "no-meta", "single-model":
		return "baseline"
	default:
		if val == "" {
			return "tie"
		}
		return val
	}
}

func normalisePrompts(evalPromptsRaw, skillMd string) []string {
	if strings.TrimSpace(evalPromptsRaw) != "" {
		text := strings.TrimSpace(evalPromptsRaw)
		var arr []string
		if err := json.Unmarshal([]byte(text), &arr); err == nil {
			var prompts []string
			for _, item := range arr {
				if strings.TrimSpace(item) != "" {
					prompts = append(prompts, item)
				}
			}
			if len(prompts) > 0 {
				return prompts
			}
		}

		lines := strings.Split(text, "\n")
		var promptLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				promptLines = append(promptLines, trimmed)
			}
		}
		if len(promptLines) > 0 {
			return promptLines
		}
	}

	re := regexp.MustCompile(`triggers:\s*\n(?:\s*-\s*"?([^"\n]+)"?\s*\n?)`)
	matches := re.FindStringSubmatch(skillMd)
	trigger := "this meta skill"
	if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
		trigger = strings.TrimSpace(matches[1])
	}

	return []string{fmt.Sprintf("please use %s", trigger)}
}

func getDictString(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}
