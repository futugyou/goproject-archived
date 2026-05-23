package graphify

type GraphifyConfig struct {
	Provider      string
	WorkingFolder string
	OutputFolder  string
	ExportFormats string
	OpenAI        *OpenAIConfig
}

type OpenAIConfig struct {
	Endpoint string
	ApiKey   string
	ModelId  string
}

type CliProviderOptions struct {
	Provider string
	Endpoint string
	ApiKey   string
	Model    string
}
