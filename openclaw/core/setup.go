package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

var GatewaySetupPathsIntance = &GatewaySetupPaths{}

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
		config.LocalInference.ReasoningMode = packageDef.Runtime.ReasoningMode
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

type LocalModelInstallRequest struct {
	SourcePath              string `json:"source_path"`
	MultimodalProjectorPath string `json:"multimodal_projector_path"`
	DraftModelPath          string `json:"draft_model_path"`
	SourceUrl               string `json:"source_url"`
	BearerToken             string `json:"bearer_token"`
	AcceptLicense           bool   `json:"accept_license"`
	ModelsRoot              string `json:"models_root"`
	DownloadOptionalFiles   bool   `json:"download_optional_files"`
}

type LocalModelInstallResult struct {
	Success bool                     `json:"success"`
	Message string                   `json:"message"`
	Status  *LocalModelPackageStatus `json:"status"`
}

type LocalModelCache struct {
}

func (l *LocalModelCache) GetPackageFiles(pack *LocalModelPackageDefinition) []LocalModelPackageFileDefinition {
	if pack == nil || len(pack.Files) == 0 {
		return []LocalModelPackageFileDefinition{
			{
				Role:             "model",
				FileName:         pack.FileName,
				DownloadUrl:      pack.DownloadUrl,
				ExpectedSha256:   pack.ExpectedSha256,
				Required:         true,
				InstallByDefault: true,
			},
		}
	}

	return pack.Files
}

func (l *LocalModelCache) ResolveModelsRoot(configuredRoot string) (string, error) {
	if strings.TrimSpace(configuredRoot) != "" {
		return l.ResolveConfiguredPath(configuredRoot)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		// 在 Windows 上通常是 C:\Users\用户名\AppData\Local
		localAppData, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(localAppData, "OpenClaw", "models"), nil
	}

	// 非 Windows 系统（Linux/macOS）
	return filepath.Join(home, ".openclaw", "models"), nil
}

func (l *LocalModelCache) ResolveConfiguredPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("configured path cannot be empty")
	}

	expanded := os.ExpandEnv(path)

	if expanded == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		expanded = home
	} else if strings.HasPrefix(expanded, "~/") || strings.HasPrefix(expanded, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		expanded = filepath.Join(home, expanded[2:])
	}

	absolutePath, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}

	return absolutePath, nil
}

func (l *LocalModelCache) GetPackageDirectory(pack *LocalModelPackageDefinition, modelsRoot string) string {
	r, err := l.ResolveModelsRoot(modelsRoot)
	if err != nil {
		return ""
	}
	return filepath.Join(r, pack.Id)
}

func (l *LocalModelCache) GetModelPath(pack *LocalModelPackageDefinition, modelsRoot string) string {
	return filepath.Join(l.GetPackageDirectory(pack, modelsRoot), pack.FileName)
}

func (l *LocalModelCache) GetPackageFilePath(pack *LocalModelPackageDefinition, file *LocalModelPackageFileDefinition, modelsRoot string) string {
	return filepath.Join(l.GetPackageDirectory(pack, modelsRoot), file.FileName)
}

func (l *LocalModelCache) GetPackageRolePath(pack *LocalModelPackageDefinition, role string, modelsRoot string) string {
	var fd LocalModelPackageFileDefinition
	fds := l.GetPackageFiles(pack)
	for _, f := range fds {
		if f.Role == role {
			fd = f
			break
		}
	}

	if len(fd.FileName) > 0 {
		return l.GetPackageFilePath(pack, &fd, modelsRoot)
	}

	return ""
}

func (l *LocalModelCache) GetManifestPath(pack *LocalModelPackageDefinition, modelsRoot string) string {
	return filepath.Join(l.GetPackageDirectory(pack, modelsRoot), "manifest.json")
}

