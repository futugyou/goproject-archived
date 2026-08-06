package agent

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Microsoft/go-winio"
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
	Start(ctx context.Context, process *exec.Cmd) error
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

func (p *BridgeTransportBase) SetNotificationHandler(handler BridgeNotificationHandler) error {
	p.notificationHandler = handler
	return nil
}

var _ IBridgeTransport = (*SocketBridgeTransport)(nil)

type SocketBridgeTransport struct {
	BridgeTransportBase

	socketPath          string
	socketDirectory     string
	ownsSocketDirectory bool
	authToken           string
	metrics             *core.RuntimeMetrics
	pipeName            string

	listener net.Listener
	conn     net.Conn

	mu sync.Mutex // 保护资源清理
}

func NewSocketBridgeTransport(
	socketPath string,
	socketDirectory string,
	ownsSocketDirectory bool,
	authToken string,
	logger *slog.Logger,
	metrics *core.RuntimeMetrics,
) *SocketBridgeTransport {
	t := &SocketBridgeTransport{
		BridgeTransportBase: BridgeTransportBase{
			logger: logger,
		},
		socketPath:          socketPath,
		socketDirectory:     socketDirectory,
		ownsSocketDirectory: ownsSocketDirectory,
		authToken:           authToken,
		metrics:             metrics,
	}

	if runtime.GOOS == "windows" {
		t.pipeName = normalizePipeName(socketPath)
	}

	return t
}

