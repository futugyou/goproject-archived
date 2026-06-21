package core

import (
	"encoding/json"
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
func (g GatewaySetupArtifacts) BuildEnvExample(apiKeyRef *string, authToken, workspacePath, baseUrl string) string {
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
func (g GatewaySetupArtifacts) ResolveProviderEnvVariable(apiKeyRef string) string {
	if len(apiKeyRef) > 4 && strings.HasPrefix(strings.ToLower(apiKeyRef), "env:") {
		return apiKeyRef[4:]
	}
	return "MODEL_PROVIDER_KEY"
}

// BuildEnvExamplePath 根据配置路径生成 .env.example 路径
func (g GatewaySetupArtifacts) BuildEnvExamplePath(configPath string) (string, error) {
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
func (g GatewaySetupArtifacts) BuildReachableBaseUrl(bindAddress string, port int) string {
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
