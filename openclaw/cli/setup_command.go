package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

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

func providerRequiresApiKey(provider string) bool {
	provider = strings.TrimSpace(provider)
	return provider != "ollama" && provider != "embedded"
}

func setupCommandRequiresPrompt(parsed *CliArgs) bool {
	var required = []string{
		"--profile",
		"--workspace",
		"--provider",
		"--model",
	}

	for _, option := range required {
		if util.Deref(parsed.GetOption(option)) == "" {
			return true
		}
	}

	return providerRequiresApiKey(util.Deref(parsed.GetOption("--provider"))) && util.Deref(parsed.GetOption("--api-key")) == ""
}

func SetupCommandRun(args []string, input io.Reader, output io.Writer, errwirter io.Writer, currentDirectory string, canPrompt bool) (*SetupCommandResult, error) {
	if len(args) > 0 && args[0] == "status" {
		code, err := SetupLifecycleCommandRunStatus(args[1:], output, errwirter)
		return &SetupCommandResult{ExitCode: code}, err
	}

	if len(args) > 0 && args[0] == "verify" {
		code, err := SetupLifecycleCommandRunVerify(args[1:], output, errwirter)
		return &SetupCommandResult{ExitCode: code}, err
	}

	if len(args) > 0 && args[0] == "channel" {
		code, err := ChannelSetupCommandRun(args[1:], input, output, errwirter, canPrompt)
		return &SetupCommandResult{ExitCode: code}, err
	}

	if len(args) > 0 && args[0] == "provide" {
		code, err := setupCommandRunProviderSetup(args[1:], input, output, errwirter, currentDirectory, canPrompt)
		return &SetupCommandResult{ExitCode: code}, err
	}

	if len(args) > 0 && args[0] == "tailscale" {
		code, err := setupCommandRunTailscaleSetup(args[1:], input, output, errwirter, canPrompt)
		return &SetupCommandResult{ExitCode: code}, err
	}

	parsed := CliArgsParse(args)
	nonInteractive := parsed.HasFlag("--non-interactive")
	requiresPrompt := setupCommandRequiresPrompt(parsed)

	if requiresPrompt && !nonInteractive && !canPrompt {
		msg := "Missing setup inputs and no interactive terminal is available. Re-run with --non-interactive and explicit values, or run 'openclaw setup' from a terminal."
		fmt.Fprintln(errwirter, msg)
		return &SetupCommandResult{ExitCode: 2}, errors.New(msg)
	}

	var answers *SetupAnswers
	var err error
	if requiresPrompt && !nonInteractive {
		answers, err = setupCommandPromptForAnswers(parsed, input, output, currentDirectory)
	} else {
		answers, err = setupCommandBuildAnswersFromArgs(parsed, currentDirectory, nonInteractive)
	}

	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return &SetupCommandResult{ExitCode: 2}, err
	}

	config, warnings, err := setupCommandBuildConfig(answers)
	if err != nil {
		return &SetupCommandResult{ExitCode: 2}, err
	}

	validationErrors := core.ConfigValidatorInstance.Validate(config)
	if len(validationErrors) > 0 {
		fmt.Fprintln(errwirter, "Config validation failed:")
		for _, validationError := range validationErrors {
			fmt.Fprintf(errwirter, "- %s\n", validationError)
		}
		return &SetupCommandResult{ExitCode: 1}, errors.New(strings.Join(validationErrors, ","))
	}

	os.MkdirAll(answers.Workspace, 0755)
	os.MkdirAll(filepath.Dir(answers.ConfigPath), 0755)
	os.MkdirAll(config.Memory.StoragePath, 0755)

	if err := core.GatewayConfigFileInstance.Save(config, answers.ConfigPath); err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return &SetupCommandResult{ExitCode: 2}, err
	}

	envExamplePath, err := core.GatewaySetupArtifactsInstance.BuildEnvExamplePath(answers.ConfigPath)
	if err != nil {
		return &SetupCommandResult{ExitCode: 2}, err
	}

	data := core.GatewaySetupArtifactsInstance.BuildEnvExample(answers.ApiKey, answers.AuthToken, answers.Workspace, buildReachableBaseUrl(answers.BindAddress, answers.Port))
	if err := os.WriteFile(envExamplePath, []byte(data), 0644); err != nil {
		return &SetupCommandResult{ExitCode: 2}, err
	}

	fmt.Fprintf(output, "Wrote config: %s\n", answers.ConfigPath)
	fmt.Fprintf(output, "Wrote env example: %s\n", envExamplePath)
	fmt.Fprintf(output, "Profile: %s\n", answers.Profile)
	fmt.Fprintf(output, "Workspace: %s\n", answers.Workspace)
	fmt.Fprintf(output, "Provider/model: %s/%s\n", config.Llm.Provider, config.Llm.Model)
	fmt.Fprintf(output, "Bind: %s/%d\n", config.BindAddress, config.Port)
	fmt.Fprintln(output, "Config validation: passed")

	if len(warnings) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Warnings:")
		for _, warning := range warnings {
			fmt.Fprintf(output, "- %s\n", warning)
		}
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Next steps:")
	fmt.Fprintf(output, "openclaw setup verify --config %s\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(answers.ConfigPath))
	fmt.Fprintf(output, "openclaw setup launch --config %s\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(answers.ConfigPath))
	fmt.Fprintf(output, "dotnet run --project src/OpenClaw.Gateway -c Release -- --config %s --doctor\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(answers.ConfigPath))

	if util.IsLoopbackBind(answers.BindAddress) {
		fmt.Fprintln(output, "dotnet run --project src/OpenClaw.Companion -c Release")
	} else {
		fmt.Fprintln(output, "Companion/public launch guidance: set OPENCLAW_BASE_URL and OPENCLAW_AUTH_TOKEN from the generated config or env file before starting the Companion app.")
	}
	return &SetupCommandResult{
		ExitCode:   0,
		ConfigPath: answers.ConfigPath,
	}, nil
}

func setupCommandRunTailscaleSetup(args []string, input io.Reader, output, errwirter io.Writer, canPrompt bool) (int, error) {
	helpRequested := len(args) > 0 && (args[0] == "-h" || args[0] == "--help")
	if len(args) == 0 || helpRequested || args[0] != "serve" {
		msg := "Usage: openclaw setup tailscale serve [--config <path>] [--local-url <url>] [--non-interactive]"
		if helpRequested {
			fmt.Fprintln(output, msg)
			return 0, nil
		} else {
			fmt.Fprintln(errwirter, msg)
			return 2, errors.New(msg)
		}
	}

	parsed := CliArgsParse(args)
	if parsed == nil {
		fmt.Fprintln(errwirter, "argument error")
		return 2, errors.New("argument error")
	}

	if parsed.ShowHelp {
		fmt.Fprintln(output, "Usage: openclaw setup tailscale serve [--config <path>] [--local-url <url>] [--non-interactive]")
		return 0, nil
	}

	explicitConfig := util.Deref(parsed.GetOption("--config")) != ""
	configPath, err := filepath.Abs(core.GatewaySetupPathsIntance.ExpandPath(getOptionOr(parsed, "--config", DefaultConfigPath)))
	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 2, err
	}

	var config *core.GatewayConfig
	configLoaded := false

	if util.FileExists(configPath) {
		config, err = core.GatewayConfigFileInstance.Load(configPath)
		if err != nil {
			fmt.Fprintf(errwirter, "Could not load config %s: %s", configPath, err.Error())
			return 1, err
		}
		configLoaded = true
	} else if explicitConfig {
		msg := fmt.Sprintf("Could not load config %s\n", configPath)
		fmt.Fprintln(errwirter, msg)
		return 1, errors.New(msg)
	} else {
		config = createTailscaleServePreviewConfig("http://127.0.0.1:18789")
	}

	localGatewayUrl := util.Deref(parsed.GetOption("--local-url"))
	if localGatewayUrl == "" {
		localGatewayUrl = util.Deref(parsed.GetOption("--url"))
	}
	if localGatewayUrl == "" {
		localGatewayUrl = core.TailscaleServeAdvisorInstance.BuildLocalGatewayUrl(config)
	}
	if !parsed.HasFlag("--non-interactive") && canPrompt {
		localGatewayUrl = prompt(output, input, "Local gateway URL", localGatewayUrl)
	}

	localGatewayUri, err := url.Parse(localGatewayUrl)
	if err != nil || (localGatewayUri != nil && (!localGatewayUri.IsAbs() || (localGatewayUri.Scheme != "http" && localGatewayUri.Scheme != "https"))) {
		msg := fmt.Sprintf("Invalid local gateway URL: %s", localGatewayUrl)
		fmt.Fprintln(errwirter, msg)
		return 2, errors.New(msg)
	}

	config.Deployment = &core.DeploymentConfig{
		Mode:             "tailscale-serve",
		ReverseProxy:     "tailscale-serve",
		ExpectedLocalUrl: strings.TrimRight(localGatewayUri.String(), "/"),
	}

	status, err := core.TailscaleServeAdvisorInstance.BuildStatus(context.Background(), config, &core.TailscaleServeProbeOptions{ForceInclude: true, CheckCli: true})
	if err != nil {
		msg := "Expected Tailscale Serve status."
		fmt.Fprintln(errwirter, msg)
		return 2, errors.New(msg)
	}

	writeTailscaleServeInstructions(output, configPath, configLoaded, status)

	return 0, nil
}

func writeTailscaleServeInstructions(output io.Writer, configPath string, configLoaded bool, status *core.TailscaleServeStatusResponse) {
	localGateway := strings.TrimRight(status.LocalGatewayUrl, "/")

	fmt.Fprintln(output, "Tailscale Serve setup for OpenClaw.NET")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "This profile keeps OpenClaw.NET bound to 127.0.0.1 and uses Tailscale Serve to expose it privately inside your tailnet.")

	if configLoaded {
		fmt.Fprintf(output, "Config: %s\n", configPath)
	} else {
		fmt.Fprintf(output, "Config: %s (not found; using default local gateway guidance)\n", configPath)
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Local gateway:")
	fmt.Fprintln(output, localGateway)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Recommended command:")
	fmt.Fprintln(output)
	fmt.Fprintln(output, status.SuggestedServeCommand)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "After enabling Serve, open the Tailscale-provided HTTPS URL from a device in your tailnet.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Useful OpenClaw.NET surfaces:")
	fmt.Fprintln(output, "- Chat: /chat")
	fmt.Fprintln(output, "- Admin: /admin")
	fmt.Fprintln(output, "- MCP: /mcp")
	fmt.Fprintln(output, "- Integration API: /api/integration/status")
	fmt.Fprintln(output, "- WebSocket: /ws")
	fmt.Fprintln(output, "- Doctor: /doctor/text")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Tailscale CLI status:")
	fmt.Fprintf(output, "- CLI detected: %s\n", strings.ToLower(strconv.FormatBool(status.TailscaleCliDetected)))
	fmt.Fprintf(output, "- Tailnet reachability: %v\n", status.TailnetReachability)
	fmt.Fprintf(output, "- Serve detected: %v\n", status.ServeDetected)

	if len(status.Warnings) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Advisory warnings:")
		for _, warning := range status.Warnings {
			fmt.Fprintf(output, "- %s\n", warning)
		}
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Security notes:")
	fmt.Fprintln(output, "- Tailscale Serve is for private tailnet access.")
	fmt.Fprintln(output, "- Do not use Funnel for /admin unless you have reviewed public exposure hardening.")
	fmt.Fprintln(output, "- Keep OpenClaw.NET auth enabled.")
	fmt.Fprintln(output, "- Use operator accounts/tokens for admin, API, and websocket clients.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Follow-up checks:")
	fmt.Fprintf(output, "openclaw setup status --config %s\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(configPath))
	fmt.Fprintf(output, "OPENCLAW_BASE_URL=%s OPENCLAW_AUTH_TOKEN=... openclaw admin posture\n", localGateway)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "If the gateway is running, check:")
	fmt.Fprintf(output, "%s/health/ready\n", localGateway)
}

