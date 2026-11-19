package whatsapp

import (
	"context"

	"github.com/futugyou/openclaw/core"
)

type WhatsAppWorkerInboundMessage struct {
	SenderId         string
	AccountId        string
	SenderName       string
	Text             string
	SessionId        string
	MessageId        string
	ReplyToMessageId string
	IsGroup          bool
	GroupId          string
	GroupName        string
	MentionedIds     []string
	MediaType        string
	MediaUrl         string
	MediaMimeType    string
	MediaFileName    string
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
	DebugSimulateInbound(ctx context.Context, payload any) error
	DebugEmitAuthEvent(ctx context.Context, payload any) error
	DebugGetState() any
}
