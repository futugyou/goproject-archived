package smokeecho

import (
	"context"
	"encoding/json"
	"strings"
)

type StreamingSmokeEchoTool struct {
}

func New() *StreamingSmokeEchoTool {
	return &StreamingSmokeEchoTool{}
}

func (a *StreamingSmokeEchoTool) Name() string {
	return "stream_echo"
}

func (a *StreamingSmokeEchoTool) Description() string {
	return "Experimental smoke tool that streams the provided chunks and returns their concatenated text."
}

func (a *StreamingSmokeEchoTool) ParameterSchema() string {
	return `{
          "type": "object",
          "properties": {
            "chunks": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Chunks to stream in order."
            }
          },
          "required": ["chunks"]
        }`
}

type SmokeEchoModel struct {
	Chunks []string `json:"chunks"`
}

func parseChunks(argumentsJson string) []string {
	if argumentsJson == "" {
		return []string{"missing-chunks"}
	}

	var model SmokeEchoModel

	json.Unmarshal([]byte(argumentsJson), &model)

	if len(model.Chunks) == 0 {
		return []string{}
	}

	return model.Chunks
}

func (a *StreamingSmokeEchoTool) Execute(ctx context.Context, argumentsJson string) string {
	return strings.Join(parseChunks(argumentsJson), "")
}

func (a *StreamingSmokeEchoTool) ExecuteStreaming(ctx context.Context, argumentsJson string) (<-chan string, error) {
	chunks := parseChunks(argumentsJson)
	data := make(chan string, len(chunks))

	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data <- chunk
	}
	close(data)
	return data, nil
}
