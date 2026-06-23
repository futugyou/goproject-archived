package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/flosch/pongo2/v7"
)

type MetaExecutionContext struct {
	input   string
	outputs map[string]string
	inputs  map[string]any
	steps   map[string]any
}

func NewMetaExecutionContext(input *string, outputs map[string]string, inputs map[string]any, steps map[string]any) *MetaExecutionContext {
	ctx := &MetaExecutionContext{}

	if input != nil {
		ctx.input = *input
	}

	ctx.outputs = make(map[string]string)
	for k, v := range outputs {
		ctx.outputs[strings.ToLower(k)] = v
	}

	ctx.inputs = make(map[string]any)
	for k, v := range inputs {
		ctx.inputs[strings.ToLower(k)] = v
	}

	if _, exists := ctx.inputs["user_message"]; !exists {
		ctx.inputs["user_message"] = ctx.input
	}

	ctx.steps = make(map[string]any)
	for k, v := range steps {
		ctx.steps[strings.ToLower(k)] = v
	}

	return ctx
}

func (m *MetaExecutionContext) Input() string {
	return m.input
}

func (m *MetaExecutionContext) Outputs() map[string]string {
	cpy := make(map[string]string, len(m.outputs))
	maps.Copy(cpy, m.outputs)
	return cpy
}

func (m *MetaExecutionContext) Inputs() map[string]any {
	cpy := make(map[string]any, len(m.inputs))
	maps.Copy(cpy, m.inputs)
	return cpy
}

func (m *MetaExecutionContext) Steps() map[string]any {
	cpy := make(map[string]any, len(m.steps))
	maps.Copy(cpy, m.steps)
	return cpy
}

type MetaTemplateRenderer struct{}

var allowedFilters = map[string]bool{
	"xml_escape": true,
	"slugify":    true,
	"truncate":   true,
	"tojson":     true,
}

var builtinFilterNames = []string{
	"upper", "lower", "capitalize", "title", "replace",
	"first", "last", "join", "reverse", "sort", "length",
	"abs", "round", "int", "float", "string", "list", "trim",
	"default", "safe", "escape", "urlencode",
	"wordcount", "wordwrap", "center", "indent", "format",
	"map", "select", "reject", "attr", "batch", "slice",
	"groupby", "unique", "sum", "min", "max", "random",
	"pprint", "striptags",
}

func init() {
	_ = pongo2.RegisterFilter("xml_escape", filterXmlEscape)
	_ = pongo2.RegisterFilter("slugify", filterSlugify)
	_ = pongo2.RegisterFilter("truncate", filterTruncate)
	_ = pongo2.RegisterFilter("tojson", filterToJson)

	blockedHandler := func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, error) {
		return in, nil
	}

	for _, name := range builtinFilterNames {
		if !allowedFilters[name] {
			_ = pongo2.RegisterFilter(name, blockedHandler)
		}
	}
}

func NewMetaTemplateRenderer() *MetaTemplateRenderer {
	return &MetaTemplateRenderer{}
}

func (r *MetaTemplateRenderer) Render(templateStr string, context *MetaExecutionContext) string {
	if context == nil {
		return "(template render error: context is nil)"
	}

	tpl, err := pongo2.FromString(templateStr)
	if err != nil {
		return fmt.Sprintf("(template render error: %s)", err.Error())
	}

	insensitiveOutputs := make(map[string]any)
	for k, v := range context.Outputs() {
		insensitiveOutputs[strings.ToLower(k)] = v
	}

	ctxData := pongo2.Context{
		"input":   context.Input,
		"inputs":  context.Inputs,
		"outputs": insensitiveOutputs,
		"steps":   context.Steps,
	}

	result, err := tpl.Execute(ctxData)
	if err != nil {
		return fmt.Sprintf("(template render error: %s)", err.Error())
	}

	return result
}

func filterXmlEscape(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, error) {
	return pongo2.AsValue(html.EscapeString(in.String())), nil
}

func filterSlugify(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, error) {
	val := strings.TrimSpace(strings.ToLower(in.String()))
	if val == "" {
		return pongo2.AsValue(""), nil
	}

	var buf bytes.Buffer
	previousDash := false

	for _, ch := range val {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			buf.WriteRune(ch)
			previousDash = false
			continue
		}

		if previousDash {
			continue
		}

		buf.WriteByte('-')
		previousDash = true
	}

	slug := strings.Trim(buf.String(), "-")
	return pongo2.AsValue(slug), nil
}

