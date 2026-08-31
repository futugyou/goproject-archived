package metaskill

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type MetaSkillSmokeRunTool struct {
}

func (e *MetaSkillSmokeRunTool) Name() string {
	return "meta_skill_smoke_run"
}

func (e *MetaSkillSmokeRunTool) Description() string {
	return "Run G3/G4 smoke tests and return JSON summary."
}

func (e *MetaSkillSmokeRunTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"skill_md": {
			"type": "string"
		},
		"fixture_gen_model": {
			"type": "string"
		},
		"classifier_model": {
			"type": "string"
		}
	},
	"required": ["skill_md"]
}
"`
}

type SmokeRunModel struct {
	SkillMd         string `json:"skill_md"`
	FixtureGenModel string `json:"fixture_gen_model"`
	ClassifierModel string `json:"classifier_model"`
}

type SmokeRunResult struct {
	G3       SmokeRunItem `json:"G3"`
	G4       SmokeRunItem `json:"G4"`
	Degraded bool         `json:"degraded"`
}

type SmokeRunItem struct {
	Passed          bool   `json:"passed"`
	PositiveFixture string `json:"positive_fixture,omitempty"`
	Classifier      string `json:"classifier"`
	NegativeFixture string `json:"negative_fixture,omitempty"`
	Degraded        bool   `json:"degraded"`
}

func (a *MetaSkillSmokeRunTool) Execute(ctx context.Context, argumentsJson string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}

	if argumentsJson == "" {
		argumentsJson = "{}"
	}

	var doc SmokeRunModel

	if err := json.Unmarshal([]byte(argumentsJson), &doc); err != nil {
		return err.Error()
	}

	if doc.SkillMd == "" {
		return SerializeError("invalid_arguments", "'skill_md' is required.")
	}

	if doc.ClassifierModel == "" {
		doc.ClassifierModel = "stub"
	}
	positive, err := DeterministicFixture(doc.SkillMd, "positive")
	if err != nil {
		return err.Error()
	}
	negative, err := DeterministicFixture(doc.SkillMd, "negative")
	if err != nil {
		return err.Error()
	}

	var g3Passed = SimulateMetaResolution(doc.SkillMd, positive)
	var g4Passed = !SimulateMetaResolution(doc.SkillMd, negative)

	result := SmokeRunResult{
		G3: SmokeRunItem{
			Passed:          g3Passed,
			PositiveFixture: positive,
			Classifier:      doc.ClassifierModel,
			Degraded:        true,
		},
		G4: SmokeRunItem{
			Passed:          g4Passed,
			NegativeFixture: negative,
			Classifier:      doc.ClassifierModel,
			Degraded:        true,
		},
		Degraded: true,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

var (
	doubleQuotedRegex  = regexp.MustCompile(`(?i)triggers:\s*\n((?:\s*-\s*"[^"]+"\s*\n)+)`)
	firstDoubleRegex   = regexp.MustCompile(`-\s*"([^"]+)"`)
	unquotedRegex      = regexp.MustCompile(`(?i)triggers:\s*\n((?:\s*-\s*[^"\n]+\n)+)`)
	firstUnquotedRegex = regexp.MustCompile(`-\s*([^"\n]+)`)
)

func DeterministicFixture(skillMd string, kind string) (string, error) {
	if kind == "positive" {
		if match := doubleQuotedRegex.FindStringSubmatch(skillMd); len(match) > 1 {
			if first := firstDoubleRegex.FindStringSubmatch(match[1]); len(first) > 1 {
				raw := first[1]
				// Unescape JSON-like backslashes
				replacer := strings.NewReplacer(`\\`, `\`, `\"`, `"`)
				return fmt.Sprintf("please use %s", replacer.Replace(raw)), nil
			}
		}

		if match := unquotedRegex.FindStringSubmatch(skillMd); len(match) > 1 {
			if first := firstUnquotedRegex.FindStringSubmatch(match[1]); len(first) > 1 {
				return fmt.Sprintf("please use %s", strings.TrimSpace(first[1])), nil
			}
		}

		return "please run this meta-skill", nil
	}

	if kind == "negative" {
		return "what's the weather forecast for tomorrow?", nil
	}

	return "", fmt.Errorf("Unknown fixture kind: %s", kind)
}

func SimulateMetaResolution(skillMd string, prompt string) bool {
	triggers := ExtractTriggers(skillMd)
	if len(triggers) == 0 {
		return false
	}

	promptLower := strings.ToLower(prompt)
	for _, trigger := range triggers {
		if TriggerMatches(trigger, promptLower) {
			return true
		}
	}

	return false
}

func ExtractTriggers(skillMd string) []string {
	lines := strings.FieldsFunc(skillMd, func(r rune) bool {
		return r == '\r' || r == '\n'
	})

	var triggers []string
	inTriggers := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if !inTriggers {
			if strings.EqualFold(line, "triggers:") {
				inTriggers = true
			}
			continue
		}

		if !strings.HasPrefix(line, "-") {
			break
		}

		value := strings.TrimSpace(line[1:])
		if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}

		if strings.TrimSpace(value) != "" {
			triggers = append(triggers, value)
		}
	}

	return triggers
}

func TriggerMatches(trigger string, promptLower string) bool {
	needle := strings.ToLower(strings.TrimSpace(trigger))
	if len(needle) == 0 {
		return false
	}

	if strings.Contains(promptLower, needle) {
		return true
	}

	// Check for non-ASCII characters (> 127)
	for _, ch := range needle {
		if ch > 127 {
			return false
		}
	}

	// Build word boundary regex pattern: \b<escaped_needle>\b with flexible spaces
	escaped := regexp.QuoteMeta(needle)
	pattern := `\b` + strings.ReplaceAll(escaped, `\ `, `\s+`) + `\b`

	matched, err := regexp.MatchString(pattern, promptLower)
	if err != nil {
		return false
	}

	return matched
}
