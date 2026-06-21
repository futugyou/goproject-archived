package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type GatewayConfigFile struct{}

func (g GatewayConfigFile) Load(configPath string) (*GatewayConfig, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		OpenClaw *GatewayConfig `json:"OpenClaw"`
	}

	// 如果 JSON 带有 {"OpenClaw": {...}} 结构，它会被正确解析到 wrapper.OpenClaw 中
	if err := json.Unmarshal(data, &wrapper); err == nil && wrapper.OpenClaw != nil {
		return wrapper.OpenClaw, nil
	}

	// 如果没有 "OpenClaw" 节点，则说明整个根目录就是 GatewayConfig 本身
	var config GatewayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("could not deserialize gateway config from %s: %w", configPath, err)
	}

	return &config, nil
}

func (g GatewayConfigFile) Save(config *GatewayConfig, configPath string) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	wrapper := struct {
		OpenClaw *GatewayConfig `json:"OpenClaw"`
	}{
		OpenClaw: config,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize gateway config: %w", err)
	}

	directory := filepath.Dir(configPath)
	if err := os.MkdirAll(directory, os.ModePerm); err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	return nil
}

type GatewaySetupArtifacts struct{}

// BuildEnvExample 生成环境示例文件内容
func (g *GatewaySetupArtifacts) BuildEnvExample(apiKeyRef *string, authToken, workspacePath, baseUrl string) string {
	var lines []string

	// 处理可选的 apiKeyRef
	if apiKeyRef != nil && strings.TrimSpace(*apiKeyRef) != "" {
		resolvedKey := g.ResolveProviderEnvVariable(*apiKeyRef)
		lines = append(lines, fmt.Sprintf("%s=replace-me", resolvedKey))
	}

	lines = append(lines, fmt.Sprintf("OPENCLAW_AUTH_TOKEN=%s", authToken))
	lines = append(lines, fmt.Sprintf("OPENCLAW_BASE_URL=%s", baseUrl))
	lines = append(lines, fmt.Sprintf("OPENCLAW_WORKSPACE=%s", workspacePath))

	// 拼接每一行，并在末尾加上换行符
	return strings.Join(lines, "\n") + "\n"
}

// ResolveProviderEnvVariable 解析服务商环境变量名
func (g *GatewaySetupArtifacts) ResolveProviderEnvVariable(apiKeyRef string) string {
	if len(apiKeyRef) > 4 && strings.HasPrefix(strings.ToLower(apiKeyRef), "env:") {
		return apiKeyRef[4:]
	}
	return "MODEL_PROVIDER_KEY"
}

// BuildEnvExamplePath 根据配置路径生成 .env.example 路径
func (g *GatewaySetupArtifacts) BuildEnvExamplePath(configPath string) (string, error) {
	directory := filepath.Dir(configPath)
	//filepath.Dir 对于没有目录的路径会返回 "."
	if directory == "" || directory == "." && !strings.Contains(configPath, string(filepath.Separator)) {
		return "", fmt.Errorf("config path must contain a directory")
	}

	filename := filepath.Base(configPath)
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)

	return filepath.Join(directory, fmt.Sprintf("%s.env.example", stem)), nil
}

// BuildReachableBaseUrl 构建可达的 Base URL
func (g *GatewaySetupArtifacts) BuildReachableBaseUrl(bindAddress string, port int) string {
	portStr := strconv.Itoa(port)

	if bindAddress == "0.0.0.0" || bindAddress == "::" || bindAddress == "[::]" {
		return fmt.Sprintf("http://127.0.0.1:%s", portStr)
	}

	// 针对 IPv6 地址的处理
	if strings.Contains(bindAddress, ":") && !strings.HasPrefix(bindAddress, "[") {
		return fmt.Sprintf("http://[%s]:%s", bindAddress, portStr)
	}

	return fmt.Sprintf("http://%s:%s", bindAddress, portStr)
}

type GatewaySetupPaths struct{}

const (
	DefaultConfigPath              = "~/.openclaw/config/openclaw.settings.json"
	DefaultLocalStartupStatePath   = "~/.openclaw/state/local-startup.json"
	DefaultUpgradeSnapshotRootPath = "~/.openclaw/state/upgrade-snapshots"
)

func (g *GatewaySetupPaths) ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		if len(path) == 1 {
			return home
		}
		return filepath.Join(home, path[2:])
	}

	return path
}

func (g *GatewaySetupPaths) QuoteIfNeeded(path string) string {
	if strings.Contains(path, " ") {
		return "\"" + path + "\""
	}
	return path
}

func (g *GatewaySetupPaths) ResolveDefaultConfigPath() string {
	abs, err := filepath.Abs(g.ExpandPath(DefaultConfigPath))
	if err != nil {
		return g.ExpandPath(DefaultConfigPath)
	}
	return abs
}

func (g *GatewaySetupPaths) ResolveDefaultLocalStartupStatePath() string {
	abs, err := filepath.Abs(g.ExpandPath(DefaultLocalStartupStatePath))
	if err != nil {
		return g.ExpandPath(DefaultLocalStartupStatePath)
	}
	return abs
}

