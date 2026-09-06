package skillkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/util"
)

type StepBuilder struct {
	Id          string
	Name        string
	Type        string
	Description string
}

func (t *StepBuilder) Set(key, value string) {
	switch key {
	case "name":
		t.Name = value
	case "type":
		t.Type = value
	case "description":
		t.Description = value
	}
}

func (t *StepBuilder) ToStep() SkillWorkflowStep {
	return SkillWorkflowStep{
		Id:          t.Id,
		Name:        t.Name,
		Type:        SkillManifestParseStepType(t.Type),
		Description: t.Description,
	}
}

func SkillManifestParseStepType(value string) SkillWorkflowStepType {
	switch strings.ToLower(value) {
	case "input":
		return StepTypeInput

	case "generation":
		return StepTypeGeneration
	case "validation":
		return StepTypeValidation
	case "approval":
		return StepTypeApproval
	case "output":
		return StepTypeOutput
	default:
		return StepTypeReasoning
	}
}

func SkillManifestWrite(ctx context.Context, path string, manifest *SkillManifest) error {
	if manifest == nil {
		return errors.New("manifest can not be nil")
	}

	fullpath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fullpath), 0755); err != nil {
		return err
	}

	return util.SaveFile(ctx, path, SkillManifestSerialize(manifest))
}

func SkillManifestRead(ctx context.Context, path string) (*SkillManifest, error) {
	return util.LoadOneFile[SkillManifest](ctx, path)
}

func SkillManifestSerialize(manifest *SkillManifest) string {
	if manifest == nil {
		return ""
	}

	sb := &strings.Builder{}

	skillManifestWriteScalar(sb, 0, "id", manifest.Id)
	skillManifestWriteScalar(sb, 0, "name", manifest.Name)
	skillManifestWriteScalar(sb, 0, "version", manifest.Version)
	skillManifestWriteScalar(sb, 0, "category", manifest.Category)
	skillManifestWriteScalar(sb, 0, "description", manifest.Description)
	skillManifestWriteList(sb, 0, "aliases", manifest.Aliases)
	sb.WriteString("intent:\n")
	skillManifestWriteScalar(sb, 2, "outcome", manifest.Intent.Outcome)
	sb.WriteString("inputs:\n")
	skillManifestWriteList(sb, 2, "required", manifest.Inputs.Required)
	skillManifestWriteList(sb, 2, "optional", manifest.Inputs.Optional)
	sb.WriteString("outputs:\n")
	skillManifestWriteList(sb, 2, "required", manifest.Outputs.Required)
	skillManifestWriteList(sb, 2, "optional", manifest.Outputs.Optional)
	sb.WriteString("tools:\n")
	skillManifestWriteList(sb, 2, "allowed", manifest.Tools.Allowed)
	skillManifestWriteList(sb, 2, "forbidden", manifest.Tools.Forbidden)
	skillManifestWriteList(sb, 2, "approval_required", manifest.Tools.ApprovalRequired)
	sb.WriteString("guardrails:\n")
	skillManifestWriteList(sb, 2, "must_not", manifest.Guardrails.MustNot)
	sb.WriteString("human_approval:\n")
	skillManifestWriteList(sb, 2, "required_for", manifest.HumanApproval.RequiredFor)
	sb.WriteString("validation:\n")
	skillManifestWriteList(sb, 2, "checks", manifest.Validation.Checks)
	sb.WriteString("workflow:\n")
	sb.WriteString("  steps:\n")

	for _, step := range manifest.Workflow.Steps {
		sb.WriteString("    - id: ")
		sb.WriteString(skillManifestEncodeScalar(step.Id))
		sb.WriteString("\n")
		skillManifestWriteScalar(sb, 6, "name", step.Name)
		skillManifestWriteScalar(sb, 6, "type", stepTypeToYaml(step.Type))
		skillManifestWriteScalar(sb, 6, "description", step.Description)
	}

	return sb.String()
}

func skillManifestWriteList(sb *strings.Builder, indent int, key string, values []string) {
	sb.WriteString(strings.Repeat(" ", indent))
	sb.WriteString(key)
	sb.WriteString(":\n")
	for _, value := range values {
		sb.WriteString(strings.Repeat(" ", indent+2))
		sb.WriteString("- ")
		sb.WriteString(skillManifestEncodeScalar(value))
		sb.WriteString("\n")

	}
}

func skillManifestWriteScalar(sb *strings.Builder, indent int, key, value string) {
	sb.WriteString(strings.Repeat(" ", indent))

	// Appending key, delimiter, encoded value, and newline
	sb.WriteString(key)
	sb.WriteString(": ")
	sb.WriteString(skillManifestEncodeScalar(value))
	sb.WriteString("\n")
}

func skillManifestEncodeScalar(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "\"\""
	}

	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
	)

	escaped := replacer.Replace(value)

	return fmt.Sprintf("\"%s\"", escaped)
}

func stepTypeToYaml(wfType SkillWorkflowStepType) string {
	switch wfType {
	case StepTypeInput:
		return "input"
	case StepTypeGeneration:
		return "generation"
	case StepTypeValidation:
		return "validation"
	case StepTypeApproval:
		return "approval"
	case StepTypeOutput:
		return "output"
	default:
		return "reasoning"
	}
}