func createTailscaleServePreviewConfig(localGatewayUrl string) *core.GatewayConfig {
	var port = 18789
	baseURL, err := url.Parse(localGatewayUrl)
	if err == nil && baseURL.IsAbs() {
		p, err := strconv.Atoi(baseURL.Port())
		if err == nil && p > 0 {
			port = p
		}
	}
	return &core.GatewayConfig{
		BindAddress: "127.0.0.1",
		Port:        port,
		Deployment: &core.DeploymentConfig{
			Mode:             "tailscale-serve",
			ReverseProxy:     "tailscale-serve",
			ExpectedLocalUrl: strings.TrimRight(localGatewayUrl, "/"),
		},
	}
}

func normalizeApertureAuthMode(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized != "bearer" && normalized != "tailnet-identity" {
		return "", errors.New("Aperture auth mode must be one of: bearer, tailnet-identity.")
	}
	return normalized, nil
}

func parseBooleanOption(raw string, defaultValue bool) (bool, error) {
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.ParseBool(raw)
}

func setupCommandRunProviderSetup(args []string, input io.Reader, output io.Writer, errwirter io.Writer, currentDirectory string, canPrompt bool) (int, error) {
	providerIndex := slices.IndexFunc(args, func(arg string) bool {
		return !strings.HasPrefix(arg, "--")
	})

	provider := ""
	if providerIndex >= 0 {
		provider = args[providerIndex]
	}

	if provider != "aperture" {
		msg := "Usage: openclaw setup provider aperture [--config <path>] [--profile-id <id>] [--endpoint <url>] [--model <route>] [--auth-mode <bearer|tailnet-identity>] [--env-var <name>] [--send-request-metadata <true|false>] [--workspace <path>] [--non-interactive]"
		fmt.Fprintln(errwirter, msg)
		return 2, errors.New(msg)
	}

	filteredArgs := []string{}
	for i := range args {
		if i != providerIndex {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	parsed := CliArgsParse(filteredArgs)
	if parsed == nil {
		fmt.Fprintln(errwirter, "argument error")
		return 2, errors.New("argument error")
	}

	configpath := getOptionOr(parsed, "--config", DefaultConfigPath)
	configPath, err := filepath.Abs(core.GatewaySetupPathsIntance.ExpandPath(configpath))
	if err != nil {
		return 1, err
	}

	nonInteractive := parsed.HasFlag("--non-interactive")
	profileId := getOptionOr(parsed, "--profile-id", "aperture-default")
	endpoint := util.Deref(parsed.GetOption("--endpoint"))
	model := util.Deref(parsed.GetOption("--model"))

	authMode, err := normalizeApertureAuthMode(getOptionOr(parsed, "--auth-mode", "bearer"))
	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 2, err
	}

	envVar := getOptionOr(parsed, "--env-var", "OPENCLAW_APERTURE_TOKEN")
	sendMetadata, err := parseBooleanOption(getOptionOr(parsed, "--send-request-metadata", ""), false)
	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 2, err
	}

	if !nonInteractive && canPrompt {
		if endpoint == "" {
			endpoint = "https://YOUR_APERTURE_ENDPOINT"
		}
		if model == "" {
			model = "YOUR_APERTURE_MODEL_ROUTE"
		}
		endpoint = prompt(output, input, "Aperture endpoint", endpoint)
		model = prompt(output, input, "Aperture model route", model)

		authMode, err = normalizeApertureAuthMode(prompt(output, input, "Auth mode (bearer|tailnet-identity)", authMode))
		if err != nil {
			fmt.Fprintln(errwirter, err.Error())
			return 2, err
		}

		if authMode == "bearer" {
			envVar = prompt(output, input, "Bearer token env var", envVar)
		}

		sendMetadataStr := "false"
		if sendMetadata {
			sendMetadataStr = "true"
		}
		sendMetadata, err = parseBooleanOption(prompt(output, input, "Send request metadata (true|false)", sendMetadataStr), false)

		if err != nil {
			fmt.Fprintln(errwirter, err.Error())
			return 2, err
		}
	}

	if endpoint == "" || model == "" {
		msg := "Aperture endpoint and model route are required. Re-run with --endpoint and --model, or run from an interactive terminal."
		fmt.Fprintln(errwirter, msg)
		return 2, errors.New(msg)
	}

	apiKey := ""
	if authMode == "bearer" {
		apiKey = fmt.Sprintf("env:%s", envVar)
	}

	var config *core.GatewayConfig
	if util.FileExists(configPath) {
		config, err = core.GatewayConfigFileInstance.Load(configPath)
		if err != nil {
			fmt.Fprintln(errwirter, err.Error())
			return 2, err
		}
	} else {
		workspace, err := filepath.Abs(core.GatewaySetupPathsIntance.ExpandPath(getOptionOr(parsed, "--workspace", filepath.Join(currentDirectory, "workspace"))))
		if err != nil {
			fmt.Fprintln(errwirter, err.Error())
			return 2, err
		}

		configDirectory := filepath.Dir(configPath)
		if configDirectory == "" {
			return 2, errors.New("Config path must contain a directory.")
		}

		warnings := []string{}
		f := core.GatewaySetupProfileFactory{}

		config = f.CreateProfileConfig("local", "127.0.0.1", 18789, generateAuthToken(), workspace, filepath.Join(configDirectory, "memory"), "aperture", model, apiKey, "", &warnings)
		config.Llm.Endpoint = endpoint
		config.Llm.AuthMode = authMode
		config.Llm.SendRequestMetadata = sendMetadata
	}

	existingIndex := slices.IndexFunc(config.Models.Profiles, func(item core.ModelProfileConfig) bool {
		return item.Id == profileId
	})

	profile := core.ModelProfileConfig{
		Id:                  profileId,
		Provider:            "aperture",
		Model:               model,
		BaseUrl:             endpoint,
		ApiKey:              apiKey,
		AuthMode:            authMode,
		SendRequestMetadata: new(sendMetadata),
		Tags:                []string{"aperture", "remote", "optional"},
	}

	if existingIndex >= 0 {
		config.Models.Profiles[existingIndex] = profile
	} else {
		config.Models.Profiles = append(config.Models.Profiles, profile)
	}

	if config.Models.DefaultProfile == "" && len(config.Models.Profiles) == 1 {
		config.Models.DefaultProfile = profileId
	}

	validationErrors := core.ConfigValidatorInstance.Validate(config)
	if len(validationErrors) > 0 {
		fmt.Fprintln(errwirter, "Config validation failed:")
		for _, validationError := range validationErrors {
			fmt.Fprintf(errwirter, "- %s\n", validationError)
		}
		return 1, errors.New("Config validation failed:")
	}

	resolvedWorkspaceRoot := resolveConfiguredPath(config.Tooling.WorkspaceRoot)
	resolvedMemoryStoragePath := resolveConfiguredPath(config.Memory.StoragePath)

	os.MkdirAll(filepath.Dir(configPath), 0755)
	if resolvedWorkspaceRoot != "" {
		os.MkdirAll(resolvedWorkspaceRoot, 0755)
	}
	if resolvedMemoryStoragePath != "" {
		os.MkdirAll(resolvedMemoryStoragePath, 0755)
	}
	if err := core.GatewayConfigFileInstance.Save(config, configPath); err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 2, err
	}

	envExamplePath, err := core.GatewaySetupArtifactsInstance.BuildEnvExamplePath(configPath)
	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 2, err
	}

	authToken := config.AuthToken
	if authToken == "" {
		authToken = generateAuthToken()
	}
	workspace := resolvedWorkspaceRoot
	if workspace == "" {
		workspace = filepath.Join(currentDirectory, "workspace")
	}
	data := core.GatewaySetupArtifactsInstance.BuildEnvExample(apiKey, authToken, workspace, buildReachableBaseUrl(config.BindAddress, config.Port))
	if err := os.WriteFile(envExamplePath, []byte(data), 0644); err != nil {
		return 2, err
	}

	fmt.Fprintf(output, "Wrote config: %s\n", configPath)
	fmt.Fprintf(output, "Wrote env example: %s\n", envExamplePath)
	fmt.Fprintf(output, "Aperture profile: %s\n", profileId)
	fmt.Fprintf(output, "Endpoint: %s\n", endpoint)
	fmt.Fprintf(output, "Model route: %s\n", model)
	fmt.Fprintf(output, "Auth mode: %s\n", authMode)
	if sendMetadata {
		fmt.Fprintf(output, "Request metadata: %s\n", "enabled")
	} else {
		fmt.Fprintf(output, "Request metadata: %s\n", "disabled")
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Next steps:")
	fmt.Fprintf(output, "openclaw setup verify --config %s --offline\n", core.GatewaySetupPathsIntance.QuoteIfNeeded(configPath))
	fmt.Fprintln(output, "openclaw models doctor")

	return 0, nil
}

