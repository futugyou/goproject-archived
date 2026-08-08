package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type BridgedPluginTool struct {
	bridge   *PluginBridgeProcess
	pluginId string

	name            string
	description     string
	parameterSchema string
	OutputSchema    string
	Optional        bool
}

func NewBridgedPluginTool(bridge *PluginBridgeProcess, pluginId string, registration core.PluginToolRegistration) *BridgedPluginTool {
	tool := &BridgedPluginTool{
		bridge:          bridge,
		pluginId:        pluginId,
		name:            registration.Name,
		description:     registration.Description,
		Optional:        registration.Optional,
		parameterSchema: registration.GetParameterSchema(),
		OutputSchema:    registration.GetOutputSchema(),
	}
	return tool
}

func (b *BridgedPluginTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	return b.bridge.ExecuteTool(ctx, b.name, argumentsJson)
}

func (b *BridgedPluginTool) Name() string {
	return b.name
}

func (b *BridgedPluginTool) Description() string {
	return b.description
}

func (b *BridgedPluginTool) ParameterSchema() string {
	return b.parameterSchema
}

type BridgedChannelAdapter struct {
	mu        sync.RWMutex
	bridge    *PluginBridgeProcess
	logger    *slog.Logger
	channelId string
	selfID    *string
	selfIDs   []string

	OnMessageReceived func(ctx context.Context, msg *core.InboundMessage) error
	OnAuthEvent       func(evt *core.BridgeChannelAuthEvent)
}

func NewBridgedChannelAdapter(bridge *PluginBridgeProcess, channelID string, logger *slog.Logger) *BridgedChannelAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &BridgedChannelAdapter{
		bridge:    bridge,
		channelId: channelID,
		logger:    logger,
		selfIDs:   make([]string, 0),
	}
}
func (a *BridgedChannelAdapter) ChannelId() string {
	return a.channelId
}

func (a *BridgedChannelAdapter) GetMessageReceivedHandler() (handler func(ctx context.Context, msg *core.InboundMessage) error) {
	return a.OnMessageReceived
}

func (a *BridgedChannelAdapter) SelfId() *string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.selfID
}

func (a *BridgedChannelAdapter) SelfIds() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	copied := make([]string, len(a.selfIDs))
	copy(copied, a.selfIDs)
	return copied
}

