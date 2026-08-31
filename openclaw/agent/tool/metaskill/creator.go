package metaskill

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type MetaSkillAssembleTool struct {
}

func (e *MetaSkillAssembleTool) Name() string {
	return "meta_skill_assemble"
}

func (e *MetaSkillAssembleTool) Description() string {
	return "Render SKILL.md from pattern_id and validated slots_json."
}

func (e *MetaSkillAssembleTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"pattern_id": {
			"type": "string"
		},
		"slots_json": {
			"type": "string"
		}
	},
	"required": ["pattern_id", "slots_json"]
}
"`
}

type AssembleModel struct {
	PatternId string `json:"pattern_id"`
	SlotsJson string `json:"slots_json"`
}

var idRegex = regexp.MustCompile("^[a-z][a-z0-9_]{0,30}$")

type CommonSlots struct {
	Name         string
	Description  string
	MetaPriority int
	Triggers     []string
}

type CreatorStep struct {
	ID       string
	Skill    string
	Task     string
	WithKeys map[string]string
}

func (a *MetaSkillAssembleTool) Execute(ctx context.Context, argumentsJson string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}

	if argumentsJson == "" {
		argumentsJson = "{}"
	}

	var doc AssembleModel

	if err := json.Unmarshal([]byte(argumentsJson), &doc); err != nil {
		return err.Error()
	}

	if !IsSupportedPattern(doc.PatternId) {
		return SerializeError("unknown_pattern_id", "Unsupported pattern_id "+doc.PatternId)
	}

	var slots map[string]any

	if err := json.Unmarshal([]byte(doc.SlotsJson), &slots); err != nil {
		return SerializeError("invalid_slots_json", "slots_json is not valid JSON.")
	}

	var result string
	var err error
	switch doc.PatternId {
	case "p1_sequential":
		result, err = RenderP1(slots)
	case "p2_fan_out_merge":
		result, err = RenderP2(slots)
	case "p3_condition_gated":
		result, err = RenderP3(slots)
	default:
		return fmt.Sprintf("Unsupported pattern_id '%s'.", doc.PatternId)
	}

	if err != nil {
		return err.Error()
	}

	return result
}
func RenderP1(slots map[string]any) (string, error) {
	common, err := ParseCommonSlots(slots)
	if err != nil {
		return "", err
	}

	stepsNode, err := RequireArray(slots, "steps")
	if err != nil {
		return "", err
	}

	if len(stepsNode) < 2 || len(stepsNode) > 5 {
		return "", fmt.Errorf("p1_sequential requires steps length between 2 and 5")
	}

	steps, err := ParseStepList(stepsNode)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	AppendHeader(&sb, common)
	sb.WriteString("composition:\n")
	sb.WriteString("  steps:\n")

	for i, step := range steps {
		kind := ResolveCreatorStepKind(step.Skill)
		fmt.Fprintf(&sb, "    - id: %s\n", step.ID)
		if kind == "skill_exec" || kind == "llm_chat" {
			fmt.Fprintf(&sb, "      kind: %s\n", kind)
		}
		fmt.Fprintf(&sb, "      skill: %s\n", ToJsonString(step.Skill))
		sb.WriteString("      read_only: true\n")
		sb.WriteString("      write_access: false\n")
		sb.WriteString("      network_access: false\n")
		if i > 0 {
			fmt.Fprintf(&sb, "      depends_on: [%s]\n", steps[i-1].ID)
		}
		sb.WriteString("      with:\n")

		if len(step.WithKeys) > 0 {
			for k, v := range step.WithKeys {
				fmt.Fprintf(&sb, "        %s: %s\n", k, ToJsonString(v))
			}
		} else if kind == "skill_exec" && step.Skill == "history-explorer" {
			fmt.Fprintf(&sb, "        query: \"%s: {{ inputs.user_message | xml_escape | truncate(512) }}\"\n", step.Task)
			sb.WriteString("        window_days: \"30\"\n")
			sb.WriteString("        include: \"co_occurrences,meta_usage\"\n")
		} else if i == 0 {
			fmt.Fprintf(&sb, "        task: \"%s: {{ inputs.user_message | xml_escape | truncate(512) }}\"\n", step.Task)
		} else {
			fmt.Fprintf(&sb, "        task: %s\n", ToJsonString(step.Task))
			sb.WriteString("        user_request: \"{{ inputs.user_message | xml_escape | truncate(1200) }}\"\n")
			sb.WriteString("        prior_outputs:\n")
			for p := range i {
				fmt.Fprintf(&sb, "          %s: \"{{ outputs.%s | truncate(2000) }}\"\n", steps[p].ID, steps[p].ID)
			}
			fmt.Fprintf(&sb, "        upstream: \"{{ outputs.%s | truncate(2000) }}\"\n", steps[i-1].ID)
		}
	}

	AppendP1Body(&sb, common.Name, common.Description, steps)
	return sb.String(), nil
}