func resolveConfiguredPath(value string) string {
	if value == "" {
		return ""
	}

	resolved := core.SecretResolverInstance.Resolve(value)
	if resolved == "" {
		if strings.HasPrefix(strings.TrimLeft(value, "\n\r"), "env:") {
			return ""
		}
		resolved = value
	}
	resolved = core.GatewaySetupPathsIntance.ExpandPath(resolved)
	if filepath.IsAbs(resolved) {
		return resolved
	}

	resolved, _ = filepath.Abs(resolved)
	return resolved
}

func setupCommandBuildConfig(answers *SetupAnswers) (*core.GatewayConfig, []string, error) {
	warnings := []string{}
	configDirectory := filepath.Dir(answers.ConfigPath)
	if configDirectory == "" {
		return nil, nil, errors.New("Config path must contain a directory.")
	}

	memoryRoot := filepath.Join(configDirectory, "memory")
	f := core.GatewaySetupProfileFactory{}
	var config = f.CreateProfileConfig(
		answers.Profile,
		answers.BindAddress,
		answers.Port,
		answers.AuthToken,
		answers.Workspace,
		memoryRoot,
		answers.Provider,
		answers.Model,
		answers.ApiKey,
		answers.ModelPresetId,
		&warnings)

	if err := setupCommandApplyBackend(config, answers, &warnings); err != nil {
		return nil, nil, err
	}

	return config, warnings, nil
}