func (a *BridgedChannelAdapter) Start(ctx context.Context) error {
	req := core.BridgeChannelControlRequest{ChannelId: a.channelId}
	response, err := a.bridge.SendAndWait(ctx, "channel_start", req)
	if err != nil {
		return err
	}

	if response == nil || response.Result == nil || len(*response.Result) == 0 {
		return nil
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(*response.Result, &rawMap); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	var extractedSelfID *string
	if val, ok := rawMap["selfId"].(string); ok && strings.TrimSpace(val) != "" {
		extractedSelfID = &val
		a.selfID = extractedSelfID
	}

	if rawSelfIDs, ok := rawMap["selfIds"].([]interface{}); ok {
		var parsedSelfIDs []string
		for _, item := range rawSelfIDs {
			if strVal, ok := item.(string); ok {
				if strings.TrimSpace(strVal) != "" {
					parsedSelfIDs = append(parsedSelfIDs, strVal)
				}
			}
		}

		if len(parsedSelfIDs) > 0 {
			a.selfIDs = parsedSelfIDs
			if a.selfID == nil || strings.TrimSpace(*a.selfID) == "" {
				first := parsedSelfIDs[0]
				a.selfID = &first
			}
		}
	} else if a.selfID != nil && strings.TrimSpace(*a.selfID) != "" {
		a.selfIDs = []string{*a.selfID}
	}

	return nil
}

func (a *BridgedChannelAdapter) Send(ctx context.Context, message *core.OutboundMessage) error {
	markers, remainingText := core.MediaMarkerExtract(message.Text)

	var attachments []core.BridgeMediaAttachment
	var passthroughMarkerLines []string

	for _, marker := range markers {
		if isBridgeAttachmentMarker(marker.Kind) {
			attachments = append(attachments, core.BridgeMediaAttachment{
				Type: markerKindToMediaType(marker.Kind),
				Url:  marker.Value,
			})
		} else {
			passthroughMarkerLines = append(passthroughMarkerLines, toMarkerLine(marker))
		}
	}

	text := remainingText
	if len(passthroughMarkerLines) > 0 {
		markerText := strings.Join(passthroughMarkerLines, "\n")
		if strings.TrimSpace(text) == "" {
			text = markerText
		} else {
			text = markerText + "\n" + text
		}
	}

	req := core.BridgeChannelSendRequest{
		ChannelId:        a.channelId,
		RecipientId:      message.RecipientId,
		Text:             text,
		AccountId:        message.AccountId,
		SessionId:        message.SessionId,
		ReplyToMessageId: message.ReplyToMessageId,
		Subject:          message.Subject,
		Attachments:      attachments,
	}

	return a.bridge.SendRequest(ctx, "channel_send", req)
}

func (a *BridgedChannelAdapter) SendTyping(ctx context.Context, recipientId string, isTyping bool, accountId string) error {
	req := core.BridgeChannelTypingRequest{
		ChannelId:   a.channelId,
		RecipientId: recipientId,
		AccountId:   accountId,
		IsTyping:    isTyping,
	}

	err := a.bridge.SendRequest(ctx, "channel_typing", req)
	if err != nil {
		a.logger.Debug("Failed to send typing indicator", "channelId", a.channelId, "error", err)
	}
	return nil
}

func (a *BridgedChannelAdapter) SendReadReceipt(ctx context.Context, messageId string, remoteJid string, participant string, accountId string) error {
	req := core.BridgeChannelReceiptRequest{
		ChannelId:   a.channelId,
		MessageId:   messageId,
		AccountId:   accountId,
		RemoteJid:   remoteJid,
		Participant: participant,
	}

	err := a.bridge.SendRequest(ctx, "channel_read_receipt", req)
	if err != nil {
		a.logger.Debug("Failed to send read receipt", "channelId", a.channelId, "error", err)
	}
	return nil
}

func (a *BridgedChannelAdapter) SendReaction(ctx context.Context, messageId, emoji, remoteJid, participant string, accountId *string) error {
	req := core.BridgeChannelReactionRequest{
		ChannelId:   a.channelId,
		MessageId:   messageId,
		Emoji:       emoji,
		AccountId:   *accountId,
		RemoteJid:   remoteJid,
		Participant: participant,
	}

	err := a.bridge.SendRequest(ctx, "channel_react", req)
	if err != nil {
		a.logger.Debug("Failed to send reaction", "channelId", a.channelId, "error", err)
	}
	return nil
}

func (a *BridgedChannelAdapter) HandleInbound(ctx context.Context, rawParams json.RawMessage) error {
	var params map[string]interface{}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return err
	}

	senderID := getStringOrDefault(util.GetString(params, "senderId"), "unknown")
	text := getStringOrDefault(util.GetString(params, "text"), "")
	sessionID := util.GetString(params, "sessionId")
	senderName := util.GetString(params, "senderName")
	messageID := util.GetString(params, "messageId")
	replyToMessageID := util.GetString(params, "replyToMessageId")
	accountID := util.GetString(params, "accountId")

	isGroup := false
	if ig, ok := params["isGroup"].(bool); ok {
		isGroup = ig
	}

	groupID := util.GetString(params, "groupId")
	groupName := util.GetString(params, "groupName")

	var mentionedIDs []string
	if rawMids, ok := params["mentionedIds"].([]interface{}); ok {
		for _, item := range rawMids {
			if v, ok := item.(string); ok {
				mentionedIDs = append(mentionedIDs, v)
			}
		}
	}

	mediaType := util.GetString(params, "mediaType")
	mediaURL := util.GetString(params, "mediaUrl")
	mediaMimeType := util.GetString(params, "mediaMimeType")
	mediaFileName := util.GetString(params, "mediaFileName")

	if mediaType != nil {
		if mediaURL != nil && strings.TrimSpace(*mediaURL) != "" && strings.TrimSpace(*mediaType) != "" {
			var markerPrefix string
			switch *mediaType {
			case "image":
				markerPrefix = fmt.Sprintf("[IMAGE_URL:%s]", *mediaURL)
			case "video":
				markerPrefix = fmt.Sprintf("[VIDEO_URL:%s]", *mediaURL)
			case "audio":
				markerPrefix = fmt.Sprintf("[AUDIO_URL:%s]", *mediaURL)
			case "document":
				markerPrefix = fmt.Sprintf("[DOCUMENT_URL:%s]", *mediaURL)
			case "sticker":
				markerPrefix = fmt.Sprintf("[STICKER_URL:%s]", *mediaURL)
			default:
				markerPrefix = fmt.Sprintf("[FILE_URL:%s]", *mediaURL)
			}

			if strings.TrimSpace(text) == "" {
				text = markerPrefix
			} else {
				text = fmt.Sprintf("%s\n%s", markerPrefix, text)
			}
		}
	}

	var attachments []core.MediaAttachment
	if rawAtts, ok := params["attachments"].([]interface{}); ok {
		var markerLines []string
		for _, rawAtt := range rawAtts {
			if attMap, ok := rawAtt.(map[string]interface{}); ok {
				attType := util.GetString(attMap, "mediaType")
				attURL := util.GetString(attMap, "url")
				attMime := util.GetString(attMap, "mimeType")
				attFile := util.GetString(attMap, "fileName")

				if attURL == nil || attType == nil || strings.TrimSpace(*attURL) == "" || strings.TrimSpace(*attType) == "" {
					continue
				}

				attachments = append(attachments, core.MediaAttachment{
					MediaType: *attType,
					Url:       *attURL,
					MimeType:  util.Deref(attMime),
					FileName:  util.Deref(attFile),
				})

				var marker string
				switch *attType {
				case "image":
					marker = fmt.Sprintf("[IMAGE_URL:%s]", *attURL)
				case "video":
					marker = fmt.Sprintf("[VIDEO_URL:%s]", *attURL)
				case "audio":
					marker = fmt.Sprintf("[AUDIO_URL:%s]", *attURL)
				case "document":
					marker = fmt.Sprintf("[DOCUMENT_URL:%s]", *attURL)
				default:
					marker = fmt.Sprintf("[FILE_URL:%s]", *attURL)
				}
				markerLines = append(markerLines, marker)
			}
		}

		if len(markerLines) > 0 {
			allMarkers := strings.Join(markerLines, "\n")
			if strings.TrimSpace(text) == "" {
				text = allMarkers
			} else {
				text = fmt.Sprintf("%s\n%s", allMarkers, text)
			}
		}
	}

	msg := &core.InboundMessage{
		ChannelId:        a.channelId,
		SenderId:         senderID,
		Text:             text,
		AccountId:        util.Deref(accountID),
		SessionId:        util.Deref(sessionID),
		SenderName:       util.Deref(senderName),
		MessageId:        util.Deref(messageID),
		ReplyToMessageId: util.Deref(replyToMessageID),
		IsGroup:          isGroup,
		GroupId:          util.Deref(groupID),
		GroupName:        util.Deref(groupName),
		MentionedIds:     mentionedIDs,
		MediaType:        util.Deref(mediaType),
		MediaUrl:         util.Deref(mediaURL),
		MediaMimeType:    util.Deref(mediaMimeType),
		MediaFileName:    util.Deref(mediaFileName),
		Attachments:      attachments,
	}

	if a.OnMessageReceived != nil {
		if err := a.OnMessageReceived(ctx, msg); err != nil {
			a.logger.Warn("OnMessageReceived handler failed", "channelId", a.channelId, "error", err)
		}
	}

	return nil
}

