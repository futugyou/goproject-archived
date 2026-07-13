package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
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
		return "", err
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
		return "", fmt.Errorf("Error: skill '%s' not found. Available: %s.", requested, available)
	}

	if match.DisableModelInvocation {
		return "", fmt.Errorf("Error: skill '%s' is not available for model invocation.", match.Name)
	}

	var builder SkillPromptBuilder
	body := builder.BuildSkillBody(match)
	if len(body) == 0 {
		return "", fmt.Errorf("Skill '%s' has no instructions body.", match.Name)
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

	var rawMap map[string]any
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

func (s SkillPromptBuilder) EstimateCharacterCost(skills []SkillDefinition) int {
	modelSkills := make([]SkillDefinition, 0)
	for _, skill := range skills {
		if !skill.DisableModelInvocation {
			modelSkills = append(modelSkills, skill)
		}
	}

	if len(modelSkills) == 0 {
		return 0
	}

	// Base overhead (XML wrapper + header text)
	var cost = 195
	for _, skill := range modelSkills {
		// Per-skill overhead (XML tags + indentation) + actual content
		cost += 97 + len(html.EscapeString(skill.Name)) + len(html.EscapeString(skill.Description)) + len(html.EscapeString(skill.Location)) + len(skill.Instructions)
	}

	return cost
}

func (s SkillPromptBuilder) EstimateIndexCharacterCost(skills []SkillDefinition, template string) int {
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
		return 0
	}

	effectiveTemplate := template
	if template == "" {
		effectiveTemplate = DefaultIndexTemplate
	}

	var templateOverhead = len(effectiveTemplate) - len(SkillsPlaceholder) - len("{load_instruction}") + len(LoadInstructionFragment) + 2 // BuildIndex prepends and appends "\n"

	if hasResources {
		templateOverhead += len(ResourceInstructionFragment)
	}

	if strings.Contains(effectiveTemplate, "{resource_instruction}") {
		templateOverhead -= len("{resource_instruction}")
	}
	var cost = templateOverhead

	for _, skill := range modelSkills {
		// Per-skill XML overhead (<skill>, <name>, <description>, <location>, closing tags + newlines)
		cost += 97 + len(html.EscapeString(skill.Name)) + len(html.EscapeString(skill.Description)) + len(html.EscapeString(skill.Location))

		if len(skill.Resources) > 0 {
			// <resources> + </resources> wrapper (incl. indentation + newlines)
			cost += 30
			for _, resource := range skill.Resources {
				// <resource kind="..." path="..." />
				cost += 33 + len(html.EscapeString(resource.RelativePath))
				if resource.Kind == SkillResourceKind_Reference {
					cost += len("reference")
				} else {
					cost += len("script")
				}
			}
		}
	}

	return cost
}

func (s SkillPromptBuilder) EstimateSkillEagerCost(skill *SkillDefinition) int {
	if skill == nil || skill.DisableModelInvocation {
		return 0
	}

	return 97 + len(html.EscapeString(skill.Name)) + len(html.EscapeString(skill.Description)) + len(html.EscapeString(skill.Location)) + len(skill.Instructions)
}

func (s SkillPromptBuilder) EstimateSkillIndexCost(skill *SkillDefinition) int {
	if skill == nil || skill.DisableModelInvocation {
		return 0
	}

	var cost = 97 + len(html.EscapeString(skill.Name)) + len(html.EscapeString(skill.Description)) + len(html.EscapeString(skill.Location))

	if len(skill.Resources) > 0 {
		cost += 30 // <resources>...</resources> wrapper
		for _, resource := range skill.Resources {
			cost += 33 + len(html.EscapeString(resource.RelativePath))
			if resource.Kind == SkillResourceKind_Reference {
				cost += len("reference")
			} else {
				cost += len("script")
			}
		}
	}

	return cost
}

var _ ITool = (*MetaInvokeTool)(nil)

type MetaInvokeTool struct {
	provider func() []SkillDefinition
}

func NewMetaInvokeTool(provider func() []SkillDefinition) *MetaInvokeTool {
	return &MetaInvokeTool{provider: provider}
}

// Description implements [ITool].
func (m *MetaInvokeTool) Description() string {
	return "Invoke a meta skill by name and return a structured execution intent payload. Use when a user intent matches a kind=meta skill."
}

