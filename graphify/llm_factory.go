package graphify

import (
	"strings"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/extensions_ai/openai"
)

func ChatClientResolver(config *GraphifyConfig) chatcompletion.IChatClient {
	if config == nil {
		return nil
	}

	switch strings.ToLower(config.Provider) {
	case "openai":
		opt := &openai.OpenAIOption{}
		if len(config.OpenAI.ApiKey) > 0 {
			opt.ApiKey = config.OpenAI.ApiKey
		}
		if len(config.OpenAI.Endpoint) > 0 {
			opt.Endpoint = config.OpenAI.Endpoint
		}
		if len(config.OpenAI.ModelId) > 0 {
			opt.ModelId = config.OpenAI.ModelId
		}
		return openai.DefaultOpenAIChatClient(opt)
	}
	return nil
}