func setupCommandApplyBackend(config *core.GatewayConfig, answers *SetupAnswers, warnings *[]string) error {
	switch answers.BackendChoice {
	case "none":
	case "docker":
		if answers.DockerImage == "" {
			return errors.New("--docker-image is required when docker is selected.")
		}

		config.Execution.Profiles["docker"] = core.ExecutionBackendProfileConfig{
			Type:             core.ExecutionBackendTypeDocker,
			Image:            answers.DockerImage,
			WorkingDirectory: answers.Workspace,
		}
		if config.Tooling.AllowShell {
			config.Execution.Tools["shell"] = core.ExecutionToolRouteConfig{Backend: "docker", FallbackBackend: "local", RequireWorkspace: true}
		}

		*warnings = append(*warnings, checkCommandAvailability("docker", "--version", "Docker backend requested but docker was not found on PATH.")...)

		if !config.Tooling.AllowShell {
			*warnings = append(*warnings, "Public profile keeps shell disabled even though a Docker backend was configured. Enable shell deliberately later if you want agent tool execution routed to Docker.")
		}
	case "opensandbox":
		if answers.OpenSandboxEndpoint == "" {
			return errors.New("--opensandbox-endpoint is required when opensandbox is selected.")
		}

		baseURL, err := url.Parse(answers.OpenSandboxEndpoint)
		if err != nil || (baseURL != nil && (!baseURL.IsAbs())) {
			return fmt.Errorf("Invalid OpenSandbox endpoint:  %s", answers.OpenSandboxEndpoint)
		}

		config.Tooling.EnableBrowserTool = true
		config.Sandbox.Provider = core.SandboxProviderNamesOpenSandbox
		config.Sandbox.Endpoint = answers.OpenSandboxEndpoint
		config.Execution.Profiles["opensandbox"] = core.ExecutionBackendProfileConfig{
			Type:     core.ExecutionBackendTypeOpenSandbox,
			Endpoint: answers.OpenSandboxEndpoint,
		}
		*warnings = append(*warnings, "OpenSandbox backend was configured. You still need to define sandbox tool templates before the gateway can sandbox tool execution.")
	case "ssh":
		if answers.SshHost == "" {
			return errors.New("--ssh-host is required when ssh is selected.")
		}

		if answers.SshUser == "" {
			return errors.New("--ssh-user is required when ssh is selected.")
		}

		config.Execution.Profiles["ssh"] = core.ExecutionBackendProfileConfig{
			Type:             core.ExecutionBackendTypeSsh,
			Host:             answers.SshHost,
			Username:         answers.SshUser,
			PrivateKeyPath:   answers.SshKey,
			WorkingDirectory: answers.Workspace,
		}
		if config.Tooling.AllowShell {
			config.Execution.Tools["shell"] = core.ExecutionToolRouteConfig{Backend: "ssh", FallbackBackend: "local", RequireWorkspace: true}
		}

		*warnings = append(*warnings, checkCommandAvailability("ssh", "-V", "SSH backend requested but ssh was not found on PATH.")...)
		if !config.Tooling.AllowShell {
			*warnings = append(*warnings, "Public profile keeps shell disabled even though an SSH backend was configured. Enable shell deliberately later if you want agent tool execution routed to SSH.")
		}

	default:
		return fmt.Errorf("Unsupported execution backend: %s", answers.BackendChoice)
	}

	return nil
}