// ParameterSchema implements [ITool].
func (m *MetaInvokeTool) ParameterSchema() string {
	return `{"type":"object","properties":{"skill":{"type":"string","description":"Meta skill name to invoke."},"input":{"type":"string","description":"Optional user input passed to the meta execution pipeline."}},"required":["skill"]}`
}

// Name implements [ITool].
func (m *MetaInvokeTool) Name() string {
	return "meta_invoke"
}

// Execute implements [ITool].
func (m *MetaInvokeTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	result, skillName, input, errorstr := m.tryParseArguments(argumentsJson)
	if !result {
		return errorstr, errors.New(errorstr)
	}

	skills := []SkillDefinition{}
	if m.provider != nil {
		skills = m.provider()
	}
	var matched *SkillDefinition
	for _, skill := range skills {
		if skill.Kind == SkillKind_Meta && skill.Name == skillName && !skill.DisableModelInvocation {
			matched = &skill
			break
		}
	}

	if matched == nil {
		msgs := []string{}
		for _, skill := range skills {
			if skill.Kind == SkillKind_Meta && !skill.DisableModelInvocation {
				msgs = append(msgs, skill.Name)
			}
		}

		available := strings.Join(msgs, ", ")
		errorstr = fmt.Sprintf("Error: meta skill '%s' not found. Available: (none)).", skillName)
		if len(msgs) > 0 {
			errorstr = fmt.Sprintf("Error: meta skill '%s' not found. Available: %s).", skillName, available)
		}
		return "", errors.New(errorstr)
	}

	var payload = MetaInvokeIntent{
		Skill:         matched.Name,
		Input:         &input,
		FinalTextMode: &matched.FinalTextMode,
		MetaPriority:  &matched.MetaPriority,
	}
	steps := []MetaInvokeStepSummary{}
	if matched.Composition != nil {
		for _, v := range matched.Composition.Steps {
			steps = append(steps, MetaInvokeStepSummary{
				Id:        v.ID,
				Kind:      v.Kind,
				DependsOn: v.DependsOn,
			})
		}
	}

	payload.Steps = steps

	data, err := json.Marshal(payload)
	if err != nil {
		return err.Error(), err
	}
	return string(data), nil
}

func (m *MetaInvokeTool) tryParseArguments(jsonStr string) (result bool, skill string, input string, errorstr string) {
	if strings.TrimSpace(jsonStr) == "" {
		return false, "", "", "Error: missing required argument 'skill'."
	}

	var data map[string]any
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return false, "", "", "Error: invalid JSON arguments. Expected {\"skill\":\"<name>\"}."
	}

	if skillVal, exists := data["skill"]; exists {
		if str, ok := skillVal.(string); ok {
			skill = str
		}
	}

	if inputVal, exists := data["input"]; exists {
		if str, ok := inputVal.(string); ok {
			input = str
		}
	}

	if strings.TrimSpace(skill) == "" {
		return false, "", "", "Error: missing required argument 'skill'."
	}

	return true, skill, input, ""
}

type MetaInvokeIntent struct {
	Skill         string                  `json:"skill"`
	Input         *string                 `json:"input,omitempty"`
	FinalTextMode *string                 `json:"final_text_mode,omitempty"`
	MetaPriority  *int                    `json:"meta_priority,omitempty"`
	Steps         []MetaInvokeStepSummary `json:"steps"`
}

type MetaInvokeStepSummary struct {
	Id        string   `json:"id"`
	Kind      string   `json:"kind"`
	DependsOn []string `json:"depends_on"`
}

type MetaSkillResolver struct{}

func (m *MetaSkillResolver) isTriggerMatch(userMessage, trigger string) bool {
	normalizedTrigger := strings.TrimSpace(trigger)
	if len(normalizedTrigger) == 0 {
		return false
	}

	return strings.Contains(userMessage, normalizedTrigger)
}

func (m *MetaSkillResolver) TryResolve(skills []SkillDefinition, userMessage string) (matched *SkillDefinition, result bool) {
	if len(skills) == 0 || len(userMessage) == 0 {
		return
	}

	var message = strings.TrimSpace(userMessage)

	var bestSkill *SkillDefinition
	bestPriority := math.MinInt
	bestTriggerLength := -1
	for _, skill := range skills {
		if skill.Kind != SkillKind_Meta || skill.DisableModelInvocation {
			continue
		}

		if len(skill.Triggers) == 0 {
			continue
		}

		for _, trigger := range skill.Triggers {
			if len(trigger) == 0 {
				continue
			}
			if !m.isTriggerMatch(message, trigger) {
				continue
			}

			var priority = skill.MetaPriority
			var triggerLength = len(trigger)

			if priority > bestPriority || (priority == bestPriority && triggerLength > bestTriggerLength) {
				bestSkill = &skill
				bestPriority = priority
				bestTriggerLength = triggerLength
			}
		}
	}

	matched = bestSkill
	return matched, matched != nil
}

