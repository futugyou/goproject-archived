package whatsapp

import (
	"context"

	"github.com/futugyou/openclaw/core"
)

type WhatsAppWorkerInboundMessage struct {
	SenderId         string   `json:"senderId"`
	AccountId        string   `json:"accountId"`
	SenderName       string   `json:"senderName"`
	Text             string   `json:"text"`
	SessionId        string   `json:"sessionId"`
	MessageId        string   `json:"messageId"`
	ReplyToMessageId string   `json:"replyToMessageId"`
	IsGroup          bool     `json:"isGroup"`
	GroupId          string   `json:"groupId"`
	GroupName        string   `json:"groupName"`
	MentionedIds     []string `json:"mentionedIds"`
	MediaType        string   `json:"mediaType"`
	MediaUrl         string   `json:"mediaUrl"`
	MediaMimeType    string   `json:"mediaMimeType"`
	MediaFileName    string   `json:"mediaFileName"`
}

type WhatsAppWorkerStartResult struct {
	SelfIds []string
}

type MessageReceived func(WhatsAppWorkerInboundMessage) error
type AuthEventReceived func(core.BridgeChannelAuthEvent) error

type IWhatsAppWorkerEngine interface {
	Start(ctx context.Context) (*WhatsAppWorkerStartResult, error)
	Stop(ctx context.Context) error
	Send(ctx context.Context, request core.BridgeChannelSendRequest) error
	SendTyping(ctx context.Context, request core.BridgeChannelTypingRequest) error
	SendReadReceipt(ctx context.Context, request core.BridgeChannelReceiptRequest) error
	SendReaction(ctx context.Context, request core.BridgeChannelReactionRequest) error
	DebugSimulateInbound(ctx context.Context, payload []byte) error
	DebugEmitAuthEvent(ctx context.Context, payload []byte) error
	DebugGetState() any
}
