package server

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/extensions_ai/abstractions/functions"
	"github.com/futugyou/mcp"
	"github.com/futugyou/mcp/core"
)

var _ IMcpServerPrompt = (*AIFunctionMcpServerPrompt)(nil)

type AIFunctionMcpServerPrompt struct {
	AIFunction     functions.AIFunction
	ProtocolPrompt *core.Prompt
}

// GetId implements IMcpServerPrompt.
func (m *AIFunctionMcpServerPrompt) GetId() string {
	if m == nil || m.AIFunction == nil {
		return ""
	}

	return m.AIFunction.GetName()
}

// GetProtocolPrompt implements IMcpServerPrompt.
func (m *AIFunctionMcpServerPrompt) GetProtocolPrompt() *core.Prompt {
	if m == nil {
		return nil
	}

	return m.ProtocolPrompt
}

func PromptCreate(function functions.AIFunction, options McpServerPromptCreateOptions) AIFunctionMcpServerPrompt {
	args := []core.PromptArgument{}
	requiredList := []string{}
	if requireds, ok := function.GetJsonSchema()["required"].(json.RawMessage); ok {
		json.Unmarshal(requireds, &requiredList)
	}

	if properties, ok := function.GetJsonSchema()["properties"].(json.RawMessage); ok {
		var propertyList []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(properties, &propertyList); err == nil {
			for _, property := range propertyList {
				required := false
				for _, v := range requiredList {
					if v == property.Name {
						required = true
						break
					}
				}
				args = append(args, core.PromptArgument{
					Name:        property.Name,
					Description: &property.Description,
					Required:    &required,
				})
			}
		}
	}

	prompt := &core.Prompt{
		Arguments:   args,
		Description: options.Description,
	}

	if options.Name != nil {
		prompt.Name = *options.Name
	}

	return AIFunctionMcpServerPrompt{
		ProtocolPrompt: prompt,
		AIFunction:     function,
	}
}

func PromptDynamicCreate(methodInfo reflect.Value, options McpServerPromptCreateOptions) AIFunctionMcpServerPrompt {
	factory := functions.NewAIFunctionFactory()
	op := createPromptCreateOption(options, methodInfo)
	function, err := factory.Create(methodInfo, op)
	if err != nil {
		return AIFunctionMcpServerPrompt{}
	}
	return PromptCreate(function, options)
}

func createPromptCreateOption(options McpServerPromptCreateOptions, methodInfo reflect.Value) *functions.AIFunctionFactoryOptions {
	op := &functions.AIFunctionFactoryOptions{
		SerializerOptions:    &json.Encoder{},
		ParameterNames:       []string{},
		JSONSchemaOptions:    map[string]any{},
		AdditionalProperties: map[string]any{},
	}
	if options.Name != nil {
		op.Name = *options.Name
	} else {
		op.Name = methodInfo.Type().Method(0).Name
	}
	if options.Description != nil {
		op.Description = *options.Description
	}
	return op
}

func (m *AIFunctionMcpServerPrompt) Get(ctx context.Context, request RequestContext[*core.GetPromptRequestParams]) (*core.GetPromptResult, error) {
	if m == nil || m.AIFunction == nil {
		return nil, fmt.Errorf("ai function is nil")
	}

	if request.Params == nil {
		return nil, fmt.Errorf("request.Params is nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	arguments := functions.AIFunctionArguments{
		Context: map[any]any{reflect.TypeOf(request): request},
	}

	if request.Params.Arguments != nil {
		for key, v := range request.Params.Arguments {
			arguments.Set(key, v)
		}
	}

	result, err := m.AIFunction.Invoke(ctx, arguments)
	if err != nil {
		return nil, err
	}
	switch r := result.(type) {
	case *string:
		return &core.GetPromptResult{
			Description: m.ProtocolPrompt.Description,
			Messages: []core.PromptMessage{{
				Content: core.ContentBlock{
					IContentBlock: &core.TextContentBlock{
						Text: *r,
					}},
				Role: core.RoleUser,
			}},
		}, nil
	case *core.GetPromptResult:
		return r, nil
	case *core.PromptMessage:
		return &core.GetPromptResult{
			Description: m.ProtocolPrompt.Description,
			Messages:    []core.PromptMessage{*r},
		}, nil
	case []core.PromptMessage:
		return &core.GetPromptResult{
			Description: m.ProtocolPrompt.Description,
			Messages:    r,
		}, nil
	case *chatcompletion.ChatMessage:
		return &core.GetPromptResult{
			Description: m.ProtocolPrompt.Description,
			Messages:    mcp.ChatMessageToPromptMessages(*r),
		}, nil
	case []chatcompletion.ChatMessage:
		msg := []core.PromptMessage{}
		for _, r := range r {
			msg = append(msg, mcp.ChatMessageToPromptMessages(r)...)
		}
		return &core.GetPromptResult{
			Description: m.ProtocolPrompt.Description,
			Messages:    msg,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
}
