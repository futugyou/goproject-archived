package agent

import (
	"bufio"
	"bytes"
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
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
	"github.com/shirou/gopsutil/v4/process"
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
	Close() error
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

type PluginBridgeProcess struct {
	mu                  sync.Mutex
	process             *exec.Cmd
	bridgeScriptPath    string
	logger              *slog.Logger
	transportConfig     core.BridgeTransportConfig
	launchSpec          *BridgeProcessLaunchSpec
	runtimeRoot         string
	metrics             *core.RuntimeMetrics
	transport           IBridgeTransport
	runtimeTransport    *core.BridgeTransportRuntimeConfig
	entryPath           string
	pluginID            string
	pluginConfig        *json.RawMessage
	notificationHandler BridgeNotificationHandler

	initialized         atomic.Bool
	disposed            atomic.Bool
	intentionalShutdown atomic.Bool
	restartCount        atomic.Int32

	// Exit notification signaling channel
	exitChan chan struct{}
}

func NewPluginBridgeProcess(
	bridgeScriptPath string,
	logger *slog.Logger,
	transportConfig *core.BridgeTransportConfig,
	launchSpec *BridgeProcessLaunchSpec,
	runtimeRoot string,
	metrics *core.RuntimeMetrics,
) *PluginBridgeProcess {
	if logger == nil {
		logger = slog.Default()
	}

	cfg := core.BridgeTransportConfig{}
	if transportConfig != nil {
		cfg = *transportConfig
	}

	return &PluginBridgeProcess{
		bridgeScriptPath: bridgeScriptPath,
		logger:           logger,
		transportConfig:  cfg,
		launchSpec:       launchSpec,
		runtimeRoot:      runtimeRoot,
		metrics:          metrics,
	}
}

func (p *PluginBridgeProcess) RestartCount() int32 {
	return p.restartCount.Load()
}

func (p *PluginBridgeProcess) SetNotificationHandler(handler BridgeNotificationHandler) {
	p.mu.Lock()
	p.notificationHandler = handler
	transport := p.transport
	p.mu.Unlock()

	if transport != nil {
		transport.SetNotificationHandler(handler)
	}
}

// GetMemorySnapshot fetches cross-platform process metrics using gopsutil.
func (p *PluginBridgeProcess) GetMemorySnapshot() *PluginBridgeMemorySnapshot {
	p.mu.Lock()
	cmd := p.process
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	proc, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		return nil
	}

	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return nil
	}

	return &PluginBridgeMemorySnapshot{
		ProcessId:          cmd.Process.Pid,
		WorkingSetBytes:    int64(memInfo.RSS),
		PrivateMemoryBytes: int64(memInfo.VMS),
	}
}

func (p *PluginBridgeProcess) Start(
	ctx context.Context,
	entryPath string,
	pluginID string,
	pluginConfig *json.RawMessage,
) (*core.BridgeInitResult, error) {
	p.entryPath = entryPath
	p.pluginID = pluginID
	p.pluginConfig = pluginConfig
	p.intentionalShutdown.Store(false)

	resp, err := p.initializeProcess(ctx)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("plugin init failed: %s", resp.Error.Message)
	}

	if resp.Result == nil {
		return &core.BridgeInitResult{}, nil
	}

	var init core.BridgeInitResult
	if err := json.Unmarshal(*resp.Result, &init); err != nil {
		return nil, fmt.Errorf("failed to deserialize init response: %w", err)
	}

	return &init, nil
}

