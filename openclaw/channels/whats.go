package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/futugyou/openclaw/core"
)

var _ core.IChannelAdapter = (*WhatsAppChannel)(nil)

type WhatsAppChannel struct {
	config     core.WhatsAppChannelConfig
	msgHandler core.ChannelMessageHandler
	httpClient *http.Client
	logger     *slog.Logger
}

func (c *WhatsAppChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return c.msgHandler
}

func (c *WhatsAppChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	c.msgHandler = handler
}

func NewWhatsAppChannel(config core.WhatsAppChannelConfig,
	httpClient *http.Client,
	logger *slog.Logger) *WhatsAppChannel {

	if logger == nil {
		logger = slog.Default()
	}
	return &WhatsAppChannel{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

func (c *WhatsAppChannel) ChannelId() string {
	return "whatsapp"
}

func (c *WhatsAppChannel) Close(ctx context.Context) error {
	return nil
}

func (c *WhatsAppChannel) Start(ctx context.Context) error {
	return nil
}

func (c *WhatsAppChannel) RaiseInbound(ctx context.Context, message *core.InboundMessage) error {
	if c.msgHandler != nil {
		return c.msgHandler(ctx, message)
	}
	return nil
}

func (w *WhatsAppChannel) Send(ctx context.Context, message *core.OutboundMessage) error {
	if strings.TrimSpace(w.config.PhoneNumberId) == "" {
		w.logger.Warn("WhatsApp Send aborted: PhoneNumberId is not configured.")
		return nil
	}

	var tokenSource = core.SecretResolverInstance.Resolve(w.config.CloudApiTokenRef)
	if tokenSource == "" {
		tokenSource = w.config.CloudApiToken
	}

	if tokenSource == "" {
		return errors.New("slack token can not be empty")
	}

	markers, remainingText := core.MediaMarkerExtract(message.Text)
	if len(markers) == 0 && strings.TrimSpace(remainingText) == "" {
		return nil
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages", w.config.PhoneNumberId)

	payload, err := w.buildPayload(message, markers, remainingText)
	if err != nil {
		return fmt.Errorf("failed to build payload: %w", err)
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokenSource)
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Error("Failed to send WhatsApp message", "recipientId", message.RecipientId, "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
		w.logger.Error("Failed to send WhatsApp message", "recipientId", message.RecipientId, "error", err)
		return err
	}

	w.logger.Info("Sent WhatsApp message", "recipientId", message.RecipientId)
	return nil
}

func (w *WhatsAppChannel) buildPayload(outbound *core.OutboundMessage, markers []core.MediaMarker, remainingText string) (*WhatsAppSendPayload, error) {
	payload := &WhatsAppSendPayload{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               outbound.RecipientId,
	}

	if strings.TrimSpace(outbound.ReplyToMessageId) != "" {
		payload.Context = &WhatsAppContextObj{
			MessageId: outbound.ReplyToMessageId,
		}
	}

	if len(markers) == 0 {
		payload.Type = "text"
		payload.Text = &WhatsAppTextObj{
			Body:       remainingText,
			PreviewUrl: false,
		}
		return payload, nil
	}

	if len(markers) > 1 {
		w.logger.Warn("WhatsApp Cloud API only supports one outbound media attachment per message. Using the first attachment.", "recipientId", outbound.RecipientId)
	}

	marker := markers[0]
	msgType, err := markerKindToMessageType(marker.Kind)
	if err != nil {
		return nil, err
	}
	payload.Type = msgType

	link, err := markerKindToLink(marker)
	if err != nil {
		return nil, err
	}

	media := &WhatsAppMediaObj{
		Link: link,
	}

	if supportsCaption(payload.Type) && strings.TrimSpace(remainingText) != "" {
		media.Caption = remainingText
	}

	if payload.Type == "document" && (marker.Kind == core.MediaMarkerFileUrl || marker.Kind == core.MediaMarkerDocumentUrl) {
		if fileName := getFileName(marker.Value); fileName != "" {
			media.Filename = fileName
		}
	}

	switch payload.Type {
	case "image":
		payload.Image = media
	case "video":
		payload.Video = media
	case "audio":
		payload.Audio = media
	case "document":
		payload.Document = media
	case "sticker":
		payload.Sticker = media
	default:
		return nil, fmt.Errorf("unsupported WhatsApp media type '%s'", payload.Type)
	}

	return payload, nil
}

func markerKindToMessageType(kind core.MediaMarkerKind) (string, error) {
	switch kind {
	case core.MediaMarkerImageUrl:
		return "image", nil
	case core.MediaMarkerVideoUrl:
		return "video", nil
	case core.MediaMarkerAudioUrl:
		return "audio", nil
	case core.MediaMarkerDocumentUrl, core.MediaMarkerFileUrl:
		return "document", nil
	case core.MediaMarkerStickerUrl:
		return "sticker", nil
	default:
		return "", fmt.Errorf("whatsApp Cloud API does not support marker kind '%v'", kind)
	}
}

func markerKindToLink(marker core.MediaMarker) (string, error) {
	u, err := url.Parse(marker.Value)
	if err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") {
		return marker.Value, nil
	}
	return "", fmt.Errorf("whatsApp Cloud API outbound media markers must use absolute http(s) URLs. Unsupported value: '%s'", marker.Value)
}

func supportsCaption(messageType string) bool {
	return messageType == "image" || messageType == "video" || messageType == "document"
}

func getFileName(value string) string {
	u, err := url.Parse(value)
	if err != nil || !u.IsAbs() {
		return ""
	}
	fileName := path.Base(u.Path)
	if fileName == "." || fileName == "/" || strings.TrimSpace(fileName) == "" {
		return ""
	}
	return fileName
}

type WhatsAppSendPayload struct {
	MessagingProduct string              `json:"messaging_product"`
	RecipientType    string              `json:"recipient_type"`
	Context          *WhatsAppContextObj `json:"context,omitempty"`
	To               string              `json:"to"`
	Type             string              `json:"type"`
	Text             *WhatsAppTextObj    `json:"text,omitempty"`
	Image            *WhatsAppMediaObj   `json:"image,omitempty"`
	Video            *WhatsAppMediaObj   `json:"video,omitempty"`
	Audio            *WhatsAppMediaObj   `json:"audio,omitempty"`
	Document         *WhatsAppMediaObj   `json:"document,omitempty"`
	Sticker          *WhatsAppMediaObj   `json:"sticker,omitempty"`
}

type WhatsAppContextObj struct {
	MessageId string `json:"message_id"`
}

type WhatsAppTextObj struct {
	Body       string `json:"body"`
	PreviewUrl bool   `json:"preview_url"`
}

type WhatsAppMediaObj struct {
	Link     string `json:"link"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// Webhook Inbound Payload Models

type WhatsAppInboundPayload struct {
	Object string          `json:"object,omitempty"`
	Entry  []WhatsAppEntry `json:"entry,omitempty"`
}

type WhatsAppEntry struct {
	ID      string           `json:"id,omitempty"`
	Changes []WhatsAppChange `json:"changes,omitempty"`
}

type WhatsAppChange struct {
	Value *WhatsAppValue `json:"value,omitempty"`
	Field string         `json:"field,omitempty"`
}

type WhatsAppValue struct {
	MessagingProduct string            `json:"messaging_product,omitempty"`
	Contacts         []WhatsAppContact `json:"contacts,omitempty"`
	Messages         []WhatsAppMessage `json:"messages,omitempty"`
}

type WhatsAppContact struct {
	Profile *WhatsAppProfile `json:"profile,omitempty"`
	WaID    string           `json:"wa_id,omitempty"`
}

type WhatsAppProfile struct {
	Name string `json:"name,omitempty"`
}

type WhatsAppMessage struct {
	From      string           `json:"from,omitempty"`
	ID        string           `json:"id,omitempty"`
	Timestamp string           `json:"timestamp,omitempty"`
	Type      string           `json:"type,omitempty"`
	Text      *WhatsAppTextObj `json:"text,omitempty"`
}