func RenderP2(slots map[string]any) (string, error) {
	common, err := ParseCommonSlots(slots)
	if err != nil {
		return "", err
	}

	branchesNode, err := RequireArray(slots, "branches")
	if err != nil {
		return "", err
	}

	branches, err := ParseStepList(branchesNode)
	if err != nil {
		return "", err
	}

	if len(branches) < 2 || len(branches) > 4 {
		return "", fmt.Errorf("p2_fan_out_merge requires branches length between 2 and 4")
	}

	mergeNode, err := RequireObject(slots, "merge")
	if err != nil {
		return "", err
	}

	merge, err := ParseStep(mergeNode)
	if err != nil {
		return "", err
	}

	var tail *CreatorStep
	if tailNode, ok := slots["tail"].(map[string]any); ok {
		parsedTail, err := ParseStep(tailNode)
		if err != nil {
			return "", err
		}
		tail = &parsedTail
	}

	var sb strings.Builder
	AppendHeader(&sb, common)
	sb.WriteString("composition:\n")
	sb.WriteString("  steps:\n")

	for _, branch := range branches {
		AppendStepWithPayload(&sb, branch, nil, true, nil)
	}

	branchIDs := make([]string, len(branches))
	for i, b := range branches {
		branchIDs[i] = b.ID
	}

	AppendStepWithPayload(&sb, merge, branchIDs, false, branches)

	if tail != nil {
		AppendStepWithPayload(&sb, *tail, []string{merge.ID}, false, nil)
	}

	AppendP2Body(&sb, common.Name, common.Description, branches, merge, tail)
	return sb.String(), nil
}

func RenderP3(slots map[string]any) (string, error) {
	common, err := ParseCommonSlots(slots)
	if err != nil {
		return "", err
	}

	stepsNode, err := RequireArray(slots, "steps")
	if err != nil {
		return "", err
	}

	steps, err := ParseStepList(stepsNode)
	if err != nil {
		return "", err
	}

	if len(steps) < 2 || len(steps) > 5 {
		return "", fmt.Errorf("p3_condition_gated requires steps length between 2 and 5")
	}

	var sb strings.Builder
	AppendHeader(&sb, common)
	sb.WriteString("composition:\n")
	sb.WriteString("  steps:\n")

	for i, step := range steps {
		kind := ResolveCreatorStepKind(step.Skill)
		fmt.Fprintf(&sb, "    - id: %s\n", step.ID)
		if kind == "skill_exec" || kind == "llm_chat" {
			fmt.Fprintf(&sb, "      kind: %s\n", kind)
		}
		fmt.Fprintf(&sb, "      skill: %s\n", ToJsonString(step.Skill))
		sb.WriteString("      read_only: true\n")
		sb.WriteString("      write_access: false\n")
		sb.WriteString("      network_access: false\n")
		if i > 0 {
			fmt.Fprintf(&sb, "      depends_on: [%s]\n", steps[i-1].ID)
		}
		sb.WriteString("      with:\n")

		if len(step.WithKeys) > 0 {
			for k, v := range step.WithKeys {
				fmt.Fprintf(&sb, "        %s: %s\n", k, ToJsonString(v))
			}
		} else if kind == "skill_exec" && step.Skill == "history-explorer" {
			fmt.Fprintf(&sb, "        query: \"%s: {{ inputs.user_message | xml_escape | truncate(512) }}\"\n", step.Task)
			sb.WriteString("        window_days: \"30\"\n")
			sb.WriteString("        include: \"co_occurrences,meta_usage,router_fixtures\"\n")
		} else if i == 0 {
			fmt.Fprintf(&sb, "        task: \"%s: {{ inputs.user_message | xml_escape | truncate(512) }}\"\n", step.Task)
		} else {
			fmt.Fprintf(&sb, "        task: %s\n", ToJsonString(step.Task))
			sb.WriteString("        user_request: \"{{ inputs.user_message | xml_escape | truncate(1200) }}\"\n")
			sb.WriteString("        prior_outputs:\n")
			for p := range i {
				fmt.Fprintf(&sb, "          %s: \"{{ outputs.%s | truncate(2000) }}\"\n", steps[p].ID, steps[p].ID)
			}
		}
	}

	AppendP3Body(&sb, common.Name, common.Description, steps)
	return sb.String(), nil
}