func (p *PluginBridgeProcess) ExecuteTool(ctx context.Context, toolName string, argumentsJSON string) string {
	if err := p.ensureProcessRunning(ctx); err != nil {
		return err.Error()
	}

	p.mu.Lock()
	cmd := p.process
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return "Error: Plugin bridge process is not running."
	}

	var rawParams json.RawMessage
	if err := json.Unmarshal([]byte(argumentsJSON), &rawParams); err != nil {
		return fmt.Sprintf("invalid arguments json: %s", err.Error())
	}

	execRequest := core.BridgeExecuteRequest{
		Name:   toolName,
		Params: rawParams,
	}

	resp, err := p.SendAndWait(ctx, "execute", execRequest)
	if err != nil {
		return err.Error()
	}

	if resp.Error != nil {
		return fmt.Sprintf("Error: %s", resp.Error.Message)
	}

	if resp.Result != nil {
		var resultObj map[string]json.RawMessage
		if err := json.Unmarshal(*resp.Result, &resultObj); err == nil {
			if details, ok := resultObj["details"]; ok && string(details) != "null" {
				return string(details)
			}

			if contentArray, ok := resultObj["content"]; ok {
				var items []struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(contentArray, &items); err == nil {
					var buf bytes.Buffer
					for i, item := range items {
						if i > 0 {
							buf.WriteByte('\n')
						}
						buf.WriteString(item.Text)
					}
					return buf.String()
				}
			}
		}
		return string(*resp.Result)
	}

	return ""
}

func (p *PluginBridgeProcess) SendRequest(ctx context.Context, method string, parameters any) error {
	_, err := p.SendAndWait(ctx, method, parameters)
	return err
}

func (p *PluginBridgeProcess) SendAndWait(ctx context.Context, method string, parameters any) (*core.BridgeResponse, error) {
	if err := p.ensureProcessRunning(ctx); err != nil {
		return nil, err
	}

	p.mu.Lock()
	transport := p.transport
	p.mu.Unlock()

	if transport == nil {
		return nil, errors.New("plugin bridge transport is not running")
	}

	return transport.SendAndWait(ctx, method, parameters)
}

// Close implements asynchronous teardown / graceful disposal.
func (p *PluginBridgeProcess) Close() error {
	p.disposed.Store(true)
	p.intentionalShutdown.Store(true)

	p.mu.Lock()
	cmd := p.process
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return p.disposeTransport()
	}

	// Graceful shutdown ping
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, _ = p.SendAndWait(shutdownCtx, "shutdown", nil)

	// Wait for exit or force kill
	done := make(chan struct{})
	go func() {
		if cmd.ProcessState != nil {
			close(done)
			return
		}
		_, _ = cmd.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
	}

	_ = p.disposeTransport()
	p.cleanupProcess()

	return nil
}

func (p *PluginBridgeProcess) ensureProcessRunning(ctx context.Context) error {
	p.mu.Lock()
	isRunning := p.initialized.Load() && p.process != nil && p.transport != nil
	p.mu.Unlock()

	if isRunning {
		return nil
	}

	return p.restart(ctx)
}

