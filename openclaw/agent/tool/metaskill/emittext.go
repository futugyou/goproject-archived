package metaskill

import (
	"context"
	"encoding/json"
)

type EmitTextTool struct {
}

func (e *EmitTextTool) Name() string {
	return "emit_text"
}

func (e *EmitTextTool) Description() string {
	return "Emit fixed text as tool output."
}

func (e *EmitTextTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"text": {
			"type": "string"
		}
	},
	"required": ["text"]
}`
}

func (a *EmitTextTool) Execute(ctx context.Context, argumentsJson string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}

	if argumentsJson == "" {
		argumentsJson = "{}"
	}

	var doc map[string]any

	if err := json.Unmarshal([]byte(argumentsJson), &doc); err != nil {
		return err.Error()
	}

	text, ok := doc["text"].(string)
	if !ok {
		return SerializeError("invalid_arguments", "'text' is required.")
	}

	return text
}