func filterTruncate(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, error) {
	value := in.String()
	maxLength := 80

	if param != nil && param.String() != "" {
		if idx, err := strconv.Atoi(param.String()); err == nil {
			maxLength = idx
		}
	}

	if maxLength <= 0 {
		maxLength = 80
	}

	runes := []rune(value)
	if len(runes) <= maxLength {
		return pongo2.AsValue(value), nil
	}

	if maxLength <= 3 {
		return pongo2.AsValue(strings.Repeat(".", maxLength)), nil
	}

	truncated := strings.TrimRightFunc(string(runes[:maxLength-3]), unicode.IsSpace)
	return pongo2.AsValue(truncated + "..."), nil
}

func filterToJson(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, error) {
	if in.IsNil() {
		return pongo2.AsValue("null"), nil
	}

	jsonBytes, err := json.Marshal(in.Interface())
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:tojson",
			OrigError: fmt.Errorf("unsupported tojson value type: %w", err),
		}
	}

	return pongo2.AsValue(string(jsonBytes)), nil
}

type MetaToolArgumentResolver struct {
	renderer *MetaTemplateRenderer
}

func NewMetaToolArgumentResolver(renderer *MetaTemplateRenderer) *MetaToolArgumentResolver {
	return &MetaToolArgumentResolver{
		renderer: renderer,
	}
}

func (r *MetaToolArgumentResolver) Resolve(
	compositionToolArgsJSON *string,
	withJSON *string,
	stepToolArgsJSON *string,
	context *MetaExecutionContext,
) (string, error) {

	merged := make(map[string]any)
	if err := mergeInto(merged, compositionToolArgsJSON); err != nil {
		return "", err
	}
	if err := mergeInto(merged, withJSON); err != nil {
		return "", err
	}
	if err := mergeInto(merged, stepToolArgsJSON); err != nil {
		return "", err
	}

	serialized, err := json.Marshal(merged)
	if err != nil {
		return "", errors.New("invalid_tool_args")
	}

	rendered := r.renderer.Render(string(serialized), context)

	var finalObj map[string]any
	decoder := json.NewDecoder(strings.NewReader(rendered))
	if err := decoder.Decode(&finalObj); err != nil {
		return "", errors.New("invalid_tool_args")
	}

	finalBytes, err := json.Marshal(finalObj)
	if err != nil {
		return "", errors.New("invalid_tool_args")
	}

	return string(finalBytes), nil
}

func mergeInto(target map[string]any, jsonStr *string) error {
	if jsonStr == nil || strings.TrimSpace(*jsonStr) == "" {
		return nil
	}

	var parsedNode map[string]any
	if err := json.Unmarshal([]byte(*jsonStr), &parsedNode); err != nil {
		return errors.New("invalid_tool_args")
	}

	maps.Copy(target, parsedNode)

	return nil
}

type operatorPart struct {
	Expression string
	Operator   string
}

type MetaConditionEvaluator struct {
	renderer *MetaTemplateRenderer
}

// 编译正则表达式（不区分大小写）
var notPrefix = regexp.MustCompile(`(?i)^not\s+`)

func NewMetaConditionEvaluator(renderer *MetaTemplateRenderer) *MetaConditionEvaluator {
	return &MetaConditionEvaluator{
		renderer: renderer,
	}
}

// Evaluate 评估表达式的布尔结果
func (m *MetaConditionEvaluator) Evaluate(expression string, context *MetaExecutionContext) bool {
	candidate := strings.TrimSpace(expression)
	if candidate == "" {
		return false
	}

	parts := m.SplitByTopLevelOperators(candidate)

	if len(parts) == 1 {
		return m.EvaluateAtomic(parts[0].Expression, context)
	}

	// ── 按照正确的优先级组合 ──
	// 首先评估所有原子表达式，然后尊重 "and" > "or" 的优先级：
	// 按 "or" 边界进行分组，每个分组内是一个 "and" 链条。
	evaluated := make([]bool, len(parts))
	for i, part := range parts {
		evaluated[i] = m.EvaluateAtomic(part.Expression, context)
	}

	var orResults []bool
	currentAnd := evaluated[0]

	for i := 1; i < len(parts); i++ {
		op := parts[i-1].Operator
		if strings.EqualFold(op, "and") {
			currentAnd = currentAnd && evaluated[i]
		} else { // "or"
			orResults = append(orResults, currentAnd)
			currentAnd = evaluated[i]
		}
	}
	orResults = append(orResults, currentAnd)

	// 任何一个 "or" 分组为 true，整个表达式即为 true
	for _, r := range orResults {
		if r {
			return true
		}
	}

	return false
}