func (p *PluginBridgeProcess) restart(ctx context.Context) error {
	if p.disposed.Load() || p.entryPath == "" || p.pluginID == "" {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.disposed.Load() {
		return nil
	}

	if p.initialized.Load() && p.process != nil && p.transport != nil {
		return nil
	}

	delay := 1 * time.Second
	var lastErr error

	for attempt := 1; attempt <= 3; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if p.metrics != nil {
			p.metrics.IncrementPluginBridgeRestartAttempts()
		}

		_ = p.disposeTransport()
		p.cleanupProcess()
		p.intentionalShutdown.Store(false)

		_, err := p.initializeProcess(ctx)
		if err == nil {
			p.restartCount.Add(1)
			p.logger.Info("Plugin bridge restarted successfully", "pluginId", p.pluginID, "attempt", attempt)
			return nil
		}

		lastErr = err
		if p.metrics != nil {
			p.metrics.IncrementPluginBridgeRestartFailures()
		}

		p.logger.Warn("Failed to restart plugin bridge", "pluginId", p.pluginID, "attempt", attempt, "err", err)

		_ = p.disposeTransport()
		p.cleanupProcess()

		if attempt < 3 {
			select {
			case <-time.After(delay):
				delay *= 2
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	p.logger.Error("Plugin bridge could not be restarted", "pluginId", p.pluginID, "err", lastErr)
	return fmt.Errorf("failed to restart bridge process after 3 attempts: %w", lastErr)
}

func (p *PluginBridgeProcess) initializeProcess(ctx context.Context) (*core.BridgeResponse, error) {
	p.initialized.Store(false)

	transport, runtimeTransport, err := CreateBridgeTransport(p.transportConfig, p.pluginID, p.logger, p.runtimeRoot, p.metrics)
	if err != nil {
		return nil, err
	}

	if p.notificationHandler != nil {
		transport.SetNotificationHandler(p.notificationHandler)
	}

	if err := transport.Prepare(ctx); err != nil {
		return nil, err
	}

	cmd, err := p.startProcess(runtimeTransport)
	if err != nil {
		return nil, err
	}

	if err := transport.Start(ctx, cmd); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}

	p.process = cmd
	p.transport = transport
	p.runtimeTransport = runtimeTransport

	exitChan := make(chan struct{})
	p.exitChan = exitChan
	go p.monitorProcess(cmd, exitChan)

	initReq := core.BridgeInitRequest{
		EntryPath: p.entryPath,
		PluginId:  p.pluginID,
		Config:    p.pluginConfig,
		Transport: runtimeTransport,
	}

	resp, err := transport.SendAndWait(ctx, "init", initReq)
	if err != nil {
		_ = transport.Close()
		p.transport = nil
		p.cleanupProcess()
		return nil, err
	}

	if hybrid, ok := transport.(*HybridBridgeTransport); ok {
		hybrid.UseSocketTransport()
	}

	p.initialized.Store(true)

	return resp, nil
}

func (p *PluginBridgeProcess) startProcess(transport *core.BridgeTransportRuntimeConfig) (*exec.Cmd, error) {
	if p.launchSpec != nil {
		return p.startExternalProcess(transport)
	}

	nodeExe := FindNodeExecutable()
	if len(nodeExe) == 0 {
		return nil, errors.New("Node.js is required for OpenClaw plugin support but was not found. " +
			"Install Node.js 18+ and ensure 'node' is on your PATH")
	}

	args := []string{"--experimental-vm-modules", p.bridgeScriptPath}
	cmd := exec.Command(nodeExe, args...)

	workDir := filepath.Dir(p.entryPath)
	if workDir == "" {
		workDir = "."
	}
	cmd.Dir = workDir

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("OPENCLAW_BRIDGE_TRANSPORT_MODE=%s", transport.Mode))
	if transport.SocketPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("OPENCLAW_BRIDGE_SOCKET_PATH=%s", transport.SocketPath))
	}
	if transport.SocketAuthToken != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("OPENCLAW_BRIDGE_SOCKET_AUTH_TOKEN=%s", transport.SocketAuthToken))
	}

	stderr, err := cmd.StderrPipe()
	if err == nil {
		go p.streamStderr(stderr, "[Node]")
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Node.js plugin bridge process: %w", err)
	}

	return cmd, nil
}

func (p *PluginBridgeProcess) startExternalProcess(transport *core.BridgeTransportRuntimeConfig) (*exec.Cmd, error) {
	spec := p.launchSpec
	cmd := exec.Command(spec.FileName, spec.Arguments...)

	if spec.WorkingDirectory != "" {
		cmd.Dir = spec.WorkingDirectory
	} else {
		cmd.Dir, _ = os.Getwd()
	}

	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		// Basic split key-value
		envMap[env] = env
	}

	envMap["OPENCLAW_BRIDGE_TRANSPORT_MODE"] = transport.Mode
	if transport.SocketPath != "" {
		envMap["OPENCLAW_BRIDGE_SOCKET_PATH"] = transport.SocketPath
	}
	if transport.SocketAuthToken != "" {
		envMap["OPENCLAW_BRIDGE_SOCKET_AUTH_TOKEN"] = transport.SocketAuthToken
	}

	for k, v := range spec.EnvironmentVariables {
		if v == "" {
			delete(envMap, k)
		} else {
			envMap[k] = fmt.Sprintf("%s=%s", k, v)
		}
	}

	cmd.Env = make([]string, 0, len(envMap))
	for _, v := range envMap {
		cmd.Env = append(cmd.Env, v)
	}

	stderr, err := cmd.StderrPipe()
	if err == nil {
		go p.streamStderr(stderr, "[Bridge]")
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start bridge child process '%s': %w", spec.FileName, err)
	}

	return cmd, nil
}

