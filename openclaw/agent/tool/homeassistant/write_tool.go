package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type HomeAssistantWriteTool struct {
	config        core.HomeAssistantConfig
	rest          *HomeAssistantRestClient
	toolingConfig *core.ToolingConfig
}

func NewHomeAssistantWriteTool(config core.HomeAssistantConfig, httpClient *http.Client, toolingConfig *core.ToolingConfig) *HomeAssistantWriteTool {
	return &HomeAssistantWriteTool{
		config:        config,
		rest:          NewHomeAssistantRestClient(config, httpClient),
		toolingConfig: toolingConfig,
	}
}

func (a *HomeAssistantWriteTool) Name() string {
	return "home_assistant_write"
}

func (a *HomeAssistantWriteTool) Description() string {
	return "Control Home Assistant devices by calling services (write operations). Use with care."
}

func (a *HomeAssistantWriteTool) ParameterSchema() string {
	return `
	{
          "type": "object",
          "properties": {
            "op": { "type": "string", "enum": ["call_service","call_services"] },
            "domain": { "type": "string" },
            "service": { "type": "string" },
            "entity_id": {
              "oneOf": [
                { "type": "string" },
                { "type": "array", "items": { "type": "string" } },
                { "type": "null" }
              ]
            },
            "data": { "type": ["object","null"] },
            "calls": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "domain": { "type": "string" },
                  "service": { "type": "string" },
                  "entity_id": {
                    "oneOf": [
                      { "type": "string" },
                      { "type": "array", "items": { "type": "string" } },
                      { "type": "null" }
                    ]
                  },
                  "data": { "type": ["object","null"] }
                },
                "required": ["domain","service"]
              }
            }
          },
          "required": ["op"]
        }`
}

type WriteAssistantModel struct {
	Op       string `json:"op"`
	Domain   string `json:"domain"`
	Service  string `json:"service"`
	EntityId string `json:"entity_id"`

	NameContains string `json:"name_contains"`
	Limit        int    `json:"limit"`
}

func (a *HomeAssistantWriteTool) Execute(ctx context.Context, argumentsJson string) string {
	if a.toolingConfig != nil && a.toolingConfig.ReadOnlyMode {
		return "Error: home_assistant_write is disabled because Tooling.ReadOnlyMode is enabled."
	}

	var model map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if op, ok := model["op"].(string); ok {
		switch op {
		case "call_service":
			return a.callService(ctx, model)
		case "call_services":
			return a.callServices(ctx, model)
		default:
			return fmt.Sprintf("Error: Unknown op '%s'.", op)
		}
	}

	return "Error: Unknown op"
}

func readEntityIds(root map[string]any) []string {
	eid, exists := root["entity_id"]
	if !exists {
		return []string{}
	}

	switch v := eid.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return []string{}
		}
		return []string{v}
	case []any:
		var list []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					list = append(list, s)
				}
			}
		}
		return list
	default:
		return []string{}
	}
}

func buildServiceCallBody(root map[string]any, entityIds []string) string {
	body := make(map[string]any)

	if len(entityIds) == 1 {
		body["entity_id"] = entityIds[0]
	} else if len(entityIds) > 1 {
		body["entity_id"] = entityIds
	}

	if data, ok := root["data"].(map[string]any); ok {
		maps.Copy(body, data)
	}

	str, err := json.Marshal(body)
	if err != nil {
		return ""
	}

	return string(str)
}

func (a *HomeAssistantWriteTool) callServices(ctx context.Context, model map[string]any) string {
	var domain = util.GetString(model, "domain")
	if domain == nil {
		return "Missing required string field 'domain'."
	}
	var service = util.GetString(model, "service")
	if service == nil {
		return "Missing required string field 'service'."
	}
	var serviceName = fmt.Sprintf("%s.%s", *domain, *service)

	if !core.GlobMatcherInstance.IsAllowed(a.config.Policy.AllowServiceGlobs, a.config.Policy.DenyServiceGlobs, serviceName) {
		return fmt.Sprintf("Error: Service '%s' is not allowed by policy.", serviceName)
	}

	var entityIds = readEntityIds(model)
	for _, entityId := range entityIds {
		if !core.GlobMatcherInstance.IsAllowed(a.config.Policy.AllowEntityIdGlobs, a.config.Policy.DenyEntityIdGlobs, entityId) {
			return fmt.Sprintf("Error: Entity '%s' is not allowed by policy.", entityId)
		}
	}

	var bodyJson = buildServiceCallBody(model, entityIds)
	if bodyJson == "" {
		return "can not build service call body"
	}

	result, err := a.rest.CallService(ctx, *domain, *service, bodyJson)
	if err != nil {
		return err.Error()
	}
	return result
}

func (a *HomeAssistantWriteTool) callService(ctx context.Context, model map[string]any) string {
	callsRaw, ok := model["calls"].([]any)
	if !ok {
		return "Error: calls is required for call_services."
	}

	var sb = strings.Builder{}

	for i, item := range callsRaw {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}

		i++
		var domain = util.GetString(model, "domain")
		if domain == nil {
			return "Missing required string field 'domain'."
		}
		var service = util.GetString(model, "service")
		if service == nil {
			return "Missing required string field 'service'."
		}
		var serviceName = fmt.Sprintf("%s.%s", *domain, *service)

		if !core.GlobMatcherInstance.IsAllowed(a.config.Policy.AllowServiceGlobs, a.config.Policy.DenyServiceGlobs, serviceName) {
			sb.WriteString(fmt.Sprintf("[%d] Error: Service '%s' is not allowed by policy.\n", i, serviceName))
			continue
		}

		var entityIds = readEntityIds(call)
		denied := ""
		for _, eid := range entityIds {
			if !core.GlobMatcherInstance.IsAllowed(a.config.Policy.AllowEntityIdGlobs, a.config.Policy.DenyEntityIdGlobs, eid) {
				denied = eid
				break
			}
		}

		if denied != "" {
			sb.WriteString(fmt.Sprintf("[%d] Error: Entity '%s' is not allowed by policy.\n", i, denied))
			continue
		}

		var bodyJson = buildServiceCallBody(call, entityIds)
		if bodyJson == "" {
			sb.WriteString(fmt.Sprintf("[%d] can not build service call body\n", i))
			continue
		}

		result, err := a.rest.CallService(ctx, *domain, *service, bodyJson)
		if err != nil {
			sb.WriteString(fmt.Sprintf("[%d] Error calling%s: %s\n", i, serviceName, err.Error()))
		}
		sb.WriteString(fmt.Sprintf("[%d] OK: %s\n", i, serviceName))
		if result != "" {
			sb.WriteString(result)
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\r\n")
}
