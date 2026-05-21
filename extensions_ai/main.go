package main

import (
	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/extensions_ai/abstractions/embeddings"
	"github.com/futugyou/extensions_ai/ollama"
	"github.com/futugyou/extensions_ai/openai"
)

func main() {
	var _ embeddings.IEmbedding = (*embeddings.EmbeddingT[float64])(nil)

	var _ chatcompletion.IChatClient = (*ollama.OllamaChatClient)(nil)
	var _ embeddings.IEmbeddingGenerator[string, embeddings.EmbeddingT[float64]] = (*ollama.OllamaEmbeddingGenerator[string, embeddings.EmbeddingT[float64]])(nil)

	var _ embeddings.IEmbeddingGenerator[string, embeddings.EmbeddingT[float64]] = (*openai.OpenAIEmbeddingGenerator[embeddings.EmbeddingT[float64]])(nil)
	var _ chatcompletion.IChatClient = (*openai.OpenAIChatClient)(nil)
	var _ chatcompletion.IChatClient = (*openai.OpenAIResponseChatClient)(nil)
}
