package client

import (
	"context"
	"net/http"

	"github.com/futugyou/mcp/core"
)

var _ core.ITransport = (*AutoDetectingClientSessionTransport)(nil)

type AutoDetectingClientSessionTransport struct {
	options         *SseClientTransportOptions
	httpClient      *http.Client
	name            string
	messageChannel  chan core.JsonRpcMessage
	activeTransport core.ITransport
}

// SessionId implements [core.ITransport].
func (a *AutoDetectingClientSessionTransport) SessionId() string {
	panic("unimplemented")
}

func NewAutoDetectingClientSessionTransport(httpClient *http.Client, options *SseClientTransportOptions, name string) *AutoDetectingClientSessionTransport {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &AutoDetectingClientSessionTransport{
		options:        options,
		httpClient:     httpClient,
		name:           name,
		messageChannel: make(chan core.JsonRpcMessage),
	}
}

// Close implements core.ITransport.
func (a *AutoDetectingClientSessionTransport) Close() error {
	var err error
	if a.activeTransport != nil {
		err = a.activeTransport.Close()
	}
	if a.messageChannel != nil {
		close(a.messageChannel)
	}
	return err
}

// GetTransportKind implements core.ITransport.
func (a *AutoDetectingClientSessionTransport) GetTransportKind() core.TransportKind {
	// if a.activeTransport != nil {
	// 	return a.activeTransport.GetTransportKind()
	// }
	panic("active transport is nil")
}

// MessageReader implements core.ITransport.
func (a *AutoDetectingClientSessionTransport) MessageReader() <-chan core.JsonRpcMessage {
	return a.messageChannel
}

// SendMessage implements core.ITransport.
func (a *AutoDetectingClientSessionTransport) SendMessage(ctx context.Context, message core.JsonRpcMessage) error {
	if a.activeTransport == nil {
		return a.initialize(ctx, message)
	}

	return a.activeTransport.SendMessage(ctx, message)
}

// Close implements core.ITransport.
func (a *AutoDetectingClientSessionTransport) ActiveTransport() core.ITransport {
	return a.activeTransport
}

func (a *AutoDetectingClientSessionTransport) initialize(ctx context.Context, message core.JsonRpcMessage) error {
	// Try StreamableHttp first
	streamableHttpTransport := NewStreamableHttpClientSessionTransport(a.httpClient, a.options, a.name)
	err := streamableHttpTransport.SendMessage(ctx, message)
	if err == nil {
		// a.activeTransport = streamableHttpTransport
		return nil
	}

	streamableHttpTransport.Close()
	return a.initializeSseTransport(ctx, message)
}

func (s *AutoDetectingClientSessionTransport) initializeSseTransport(ctx context.Context, message core.JsonRpcMessage) error {
	// sseTransport := NewSseClientSessionTransport(s.name, s.options, s.httpClient, s.messageChannel)
	// err := sseTransport.Connect(ctx)
	// if err != nil {
	// 	sseTransport.Close()
	// 	return err
	// }
	// err = sseTransport.SendMessage(ctx, message)
	// if err != nil {
	// 	sseTransport.Close()
	// 	return err
	// }
	// s.activeTransport = sseTransport
	return nil
}
