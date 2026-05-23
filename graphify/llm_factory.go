package graphify

import (
	"strings"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	openaiex "github.com/futugyou/extensions_ai/openai"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func ChatClientResolver(config *GraphifyConfig) chatcompletion.IChatClient {
	switch strings.ToLower(config.Provider) {
	case "openai":
		op := []option.RequestOption{}
		if config != nil && len(config.OpenAI.ApiKey) > 0 {
			op = append(op, option.WithAPIKey(config.OpenAI.ApiKey))
		}
		if config != nil && len(config.OpenAI.Endpoint) > 0 {
			op = append(op, option.WithBaseURL(config.OpenAI.Endpoint))
		}
		var modelId *string = nil
		if config != nil && len(config.OpenAI.ModelId) > 0 {
			modelId = &config.OpenAI.ModelId
		}
		c := openai.NewClient(op...)
		return openaiex.NewOpenAIChatClient(&c, modelId)
	}
	return nil
}
