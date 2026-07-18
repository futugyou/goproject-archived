package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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

var _ ITool = (*ListToolsTool)(nil)

type ListToolsTool struct {
	provider func() []ToolDescriptor
}

func NewListToolsTool(provider func() []ToolDescriptor) *ListToolsTool {
	if provider == nil {
		provider = func() []ToolDescriptor { return nil }
	}
	return &ListToolsTool{
		provider: provider,
	}
}

// Description implements [ITool].
func (l *ListToolsTool) Description() string {
	return "List all available tools with their names, descriptions, and parameter schemas. " +
		"Use this to discover which tools are registered before calling them. " +
		"Optionally filter by a substring in the tool name."
}

// Name implements [ITool].
func (l *ListToolsTool) Name() string {
	return "list_tools"
}

// ParameterSchema implements [ITool].
func (l *ListToolsTool) ParameterSchema() string {
	return `{"type":"object","properties":{"filter":{"type":"string","description":"Optional substring filter for tool names"}}}`
}

// Execute implements [ITool].
func (l *ListToolsTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var filter = l.tryGetFilter(argumentsJson)
	var descriptors = l.provider()
	if !IsBlank(filter) {
		n := []ToolDescriptor{}
		for _, v := range descriptors {
			if strings.Contains(v.Name, filter) {
				n = append(n, v)
			}
		}
		descriptors = n

	}

	data, err := json.Marshal(descriptors)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (l *ListToolsTool) tryGetFilter(argumentsJson string) string {
	if IsBlank(argumentsJson) {
		return ""
	}

	var data struct {
		Filter string `json:"filter"`
	}

	if err := json.Unmarshal([]byte(argumentsJson), &data); err != nil {
		return ""
	}

	return data.Filter
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
		Input:         input,
		FinalTextMode: matched.FinalTextMode,
		MetaPriority:  &matched.MetaPriority,
	}
	steps := []MetaInvokeStepSummary{}
	if matched.Composition != nil {
		for _, v := range matched.Composition.Steps {
			steps = append(steps, MetaInvokeStepSummary{
				Id:        v.Id,
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
		if skill.CommandDispatch != "" {
			flags = append(flags, fmt.Sprintf("dispatch:%s", skill.CommandDispatch))
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

type MetaInvokeIntent struct {
	Skill         string                  `json:"skill"`
	Input         string                  `json:"input,omitempty"`
	FinalTextMode string                  `json:"final_text_mode,omitempty"`
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

type ProjectionScore[T any] struct {
	Item  T
	Score int
}

type ProjectionRouteAttempt struct {
	Resolution       *SkillProjectionResolution
	Score            int
	ProducerPriority int
	IsAmbiguous      bool
	AmbiguousReason  string
}

func SuccessAttempt(resolution *SkillProjectionResolution, score int, producerPriority int) *ProjectionRouteAttempt {
	return &ProjectionRouteAttempt{Resolution: resolution, Score: score, ProducerPriority: producerPriority, IsAmbiguous: false}
}

func BlockedAttempt(resolution *SkillProjectionResolution, score int) *ProjectionRouteAttempt {
	return &ProjectionRouteAttempt{Resolution: resolution, Score: score, ProducerPriority: 0, IsAmbiguous: false}
}

func AmbiguousAttempt(reason string) *ProjectionRouteAttempt {
	return &ProjectionRouteAttempt{Resolution: &SkillProjectionResolution{
		HasContracts: true,
		IsBlocked:    true,
		BlockReason:  reason,
	}, Score: -1 << 31, ProducerPriority: -1 << 31, IsAmbiguous: true, AmbiguousReason: reason}
}

func GetExplicitArtifactTerms(targetView string) []string {
	switch strings.ToLower(strings.TrimSpace(targetView)) {
	case "json-schema":
		return []string{"json schema", "schema file", "schema definition", "json schema 文件", "schema 文件", "schema 定义"}
	case "workflow-contract":
		return []string{"workflow contract", "工作流契约"}
	case "domain-model":
		return []string{"domain model", "领域模型"}
	case "prompt-constraint":
		return []string{"prompt policy", "prompt constraint", "提示词策略", "提示词约束"}
	default:
		return []string{}
	}
}

type SkillProjectionResolver struct{}

func (r SkillProjectionResolver) ResolveForRequest(skill *SkillDefinition, requestText string, logger *slog.Logger) *SkillProjectionResolution {
	if len(skill.ProjectionContracts) == 0 {
		return &SkillProjectionResolution{
			SkillName:    skill.Name,
			HasContracts: false,
		}
	}

	normalizedRequest := strings.ToLower(strings.TrimSpace(requestText))
	matchedAttempts := make([]ProjectionRouteAttempt, 0, len(skill.ProjectionContracts))
	var ambiguousAttempts []string

	for _, contract := range skill.ProjectionContracts {
		attempt, ok := r.tryResolveContract(skill.Name, &contract, normalizedRequest)
		if !ok {
			continue
		}

		if attempt.IsAmbiguous {
			if strings.TrimSpace(attempt.AmbiguousReason) != "" {
				ambiguousAttempts = append(ambiguousAttempts, attempt.AmbiguousReason)
			}
			continue
		}

		matchedAttempts = append(matchedAttempts, *attempt)
	}

	if len(matchedAttempts) == 0 {
		if len(ambiguousAttempts) > 0 {
			return r.block(skill.Name, ambiguousAttempts[0])
		}
		return r.block(skill.Name, "Projection topic selection did not produce a usable route for this request.")
	}

	sort.Slice(matchedAttempts, func(i, j int) bool {
		if matchedAttempts[i].Score != matchedAttempts[j].Score {
			return matchedAttempts[i].Score > matchedAttempts[j].Score
		}
		return matchedAttempts[i].ProducerPriority > matchedAttempts[j].ProducerPriority
	})

	if len(matchedAttempts) > 1 &&
		matchedAttempts[0].Score == matchedAttempts[1].Score &&
		matchedAttempts[0].ProducerPriority == matchedAttempts[1].ProducerPriority {
		return r.block(skill.Name, "Projection route selection is ambiguous across multiple producers for this request.")
	}

	return matchedAttempts[0].Resolution
}

func (r SkillProjectionResolver) BuildPromptPatch(resolution *SkillProjectionResolution) string {
	if resolution.Projection == nil ||
		strings.TrimSpace(resolution.SelectedTopic) == "" ||
		strings.TrimSpace(resolution.SelectedTargetView) == "" ||
		strings.TrimSpace(resolution.ProjectionFilePath) == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n[Projection Route]\n")
	sb.WriteString(fmt.Sprintf("Selected topic: %s\n", resolution.SelectedTopic))
	sb.WriteString(fmt.Sprintf("Selected target view: %s\n", resolution.SelectedTargetView))
	sb.WriteString(fmt.Sprintf("Projection source: %s\n", resolution.ProjectionFilePath))

	promptConstraint := r.buildPromptAssumptionConstraint(resolution.Projection.MappingPolicy.PromptAssumptionPolicy)
	if strings.TrimSpace(promptConstraint) != "" {
		sb.WriteString(fmt.Sprintf("Prompt constraint: %s\n", promptConstraint))
	}

	r.appendList(&sb, "Allowed terms", resolution.Projection.PromptProjection.AllowedTerms)
	r.appendList(&sb, "Forbidden assumptions", resolution.Projection.PromptProjection.ForbiddenAssumptions)
	r.appendList(&sb, "Required clarifications", resolution.Projection.PromptProjection.RequiredClarifications)
	r.appendList(&sb, "Reasoning paths", resolution.Projection.PromptProjection.ReasoningPaths)
	r.appendList(&sb, "Source digest", resolution.Projection.PromptProjection.SourceDigest)

	var formattedArtifacts []string
	for _, artifact := range resolution.Projection.DeliveryArtifacts {
		formattedArtifacts = append(formattedArtifacts, r.formatDeliveryArtifact(&artifact))
	}
	r.appendList(&sb, "Delivery artifacts", formattedArtifacts)

	if len(resolution.Projection.DroppedItems) > 0 {
		r.appendList(&sb, "Dropped items", resolution.Projection.DroppedItems)
	}

	return strings.TrimRight(sb.String(), "\r\n\t ")
}

func (r SkillProjectionResolver) block(skillName string, reason string) *SkillProjectionResolution {
	return &SkillProjectionResolution{
		SkillName:    skillName,
		HasContracts: true,
		IsBlocked:    true,
		BlockReason:  reason,
	}
}

func (r SkillProjectionResolver) tryResolveContract(skillName string, contract *SkillProjectionContractSet, requestText string) (*ProjectionRouteAttempt, bool) {
	index := contract.Index

	var resolvedTopic ProjectionScore[ProjectionTopicRecord]
	selectedTopic, foundTopic := r.SelectTopic(index, requestText)
	if !foundTopic {
		fallbackTopic, foundFallback := r.tryResolveNoSignalFallback(index)
		if !foundFallback {
			return AmbiguousAttempt("Projection topic selection is ambiguous for this request."), true
		}
		resolvedTopic = *fallbackTopic
	} else {
		resolvedTopic = selectedTopic
	}

	var resolvedView ProjectionScore[ProjectionViewRecord]
	selectedView, foundView := r.SelectView(index, &resolvedTopic.Item, requestText)
	if !foundView {
		if resolvedTopic.Score == 0 {
			fallbackView, foundFallbackView := r.tryResolveFallbackView(index, &resolvedTopic.Item)
			if foundFallbackView {
				resolvedView = *fallbackView
				goto VIEW_RESOLVED
			}
		}
		return AmbiguousAttempt(fmt.Sprintf("Projection target view selection is ambiguous for topic '%s'.", resolvedTopic.Item.DomainSlug)), true
	} else {
		resolvedView = selectedView
	}

VIEW_RESOLVED:
	totalScore := resolvedTopic.Score + resolvedView.Score
	if index.DefaultSelectionPolicy.PreferReadyOnly && !strings.EqualFold(resolvedView.Item.Status, "READY") {
		return BlockedAttempt(r.block(skillName, fmt.Sprintf("Projection '%s/%s' is not READY.", resolvedTopic.Item.DomainSlug, resolvedView.Item.TargetView)), totalScore), true
	}

	projectionPath, pathOk := r.tryResolveProjectionPath(contract.RootPath, resolvedView.Item.Path)
	if !pathOk {
		return BlockedAttempt(r.block(skillName, fmt.Sprintf("Projection file '%s' is outside the projection contract root.", resolvedView.Item.Path)), totalScore), true
	}

	if _, err := os.Stat(projectionPath); os.IsNotExist(err) {
		return BlockedAttempt(r.block(skillName, fmt.Sprintf("Projection file '%s' was not found.", resolvedView.Item.Path)), totalScore), true
	}

	projection, err := r.loadProjection(projectionPath)
	if err != nil {
		return BlockedAttempt(r.block(skillName, fmt.Sprintf("Projection file '%s' could not be parsed.", resolvedView.Item.Path)), totalScore), true
	}

	if projection == nil {
		return BlockedAttempt(r.block(skillName, fmt.Sprintf("Projection file '%s' is missing required route fields.", resolvedView.Item.Path)), totalScore), true
	}

	if index.DefaultSelectionPolicy.BlockOnOpenQuestions && len(projection.OpenQuestions) > 0 {
		return BlockedAttempt(r.block(skillName, fmt.Sprintf("Projection '%s/%s' has blocking open questions.", resolvedTopic.Item.DomainSlug, resolvedView.Item.TargetView)), totalScore), true
	}

	if strings.EqualFold(projection.MappingPolicy.UnresolvedItemPolicy, "block_or_escalate") && len(projection.OpenQuestions) > 0 {
		return BlockedAttempt(r.block(skillName, fmt.Sprintf("Projection '%s/%s' requires escalation before use.", resolvedTopic.Item.DomainSlug, resolvedView.Item.TargetView)), totalScore), true
	}

	return SuccessAttempt(&SkillProjectionResolution{
		SkillName:          skillName,
		HasContracts:       true,
		SelectedTopic:      resolvedTopic.Item.DomainSlug,
		SelectedTargetView: resolvedView.Item.TargetView,
		ProjectionFilePath: projectionPath,
		Projection:         projection,
	}, totalScore, contract.ProducerPriority), true
}

func (r SkillProjectionResolver) SelectTopic(index *ProjectionContractIndex, requestText string) (ProjectionScore[ProjectionTopicRecord], bool) {
	if len(index.Topics) == 0 {
		return ProjectionScore[ProjectionTopicRecord]{}, false
	}

	dimensionScores := r.toScoreMap(index.TopicScoring.ScoreDimensions)
	explicitArtifactBonus := r.getDimensionScore(dimensionScores, "explicit_artifact_bonus", 4)
	primaryIntentMatch := r.getDimensionScore(dimensionScores, "primary_intent_match", 5)
	strongKeywordMatch := r.getDimensionScore(dimensionScores, "strong_keyword_match", 3)
	supportingKeywordMatch := r.getDimensionScore(dimensionScores, "supporting_keyword_match", 1)
	crossTopicPenalty := r.getDimensionScore(dimensionScores, "cross_topic_conflict_penalty", -2)

	threshold := 2
	if index.TopicScoring != nil && index.TopicScoring.ClarifyWhenScoreGapBelow != 0 {
		threshold = index.TopicScoring.ClarifyWhenScoreGapBelow
	}

	topicSignals := make(map[string]ProjectionTopicSignals)
	if index.TopicScoring != nil {
		for i := range index.TopicScoring.Topics {
			t := index.TopicScoring.Topics[i]
			if key := strings.ToLower(t.DomainSlug); key != "" {
				if _, ok := topicSignals[key]; !ok {
					topicSignals[key] = t
				}
			}
		}
	}

	scored := make([]ProjectionScore[ProjectionTopicRecord], 0, len(index.Topics))
	for _, topic := range index.Topics {
		signals := topicSignals[strings.ToLower(topic.DomainSlug)]

		score := r.countMatches(requestText, signals.PrimaryIntentSignals) * strongKeywordMatch
		score += r.countMatches(requestText, signals.SupportingSignals) * supportingKeywordMatch
		if r.hasAnyMatch(requestText, signals.ExplicitArtifactSignals) {
			score += explicitArtifactBonus
		}
		if r.hasAnyMatch(requestText, signals.PrimaryIntentSignals) {
			score += primaryIntentMatch
		}
		if r.hasAnyMatch(requestText, signals.DemoteWhenCompetingTopicSignals) {
			score += crossTopicPenalty
		}
		scored = append(scored, ProjectionScore[ProjectionTopicRecord]{Item: topic, Score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) == 0 || scored[0].Score <= 0 {
		return ProjectionScore[ProjectionTopicRecord]{}, false
	}

	if len(scored) > 1 && (scored[0].Score-scored[1].Score) < threshold {
		return ProjectionScore[ProjectionTopicRecord]{}, false
	}

	return scored[0], true
}

func (r SkillProjectionResolver) SelectView(index *ProjectionContractIndex, topic *ProjectionTopicRecord, requestText string) (ProjectionScore[ProjectionViewRecord], bool) {
	var candidates []ProjectionViewRecord
	for _, view := range topic.Views {
		if index.DefaultSelectionPolicy.PreferReadyOnly {
			if strings.EqualFold(view.Status, "READY") {
				candidates = append(candidates, view)
			}
		} else {
			candidates = append(candidates, view)
		}
	}

	if len(candidates) == 0 {
		return ProjectionScore[ProjectionViewRecord]{}, false
	}

	var scoreDimensions []ProjectionScoreDimension
	var viewScoringViews []ProjectionViewSignals
	var withinTopicOverrides []ProjectionTopicViewOverride
	threshold := 2
	preferExplicit := false

	if index.TargetViewScoring != nil {
		scoreDimensions = index.TargetViewScoring.ScoreDimensions
		viewScoringViews = index.TargetViewScoring.Views
		withinTopicOverrides = index.TargetViewScoring.WithinTopicOverrides
		if index.TargetViewScoring.ClarifyWhenScoreGapBelow != 0 {
			threshold = index.TargetViewScoring.ClarifyWhenScoreGapBelow
		}
		preferExplicit = index.TargetViewScoring.PreferExplicitUserArtifactRequests
	}

	dimensionScores := r.toScoreMap(scoreDimensions)
	explicitOutputMatch := r.getDimensionScore(dimensionScores, "explicit_output_match", 5)
	strongSignalMatch := r.getDimensionScore(dimensionScores, "strong_signal_match", 3)
	supportingSignalMatch := r.getDimensionScore(dimensionScores, "supporting_signal_match", 1)
	crossViewPenalty := r.getDimensionScore(dimensionScores, "cross_view_conflict_penalty", -2)
	defaultViewBonus := r.getDimensionScore(dimensionScores, "topic_default_view_bonus", 1)
	explicitArtifactRequestBonus := r.getDimensionScore(dimensionScores, "explicit_user_artifact_request_bonus", 4)

	viewSignals := make(map[string]ProjectionViewSignals)
	for i := range viewScoringViews {
		v := viewScoringViews[i]
		if key := strings.ToLower(v.TargetView); key != "" {
			if _, ok := viewSignals[key]; !ok {
				viewSignals[key] = v
			}
		}
	}

	var topicOverride *ProjectionTopicViewOverride
	for i := range withinTopicOverrides {
		o := withinTopicOverrides[i]
		if strings.EqualFold(o.DomainSlug, topic.DomainSlug) {
			topicOverride = &o
			break
		}
	}

	scored := make([]ProjectionScore[ProjectionViewRecord], 0, len(candidates))
	for _, view := range candidates {
		signals := viewSignals[strings.ToLower(view.TargetView)]
		score := 0
		if r.hasAnyMatch(requestText, signals.ExplicitOutputSignals) {
			score += explicitOutputMatch
		}
		score += r.countMatches(requestText, signals.StrongSignals) * strongSignalMatch
		score += r.countMatches(requestText, signals.SupportingSignals) * supportingSignalMatch
		if r.hasAnyMatch(requestText, signals.DemoteWhenCompetingViewSignals) {
			score += crossViewPenalty
		}

		if preferExplicit && r.hasExplicitArtifactRequestForView(requestText, view.TargetView) {
			score += explicitArtifactRequestBonus
		}

		if strings.EqualFold(view.TargetView, topic.DefaultTargetView) {
			score += defaultViewBonus
		}

		if topicOverride != nil {
			for _, bonus := range topicOverride.Bonuses {
				if strings.EqualFold(bonus.TargetView, view.TargetView) {
					if r.hasAnyMatch(requestText, bonus.WhenRequestSignals) {
						score += bonus.Score
					}
				}
			}
		}

		scored = append(scored, ProjectionScore[ProjectionViewRecord]{Item: view, Score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) == 0 || scored[0].Score <= 0 {
		return ProjectionScore[ProjectionViewRecord]{}, false
	}

	if len(scored) > 1 && (scored[0].Score-scored[1].Score) < threshold {
		return ProjectionScore[ProjectionViewRecord]{}, false
	}

	return scored[0], true
}

func (r SkillProjectionResolver) tryResolveNoSignalFallback(index *ProjectionContractIndex) (*ProjectionScore[ProjectionTopicRecord], bool) {
	for _, targetView := range index.DefaultSelectionPolicy.FallbackOrderByTargetView {
		if strings.TrimSpace(targetView) == "" {
			continue
		}

		for _, topic := range index.Topics {
			for _, view := range topic.Views {
				if strings.EqualFold(view.TargetView, targetView) {
					return &ProjectionScore[ProjectionTopicRecord]{Item: topic, Score: 0}, true
				}
			}
		}
	}
	return &ProjectionScore[ProjectionTopicRecord]{}, false
}

func (r SkillProjectionResolver) tryResolveFallbackView(index *ProjectionContractIndex, topic *ProjectionTopicRecord) (*ProjectionScore[ProjectionViewRecord], bool) {
	var candidates []ProjectionViewRecord
	for _, view := range topic.Views {
		if index.DefaultSelectionPolicy.PreferReadyOnly {
			if strings.EqualFold(view.Status, "READY") {
				candidates = append(candidates, view)
			}
		} else {
			candidates = append(candidates, view)
		}
	}

	for _, targetView := range index.DefaultSelectionPolicy.FallbackOrderByTargetView {
		if strings.TrimSpace(targetView) == "" {
			continue
		}

		for _, view := range candidates {
			if strings.EqualFold(view.TargetView, targetView) {
				return &ProjectionScore[ProjectionViewRecord]{Item: view, Score: 0}, true
			}
		}
	}
	return &ProjectionScore[ProjectionViewRecord]{}, false
}

func (r SkillProjectionResolver) loadProjection(projectionPath string) (*ProjectionDocument, error) {
	data, err := os.ReadFile(projectionPath)
	if err != nil {
		return nil, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	doc := &ProjectionDocument{}

	if policyData, ok := raw["mapping_policy"]; ok {
		var p map[string]string
		if err := json.Unmarshal(policyData, &p); err == nil {
			doc.MappingPolicy.UnresolvedItemPolicy = p["unresolved_item_policy"]
			doc.MappingPolicy.PromptAssumptionPolicy = p["prompt_assumption_policy"]
		}
	}

	if payloadData, ok := raw["prompt_projection"]; ok {
		var payload struct {
			AllowedTerms           []string `json:"allowed_terms"`
			ForbiddenAssumptions   []string `json:"forbidden_assumptions"`
			RequiredClarifications []string `json:"required_clarifications"`
			ReasoningPaths         []string `json:"reasoning_paths"`
			SourceDigest           []string `json:"source_digest"`
		}
		if err := json.Unmarshal(payloadData, &payload); err == nil {
			doc.PromptProjection.AllowedTerms = payload.AllowedTerms
			doc.PromptProjection.ForbiddenAssumptions = payload.ForbiddenAssumptions
			doc.PromptProjection.RequiredClarifications = payload.RequiredClarifications
			doc.PromptProjection.ReasoningPaths = payload.ReasoningPaths
			doc.PromptProjection.SourceDigest = payload.SourceDigest
		}
	}

	if artifactsData, ok := raw["delivery_artifacts"]; ok {
		var list []map[string]string
		if err := json.Unmarshal(artifactsData, &list); err == nil {
			for _, item := range list {
				name := item["artifact_name"]
				aType := item["artifact_type"]
				path := item["path"]
				if strings.TrimSpace(name) == "" || strings.TrimSpace(aType) == "" || strings.TrimSpace(path) == "" {
					continue
				}
				doc.DeliveryArtifacts = append(doc.DeliveryArtifacts, ProjectionDeliveryArtifact{
					ArtifactName: name,
					ArtifactType: aType,
					Path:         path,
					Status:       item["status"],
				})
			}
		}
	}

	doc.DroppedItems = r.readDisplayArray(raw["dropped_items"])
	doc.OpenQuestions = r.readDisplayArray(raw["open_questions"])

	return doc, nil
}

func (r SkillProjectionResolver) appendList(sb *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	sb.WriteString("\n")
	sb.WriteString(label)
	sb.WriteString(":\n")
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("- %s\n", item))
	}
}

func (r SkillProjectionResolver) formatDeliveryArtifact(artifact *ProjectionDeliveryArtifact) string {
	statusSuffix := ""
	if strings.TrimSpace(artifact.Status) != "" {
		statusSuffix = fmt.Sprintf(" [%s]", artifact.Status)
	}
	return fmt.Sprintf("%s (%s) -> %s%s", artifact.ArtifactName, artifact.ArtifactType, artifact.Path, statusSuffix)
}

func (r SkillProjectionResolver) buildPromptAssumptionConstraint(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "disallow_unmapped_terms":
		return "Do not use unmapped terms or invent terminology beyond this projection."
	case "warn_on_unmapped_terms":
		return "If you use terms not mapped by this projection, explicitly warn that they are unmapped assumptions."
	case "allow_unmapped_terms":
		return "Unmapped terms are allowed, but prefer mapped terminology when available."
	case "":
		return ""
	default:
		return fmt.Sprintf("Follow prompt assumption policy '%s' when introducing terms not mapped by this projection.", policy)
	}
}

func (r SkillProjectionResolver) toScoreMap(dimensions []ProjectionScoreDimension) map[string]int {
	m := make(map[string]int)
	for _, dim := range dimensions {
		key := strings.ToLower(dim.Dimension)
		if strings.TrimSpace(key) != "" {
			if _, exists := m[key]; !exists {
				m[key] = dim.Score
			}
		}
	}
	return m
}

func (r SkillProjectionResolver) tryResolveProjectionPath(rootPath, relativePath string) (string, bool) {
	if strings.TrimSpace(rootPath) == "" || strings.TrimSpace(relativePath) == "" {
		return "", false
	}

	normalizedRelativePath := filepath.FromSlash(relativePath)
	if filepath.IsAbs(normalizedRelativePath) {
		return "", false
	}

	rootFullPath, err := filepath.Abs(rootPath)
	if err != nil {
		return "", false
	}

	candidateFullPath, err := filepath.Abs(filepath.Join(rootFullPath, normalizedRelativePath))
	if err != nil {
		return "", false
	}

	sep := string(filepath.Separator)
	rootWithSeparator := rootFullPath
	if !strings.HasSuffix(rootWithSeparator, sep) {
		rootWithSeparator += sep
	}

	isWindows := runtime.GOOS == "windows"
	match := false
	if isWindows {
		match = strings.HasPrefix(strings.ToLower(candidateFullPath), strings.ToLower(rootWithSeparator)) ||
			strings.EqualFold(candidateFullPath, rootFullPath)
	} else {
		match = strings.HasPrefix(candidateFullPath, rootWithSeparator) || candidateFullPath == rootFullPath
	}

	if !match {
		return "", false
	}

	return candidateFullPath, true
}

func (r SkillProjectionResolver) getDimensionScore(scores map[string]int, name string, fallback int) int {
	if val, ok := scores[strings.ToLower(name)]; ok {
		return val
	}
	return fallback
}

func (r SkillProjectionResolver) countMatches(requestText string, signals []string) int {
	count := 0
	for _, signal := range signals {
		if r.containsPhrase(requestText, signal) {
			count++
		}
	}
	return count
}

func (r SkillProjectionResolver) hasAnyMatch(requestText string, signals []string) bool {
	for _, signal := range signals {
		if r.containsPhrase(requestText, signal) {
			return true
		}
	}
	return false
}

func (r SkillProjectionResolver) hasExplicitArtifactRequestForView(requestText string, targetView string) bool {
	terms := GetExplicitArtifactTerms(targetView)
	for _, signal := range terms {
		if r.containsPhrase(requestText, signal) {
			return true
		}
	}
	return false
}

func (r SkillProjectionResolver) containsPhrase(requestText string, signal string) bool {
	if strings.TrimSpace(signal) == "" {
		return false
	}
	return strings.Contains(requestText, strings.ToLower(strings.TrimSpace(signal)))
}

func (r SkillProjectionResolver) readDisplayArray(data json.RawMessage) []string {
	if len(data) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil
	}

	var values []string
	for _, item := range arr {
		if text, ok := r.toDisplayText(item); ok && strings.TrimSpace(text) != "" {
			values = append(values, text)
		}
	}
	return values
}

func (r SkillProjectionResolver) toDisplayText(element json.RawMessage) (string, bool) {
	if len(element) == 0 {
		return "", false
	}

	var s string
	if err := json.Unmarshal(element, &s); err == nil {
		return s, true
	}

	var obj map[string]any
	if err := json.Unmarshal(element, &obj); err != nil {
		return "", false
	}

	if text, ok := r.tryBuildOpenQuestionText(obj); ok {
		return text, true
	}

	if text, ok := r.tryBuildDroppedItemText(obj); ok {
		return text, true
	}

	return string(element), true
}

func (r SkillProjectionResolver) tryBuildOpenQuestionText(obj map[string]any) (string, bool) {
	q, ok := obj["question"].(string)
	if !ok || strings.TrimSpace(q) == "" {
		return "", false
	}

	var details []string
	if impact, ok := obj["impact"].(string); ok && strings.TrimSpace(impact) != "" {
		details = append(details, fmt.Sprintf("Impact: %s", impact))
	}
	if reqInput, ok := obj["required_input"].(string); ok && strings.TrimSpace(reqInput) != "" {
		details = append(details, fmt.Sprintf("Required input: %s", reqInput))
	}

	if len(details) > 0 {
		q = fmt.Sprintf("%s (%s)", q, strings.Join(details, "; "))
	}
	return q, true
}

func (r SkillProjectionResolver) tryBuildDroppedItemText(obj map[string]any) (string, bool) {
	reason, ok := obj["reason"].(string)
	if !ok || strings.TrimSpace(reason) == "" {
		return "", false
	}

	itemType, _ := obj["item_type"].(string)
	itemID, _ := obj["item_id"].(string)

	var prefixParts []string
	if strings.TrimSpace(itemType) != "" {
		prefixParts = append(prefixParts, itemType)
	}
	if strings.TrimSpace(itemID) != "" {
		prefixParts = append(prefixParts, itemID)
	}

	prefix := strings.Join(prefixParts, " ")
	if strings.TrimSpace(prefix) != "" {
		reason = fmt.Sprintf("%s: %s", prefix, reason)
	}
	return reason, true
}