func AppendStepWithPayload(
	sb *strings.Builder,
	step CreatorStep,
	dependsOn []string,
	branchMode bool,
	branchesForMerge []CreatorStep,
) {
	kind := ResolveCreatorStepKind(step.Skill)
	fmt.Fprintf(sb, "    - id: %s\n", step.ID)
	if kind == "skill_exec" || kind == "llm_chat" {
		fmt.Fprintf(sb, "      kind: %s\n", kind)
	}
	fmt.Fprintf(sb, "      skill: %s\n", ToJsonString(step.Skill))
	sb.WriteString("      read_only: true\n")
	sb.WriteString("      write_access: false\n")
	sb.WriteString("      network_access: false\n")

	if len(dependsOn) > 0 {
		fmt.Fprintf(sb, "      depends_on: [%s]\n", strings.Join(dependsOn, ", "))
	}

	sb.WriteString("      with:\n")
	if len(step.WithKeys) > 0 {
		for k, v := range step.WithKeys {
			fmt.Fprintf(sb, "        %s: %s\n", k, ToJsonString(v))
		}
		return
	}

	if kind == "skill_exec" && step.Skill == "history-explorer" {
		fmt.Fprintf(sb, "        query: \"%s: {{ inputs.user_message | xml_escape | truncate(512) }}\"\n", step.Task)
		sb.WriteString("        window_days: \"30\"\n")
		sb.WriteString("        include: \"co_occurrences,meta_usage\"\n")
		return
	}

	if branchMode {
		fmt.Fprintf(sb, "        task: \"%s: {{ inputs.user_message | xml_escape | truncate(512) }}\"\n", step.Task)
		return
	}

	fmt.Fprintf(sb, "        task: %s\n", ToJsonString(step.Task))
	if len(branchesForMerge) > 0 {
		for _, branch := range branchesForMerge {
			fmt.Fprintf(sb, "        %s_output: \"{{ outputs.%s | truncate(2000) }}\"\n", branch.ID, branch.ID)
		}
	} else if len(dependsOn) > 0 {
		fmt.Fprintf(sb, "        upstream: \"{{ outputs.%s | truncate(2000) }}\"\n", dependsOn[0])
	}
}

func ParseCommonSlots(root map[string]any) (CommonSlots, error) {
	name, err := RequireString(root, "name")
	if err != nil {
		return CommonSlots{}, err
	}

	description, err := RequireString(root, "description")
	if err != nil {
		return CommonSlots{}, err
	}

	metaPriority := 50
	if val, ok := root["meta_priority"]; ok {
		if floatVal, isNum := val.(float64); isNum {
			metaPriority = int(floatVal)
		}
	}

	if metaPriority < 30 || metaPriority > 80 {
		return CommonSlots{}, fmt.Errorf("meta_priority must be between 30 and 80")
	}

	if len(description) < 30 || len(description) > 200 {
		return CommonSlots{}, fmt.Errorf("description length must be between 30 and 200")
	}

	triggersNode, err := RequireArray(root, "triggers")
	if err != nil {
		return CommonSlots{}, err
	}

	if len(triggersNode) < 1 || len(triggersNode) > 8 {
		return CommonSlots{}, fmt.Errorf("triggers length must be between 1 and 8")
	}

	triggers := make([]string, 0, len(triggersNode))
	for _, item := range triggersNode {
		strVal, ok := item.(string)
		if !ok || strings.TrimSpace(strVal) == "" {
			return CommonSlots{}, fmt.Errorf("trigger value must be a non-empty string")
		}
		triggers = append(triggers, strVal)
	}

	return CommonSlots{
		Name:         name,
		Description:  description,
		MetaPriority: metaPriority,
		Triggers:     triggers,
	}, nil
}

