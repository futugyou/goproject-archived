package channels

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/futugyou/openclaw/core"
)

var _ core.IChannelAdapter = (*TwilioSmsChannel)(nil)

type TwilioSmsChannel struct {
	config     core.TwilioSmsConfig
	contacts   core.IContactStore
	msgHandler core.ChannelMessageHandler
	client     *TwilioSmsClient
}

func (c *TwilioSmsChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return c.msgHandler
}

func (c *TwilioSmsChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	c.msgHandler = handler
}

func NewTwilioSmsChannel(config core.TwilioSmsConfig,
	contacts core.IContactStore,
	authToken string,
	httpClient *http.Client) *TwilioSmsChannel {

	return &TwilioSmsChannel{
		config:   config,
		contacts: contacts,
		client:   NewTwilioSmsClient(httpClient, config, authToken),
	}
}

func (c *TwilioSmsChannel) ChannelId() string {
	return "sms"
}

func (c *TwilioSmsChannel) Close(ctx context.Context) error {
	return nil
}

func (c *TwilioSmsChannel) Start(ctx context.Context) error {
	return nil
}

func (c *TwilioSmsChannel) RaiseInbound(ctx context.Context, message *core.InboundMessage) error {
	if c.msgHandler != nil {
		return c.msgHandler(ctx, message)
	}
	return nil
}

func (c *TwilioSmsChannel) isAllowedRecipient(toE164 string) bool {
	if len(c.config.AllowedToNumbers) == 0 {
		return false
	}

	return slices.Contains(c.config.AllowedToNumbers, toE164)
}

func (c *TwilioSmsChannel) Send(ctx context.Context, message *core.OutboundMessage) error {
	if message.Text == "" {
		return nil
	}
	var to = message.RecipientId
	if !c.isAllowedRecipient(to) {
		return nil
	}

	contact, err := c.contacts.Get(ctx, to)
	if err != nil {
		return err
	}

	if contact.DoNotText {
		return nil
	}

	ok, result := c.client.Send(ctx, to, message.Text)
	if !ok {
		return errors.New(result)
	}

	return nil
}