func (g *GatewaySetupPaths) ResolveDefaultUpgradeSnapshotRootPath() string {
	abs, err := filepath.Abs(g.ExpandPath(DefaultUpgradeSnapshotRootPath))
	if err != nil {
		return g.ExpandPath(DefaultUpgradeSnapshotRootPath)
	}
	return abs
}

type GatewaySetupProfileFactory struct{}

func (g *GatewaySetupProfileFactory) CreateProfileConfig(
	profile string,
	bindAddress string,
	port int,
	authToken string,
	workspacePath string,
	memoryPath string,
	provider string,
	model string,
	apiKey string,
	modelPresetId *string,
	warnings *[]string,
) *GatewayConfig {

	normalizedProfile, err := g.NormalizeProfile(profile)
	if err != nil {
		panic(err)
	}

	localLikeProfile := normalizedProfile == "local" || normalizedProfile == "tailscale-serve"
	normalizedProvider := strings.TrimSpace(provider)

	var cleanApiKey *string
	if strings.TrimSpace(apiKey) != "" {
		cleanApiKey = &apiKey
	}

	config := &GatewayConfig{
		BindAddress: bindAddress,
		Port:        port,
		AuthToken:   authToken,
		Llm: LlmProviderConfig{
			Provider: normalizedProvider,
			Model:    model,
			ApiKey:   cleanApiKey,
		},
		Memory: MemoryConfig{
			Provider:    "file",
			StoragePath: memoryPath,
			Retention: &MemoryRetentionConfig{
				ArchivePath: filepath.Join(memoryPath, "archive"),
			},
		},
		Tooling: ToolingConfig{
			WorkspaceRoot:       workspacePath,
			WorkspaceOnly:       true,
			AllowShell:          localLikeProfile,
			EnableBrowserTool:   false,
			AllowedReadRoots:    []string{workspacePath},
			AllowedWriteRoots:   []string{workspacePath},
			RequireToolApproval: normalizedProfile == "public",
		},
		Security: SecurityConfig{
			AllowQueryStringToken:                    false,
			TrustForwardedHeaders:                    normalizedProfile == "public",
			RequireRequesterMatchForHttpToolApproval: normalizedProfile == "public",
		},
	}

	if normalizedProfile == "tailscale-serve" {
		config.Deployment = &DeploymentConfig{
			Mode:             "tailscale-serve",
			PublicExposure:   false,
			ReverseProxy:     "tailscale-serve",
			ExpectedLocalUrl: (&GatewaySetupArtifacts{}).BuildReachableBaseUrl(bindAddress, port),
		}
	}

	g.configureModelProfiles(config, normalizedProvider, model, modelPresetId, warnings)

	if normalizedProfile == "public" {
		config.Plugins.Enabled = false
		if warnings != nil {
			*warnings = append(*warnings, "Public profile disables third-party bridge plugins by default. Re-enable them only after you have a proxy, TLS, and explicit public-bind trust settings in place.")
		}
	}

	if normalizedProfile == "public" &&
		strings.TrimSpace(apiKey) != "" &&
		!strings.HasPrefix(strings.ToLower(apiKey), "env:") {
		if warnings != nil {
			*warnings = append(*warnings, "Public profile is using a direct API key value in the config file. Prefer env:... references or OS-backed secret storage.")
		}
	}

	return config
}

func (g *GatewaySetupProfileFactory) NormalizeProfile(profile string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(profile))
	if normalized != "local" && normalized != "public" && normalized != "tailscale-serve" {
		return "", errors.New("invalid value for --profile (expected: local|public|tailscale-serve)")
	}
	return normalized, nil
}

func (g *GatewaySetupProfileFactory) configureModelProfiles(
	config *GatewayConfig,
	provider string,
	model string,
	modelPresetId *string,
	warnings *[]string,
) {
	if !strings.EqualFold(provider, "ollama") {
		if strings.EqualFold(provider, "embedded") {
			g.configureEmbeddedModelProfile(config, model, modelPresetId, warnings)
			return
		}

		if modelPresetId != nil && strings.TrimSpace(*modelPresetId) != "" {
			if warnings != nil {
				*warnings = append(*warnings, "Ignoring model preset '"+*modelPresetId+"' because local presets currently apply only to Ollama or embedded providers.")
			}
		}
		return
	}

	config.Llm.Endpoint = "http://127.0.0.1:11434"
	config.Models.DefaultProfile = "local-primary"

	var preset *LocalModelPresetDefinition
	var hasPreset bool

	if modelPresetId != nil && strings.TrimSpace(*modelPresetId) != "" {
		preset, hasPreset = TryGetLocalModelPreset(*modelPresetId)
		if !hasPreset && warnings != nil {
			*warnings = append(*warnings, "Unknown model preset '"+*modelPresetId+"'. Falling back to inferred local capabilities.")
		}
	}

	var capabilities ModelCapabilities
	if hasPreset && preset != nil {
		capabilities = preset.Capabilities
	} else {
		capabilities = ModelCapabilities{
			SupportsStreaming:      true,
			SupportsSystemMessages: true,
			MaxContextTokens:       32768,
			MaxOutputTokens:        4096,
		}
	}

	var presetId *string
	var tags []string
	if hasPreset && preset != nil {
		presetId = &preset.Id
		tags = preset.Tags
	} else {
		tags = []string{"local", "private"}
	}

	config.Models.Profiles = []ModelProfileConfig{
		{
			Id:           "local-primary",
			PresetId:     presetId,
			Provider:     "ollama",
			Model:        model,
			BaseUrl:      "http://127.0.0.1:11434",
			Tags:         tags,
			Capabilities: g.cloneCapabilities(capabilities),
		},
	}
}

