package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/paho"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type MqttTool struct {
	config core.MqttConfig
}

func NewMqttTool(config core.MqttConfig) *MqttTool {
	return &MqttTool{config: config}
}

func (a *MqttTool) Name() string {
	return "mqtt"
}

func (a *MqttTool) Description() string {
	return "Subscribe to MQTT topics (read-only). Use subscribe_once for a single message or get_last when event bridge is enabled."
}

func (a *MqttTool) ParameterSchema() string {
	return ` {
          "type": "object",
          "properties": {
            "op": { "type": "string", "enum": ["subscribe_once","get_last"] },
            "topic": { "type": "string" },
            "timeout_ms": { "type": "integer", "default": 5000 },
            "qos": { "type": "integer", "minimum": 0, "maximum": 2, "default": 0 }
          },
          "required": ["op","topic"]
        }`
}

type SubscribeDto struct {
	Op        string `json:"op"`
	Topic     string `json:"topic"`
	TimeoutMs int    `json:"timeout_ms"`
	Qos       int    `json:"qos"`
}

func (a *MqttTool) Execute(ctx context.Context, argumentsJson string) string {
	var dto SubscribeDto

	if err := json.Unmarshal([]byte(argumentsJson), &dto); err != nil {
		return err.Error()
	}

	if dto.Topic == "" {
		return "Error: topic is required"
	}

	switch dto.Op {
	case "subscribe_once":
		return a.subscribeOnce(ctx, dto)
	case "get_last":
		return a.getLast(ctx, dto)
	default:
		return fmt.Sprintf("Error: Unknown op '%s'.", dto.Op)
	}
}

func (a *MqttTool) getLast(_ context.Context, dto SubscribeDto) string {
	if strings.Contains(dto.Topic, "*") {
		var matches = FindByGlob(dto.Topic)
		if len(matches) == 0 {
			return "No cached messages matched."
		}

		sb := strings.Builder{}
		for i := 0; i < min(10, len(matches)); i++ {
			fmt.Fprintf(&sb, "topic: %s\n", matches[i].Topic)
			fmt.Fprintf(&sb, "received_at: %s\n", matches[i].ReceivedAt.Format(time.RFC3339Nano))
			sb.WriteString(matches[i].Payload)
			sb.WriteString("\n\n")
		}

		return util.TrimEnd(sb.String())
	}

	payload, receivedAt, found := GetPayload(dto.Topic)
	if !found {
		return "No cached message for topic. Enable Mqtt.Events.Enabled to cache subscriptions."
	}

	return fmt.Sprintf("topic: %s\nreceived_at: %s\n%s", dto.Topic, receivedAt.Format(time.RFC3339Nano), *payload)
}

type Result struct {
	Topic   string
	Payload string
	Err     error
}

func (a *MqttTool) subscribeOnce(ctx context.Context, dto SubscribeDto) string {
	if core.GlobMatcherInstance.IsAllowed(a.config.Policy.AllowSubscribeTopicGlobs, a.config.Policy.DenySubscribeTopicGlobs, dto.Topic) {
		return fmt.Sprintf("Error: Subscribe to topic '%s' is not allowed by policy.", dto.Topic)
	}

	qos := util.Clamp(dto.Qos, 0, 2)
	var maxPayloadBytes int64 = max(0, int64(a.config.MaxPayloadBytes))
	timeoutMs := util.Clamp(dto.TimeoutMs, 100, 120_000)
	timeout := time.Duration(timeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	msgChan := make(chan Result, 1)

	receiveHandle := func(pr paho.PublishReceived) (bool, error) {
		p := pr.Packet
		payloadLen := int64(len(p.Payload))

		var res Result
		if payloadLen > maxPayloadBytes {
			res = Result{
				Topic: p.Topic,
				Err:   fmt.Errorf("Error: payload exceeds configured limit (%d bytes)", maxPayloadBytes),
			}
		} else {
			res = Result{
				Topic:   p.Topic,
				Payload: string(p.Payload),
			}
		}

		select {
		case msgChan <- res:
		default:
		}
		return true, nil
	}

	client, err := CreateMqttClient(ctx, a.config, []func(paho.PublishReceived) (bool, error){receiveHandle}, nil)
	if err != nil {
		return err.Error()
	}

	defer client.Disconnect(&paho.Disconnect{ReasonCode: 0})

	_, err = client.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{Topic: dto.Topic, QoS: byte(qos)},
		},
	})
	if err != nil {
		return fmt.Sprintf("subscribe failed: %s", err.Error())
	}

	select {
	case res := <-msgChan:
		if res.Err != nil {
			return fmt.Sprintf("topic: %s\n%s", res.Topic, res.Err.Error())
		}
		return fmt.Sprintf("topic: %s\npayload: %s", res.Topic, res.Payload)

	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Sprintf("Error: timeout (%d ms) reached before receiving message", timeoutMs)
		}
		return ctx.Err().Error()
	}
}