func checkCommandAvailability(command, arg, failureMessage string) []string {
	result := util.RunProcess(context.Background(), command, []string{arg}, "", 0, 0, 0)
	if result.Error != "" {
		return []string{failureMessage}
	}
	return []string{}
}

func setupCommandBuildAnswersFromArgs(parsed *CliArgs, currentDirectory string, requireProfile bool) (*SetupAnswers, error) {
	rawProfile := util.Deref(parsed.GetOption("--profile"))
	if requireProfile && rawProfile == "" {
		return nil, errors.New("--profile is required when --non-interactive is set.")
	}

	if rawProfile == "" {
		rawProfile = "local"
	}
	f := core.GatewaySetupProfileFactory{}
	profile, err := f.NormalizeProfile(rawProfile)
	if err != nil {
		return nil, err
	}
	backendChoice := inferBackendChoice(parsed)
	if backendChoice == "" {
		backendChoice = DefaultBackendChoice
	}
	backendChoice, err = normalizeBackendChoice(backendChoice)
	if err != nil {
		return nil, err
	}

	configPath := util.Deref(parsed.GetOption("--config"))
	if configPath == "" {
		configPath = DefaultConfigPath
	}
	configPath = core.GatewaySetupPathsIntance.ExpandPath(configPath)
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}

	workspace := util.Deref(parsed.GetOption("--workspace"))
	if workspace == "" {
		workspace = filepath.Join(currentDirectory, "workspace")
	}
	workspace = core.GatewaySetupPathsIntance.ExpandPath(workspace)
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}

	provider := util.Deref(parsed.GetOption("--provider"))
	if provider == "" {
		provider = DefaultProvider
	}
	model := util.Deref(parsed.GetOption("--model"))
	if model == "" {
		model = getDefaultModel(provider, nil)
	}
	apiKey := util.Deref(parsed.GetOption("--api-key"))
	if providerRequiresApiKey(provider) && apiKey == "" {
		apiKey = getDefaultApiKeyRef(provider)
	}
	bindAddress := util.Deref(parsed.GetOption("--bind"))
	if bindAddress == "" {
		bindAddress = getDefaultBindAddress(profile)
	}
	port := util.Deref(parsed.GetOption("--port"))
	if port == "" {
		port = "18789"
	}
	p, _ := strconv.Atoi(port)

	authToken := util.Deref(parsed.GetOption("--auth-token"))
	if authToken == "" {
		authToken = generateAuthToken()
	}
	return &SetupAnswers{
		Profile:             profile,
		ConfigPath:          configPath,
		Workspace:           workspace,
		Provider:            provider,
		ModelPresetId:       getDefaultModelPreset(provider, util.Deref(parsed.GetOption("--model-preset"))),
		Model:               model,
		ApiKey:              apiKey,
		DockerImage:         util.Deref(parsed.GetOption("--docker-image")),
		OpenSandboxEndpoint: util.Deref(parsed.GetOption("--opensandbox-endpoint")),
		SshHost:             util.Deref(parsed.GetOption("--ssh-host")),
		SshUser:             util.Deref(parsed.GetOption("--ssh-user")),
		SshKey:              util.Deref(parsed.GetOption("--ssh-key")),
		BindAddress:         bindAddress,
		Port:                p,
		AuthToken:           authToken,
		BackendChoice:       backendChoice,
	}, nil
}

