package core

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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
		sb.WriteString(securityEscape(resource.RelativePath))
		sb.WriteString("\" />\n")
	}
	sb.WriteString("</skill-resources>")
	return sb.String()
}

func securityEscape(s string) string {
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

func (s SkillPromptBuilder) appendSkillEntry(sb *strings.Builder, skill *SkillDefinition) {
	sb.WriteString("<skill>\n")
	sb.WriteString("  <name>")
	sb.WriteString(html.EscapeString(skill.Name))
	sb.WriteString("</name>\n")
	sb.WriteString("  <kind>")
	sb.WriteString(html.EscapeString(skill.Kind.ToString()))
	sb.WriteString("</kind>\n")
	sb.WriteString("  <description>")
	sb.WriteString(html.EscapeString(skill.Description))
	sb.WriteString("</description>\n")
	sb.WriteString("  <location>")
	sb.WriteString(html.EscapeString(skill.Location))
	sb.WriteString("</location>\n")

	if skill.MetaPriority > 0 {
		sb.WriteString("  <meta-priority>")
		fmt.Fprintf(sb, "%d", skill.MetaPriority)
		sb.WriteString("</meta-priority>\n")
	}

	if len(skill.Triggers) > 0 {
		sb.WriteString("  <triggers>\n")
		for _, trigger := range skill.Triggers {
			sb.WriteString("    <trigger>")
			sb.WriteString(html.EscapeString(trigger))
			sb.WriteString("</trigger>\n")
		}

		sb.WriteString("  </triggers>\n")
	}

	if len(skill.Resources) > 0 {
		sb.WriteString("  <resources>\n")

		for _, resource := range skill.Resources {
			kind := "script"
			if resource.Kind == SkillResourceKind_Reference {
				kind = "reference"
			}
			sb.WriteString("    <resource kind=\"")
			sb.WriteString(kind)
			sb.WriteString("\" path=\"")
			sb.WriteString(html.EscapeString(resource.RelativePath))
			sb.WriteString("\" />\n")
		}
		sb.WriteString("  </resources>\n")
	}

	sb.WriteString("</skill>\n")
}

func (s SkillPromptBuilder) Build(skills []SkillDefinition) string {
	modelSkills := make([]SkillDefinition, 0)
	for _, skill := range skills {
		if !skill.DisableModelInvocation {
			modelSkills = append(modelSkills, skill)
		}
	}

	if len(modelSkills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("<available-skills>\n")
	sb.WriteString("The following skills are available to help you complete tasks. Use them when relevant.\n")
	sb.WriteString("\n")

	for _, skill := range modelSkills {
		s.appendSkillEntry(&sb, &skill)
	}

	sb.WriteString("</available-skills>\n")
	sb.WriteString("\n")
	sb.WriteString("<skill-instructions>\n")

	for _, skill := range modelSkills {
		if len(skill.Instructions) == 0 {
			continue
		}

		sb.WriteString("\n")
		sb.WriteString("## Skill: ")
		sb.WriteString(skill.Name)
		sb.WriteString("\n")
		sb.WriteString(skill.Instructions)
		sb.WriteString("\n")
	}

	sb.WriteString("</skill-instructions>\n")

	return sb.String()
}

var DefaultIndexTemplate string = `
        <available-skills>
        The following skills are available. Only metadata and a resource manifest are shown here.
        {load_instruction}{resource_instruction}Only load what is needed, when it is needed.

        {skills}
        </available-skills>
        `

var LoadInstructionFragment string = "Call the `load_skill` tool with a skill name to fetch its full instructions on demand.\n"

var ResourceInstructionFragment string = "Call the `read_skill_resource` tool with a skill name and resource path to fetch a single reference or script body. " +
	"Only paths listed inside that skill's <resources> manifest are valid; if a skill has no <resources> node, do not call this tool for it. " +
	"Note: `SKILL.md` is the skill body itself — use `load_skill` to fetch it, never `read_skill_resource`.\n"

var SkillsPlaceholder string = "{skills}"

func (s SkillPromptBuilder) BuildIndex(skills []SkillDefinition, template string) string {
	modelSkills := make([]SkillDefinition, 0)
	hasResources := false
	for _, skill := range skills {
		if !skill.DisableModelInvocation {
			modelSkills = append(modelSkills, skill)
			if len(skill.Resources) > 0 {
				hasResources = true
			}
		}
	}

	if len(modelSkills) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, skill := range modelSkills {
		s.appendSkillEntry(&sb, &skill)
	}

	var skillsBlock = strings.TrimRight(sb.String(), "\r\n")

	effectiveTemplate := template
	if template == "" {
		effectiveTemplate = DefaultIndexTemplate
	}

	if !strings.Contains(effectiveTemplate, SkillsPlaceholder) {
		return ""
	}

	resourceVal := ""
	if hasResources {
		resourceVal = ResourceInstructionFragment
	}

	replacer := strings.NewReplacer(
		"{load_instruction}", LoadInstructionFragment,
		"{resource_instruction}", resourceVal,
		SkillsPlaceholder, skillsBlock,
	)

	rendered := replacer.Replace(effectiveTemplate)

	// Preserve the original behaviour of leading/trailing newlines around the section
	// so callers can append it directly to the base prompt with a single separator.
	return "\n" + rendered + "\n"
}

func (s SkillPromptBuilder) BuildSummary(skills []SkillDefinition) string {
	if len(skills) == 0 {
		return "No skills loaded."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Loaded skills (%d):", len(skills)))

	for _, skill := range skills {
		flags := make([]string, 0)
		if skill.DisableModelInvocation {
			flags = append(flags, "no-model")
		}
		if !skill.UserInvocable {
			flags = append(flags, "no-slash")
		}
		if skill.Metadata.Always {
			flags = append(flags, "always")
		}
		if skill.CommandDispatch != nil {
			flags = append(flags, fmt.Sprintf("dispatch:%s", *skill.CommandDispatch))
		}
		if skill.Kind == SkillKind_Meta {
			flags = append(flags, "kind:meta")
		}
		if skill.MetaPriority > 0 {
			flags = append(flags, fmt.Sprintf("meta-priority:%d", skill.MetaPriority))
		}

		flagStr := ""
		if len(flags) > 0 {
			flagStr = fmt.Sprintf(" [%s]", strings.Join(flags, ", "))
		}
		fmt.Fprintf(&sb, "  - %s: %s%s (%s)", skill.Name, skill.Description, flagStr, skill.Source.ToString())
	}

	return sb.String()
}