func (l *LocalModelCache) TryReadManifest(path string) (*LocalModelInstallManifest, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest LocalModelInstallManifest

	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func (l *LocalModelCache) FindManifestFile(file *LocalModelPackageFileDefinition, manifest *LocalModelInstallManifest) *LocalModelInstallFileManifest {
	if manifest == nil {
		return nil
	}

	var match *LocalModelInstallFileManifest
	for _, item := range manifest.Files {
		if item.FileName == file.FileName || item.Role == file.Role {
			match = &item
			break
		}
	}
	if match != nil {
		return match
	}

	if file.Role == "model" && len(manifest.Sha256) > 0 {
		return &LocalModelInstallFileManifest{
			Role:     "model",
			FileName: manifest.FileName,
			Sha256:   manifest.Sha256,
			Source:   manifest.Source,
		}
	}

	return nil
}

func (l *LocalModelCache) WriteManifest(pack *LocalModelPackageDefinition, modelsRoot string, manifest *LocalModelInstallManifest) error {
	var path = l.GetManifestPath(pack, modelsRoot)

	path = filepath.Dir(path)
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (l *LocalModelCache) GetFileStatus(pack *LocalModelPackageDefinition, file *LocalModelPackageFileDefinition, manifest *LocalModelInstallManifest, modelsRoot string) *LocalModelPackageFileStatus {
	var path = l.GetPackageFilePath(pack, file, modelsRoot)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		issue := fmt.Sprintf("%s file is not installed", file.Role)
		if file.Required {
			issue = "Required " + issue
		}
		return &LocalModelPackageFileStatus{
			Role:     file.Role,
			FileName: file.FileName,
			Required: file.Required,
			Path:     path,
			Issue:    issue,
		}
	}

	var fileManifest = l.FindManifestFile(file, manifest)
	if fileManifest == nil {
		return &LocalModelPackageFileStatus{
			Role:      file.Role,
			FileName:  file.FileName,
			Required:  file.Required,
			Installed: true,
			Path:      path,
			Issue:     "Install manifest does not contain this file.",
		}
	}
	expected := file.ExpectedSha256
	if len(expected) == 0 {
		expected = fileManifest.Sha256
	}

	verified := len(expected) > 0 && (expected == fileManifest.Sha256)
	issue := "manifest checksum does not match the expected package checksum"
	if verified {
		issue = ""
	}

	return &LocalModelPackageFileStatus{
		Role:      file.Role,
		FileName:  file.FileName,
		Required:  file.Required,
		Installed: true,
		Path:      path,
		Verified:  verified,
		Sha256:    manifest.Sha256,
		Issue:     issue,
	}
}

func (l *LocalModelCache) ResolveManualSource(role string, request *LocalModelInstallRequest) string {
	if role == "mmproj" {
		return request.MultimodalProjectorPath
	}

	if role == "draft" {
		return request.DraftModelPath
	}
	return ""
}

func (l *LocalModelCache) WriteManifestAndVerify(pack *LocalModelPackageDefinition, installedFiles []LocalModelInstallFileManifest, request *LocalModelInstallRequest, primarySource string) *LocalModelInstallResult {
	var primary *LocalModelInstallFileManifest
	for _, v := range installedFiles {
		if v.Role == "model" {
			primary = &v
			break
		}
	}

	sha256 := ""
	if primary != nil {
		sha256 = primary.Sha256
	}

	l.WriteManifest(pack, request.ModelsRoot, &LocalModelInstallManifest{
		PackageId:       pack.Id,
		PresetId:        pack.PresetId,
		ModelId:         pack.ModelId,
		FileName:        pack.FileName,
		Sha256:          sha256,
		Source:          primarySource,
		LicenseUrl:      pack.LicenseUrl,
		LicenseAccepted: request.AcceptLicense,
		Files:           installedFiles,
	})

	var status = l.GetStatus(pack, request.ModelsRoot)
	if status == nil {
		return nil
	}

	message := status.Issue
	if status.Verified {
		message = "installed " + pack.Id
	} else if len(message) == 0 {
		message = fmt.Sprintf("installed %s, but verification did not pass.", pack.Id)
	}
	return &LocalModelInstallResult{
		Success: status.Verified,
		Message: message,
		Status:  status,
	}
}

type InstallFileWrite struct {
	Success  bool
	Manifest *LocalModelInstallFileManifest
	Result   *LocalModelInstallResult
}

func (l *LocalModelCache) BuildManifestEntry(ctx context.Context, pack *LocalModelPackageDefinition, file *LocalModelPackageFileDefinition, path, source string) (*InstallFileWrite, error) {
	sha, err := l.ComputeSha256(ctx, path)
	if err != nil {
		return nil, err
	}

	if file != nil && len(file.ExpectedSha256) > 0 && sha != file.ExpectedSha256 {
		os.Remove(path)
		return &InstallFileWrite{
			Success: false,
			Result: &LocalModelInstallResult{
				Message: fmt.Sprintf("checksum mismatch for %s file '%s' in package '%s'", file.Role, file.FileName, pack.Id),
			},
		}, nil
	}

	return &InstallFileWrite{
		Success: true,
		Manifest: &LocalModelInstallFileManifest{
			Role:     file.Role,
			FileName: file.FileName,
			Sha256:   sha,
			Source:   source,
		},
	}, nil
}

func (l *LocalModelCache) ComputeSha256(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()

	cancelReader := &contextReader{ctx: ctx, r: file}

	if _, err := io.Copy(hasher, cancelReader); err != nil {
		return "", err
	}

	hashBytes := hasher.Sum(nil)

	return hex.EncodeToString(hashBytes), nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *contextReader) Read(p []byte) (n int, err error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}

func (l *LocalModelCache) DownloadInstallFile(ctx context.Context, packageDef *LocalModelPackageDefinition, file *LocalModelPackageFileDefinition, downloadUrl string, request *LocalModelInstallRequest, httpCli *http.Client) (*InstallFileWrite, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadUrl, nil)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(request.BearerToken) != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+request.BearerToken)
	}

	response, err := httpCli.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &InstallFileWrite{
			Success: false,
			Result: &LocalModelInstallResult{
				Success: false,
				Message: fmt.Sprintf("Download for %s file '%s' failed with HTTP %d %s.",
					file.Role, file.FileName, response.StatusCode, response.Status),
			},
		}, nil
	}

	destinationPath := l.GetPackageFilePath(packageDef, file, request.ModelsRoot)

	destination, err := os.Create(destinationPath)
	if err != nil {
		return nil, err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, response.Body); err != nil {
		return nil, err
	}

	return l.BuildManifestEntry(ctx, packageDef, file, destinationPath, downloadUrl)
}