func normalizeBackendChoice(backendChoice string) (string, error) {
	backendChoice = strings.ToLower(strings.TrimSpace(backendChoice))
	if backendChoice != "none" && backendChoice != "docker" && backendChoice != "opensandbox" && backendChoice != "ssh" {
		return "", errors.New("Execution backend must be one of: none, docker, opensandbox, ssh.")
	}
	return backendChoice, nil
}

func inferBackendChoice(parsed *CliArgs) string {
	if util.Deref(parsed.GetOption("--docker-image")) != "" {
		return "docker"
	}
	if util.Deref(parsed.GetOption("--opensandbox-endpoint")) != "" {
		return "opensandbox"
	}
	if util.Deref(parsed.GetOption("--ssh-host")) != "" {
		return "ssh"
	}
	return DefaultBackendChoice
}

func generateAuthToken() string {
	return "oc_" + util.CleanUUID()
}

func getDefaultBindAddress(profile string) string {
	if profile == "public" {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

func getDefaultApiKeyRef(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == core.DeepSeekProviderDefaultsProviderId {
		return core.DeepSeekProviderDefaultsDefaultApiKeyRef
	}
	return DefaultApiKeyRef
}

func getDefaultModel(provider string, discoveredOllamaModels []string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case core.DeepSeekProviderDefaultsProviderId:
		return core.DeepSeekProviderDefaultsDefaultModel
	case "embedded":
		return DefaultEmbeddedModelId
	case "ollama":
		if len(discoveredOllamaModels) > 0 {
			return discoveredOllamaModels[0]
		}
		return core.DefaultGatewayConfig().Llm.Model
	}
	return core.DefaultGatewayConfig().Llm.Model
}

func getDefaultModelPreset(provider string, configured string) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return configured
	}

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama":
		return DefaultOllamaPresetId
	case "embedded":
		return DefaultEmbeddedPresetId
	}
	return ""
}

