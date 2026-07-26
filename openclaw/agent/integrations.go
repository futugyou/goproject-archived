package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
	"github.com/gorilla/websocket"
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

type HAMessage struct {
	ID      int             `json:"id,omitempty"`
	Type    string          `json:"type"` // type auth_required, auth_ok, result, event
	Success bool            `json:"success,omitempty"`
	Message string          `json:"message,omitempty"`
	Event   json.RawMessage `json:"event,omitempty"`
}

type HAEventEnvelope struct {
	EventType string      `json:"event_type"` // eg. "state_changed"
	Data      HAEventData `json:"data"`
	Origin    string      `json:"origin"`
	TimeFired time.Time   `json:"time_fired"`
}

type HAEventData struct {
	EntityID string   `json:"entity_id"`
	OldState *HAState `json:"old_state"`
	NewState *HAState `json:"new_state"`
}

type HAState struct {
	State       string            `json:"state"`
	Attributes  HAStateAttributes `json:"attributes"`
	LastChanged time.Time         `json:"last_changed"`
}

type HAStateAttributes struct {
	FriendlyName string `json:"friendly_name"`
}

type HomeAssistantEventBridge struct {
	config         *core.HomeAssistantConfig
	logger         *slog.Logger
	inbound        chan<- core.InboundMessage
	lastGlobalEmit time.Time
	cooldowns      map[string]time.Time
}

func NewHomeAssistantEventBridge(
	config *core.HomeAssistantConfig,
	logger *slog.Logger,
	inbound chan<- core.InboundMessage,
) *HomeAssistantEventBridge {
	return &HomeAssistantEventBridge{
		config:    config,
		logger:    logger,
		inbound:   inbound,
		cooldowns: map[string]time.Time{},
	}
}

func (b *HomeAssistantEventBridge) RunOnce(ctx context.Context) error {
	url, err := b.buildWebSocketUrl()
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	var first HAMessage
	if err := conn.ReadJSON(&first); err != nil {
		return err
	}
	if first.Type != "auth_required" {
		return fmt.Errorf("expected auth_required, got %s", first.Type)
	}

	var token = core.SecretResolverInstance.Resolve(b.config.TokenRef)
	authReq := map[string]string{
		"type":         "auth",
		"access_token": token,
	}
	if err := conn.WriteJSON(authReq); err != nil {
		return err
	}

	var authReply HAMessage
	if err := conn.ReadJSON(&authReply); err != nil {
		return err
	}
	if authReply.Type != "auth_ok" {
		return fmt.Errorf("auth failed: %s %s", authReply.Type, authReply.Message)
	}

	var reqID int
	for _, eventType := range b.config.Events.SubscribeEventTypes {
		reqID++
		subReq := map[string]any{
			"id":         reqID,
			"type":       "subscribe_events",
			"event_type": eventType,
		}
		if err := conn.WriteJSON(subReq); err != nil {
			return err
		}

		if err := b.waitForResult(conn, reqID); err != nil {
			return fmt.Errorf("subscribe %s failed: %w", eventType, err)
		}
	}

	for {
		var msg HAMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}

		if msg.Type != "event" || len(msg.Event) == 0 {
			continue
		}

		var eventEnv HAEventEnvelope
		if err := json.Unmarshal(msg.Event, &eventEnv); err != nil {
			continue
		}

		b.handleEvent(ctx, &eventEnv)
	}
}

func (b *HomeAssistantEventBridge) buildWebSocketUrl() (string, error) {
	baseURL, err := url.Parse(b.config.BaseURL)
	if err != nil || (baseURL != nil && (!baseURL.IsAbs() || (baseURL.Scheme != "http" && baseURL.Scheme != "https"))) {
		return "", fmt.Errorf("invalid Home Assistant BaseUrl: %s", b.config.BaseURL)
	}

	var scheme = "ws"
	if baseURL.Scheme == "https" {
		scheme = "wss"
	}

	u := *baseURL
	u.Scheme = scheme
	u.Path = "/api/websocket"
	u.RawQuery = ""

	return u.String(), nil
}

func (b *HomeAssistantEventBridge) waitForResult(conn *websocket.Conn, targetID int) error {
	for {
		var msg HAMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		if msg.Type == "result" && msg.ID == targetID {
			if !msg.Success {
				return fmt.Errorf("HA returned error: %s", msg.Message)
			}
			return nil
		}
	}
}

func (b *HomeAssistantEventBridge) tryConsumeCooldown(rule *core.HomeAssistantEventRule, now time.Time) bool {
	cdSeconds := max(0, b.config.Events.GlobalCooldownSeconds)
	globalCooldown := time.Duration(cdSeconds) * time.Second

	if globalCooldown > 0 && now.Sub(b.lastGlobalEmit) < globalCooldown {
		return false
	}

	if rule == nil {
		b.lastGlobalEmit = now
		return true
	}

	var cooldown = time.Duration(max(0, rule.CooldownSeconds)) * time.Second
	if cooldown <= 0 {
		b.lastGlobalEmit = now
		return true
	}

	var key = fmt.Sprintf("rule:%s", rule.Name)
	last, ok := b.cooldowns[key]
	if ok && now.Sub(last) < cooldown {
		return false
	}

	b.cooldowns[key] = now
	b.lastGlobalEmit = now
	return true
}

func (b *HomeAssistantEventBridge) handleEvent(ctx context.Context, ev *HAEventEnvelope) {
	entityId := ev.Data.EntityID
	fromState := ""
	toState := ""
	friendlyName := ""

	if ev.Data.OldState != nil {
		fromState = ev.Data.OldState.State
	}
	if ev.Data.NewState != nil {
		toState = ev.Data.NewState.State
		friendlyName = ev.Data.NewState.Attributes.FriendlyName
	}

	if !util.IsBlank(entityId) {
		if !core.GlobMatcherInstance.IsAllowed(b.config.Events.AllowEntityIdGlobs, b.config.Events.DenyEntityIdGlobs, entityId) {
			return
		}
	}

	now := time.Now().UTC()
	info := EventInfo{
		EventType:    ev.EventType,
		EntityId:     entityId,
		FromState:    fromState,
		ToState:      toState,
		FriendlyName: friendlyName,
	}
	matchedRule := HomeAssistantRuleEngineInstance.SelectRule(&b.config.Events, &info, now)
	if matchedRule == nil && !b.config.Events.EmitAllMatchingEvents {
		return
	}

	if !b.tryConsumeCooldown(matchedRule, now) {
		return
	}

	var text = HomeAssistantRuleEngineInstance.Render(&b.config.Events, matchedRule, &info)
	var msg = core.InboundMessage{
		ChannelId: b.config.Events.ChannelId,
		SessionId: b.config.Events.SessionId,
		SenderId:  "system",
		Text:      text,
	}

	select {
	case b.inbound <- msg:
	case <-ctx.Done():
		b.logger.Warn("[HomeAssistantEventBridge] failed to send event", "error", ctx.Err())
	}
}
