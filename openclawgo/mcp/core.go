package mcp

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/futugyou/mcp/protocol"
)

var _ protocol.ITransport = (*InMemoryDuplexTransport)(nil)

type InMemoryDuplexTransport struct {
	outgoing  chan protocol.IJsonRpcMessage
	incoming  <-chan protocol.IJsonRpcMessage
	name      string
	disposed  atomic.Int32
	SessionId string
}

func NewInMemoryDuplexTransport(
	outgoing chan protocol.IJsonRpcMessage,
	incoming <-chan protocol.IJsonRpcMessage,
	name, sessionId string,
) *InMemoryDuplexTransport {
	return &InMemoryDuplexTransport{
		outgoing:  outgoing,
		incoming:  incoming,
		name:      name,
		SessionId: sessionId,
	}
}

// GetTransportKind implements [protocol.ITransport].
func (i *InMemoryDuplexTransport) GetTransportKind() protocol.TransportKind {
	return protocol.TransportKindSse
}

// MessageReader implements [protocol.ITransport].
func (i *InMemoryDuplexTransport) MessageReader() <-chan protocol.IJsonRpcMessage {
	return i.incoming
}

// SendMessage implements [protocol.ITransport].
func (i *InMemoryDuplexTransport) SendMessage(ctx context.Context, message protocol.IJsonRpcMessage) error {
	if i.disposed.Load() == 1 {
		return fmt.Errorf("transport '%s' has been disposed", i.name)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case i.outgoing <- message:
		return nil
	}
}

// Close implements [protocol.ITransport].
func (i *InMemoryDuplexTransport) Close() error {
	if !i.disposed.CompareAndSwap(0, 1) {
		return nil
	}

	if i.outgoing != nil {
		close(i.outgoing)
	}

	return nil
}
