package core

import "time"

type InboundMessage struct {
	ChannelId               string    `json:"channel_id"`
	SenderId                string    `json:"sender_id"`
	AccountId               *string   `json:"account_id,omitempty"`
	SessionId               *string   `json:"session_id,omitempty"`
	CronJobName             *string   `json:"cron_job_name,omitempty"`
	AutomationRunId         *string   `json:"automation_run_id,omitempty"`
	AutomationTriggerSource *string   `json:"automation_trigger_source,omitempty"`
	Type                    *string   `json:"type,omitempty"`
	Text                    string    `json:"text"`
	SenderName              *string   `json:"sender_name,omitempty"`
	MessageId               *string   `json:"message_id,omitempty"`
	ReplyToMessageId        *string   `json:"reply_to_message_id,omitempty"`
	RequestId               *string   `json:"request_id,omitempty"`
	SurfaceId               *string   `json:"surface_id,omitempty"`
	ComponentId             *string   `json:"component_id,omitempty"`
	Event                   *string   `json:"event,omitempty"`
	ValueJson               *string   `json:"value_json,omitempty"`
	Sequence                *int64    `json:"sequence,omitempty"`
	IsSystem                bool      `json:"is_system"`
	Subject                 *string   `json:"subject,omitempty"`
	ApprovalId              *string   `json:"approval_id,omitempty"`
	Approved                *bool     `json:"approved,omitempty"`
	ReceivedAt              time.Time `json:"received_at"`

	// Group chat fields
	IsGroup      bool     `json:"is_group"`
	GroupId      *string  `json:"group_id,omitempty"`
	GroupName    *string  `json:"group_name,omitempty"`
	MentionedIds []string `json:"mentioned_ids,omitempty"`

	// Media fields
	MediaType     *string `json:"media_type,omitempty"`
	MediaUrl      *string `json:"media_url,omitempty"`
	MediaMimeType *string `json:"media_mime_type,omitempty"`
	MediaFileName *string `json:"media_file_name,omitempty"`
}

func DefaultInboundMessage() InboundMessage {
	return InboundMessage{
		ReceivedAt: time.Now().UTC(),
	}
}

// OutboundMessage 发送出去的消息体
type OutboundMessage struct {
	ChannelId        string  `json:"channel_id"`
	RecipientId      string  `json:"recipient_id"`
	Text             string  `json:"text"`
	AccountId        *string `json:"account_id,omitempty"`
	SessionId        *string `json:"session_id,omitempty"`
	CronJobName      *string `json:"cron_job_name,omitempty"`
	AutomationRunId  *string `json:"automation_run_id,omitempty"`
	Subject          *string `json:"subject,omitempty"`
	ReplyToMessageId *string `json:"reply_to_message_id,omitempty"`
}
