package capability

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type ResolveCapabilityTool struct {
	providers     CapabilityProviderRegistry
	isToolAllowed func(string) bool
}

func NewResolveCapabilityTool(providers CapabilityProviderRegistry, isToolAllowed func(string) bool) *ResolveCapabilityTool {
	return &ResolveCapabilityTool{providers: providers, isToolAllowed: isToolAllowed}
}

func (g *ResolveCapabilityTool) Name() string { return "resolve_capability" }

func (g *ResolveCapabilityTool) Description() string {
	return "Resolve a capability through a configured provider using deterministic selection."
}

func (g *ResolveCapabilityTool) ParameterSchema() string {
	return `
	 {
	"type": "object",
	"required": ["task_description"],
	"properties": {
		"task_description": {
			"type": "string"
		},
		"provider": {
			"type": "string"
		},
		"keywords": {
			"type": "array",
			"items": {
				"type": "string"
			}
		},
		"key_words": {
			"type": "string"
		},
		"selection_policy": {
			"type": "string",
			"enum": ["first", "exact_name"]
		}
	}
}
`
}

func (g *ResolveCapabilityTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(argumentsJson), &raw); err != nil {
		return "", err
	}

	rawTask, ok := raw["task_description"]
	if !ok {
		return "", errors.New("task_description is required")
	}
	var task string
	if err := json.Unmarshal(rawTask, &task); err != nil || strings.TrimSpace(task) == "" {
		return "", errors.New("task_description is required")
	}

	policyStr := "first"
	if rawPolicy, ok := raw["selection_policy"]; ok {
		if err := json.Unmarshal(rawPolicy, &policyStr); err != nil {
			return "", err
		}
	}
	if policyStr != "first" && policyStr != "exact_name" {
		return "", errors.New("Unsupported selection policy")
	}

	if _, ok := raw["prefer_version"]; ok {
		return "", errors.New("Unsupported constraint")
	}
	if _, ok := raw["top_k"]; ok {
		return "", errors.New("Unsupported constraint")
	}

	var providerStr string
	if rawProvider, ok := raw["provider"]; ok {
		if err := json.Unmarshal(rawProvider, &providerStr); err != nil || strings.TrimSpace(providerStr) == "" {
			return "", errors.New("provider must be a non-empty identifier")
		}
	}

	_, hasKeywords := raw["keywords"]
	_, hasKeyWords := raw["key_words"]

	if hasKeywords && hasKeyWords {
		return "", errors.New("Specify keywords or legacy key_words, not both")
	}

	var keywords string
	if hasKeywords {
		var kwList []string
		if err := json.Unmarshal(raw["keywords"], &kwList); err != nil {
			return "", errors.New("keywords must contain non-empty strings")
		}
		for _, w := range kwList {
			if strings.TrimSpace(w) == "" {
				return "", errors.New("keywords must contain non-empty strings")
			}
		}
		keywords = strings.Join(kwList, ",")
	} else if hasKeyWords {
		if err := json.Unmarshal(raw["key_words"], &keywords); err != nil {
			return "", err
		}
	}

	var policy core.ResolveCapabilitySelectionPolicy = "first"
	if policyStr == "exact_name" {
		policy = "exact_name"
	}

	req := core.ResolveCapabilityRequest{
		TaskDescription: task,
		KeyWords:        &keywords,
		SelectionPolicy: policy,
		Provider:        providerStr,
	}

	binding, failure, _, _, err := g.providers.Resolve(ctx, req, g.isToolAllowed)
	if err != nil {
		return "", err
	}

	var responseBytes []byte
	if binding != nil {
		responseBytes, err = json.Marshal(binding)
	} else {
		responseBytes, err = json.Marshal(failure)
	}

	if err != nil {
		return "", err
	}

	return string(responseBytes), nil
}
