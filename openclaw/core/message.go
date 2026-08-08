package core

import "time"

type InboundMessage struct {
	ChannelId               string    `json:"channel_id"`
	SenderId                string    `json:"sender_id"`
	AccountId               string    `json:"account_id"`
	SessionId               string    `json:"session_id"`
	CronJobName             string    `json:"cron_job_name"`
	AutomationRunId         string    `json:"automation_run_id"`
	AutomationTriggerSource string    `json:"automation_trigger_source"`
	Type                    string    `json:"type"`
	Text                    string    `json:"text"`
	SenderName              string    `json:"sender_name"`
	MessageId               string    `json:"message_id"`
	ReplyToMessageId        string    `json:"reply_to_message_id"`
	RequestId               string    `json:"request_id"`
	SurfaceId               string    `json:"surface_id"`
	ComponentId             string    `json:"component_id"`
	Event                   string    `json:"event"`
	ValueJson               string    `json:"value_json"`
	Sequence                *int64    `json:"sequence"`
	IsSystem                bool      `json:"is_system"`
	Subject                 string    `json:"subject"`
	ApprovalId              string    `json:"approval_id"`
	Approved                *bool     `json:"approved"`
	ReceivedAt              time.Time `json:"received_at"`

	// Group chat fields
	IsGroup      bool     `json:"is_group"`
	GroupId      string   `json:"group_id"`
	GroupName    string   `json:"group_name"`
	MentionedIds []string `json:"mentioned_ids,omitempty"`

	// Media fields
	MediaType     string `json:"media_type"`
	MediaUrl      string `json:"media_url"`
	MediaMimeType string `json:"media_mime_type"`
	MediaFileName string `json:"media_file_name"`

	BackgroundRunId                string            `json:"background_run_id"`
	BackgroundContinuationSequence int               `json:"background_continuation_sequence"`
	AuthenticatedUserId            string            `json:"authenticated_user_id"`
	Attachments                    []MediaAttachment `json:"attachments,omitempty"`
}

func DefaultInboundMessage() InboundMessage {
	return InboundMessage{
		ReceivedAt: time.Now().UTC(),
	}
}

// OutboundMessage 发送出去的消息体
type OutboundMessage struct {
	ChannelId        string `json:"channel_id"`
	RecipientId      string `json:"recipient_id"`
	Text             string `json:"text"`
	AccountId        string `json:"account_id,omitempty"`
	SessionId        string `json:"session_id,omitempty"`
	CronJobName      string `json:"cron_job_name,omitempty"`
	AutomationRunId  string `json:"automation_run_id,omitempty"`
	Subject          string `json:"subject,omitempty"`
	ReplyToMessageId string `json:"reply_to_message_id,omitempty"`
}

type MediaAttachment struct {
	MediaType string `json:"media_type"`
	Url       string `json:"url"`
	MimeType  string `json:"mime_type"`
	FileName  string `json:"file_name"`
}
