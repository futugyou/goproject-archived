package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type HomeAssistantTool struct {
	config core.HomeAssistantConfig
	rest   *HomeAssistantRestClient
	index  *HomeAssistantIndex
}

func NewHomeAssistantTool(config core.HomeAssistantConfig, httpClient *http.Client) *HomeAssistantTool {
	return &HomeAssistantTool{
		config: config,
		rest:   NewHomeAssistantRestClient(config, httpClient),
		index:  NewHomeAssistantIndex(config),
	}
}

func (a *HomeAssistantTool) Name() string {
	return "home_assistant"
}

func (a *HomeAssistantTool) Description() string {
	return "Interact with a Home Assistant instance (read-only). List entities, read state, list services, and resolve targets by area/name."
}

func (a *HomeAssistantTool) ParameterSchema() string {
	return `
	{
          "type": "object",
          "properties": {
            "op": {
              "type": "string",
              "enum": ["list_entities","get_state","list_services","resolve_targets","describe_entity"],
              "description": "Operation to perform"
            },
            "entity_id": { "type": "string", "description": "Entity id (for get_state/describe_entity)" },
            "domain": { "type": "string", "description": "Optional domain filter (e.g. light, switch)" },
            "area": { "type": "string", "description": "Optional area filter (P2 indexing)" },
            "name_contains": { "type": "string", "description": "Optional name substring filter (P2 indexing)" },
            "limit": { "type": "integer", "description": "Maximum items to return" },
            "format": { "type": "string", "enum": ["text","json"], "default": "text" }
          },
          "required": ["op"]
        }`
}

type ReadAssistantModel struct {
	Op           string `json:"op"`
	Format       string `json:"format"`
	Domain       string `json:"domain"`
	Area         string `json:"area"`
	NameContains string `json:"name_contains"`
	Limit        int    `json:"limit"`
	EntityId     string `json:"entity_id"`
}

func (a *HomeAssistantTool) Execute(ctx context.Context, argumentsJson string) string {
	var model ReadAssistantModel
	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if model.Limit <= 0 {
		model.Limit = a.config.MaxEntities
	}

	if model.Format == "" {
		model.Format = "text"
	}

	model.Limit = util.Clamp(model.Limit, 1, a.config.MaxEntities)

	switch model.Op {
	case "list_entities":
		return a.listEntities(ctx, model.Domain, model.Area, model.NameContains, model.Limit, model.Format)
	case "get_state":
		return a.getState(ctx, model)
	case "list_services":
		return a.listServices(ctx, model.Format)
	case "resolve_targets":
		return a.resolveTargets(ctx, model)
	case "describe_entity":
		return a.describeEntity(ctx, model)
	default:
		return fmt.Sprintf("Error: Unknown op '%s'.", model.Op)
	}
}

func (a *HomeAssistantTool) describeEntity(ctx context.Context, model ReadAssistantModel) string {
	if model.EntityId == "" {
		return "Error: entity_id is required for describe_entity."
	}
	if err := a.index.RefreshRegistries(ctx); err != nil {
		return err.Error()
	}
	if err := a.index.EnsureWarm(ctx, *a.rest); err != nil {
		return err.Error()
	}

	raw, err := a.rest.GetState(ctx, model.EntityId)
	if err != nil {
		return err.Error()
	}

	if model.Format == "json" {
		return raw
	}

	var entry = a.index.Get(model.EntityId)

	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "entity_id: %s\n", model.EntityId)
	if entry != nil {
		if entry.FriendlyName != "" {
			fmt.Fprintf(&sb, "name: %s\n", entry.FriendlyName)
		}

		if entry.AreaName != "" {
			fmt.Fprintf(&sb, "area: %s\n", entry.AreaName)
		}

		if entry.DeviceName != "" {
			fmt.Fprintf(&sb, "device: %s\n", entry.DeviceName)
		}

		if entry.Platform != "" {
			fmt.Fprintf(&sb, "platform: %s\n", entry.Platform)
		}
	}

	var formatted = tryFormatSingleState(raw)
	if formatted != "" {
		sb.WriteString("\n")
		sb.WriteString(formatted)
		sb.WriteString("\n")
	} else {
		sb.WriteString("\n")
		sb.WriteString(raw)
		sb.WriteString("\n")
	}

	var entityDomain = strings.SplitAfterN(model.EntityId, ".", 2)[0]
	services, err := a.index.GetServicesForDomain(ctx, *a.rest, entityDomain)
	if err == nil && len(services) > 0 {
		sb.WriteString("\n")
		sb.WriteString("services:")
		sb.WriteString("\n")
		for _, svc := range services {
			fmt.Fprintf(&sb, "  - %s.%s\n", entityDomain, svc)
		}
	}

	return sb.String()
}