type SkillInspector struct{}

func (s *SkillInspector) TryLocateSkillRoot(candidatePath string) (string, error) {
	if !DirectoryExists(candidatePath) {
		return "", fmt.Errorf("Skill path not found: %s", candidatePath)
	}

	if FileExists(filepath.Join(candidatePath, "SKILL.md")) {
		return filepath.Abs(candidatePath)
	}

	matches, err := FindDirectoriesCantainsFileName(candidatePath, "SKILL.md")
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("No SKILL.md file was found under %s", candidatePath)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("Multiple SKILL.md files were found under %s. Point the command at a single skill directory.", candidatePath)
	}

	return filepath.Abs(matches[0])
}

func (s *SkillInspector) InspectPath(candidatePath string, source SkillSource) *SkillInspectionResult {
	skillRootPath, err := s.TryLocateSkillRoot(candidatePath)
	if err != nil {
		return FailureSkillInspectionResult(err.Error())
	}

	var skillFilePath = filepath.Join(skillRootPath, "SKILL.md")
	loader := &SkillLoader{}
	definition, err := loader.ParseSkillFile(skillFilePath, skillRootPath, source)
	if err != nil || definition == nil {
		return FailureSkillInspectionResult("failed to parse skill frontmatter at  " + skillFilePath)
	}

	return &SkillInspectionResult{
		Success:       true,
		SkillRootPath: skillRootPath,
		SkillFilePath: skillFilePath,
		Definition:    definition,
	}
}

var _ ITool = (*ReadSkillResourceTool)(nil)

type ReadSkillResourceTool struct {
	provider         func() []SkillDefinition
	maxResourceBytes int64
}

func NewReadSkillResourceTool(provider func() []SkillDefinition, maxResourceBytes int64) *ReadSkillResourceTool {
	if provider == nil {
		provider = func() []SkillDefinition {
			return []SkillDefinition{}
		}
	}

	if maxResourceBytes == -1 {
		maxResourceBytes = 256 * 1024
	}

	return &ReadSkillResourceTool{
		provider:         provider,
		maxResourceBytes: maxResourceBytes,
	}
}

func NewReadSkillResourceToolWithSkills(skills []SkillDefinition, maxResourceBytes int64) *ReadSkillResourceTool {
	provider := func() []SkillDefinition {
		return skills
	}

	if maxResourceBytes == -1 {
		maxResourceBytes = 256 * 1024
	}

	return &ReadSkillResourceTool{
		provider:         provider,
		maxResourceBytes: maxResourceBytes,
	}
}

// Description implements [ITool].
func (r *ReadSkillResourceTool) Description() string {
	return "Read the contents of a single auxiliary resource (reference document or script) " +
		"associated with a skill. Resource names are listed in the <skill-resources> manifest " +
		"either inside the index or alongside a loaded skill body. " +
		"Never call this tool with 'SKILL.md' — that is the skill body itself, fetch it via `load_skill`. " +
		"Cross-skill paths (e.g. '../other-skill/...') and absolute paths are not accepted."
}

// Name implements [ITool].
func (r *ReadSkillResourceTool) Name() string {
	return "read_skill_resource"
}

// ParameterSchema implements [ITool].
func (r *ReadSkillResourceTool) ParameterSchema() string {
	return `{"type":"object","properties":{"skill":{"type":"string","description":"Skill name (as listed in <available-skills>)"},"resource":{"type":"string","description":"Resource name — either bare file name (e.g. \"lookup.md\") or relative path (e.g. \"references/lookup.md\"). Must be listed in this skill's own <resources> manifest; do not pass \"SKILL.md\" (use load_skill) or paths containing \"..\""}},"required":["skill","resource"]}`
}

