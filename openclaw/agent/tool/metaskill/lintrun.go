package metaskill

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type MetaSkillLintRunTool struct {
}

func (e *MetaSkillLintRunTool) Name() string {
	return "meta_skill_lint_run"
}

func (e *MetaSkillLintRunTool) Description() string {
	return "Run creator lint gates and return JSON summary."
}

func (e *MetaSkillLintRunTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"skill_md": {
			"type": "string"
		},
		"gates": {
			"type": "string"
		}
	},
	"required": ["skill_md"]
}
"`
}

type LintRunModel struct {
	SkillMd string `json:"skill_md"`
	Gates   string `json:"gates"`
}

func (a *MetaSkillLintRunTool) Execute(ctx context.Context, argumentsJson string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}

	if argumentsJson == "" {
		argumentsJson = "{}"
	}

	var doc LintRunModel

	if err := json.Unmarshal([]byte(argumentsJson), &doc); err != nil {
		return err.Error()
	}

	if doc.SkillMd == "" {
		return SerializeError("invalid_arguments", "'skill_md' is required.")
	}

	failed := []string{}
	var g1Passed = strings.Contains(doc.SkillMd, "kind: meta") && strings.Contains(doc.SkillMd, "composition:") && strings.Contains(doc.SkillMd, "  steps:") && strings.Contains(doc.SkillMd, "    - id:")
	if !g1Passed {
		failed = append(failed, "G1")
	}

	var g2Passed = g1Passed && !hasInvalidDependency(doc.SkillMd)
	if !g2Passed {
		failed = append(failed, "G2")
	}

	var passed = len(failed) == 0
	summary := "G1,G2 passed"
	if !passed {
		summary = fmt.Sprintf("Lint failed for gates: %s", strings.Join(failed, ","))
	}

	return SerializeLintResult(passed, failed, summary)
}

var (
	idLintRunRegex = regexp.MustCompile(`(?m)^\s*-\s+id:\s*([a-zA-Z0-9_-]+)\s*$`)
	depRegex       = regexp.MustCompile(`(?m)^\s*depends_on:\s*\[([^\]]*)\]\s*$`)
)

func hasInvalidDependency(skillMd string) bool {
	ids := make(map[string]struct{})
	idMatches := idLintRunRegex.FindAllStringSubmatch(skillMd, -1)
	for _, match := range idMatches {
		if len(match) > 1 {
			ids[match[1]] = struct{}{}
		}
	}

	depMatches := depRegex.FindAllStringSubmatch(skillMd, -1)
	for _, match := range depMatches {
		if len(match) > 1 {
			raw := match[1]
			tokens := strings.Split(raw, ",")
			for _, token := range tokens {
				dependency := strings.TrimSpace(token)
				if dependency == "" {
					continue
				}
				if _, exists := ids[dependency]; !exists {
					return true
				}
			}
		}
	}

	return false
}