// EvaluateAtomic 评估单个原子表达式
func (m *MetaConditionEvaluator) EvaluateAtomic(expression string, context *MetaExecutionContext) bool {
	candidate := strings.TrimSpace(expression)

	// ── 预包装 {{ … and … }} 回退处理 ──
	if (strings.HasPrefix(candidate, "{{") || strings.HasPrefix(candidate, "{%")) &&
		(strings.Contains(strings.ToLower(candidate), " and ") || strings.Contains(strings.ToLower(candidate), " or ")) {

		inner := candidate
		if strings.HasPrefix(candidate, "{{") && strings.HasSuffix(candidate, "}}") {
			inner = strings.TrimSpace(candidate[2 : len(candidate)-2])
		} else if strings.HasPrefix(candidate, "{%") && strings.HasSuffix(candidate, "%}") {
			inner = m.ExtractIfCondition(inner)
		}

		return m.Evaluate(inner, context)
	}

	// ── 正常原子评估 ──
	negate := false
	loc := notPrefix.FindStringIndex(candidate)
	if loc != nil {
		negate = true
		candidate = strings.TrimSpace(candidate[loc[1]:])
	}

	var template string
	if strings.Contains(candidate, "{{") || strings.Contains(candidate, "{%") {
		template = candidate
	} else {
		template = "{{ " + candidate + " }}"
	}

	rendered := m.renderer.Render(template, context)
	result := m.IsTruthy(rendered)

	if negate {
		return !result
	}
	return result
}

// ExtractIfCondition 从 {% if ... %} 或 {% unless ... %} 中提取条件
func (m *MetaConditionEvaluator) ExtractIfCondition(template string) string {
	inner := template[2:] // 剥离前面的 {%
	endIdx := strings.Index(inner, "%}")
	if endIdx < 0 {
		return inner
	}
	inner = strings.TrimSpace(inner[:endIdx])

	const ifKeyword = "if "
	const unlessKeyword = "unless "

	if len(inner) >= len(ifKeyword) && strings.EqualFold(inner[:len(ifKeyword)], ifKeyword) {
		return strings.TrimSpace(inner[len(ifKeyword):])
	}
	if len(inner) >= len(unlessKeyword) && strings.EqualFold(inner[:len(unlessKeyword)], unlessKeyword) {
		return strings.TrimSpace(inner[len(unlessKeyword):])
	}

	return inner
}

// SplitByTopLevelOperators 在顶层 "and"/"or" 边界拆分表达式
func (m *MetaConditionEvaluator) SplitByTopLevelOperators(expression string) []operatorPart {
	var parts []operatorPart
	depth := 0
	inSingleQuote := false
	inDoubleQuote := false
	inCurly := 0
	lastSplit := 0

	runes := []rune(expression) // 转为 rune 切片以正确处理多字节字符（如果存在）
	length := len(runes)

	for i := 0; i < length; i++ {
		ch := runes[i]

		// 跟踪字符串字面量边界
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
		} else if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
		}

		if inSingleQuote || inDoubleQuote {
			continue
		}

		// 跟踪括号深度
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		}

		// 跟踪 {{ ... }} / {% ... %} 深度
		if ch == '{' && i+1 < length && (runes[i+1] == '{' || runes[i+1] == '%') {
			inCurly++
			i++ // 跳过第二个大括号/百分号
			continue
		}
		if (ch == '}' || ch == '%') && i+1 < length && runes[i+1] == '}' {
			if inCurly > 0 {
				inCurly--
			}
			i++
			continue
		}

		// 仅在深度为 0 且在 Jinja 定界符之外时拆分
		if depth > 0 || inCurly > 0 {
			continue
		}

		// 检查单词边界处的 "and" / "or"
		if nextIndex, matched := m.TryMatchLogicalOperator(runes, i, "and"); matched {
			expr := strings.TrimSpace(string(runes[lastSplit:i]))
			if len(expr) > 0 {
				parts = append(parts, operatorPart{Expression: expr, Operator: "and"})
			}
			i = nextIndex - 1
			lastSplit = nextIndex
			continue
		}

		if nextIndex, matched := m.TryMatchLogicalOperator(runes, i, "or"); matched {
			expr := strings.TrimSpace(string(runes[lastSplit:i]))
			if len(expr) > 0 {
				parts = append(parts, operatorPart{Expression: expr, Operator: "or"})
			}
			i = nextIndex - 1
			lastSplit = nextIndex
			continue
		}
	}

	// 添加尾部剩余部分
	tail := strings.TrimSpace(string(runes[lastSplit:]))
	if len(tail) > 0 {
		parts = append(parts, operatorPart{Expression: tail, Operator: ""})
	}

	return parts
}

