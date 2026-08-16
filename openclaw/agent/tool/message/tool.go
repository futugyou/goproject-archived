package message

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/futugyou/openclaw/core"
)

type MessageTool struct {
	outbound chan<- core.OutboundMessage
}

func New(pipeline *core.MessagePipeline) *MessageTool {
	return &MessageTool{outbound: pipeline.OutboundWriter()}
}

func (a *MessageTool) Name() string {
	return "message"
}

func (a *MessageTool) Description() string {
	return "Send a message to a specific channel and recipient. Use to communicate across channels."
}

func (a *MessageTool) ParameterSchema() string {
	return `
{
	"type": "object",
	"properties": {
		"channel_id": {
			"type": "string",
			"description": "Target channel (e.g. 'telegram', 'slack', 'discord', 'sms', 'email', 'websocket')"
		},
		"recipient_id": {
			"type": "string",
			"description": "Recipient identifier (chat ID, user ID, phone number, etc.)"
		},
		"text": {
			"type": "string",
			"description": "Message text to send"
		},
		"reply_to": {
			"type": "string",
			"description": "Optional message ID to reply to"
		}
	},
	"required": ["channel_id", "recipient_id", "text"]
} `
}

type MessageModel struct {
	ChannelId   string `json:"channel_id"`
	RecipientId string `json:"recipient_id"`
	Text        string `json:"text"`
	ReplyTo     string `json:"reply_to"`
}

func (a *MessageTool) Execute(ctx context.Context, argumentsJson string) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	var model MessageModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if model.ChannelId == "" {
		return "Error: channel_id is required."
	}
	if model.RecipientId == "" {
		return "Error: recipient_id is required."
	}
	if model.Text == "" {
		return "Error: text is required."
	}

	msg := core.OutboundMessage{
		ChannelId:        model.ChannelId,
		RecipientId:      model.RecipientId,
		Text:             model.Text,
		ReplyToMessageId: model.ReplyTo,
	}
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	case a.outbound <- msg:
	}

	return fmt.Sprintf("Message queued for delivery to %s:%s.", model.ChannelId, model.RecipientId)
}
