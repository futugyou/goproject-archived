package server

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/futugyou/mcp/core"
	"github.com/futugyou/mcp/shared"
)

var _ core.ITransport = (*StreamableHttpServerTransport)(nil)

type StreamableHttpServerTransport struct {
	sseWriter         *shared.SseWriter
	Stateless         bool
	InitializeRequest core.InitializeRequestParams
	incomingChannel   chan core.JsonRpcMessage
	ctx               context.Context
	cancelFunc        context.CancelFunc
	getRequestStarted int32
}

// SessionId implements [core.ITransport].
func (s *StreamableHttpServerTransport) SessionId() string {
	panic("unimplemented")
}

// GetTransportKind implements ITransport.
func (s *StreamableHttpServerTransport) GetTransportKind() core.TransportKind {
	return core.TransportKindHttp
}

func NewStreamableHttpServerTransport() *StreamableHttpServerTransport {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &StreamableHttpServerTransport{
		sseWriter:         shared.NewSseWriter(""),
		incomingChannel:   make(chan core.JsonRpcMessage),
		ctx:               ctx,
		cancelFunc:        cancelFunc,
		getRequestStarted: 0,
	}
}

// Close implements ITransport.
func (s *StreamableHttpServerTransport) Close() error {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	s.sseWriter.Dispose()
	return nil
}

// MessageReader implements ITransport.
func (s *StreamableHttpServerTransport) MessageReader() <-chan core.JsonRpcMessage {
	return s.incomingChannel
}

// SendMessage implements ITransport.
func (s *StreamableHttpServerTransport) SendMessage(ctx context.Context, message core.JsonRpcMessage) error {
	if s.Stateless {
		return fmt.Errorf("stateless mode is not supported for GET requests")
	}

	return s.sseWriter.SendMessage(ctx, message)
}

func (s *StreamableHttpServerTransport) HandleGetRequest(ctx context.Context, sseResponseStream io.Writer) error {
	if s.Stateless {
		return fmt.Errorf("stateless mode is not supported for GET requests")
	}

	if atomic.SwapInt32(&s.getRequestStarted, 1) != 0 {
		return fmt.Errorf("session resumption is not yet supported. Please start a new session")
	}

	// We do not need to reference ctx like in HandlePostRequest, because the session ending completes the sseWriter gracefully.
	resultCh := s.sseWriter.WriteAll(ctx, sseResponseStream)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-resultCh:
		return err
	}
}

func (s *StreamableHttpServerTransport) HandlePostRequest(ctx context.Context, httpBodies *shared.DuplexPipe) (bool, error) {
	ctx, _ = core.MergeContexts(s.ctx, ctx)
	postTransport := NewStreamableHttpPostTransport(s, httpBodies)
	return postTransport.Run(ctx)
}
