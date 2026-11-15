package cli

const DefaultConfigPath = "~/.openclaw/config/openclaw.settings.json"
const DefaultApiKeyRef = "env:MODEL_PROVIDER_KEY"
const DefaultProvider = "openai"
const DefaultOllamaPresetId = "ollama-general"
const DefaultEmbeddedPresetId = "embedded-gemma-small-q4"
const DefaultEmbeddedModelId = "gemma-local-small-q4"
const DefaultBackendChoice = "none"

type SetupAnswers struct {
	Profile             string
	ConfigPath          string
	Workspace           string
	Provider            string
	ModelPresetId       string
	Model               string
	ApiKey              string
	BindAddress         string
	Port                int
	AuthToken           string
	BackendChoice       string
	DockerImage         string
	OpenSandboxEndpoint string
	SshHost             string
	SshUser             string
	SshKey              string
}

type OllamaDiscoveryResult struct {
	IsAvailable bool
	Models      []string
}

type SetupCommandResult struct {
	ExitCode   int
	ConfigPath string
}