func ParseStepList(arrayNode []any) ([]CreatorStep, error) {
	steps := make([]CreatorStep, 0, len(arrayNode))
	for _, item := range arrayNode {
		node, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("step element must be an object")
		}
		step, err := ParseStep(node)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func ParseStep(node map[string]any) (CreatorStep, error) {
	id, err := RequireString(node, "id")
	if err != nil {
		return CreatorStep{}, err
	}

	skill, err := RequireString(node, "skill")
	if err != nil {
		return CreatorStep{}, err
	}

	task, err := RequireString(node, "task")
	if err != nil {
		return CreatorStep{}, err
	}

	if !idRegex.MatchString(id) {
		return CreatorStep{}, fmt.Errorf("invalid step id '%s'", id)
	}

	if len(task) > 400 {
		return CreatorStep{}, fmt.Errorf("step.task max length is 400")
	}

	if err := EnforceYamlSafe(skill, "skill"); err != nil {
		return CreatorStep{}, err
	}
	if err := EnforceYamlSafe(task, "task"); err != nil {
		return CreatorStep{}, err
	}

	withKeys := make(map[string]string)
	if withNode, ok := node["with_keys"].(map[string]any); ok {
		for k, v := range withNode {
			if strVal, isStr := v.(string); isStr {
				if err := EnforceYamlSafe(strVal, "with_keys value"); err != nil {
					return CreatorStep{}, err
				}
				withKeys[k] = strVal
			}
		}
	}

	return CreatorStep{
		ID:       id,
		Skill:    skill,
		Task:     task,
		WithKeys: withKeys,
	}, nil
}

func RequireString(node map[string]any, propertyName string) (string, error) {
	val, ok := node[propertyName]
	if !ok {
		return "", fmt.Errorf("'%s' is required", propertyName)
	}

	strVal, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be string", propertyName)
	}

	if strings.TrimSpace(strVal) == "" {
		return "", fmt.Errorf("'%s' must not be empty", propertyName)
	}

	return strVal, nil
}

func RequireArray(node map[string]any, propertyName string) ([]any, error) {
	val, ok := node[propertyName]
	if !ok {
		return nil, fmt.Errorf("'%s' is required", propertyName)
	}

	arr, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("'%s' must be array", propertyName)
	}

	return arr, nil
}

func RequireObject(node map[string]any, propertyName string) (map[string]any, error) {
	val, ok := node[propertyName]
	if !ok {
		return nil, fmt.Errorf("'%s' is required", propertyName)
	}

	obj, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("'%s' must be object", propertyName)
	}

	return obj, nil
}

func EnforceYamlSafe(value, fieldName string) error {
	if strings.ContainsAny(value, "\"\n\r\\") {
		return fmt.Errorf("%s may not contain double quotes, newlines, or backslashes", fieldName)
	}
	return nil
}

func ToJsonString(value string) string {
	bytes, _ := json.Marshal(value)
	return string(bytes)
}

func AppendHeader(sb *strings.Builder, common CommonSlots) {
	sb.WriteString("---\n")
	fmt.Fprintf(sb, "name: %s\n", ToJsonString(common.Name))
	fmt.Fprintf(sb, "description: %s\n", ToJsonString(common.Description))
	sb.WriteString("kind: meta\n")
	fmt.Fprintf(sb, "meta_priority: %d\n", common.MetaPriority)
	sb.WriteString("always: false\n")
	sb.WriteString("triggers:\n")
	for _, trigger := range common.Triggers {
		sb.WriteString(fmt.Sprintf("  - %s\n", ToJsonString(trigger)))
	}
	sb.WriteString("provenance:\n")
	sb.WriteString("  origin: opensquilla-user\n")
	sb.WriteString("  license: Apache-2.0\n")
	sb.WriteString("metadata:\n")
	sb.WriteString("  opensquilla:\n")
	sb.WriteString("    risk: \"low\"\n")
	sb.WriteString("    read_only: true\n")
	sb.WriteString("    no_write: true\n")
	sb.WriteString("    write_access: false\n")
	sb.WriteString("    network_access: false\n")
	sb.WriteString("    creator_gates:\n")
	sb.WriteString("      - \"G1 structural lint\"\n")
	sb.WriteString("      - \"G2 scheduler dry-run\"\n")
	sb.WriteString("      - \"G3 positive trigger smoke\"\n")
	sb.WriteString("      - \"G4 unrelated negative smoke\"\n")
	sb.WriteString("      - \"acceptance_compare versus highest-tier no-meta baseline\"\n")
	sb.WriteString("      - \"runtime_e2e versus highest-tier no-meta baseline\"\n")
}

func AppendP1Body(sb *strings.Builder, name, description string, steps []CreatorStep) {
	sb.WriteString("---\n\n")
	fmt.Fprintf(sb, "# %s (Meta-Skill, P1 sequential)\n\n", name)
	sb.WriteString(description)
	sb.WriteString("\n\n")
	sb.WriteString("## Execution Flow\n")

	for i, step := range steps {
		sb.WriteString("\n")
		fmt.Fprintf(sb, "%d. `%s` runs `%s`.\n", i+1, step.ID, step.Skill)
		fmt.Fprintf(sb, "   Purpose: %s\n", step.Task)
	}

	sb.WriteString("\n## Expected Output\n\n")
	sb.WriteString("Return a grounded final deliverable from the ordered step outputs. Cite the\n")
	sb.WriteString("specific upstream evidence used, keep claims within the user request and\n")
	sb.WriteString("available step outputs, and do not invent missing facts.\n\n")
	sb.WriteString("## Safety\n\n")
	sb.WriteString("This proposal is read-only. It does not request file writes, network access, or\n")
	sb.WriteString("destructive operations; every generated step carries explicit `read_only`,\n")
	sb.WriteString("`write_access: false`, and `network_access: false` annotations for gate review.\n\n")
	sb.WriteString("## Creator Gates\n\n")
	sb.WriteString("Generated proposals must pass structural lint, scheduler dry-run, positive and\n")
	sb.WriteString("negative trigger smoke tests, highest-tier no-meta acceptance comparison, and\n")
	sb.WriteString("runtime E2E comparison before auto-enable eligibility.\n\n")
	sb.WriteString("## Fallback\n\n")
	sb.WriteString("LLM should invoke the listed skills in order.\n")
}

