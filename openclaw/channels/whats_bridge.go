package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/futugyou/openclaw/core"
)

var _ core.IChannelAdapter = (*WhatsAppBridgeChannel)(nil)

type WhatsAppBridgeChannel struct {
	config     core.WhatsAppChannelConfig
	msgHandler core.ChannelMessageHandler
	httpClient *http.Client
	logger     *slog.Logger
}

func (c *WhatsAppBridgeChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return c.msgHandler
}

func (c *WhatsAppBridgeChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	c.msgHandler = handler
}

func NewWhatsAppBridgeChannel(config core.WhatsAppChannelConfig,
	httpClient *http.Client,
	logger *slog.Logger) *WhatsAppBridgeChannel {

	if logger == nil {
		logger = slog.Default()
	}
	return &WhatsAppBridgeChannel{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

func (c *WhatsAppBridgeChannel) ChannelId() string {
	return "whatsapp"
}

func (c *WhatsAppBridgeChannel) Close(ctx context.Context) error {
	return nil
}

func (c *WhatsAppBridgeChannel) Start(ctx context.Context) error {
	return nil
}

func (c *WhatsAppBridgeChannel) RaiseInbound(ctx context.Context, message *core.InboundMessage) error {
	if c.msgHandler != nil {
		return c.msgHandler(ctx, message)
	}
	return nil
}

func (w *WhatsAppBridgeChannel) Send(ctx context.Context, message *core.OutboundMessage) error {
	if strings.TrimSpace(w.config.BridgeUrl) == "" {
		w.logger.Warn("WhatsApp Bridge Send aborted: BridgeUrl is not configured.")
		return nil
	}

	var tokenSource = core.SecretResolverInstance.Resolve(w.config.CloudApiTokenRef)
	if tokenSource == "" {
		tokenSource = w.config.CloudApiToken
	}

	markers, remainingText := core.MediaMarkerExtract(message.Text)
	if len(markers) == 0 && strings.TrimSpace(remainingText) == "" {
		return nil
	}

	var attachments []WhatsAppBridgeAttachmentPayload
	if len(markers) > 0 {
		attachments = make([]WhatsAppBridgeAttachmentPayload, len(markers))
		for i, marker := range markers {
			mediaType, err := markerKindToMediaType(marker.Kind)
			if err != nil {
				return err
			}
			attachments[i] = WhatsAppBridgeAttachmentPayload{
				Type: mediaType,
				Url:  marker.Value,
			}
		}
	}

	payload := WhatsAppBridgeSendPayload{
		To:               message.RecipientId,
		Text:             remainingText,
		ReplyToMessageId: message.ReplyToMessageId,
		Attachments:      attachments,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal bridge payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.BridgeUrl, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create bridge request: %w", err)
	}

	if tokenSource != "" {
		req.Header.Set("Authorization", "Bearer "+tokenSource)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Error("Failed to send WhatsApp Bridge message", "recipientId", message.RecipientId, "error", err)
		if w.config.BridgeSuppressSendExceptions {
			return nil
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("unexpected bridge status code %d: %s", resp.StatusCode, string(respBody))
		w.logger.Error("Failed to send WhatsApp Bridge message", "recipientId", message.RecipientId, "error", err)
		if w.config.BridgeSuppressSendExceptions {
			return nil
		}
		return err
	}

	w.logger.Info("Sent WhatsApp Bridge message", "recipientId", message.RecipientId)
	return nil
}

func markerKindToMediaType(kind core.MediaMarkerKind) (string, error) {
	switch kind {
	case core.MediaMarkerImageUrl, core.MediaMarkerImagePath:
		return "image", nil
	case core.MediaMarkerVideoUrl:
		return "video", nil
	case core.MediaMarkerAudioUrl:
		return "audio", nil
	case core.MediaMarkerDocumentUrl, core.MediaMarkerFileUrl, core.MediaMarkerFilePath:
		return "document", nil
	case core.MediaMarkerStickerUrl:
		return "sticker", nil
	default:
		return "", fmt.Errorf("whatsApp bridge does not support marker kind '%v'", kind)
	}
}

type WhatsAppBridgeSendPayload struct {
	To               string                            `json:"to"`
	Text             string                            `json:"text"`
	ReplyToMessageId string                            `json:"reply_to_message_id,omitempty"`
	Attachments      []WhatsAppBridgeAttachmentPayload `json:"attachments,omitempty"`
}

type WhatsAppBridgeAttachmentPayload struct {
	Type        string `json:"type"`
	Url         string `json:"url,omitempty"`
	Caption     string `json:"caption,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	GifPlayback bool   `json:"gifPlayback"`
}

type WhatsAppBridgeInboundPayload struct {
	From             string                            `json:"from"`
	Text             string                            `json:"text,omitempty"`
	AccountId        string                            `json:"account_id,omitempty"`
	SessionId        string                            `json:"session_id,omitempty"`
	SenderName       string                            `json:"sender_name,omitempty"`
	MessageId        string                            `json:"message_id,omitempty"`
	ReplyToMessageId string                            `json:"reply_to_message_id,omitempty"`
	IsGroup          bool                              `json:"is_group"`
	GroupId          string                            `json:"group_id,omitempty"`
	GroupName        string                            `json:"group_name,omitempty"`
	MentionedIds     []string                          `json:"mentioned_ids,omitempty"`
	MediaType        string                            `json:"media_type,omitempty"`
	MediaUrl         string                            `json:"media_url,omitempty"`
	MediaMimeType    string                            `json:"media_mime_type,omitempty"`
	MediaFileName    string                            `json:"media_file_name,omitempty"`
	Attachments      []WhatsAppBridgeAttachmentPayload `json:"attachments,omitempty"`
}