func (p *PluginBridgeProcess) monitorProcess(cmd *exec.Cmd, exitChan chan struct{}) {
	_ = cmd.Wait()
	close(exitChan)

	p.mu.Lock()
	if p.process != cmd {
		// A prior process exited after a replacement was spun up. Ignore.
		p.mu.Unlock()
		return
	}

	p.initialized.Store(false)
	_ = p.disposeTransport()
	p.cleanupProcess()

	shouldRestart := !p.disposed.Load() && !p.intentionalShutdown.Load()
	p.mu.Unlock()

	if shouldRestart {
		p.logger.Warn("Plugin bridge process exited unexpectedly. Restarting...", "pluginId", p.pluginID)
		go func() {
			_ = p.restart(context.Background())
		}()
	}
}

func (p *PluginBridgeProcess) streamStderr(r io.Reader, prefix string) {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			p.logger.Info(fmt.Sprintf("%s %s", prefix, string(buf[:n])))
		}
		if err != nil {
			break
		}
	}
}

func (p *PluginBridgeProcess) disposeTransport() error {
	if p.transport == nil {
		return nil
	}
	err := p.transport.Close()
	p.transport = nil
	return err
}

func (p *PluginBridgeProcess) cleanupProcess() {
	p.initialized.Store(false)
	if p.process == nil || p.process.Process == nil {
		return
	}

	_ = p.process.Process.Kill()
	p.process = nil
}

type ChannelRegistration struct {
	PluginId  string
	ChannelId string
	Adapter   core.IChannelAdapter
}

type CommandRegistration struct {
	PluginId    string
	CommandName string
	Description string
	Bridge      *PluginBridgeProcess
}

type ProviderRegistration struct {
	ProviderId string
	Models     []string
	Bridge     *PluginBridgeProcess
}

type PluginProviderRegistration struct {
	PluginId   string
	ProviderId string
	Models     []string
	Bridge     *PluginBridgeProcess
}

type PluginHost struct {
	config           core.PluginsConfig
	bridgeScriptPath string
	logger           *slog.Logger
	blockedPluginIds map[string]struct{}
	runtimeRoot      *string
	metrics          *core.RuntimeMetrics

	pluginTools    []core.ITool
	reports        []core.PluginLoadReport
	skillRoots     []string
	pluginChannels []core.IChannelAdapter
	bridges        []*PluginBridgeProcess

	bridgesByPluginID           map[string]*PluginBridgeProcess
	pluginChannelRegistrations  []ChannelRegistration
	pluginHooks                 []core.IToolHook
	pluginCommands              []CommandRegistration
	pluginProviders             []ProviderRegistration
	pluginProviderRegistrations []PluginProviderRegistration
}

