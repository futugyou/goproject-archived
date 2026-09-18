package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/paho"
	"github.com/futugyou/openclaw/agent/tool/mqtt"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type MqttEventBridge struct {
	config         *core.MqttConfig
	logger         *slog.Logger
	inbound        chan<- core.InboundMessage
	lastGlobalEmit time.Time
	cooldowns      map[string]time.Time
}

func NewMqttEventBridge(
	config *core.MqttConfig,
	logger *slog.Logger,
	inbound chan<- core.InboundMessage,
) *MqttEventBridge {
	if logger == nil {
		logger = slog.Default()
	}
	return &MqttEventBridge{
		config:    config,
		logger:    logger,
		inbound:   inbound,
		cooldowns: map[string]time.Time{},
	}
}

func (m *MqttEventBridge) Execute(ctx context.Context) error {
	if !m.config.Enabled || !m.config.Events.Enabled {
		m.logger.Info("mqtt event bridge disabled")
		return nil
	}

	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("[MqttEventBridge] closing...")
			return nil
		default:
		}

		err := m.RunOnce(ctx)
		if err == nil {
			backoff = 1 * time.Second
			continue
		}

		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			m.logger.Info("[MqttEventBridge] closing...")
			return nil
		}

		m.logger.Warn("mqtt event bridge error; reconnecting",
			"delay_seconds", backoff.Seconds(),
			"error", err,
		)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			m.logger.Info("[MqttEventBridge] closing...")
			return nil
		case <-timer.C:
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (m *MqttEventBridge) RunOnce(ctx context.Context) error {
	disconnectChan := make(chan error, 1)
	receiveHandle := func(pr paho.PublishReceived) (bool, error) {
		return m.handleMqttMessage(ctx, pr.Packet)
	}

	client, err := mqtt.CreateMqttClient(
		ctx,
		*m.config,
		[]func(paho.PublishReceived) (bool, error){receiveHandle},
		func(err error) {
			select {
			case disconnectChan <- err:
			default:
			}
		},
	)
	if err != nil {
		return fmt.Errorf("connect mqtt failed: %w", err)
	}

	defer client.Disconnect(&paho.Disconnect{ReasonCode: 0})

	for _, sub := range m.config.Events.Subscriptions {
		if sub.Topic == "" {
			continue
		}

		if !core.GlobMatcherInstance.IsAllowed(m.config.Policy.AllowSubscribeTopicGlobs, m.config.Policy.DenySubscribeTopicGlobs, sub.Topic) {
			m.logger.Warn(fmt.Sprintf("MQTT subscription topic denied by policy: %s", sub.Topic))
			continue
		}

		qos := util.Clamp(sub.Qos, 0, 2)
		_, err = client.Subscribe(ctx, &paho.Subscribe{
			Subscriptions: []paho.SubscribeOptions{
				{Topic: sub.Topic, QoS: byte(qos)},
			},
		})
		if err != nil {
			return fmt.Errorf("subscribe topic %s failed: %w", sub.Topic, err)
		}
	}

	m.logger.Info(fmt.Sprintf("MQTT event bridge connected; subscribed to %d patterns.", len(m.config.Events.Subscriptions)))

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-disconnectChan:
		return fmt.Errorf("mqtt connection lost: %w", err)
	}
}

func (m *MqttEventBridge) handleMqttMessage(ctx context.Context, p *paho.Publish) (bool, error) {
	if len(p.Payload) > m.config.MaxPayloadBytes {
		return false, fmt.Errorf("Error: payload exceeds configured limit (%d bytes)", m.config.MaxPayloadBytes)
	}

	payload := string(p.Payload)
	mqtt.SetPayload(p.Topic, payload)
	var sub = m.findSubscriptionForTopic(p.Topic)
	if sub == nil {
		return true, nil
	}

	now := time.Now()
	if !m.tryConsumeCooldown(sub, now) {
		return true, nil
	}

	template := sub.PromptTemplate
	if template == "" {
		template = fmt.Sprintf("MQTT message on %s: %s", p.Topic, payload)
	}

	replacer := strings.NewReplacer(
		"{topic}", p.Topic,
		"{payload}", payload,
	)

	text := replacer.Replace(template)

	var msg = core.InboundMessage{
		ChannelId: m.config.Events.ChannelId,
		SessionId: m.config.Events.SessionId,
		SenderId:  "system",
		Text:      text,
	}

	select {
	case m.inbound <- msg:
	case <-ctx.Done():
		m.logger.Warn("[MqttEventBridge] failed to send event", "error", ctx.Err())
	}

	return true, nil
}

func (m *MqttEventBridge) findSubscriptionForTopic(topic string) *core.MqttSubscriptionConfig {
	for _, sub := range m.config.Events.Subscriptions {
		if sub.Topic == "" {
			continue
		}

		if mqtt.Match(sub.Topic, topic) {
			return &sub
		}
	}

	return nil
}

func (m *MqttEventBridge) tryConsumeCooldown(sub *core.MqttSubscriptionConfig, now time.Time) bool {
	if sub.CooldownSeconds <= 0 {
		return true
	}

	var cooldown = time.Duration(max(0, sub.CooldownSeconds)) * time.Second
	var key = fmt.Sprintf("sub:%s", sub.Topic)
	if last, ok := m.cooldowns[key]; ok && now.Sub(last) < cooldown {
		return false
	}

	m.cooldowns[key] = now
	return true
}

func (m *MqttEventBridge) Name() string {
	return "MqttEventBridge"
}
