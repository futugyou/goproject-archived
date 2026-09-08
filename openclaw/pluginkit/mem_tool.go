package pluginkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type KnowledgeGraphTool struct {
	provider func() (IKnowledgeGraph, error)
}

func NewKnowledgeGraphTool(provider func() (IKnowledgeGraph, error)) *KnowledgeGraphTool {
	return &KnowledgeGraphTool{provider: provider}
}

func (a *KnowledgeGraphTool) Name() string {
	return "mempalace_kg"
}

func (a *KnowledgeGraphTool) Description() string {
	return "Read and write MemPalace temporal knowledge graph relationships with validity windows."
}

func (a *KnowledgeGraphTool) ParameterSchema() string {
	return `{
          "type": "object",
          "properties": {
            "action": { "type": "string", "enum": ["add","query","timeline"] },
            "subject": { "type": "string", "description": "Subject entity as type:id for add/query" },
            "predicate": { "type": "string", "description": "Relationship predicate for add/query" },
            "object": { "type": "string", "description": "Object entity as type:id for add/query" },
            "entity": { "type": "string", "description": "Entity as type:id for timeline" },
            "at": { "type": "string", "description": "Optional ISO8601 point-in-time query for query actions" },
            "valid_from": { "type": "string", "description": "Optional ISO8601 valid-from timestamp for add actions" },
            "from": { "type": "string", "description": "Optional ISO8601 timeline start" },
            "to": { "type": "string", "description": "Optional ISO8601 timeline end" }
          },
          "required": ["action"]
        }`
}

type kgArgs struct {
	Action    string `json:"action"`
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
	Entity    string `json:"entity"`
	At        string `json:"at"`
	ValidFrom string `json:"valid_from"`
	From      string `json:"from"`
	To        string `json:"to"`
}

func (t *KnowledgeGraphTool) Execute(ctx context.Context, argumentsJson string) string {
	kg, err := t.provider()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	var args kgArgs
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return "Error: invalid JSON arguments"
	}

	switch args.Action {
	case "add":
		return t.handleAdd(ctx, kg, args)
	case "query":
		return t.handleQuery(ctx, kg, args)
	case "timeline":
		return t.handleTimeline(ctx, kg, args)
	default:
		return "Error: action must be one of add, query, or timeline."
	}
}

func (a *KnowledgeGraphTool) handleTimeline(ctx context.Context, kg IKnowledgeGraph, args kgArgs) string {
	entity, err := ParseEntityRef(args.Entity)
	if err != nil {
		return err.Error()
	}

	if entity.ID == "" {
		return "Error: timeline requires entity."
	}

	var from *time.Time
	if args.From != "" {
		if parsed, err := time.Parse(time.RFC3339, args.From); err == nil {
			from = &parsed
		}
	}

	var to *time.Time
	if args.To != "" {
		if parsed, err := time.Parse(time.RFC3339, args.To); err == nil {
			from = &parsed
		}
	}

	events, err := kg.Timeline(
		ctx,
		entity,
		from,
		to,
	)

	if err != nil {
		return fmt.Sprintf("Error timeline triple: %v", err)
	}

	if len(events) == 0 {
		return "No temporal knowledge graph events found."
	}

	sb := &strings.Builder{}
	fmt.Fprintf(sb, "Events: %d\n", len(events))
	for _, item := range events {
		arrow := "<-"
		if item.Direction == "outgoing" {
			arrow = "->"
		}

		sb.WriteString(item.At.Format(time.RFC3339Nano))
		sb.WriteString(" ")
		sb.WriteString(arrow)
		sb.WriteString(" ")
		sb.WriteString(item.Predicate)
		sb.WriteString(" ")
		sb.WriteString(item.Other.String())
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

func (a *KnowledgeGraphTool) handleQuery(ctx context.Context, kg IKnowledgeGraph, args kgArgs) string {
	subject, err := ParseEntityRef(args.Subject)
	if err != nil {
		return err.Error()
	}

	obj, err := ParseEntityRef(args.Object)
	if err != nil {
		return err.Error()
	}

	predicate := args.Predicate
	if subject.ID == "" || obj.ID == "" || predicate == "" {
		return "Error: add requires subject, predicate, and object."
	}

	var now = time.Now().UTC()
	var validFrom *time.Time
	if args.ValidFrom != "" {
		if parsed, err := time.Parse(time.RFC3339, args.ValidFrom); err == nil {
			validFrom = &parsed
		}
	}

	if validFrom == nil && args.At != "" {
		if parsed, err := time.Parse(time.RFC3339, args.At); err == nil {
			validFrom = &parsed
		}
	}

	if validFrom == nil {
		validFrom = &now
	}

	id, err := kg.Add(ctx, TemporalTriple{
		Subject:   subject,
		Predicate: predicate,
		Object:    obj,
		ValidFrom: *validFrom,
		CreatedAt: now,
	})

	if err != nil {
		return fmt.Sprintf("Error adding triple: %v", err)
	}

	return fmt.Sprintf("Added temporal triple %d: %s %s %s", id, subject, args.Predicate, obj)
}

func (a *KnowledgeGraphTool) handleAdd(ctx context.Context, kg IKnowledgeGraph, args kgArgs) string {
	sub, err := ParseEntityRef(args.Subject)
	if err != nil {
		return err.Error()
	}

	obj, err := ParseEntityRef(args.Object)
	if err != nil {
		return err.Error()
	}

	now := time.Now().UTC()
	validFrom := now
	if args.ValidFrom != "" {
		if parsed, err := time.Parse(time.RFC3339, args.ValidFrom); err == nil {
			validFrom = parsed
		}
	}

	id, err := kg.Add(ctx, TemporalTriple{
		Subject:   sub,
		Predicate: args.Predicate,
		Object:    obj,
		ValidFrom: validFrom,
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Sprintf("Error adding triple: %v", err)
	}

	return fmt.Sprintf("Added temporal triple %d: %s %s %s", id, sub, args.Predicate, obj)
}