func prompt(output io.Writer, input io.Reader, label, defaultValue string) string {
	fmt.Fprintf(output, "%s [%s]\n", label, defaultValue)
	reader := bufio.NewReader(input)
	line, err := reader.ReadString('\n')

	if err != nil && err != io.EOF {
		return defaultValue
	}

	trimmed := strings.TrimSpace(line)

	if trimmed == "" {
		return defaultValue
	}

	return trimmed
}

func setupCommandPromptForAnswers(parsed *CliArgs, input io.Reader, output io.Writer, currentDirectory string) (*SetupAnswers, error) {
	f := core.GatewaySetupProfileFactory{}
	profile, err := f.NormalizeProfile(prompt(output, input, "Deployment profile (local|public|tailscale-serve)", getOptionOr(parsed, "--profile", "local")))
	if err != nil {
		return nil, err
	}

	inferredBackend := inferBackendChoice(parsed)
	if inferredBackend == "" {
		inferredBackend = DefaultBackendChoice
	}
	backendChoice, err := normalizeBackendChoice(prompt(output, input, "Execution backend (none|docker|opensandbox|ssh)", inferredBackend))
	if err != nil {
		return nil, err
	}

	configPathRaw := core.GatewaySetupPathsIntance.ExpandPath(prompt(output, input, "Config path", getOptionOr(parsed, "--config", DefaultConfigPath)))
	configPath, _ := filepath.Abs(configPathRaw)

	workspaceRaw := core.GatewaySetupPathsIntance.ExpandPath(prompt(output, input, "Workspace path", getOptionOr(parsed, "--workspace", filepath.Join(currentDirectory, "workspace"))))
	workspace, _ := filepath.Abs(workspaceRaw)

	discoveredOllama := tryDiscoverOllamaModels()
	if len(discoveredOllama.Models) > 0 {
		fmt.Fprintf(output, "Detected Ollama models: %s\n", strings.Join(discoveredOllama.Models, ", "))
	}

	providerDefault := getOptionOr(parsed, "--provider", func() string {
		if discoveredOllama.IsAvailable {
			return "ollama"
		}
		return DefaultProvider
	}())

	provider := prompt(output, input, "Provider", providerDefault)
	modelPresetId := getDefaultModelPreset(provider, util.Deref(parsed.GetOption("--model-preset")))

	if requiresModelPresetPrompt(provider) {
		modelPresetId = prompt(output, input, "Model preset", modelPresetId)
	}

	modelDefault := getOptionOr(parsed, "--model", getDefaultModel(provider, discoveredOllama.Models))
	model := prompt(output, input, "Model", modelDefault)

	var apiKey string
	if providerRequiresApiKey(provider) {
		apiKey = prompt(output, input, "API key or env: reference", getOptionOr(parsed, "--api-key", getDefaultApiKeyRef(provider)))
	} else {
		apiKey = util.Deref(parsed.GetOption("--api-key"))
	}

	bindDefault := getOptionOr(parsed, "--bind", getDefaultBindAddress(profile))
	bindAddress := prompt(output, input, "Bind address", bindDefault)

	portDefault := getOptionOr(parsed, "--port", "18789")
	port := parsePort(prompt(output, input, "Port", portDefault))

	authDefault := getOptionOr(parsed, "--auth-token", generateAuthToken())
	authToken := prompt(output, input, "Auth token", authDefault)

	dockerImage := util.Deref(parsed.GetOption("--docker-image"))
	opensandboxEndpoint := util.Deref(parsed.GetOption("--opensandbox-endpoint"))
	sshHost := util.Deref(parsed.GetOption("--ssh-host"))
	sshUser := util.Deref(parsed.GetOption("--ssh-user"))
	sshKey := util.Deref(parsed.GetOption("--ssh-key"))

	switch backendChoice {
	case "docker":
		defVal := dockerImage
		if defVal == "" {
			defVal = "python:3.12-slim"
		}
		dockerImage = prompt(output, input, "Docker image", defVal)
	case "opensandbox":
		defVal := opensandboxEndpoint
		if defVal == "" {
			defVal = "http://127.0.0.1:8080"
		}
		opensandboxEndpoint = prompt(output, input, "OpenSandbox endpoint", defVal)
	case "ssh":
		defValHost := sshHost
		if defValHost == "" {
			defValHost = "remote-host"
		}
		sshHost = prompt(output, input, "SSH host", defValHost)

		defValUser := sshUser
		if defValUser == "" {
			defValUser = getCurrentUserName()
		}
		sshUser = prompt(output, input, "SSH user", defValUser)
		sshKey = promptOptional(output, input, "SSH private key path", sshKey)
	}

	return &SetupAnswers{
		Profile:             profile,
		ConfigPath:          configPath,
		Workspace:           workspace,
		Provider:            provider,
		ModelPresetId:       modelPresetId,
		Model:               model,
		ApiKey:              apiKey,
		BindAddress:         bindAddress,
		Port:                port,
		AuthToken:           authToken,
		BackendChoice:       backendChoice,
		DockerImage:         dockerImage,
		OpenSandboxEndpoint: opensandboxEndpoint,
		SshHost:             sshHost,
		SshUser:             sshUser,
		SshKey:              sshKey,
	}, nil
}