func (a *BridgedChannelAdapter) HandleAuthEvent(rawParams json.RawMessage) {
	var params map[string]interface{}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		a.logger.Warn("Failed to unmarshal auth event parameters", "channelId", a.channelId, "error", err)
		return
	}

	state := getStringOrDefault(util.GetString(params, "state"), "unknown")
	data := util.GetString(params, "data")
	accountID := util.GetString(params, "accountId")

	evt := &core.BridgeChannelAuthEvent{
		ChannelId:    a.channelId,
		State:        state,
		Data:         util.Deref(data),
		AccountId:    util.Deref(accountID),
		UpdatedAtUtc: time.Now().UTC(),
	}

	if a.OnAuthEvent != nil {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Warn("OnAuthEvent handler panicked", "channelId", a.channelId, "panic", r)
			}
		}()
		a.OnAuthEvent(evt)
	}
}

func (a *BridgedChannelAdapter) Close(ctx context.Context) error {
	return nil
}

func (a *BridgedChannelAdapter) Restart(ctx context.Context) error {
	req := core.BridgeChannelControlRequest{ChannelId: a.channelId}
	_ = a.bridge.SendRequest(ctx, "channel_stop", req)

	a.mu.Lock()
	a.selfID = nil
	a.selfIDs = nil
	a.mu.Unlock()

	return a.Start(ctx)
}

// --- Helper Functions ---