// TryMatchLogicalOperator 尝试匹配逻辑运算符并返回下一个索引位置
func (m *MetaConditionEvaluator) TryMatchLogicalOperator(runes []rune, index int, operatorText string) (int, bool) {
	nextIndex := index
	opRunes := []rune(operatorText)
	opLen := len(opRunes)

	if index < 0 || index+opLen > len(runes) {
		return index, false
	}

	// 不区分大小写比对
	for i := 0; i < opLen; i++ {
		if unicode.ToLower(runes[index+i]) != unicode.ToLower(opRunes[i]) {
			return index, false
		}
	}

	// 检查前边界
	beforeOk := index == 0 || unicode.IsSpace(runes[index-1]) || runes[index-1] == '('
	if !beforeOk {
		return index, false
	}

	// 检查后边界
	afterIndex := index + opLen
	afterOk := afterIndex == len(runes) || unicode.IsSpace(runes[afterIndex]) || runes[afterIndex] == ')'
	if !afterOk {
		return index, false
	}

	nextIndex = afterIndex
	for nextIndex < len(runes) && unicode.IsSpace(runes[nextIndex]) {
		nextIndex++
	}

	return nextIndex, true
}

// IsTruthy 判断字符串的真假值
func (m *MetaConditionEvaluator) IsTruthy(value string) bool {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return false
	}

	if boolValue, err := strconv.ParseBool(strings.ToLower(normalized)); err == nil {
		return boolValue
	}

	// 额外排查特定代表 false 的字符串
	lower := strings.ToLower(normalized)
	return lower != "0" &&
		lower != "no" &&
		lower != "off" &&
		lower != "null" &&
		lower != "none" &&
		lower != "undefined"
}

type MetaSkillStepDefinition struct {
	ID string `json:"id"`
	// Step kind (agent, tool_call, llm_chat, etc.).
	Kind                string                 `json:"kind"`
	Skill               *string                `json:"skill,omitempty"`
	Tool                *string                `json:"tool,omitempty"`
	SkillExecEntrypoint *string                `json:"skill_exec_entrypoint,omitempty"`
	SkillExecArgs       []string               `json:"skill_exec_args,omitempty"`
	SkillExecStdin      *string                `json:"skill_exec_stdin,omitempty"`
	SkillExecCwd        *string                `json:"skill_exec_cwd,omitempty"`
	SkillExecParseMode  *string                `json:"skill_exec_parse_mode,omitempty"`
	WithJSON            *string                `json:"with_json,omitempty"`
	When                *string                `json:"when,omitempty"`
	ToolArgsJSON        *string                `json:"tool_args_json,omitempty"`
	ToolAllowlist       []string               `json:"tool_allowlist,omitempty"`
	OutputChoices       []string               `json:"output_choices,omitempty"`
	Clarify             *MetaClarifySchema     `json:"clarify,omitempty"`
	Routes              []MetaRouteDefinition  `json:"routes,omitempty"`
	DependsOn           []string               `json:"depends_on,omitempty"`
	OnFailure           *string                `json:"on_failure,omitempty"`
	TimeoutSeconds      *int                   `json:"timeout_seconds,omitempty"`
	Retry               MetaStepRetryPolicy    `json:"retry"`
	OutputContract      MetaStepOutputContract `json:"output_contract"`
}

type MetaClarifySchema struct {
	// Interaction mode, such as chat or form. Defaults to "chat".
	Mode                   string             `json:"mode"`
	ExtractNaturalLanguage bool               `json:"extract_natural_language"`
	Fields                 []MetaClarifyField `json:"fields,omitempty"`
	CancelWords            []string           `json:"cancel_words,omitempty"`
	SkipIf                 *string            `json:"skip_if,omitempty"`
	TimeoutSeconds         *int               `json:"timeout_seconds,omitempty"`
}

