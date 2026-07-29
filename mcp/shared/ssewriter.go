package shared

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/futugyou/mcp/core"
	"github.com/futugyou/yomawari/runtime/sse"
)

type SseWriter struct {
	messages        chan sse.SseItem[core.JsonRpcMessage]
	mu              sync.Mutex
	ctx             context.Context
	cancelFunc      context.CancelFunc
	messageEndpoint string
	task            chan error
	disposed        bool
	MessageFilter   func(ctx context.Context, mesg chan sse.SseItem[core.JsonRpcMessage]) chan sse.SseItem[core.JsonRpcMessage]
}

func NewSseWriter(messageEndpoint string) *SseWriter {
	return &SseWriter{
		messages:        make(chan sse.SseItem[core.JsonRpcMessage]),
		messageEndpoint: messageEndpoint,
	}
}

func (s *SseWriter) WriteAll(ctx context.Context, sseResponseStream io.Writer) chan error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.task != nil {
		select {
		case <-s.task:
		default:
			s.task <- fmt.Errorf("WriteAll already called")
			return s.task
		}
	}
	s.task = make(chan error, 1)

	if len(s.messageEndpoint) > 0 {
		select {
		case s.messages <- *sse.NewSseItem[core.JsonRpcMessage](core.JsonRpcMessage{}, "endpoint"):
		default:
			s.task <- fmt.Errorf("you must call RunAsync before calling SendMessage")
			return s.task
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	s.ctx = ctx
	s.cancelFunc = cancel

	var msg = s.messages
	if s.MessageFilter != nil {
		msg = s.MessageFilter(ctx, msg)
	}

	go func() {
		s.task <- sse.Write(ctx, msg, sseResponseStream, s.writeJsonRpcMessageToBuffer())
	}()

	return s.task
}

func (s *SseWriter) writeJsonRpcMessageToBuffer() sse.ItemFormatter[core.JsonRpcMessage] {
	return func(item sse.SseItem[core.JsonRpcMessage], writer *bufio.Writer) error {
		if item.EventType == "endpoint" && len(s.messageEndpoint) > 0 {
			_, err := fmt.Fprintf(writer, "%s", base64.URLEncoding.EncodeToString([]byte(s.messageEndpoint)))
			return err
		}
		d, err := json.Marshal(item.Data)
		if err != nil {
			return err
		}
		_, err = writer.Write(d)
		return err
	}
}

func (s *SseWriter) SendMessage(ctx context.Context, message core.JsonRpcMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.disposed {
		return fmt.Errorf("sseWriter is disposed")
	}

	var err error
	select {
	case s.messages <- *sse.NewSseItem[core.JsonRpcMessage](message, "message"):
	default:
		err = fmt.Errorf("something went wrong sending the message")
	}

	return err
}

func (s *SseWriter) Dispose() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.disposed {
		return
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}

	close(s.messages)

	if s.task != nil {
		select {
		case <-s.task:
		default:
			s.task <- nil
		}
		close(s.task)
		s.task = nil
	}

	s.disposed = true
}
