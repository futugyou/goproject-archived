package metaskill

import (
	"context"
	"encoding/json"
)

type MetaSkillFillSlotsTool struct {
}

func (e *MetaSkillFillSlotsTool) Name() string {
	return "meta_skill_fill_slots"
}

func (e *MetaSkillFillSlotsTool) Description() string {
	return "Drive slot-filling and return validated JSON consumed by meta_skill_assemble."
}

func (e *MetaSkillFillSlotsTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"pattern_id": {
			"type": "string"
		},
		"history_summary": {
			"type": "string"
		},
		"user_intent": {
			"type": "string"
		}
	},
	"required": ["pattern_id", "history_summary", "user_intent"]
}
"`
}

type FillSlotModel struct {
	PatternId      string `json:"pattern_id"`
	HistorySummary string `json:"history_summary"`
	UserIntent     string `json:"user_intent"`
}

type FillSlotsResult struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	MetaPriority int             `json:"meta_priority"`
	Triggers     []string        `json:"triggers"`
	Steps        []FillSlotsStep `json:"steps,omitempty"`
	Branches     []FillSlotsStep `json:"branches,omitempty"`
	Merge        *FillSlotsStep  `json:"merge,omitempty"`
	Tail         *FillSlotsStep  `json:"tail,omitempty"`
}

type FillSlotsStep struct {
	ID       string            `json:"id"`
	Skill    string            `json:"skill"`
	Task     string            `json:"task"`
	WithKeys map[string]string `json:"with_keys"`
}

func (a *MetaSkillFillSlotsTool) Execute(ctx context.Context, argumentsJson string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}

	if argumentsJson == "" {
		argumentsJson = "{}"
	}

	var doc FillSlotModel

	if err := json.Unmarshal([]byte(argumentsJson), &doc); err != nil {
		return err.Error()
	}

	if !IsSupportedPattern(doc.PatternId) {
		msg, err := SerializeError("unknown_pattern_id", "Unsupported pattern_id "+doc.PatternId)
		if err != nil {
			return err.Error()
		}
		return msg
	}

	if doc.UserIntent == "" {
		msg, err := SerializeError("invalid_arguments", "'user_intent' is required.")
		if err != nil {
			return err.Error()
		}
		return msg
	}

	if doc.HistorySummary != "" {
		msg, err := SerializeError("invalid_arguments", "'history_summary' is required.")
		if err != nil {
			return err.Error()
		}
		return msg
	}

	var requiredTriggers = ExtractRequiredTriggersFromIntent(doc.UserIntent)
	var fallbackTrigger = "create a meta-skill"
	var triggers = []string{fallbackTrigger}
	if len(requiredTriggers) > 0 {
		triggers = requiredTriggers
	}

	if len(triggers) > 8 {
		triggers = triggers[:8]
	}

	var baseName = BuildNameFromIntent(doc.UserIntent, doc.PatternId)
	description, err := BuildDescription(doc.UserIntent)
	if err != nil {
		return err.Error()
	}

	wf := FillSlotsResult{
		Name:         baseName,
		Description:  description,
		MetaPriority: 50,
		Triggers:     triggers,
	}

	emptyKeys := make(map[string]string)

	switch doc.PatternId {
	case "p1_sequential":
		wf.Steps = []FillSlotsStep{
			{
				ID:       "gather",
				Skill:    "history-explorer",
				Task:     "Collect relevant historical context for the request",
				WithKeys: emptyKeys,
			},
			{
				ID:       "synthesize",
				Skill:    "summarize",
				Task:     BuildTask("Synthesize a grounded answer", doc.UserIntent),
				WithKeys: emptyKeys,
			},
		}

	case "p2_fan_out_merge":
		wf.Branches = []FillSlotsStep{
			{
				ID:       "context",
				Skill:    "history-explorer",
				Task:     "Collect prior related context",
				WithKeys: emptyKeys,
			},
			{
				ID:       "analysis",
				Skill:    "summarize",
				Task:     BuildTask("Generate focused analysis", doc.UserIntent),
				WithKeys: emptyKeys,
			},
		}
		wf.Merge = &FillSlotsStep{
			ID:       "merge",
			Skill:    "summarize",
			Task:     "Merge branch outputs into one coherent deliverable",
			WithKeys: emptyKeys,
		}
	case "p3_condition_gated":
		wf.Steps = []FillSlotsStep{
			{
				ID:       "intake",
				Skill:    "summarize",
				Task:     BuildTask("Extract constraints and missing information", doc.UserIntent),
				WithKeys: emptyKeys,
			},
			{
				ID:       "evidence",
				Skill:    "history-explorer",
				Task:     "Find relevant prior context when available",
				WithKeys: emptyKeys,
			},
			{
				ID:       "decision",
				Skill:    "summarize",
				Task:     "Produce final answer with caveats and next actions",
				WithKeys: emptyKeys,
			},
		}
	}

	bytes, err := json.Marshal(wf)
	if err != nil {
		return err.Error()
	}

	return string(bytes)
}