func (a *HomeAssistantTool) resolveTargets(ctx context.Context, model ReadAssistantModel) string {
	// Ensure we have registry metadata for area/name resolution (P2)
	if err := a.index.RefreshRegistries(ctx); err != nil {
		return err.Error()
	}
	if err := a.index.EnsureWarm(ctx, *a.rest); err != nil {
		return err.Error()
	}

	var matches = a.index.Query(model.Domain, model.Area, model.NameContains)
	limit := min(model.Limit, len(matches))
	matches = matches[:limit]

	if model.Format == "json" {
		data, err := json.Marshal(matches)
		if err != nil {
			return err.Error()
		}
		return string(data)
	}

	if len(matches) == 0 {
		return "No targets matched."
	}

	var sb = strings.Builder{}
	sb.WriteString("Matches:\n")
	for i := 0; i < len(matches); i++ {
		var e = matches[i]
		sb.WriteString("  - ")
		sb.WriteString(e.EntityId)
		if e.FriendlyName != "" {
			sb.WriteString("  \"")
			sb.WriteString(e.FriendlyName)
			sb.WriteByte('"')
		}
		if e.AreaName != "" {
			sb.WriteString("  (")
			sb.WriteString(e.AreaName)
			sb.WriteByte(')')
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (a *HomeAssistantTool) listServices(ctx context.Context, format string) string {
	raw, err := a.rest.GetServices(ctx)
	if err != nil {
		return err.Error()
	}
	if format == "json" {
		return raw
	}

	var model []struct {
		Domain   string         `json:"domain"`
		Services map[string]any `json:"services"`
	}

	if err := json.Unmarshal([]byte(raw), &model); err != nil {
		return raw
	}

	sb := strings.Builder{}
	for _, domain := range model {
		if domain.Domain == "" {
			continue
		}

		for key := range domain.Services {
			sb.WriteString(domain.Domain)
			sb.WriteByte('.')
			sb.WriteString(key)
			sb.WriteString("\n")
		}
	}

	if sb.Len() > 0 {
		return "No services found."
	}

	return sb.String()
}

func tryFormatSingleState(rawJson string) string {
	var model struct {
		EntityId   string  `json:"entity_id"`
		State      *string `json:"state,omitempty"`
		Attributes struct {
			FriendlyName string `json:"friendly_name"`
		} `json:"attributes"`
		LastChanged string `json:"last_changed"`
	}

	if err := json.Unmarshal([]byte(rawJson), &model); err != nil {
		return err.Error()
	}

	var sb = strings.Builder{}
	if model.EntityId != "" {
		fmt.Fprintf(&sb, "entity_id: %s\n", model.EntityId)
	}

	if model.Attributes.FriendlyName != "" {
		fmt.Fprintf(&sb, "name: %s\n", model.Attributes.FriendlyName)
	}

	if model.State != nil && *model.State != "" {
		fmt.Fprintf(&sb, "state: %s\n", *model.State)
	}

	if model.LastChanged != "" {
		fmt.Fprintf(&sb, "last_changed: %s\n", model.LastChanged)
	}

	return util.TrimEnd(sb.String())
}

func (a *HomeAssistantTool) getState(ctx context.Context, model ReadAssistantModel) string {
	if model.EntityId != "" {
		return "Error: entity_id is required for get_state."
	}

	raw, err := a.rest.GetState(ctx, model.EntityId)
	if err != nil {
		return err.Error()
	}
	if model.Format == "json" {
		return raw
	}

	ss := tryFormatSingleState(raw)
	if ss == "" {
		return raw
	}
	return ss
}

func (a *HomeAssistantTool) listEntities(ctx context.Context, domain string, area string, contains string, limit int, format string) string {
	if err := a.index.EnsureWarm(ctx, *a.rest); err != nil {
		return err.Error()
	}

	var matches = a.index.Query(domain, area, contains)
	limit = min(limit, len(matches))
	matches = matches[:limit]

	if format == "json" {
		data, err := json.Marshal(matches)
		if err != nil {
			return err.Error()
		}
		return string(data)
	}

	if len(matches) == 0 {
		return "No entities matched."
	}

	var sb = strings.Builder{}
	for i := 0; i < len(matches); i++ {
		var e = matches[i]
		sb.WriteByte('[')
		sb.WriteString(strconv.Itoa(i + 1))
		sb.WriteString("] ")
		sb.WriteString(e.EntityId)

		if e.FriendlyName != "" {
			sb.WriteString("  \"")
			sb.WriteString(e.FriendlyName)
			sb.WriteByte('"')
		}

		if e.AreaName != "" {
			sb.WriteString("  (")
			sb.WriteString(e.AreaName)
			sb.WriteByte(')')
		}

		if e.State != "" {
			sb.WriteString("  = ")
			sb.WriteString(e.State)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