func AppendP2Body(sb *strings.Builder, name, description string, branches []CreatorStep, merge CreatorStep, tail *CreatorStep) {
	sb.WriteString("---\n\n")
	fmt.Fprintf(sb, "# %s (Meta-Skill, P2 fan_out_merge)\n\n", name)
	sb.WriteString(description)
	sb.WriteString("\n\n")
	sb.WriteString("## Execution Flow\n")

	for _, branch := range branches {
		sb.WriteString("\n")
		fmt.Fprintf(sb, "- Branch `%s` runs `%s`.\n", branch.ID, branch.Skill)
		fmt.Fprintf(sb, "   Purpose: %s\n", branch.Task)
	}

	sb.WriteString("\n")
	fmt.Fprintf(sb, "- Merge `%s` runs `%s`.\n", merge.ID, merge.Skill)
	fmt.Fprintf(sb, "   Purpose: %s\n", merge.Task)

	if tail != nil {
		sb.WriteString("\n")
		fmt.Fprintf(sb, "- Tail `%s` runs `%s`.\n", tail.ID, tail.Skill)
		fmt.Fprintf(sb, "   Purpose: %s\n", tail.Task)
	}

	sb.WriteString("\n## Expected Output\n\n")
	sb.WriteString("Return a grounded final deliverable from the branch and merge outputs. Cite the\n")
	sb.WriteString("specific upstream evidence used, keep claims within the user request and\n")
	sb.WriteString("available step outputs, and do not invent missing facts.\n\n")
	sb.WriteString("## Safety\n\n")
	sb.WriteString("This proposal is read-only. It does not request file writes, network access, or\n")
	sb.WriteString("destructive operations; every generated step carries explicit `read_only`,\n")
	sb.WriteString("`write_access: false`, and `network_access: false` annotations for gate review.\n\n")
	sb.WriteString("## Creator Gates\n\n")
	sb.WriteString("Generated proposals must pass structural lint, scheduler dry-run, positive and\n")
	sb.WriteString("negative trigger smoke tests, highest-tier no-meta acceptance comparison, and\n")
	sb.WriteString("runtime E2E comparison before auto-enable eligibility.\n\n")
	sb.WriteString("## Fallback\n\n")
	sb.WriteString("LLM should invoke the branch skills (in parallel where possible), then call the merge skill to aggregate, then optionally call the tail skill.\n")
}

func AppendP3Body(sb *strings.Builder, name, description string, steps []CreatorStep) {
	sb.WriteString("---\n\n")
	fmt.Fprintf(sb, "# %s (Meta-Skill, P3 condition_gated)\n\n", name)
	sb.WriteString(description)
	sb.WriteString("\n\n")
	sb.WriteString("## Execution Flow\n")

	for i, step := range steps {
		sb.WriteString("\n")
		fmt.Fprintf(sb, "%d. `%s` runs `%s`.\n", i+1, step.ID, step.Skill)
		fmt.Fprintf(sb, "   Purpose: %s\n", step.Task)
	}

	sb.WriteString("\n## Expected Output\n\n")
	sb.WriteString("Return a decision-ready final artifact with explicit assumptions, missing-data\n")
	sb.WriteString("limits, and next actions. Keep claims grounded in the step outputs.\n\n")
	sb.WriteString("## Safety\n\n")
	sb.WriteString("This proposal is read-only and keeps all generated step inputs escaped or\n")
	sb.WriteString("bounded for gate review.\n\n")
	sb.WriteString("## Creator Gates\n\n")
	sb.WriteString("Generated proposals must pass structural lint, scheduler dry-run, trigger\n")
	sb.WriteString("smoke tests, collision/risk checks, acceptance comparison, and runtime E2E\n")
	sb.WriteString("comparison before auto-enable eligibility.\n")
}
