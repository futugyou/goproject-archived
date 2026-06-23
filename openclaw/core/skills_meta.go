package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"maps"
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