func NewPluginHost(
	config core.PluginsConfig,
	bridgeScriptPath string,
	logger *slog.Logger,
	blockedPluginIds []string,
	runtimeRoot *string,
	metrics *core.RuntimeMetrics,
) *PluginHost {
	blockedMap := make(map[string]struct{})
	for _, id := range blockedPluginIds {
		if id != "" {
			blockedMap[id] = struct{}{}
		}
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &PluginHost{
		config:           config,
		bridgeScriptPath: bridgeScriptPath,
		logger:           logger,
		blockedPluginIds: blockedMap,
		runtimeRoot:      runtimeRoot,
		metrics:          metrics,
	}
}

func (ph *PluginHost) Tools() []core.ITool                     { return ph.pluginTools }
func (ph *PluginHost) Reports() []core.PluginLoadReport        { return ph.reports }
func (ph *PluginHost) SkillRoots() []string                    { return ph.skillRoots }
func (ph *PluginHost) ChannelAdapters() []core.IChannelAdapter { return ph.pluginChannels }

func (ph *PluginHost) Load(ctx context.Context, workspacePath *string) ([]core.ITool, error) {
	if !ph.config.Enabled {
		ph.logger.Info("Plugin system is disabled")
		return nil, nil
	}

	ph.resetState()

	path := ""
	if workspacePath != nil {
		path = *workspacePath
	}
	discovery := core.PluginDiscoveryInstance.DiscoverWithDiagnostics(&ph.config, path)
	ph.reports = append(ph.reports, discovery.Reports...)
	ph.logger.Info("Discovered plugins", "count", len(discovery.Plugins))

	enabled := core.PluginDiscoveryInstance.Filter(discovery.Plugins, &ph.config)
	ph.logger.Info("Plugins enabled after filtering", "count", len(enabled))

	// 逐个加载
	for _, plugin := range enabled {
		if _, isBlocked := ph.blockedPluginIds[plugin.Manifest.ID]; isBlocked {
			msg := fmt.Sprintf("Plugin '%s' is disabled or quarantined by operator state.", plugin.Manifest.ID)
			ph.reports = append(ph.reports, core.PluginLoadReport{
				PluginId:      plugin.Manifest.ID,
				SourcePath:    plugin.RootPath,
				EntryPath:     plugin.EntryPath,
				Origin:        ph.getOriginFormat(plugin),
				BundleFormat:  plugin.BundleFormat,
				Loaded:        false,
				BlockedReason: msg,
				Error:         msg,
				Diagnostics: []core.PluginCompatibilityDiagnostic{
					{
						Severity: "warning",
						Code:     "operator_blocked",
						Message:  msg,
						Surface:  "operator_state",
						Path:     plugin.Manifest.ID,
					},
				},
			})
			continue
		}

		if err := ph.loadPlugin(ctx, plugin); err != nil {
			ph.reports = append(ph.reports, core.PluginLoadReport{
				PluginId:     plugin.Manifest.ID,
				SourcePath:   plugin.RootPath,
				EntryPath:    plugin.EntryPath,
				Origin:       ph.getOriginFormat(plugin),
				BundleFormat: plugin.BundleFormat,
				Loaded:       false,
				Error:        err.Error(),
			})
			ph.logger.Error("Failed to load plugin", "pluginId", plugin.Manifest.ID, "error", err.Error())
		}
	}

	ph.logger.Info("Loaded tools and plugins", "toolCount", len(ph.pluginTools), "bridgeCount", len(ph.bridges))
	return ph.pluginTools, nil
}

func (ph *PluginHost) loadPlugin(ctx context.Context, plugin core.DiscoveredPlugin) error {
	id := plugin.Manifest.ID

	if plugin.Format == "bundle" {
		ph.loadBundle(plugin)
		return nil
	}

	ph.logger.Info("Loading plugin bridge", "pluginId", id, "entryPath", plugin.EntryPath)

	configDiagnostics := append(
		core.ValidateDiscoveredPlugin(plugin),
		core.PluginConfigValidatorinstance.Validate(plugin.Manifest, *ph.getPluginConfig(id))...,
	)

	if len(configDiagnostics) > 0 {
		ph.reports = append(ph.reports, core.PluginLoadReport{
			PluginId:    id,
			SourcePath:  plugin.RootPath,
			EntryPath:   plugin.EntryPath,
			Origin:      "bridge",
			Loaded:      false,
			Diagnostics: configDiagnostics,
			Error:       "Plugin package or config compatibility validation failed.",
		})
		return errors.New("compatibility validation failed")
	}

	runtimeRoot := ""
	if ph.runtimeRoot != nil {
		runtimeRoot = *ph.runtimeRoot
	}
	bridge := NewPluginBridgeProcess(ph.bridgeScriptPath, ph.logger, &ph.config.Transport, nil, runtimeRoot, ph.metrics)
	pluginCfg := ph.getPluginConfig(id)

	initResult, err := bridge.Start(ctx, plugin.EntryPath, id, pluginCfg)
	if err != nil {
		_ = bridge.Close()
		return err
	}

	var skillDiagnostics []core.PluginCompatibilityDiagnostic
	skillDirs := ph.resolveSkillDirectories(plugin, &skillDiagnostics)
	requestedCapabilities := ph.determineRequestedCapabilities(initResult, skillDirs)

	if !initResult.Compatible {
		ph.reports = append(ph.reports, core.PluginLoadReport{
			PluginId:              id,
			SourcePath:            plugin.RootPath,
			EntryPath:             plugin.EntryPath,
			Origin:                "bridge",
			RequestedCapabilities: requestedCapabilities,
			Loaded:                false,
			Diagnostics:           append(initResult.Diagnostics, skillDiagnostics...),
			Error:                 "Plugin uses unsupported OpenClaw extension APIs.",
		})
		_ = bridge.Close()
		return errors.New("plugin incompatible")
	}

	// 检查能力受限规则
	blockedCapabilities := core.PluginCapabilityPolicyInstance.GetBlockedCapabilities(
		requestedCapabilities,
		core.ExecutionHostKind_Bridge,
	)

	if len(blockedCapabilities) > 0 {
		msg := fmt.Sprintf("Plugin '%s' requires JIT runtime mode for capabilities: %v.", id, blockedCapabilities)
		diags := append(initResult.Diagnostics, skillDiagnostics...)
		diags = append(diags, core.PluginCompatibilityDiagnostic{
			Severity: "error",
			Message:  msg,
			Surface:  "runtime_mode",
			Path:     id,
		})

		ph.reports = append(ph.reports, core.PluginLoadReport{
			PluginId:              id,
			SourcePath:            plugin.RootPath,
			EntryPath:             plugin.EntryPath,
			Origin:                "bridge",
			RequestedCapabilities: requestedCapabilities,
			Loaded:                false,
			BlockedByRuntimeMode:  true,
			BlockedReason:         msg,
			Diagnostics:           diags,
			Error:                 msg,
		})
		_ = bridge.Close()
		return errors.New(msg)
	}

	// 保存跨进程 Bridge 引用
	ph.bridges = append(ph.bridges, bridge)
	ph.bridgesByPluginID[id] = bridge

	for _, sDir := range skillDirs {
		if !slices.Contains(ph.skillRoots, sDir) {
			ph.skillRoots = append(ph.skillRoots, sDir)
		}
	}

	reportDiagnostics := append([]core.PluginCompatibilityDiagnostic{}, skillDiagnostics...)
	registeredCount := 0

	// 注册 Tools
	for _, reg := range initResult.Tools {
		if ph.hasToolName(reg.Name) {
			ph.logger.Info("Plugin tool skipped — name already registered", "pluginId", id, "toolName", reg.Name)
			reportDiagnostics = append(reportDiagnostics, core.PluginCompatibilityDiagnostic{
				Severity: "warning",
				Code:     "duplicate_tool_name",
				Message:  fmt.Sprintf("Tool '%s' from plugin '%s' was skipped because that tool name is already registered.", reg.Name, id),
				Surface:  "registerTool",
				Path:     reg.Name,
			})
			continue
		}

		tool := NewBridgedPluginTool(bridge, id, reg)
		ph.pluginTools = append(ph.pluginTools, tool)
		registeredCount++
	}

	// 注册 Channels
	var channelAdapters []*BridgedChannelAdapter
	for _, ch := range initResult.Channels {
		adapter := NewBridgedChannelAdapter(bridge, ch.Id, ph.logger)
		channelAdapters = append(channelAdapters, adapter)
		ph.pluginChannels = append(ph.pluginChannels, adapter)
		ph.pluginChannelRegistrations = append(ph.pluginChannelRegistrations, ChannelRegistration{
			PluginId: id, ChannelId: ch.Id, Adapter: adapter,
		})
	}

	// 设置 Channel 异步通知回调
	if len(channelAdapters) > 0 {
		bridge.SetNotificationHandler(func(notification core.BridgeNotification) error {
			if notification.Params == nil {
				return nil
			}

			channelID, ok := util.TryGetPropertyString(*notification.Params, "channelId")
			if !ok || channelID == "" {
				return nil
			}

			var target *BridgedChannelAdapter
			for _, a := range channelAdapters {
				if a.channelId == channelID {
					target = a
					break
				}
			}
			if target == nil {
				return nil
			}

			switch notification.Notification {
			case "channel_message":
				go func() {
					if err := target.HandleInbound(context.Background(), *notification.Params); err != nil {
						ph.logger.Error("Failed to handle inbound channel message", "channelId", channelID, "error", err.Error())
					}
				}()
			case "channel_auth_event":
				target.HandleAuthEvent(*notification.Params)
			case "channel_typing", "channel_receipt", "channel_reaction":
				ph.logger.Info("Received channel notification", "type", notification.Notification, "channelId", channelID)
			}
			return nil
		})
	}

	// 注册 Commands
	for _, cmd := range initResult.Commands {
		ph.pluginCommands = append(ph.pluginCommands, CommandRegistration{
			PluginId: id, CommandName: cmd.Name, Description: cmd.Description, Bridge: bridge,
		})
	}

	// 注册 Hooks
	if len(initResult.EventSubscriptions) > 0 {
		hook := NewBridgedToolHook(bridge, id, initResult.EventSubscriptions, ph.logger)
		ph.pluginHooks = append(ph.pluginHooks, hook)
	}

	// 注册 Providers
	for _, prov := range initResult.Providers {
		ph.pluginProviders = append(ph.pluginProviders, ProviderRegistration{
			ProviderId: prov.Id, Models: prov.Models, Bridge: bridge,
		})
		ph.pluginProviderRegistrations = append(ph.pluginProviderRegistrations, PluginProviderRegistration{
			PluginId: id, ProviderId: prov.Id, Models: prov.Models, Bridge: bridge,
		})
	}

	ph.reports = append(ph.reports, core.PluginLoadReport{
		PluginId:               id,
		SourcePath:             plugin.RootPath,
		EntryPath:              plugin.EntryPath,
		Origin:                 "bridge",
		RequestedCapabilities:  requestedCapabilities,
		Loaded:                 true,
		ToolCount:              registeredCount,
		ChannelCount:           len(initResult.Channels),
		CommandCount:           len(initResult.Commands),
		CliCommandCount:        len(initResult.CliCommands),
		CliCommandNames:        extractCommandNames(initResult.CliCommands),
		EventSubscriptionCount: len(initResult.EventSubscriptions),
		ProviderCount:          len(initResult.Providers),
		SkillDirectories:       skillDirs,
		Diagnostics:            append(initResult.Diagnostics, reportDiagnostics...),
	})

	return nil
}

func (ph *PluginHost) loadBundle(plugin core.DiscoveredPlugin) {
	id := plugin.Manifest.ID
	var diagnostics []core.PluginCompatibilityDiagnostic

	skillDirs := ph.resolveSkillDirectories(plugin, &diagnostics)
	for _, capName := range plugin.BundleDetectedCapabilities {
		diagnostics = append(diagnostics, core.PluginCompatibilityDiagnostic{
			Severity: "warning",
			Code:     "bundle_capability_detected_only",
			Message:  fmt.Sprintf("Bundle capability '%s' was detected but has no OpenClaw.NET runtime mapping.", capName),
			Surface:  capName,
			Path:     plugin.RootPath,
		})
	}

	if len(plugin.BundleMappedCapabilities) == 0 {
		diagnostics = append(diagnostics, core.PluginCompatibilityDiagnostic{
			Severity: "warning",
			Code:     "bundle_has_no_mapped_capabilities",
			Message:  fmt.Sprintf("%s bundle '%s' was detected, but it contains no mapped skill or command content.", plugin.BundleFormat, id),
			Surface:  "bundle",
			Path:     plugin.RootPath,
		})
	}

	hasErrors := false
	for _, diag := range diagnostics {
		if diag.Severity == "error" {
			hasErrors = true
			break
		}
	}

	if !hasErrors {
		for _, sDir := range skillDirs {
			if !slices.Contains(ph.skillRoots, sDir) {
				ph.skillRoots = append(ph.skillRoots, sDir)
			}
		}
	}

	var errStr *string
	if hasErrors {
		msg := "Bundle content validation failed."
		errStr = &msg
	}

	ph.reports = append(ph.reports, core.PluginLoadReport{
		PluginId:              id,
		SourcePath:            plugin.RootPath,
		Origin:                "bridge",
		BundleFormat:          plugin.BundleFormat,
		RequestedCapabilities: core.PluginCapabilityPolicyInstance.Normalize(plugin.BundleMappedCapabilities),
		Loaded:                !hasErrors,
		SkillDirectories:      skillDirs,
		Diagnostics:           diagnostics,
		Error:                 derefString(errStr),
	})
}

// RegisterCommandsWith 注册动态聊天命令
func (ph *PluginHost) RegisterCommandsWith(processor *core.ChatCommandProcessor) {
	for _, cmd := range ph.pluginCommands {
		c := cmd
		processor.RegisterDynamic(c.CommandName, func(ctx context.Context, args string) (string, error) {
			resp, err := c.Bridge.SendAndWait(ctx, "command_execute", core.BridgeCommandExecuteRequest{
				Name: c.CommandName,
				Args: args,
			})
			if err != nil {
				return fmt.Sprintf("Command error: %v", err), nil
			}

			if resp.Error != nil {
				return fmt.Sprintf("Command error: %s", resp.Error.Message), nil
			}

			if resp.Result != nil {
				if rVal, ok := util.TryGetPropertyString(*resp.Result, "result"); ok {
					return rVal, nil
				}
			}
			return "", nil
		})
	}
}

func (ph *PluginHost) Close() error {
	for _, b := range ph.bridges {
		_ = b.Close()
	}
	ph.resetState()
	return nil
}

// 辅助私有方法
func (ph *PluginHost) resetState() {
	ph.reports = nil
	ph.skillRoots = nil
	ph.bridges = nil
	ph.bridgesByPluginID = make(map[string]*PluginBridgeProcess)
	ph.pluginTools = nil
	ph.pluginChannels = nil
	ph.pluginChannelRegistrations = nil
	ph.pluginHooks = nil
	ph.pluginCommands = nil
	ph.pluginProviders = nil
	ph.pluginProviderRegistrations = nil
}

func (ph *PluginHost) hasToolName(name string) bool {
	for _, t := range ph.pluginTools {
		if t.Name() == name {
			return true
		}
	}
	return false
}

func (ph *PluginHost) getOriginFormat(p core.DiscoveredPlugin) string {
	if p.Format == "bundle" {
		return "bundle"
	}
	return "bridge"
}

func (ph *PluginHost) getPluginConfig(pluginID string) *json.RawMessage {
	if entry, ok := ph.config.Entries[pluginID]; ok {
		return entry.Config
	}
	return nil
}

func (ph *PluginHost) determineRequestedCapabilities(initResult *core.BridgeInitResult, skillDirs []string) []string {
	caps := append([]string{}, initResult.Capabilities...)
	if len(skillDirs) > 0 {
		caps = append(caps, "skills")
	}

	if len(caps) == 0 {
		if len(initResult.Tools) > 0 {
			caps = append(caps, "tools")
		}
		if len(initResult.Channels) > 0 {
			caps = append(caps, "channels")
		}
		if len(initResult.Commands) > 0 {
			caps = append(caps, "commands")
		}
		if len(initResult.EventSubscriptions) > 0 {
			caps = append(caps, "hooks")
		}
		if len(initResult.Providers) > 0 {
			caps = append(caps, "providers")
		}
	}
	return core.PluginCapabilityPolicyInstance.Normalize(caps)
}

func (ph *PluginHost) resolveSkillDirectories(plugin core.DiscoveredPlugin, diagnostics *[]core.PluginCompatibilityDiagnostic) []string {
	var resolvedDirs []string
	for _, skillDir := range plugin.Manifest.Skills {
		if skillDir == "" {
			continue
		}

		resolved, ok := core.PluginDiscoveryInstance.TryResolveContainedPath(plugin.RootPath, skillDir)
		if !ok {
			if diagnostics != nil {
				*diagnostics = append(*diagnostics, core.PluginCompatibilityDiagnostic{
					Severity: "error",
					Code:     "skill_dir_outside_root",
					Message:  fmt.Sprintf("Plugin '%s' skill directory resolves outside the plugin root.", plugin.Manifest.ID),
					Surface:  "skills",
					Path:     skillDir,
				})
			}
			continue
		}

		if !util.DirectoryExists(resolved) {
			if diagnostics != nil {
				*diagnostics = append(*diagnostics, core.PluginCompatibilityDiagnostic{
					Severity: "error",
					Code:     "skill_directory_missing",
					Message:  fmt.Sprintf("Plugin '%s' declared a skill directory that does not exist.", plugin.Manifest.ID),
					Surface:  "skills",
					Path:     resolved,
				})
			}
			continue
		}

		resolvedDirs = append(resolvedDirs, resolved)
	}
	return resolvedDirs
}

func extractCommandNames(cmds []core.BridgeCliCommandRegistration) []string {
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}
	return names
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
