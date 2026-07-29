package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

type ITransport interface {
	SessionId() string
	MessageReader() <-chan JsonRpcMessage
	SendMessage(ctx context.Context, message JsonRpcMessage) error
	Close() error
}

type TransportKind string

var TransportKindUnknown TransportKind = "unknownTransport"
var TransportKindStdio TransportKind = "stdio"
var TransportKindStream TransportKind = "stream"
var TransportKindSse TransportKind = "sse"
var TransportKindHttp TransportKind = "http"

const (
	stateInitial int32 = iota
	stateConnected
	stateDisconnected
)

type TransportBase struct {
	name           string
	sessionId      string
	state          int32 // 使用 atomic 操作保护状态转换
	messageChannel chan JsonRpcMessage
	logger         *slog.Logger

	mu sync.RWMutex
}

func NewTransportBase(name string, messageChannel chan JsonRpcMessage, logger *slog.Logger) *TransportBase {
	if logger == nil {
		logger = slog.Default()
	}

	if messageChannel == nil {
		messageChannel = make(chan JsonRpcMessage, 1024)
	}

	return &TransportBase{
		name:           name,
		messageChannel: messageChannel,
		state:          stateInitial,
		logger:         logger,
	}
}

func (t *TransportBase) SessionId() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessionId
}

func (t *TransportBase) SetSessionId(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionId = sessionID
}

func (t *TransportBase) IsConnected() bool {
	return atomic.LoadInt32(&t.state) == stateConnected
}

func (t *TransportBase) MessageReader() <-chan JsonRpcMessage {
	return t.messageChannel
}

func (t *TransportBase) WriteMessage(ctx context.Context, message JsonRpcMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !t.IsConnected() {
		return nil
	}

	t.logger.Debug("Transport received message", "name", t.name)

	select {
	case t.messageChannel <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		if !t.IsConnected() {
			return nil
		}
		select {
		case t.messageChannel <- message:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (t *TransportBase) SetConnected() error {
	for {
		currentState := atomic.LoadInt32(&t.state)
		switch currentState {
		case stateInitial:
			if atomic.CompareAndSwapInt32(&t.state, stateInitial, stateConnected) {
				return nil
			}
		case stateConnected:
			return nil
		case stateDisconnected:
			return errors.New("transport is already disconnected and can't be reconnected")
		default:
			return fmt.Errorf("unexpected state: %d", currentState)
		}
	}
}

// disconnected and close channel
func (t *TransportBase) SetDisconnected(err error) {
	for {
		currentState := atomic.LoadInt32(&t.state)
		if currentState == stateDisconnected {
			return
		}

		if atomic.CompareAndSwapInt32(&t.state, currentState, stateDisconnected) {
			// close channel when first change state to desconnected
			close(t.messageChannel)
			if err != nil {
				t.logger.Error("Transport disconnected with error", "error", err)
			}
			return
		}
	}
}

func (t *TransportBase) SendMessage(ctx context.Context, message JsonRpcMessage) error {
	panic("abstract method SendMessage must be implemented by subclass")
}

func (t *TransportBase) Close() error {
	t.SetDisconnected(nil)
	return nil
}

var _ ITransport = (*TransportBase)(nil)

func MergeContexts(ctx1, ctx2 context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-ctx1.Done():
			cancel()
		case <-ctx2.Done():
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