func (g *GatewaySetupProfileFactory) configureEmbeddedModelProfile(
	config *GatewayConfig,
	model string,
	modelPresetId *string,
	warnings *[]string,
) {
	config.Llm.ApiKey = nil
	config.LocalInference.Enabled = true
	config.LocalInference.AutoStart = true
	config.Models.DefaultProfile = "embedded-local"

	var packageDef *LocalModelPackageDefinition
	var hasPackage bool

	if modelPresetId != nil && strings.TrimSpace(*modelPresetId) != "" {
		packageDef, hasPackage = TryGetLocalModelPackage(*modelPresetId)
		if !hasPackage && warnings != nil {
			*warnings = append(*warnings, "Unknown embedded local model preset or package '"+*modelPresetId+"'. Falling back to inferred embedded capabilities.")
		}
	} else {
		packageDef, hasPackage = TryGetLocalModelPackage(model)
		if !hasPackage {
			packageDef, _ = TryGetLocalModelPackage("gemma-local-small-q4")
		}
	}

	var capabilities ModelCapabilities
	if packageDef != nil {
		capabilities = packageDef.Capabilities
	} else {
		capabilities = ModelCapabilities{
			SupportsStreaming:      true,
			SupportsSystemMessages: true,
			MaxContextTokens:       4096,
			MaxOutputTokens:        1024,
		}
	}

	modelId := model
	if packageDef != nil {
		modelId = packageDef.ModelId
	}
	config.Llm.Model = modelId

	if packageDef != nil {
		config.LocalInference.Backend = packageDef.Runtime.Backend
		config.LocalInference.ContextSize = packageDef.Runtime.ContextSize
		config.LocalInference.EnableJinja = packageDef.Runtime.EnableJinja
		config.LocalInference.ChatTemplate = packageDef.Runtime.ChatTemplate
		config.LocalInference.ReasoningMode = &packageDef.Runtime.ReasoningMode
		config.LocalInference.ReasoningBudget = packageDef.Runtime.ReasoningBudget
	}

	var presetId *string
	var tags []string
	if packageDef != nil {
		presetId = &packageDef.PresetId
		tags = packageDef.Tags
	} else {
		tags = []string{"local", "private", "offline", "cheap"}
	}

	config.Models.Profiles = []ModelProfileConfig{
		{
			Id:           "embedded-local",
			PresetId:     presetId,
			Provider:     "embedded",
			Model:        modelId,
			Tags:         tags,
			Capabilities: g.cloneCapabilities(capabilities),
		},
	}
}

func (g *GatewaySetupProfileFactory) cloneCapabilities(source ModelCapabilities) *ModelCapabilities {
	return &ModelCapabilities{
		SupportsTools:                  source.SupportsTools,
		SupportsVision:                 source.SupportsVision,
		SupportsJsonSchema:             source.SupportsJsonSchema,
		SupportsStructuredOutputs:      source.SupportsStructuredOutputs,
		SupportsStreaming:              source.SupportsStreaming,
		SupportsParallelToolCalls:      source.SupportsParallelToolCalls,
		SupportsReasoningEffort:        source.SupportsReasoningEffort,
		SupportsSystemMessages:         source.SupportsSystemMessages,
		SupportsImageInput:             source.SupportsImageInput,
		SupportsVideoInput:             source.SupportsVideoInput,
		SupportsAudioInput:             source.SupportsAudioInput,
		SupportsPromptCaching:          source.SupportsPromptCaching,
		SupportsExplicitCacheRetention: source.SupportsExplicitCacheRetention,
		ReportsCacheReadTokens:         source.ReportsCacheReadTokens,
		ReportsCacheWriteTokens:        source.ReportsCacheWriteTokens,
		MaxContextTokens:               source.MaxContextTokens,
		MaxOutputTokens:                source.MaxOutputTokens,
	}
}

func TryGetLocalModelPackage(s string) (*LocalModelPackageDefinition, bool) {
	// TODO
	panic("unimplemented")
}
func TryGetLocalModelPreset(s string) (*LocalModelPresetDefinition, bool) {
	// TODO
	panic("unimplemented")
}
