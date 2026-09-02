package channels

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	gomail "github.com/wneessen/go-mail"
)

var _ core.IChannelAdapter = (*EmailChannel)(nil)

type EmailChannel struct {
	config     core.EmailConfig
	logger     *slog.Logger
	msgHandler core.ChannelMessageHandler
}

func NewEmailChannel(config core.EmailConfig, logger *slog.Logger) *EmailChannel {
	if logger == nil {
		logger = slog.Default()
	}

	return &EmailChannel{config: config, logger: logger}
}

// ChannelId implements [core.IChannelAdapter].
func (e *EmailChannel) ChannelId() string {
	return "email"
}

// Close implements [core.IChannelAdapter].
func (e *EmailChannel) Close(ctx context.Context) error {
	return nil
}

// GetMessageReceivedHandler implements [core.IChannelAdapter].
func (e *EmailChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return e.msgHandler
}

// Send implements [core.IChannelAdapter].
func (e *EmailChannel) Send(ctx context.Context, message *core.OutboundMessage) error {
	if e.config.SmtpHost == "" {
		return errors.New("Email channel requires Email.SmtpHost to be configured.")
	}

	to := message.RecipientId
	if to == "" {
		return nil
	}

	var password = core.SecretResolverInstance.Resolve(e.config.PasswordRef)
	client, err := gomail.NewClient(e.config.SmtpHost,
		gomail.WithPort(e.config.SmtpPort),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
		gomail.WithUsername(e.config.Username),
		gomail.WithPassword(password),
	)
	if err != nil {
		return err
	}

	msg := gomail.NewMsg()
	from := e.config.FromAddress
	if from == "" {
		from = e.config.Username
	}
	subject := message.Subject
	if subject == "" {
		subject = "OpenClaw"
	}
	msg.From(from)
	msg.To(to)
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, message.Text)

	return client.DialAndSendWithContext(ctx, msg)
}

// SetMessageReceivedHandler implements [core.IChannelAdapter].
func (e *EmailChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	e.msgHandler = handler
}

// Start implements [core.IChannelAdapter].
func (e *EmailChannel) Start(ctx context.Context) error {
	if !e.config.InboundEnabled {
		return nil
	}

	if !e.CanListenForInbound() {
		e.logger.Warn("Email inbound listener is enabled but IMAP host or credentials are not fully configured.")
		return nil
	}

	if err := core.Sanitizer.CheckImapFolderName(e.config.InboundFolder); err != nil {
		e.logger.Warn("Email inbound listener is disabled because the configured IMAP folder is invalid:  " + err.Error())
		return nil
	}

	var secs = (util.Clamp(e.config.InboundPollSeconds, 5, 3600))

	interval := time.Duration(secs) * time.Second

	e.pollInboxSafe(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			e.pollInboxSafe(ctx)
		}
	}
}

func (e *EmailChannel) CanListenForInbound() bool {
	var password = core.SecretResolverInstance.Resolve(e.config.PasswordRef)
	return e.config.ImapHost != "" && e.config.Username != "" && password != ""
}

func (e *EmailChannel) pollInboxSafe(ctx context.Context) error {
	if e.msgHandler == nil {
		return nil
	}

	var password = core.SecretResolverInstance.Resolve(e.config.PasswordRef)

	addr := fmt.Sprintf("%s:%d", e.config.ImapHost, e.config.ImapPort)
	var c *imapclient.Client
	var err error

	if e.config.ImapPort == 993 || e.config.ImapUseTls {
		c, err = imapclient.DialTLS(addr, nil)
	} else {
		c, err = imapclient.DialInsecure(addr, nil)
	}
	if err != nil {
		return err
	}
	defer c.Logout()

	if err := c.Login(e.config.Username, password).Wait(); err != nil {
		return err
	}

	_, err = c.Select(e.config.InboundFolder, nil).Wait()
	if err != nil {
		return err
	}

	criteria := &imap.SearchCriteria{}
	criteria.Flag = []imap.Flag{imap.FlagSeen}
	searchData, err := c.Search(criteria, nil).Wait()
	if err != nil {
		return err
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil
	}

	var maxMessages = max(1, e.config.InboundMaxMessagesPerPoll)
	var startIndex = max(0, len(uids)-maxMessages)

	nums := []uint32{}
	for _, v := range uids[startIndex:] {
		nums = append(nums, uint32(v))
	}
	messages, err := c.Fetch(
		imap.SeqSetNum(nums...),
		&imap.FetchOptions{
			UID:      true,
			Envelope: true,
			BodySection: []*imap.FetchItemBodySection{
				{},
			},
		},
	).Collect()

	if err != nil {
		return err
	}

	var readed []imap.UID
	for _, msg := range messages {
		body := msg.FindBodySection(&imap.FetchItemBodySection{})
		if body == nil {
			continue
		}

		e.msgHandler(ctx, &core.InboundMessage{
			ChannelId:  e.ChannelId(),
			SenderId:   msg.Envelope.From[0].Addr(),
			SenderName: msg.Envelope.From[0].Name,
			Subject:    msg.Envelope.Subject,
			Text:       string(body),
			MessageId:  msg.Envelope.MessageID,
			Type:       "email",
			ReceivedAt: msg.InternalDate,
		})

		if e.config.MarkInboundAsRead {
			readed = append(readed, msg.UID)
		}
	}

	if len(readed) > 0 {
		uidSet := imap.UIDSetNum(uids...)
		if err := c.Store(uidSet, &imap.StoreFlags{
			Op:     imap.StoreFlagsAdd,
			Silent: true,
			Flags:  []imap.Flag{imap.FlagSeen},
		}, nil).Close(); err != nil {
			return err
		}
	}

	return nil
}
