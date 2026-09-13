package channels

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/core"
)

var _ core.IChannelAdapter = (*SignalChannel)(nil)

type SignalChannel struct {
	config            core.SignalChannelConfig
	logger            *slog.Logger
	accountNumber     string
	onMessageReceived core.ChannelMessageHandler

	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

func (c *SignalChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return c.onMessageReceived
}

func (c *SignalChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	c.onMessageReceived = handler
}

func NewSignalChannel(config core.SignalChannelConfig, logger *slog.Logger) *SignalChannel {
	if logger == nil {
		logger = slog.Default()
	}
	return &SignalChannel{
		config: config,
		logger: logger,
	}
}

func (c *SignalChannel) ChannelId() string {
	return "signal"
}

func (s *SignalChannel) ChannelType() string { return "signal" }

func (c *SignalChannel) Close(ctx context.Context) error {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}

	c.wg.Wait()
	return nil
}

func (c *SignalChannel) Start(parentCtx context.Context) error {
	var tokenSource = core.SecretResolverInstance.Resolve(c.config.AccountPhoneNumberRef)
	if tokenSource == "" {
		tokenSource = c.config.AccountPhoneNumber
	}

	if tokenSource == "" {
		return errors.New("AccountPhoneNumber can not be empty")
	}

	ctx, cancel := context.WithCancel(parentCtx)
	c.cancelFunc = cancel
	c.wg.Add(1)

	c.accountNumber = tokenSource

	switch c.config.Driver {
	case "signald":
		go func() {
			defer c.wg.Done()
			c.runSignaldLoop(ctx)
		}()
	case "signal_cli":
		go func() {
			defer c.wg.Done()
			c.runSignalCliLoop(ctx)
		}()
	default:
		cancel()
		c.wg.Done()
		return fmt.Errorf("Unknown Signal driver: '%s'. Expected 'signald' or 'signal_cli'.", c.config.Driver)
	}

	return nil
}

func (s *SignalChannel) runSignalCliLoop(ctx context.Context) error {
	cliPath := s.config.SignalCliPath
	if cliPath == "" {
		cliPath = "signal-cli"
	}

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cmd := exec.CommandContext(ctx, cliPath, "-u", s.accountNumber, "daemon", "--json")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			s.logger.Error("Failed to create stdout pipe for signal-cli", "error", err)
			return err
		}

		if err := cmd.Start(); err != nil {
			s.logger.Error("Failed to start signal-cli process", "error", err)
			return err
		}

		s.logger.Info("Started signal-cli daemon", "account", s.accountNumber)
		backoff = 1 * time.Second

		s.readCliOutput(ctx, stdout)

		// 确保子进程被清理
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()

		if err := ctx.Err(); err != nil {
			return err
		}

		s.logger.Warn("signal-cli process exited. Restarting", "backoff_sec", backoff.Seconds())

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (s *SignalChannel) runSignaldLoop(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := s.connectAndListenSignald(ctx)
		if ctx.Err() != nil {
			return // Context 取消，正常退出
		}

		if err != nil {
			s.logger.Warn("signald connection lost. Reconnecting", "backoff_sec", backoff.Seconds(), "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *SignalChannel) RaiseInbound(ctx context.Context, message *core.InboundMessage) error {
	if c.onMessageReceived != nil {
		return c.onMessageReceived(ctx, message)
	}
	return nil
}

func (s *SignalChannel) Send(ctx context.Context, message *core.OutboundMessage) error {
	if strings.TrimSpace(message.Text) == "" {
		return nil
	}

	var err error
	if strings.EqualFold(s.config.Driver, "signald") {
		err = s.sendViaSignald(ctx, message.RecipientId, message.Text)
	} else {
		err = s.sendViaSignalCli(ctx, message.RecipientId, message.Text)
	}

	if err != nil {
		s.logger.Error("Failed to send Signal message", "recipient", message.RecipientId, "error", err)
		return err
	}

	s.logSend(message.RecipientId)
	return nil
}

func (s *SignalChannel) logSend(recipient string) {
	if s.config.NoContentLogging {
		s.logger.Info("Sent Signal message [content redacted]", "recipient", recipient)
	} else {
		s.logger.Info("Sent Signal message", "recipient", recipient)
	}
}

func (s *SignalChannel) processSignaldMessage(ctx context.Context, data []byte) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		s.logger.Warn("Failed to process signald message", "error", err)
		return
	}

	msgType, _ := root["type"].(string)
	if !strings.EqualFold(msgType, "IncomingMessage") {
		return
	}

	dataMap, ok := root["data"].(map[string]any)
	if !ok {
		return
	}

	if dataMsg, ok := dataMap["data_message"].(map[string]any); ok {
		// 忽略群组消息
		if _, hasGroup := dataMsg["group"]; hasGroup {
			s.logger.Debug("Ignoring Signal group message (DM-only mode).")
			return
		}
		if _, hasGroupV2 := dataMsg["groupV2"]; hasGroupV2 {
			s.logger.Debug("Ignoring Signal group message (DM-only mode).")
			return
		}

		body, _ := dataMsg["body"].(string)
		source, _ := dataMap["source"].(string)

		if strings.TrimSpace(body) == "" || strings.TrimSpace(source) == "" {
			return
		}

		s.dispatchInbound(ctx, source, body)
	}
}

