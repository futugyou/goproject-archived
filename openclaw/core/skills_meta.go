package core

import (
	"maps"
	"strings"
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
