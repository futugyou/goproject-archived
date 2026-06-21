package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