func (l *LocalModelCache) CopyInstallFile(ctx context.Context, packageData *LocalModelPackageDefinition, file *LocalModelPackageFileDefinition, source, modelsRoot string) (*InstallFileWrite, error) {
	sourcePath, err := l.ResolveConfiguredPath(source)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return &InstallFileWrite{
			Success: false,
			Result: &LocalModelInstallResult{
				Success: false,
				Message: fmt.Sprintf("Source %s file was not found: %s", file.Role, sourcePath),
			},
		}, nil
	}

	destinationPath := l.GetPackageFilePath(packageData, file, modelsRoot)

	if err := copyFile(sourcePath, destinationPath); err != nil {
		return nil, err
	}

	return l.BuildManifestEntry(ctx, packageData, file, destinationPath, sourcePath)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// os.Create 会默认清空并覆盖已存在的文件（对应 overwrite: true）
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (l *LocalModelCache) GetStatus(pkg *LocalModelPackageDefinition, modelsRoot string) *LocalModelPackageStatus {
	packageFiles := l.GetPackageFiles(pkg)

	var primaryFile LocalModelPackageFileDefinition
	for _, item := range packageFiles {
		if strings.EqualFold(item.Role, "model") {
			primaryFile = item
			break
		}
	}

	modelPath := l.GetPackageFilePath(pkg, &primaryFile, modelsRoot)
	manifestPath := l.GetManifestPath(pkg, modelsRoot)

	manifest, manifestError := l.TryReadManifest(manifestPath)

	fileStatuses := make([]LocalModelPackageFileStatus, len(packageFiles))
	for i, file := range packageFiles {
		status := l.GetFileStatus(pkg, &file, manifest, modelsRoot)
		if status != nil {
			fileStatuses[i] = *status
		}
	}

	installed := true
	verified := manifest != nil

	for _, file := range fileStatuses {
		if file.Required {
			if !file.Installed {
				installed = false
			}
			if !file.Verified {
				verified = false
			}
		}
	}

	var issue string
	if manifest == nil {
		if manifestError != nil {
			issue = manifestError.Error()
		} else {
			issue = "Install manifest is missing."
		}
	} else {
		for _, file := range fileStatuses {
			if file.Required && !file.Verified {
				issue = file.Issue
				break
			}
		}
	}

	var sha256 string
	if manifest != nil {
		for _, file := range manifest.Files {
			if strings.EqualFold(file.Role, "model") {
				sha256 = file.Sha256
				break
			}
		}
		if sha256 == "" {
			sha256 = manifest.Sha256
		}
	}

	if verified {
		issue = ""
	} else if issue == "" {
		issue = "Package files are not installed and verified."
	}

	return &LocalModelPackageStatus{
		PackageId:   pkg.Id,
		PresetId:    pkg.PresetId,
		ModelId:     pkg.ModelId,
		DisplayName: pkg.DisplayName,
		Installed:   installed,
		Verified:    verified,
		ModelPath:   modelPath,
		Sha256:      sha256,
		Issue:       issue,
		Files:       fileStatuses,
	}
}