// Prepare 准备 Socket/NamedPipe 监听器
func (s *SocketBridgeTransport) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		pipePath := `\\.\pipe\` + s.pipeName
		l, err := winio.ListenPipe(pipePath, nil)
		if err != nil {
			return fmt.Errorf("failed to listen on windows pipe: %w", err)
		}
		s.listener = l
		return nil
	}

	// Unix 逻辑
	dir := filepath.Dir(s.socketPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		s.tryRestrictUnixDirectory(dir)
	}

	// 如果旧的 socket 文件已存在则先删除
	_ = os.Remove(s.socketPath)

	l, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}
	s.listener = l

	// 轮询等待 Socket 文件生成，避免测试高负载下的 connection refused 竞态
	for range 20 {
		if err := ctx.Err(); err != nil {
			_ = l.Close()
			return err
		}
		if _, err := os.Stat(s.socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// Start 启动监听并开始认证连接
func (s *SocketBridgeTransport) Start(ctx context.Context, proc *exec.Cmd) error {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.listener == nil {
		return errors.New("listener is not prepared")
	}

	// 异步监听 Context 取消，以强行关闭 Listener 解除 Accept 阻塞
	go func() {
		<-connectCtx.Done()
		if connectCtx.Err() == context.DeadlineExceeded || errors.Is(connectCtx.Err(), context.Canceled) {
			s.mu.Lock()
			if s.conn == nil { // 如果还没成功连上，关闭 listener 终止 Accept
				_ = s.listener.Close()
			}
			s.mu.Unlock()
		}
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-connectCtx.Done():
				return fmt.Errorf("connection timed out or canceled: %w", connectCtx.Err())
			default:
				return fmt.Errorf("accept failed: %w", err)
			}
		}

		reader, writer, ok := s.tryAuthenticateStream(connectCtx, conn)
		if !ok {
			_ = conn.Close()
			continue
		}

		s.mu.Lock()
		s.conn = conn
		s.mu.Unlock()

		// 绑定 Reader & Writer 到基类并启动 ReadLoop
		s.AttachReaderWriter(reader, writer)
		break
	}

	return nil
}

// CloseCore 重写基类的清理逻辑，回收文件与连接
func (s *SocketBridgeTransport) CloseCore() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}

	if runtime.GOOS != "windows" {
		if _, err := os.Stat(s.socketPath); err == nil {
			if err := os.Remove(s.socketPath); err != nil && s.logger != nil {
				s.logger.Debug("Failed to remove bridge socket path", "path", s.socketPath, "err", err)
			}
		}

		if s.ownsSocketDirectory && s.socketDirectory != "" {
			if err := os.RemoveAll(s.socketDirectory); err != nil && s.logger != nil {
				s.logger.Debug("Failed to remove bridge socket directory", "dir", s.socketDirectory, "err", err)
			}
		}
	}

	return nil
}

// 尝试认证 Client 连接
func (s *SocketBridgeTransport) tryAuthenticateStream(ctx context.Context, conn net.Conn) (io.Reader, *bufio.Writer, bool) {
	reader, writer, err := s.authenticateStream(ctx, conn)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncrementPluginBridgeAuthFailures()
		}
		if s.logger != nil {
			s.logger.Warn("Rejected unauthenticated local IPC client", "path", s.socketPath, "err", err)
		}
		return nil, nil, false
	}
	return reader, writer, true
}

func (s *SocketBridgeTransport) authenticateStream(ctx context.Context, conn net.Conn) (io.Reader, *bufio.Writer, error) {
	bufReader := bufio.NewReader(conn)
	bufWriter := bufio.NewWriter(conn)

	lineChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			errChan <- err
			return
		}
		lineChan <- line
	}()

	var line string
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case err := <-errChan:
		return nil, nil, err
	case line = <-lineChan:
	}

	if !s.isExpectedAuthLine(line) {
		return nil, nil, errors.New("bridge client failed local IPC authentication")
	}

	return bufReader, bufWriter, nil
}

type AuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// 校验 Auth Token 是否准确（使用防时序攻击的 ConstantTimeCompare）
func (s *SocketBridgeTransport) isExpectedAuthLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	var auth AuthMessage
	if err := json.Unmarshal([]byte(line), &auth); err != nil {
		return false
	}

	if auth.Type != "bridge_auth" || auth.Token == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(auth.Token), []byte(s.authToken)) == 1
}

func (s *SocketBridgeTransport) tryRestrictUnixDirectory(path string) {
	if runtime.GOOS == "windows" {
		return
	}
	// chmod 0700 (read | write | execute)
	_ = os.Chmod(path, 0700)
}

func normalizePipeName(socketPath string) string {
	prefix := `\\.\pipe\`
	if strings.HasPrefix(strings.ToLower(socketPath), strings.ToLower(prefix)) {
		return socketPath[len(prefix):]
	}

	sanitized := strings.ReplaceAll(socketPath, `\`, "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")

	return strings.Trim(sanitized, "-")
}

var _ IBridgeTransport = (*StdioBridgeTransport)(nil)

type StdioBridgeTransport struct {
	BridgeTransportBase
}

func NewStdioBridgeTransport(logger *slog.Logger) *StdioBridgeTransport {
	return &StdioBridgeTransport{BridgeTransportBase: BridgeTransportBase{
		logger: logger,
	}}
}

func (s *StdioBridgeTransport) Start(ctx context.Context, cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	writer := bufio.NewWriter(stdin)

	s.AttachReaderWriter(stdout, writer)

	return nil
}

var _ IBridgeTransport = (*HybridBridgeTransport)(nil)

type HybridBridgeTransport struct {
	bootstrap      *StdioBridgeTransport
	socket         *SocketBridgeTransport
	logger         *slog.Logger
	currentHandler BridgeNotificationHandler
	useSocket      atomic.Bool
}

func NewHybridBridgeTransport(socketPath, socketDir string, ownsDir bool, authToken string, logger *slog.Logger, metrics *core.RuntimeMetrics) *HybridBridgeTransport {
	return &HybridBridgeTransport{
		logger:    logger,
		bootstrap: NewStdioBridgeTransport(logger),
		socket:    NewSocketBridgeTransport(socketPath, socketDir, ownsDir, authToken, logger, metrics),
	}
}

func (h *HybridBridgeTransport) Prepare(ctx context.Context) error {
	return h.socket.Prepare(ctx)
}

func (h *HybridBridgeTransport) Start(ctx context.Context, process *exec.Cmd) error {
	if err := h.bootstrap.Start(ctx, process); err != nil {
		return err
	}

	return h.socket.Start(ctx, process)
}

func (h *HybridBridgeTransport) UseSocketTransport() {
	h.useSocket.Store(true)
	if h.bootstrap != nil {
		h.bootstrap.SetNotificationHandler(nil)
	}
}

func (h *HybridBridgeTransport) SetNotificationHandler(handler BridgeNotificationHandler) error {
	h.currentHandler = handler
	if h.useSocket.Load() {
		return h.socket.SetNotificationHandler(handler)
	} else {
		if err := h.bootstrap.SetNotificationHandler(handler); err != nil {
			return err
		}

		return h.socket.SetNotificationHandler(handler)
	}
}

func (h *HybridBridgeTransport) SendAndWait(ctx context.Context, method string, parameters any) (*core.BridgeResponse, error) {
	if !h.useSocket.Load() {
		return h.bootstrap.SendAndWait(ctx, method, parameters)
	}

	response, err := h.socket.SendAndWait(ctx, method, parameters)
	if err == nil {
		return response, nil
	}

	if h.isFallbackError(err) || method == "shutdown" {
		h.logger.Warn("socket transport failed, falling back to stdio", "method", method, "error", err.Error())
		h.useSocket.Store(false)
		if h.currentHandler != nil {
			h.bootstrap.SetNotificationHandler(h.currentHandler)
		}

		return h.bootstrap.SendAndWait(ctx, method, parameters)
	}

	return nil, err
}

func (b *HybridBridgeTransport) isFallbackError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

func (h *HybridBridgeTransport) SendRequest(ctx context.Context, method string, parameters any) error {
	_, err := h.SendAndWait(ctx, method, parameters)
	return err
}

func (h *HybridBridgeTransport) Close() error {
	h.socket.Close()
	h.bootstrap.Close()

	return nil
}
