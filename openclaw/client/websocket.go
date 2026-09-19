package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/gorilla/websocket"
)

type OpenClawWebSocketClient struct {
	maxMessageBytes int64

	mu       sync.Mutex
	sendMu   sync.Mutex
	ws       *websocket.Conn
	cancelRx context.CancelFunc
	rxDone   chan struct{}

	OnEnvelopeReceived func(envelope *core.WsServerEnvelope)
	OnTextMessage      func(text string)
	OnError            func(err error)
}

func NewOpenClawWebSocketClient(maxMessageBytes int64) *OpenClawWebSocketClient {
	if maxMessageBytes <= 0 {
		maxMessageBytes = 256 * 1024
	}
	return &OpenClawWebSocketClient{
		maxMessageBytes: maxMessageBytes,
	}
}

func (c *OpenClawWebSocketClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws != nil
}

func (c *OpenClawWebSocketClient) Connect(
	ctx context.Context,
	wsUri *url.URL,
	bearerToken string,
) error {
	if err := c.Disconnect(ctx); err != nil {
	}

	header := make(http.Header)
	if strings.TrimSpace(bearerToken) != "" {
		header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	ws, _, err := dialer.DialContext(ctx, wsUri.String(), header)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	ws.SetReadLimit(c.maxMessageBytes)

	rxCtx, rxCancel := context.WithCancel(context.Background())
	rxDone := make(chan struct{})

	c.mu.Lock()
	c.ws = ws
	c.cancelRx = rxCancel
	c.rxDone = rxDone
	c.mu.Unlock()

	go c.receiveLoop(rxCtx, ws, rxDone)

	return nil
}

func (c *OpenClawWebSocketClient) Disconnect(ctx context.Context) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.mu.Lock()
	ws := c.ws
	rxDone := c.rxDone

	c.ws = nil
	c.cancelRx = nil
	c.rxDone = nil
	c.mu.Unlock()

	if ws == nil {
		return nil
	}

	_ = ws.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closing"),
		time.Now().Add(time.Second),
	)

	err := ws.Close()

	if rxDone != nil {
		select {
		case <-rxDone:
		case <-ctx.Done():
		}
	}

	return err
}

func (c *OpenClawWebSocketClient) SendUserMessage(ctx context.Context, text, messageId, replyToMessageId string) error {
	return c.sendEnvelope(ctx, core.WsClientEnvelope{
		Type:             "user_message",
		Text:             text,
		MessageId:        messageId,
		ReplyToMessageId: replyToMessageId,
	})
}

func (c *OpenClawWebSocketClient) sendEnvelope(ctx context.Context, envelope core.WsClientEnvelope) error {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("serialize envelope failed: %w", err)
	}

	if int64(len(payload)) > c.maxMessageBytes {
		return errors.New("message too large")
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.mu.Lock()
	ws := c.ws
	c.mu.Unlock()

	if ws == nil {
		return errors.New("websocket is not connected")
	}

	done := make(chan error, 1)
	go func() {
		done <- ws.WriteMessage(websocket.TextMessage, payload)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (c *OpenClawWebSocketClient) receiveLoop(ctx context.Context, ws *websocket.Conn, done chan<- struct{}) {
	defer close(done)

	stopCtxMonitor := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ws.Close()
		case <-stopCtxMonitor:
			return
		}
	}()
	defer close(stopCtxMonitor)

	for {
		msgType, reader, err := ws.NextReader()
		if ctx.Err() != nil {
			return
		}

		if err != nil {
			if c.OnError != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				c.OnError(err)
			}
			return
		}

		if msgType != websocket.TextMessage {
			continue
		}

		buf := new(bytes.Buffer)
		written, err := buf.ReadFrom(reader)
		if err != nil {
			if c.OnError != nil {
				c.OnError(err)
			}
			continue
		}

		if written > c.maxMessageBytes {
			if c.OnError != nil {
				c.OnError(errors.New("inbound message too large"))
			}
			continue
		}

		payload := buf.Bytes()
		if c.OnTextMessage != nil {
			text := string(payload)
			func() {
				defer func() {
					if r := recover(); r != nil && c.OnError != nil {
						c.OnError(fmt.Errorf("panic in OnTextMessage callback: %v", r))
					}
				}()
				c.OnTextMessage(text)
			}()
		}

		var envelope core.WsServerEnvelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			if c.OnError != nil {
				c.OnError(fmt.Errorf("json unmarshal failed: %w", err))
			}
			continue
		}

		if c.OnEnvelopeReceived != nil {
			func() {
				defer func() {
					if r := recover(); r != nil && c.OnError != nil {
						c.OnError(fmt.Errorf("panic in OnEnvelopeReceived callback: %v", r))
					}
				}()
				c.OnEnvelopeReceived(&envelope)
			}()
		}
	}
}

func (c *OpenClawWebSocketClient) Close() error {
	return c.Disconnect(context.Background())
}
