package server

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/futugyou/mcp/core"
	"github.com/futugyou/mcp/shared"
)

var _ core.ITransport = (*SseResponseStreamTransport)(nil)

type SseResponseStreamTransport struct {
	sseResponseStream io.Writer
	messageEndpoint   string

	incomingChannel chan core.JsonRpcMessage

	isConnected bool
	mu          sync.Mutex
	sseWriter   *shared.SseWriter
}

// SessionId implements [core.ITransport].
func (t *SseResponseStreamTransport) SessionId() string {
	panic("unimplemented")
}

// GetTransportKind implements ITransport.
func (t *SseResponseStreamTransport) GetTransportKind() core.TransportKind {
	return core.TransportKindSse
}

func NewSseResponseStreamTransport(sseResponseStream io.Writer, messageEndpoint string) *SseResponseStreamTransport {
	if messageEndpoint == "" {
		messageEndpoint = "/message"
	}

	return &SseResponseStreamTransport{
		sseResponseStream: sseResponseStream,
		messageEndpoint:   messageEndpoint,
		incomingChannel:   make(chan core.JsonRpcMessage, 1), // Buffered channel
		sseWriter:         shared.NewSseWriter(messageEndpoint),
	}
}

// Close implements I
func (t *SseResponseStreamTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isConnected {
		return nil // Already closed
	}

	t.isConnected = false

	// Close channels
	close(t.incomingChannel)

	t.sseWriter.Dispose()
	return nil
}

// MessageReader implements I
func (s *SseResponseStreamTransport) MessageReader() <-chan core.JsonRpcMessage {
	return s.incomingChannel
}

// SendMessage implements I
func (t *SseResponseStreamTransport) SendMessage(ctx context.Context, message core.JsonRpcMessage) error {
	return t.sseWriter.SendMessage(ctx, message)
}

// Run starts the transport and writes the JSON-RPC messages to the SSE response stream
func (t *SseResponseStreamTransport) Run(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.isConnected = true

	errCh := t.sseWriter.WriteAll(ctx, t.sseResponseStream)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// OnMessageReceived handles incoming JSON-RPC messages
func (t *SseResponseStreamTransport) OnMessageReceived(ctx context.Context, message core.JsonRpcMessage) error {
	t.mu.Lock()
	if !t.isConnected {
		t.mu.Unlock()
		return fmt.Errorf("transport is not connected. Make sure to call Run first")
	}
	t.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case t.incomingChannel <- message:
		return nil
	}
}
