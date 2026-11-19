package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/futugyou/openclaw/core"
)

func CreateEngine(config core.WhatsAppFirstPartyWorkerConfig) (IWhatsAppWorkerEngine, error) {
	if config.Driver == "simulated" {
		return NewSimulatedWhatsAppWorkerEngine(config), nil
	}

	return nil, fmt.Errorf("The in-process WhatsApp worker only supports Driver='simulated'. Driver='%s' must be launched through the first-party worker host.", config.Driver)
}

func EnsureChannel(channelId string) error {
	if channelId != "whatsapp" {
		return fmt.Errorf("Unsupported channel %s", channelId)
	}
	return nil
}

type WhatsAppWorkerService struct {
	engine       IWhatsAppWorkerEngine
	config       core.WhatsAppFirstPartyWorkerConfig
	Notification func(core.BridgeNotification) error
}

func (w *WhatsAppWorkerService) ensureEngine() (IWhatsAppWorkerEngine, error) {
	if w.engine != nil {
		return w.engine, nil
	}

	return nil, errors.New("Worker was not initialized.")
}

func (w *WhatsAppWorkerService) handleAuthEvent(evt core.BridgeChannelAuthEvent) error {
	if w.Notification == nil {
		return nil
	}

	data := map[string]any{}
	data["channelId"] = "whatsapp"
	data["state"] = evt.State
	data["data"] = evt.Data
	data["accountId"] = evt.AccountId
	data["updatedAtUtc"] = evt.UpdatedAtUtc

	payload, _ := json.Marshal(data)
	return w.Notification(core.BridgeNotification{
		Notification: "channel_auth_event",
		Params:       payload,
	})
}

func (w *WhatsAppWorkerService) handleMessage(message WhatsAppWorkerInboundMessage) error {
	if w.Notification == nil {
		return nil
	}

	data := map[string]any{}
	data["channelId"] = "whatsapp"
	data["senderId"] = message.SenderId
	data["accountId"] = message.AccountId
	data["senderName"] = message.SenderName
	data["text"] = message.Text
	data["sessionId"] = message.SessionId
	data["messageId"] = message.MessageId
	data["replyToMessageId"] = message.ReplyToMessageId
	data["isGroup"] = message.IsGroup
	data["groupId"] = message.GroupId
	data["groupName"] = message.GroupName
	data["mentionedIds"] = message.MentionedIds
	data["mediaType"] = message.MediaType
	data["mediaUrl"] = message.MediaUrl
	data["mediaMimeType"] = message.MediaMimeType
	data["mediaFileName"] = message.MediaFileName

	payload, _ := json.Marshal(data)
	return w.Notification(core.BridgeNotification{
		Notification: "channel_message",
		Params:       payload,
	})
}

func (w *WhatsAppWorkerService) Shutdown() any {
	if w.engine != nil {
		w.engine.Stop(context.Background())
	}

	return struct{ Shutdown bool }{Shutdown: true}
}

func (w *WhatsAppWorkerService) DebugGetState() any {
	e, err := w.ensureEngine()
	if err != nil {
		return nil
	}
	return e.DebugGetState()
}

func (w *WhatsAppWorkerService) DebugEmitAuthEvent(payload []byte) any {
	e, err := w.ensureEngine()
	if err != nil {
		return struct{ Accepted bool }{Accepted: false}
	}
	if err := e.DebugEmitAuthEvent(context.Background(), payload); err != nil {
		return struct{ Accepted bool }{Accepted: false}
	}
	return struct{ Accepted bool }{Accepted: true}
}

func (w *WhatsAppWorkerService) DebugSimulateInbound(payload []byte) any {
	e, err := w.ensureEngine()
	if err != nil {
		return struct{ Accepted bool }{Accepted: false}
	}
	if err := e.DebugSimulateInbound(context.Background(), payload); err != nil {
		return struct{ Accepted bool }{Accepted: false}
	}
	return struct{ Accepted bool }{Accepted: true}
}

func (w *WhatsAppWorkerService) Initialize(request core.BridgeInitRequest) (*core.BridgeInitResult, error) {
	var config core.WhatsAppFirstPartyWorkerConfig
	if err := json.Unmarshal(request.Config, &config); err != nil {
		config = core.WhatsAppFirstPartyWorkerConfig{}
	}
	w.config = config
	engine, err := CreateEngine(w.config)
	if err != nil {
		return nil, err
	}
	w.engine = engine
	w.engine.SetMessageReceived(w.handleMessage)
	w.engine.SetAuthEventReceived(w.handleAuthEvent)

	return &core.BridgeInitResult{
		Channels:     []core.BridgeChannelRegistration{{Id: "whatsapp"}},
		Capabilities: []string{"channels"},
	}, nil
}

func (w *WhatsAppWorkerService) Start(request core.BridgeChannelControlRequest) (any, error) {
	if err := EnsureChannel(request.ChannelId); err != nil {
		return nil, err
	}
	engine, err := w.ensureEngine()
	if err != nil {
		return nil, err
	}

	result, err := engine.Start(context.Background())
	if err != nil {
		return nil, err
	}

	data := map[string]any{}
	data["selfIds"] = result.SelfIds
	data["selfId"] = ""
	if len(result.SelfIds) > 0 {
		data["selfId"] = result.SelfIds[0]
	}

	return data, nil
}

func (w *WhatsAppWorkerService) Stop(request core.BridgeChannelControlRequest) (any, error) {
	if err := EnsureChannel(request.ChannelId); err != nil {
		return nil, err
	}
	engine, err := w.ensureEngine()
	if err != nil {
		return nil, err
	}

	if err := engine.Stop(context.Background()); err != nil {
		return nil, err
	}

	return struct{ Stopped bool }{Stopped: true}, nil
}

func (w *WhatsAppWorkerService) Send(request core.BridgeChannelSendRequest) (any, error) {
	if err := EnsureChannel(request.ChannelId); err != nil {
		return nil, err
	}
	engine, err := w.ensureEngine()
	if err != nil {
		return nil, err
	}

	if err := engine.Send(context.Background(), request); err != nil {
		return nil, err
	}

	return struct{ Send bool }{Send: true}, nil
}

func (w *WhatsAppWorkerService) SendTyping(request core.BridgeChannelTypingRequest) (any, error) {
	if err := EnsureChannel(request.ChannelId); err != nil {
		return nil, err
	}
	engine, err := w.ensureEngine()
	if err != nil {
		return nil, err
	}

	if err := engine.SendTyping(context.Background(), request); err != nil {
		return nil, err
	}

	return struct{ Accepted bool }{Accepted: true}, nil
}

func (w *WhatsAppWorkerService) SendReadReceipt(request core.BridgeChannelReceiptRequest) (any, error) {
	if err := EnsureChannel(request.ChannelId); err != nil {
		return nil, err
	}
	engine, err := w.ensureEngine()
	if err != nil {
		return nil, err
	}

	if err := engine.SendReadReceipt(context.Background(), request); err != nil {
		return nil, err
	}

	return struct{ Accepted bool }{Accepted: true}, nil
}

func (w *WhatsAppWorkerService) SendReaction(request core.BridgeChannelReactionRequest) (any, error) {
	if err := EnsureChannel(request.ChannelId); err != nil {
		return nil, err
	}
	engine, err := w.ensureEngine()
	if err != nil {
		return nil, err
	}

	if err := engine.SendReaction(context.Background(), request); err != nil {
		return nil, err
	}

	return struct{ Accepted bool }{Accepted: true}, nil
}