func markerKindToMediaType(kind core.MediaMarkerKind) string {
	switch kind {
	case core.MediaMarkerImageUrl, core.MediaMarkerImagePath:
		return "image"
	case core.MediaMarkerVideoUrl:
		return "video"
	case core.MediaMarkerAudioUrl:
		return "audio"
	case core.MediaMarkerDocumentUrl, core.MediaMarkerFileUrl, core.MediaMarkerFilePath:
		return "document"
	case core.MediaMarkerStickerUrl:
		return "sticker"
	default:
		return "document"
	}
}

func isBridgeAttachmentMarker(kind core.MediaMarkerKind) bool {
	switch kind {
	case core.MediaMarkerImageUrl, core.MediaMarkerImagePath,
		core.MediaMarkerVideoUrl, core.MediaMarkerAudioUrl,
		core.MediaMarkerDocumentUrl, core.MediaMarkerFileUrl,
		core.MediaMarkerFilePath, core.MediaMarkerStickerUrl:
		return true
	default:
		return false
	}
}

func toMarkerLine(marker core.MediaMarker) string {
	switch marker.Kind {
	case core.MediaMarkerTelegramImageFileId:
		return fmt.Sprintf("[IMAGE:telegram:file_id=%s]", marker.Value)
	case core.MediaMarkerTelegramVideoFileId:
		return fmt.Sprintf("[VIDEO:telegram:file_id=%s]", marker.Value)
	case core.MediaMarkerTelegramAudioFileId:
		return fmt.Sprintf("[AUDIO:telegram:file_id=%s]", marker.Value)
	case core.MediaMarkerTelegramDocumentFileId:
		return fmt.Sprintf("[DOCUMENT:telegram:file_id=%s]", marker.Value)
	case core.MediaMarkerTelegramStickerFileId:
		return fmt.Sprintf("[STICKER:telegram:file_id=%s]", marker.Value)
	default:
		return marker.Value
	}
}

type BridgedToolHook struct {
	name               string
	bridge             *PluginBridgeProcess
	pluginId           string
	eventSubscriptions []string
	logger             *slog.Logger
	beforeTimeout      time.Duration
}

func NewBridgedToolHook(bridge *PluginBridgeProcess, pluginId string, eventSubscriptions []string, logger *slog.Logger) *BridgedToolHook {
	if logger == nil {
		logger = slog.Default()
	}
	hook := &BridgedToolHook{
		bridge:             bridge,
		pluginId:           pluginId,
		eventSubscriptions: eventSubscriptions,
		logger:             logger,
		name:               fmt.Sprintf("plugin:%s", pluginId),
		beforeTimeout:      time.Second * 5,
	}
	return hook
}

func (b *BridgedToolHook) AfterExecute(ctx context.Context, toolName string, arguments string, result string, duration time.Duration, failed bool) error {
	var eventName = "tool:after"
	if !slices.Contains(b.eventSubscriptions, eventName) && !slices.Contains(b.eventSubscriptions, "tool:*") {
		return nil
	}

	err := b.bridge.SendRequest(ctx, "hook_after", core.BridgeHookAfterRequest{
		EventName:  eventName,
		ToolName:   toolName,
		Arguments:  arguments,
		Result:     result,
		DurationMs: float64(duration.Milliseconds()),
		Failed:     failed,
	})

	if err != nil {
		b.logger.Warn("hook failed", "PluginId", b.pluginId, "ToolName", toolName)
	}

	return err
}

func (b *BridgedToolHook) BeforeExecute(ctx context.Context, toolName string, arguments string) bool {
	var eventName = "tool:before"
	if !slices.Contains(b.eventSubscriptions, eventName) && !slices.Contains(b.eventSubscriptions, "tool:*") {
		return true
	}

	initCtx, cancel := context.WithTimeout(ctx, b.beforeTimeout)
	defer cancel()

	response, err := b.bridge.SendAndWait(initCtx, "hook_before", core.BridgeHookBeforeRequest{
		EventName: eventName,
		ToolName:  toolName,
		Arguments: arguments,
	})

	if err != nil {
		b.logger.Warn("hook failed on tool", "PluginId", b.pluginId, "ToolName", toolName)
		return true
	}

	if response.Result == nil {
		return true
	}

	var raw map[string]any
	if err := json.Unmarshal(*response.Result, &raw); err != nil {
		return true
	}

	if value, ok := raw["allow"].(bool); ok {
		return value
	}

	return true
}

func (b *BridgedToolHook) Name() string {
	return b.name
}
