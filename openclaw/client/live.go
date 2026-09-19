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

type OpenClawLiveClient struct {
	maxMessageBytes int64

	mu       sync.Mutex // 保护 ws、cancelRx 状态
	sendMu   sync.Mutex // 确保单次并发写入安全的锁
	ws       *websocket.Conn
	cancelRx context.CancelFunc
	rxDone   chan struct{}

	OnEnvelopeReceived func(envelope *core.LiveServerEnvelope)
	OnTextChunk        func(text string)
	OnError            func(err error)
}

func NewOpenClawLiveClient(maxMessageBytes int64) *OpenClawLiveClient {
	if maxMessageBytes <= 0 {
		maxMessageBytes = 512 * 1024 // 默认 512 KB
	}
	return &OpenClawLiveClient{
		maxMessageBytes: maxMessageBytes,
	}
}

// IsConnected 检查 WebSocket 是否已连接
func (c *OpenClawLiveClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws != nil
}

// BuildWebSocketURI 构建用于连接的 WebSocket URL
func BuildWebSocketURI(baseUrlStr string) (*url.URL, error) {
	if strings.TrimSpace(baseUrlStr) == "" {
		return nil, errors.New("base URL is required")
	}

	baseUri, err := url.Parse(strings.TrimRight(baseUrlStr, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %s, err: %w", baseUrlStr, err)
	}

	scheme := "ws"
	if strings.EqualFold(baseUri.Scheme, "https") {
		scheme = "wss"
	}

	u := &url.URL{
		Scheme:   scheme,
		Host:     baseUri.Host,
		Path:     "/ws/live",
		RawQuery: "",
	}
	return u, nil
}

// Connect 发起 WebSocket 连接并开始接收消息循环
func (c *OpenClawLiveClient) Connect(
	ctx context.Context,
	wsUri *url.URL,
	bearerToken string,
	request core.LiveSessionOpenRequest,
) error {
	// 连接前先断开旧连接
	if err := c.Disconnect(ctx); err != nil {
		// 忽略断开异常，继续重新连接
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

	// 发送握手/初始化请求
	payload, err := json.Marshal(request)
	if err != nil {
		ws.Close()
		return fmt.Errorf("failed to serialize open request: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, payload); err != nil {
		ws.Close()
		return fmt.Errorf("failed to send open request: %w", err)
	}

	rxCtx, rxCancel := context.WithCancel(context.Background())
	rxDone := make(chan struct{})

	c.mu.Lock()
	c.ws = ws
	c.cancelRx = rxCancel
	c.rxDone = rxDone
	c.mu.Unlock()

	// 启动后台接收 Loop
	go c.receiveLoop(rxCtx, ws, rxDone)

	return nil
}

// SendText 发送文本消息
func (c *OpenClawLiveClient) SendText(ctx context.Context, text string, turnComplete bool) error {
	return c.sendEnvelope(ctx, core.LiveClientEnvelope{
		Type:         "text",
		Text:         text,
		TurnComplete: turnComplete,
	})
}

// SendAudio 发送 Audio 消息
func (c *OpenClawLiveClient) SendAudio(ctx context.Context, base64Data, mimeType string, turnComplete bool) error {
	return c.sendEnvelope(ctx, core.LiveClientEnvelope{
		Type:         "audio",
		Base64Data:   base64Data,
		MimeType:     mimeType,
		TurnComplete: turnComplete,
	})
}

// Interrupt 发送打断请求
func (c *OpenClawLiveClient) Interrupt(ctx context.Context) error {
	return c.sendEnvelope(ctx, core.LiveClientEnvelope{
		Type: "interrupt",
	})
}

// CloseSession 关闭 Session 并断开连接
func (c *OpenClawLiveClient) CloseSession(ctx context.Context) error {
	_ = c.sendEnvelope(ctx, core.LiveClientEnvelope{Type: "close"})
	return c.Disconnect(ctx)
}

// Disconnect 断开 WebSocket 连接并清理后台任务
func (c *OpenClawLiveClient) Disconnect(ctx context.Context) error {
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

// sendEnvelope 序列化并发送消息包
func (c *OpenClawLiveClient) sendEnvelope(ctx context.Context, envelope core.LiveClientEnvelope) error {
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
		return errors.New("live websocket is not connected")
	}

	// 支持通过 ctx 取消发送
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

// receiveLoop 接收消息的主循环 Goroutine
func (c *OpenClawLiveClient) receiveLoop(ctx context.Context, ws *websocket.Conn, done chan<- struct{}) {
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
		// 读取整条消息，限制大小防止 OOM
		written, err := buf.ReadFrom(reader)
		if err != nil {
			if c.OnError != nil {
				c.OnError(err)
			}
			continue
		}

		if written > c.maxMessageBytes {
			if c.OnError != nil {
				c.OnError(errors.New("inbound live message too large"))
			}
			continue
		}

		var envelope core.LiveServerEnvelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			if c.OnError != nil {
				c.OnError(fmt.Errorf("json unmarshal failed: %w", err))
			}
			continue
		}

		// 触发事件通知
		if strings.EqualFold(envelope.Type, "text") && strings.TrimSpace(envelope.Text) != "" {
			if c.OnTextChunk != nil {
				c.OnTextChunk(envelope.Text)
			}
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

func (c *OpenClawLiveClient) Close() error {
	return c.Disconnect(context.Background())
}
