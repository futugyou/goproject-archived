package core

import (
	"strings"
)

type RuntimeConfig struct {
	Orchestrator string `json:"orchestrator"`
}

func DefaultRuntimeConfig() *RuntimeConfig {
	return &RuntimeConfig{
		Orchestrator: RuntimeOrchestratorNative,
	}
}

const (
	RuntimeOrchestratorNative = "native"
	RuntimeOrchestratorMaf    = "maf"
)

func RuntimeOrchestratorNormalize(orchestrator string) string {
	if orchestrator == "" {
		return RuntimeOrchestratorNative
	}
	return strings.ToLower(strings.TrimSpace(orchestrator))
}