func (s *SignalChannel) sendViaSignald(ctx context.Context, recipient, text string) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", s.config.SocketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	payload := map[string]any{
		"type":     "send",
		"username": s.accountNumber,
		"recipientAddress": map[string]string{
			"number": recipient,
		},
		"messageBody": text,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	bytes = append(bytes, '\n')
	_, err = conn.Write(bytes)
	return err
}

func (s *SignalChannel) readCliOutput(ctx context.Context, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.processSignalCliMessage(ctx, scanner.Bytes())
	}

	if err := scanner.Err(); err != nil {
		s.logger.Error(err.Error())
	}
}

func (s *SignalChannel) processSignalCliMessage(ctx context.Context, data []byte) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		s.logger.Warn("Failed to process signal-cli message", "error", err)
		return
	}

	envelope, ok := root["envelope"].(map[string]any)
	if !ok {
		return
	}

	source, _ := envelope["sourceNumber"].(string)

	dataMsg, ok := envelope["dataMessage"].(map[string]any)
	if !ok {
		return
	}

	if _, hasGroup := dataMsg["groupInfo"]; hasGroup {
		s.logger.Debug("Ignoring Signal group message (DM-only mode).")
		return
	}

	body, _ := dataMsg["message"].(string)

	if strings.TrimSpace(body) == "" || strings.TrimSpace(source) == "" {
		return
	}

	s.dispatchInbound(ctx, source, body)
}

func (s *SignalChannel) sendViaSignalCli(ctx context.Context, recipient, text string) error {
	cliPath := s.config.SignalCliPath
	if cliPath == "" {
		cliPath = "signal-cli"
	}

	cmd := exec.CommandContext(ctx, cliPath, "-u", s.accountNumber, "send", "-m", text, recipient)
	return cmd.Run()
}

// ── 公用逻辑 ──────────────────────────────────────────────────

func (s *SignalChannel) dispatchInbound(ctx context.Context, senderNumber, text string) {
	// 白名单检查
	if len(s.config.AllowedFromNumbers) > 0 {
		allowed := slices.Contains(s.config.AllowedFromNumbers, senderNumber)
		if !allowed {
			return
		}
	}

	// 截断消息
	if s.config.MaxInboundChars > 0 && len(text) > s.config.MaxInboundChars {
		text = text[:s.config.MaxInboundChars]
	}

	if s.config.NoContentLogging {
		s.logger.Info("Received Signal message [content redacted]", "sender", senderNumber)
	} else {
		s.logger.Info("Received Signal message", "sender", senderNumber)
	}

	msg := core.InboundMessage{
		ChannelId: "signal",
		SenderId:  senderNumber,
		Text:      text,
		IsGroup:   false,
	}

	handler := s.onMessageReceived
	if handler != nil {
		if err := handler(ctx, &msg); err != nil {
			s.logger.Error("Error in message handler", "error", err)
		}
	}
}

func (s *SignalChannel) connectAndListenSignald(ctx context.Context) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", s.config.SocketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	s.logger.Info("Connected to signald", "path", s.config.SocketPath)

	// 订阅账号
	subscribeReq := fmt.Sprintf(`{"type":"subscribe","account":"%s"}`+"\n", s.accountNumber)
	if _, err := conn.Write([]byte(subscribeReq)); err != nil {
		return err
	}

	if s.config.TrustAllKeys {
		trustReq := fmt.Sprintf(`{"type":"trust","account":"%s","trust_level":"TRUSTED_UNVERIFIED"}`+"\n", s.accountNumber)
		if _, err := conn.Write([]byte(trustReq)); err != nil {
			return err
		}
	}

	reader := bufio.NewReader(conn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		s.processSignaldMessage(ctx, []byte(line))
	}
}