func (l *LocalModelCache) ListStatuses(modelsRoot string) []LocalModelPackageStatus {
	result := []LocalModelPackageStatus{}
	for _, v := range LocalModelPackageDefinitionPackages {
		statu := l.GetStatus(&v, modelsRoot)
		if statu != nil {
			result = append(result, *statu)
		}
	}

	return result
}

func (l *LocalModelCache) Install(ctx context.Context, packageDef *LocalModelPackageDefinition, request *LocalModelInstallRequest) (*LocalModelInstallResult, error) {
	// 1. 验证授权许可
	if packageDef.RequiresLicenseAcceptance && !request.AcceptLicense {
		return &LocalModelInstallResult{
			Success: false,
			Message: fmt.Sprintf("Package '%s' requires explicit license acceptance: %s", packageDef.Id, packageDef.LicenseUrl),
		}, nil
	}

	// 2. 初始化目录与文件列表
	packageDir := l.GetPackageDirectory(packageDef, request.ModelsRoot)
	if err := os.MkdirAll(packageDir, os.ModePerm); err != nil {
		return nil, err
	}

	packageFiles := l.GetPackageFiles(packageDef)

	var primaryFile LocalModelPackageFileDefinition
	for _, item := range packageFiles {
		if strings.EqualFold(item.Role, "model") {
			primaryFile = item
			break
		}
	}

	installedFiles := make([]LocalModelInstallFileManifest, 0)
	source := request.SourcePath

	// ==========================================
	// 分支 A: 从本地路径 (SourcePath) 复制安装
	// ==========================================
	if strings.TrimSpace(source) != "" {
		// 复制主模型文件
		copyResult, err := l.CopyInstallFile(ctx, packageDef, &primaryFile, source, request.ModelsRoot)
		if err != nil {
			return nil, err
		}
		if !copyResult.Success {
			return copyResult.Result, nil
		}

		installedFiles = append(installedFiles, *copyResult.Manifest)

		// 处理其他非主模型文件
		for _, file := range packageFiles {
			if strings.EqualFold(file.Role, "model") {
				continue
			}

			fileSource := l.ResolveManualSource(file.Role, request)
			if strings.TrimSpace(fileSource) == "" {
				if file.Required {
					return &LocalModelInstallResult{
						Success: false,
						Message: fmt.Sprintf("Package '%s' requires %s file '%s'. Pass --%s-path or install from the package download.", packageDef.Id, file.Role, file.FileName, file.Role),
					}, nil
				}
				continue
			}

			copyResult, err = l.CopyInstallFile(ctx, packageDef, &file, fileSource, request.ModelsRoot)
			if err != nil {
				return nil, err
			}
			if !copyResult.Success {
				return copyResult.Result, nil
			}
			installedFiles = append(installedFiles, *copyResult.Manifest)
		}

		return l.WriteManifestAndVerify(packageDef, installedFiles, request, source), nil
	}

	// ==========================================
	// 分支 B: 从远程 URL 下载安装
	// ==========================================
	var filesToDownload []*LocalModelPackageFileDefinition
	for _, file := range packageFiles {
		if file.Required || (request.DownloadOptionalFiles && file.InstallByDefault) {
			filesToDownload = append(filesToDownload, &file)
		}
	}

	if len(filesToDownload) == 0 {
		return &LocalModelInstallResult{
			Success: false,
			Message: fmt.Sprintf("Package '%s' does not define a download URL. Use --path to install an existing GGUF file.", packageDef.Id),
		}, nil
	}

	// 验证 Gated 模型 Token
	if packageDef.RequiresDownloadToken && strings.TrimSpace(request.BearerToken) == "" {
		return &LocalModelInstallResult{
			Success: false,
			Message: fmt.Sprintf("Package '%s' is gated. Pass --accept-license and --token, or install from a local file with --path.", packageDef.Id),
		}, nil
	}

	httpClient := &http.Client{}

	for _, file := range filesToDownload {
		var downloadUrl string
		if strings.EqualFold(file.Role, "model") {
			if strings.TrimSpace(request.SourceUrl) != "" {
				downloadUrl = request.SourceUrl
			} else {
				downloadUrl = file.DownloadUrl
			}
		} else {
			downloadUrl = file.DownloadUrl
		}

		if strings.TrimSpace(downloadUrl) == "" {
			if file.Required {
				return &LocalModelInstallResult{
					Success: false,
					Message: fmt.Sprintf("Package '%s' does not define a download URL for required %s file '%s'.", packageDef.Id, file.Role, file.FileName),
				}, nil
			}
			continue
		}

		// 执行文件下载
		downloadResult, err := l.DownloadInstallFile(ctx, packageDef, file, downloadUrl, request, httpClient)
		if err != nil {
			return nil, err
		}
		if !downloadResult.Success {
			return downloadResult.Result, nil
		}
		installedFiles = append(installedFiles, *downloadResult.Manifest)
	}

	finalSourceUrl := primaryFile.DownloadUrl
	if strings.TrimSpace(request.SourceUrl) != "" {
		finalSourceUrl = request.SourceUrl
	}

	return l.WriteManifestAndVerify(packageDef, installedFiles, request, finalSourceUrl), nil
}

