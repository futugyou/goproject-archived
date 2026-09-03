package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

var _ core.IChannelAdapter = (*SlackChannel)(nil)

type SlackChannel struct {
	config     core.SlackChannelConfig
	logger     *slog.Logger
	msgHandler core.ChannelMessageHandler
	httpClient *http.Client
}

func (c *SlackChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return c.msgHandler
}

func (c *SlackChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	c.msgHandler = handler
}

func NewSlackChannel(config core.SlackChannelConfig, logger *slog.Logger) *SlackChannel {
	if logger == nil {
		logger = slog.Default()
	}

	return &SlackChannel{
		config:     config,
		logger:     logger,
		httpClient: &http.Client{},
	}
}

func (c *SlackChannel) ChannelId() string {
	return "slack"
}

func (c *SlackChannel) Close(ctx context.Context) error {
	return nil
}

func (c *SlackChannel) Start(ctx context.Context) error {
	return nil
}

func (c *SlackChannel) Send(ctx context.Context, message *core.OutboundMessage) error {
	if message.Text == "" {
		return nil
	}

	var tokenSource = core.SecretResolverInstance.Resolve(c.config.BotTokenRef)
	if tokenSource == "" {
		tokenSource = c.config.BotToken
	}

	if tokenSource == "" {
		return errors.New("slack token can not be empty")
	}

	_, remaining := core.MediaMarkerExtract(message.Text)
	if remaining == "" {
		remaining = message.Text
	}

	var text = convertToMrkdwn(remaining)

	var payload = SlackPostMessageRequest{
		Channel:  message.RecipientId,
		Text:     text,
		ThreadTs: message.ReplyToMessageId,
	}

	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Bearer", tokenSource)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		retryAfter := util.GetRetryAfter(resp, 1*time.Second)
		c.logger.Warn(fmt.Sprintf("Slack rate limited for %s. Retry-After: %fs.", message.RecipientId, retryAfter.Seconds()))
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Error(fmt.Sprintf("http response code: %d", resp.StatusCode))
		return nil
	}

	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		c.logger.Error(err.Error())
		return nil
	}

	if !response.OK {
		msg := response.Error
		if msg == "" {
			msg = "unknown"
		}
		c.logger.Error(fmt.Sprintf("Slack API error sending to %s: %s", message.RecipientId, msg))
		return nil
	}

	c.logger.Info(fmt.Sprintf("Sent Slack message to %s ", message.RecipientId))
	return nil
}

var (
	boldRegex = regexp.MustCompile(`\*\*(.+?)\*\*`)
	linkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

func convertToMrkdwn(markdown string) string {
	result := boldRegex.ReplaceAllString(markdown, "*$1*")

	// Convert [text](url) to <url|text>
	result = linkRegex.ReplaceAllString(result, "<$2|$1>")

	return result
}

type SlackPostMessageRequest struct {
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	ThreadTs string `json:"thread_ts,omitempty"`
}

type SlackEventWrapper struct {
	Type      string      `json:"type"`
	Challenge string      `json:"challenge"`
	TeamId    string      `json:"team_id"`
	Event     *SlackEvent `json:"event,omitempty"`
	EventId   string      `json:"event_id"`
}

type SlackEvent struct {
	Type        string `json:"type"`
	Subtype     string `json:"subtype"`
	User        string `json:"user"`
	BotId       string `json:"bot_id"`
	Text        string `json:"text"`
	Channel     string `json:"channel"`
	ChannelType string `json:"channel_type"`
	Ts          string `json:"ts"`
	ThreadTs    string `json:"thread_ts"`
}
