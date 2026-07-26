package agent

import (
	"slices"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type EventInfo struct {
	EventType    string
	EntityId     string
	FromState    string
	ToState      string
	FriendlyName string
}

var HomeAssistantRuleEngineInstance = &HomeAssistantRuleEngine{}

type HomeAssistantRuleEngine struct{}

func (h *HomeAssistantRuleEngine) isRuleAllowedDay(rule *core.HomeAssistantEventRule, localNow time.Time) bool {
	if rule == nil || len(rule.DaysOfWeek) == 0 {
		return true
	}

	abbrev := localNow.Weekday().String()[:3]
	for _, d := range rule.DaysOfWeek {
		if strings.EqualFold(d, abbrev) {
			return true
		}
	}

	return false
}

func (h *HomeAssistantRuleEngine) isRuleInLocalWindow(rule *core.HomeAssistantEventRule, localNow time.Time) bool {
	if rule == nil || strings.TrimSpace(rule.BetweenLocalStart) == "" || strings.TrimSpace(rule.BetweenLocalEnd) == "" {
		return true
	}

	startTime, err1 := time.Parse("15:04", rule.BetweenLocalStart)
	endTime, err2 := time.Parse("15:04", rule.BetweenLocalEnd)
	if err1 != nil || err2 != nil {
		return true
	}

	start := startTime.Hour()*60 + startTime.Minute()
	end := endTime.Hour()*60 + endTime.Minute()
	now := localNow.Hour()*60 + localNow.Minute()

	// （e.g., 08:00 - 18:00）
	if start <= end {
		return now >= start && now <= end
	}

	// Overnight window (e.g., 22:00–06:00)
	return now >= start || now <= end
}

func (h *HomeAssistantRuleEngine) isRuleMatch(rule *core.HomeAssistantEventRule, info *EventInfo) bool {
	if rule == nil || info == nil {
		return false
	}

	if len(rule.EntityIdGlobs) > 0 {
		hasMatch := slices.ContainsFunc(rule.EntityIdGlobs, func(g string) bool {
			return core.GlobMatcherInstance.IsMatch(g, info.EntityId)
		})
		if !hasMatch {
			return false
		}
	}

	if !util.IsBlank(rule.FromState) && !strings.EqualFold(rule.FromState, info.FromState) {
		return false
	}

	if !util.IsBlank(rule.ToState) && !strings.EqualFold(rule.ToState, info.ToState) {
		return false
	}

	return true
}

func (h *HomeAssistantRuleEngine) Render(cfg *core.HomeAssistantEventsConfig, rule *core.HomeAssistantEventRule, info *EventInfo) string {
	template := ""
	if rule != nil && !util.IsBlank(rule.PromptTemplate) {
		template = rule.PromptTemplate
	} else if cfg != nil {
		template = cfg.PromptTemplate
	}
	if info == nil {
		return template
	}

	replacer := strings.NewReplacer(
		"{event_type}", info.EventType,
		"{entity_id}", info.EntityId,
		"{from_state}", info.FromState,
		"{to_state}", info.ToState,
		"{friendly_name}", info.FriendlyName,
	)

	return replacer.Replace(template)
}

func (h *HomeAssistantRuleEngine) SelectRule(cfg *core.HomeAssistantEventsConfig, info *EventInfo, localNow time.Time) *core.HomeAssistantEventRule {
	if cfg == nil {
		return nil
	}

	for _, rule := range cfg.Rules {
		if !h.isRuleMatch(&rule, info) {
			continue
		}

		if !h.isRuleInLocalWindow(&rule, localNow) {
			continue
		}

		if !h.isRuleAllowedDay(&rule, localNow) {
			continue
		}

		return &rule
	}
	return nil
}