func (l *LocalModelCache) Remove(packageDef *LocalModelPackageDefinition, modelsRoot string) bool {
	directory := l.GetPackageDirectory(packageDef, modelsRoot)

	if !DirectoryExists(directory) {
		return false
	}

	err := os.RemoveAll(directory)
	return err == nil
}

func (l *LocalModelCache) Verify(ctx context.Context, packageDef *LocalModelPackageDefinition, modelsRoot string) (*LocalModelPackageStatus, error) {
	packageFiles := l.GetPackageFiles(packageDef)

	// 1. 检查是否有任何“必需(Required)”的文件在本地不存在
	for _, file := range packageFiles {
		if file.Required {
			path := l.GetPackageFilePath(packageDef, &file, modelsRoot)
			if !FileExists(path) {
				return l.GetStatus(packageDef, modelsRoot), nil
			}
		}
	}

	// 2. 尝试读取现有的 Manifest 配置文件
	manifest, err := l.TryReadManifest(l.GetManifestPath(packageDef, modelsRoot))
	if err != nil || manifest == nil {
		manifest = &LocalModelInstallManifest{
			PackageId:       packageDef.Id,
			PresetId:        packageDef.PresetId,
			ModelId:         packageDef.ModelId,
			FileName:        packageDef.FileName,
			Sha256:          "",
			LicenseUrl:      packageDef.LicenseUrl,
			LicenseAccepted: false,
		}
	}

	fileManifests := make([]LocalModelInstallFileManifest, 0)

	// 3. 遍历所有实际存在的文件，计算 SHA256 哈希
	for _, file := range packageFiles {
		path := l.GetPackageFilePath(packageDef, &file, modelsRoot)
		if FileExists(path) {
			sha256, err := l.ComputeSha256(ctx, path)
			if err != nil {
				return nil, err
			}

			// 查找旧 manifest 中对应角色的 Source 来源
			var source string
			if manifest.Files != nil {
				for _, item := range manifest.Files {
					if strings.EqualFold(item.Role, file.Role) {
						source = item.Source
						break
					}
				}
			}

			fileManifests = append(fileManifests, LocalModelInstallFileManifest{
				Role:     file.Role,
				FileName: file.FileName,
				Sha256:   sha256,
				Source:   source,
			})
		}
	}

	// 4. 获取主模型文件的 SHA256 (若没有则取旧 manifest 的值)
	primarySha := manifest.Sha256
	for _, item := range fileManifests {
		if strings.EqualFold(item.Role, "model") {
			if item.Sha256 != "" {
				primarySha = item.Sha256
			}
			break
		}
	}

	// 处理 LicenseUrl 的空值合并逻辑
	finalLicenseUrl := packageDef.LicenseUrl
	if manifest.LicenseUrl != "" {
		finalLicenseUrl = manifest.LicenseUrl
	}

	// 5. 写入更新后的本地清单文件
	l.WriteManifest(packageDef, modelsRoot, &LocalModelInstallManifest{
		PackageId:       packageDef.Id,
		PresetId:        packageDef.PresetId,
		ModelId:         packageDef.ModelId,
		FileName:        packageDef.FileName,
		Sha256:          primarySha,
		Source:          manifest.Source,
		LicenseUrl:      finalLicenseUrl,
		LicenseAccepted: manifest.LicenseAccepted,
		InstalledAtUtc:  manifest.InstalledAtUtc,
		Files:           fileManifests,
	})

	return l.GetStatus(packageDef, modelsRoot), nil
}