func promptOptional(output io.Writer, input io.Reader, label, defaultValue string) string {
	suffix := ""
	if defaultValue != "" {
		suffix = fmt.Sprintf(" [%s]", defaultValue)
	}

	fmt.Fprintf(output, "%s%s: ", label, suffix)
	reader := bufio.NewReader(input)
	line, err := reader.ReadString('\n')

	if err != nil && err != io.EOF {
		return defaultValue
	}

	trimmed := strings.TrimSpace(line)

	if trimmed == "" {
		return defaultValue
	}

	return trimmed
}

func getCurrentUserName() string {
	u, _ := user.Current()
	if u != nil {
		return u.Username
	}
	return ""
}

func parsePort(s string) int {
	a, _ := strconv.Atoi(s)
	return a
}

func requiresModelPresetPrompt(provider string) bool {
	return provider == "ollama" || provider == "embedded"
}

func tryDiscoverOllamaModels() OllamaDiscoveryResult {
	emptyResult := OllamaDiscoveryResult{
		IsAvailable: false,
		Models:      []string{},
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get("http://127.0.0.1:11434/api/tags")
	if err != nil {
		return emptyResult
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return emptyResult
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return emptyResult
	}

	var models []string
	seen := make(map[string]bool)

	for _, m := range payload.Models {
		name := strings.TrimSpace(m.Name)
		if name != "" {
			lowerName := strings.ToLower(name)
			if !seen[lowerName] {
				seen[lowerName] = true
				models = append(models, name)
			}
		}
	}

	if len(models) == 0 {
		return OllamaDiscoveryResult{
			IsAvailable: true,
			Models:      []string{},
		}
	}

	sort.Slice(models, func(i, j int) bool {
		return strings.ToLower(models[i]) < strings.ToLower(models[j])
	})

	return OllamaDiscoveryResult{
		IsAvailable: true,
		Models:      models,
	}
}

func getOptionOr(parsed *CliArgs, name, defaultvalue string) string {
	value := util.Deref(parsed.GetOption(name))
	if value != "" {
		return value
	}
	return defaultvalue
}

func buildReachableBaseUrl(bindAddress string, port int) string {
	return core.GatewaySetupArtifactsInstance.BuildReachableBaseUrl(bindAddress, port)
}
