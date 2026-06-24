package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var _ ITool = (*LoadSkillTool)(nil)

type LoadSkillTool struct {
	provider func() []SkillDefinition
}

func NewLoadSkillToolFromProvider(provider func() []SkillDefinition) *LoadSkillTool {
	if provider == nil {
		panic("provider cannot be nil")
	}
	return &LoadSkillTool{provider: provider}
}

func NewLoadSkillToolFromSlice(skills []SkillDefinition) *LoadSkillTool {
	return &LoadSkillTool{
		provider: func() []SkillDefinition {
			if skills == nil {
				return []SkillDefinition{}
			}
			return skills
		},
	}
}

func (t *LoadSkillTool) Name() string {
	return "load_skill"
}

func (t *LoadSkillTool) Description() string {
	return "Load the full instructions of a named skill on demand. The system prompt only lists " +
		"skill metadata and a resource manifest; call this tool when a specific skill is " +
		"relevant to fetch its complete SKILL.md body. " +
		"Always use this tool (never `read_skill_resource`) to fetch any skill's SKILL.md, " +
		"including when another skill's body is referenced by a sibling skill."
}

func (t *LoadSkillTool) ParameterSchema() string {
	return `{"type":"object","properties":{"skill":{"type":"string","description":"Skill name to load (as listed in <available-skills>)"}},"required":["skill"]}`
}

func (t *LoadSkillTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	requested, err := tryParseSkillName(argumentsJson)
	if err != nil {
		return err.Error(), nil
	}

	var skills []SkillDefinition
	if t.provider != nil {
		skills = t.provider()
	}
	if skills == nil {
		skills = []SkillDefinition{}
	}

	match := findSkill(skills, requested)
	if match == nil {
		var availableNames []string
		for _, s := range skills {
			if !s.DisableModelInvocation {
				availableNames = append(availableNames, s.Name)
			}
		}
		available := strings.Join(availableNames, ", ")
		if available == "" {
			available = "(none)"
		}
		return fmt.Sprintf("Error: skill '%s' not found. Available: %s.", requested, available), nil
	}

	if match.DisableModelInvocation {
		return fmt.Sprintf("Error: skill '%s' is not available for model invocation.", match.Name), nil
	}

	var builder SkillPromptBuilder
	body := builder.BuildSkillBody(match)
	if len(body) == 0 {
		return fmt.Sprintf("Skill '%s' has no instructions body.", match.Name), nil
	}

	if len(match.Resources) == 0 {
		return body, nil
	}

	withManifest := strings.TrimRight(body, " \t\r\n") + "\n\n" + renderResourceManifest(match) + "\n"
	return withManifest, nil
}

func tryParseSkillName(argumentsJson string) (string, error) {
	if strings.TrimSpace(argumentsJson) == "" {
		return "", fmt.Errorf("Error: missing required argument 'skill'.")
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJson), &rawMap); err != nil {
		return "", fmt.Errorf("Error: invalid JSON arguments. Expected {\"skill\":\"<name>\"}.")
	}

	var skillName string
	aliases := []string{"skill", "skill_name", "name"}
	for _, alias := range aliases {
		if val, exists := rawMap[alias]; exists {
			if strVal, ok := val.(string); ok && strings.TrimSpace(strVal) != "" {
				skillName = strVal
				break
			}
		}
	}

	if strings.TrimSpace(skillName) == "" {
		return "", fmt.Errorf("Error: missing required argument 'skill'.")
	}

	return skillName, nil
}

func findSkill(skills []SkillDefinition, requested string) *SkillDefinition {
	for i := range skills {
		if strings.EqualFold(skills[i].Name, requested) {
			return &skills[i]
		}
	}
	for i := range skills {
		key := skills[i].Metadata.SkillKey
		if len(key) > 0 && strings.EqualFold(key, requested) {
			return &skills[i]
		}
	}
	return nil
}

func renderResourceManifest(skill *SkillDefinition) string {
	var sb strings.Builder
	sb.WriteString("<skill-resources>\n")
	for _, resource := range skill.Resources {
		kind := "script"
		if resource.Kind == SkillResourceKind_Reference {
			kind = "reference"
		}
		sb.WriteString("  <resource kind=\"")
		sb.WriteString(kind)
		sb.WriteString("\" path=\"")
		sb.WriteString(xmlEscape(resource.RelativePath))
		sb.WriteString("\" />\n")
	}
	sb.WriteString("</skill-resources>")
	return sb.String()
}

func xmlEscape(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			sb.WriteString("&amp;")
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		case '"':
			sb.WriteString("&quot;")
		case '\'':
			sb.WriteString("&apos;")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

type SkillPromptBuilder struct{}

func (s SkillPromptBuilder) BuildSkillBody(skill *SkillDefinition) string {
	if skill == nil || skill.DisableModelInvocation || len(strings.TrimSpace(skill.Instructions)) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("<skill-instructions>\n")
	sb.WriteString("\n")
	sb.WriteString("## Skill: ")
	sb.WriteString(skill.Name)
	sb.WriteString("\n")
	sb.WriteString(skill.Instructions)
	sb.WriteString("\n")
	sb.WriteString("</skill-instructions>\n")

	return sb.String()
}