type MetaClarifyField struct {
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Required     bool             `json:"required"`
	DefaultValue *json.RawMessage `json:"default_value,omitempty"`
	Options      []string         `json:"options,omitempty"`
	MinLength    *int             `json:"min_length,omitempty"`
	MaxLength    *int             `json:"max_length,omitempty"`
	Min          *float64         `json:"min,omitempty"`
	Max          *float64         `json:"max,omitempty"`
}

type MetaRouteDefinition struct {
	When string `json:"when"`
	To   string `json:"to"`
}

type MetaStepRetryPolicy struct {
	// Total attempts, including the initial try. Defaults to 1.
	MaxAttempts int `json:"max_attempts"`
	BackoffMs   int `json:"backoff_ms"`
}

type MetaStepOutputContract struct {
	// Expected format. Supported values: text, json. Defaults to "text".
	Format             string   `json:"format"`
	RequiredProperties []string `json:"required_properties,omitempty"`
}

type MetaRoutePlanner struct {
	conditionEvaluator *MetaConditionEvaluator
}

func NewMetaRoutePlanner(conditionEvaluator *MetaConditionEvaluator) *MetaRoutePlanner {
	return &MetaRoutePlanner{
		conditionEvaluator: conditionEvaluator,
	}
}

// SelectNextStep 选择下一步
func (m *MetaRoutePlanner) SelectNextStep(step *MetaSkillStepDefinition, context *MetaExecutionContext) (string, bool) {
	if step == nil || context == nil {
		return "", false
	}

	for _, route := range step.Routes {
		if strings.TrimSpace(route.When) == "" || m.conditionEvaluator.Evaluate(route.When, context) {
			return route.To, true
		}
	}

	return "", false
}

// ApplyInitialRoutingBlocks 应用初始路由阻断
func (m *MetaRoutePlanner) ApplyInitialRoutingBlocks(
	steps []*MetaSkillStepDefinition,
	blocked map[string]struct{},
	pending map[string]struct{},
) {
	if steps == nil || blocked == nil || pending == nil {
		return
	}

	for _, step := range steps {
		if step == nil {
			continue
		}
		for _, route := range step.Routes {
			blocked[route.To] = struct{}{}
			delete(pending, route.To)
		}
	}
}

func (m *MetaRoutePlanner) ApplyCompletionRouting(
	step *MetaSkillStepDefinition,
	context *MetaExecutionContext,
	stepById map[string]*MetaSkillStepDefinition,
	blocked map[string]struct{},
	pending map[string]struct{},
	dependentsByStep map[string][]string,
) {
	if step == nil || context == nil || stepById == nil || blocked == nil || pending == nil || dependentsByStep == nil {
		return
	}

	if len(step.Routes) == 0 {
		return
	}

	selectedTarget, hasTarget := m.SelectNextStep(step, context)

	for _, route := range step.Routes {
		if _, exists := stepById[route.To]; !exists {
			continue
		}

		if hasTarget && strings.EqualFold(route.To, selectedTarget) {
			delete(blocked, route.To)
			pending[route.To] = struct{}{}
			continue
		}

		m.blockStepAndDependents(route.To, blocked, pending, dependentsByStep)
	}
}

func (m *MetaRoutePlanner) blockStepAndDependents(
	stepID string,
	blocked map[string]struct{},
	pending map[string]struct{},
	dependentsByStep map[string][]string,
) {
	stack := []string{stepID}

	for len(stack) > 0 {
		index := len(stack) - 1
		current := stack[index]
		stack = stack[:index]

		if _, exists := blocked[current]; exists {
			continue
		}
		blocked[current] = struct{}{}

		delete(pending, current)

		dependents, exists := dependentsByStep[current]
		if !exists {
			continue
		}

		for _, dependent := range dependents {
			stack = append(stack, dependent)
		}
	}
}

type MetaClarifyValidator struct{}

