package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type BridgeProcessLaunchSpec struct {
	FileName             string
	Arguments            []string
	WorkingDirectory     string
	EnvironmentVariables map[string]string
}

type BridgeNotificationHandler func(notify core.BridgeNotification) error

type IBridgeTransport interface {
	Prepare(ctx context.Context) error
	Start(ctx context.Context, process *os.Process) error
	SendRequest(ctx context.Context, method string, parameters any) error
	SendAndWait(ctx context.Context, method string, parameters any) (*core.BridgeResponse, error)
	SetNotificationHandler(handler BridgeNotificationHandler) error
}

type PluginBridgeMemorySnapshot struct {
	ProcessId          int
	WorkingSetBytes    int64
	PrivateMemoryBytes int64
}

type IPluginRuntimeTelemetrySource interface {
	TryGetRestartCount(pluginId string) (int, bool)
	TryGetMemorySnapshot(pluginId string) (*PluginBridgeMemorySnapshot, bool)
}

type SocketTransportOptions struct {
	SocketPath          string
	SocketDirectory     string
	OwnsSocketDirectory bool
	AuthToken           string
}

type bridgeResponseResult struct {
	msg *core.BridgeResponse
	err error
}

type BridgeTransportBase struct {
	pending             sync.Map //map[string]chan bridgeResponseResult
	logger              *slog.Logger
	nextId              atomic.Int32
	reader              io.Reader
	writer              *bufio.Writer
	notificationHandler BridgeNotificationHandler
	disposed            atomic.Bool

	done chan struct{}
}

func (b *BridgeTransportBase) Prepare(ctx context.Context) error { return nil }

func (b *BridgeTransportBase) AttachReaderWriter(reader io.Reader, writer *bufio.Writer) {
	b.reader = reader
	b.writer = writer

	b.done = make(chan struct{})

	go func() {
		defer close(b.done)
		b.readLoop(context.Background())
	}()
}

func (b *BridgeTransportBase) readLoop(ctx context.Context) {
	if b.reader == nil {
		return
	}

	reader := bufio.NewReader(b.reader)
	for {
		select {
		case <-ctx.Done():
			b.cancelPendingRequests(ctx.Err())
			return
		default:
		}
		if b.disposed.Load() {
			break
		}

		line, err := reader.ReadString('\n')
		lineTrimmed := strings.TrimSpace(line)

		if len(lineTrimmed) > 0 {
			b.processLine(lineTrimmed)
		}

		if err != nil {
			break
		}
	}

	b.cancelPendingRequests(context.Canceled)
}

func (b *BridgeTransportBase) processLine(line string) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &rawMap); err != nil {
		if b.logger != nil {
			b.logger.Warn("Plugin bridge emitted malformed JSON", "line", util.Truncate(line, 200), "err", err)
		}
		return
	}

	if _, isNotification := rawMap["notification"]; isNotification {
		var notify core.BridgeNotification
		if err := json.Unmarshal([]byte(line), &notify); err == nil {
			if b.notificationHandler != nil {
				b.notificationHandler(notify)
			}
		}
	} else {
		var response core.BridgeResponse
		if err := json.Unmarshal([]byte(line), &response); err == nil && len(response.Id) > 0 {
			if value, loaded := b.pending.LoadAndDelete(response.Id); loaded {
				if done, ok := value.(chan bridgeResponseResult); ok {
					select {
					case done <- bridgeResponseResult{msg: &response}:
					default:
					}
				}
			}
		}
	}
}

func (p *BridgeTransportBase) cancelPendingRequests(err error) {
	p.pending.Range(func(key, value any) bool {
		if p.pending.CompareAndDelete(key, value) {
			if done, ok := value.(chan bridgeResponseResult); ok {
				select {
				case done <- bridgeResponseResult{err: err}:
				default:
				}
			}
		}
		return true
	})
}

func (p *BridgeTransportBase) SendAndWait(ctx context.Context, method string, parameters any) (*core.BridgeResponse, error) {
	if p.writer == nil {
		return nil, errors.New("bridge transport is not ready")
	}

	id := strconv.Itoa(int(p.nextId.Add(1)))

	done := make(chan bridgeResponseResult, 1)
	p.pending.Store(id, done)
	defer p.pending.Delete(id)

	req := core.BridgeRequest{
		Method: method,
		Id:     id,
		Params: parameters,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	if _, err := p.writer.Write(append(reqBytes, '\n')); err != nil {
		return nil, fmt.Errorf("write request failed: %w", err)
	}
	if err := p.writer.Flush(); err != nil {
		return nil, fmt.Errorf("flush writer failed: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	select {
	case res := <-done:
		return res.msg, res.err

	case <-timeoutCtx.Done():
		select {
		case res := <-done:
			return res.msg, res.err
		default:
		}

		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("request timed out after 60s: %w", timeoutCtx.Err())
	}
}

func (p *BridgeTransportBase) SendRequest(ctx context.Context, method string, parameters any) error {
	if p.writer == nil {
		return errors.New("bridge transport is not ready")
	}

	id := strconv.Itoa(int(p.nextId.Add(1)))

	done := make(chan bridgeResponseResult, 1)
	p.pending.Store(id, done)
	defer p.pending.Delete(id)

	req := core.BridgeRequest{
		Method: method,
		Id:     id,
		Params: parameters,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	if _, err := p.writer.Write(append(reqBytes, '\n')); err != nil {
		return fmt.Errorf("write request failed: %w", err)
	}

	return p.writer.Flush()
}

func (p *BridgeTransportBase) Close() error {
	if !p.disposed.CompareAndSwap(false, true) {
		return nil
	}

	p.cancelPendingRequests(context.Canceled)
	p.CloseCore()
	if p.done != nil {
		select {
		case <-p.done:
		case <-time.After(3 * time.Second):
		}
	}

	return nil
}

func (p *BridgeTransportBase) CloseCore() error { return nil }