func (r *ReadSkillResourceTool) tryParseArguments(argumentsJson string) (result bool, skillName string, resourceName string, errorstr string) {
	if IsBlank(argumentsJson) {
		errorstr = "Error: missing required arguments 'skill' and 'resource'."
		return
	}

	var rootElement map[string]json.RawMessage
	if err := json.Unmarshal([]byte(argumentsJson), &rootElement); err != nil {
		errorstr = "Error: invalid JSON arguments. Expected an object like {\"skill\":\"<name>\",\"resource\":\"<name>\"}."
		return
	}

	var f bool
	keys := []string{"skill", "skill_name", "name"}
	for _, key := range keys {
		if f, skillName = r.tryReadString(rootElement, key); f {
			break
		}
	}
	keys = []string{"resource", "resource_name", "path"}
	for _, key := range keys {
		if f, resourceName = r.tryReadString(rootElement, key); f {
			break
		}
	}

	if IsBlank(skillName) {
		errorstr = "Error: missing required argument 'skill'."
		return
	}

	if IsBlank(resourceName) {
		errorstr = "Error: missing required argument 'resource'."
		return
	}

	result = true
	return
}

func (r *ReadSkillResourceTool) tryReadString(element map[string]json.RawMessage, property string) (bool, string) {
	routeRaw, exists := element[property]
	if !exists {
		return false, ""
	}

	var result string
	if err := json.Unmarshal(routeRaw, &result); err != nil {
		return false, ""
	}

	return true, result
}

func (r *ReadSkillResourceTool) findSkill(skills []SkillDefinition, requested string) *SkillDefinition {
	for _, skill := range skills {
		if skill.Name == requested {
			return &skill
		}
	}

	for _, skill := range skills {
		if skill.Metadata != nil && skill.Metadata.SkillKey == requested {
			return &skill
		}
	}

	return nil
}

func (r *ReadSkillResourceTool) findResource(skill *SkillDefinition, requested string) *SkillResource {
	var normalized = strings.TrimSpace(strings.ReplaceAll(requested, "\\", "/"))

	for _, resource := range skill.Resources {
		if resource.RelativePath == normalized {
			return &resource
		}
	}
	for _, resource := range skill.Resources {
		if resource.Name == normalized {
			return &resource
		}
	}
	for _, resource := range skill.Resources {
		if strings.HasSuffix(resource.RelativePath, "/"+normalized) {
			return &resource
		}
	}

	return nil
}

func (r *ReadSkillResourceTool) looksLikeSkillBody(requested string) bool {
	var normalized = strings.TrimSpace(strings.ReplaceAll(requested, "\\", "/"))
	if IsBlank(normalized) {
		return false
	}
	var lastSlash = strings.LastIndex(normalized, "/")
	var leaf = normalized[(lastSlash + 1):]
	if lastSlash >= 0 {
		leaf = normalized
	}
	return leaf == "SKILL.md"
}

func (r *ReadSkillResourceTool) tryExtractCrossSkillName(requested string, skills []SkillDefinition) string {
	var normalized = strings.TrimSpace(strings.ReplaceAll(requested, "\\", "/"))
	var lastSlash = strings.LastIndex(normalized, "/")
	if lastSlash <= 0 {
		return ""
	}

	var parent = normalized[:lastSlash]
	var prevSlash = strings.LastIndex(parent, "/")
	var lastSeg = parent
	if prevSlash >= 0 {
		lastSeg = parent[(prevSlash + 1):]
	}
	if IsBlank(lastSeg) || lastSeg == ".." {
		return ""
	}

	for _, s := range skills {

		if s.DisableModelInvocation {
			continue
		}
		if s.Name == lastSeg {
			return s.Name
		}
		if len(s.Metadata.SkillKey) > 0 && s.Metadata.SkillKey == lastSeg {
			return s.Name
		}
	}
	return ""
}

func (r *ReadSkillResourceTool) isPathWithinSkillRoot(resourceAbsolutePath string, skill *SkillDefinition) bool {
	if skill == nil || IsBlank(skill.Location) {
		return true
	}

	skillRoot, err := filepath.Abs(skill.Location)
	if err != nil {
		return false
	}
	resolved, err := filepath.Abs(resourceAbsolutePath)
	if err != nil {
		return false
	}
	rootWithSep := skillRoot + string(os.PathSeparator)
	if strings.HasSuffix(skillRoot, string(os.PathSeparator)) {
		rootWithSep = skillRoot
	}

	return strings.HasPrefix(resolved, rootWithSep)

}