func (v *MetaClarifyValidator) ValidateAndNormalize(input string, schema *MetaClarifySchema) *MetaClarifyValidationResult {
	if schema == nil {
		panic("schema cannot be nil")
	}

	if matchesCancelWord(input, schema.CancelWords) {
		return InvalidMetaClarifyValidationResult("user_input_cancelled")
	}

	if !strings.EqualFold(schema.Mode, "form") {
		return ValidMetaClarifyValidationResult(input)
	}

	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		return InvalidMetaClarifyValidationResult("clarify_input_required")
	}

	// 解析为通用的 map[string]any 以模拟 JsonDocument/RootElement
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(trimmedInput))
	decoder.UseNumber() // 保持数字精度，避免自动转为 float64 丢失整数特征
	if err := decoder.Decode(&root); err != nil {
		return InvalidMetaClarifyValidationResult("clarify_invalid_json")
	}

	if root == nil {
		return InvalidMetaClarifyValidationResult("clarify_invalid_shape")
	}

	normalizedMap := make(map[string]any)

	for _, field := range schema.Fields {
		value, failureCode, ok := tryResolveFieldValue(root, field)
		if !ok {
			return InvalidMetaClarifyValidationResult(failureCode)
		}

		// 如果值有效且不是“未定义”，则写入结果中
		if value != nil {
			normalizedMap[field.Name] = value
		}
	}

	// 序列化回 JSON 字符串
	outputBytes, err := json.Marshal(normalizedMap)
	if err != nil {
		return InvalidMetaClarifyValidationResult("clarify_invalid_shape")
	}

	return ValidMetaClarifyValidationResult(string(outputBytes))
}

func matchesCancelWord(input string, cancelWords []string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || len(cancelWords) == 0 {
		return false
	}

	for _, cancelWord := range cancelWords {
		if strings.EqualFold(trimmed, cancelWord) {
			return true
		}
	}
	return false
}

func tryResolveFieldValue(root map[string]any, field MetaClarifyField) (any, string, bool) {
	rawVal, exists := root[field.Name]

	// 1. 处理属性不存在的情况
	if !exists {
		if field.DefaultValue != nil {
			return field.DefaultValue, "", true
		}
		if field.Required {
			return nil, "clarify_required_field_missing", false
		}
		return nil, "", true // 相当于 Undefined，不写入最后的 JSON
	}

	// 2. 处理属性存在但为 null 的情况
	if rawVal == nil {
		if field.Required {
			return nil, "clarify_required_field_missing", false
		}
		return nil, "", true
	}

	// 3. 根据字段类型进行校验
	switch strings.ToLower(field.Type) {
	case "string":
		strVal, ok := rawVal.(string)
		if !ok {
			return nil, "clarify_invalid_type", false
		}
		if field.MinLength != nil && len(strVal) < *field.MinLength {
			return nil, "clarify_min_length", false
		}
		if field.MaxLength != nil && len(strVal) > *field.MaxLength {
			return nil, "clarify_max_length", false
		}
		return strVal, "", true

	case "enum":
		strVal, ok := rawVal.(string)
		if !ok {
			return nil, "clarify_invalid_type", false
		}
		found := slices.Contains(field.Options, strVal)
		if !found {
			return nil, "clarify_invalid_option", false
		}
		return strVal, "", true

	case "number":
		// 由于启用了 UseNumber()，数字会是 json.Number 类型
		jsonNum, ok := rawVal.(json.Number)
		if !ok {
			return nil, "clarify_invalid_type", false
		}
		numVal, err := jsonNum.Float64()
		if err != nil {
			return nil, "clarify_invalid_type", false
		}
		if field.Min != nil && numVal < *field.Min {
			return nil, "clarify_min", false
		}
		if field.Max != nil && numVal > *field.Max {
			return nil, "clarify_max", false
		}
		return numVal, "", true

	case "integer":
		jsonNum, ok := rawVal.(json.Number)
		if !ok {
			return nil, "clarify_invalid_type", false
		}
		intVal, err := jsonNum.Int64()
		if err != nil {
			// 如果带有小数点，Int64() 会报错，说明它不是一个合法的整数类型
			return nil, "clarify_invalid_type", false
		}
		if field.Min != nil && float64(intVal) < *field.Min {
			return nil, "clarify_min", false
		}
		if field.Max != nil && float64(intVal) > *field.Max {
			return nil, "clarify_max", false
		}
		return intVal, "", true

	case "boolean":
		boolVal, ok := rawVal.(bool)
		if !ok {
			return nil, "clarify_invalid_type", false
		}
		return boolVal, "", true
	}

	return nil, "clarify_invalid_type", false
}

// MetaClarifyValidationResult 保存验证结果
type MetaClarifyValidationResult struct {
	IsValid          bool
	NormalizedOutput string
	FailureCode      string
}

func ValidMetaClarifyValidationResult(output string) *MetaClarifyValidationResult {
	return &MetaClarifyValidationResult{
		IsValid:          true,
		NormalizedOutput: output,
	}
}

func InvalidMetaClarifyValidationResult(failureCode string) *MetaClarifyValidationResult {
	return &MetaClarifyValidationResult{
		IsValid:     false,
		FailureCode: failureCode,
	}
}
