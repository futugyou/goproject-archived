package channels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

var _ core.IChannelAdapter = (*CronChannel)(nil)

type CronChannel struct {
	storagePath string
	logger      *slog.Logger
	msgHandler  core.ChannelMessageHandler
}

func (c *CronChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return c.msgHandler
}

func (c *CronChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	c.msgHandler = handler
}

func NewCronChannel(storagePath string, logger *slog.Logger) *CronChannel {
	if logger == nil {
		logger = slog.Default()
	}

	return &CronChannel{
		storagePath: storagePath,
		logger:      logger,
	}
}

func (c *CronChannel) ChannelId() string {
	return "cron"
}

func (c *CronChannel) Close(ctx context.Context) error {
	return nil
}

func (c *CronChannel) Start(ctx context.Context) error {
	return nil
}

func BuildSafeFilename(recipient string) string {
	hint := "recipient"
	safe := []byte{}
	for i := 0; i < min(len(recipient), 32); i++ {
		c := recipient[i]
		if util.IsLetterOrDigit(c) || c == '_' || c == '-' || c == '.' {
			safe = append(safe, c)
		}
	}

	if len(safe) > 0 {
		hint = string(safe)
	}

	hash := sha256.Sum256([]byte(recipient))
	return fmt.Sprintf("%s-%s.log", hint, strings.ToLower(hex.EncodeToString(hash[:])[:12]))
}

func (c *CronChannel) Send(ctx context.Context, message *core.OutboundMessage) error {
	recipient := "cron"
	if rec := strings.TrimSpace(message.RecipientId); rec != "" {
		recipient = rec
	}
	subject := "OpenClaw Cron"
	if sub := strings.TrimSpace(message.RecipientId); sub != "" {
		subject = sub
	}

	var dir = filepath.Join(c.storagePath, "cron")
	os.MkdirAll(dir, 0755)

	var filename = BuildSafeFilename(recipient)
	var path = filepath.Join(dir, filename)

	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "received_at: %s\n", time.Now().UTC().Format(time.RFC3339Nano))
	fmt.Fprintf(&sb, "recipient: %s\n", recipient)
	fmt.Fprintf(&sb, "subject: %s\n\n", subject)
	sb.WriteString(message.Text)
	sb.WriteString("\n\n-----\n\n")

	if err := util.SaveFile(ctx, path, sb.String()); err != nil {
		return err
	}

	c.logger.Info(fmt.Sprintf("Cron output written to %s (recipient=%s)", path, recipient))
	return nil
}