func (r *ReadSkillResourceTool) resourcePathContainsReparsePoint(skillLocation, resourceAbsolutePath string) bool {
	if IsBlank(skillLocation) {
		return false
	}
	skillRoot, err := filepath.Abs(skillLocation)
	if err != nil {
		return true
	}
	resolved, err := filepath.Abs(resourceAbsolutePath)
	if err != nil {
		return true
	}
	relative, err := filepath.Rel(skillRoot, resolved)
	if err != nil {
		return true
	}

	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || strings.HasPrefix(relative, "../") || filepath.IsAbs(relative) {
		return true
	}

	var current = skillRoot

	for _, segment := range strings.FieldsFunc(filepath.ToSlash(relative), func(r rune) bool {
		return r == '/'
	}) {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return true
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}

	}

	return false
}

// Execute implements [ITool].
func (r *ReadSkillResourceTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	f, skillName, resourceName, errorstr := r.tryParseArguments(argumentsJson)
	if !f {
		return "", errors.New(errorstr)
	}
	skills := r.provider()
	skill := r.findSkill(skills, skillName)
	if skill == nil {
		msgs := []string{}
		for _, skill := range skills {
			if skill.Kind == SkillKind_Meta && !skill.DisableModelInvocation {
				msgs = append(msgs, skill.Name)
			}
		}

		available := strings.Join(msgs, ", ")
		errorstr = fmt.Sprintf("Error: meta skill '%s' not found. Available: (none)).", skillName)
		if len(msgs) > 0 {
			errorstr = fmt.Sprintf("Error: meta skill '%s' not found. Available: %s).", skillName, available)
		}
		return "", errors.New(errorstr)
	}
	if skill.DisableModelInvocation {
		errorstr = fmt.Sprintf("Error: skill '%s' is not available for model invocation.", skill.Name)
		return "", errors.New(errorstr)
	}

	resource := r.findResource(skill, resourceName)
	if resource == nil {
		if r.looksLikeSkillBody(resourceName) {
			crossSkill := r.tryExtractCrossSkillName(resourceName, skills)
			if !IsBlank(crossSkill) && crossSkill != skill.Name {
				errorstr = fmt.Sprintf("Error: 'SKILL.md' is the body of skill '%s', not an L3 resource of '%s'. Use `load_skill` with skill='%s' to fetch it, not `read_skill_resource`.)", crossSkill, skill.Name, crossSkill)
				return "", errors.New(errorstr)
			}
			errorstr = fmt.Sprintf("Error: 'SKILL.md' is the skill body itself, not an L3 resource. Use `load_skill` with skill='%s' to fetch it, not `read_skill_resource`.", skill.Name)
			return "", errors.New(errorstr)
		}

		if strings.Contains(resourceName, "..") || filepath.IsAbs(resourceName) {
			errorstr = fmt.Sprintf("Error: cross-skill or absolute paths are not allowed for `read_skill_resource`. It only accepts paths listed in '%s's own <resources> manifest. If you want another skill's body, call `load_skill` with that skill's name instead.", skill.Name)
			return "", errors.New(errorstr)
		}
		available := "(none)"
		if len(skill.Resources) > 0 {
			paths := []string{}
			for _, v := range skill.Resources {
				paths = append(paths, v.RelativePath)
			}
			available = strings.Join(paths, ", ")
		}
		errorstr = fmt.Sprintf("Error: resource '%s' not found in skill '%s'. Available: %s.", resourceName, skill.Name, available)
		return "", errors.New(errorstr)
	}

	if !r.isPathWithinSkillRoot(resource.AbsolutePath, skill) {
		errorstr = fmt.Sprintf("Error: resource '%s' resolves outside skill root and was rejected.", resource.RelativePath)
		return "", errors.New(errorstr)
	}

	info, err := os.Stat(resource.AbsolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Error: resource '%s' no longer exists on disk.", resource.RelativePath)
		}
		return "", err
	}

	if r.resourcePathContainsReparsePoint(skill.Location, resource.AbsolutePath) {
		return "", fmt.Errorf("Error: resource '%s' resolves through a symlink or reparse point and was rejected.", resource.RelativePath)
	}

	if info.Size() > r.maxResourceBytes {
		return "", fmt.Errorf("Error: resource '%s' is %d bytes (max %d). Read it via the workspace file tools instead.",
			resource.RelativePath, info.Size(), r.maxResourceBytes)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(resource.AbsolutePath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
