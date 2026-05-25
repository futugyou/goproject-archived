package models

type ModelOptions struct {
	Provider    string
	Model       string
	Endpoint    string
	ApiKey      string
	Temperature float32
	MaxTokens   int
}

func DefefaultModelOptions() *ModelOptions {
	return &ModelOptions{
		Provider:    "ollama",
		Model:       "llama3.2",
		Temperature: 0.7,
		MaxTokens:   4096,
	}
}

const (
	OllamaProviderType = "ollama"
	OllamaEndpoint     = "http://localhost:11434"
	OllamaModel        = "gemma4:e2b"
	OllamaDisplayName  = "Ollama (Local)"
	OllamaTemperature  = 0.7
	OllamaMaxTokens    = 4096
	OpenAIProviderType = "azure-openai"
	OpenAIModel        = "gpt-5-mini"
	OpenAIAuthMode     = "api-key"
	OpenAIDisplayName  = "Azure OpenAI"
)

type ResolvedProviderConfig struct {
	ProviderType   string
	Endpoint       string
	Model          string
	ApiKey         string
	DeploymentName string
	AuthMode       string
	DefinitionName string
}