func SkillManifestDeserialize(text string) *SkillManifest {
	values := map[string]string{}
	lists := map[string][]string{}
	steps := []StepBuilder{}
	section := ""
	listKey := ""
	var currentStep *StepBuilder

	for _, rawLine := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if rawLine == "" || strings.HasPrefix(util.TrimStart(rawLine), "#") {
			continue
		}
		var trimmed = strings.TrimSpace(rawLine)
		var indent = len(rawLine) - len(strings.TrimLeft(rawLine, " "))
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
			var key = trimmed[:len(trimmed)-2]
			if indent == 0 {
				section = key
				listKey = key
				currentStep = nil
			} else if section == "workflow" && key == "steps" {
				listKey = "workflow.steps"
			} else {
				listKey = skillManifestCombine(section, key)
				if _, ok := lists[listKey]; !ok {
					lists[listKey] = []string{}
				}
			}

			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			var item = strings.TrimSpace(trimmed[2:])
			if section == "workflow" && listKey == "workflow.steps" && strings.HasPrefix(item, "id:") {
				currentStep = &StepBuilder{Id: skillManifestDecodeScalar(strings.TrimSpace(item[3:]))}
				steps = append(steps, *currentStep)
				continue
			}

			if listKey != "" {
				if _, ok := lists[listKey]; !ok {
					lists[listKey] = []string{}
				}
				lists[listKey] = append(lists[listKey], skillManifestDecodeScalar(item))
			}

			continue
		}

		var colonIndex = strings.Index(trimmed, ":")
		if colonIndex < 0 {
			continue
		}

		var scalarKey = strings.TrimSpace(trimmed[:colonIndex])
		var scalarValue = skillManifestDecodeScalar(strings.TrimSpace(trimmed[(colonIndex + 1):]))
		if section == "workflow" && currentStep != nil {
			currentStep.Set(scalarKey, scalarValue)
			continue
		}

		fullKey := scalarKey
		if indent != 0 {
			fullKey = skillManifestCombine(section, scalarKey)
		}

		values[fullKey] = scalarValue
	}

	wfsteps := []SkillWorkflowStep{}
	for _, step := range steps {
		wfsteps = append(wfsteps, step.ToStep())
	}
	return &SkillManifest{
		Id:          skillManifestGet(values, "id", ""),
		Name:        skillManifestGet(values, "name", ""),
		Version:     skillManifestGet(values, "version", "0.1.0"),
		Category:    skillManifestGet(values, "category", "general"),
		Description: skillManifestGet(values, "description", ""),
		Aliases:     skillManifestGetList(lists, "aliases"),
		Intent:      SkillIntent{Outcome: skillManifestGet(values, "intent.outcome", "")},
		Inputs: SkillInputs{
			Required: skillManifestGetList(lists, "inputs.required"),
			Optional: skillManifestGetList(lists, "inputs.optional"),
		},
		Outputs: SkillOutputs{
			Required: skillManifestGetList(lists, "outputs.required"),
			Optional: skillManifestGetList(lists, "outputs.optional"),
		},
		Tools: SkillToolPolicy{
			Allowed:          skillManifestGetList(lists, "tools.allowed"),
			Forbidden:        skillManifestGetList(lists, "tools.forbidden"),
			ApprovalRequired: skillManifestGetList(lists, "tools.approval_required"),
		},
		Guardrails:    SkillGuardrails{MustNot: skillManifestGetList(lists, "guardrails.must_not")},
		HumanApproval: SkillHumanApprovalPolicy{RequiredFor: skillManifestGetList(lists, "human_approval.required_for")},
		Validation:    SkillValidationPolicy{Checks: skillManifestGetList(lists, "validation.checks")},
		Workflow:      SkillWorkflow{Steps: wfsteps},
	}
}

func skillManifestCombine(section, key string) string {
	if section == "" {
		return key
	}

	return fmt.Sprintf("%s.%s", section, key)
}

func skillManifestDecodeScalar(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) >= 2 && runes[0] == '"' && runes[len(runes)-1] == '"' {
		runes = runes[1 : len(runes)-1]
	}

	var builder strings.Builder
	builder.Grow(len(runes))

	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++
			next := runes[i]
			switch next {
			case 'n':
				builder.WriteRune('\n')
			case '"':
				builder.WriteRune('"')
			case '\\':
				builder.WriteRune('\\')
			default:
				builder.WriteRune(next)
			}
			continue
		}

		builder.WriteRune(runes[i])
	}

	return builder.String()
}

func skillManifestGet(values map[string]string, key, fallback string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return fallback
}

func skillManifestGetList(values map[string][]string, key string) []string {
	if value, ok := values[key]; ok {
		return value
	}
	return []string{}
}

func SkillManifestSerializeWorkflow(workflow SkillWorkflow) string {
	builder := &strings.Builder{}
	builder.WriteString("steps:\n")
	for _, step := range workflow.Steps {
		builder.WriteString("  - id: ")
		builder.WriteString(skillManifestEncodeScalar(step.Id))
		builder.WriteString("\n")

		skillManifestWriteScalar(builder, 4, "name", step.Name)
		skillManifestWriteScalar(builder, 4, "type", stepTypeToYaml(step.Type))
		skillManifestWriteScalar(builder, 4, "description", step.Description)
	}

	return builder.String()
}

func SkillManifestSerializeTools(tools SkillToolPolicy) string {
	builder := &strings.Builder{}
	skillManifestWriteList(builder, 0, "allowed", tools.Allowed)
	skillManifestWriteList(builder, 0, "forbidden", tools.Forbidden)
	skillManifestWriteList(builder, 0, "approval_required", tools.ApprovalRequired)
	return builder.String()
}
