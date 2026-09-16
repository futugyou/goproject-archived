package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"

	"github.com/eclipse/paho.golang/paho"
)

type MqttPublishTool struct {
	config        core.MqttConfig
	toolingConfig *core.ToolingConfig
}

func NewMqttPublishTool(config core.MqttConfig, toolingConfig *core.ToolingConfig) *MqttPublishTool {
	if toolingConfig == nil {
		toolingConfig = &core.ToolingConfig{}
	}
	return &MqttPublishTool{config: config, toolingConfig: toolingConfig}
}

func (a *MqttPublishTool) Name() string {
	return "mqtt_publish"
}

func (a *MqttPublishTool) Description() string {
	return "Publish MQTT messages (write operations). Use with care."
}

func (a *MqttPublishTool) ParameterSchema() string {
	return `{
          "type": "object",
          "properties": {
            "op": { "type": "string", "enum": ["publish"] },
            "topic": { "type": "string" },
            "payload": { "type": "string" },
            "qos": { "type": "integer", "minimum": 0, "maximum": 2, "default": 0 },
            "retain": { "type": "boolean", "default": false }
          },
          "required": ["op","topic","payload"]
        }`
}

type PublishDto struct {
	Op      string `json:"op"`
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
	Qos     int    `json:"qos"`
	Retain  bool   `json:"retain"`
}

func (a *MqttPublishTool) Execute(ctx context.Context, argumentsJson string) string {
	if a.toolingConfig.ReadOnlyMode {
		return "Error: mqtt_publish is disabled because Tooling.ReadOnlyMode is enabled."
	}

	var dto PublishDto

	if err := json.Unmarshal([]byte(argumentsJson), &dto); err != nil {
		return err.Error()
	}

	if dto.Op != "publish" {
		return fmt.Sprintf("unknown op %s", dto.Op)
	}

	if dto.Topic == "" {
		return "Error: topic is required"
	}

	if core.GlobMatcherInstance.IsAllowed(a.config.Policy.AllowPublishTopicGlobs, a.config.Policy.DenyPublishTopicGlobs, dto.Topic) {
		return fmt.Sprintf("Error: Publish to topic '%s' is not allowed by policy.", dto.Topic)
	}

	dto.Qos = util.Clamp(dto.Qos, 0, 2)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(1, a.config.TimeoutSeconds))*time.Second)
	defer cancel()

	client, err := CreateMqttClient(ctx, a.config, nil)
	if err != nil {
		return err.Error()
	}

	defer client.Disconnect(&paho.Disconnect{ReasonCode: 0})

	_, err = client.Publish(ctx, &paho.Publish{
		QoS:     byte(dto.Qos),
		Topic:   dto.Topic,
		Payload: []byte(dto.Payload),
		Retain:  dto.Retain,
	})
	if err != nil {
		return err.Error()
	}

	return "OK"
}
