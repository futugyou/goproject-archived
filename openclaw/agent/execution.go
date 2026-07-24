package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/google/uuid"
)

type ManagedExecutionProcess struct {
	// 元数据属性
	ProcessId           string
	BackendName         string
	OwnerSessionId      string
	OwnerChannelId      string
	OwnerSenderId       string
	CreatedAtUtc        time.Time
	ProcessStartedAtUtc *time.Time
	CompletedAtUtc      *time.Time
	CommandPreview      string
	SupportsPty         bool
	Pty                 bool
	NativeProcessId     *int
	TimeoutSeconds      *int

	// 回调函数
	OnExited func(processId string, outcome string)

	// 进程和通信
	cmd         *exec.Cmd
	stdinWriter io.WriteCloser

	// 互斥锁与日志缓冲区
	stdoutGate sync.Mutex
	stderrGate sync.Mutex
	stdout     bytes.Buffer
	stderr     bytes.Buffer

	// 退出与同步控制
	exitCode             *int
	exitNotificationSent atomic.Int32
	killed               bool
	timedOut             bool

	doneChan chan struct{} // 标识进程退出信号
	mu       sync.Mutex    // 保护内部可变状态
}

func NewManagedExecutionProcess(
	processID string,
	req *core.ExecutionProcessStartRequest,
	cmd *exec.Cmd,
	nativePid *int,
	supportsPty bool,
) *ManagedExecutionProcess {

	now := time.Now().UTC()
	p := &ManagedExecutionProcess{
		ProcessId:           processID,
		BackendName:         req.BackendName,
		OwnerSessionId:      req.OwnerSessionId,
		OwnerChannelId:      req.OwnerChannelId,
		OwnerSenderId:       req.OwnerSenderId,
		CreatedAtUtc:        now,
		ProcessStartedAtUtc: &now,
		CommandPreview:      buildCommandPreview(req.Command, req.Arguments),
		SupportsPty:         supportsPty,
		Pty:                 req.Pty,
		NativeProcessId:     nativePid,
		TimeoutSeconds:      req.TimeoutSeconds,
		cmd:                 cmd,
		doneChan:            make(chan struct{}),
	}

	return p
}

// BeginCapture 启动 stdout/stderr 的异步流式读取
func (p *ManagedExecutionProcess) BeginCapture() error {
	stdoutPipe, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderrPipe, err := p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	stdinPipe, err := p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	p.stdinWriter = stdinPipe

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	if p.cmd.Process != nil {
		pid := p.cmd.Process.Pid
		p.NativeProcessId = &pid
	}

	// 异步捕获 Stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				p.stdoutGate.Lock()
				p.stdout.Write(buf[:n])
				p.stdoutGate.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	// 异步捕获 Stderr
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				p.stderrGate.Lock()
				p.stderr.Write(buf[:n])
				p.stderrGate.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	// 监听进程退出
	go func() {
		err := p.cmd.Wait()

		p.mu.Lock()
		now := time.Now().UTC()
		p.CompletedAtUtc = &now

		code := 0
		if err != nil {
			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		p.exitCode = &code
		p.mu.Unlock()

		close(p.doneChan)
		p.notifyExit()
	}()

	return nil
}

// StartTimeoutMonitor 启动后台超时监控
func (p *ManagedExecutionProcess) StartTimeoutMonitor() {
	if p.TimeoutSeconds == nil || *p.TimeoutSeconds <= 0 {
		return
	}

	go func() {
		timeout := time.Duration(*p.TimeoutSeconds) * time.Second
		select {
		case <-p.doneChan:
			// 进程正常或提前退出
			return
		case <-time.After(timeout):
			p.mu.Lock()
			p.timedOut = true
			p.mu.Unlock()

			_ = p.Kill(context.Background())
		}
	}()
}

func (p *ManagedExecutionProcess) GetStatus() core.ExecutionProcessStatus {
	p.mu.Lock()
	hasExited := p.hasExitedLocked()
	timedOut := p.timedOut
	killed := p.killed
	exitCode := p.exitCode
	completedAtUtc := p.CompletedAtUtc
	p.mu.Unlock()

	var state string
	if !hasExited {
		state = "running"
	} else if timedOut {
		state = "timed_out"
	} else if killed {
		state = "killed"
	} else if exitCode != nil && *exitCode == 0 {
		state = "completed"
	} else {
		state = "failed"
	}

	p.stdoutGate.Lock()
	stdoutLen := p.stdout.Len()
	p.stdoutGate.Unlock()

	p.stderrGate.Lock()
	stderrLen := p.stderr.Len()
	p.stderrGate.Unlock()

	endTime := time.Now().UTC()
	if completedAtUtc != nil {
		endTime = *completedAtUtc
	}
	durationMs := float64(endTime.Sub(p.CreatedAtUtc).Milliseconds())

	return core.ExecutionProcessStatus{
		ProcessId:       p.ProcessId,
		BackendName:     p.BackendName,
		OwnerSessionId:  p.OwnerSessionId,
		State:           state,
		ExitCode:        exitCode,
		TimedOut:        timedOut,
		Pty:             p.Pty,
		NativeProcessId: p.NativeProcessId,
		CreatedAtUtc:    p.CreatedAtUtc,
		StartedAtUtc:    p.ProcessStartedAtUtc,
		CompletedAtUtc:  completedAtUtc,
		DurationMs:      durationMs,
		StdoutBytes:     int64(stdoutLen),
		StderrBytes:     int64(stderrLen),
		CommandPreview:  p.CommandPreview,
	}
}

func (p *ManagedExecutionProcess) ReadLog(req core.ExecutionProcessLogRequest) core.ExecutionProcessLogResult {
	stdoutStr, nextStdout := readSlice(&p.stdout, &p.stdoutGate, req.StdoutOffset, req.MaxChars)
	stderrStr, nextStderr := readSlice(&p.stderr, &p.stderrGate, req.StderrOffset, req.MaxChars)

	return core.ExecutionProcessLogResult{
		ProcessId:        p.ProcessId,
		Stdout:           stdoutStr,
		Stderr:           stderrStr,
		NextStdoutOffset: nextStdout,
		NextStderrOffset: nextStderr,
		Completed:        p.HasExited(),
	}
}

func (p *ManagedExecutionProcess) Wait(ctx context.Context) error {
	if p.HasExited() {
		return nil
	}

	select {
	case <-p.doneChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *ManagedExecutionProcess) Write(ctx context.Context, data string) error {
	if p.HasExited() {
		return errors.New("the process has already exited")
	}

	p.mu.Lock()
	writer := p.stdinWriter
	p.mu.Unlock()

	if writer == nil {
		return errors.New("stdin writer not initialized")
	}

	_, err := writer.Write([]byte(data))
	return err
}

func (p *ManagedExecutionProcess) Kill(ctx context.Context) error {
	if p.HasExited() {
		return nil
	}

	p.mu.Lock()
	p.killed = true
	p.mu.Unlock()

	if p.cmd.Process != nil {
		// cmd.Process.Kill() 仅杀当前进程，若要杀进程树通常配合 sys/unix 设置 Process Group 机制。
		_ = p.cmd.Process.Kill()
	}

	return p.Wait(ctx)
}

// Close 实现 io.Closer
func (p *ManagedExecutionProcess) Close() error {
	_ = p.Kill(context.Background())
	if p.stdinWriter != nil {
		_ = p.stdinWriter.Close()
	}
	return nil
}

func (p *ManagedExecutionProcess) HasExited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hasExitedLocked()
}

func (p *ManagedExecutionProcess) hasExitedLocked() bool {
	select {
	case <-p.doneChan:
		return true
	default:
		return false
	}
}

func (p *ManagedExecutionProcess) notifyExit() {
	if !p.exitNotificationSent.CompareAndSwap(0, 1) {
		return
	}

	p.mu.Lock()
	outcome := "failed"
	if p.timedOut {
		outcome = "timed_out"
	} else if p.killed {
		outcome = "killed"
	} else if p.exitCode != nil && *p.exitCode == 0 {
		outcome = "completed"
	}
	onExited := p.OnExited
	p.mu.Unlock()

	if onExited != nil {
		onExited(p.ProcessId, outcome)
	}
}

func buildCommandPreview(command string, args []string) string {
	var preview string
	if len(args) == 0 {
		preview = command
	} else {
		preview = command + " " + strings.Join(args, " ")
	}

	runes := []rune(preview)
	if len(runes) <= 240 {
		return preview
	}
	return string(runes[:240]) + "…"
}

func readSlice(buf *bytes.Buffer, gate *sync.Mutex, offset int, maxChars int) (string, int) {
	gate.Lock()
	defer gate.Unlock()

	data := buf.Bytes()
	totalLen := len(data)

	safeOffset := clamp(offset, 0, totalLen)
	length := min(max(0, maxChars), totalLen-safeOffset)
	nextOffset := safeOffset + length

	if length == 0 {
		return "", nextOffset
	}

	return string(data[safeOffset : safeOffset+length]), nextOffset
}

func clamp(val, low, high int) int {
	if val < low {
		return low
	}
	if val > high {
		return high
	}
	return val
}

type ExecutionRouteResolution struct {
	BackendName      string
	FallbackBackend  *string
	Template         *string
	RequireWorkspace bool
	SandboxMode      core.ToolSandboxMode
}

type IProcessCommandBuilder interface {
	CreateProcessStartInfo(ctx context.Context, req *core.ExecutionRequest) (*exec.Cmd, error)
}

type ProcessExecutionBackendBase struct {
	capabilities *core.ExecutionBackendCapabilities
	builder      IProcessCommandBuilder
}

func NewProcessExecutionBackendBase(builder IProcessCommandBuilder) ProcessExecutionBackendBase {
	return ProcessExecutionBackendBase{
		builder: builder,
		capabilities: &core.ExecutionBackendCapabilities{
			SupportsOneShotCommands:  true,
			SupportsProcesses:        true,
			SupportsPty:              false,
			SupportsInteractiveInput: true,
		},
	}
}

func (b *ProcessExecutionBackendBase) Capabilities() *core.ExecutionBackendCapabilities {
	return b.capabilities
}

func (b *ProcessExecutionBackendBase) StartProcess(ctx context.Context, request *core.ExecutionProcessStartRequest) (*ManagedExecutionProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	execReq := &core.ExecutionRequest{
		ToolName:         request.ToolName,
		BackendName:      request.BackendName,
		Command:          request.Command,
		Arguments:        request.Arguments,
		WorkingDirectory: request.WorkingDirectory,
		Environment:      request.Environment,
		Template:         request.Template,
		RequireWorkspace: request.RequireWorkspace,
	}

	cmd, err := b.builder.CreateProcessStartInfo(ctx, execReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	rawUUID := uuid.New().String()
	cleanUUID := strings.ReplaceAll(rawUUID, "-", "")
	managedID := fmt.Sprintf("proc_%s", cleanUUID[:16])

	managed := NewManagedExecutionProcess(managedID, request, cmd, &pid, b.capabilities.SupportsPty)

	managed.BeginCapture()
	return managed, nil
}

func ExecuteProcess(ctx context.Context, backendName string, cmd *exec.Cmd, standardInput string, timeoutSeconds int) (*core.ExecutionResult, error) {
	start := time.Now()

	// 超时 Context 管理
	var cancel context.CancelFunc
	if timeoutSeconds > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
	}

	// 利用 Pipe 或 Buffer 捕获 Stdout/Stderr/Stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if standardInput != "" {
		cmd.Stdin = bytes.NewBufferString(standardInput)
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("process start failed: %w", err)
	}

	// 关联取消通知：如果 context 被取消或超时，杀死进程树
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	timedOut := false

	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			timedOut = true
		}
		// 强制杀掉包含子树在内的进程
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		waitErr = ctx.Err()
	case err := <-done:
		waitErr = err
	}

	durationMs := float64(time.Since(start).Milliseconds())

	// 提取 ExitCode
	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &core.ExecutionResult{
		BackendName:  backendName,
		ExitCode:     exitCode,
		Stdout:       stdout.String(),
		Stderr:       stderr.String(),
		TimedOut:     timedOut,
		FallbackUsed: false,
		DurationMs:   durationMs,
	}, nil
}

type IExecutionProcessBackend interface {
	Name() string
	Capabilities() *core.ExecutionBackendCapabilities
	StartProcess(ctx context.Context, request *core.ExecutionProcessStartRequest) (*ManagedExecutionProcess, error)
}

var _ core.IExecutionBackend = (*DockerExecutionBackend)(nil)
var _ IExecutionProcessBackend = (*DockerExecutionBackend)(nil)

type DockerExecutionBackend struct {
	ProcessExecutionBackendBase
	name    string
	profile core.ExecutionBackendProfileConfig
}

func NewDockerExecutionBackend(name string, profile core.ExecutionBackendProfileConfig) *DockerExecutionBackend {
	backend := &DockerExecutionBackend{
		name:    name,
		profile: profile,
	}
	backend.ProcessExecutionBackendBase = NewProcessExecutionBackendBase(backend)
	return backend
}

// GetName implements [IExecutionProcessBackend].
func (d *DockerExecutionBackend) Name() string {
	return d.name
}

func (d *DockerExecutionBackend) CreateProcessStartInfo(ctx context.Context, request *core.ExecutionRequest) (*exec.Cmd, error) {
	image := request.Template
	if strings.TrimSpace(image) == "" {
		image = d.profile.Image
	}
	if strings.TrimSpace(image) == "" {
		return nil, fmt.Errorf("execution backend '%s' requires an image", d.Name())
	}

	var args []string
	args = append(args, "run", "--rm")

	workingDir := request.WorkingDirectory
	if strings.TrimSpace(workingDir) == "" {
		workingDir = d.profile.WorkingDirectory
	}
	if strings.TrimSpace(workingDir) != "" {
		args = append(args, "-w", workingDir)
	}

	for k, v := range d.profile.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	for k, v := range request.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, image, request.Command)
	args = append(args, request.Arguments...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd, nil
}

func (d *DockerExecutionBackend) Execute(ctx context.Context, request *core.ExecutionRequest) (*core.ExecutionResult, error) {
	cmd, err := d.CreateProcessStartInfo(ctx, request)
	if err != nil {
		return nil, err
	}

	return ExecuteProcess(
		ctx,
		d.Name(),
		cmd,
		request.StandardInput,
		d.profile.TimeoutSeconds,
	)
}

var _ core.IExecutionBackend = (*LocalExecutionBackend)(nil)
var _ IExecutionProcessBackend = (*LocalExecutionBackend)(nil)

type LocalExecutionBackend struct {
	ProcessExecutionBackendBase
	workspaceRoot string
	profile       core.ExecutionBackendProfileConfig
}

func NewLocalExecutionBackend(workspaceRoot string, profile core.ExecutionBackendProfileConfig) *LocalExecutionBackend {
	backend := &LocalExecutionBackend{
		workspaceRoot: workspaceRoot,
		profile:       profile,
	}
	backend.ProcessExecutionBackendBase = NewProcessExecutionBackendBase(backend)
	return backend
}

func (l *LocalExecutionBackend) Name() string {
	return "local"
}

func (b *LocalExecutionBackend) Capabilities() *core.ExecutionBackendCapabilities {
	return &core.ExecutionBackendCapabilities{
		SupportsOneShotCommands:  true,
		SupportsProcesses:        true,
		SupportsPty:              runtime.GOOS != "windows",
		SupportsInteractiveInput: true,
	}
}

func (l *LocalExecutionBackend) Execute(ctx context.Context, request *core.ExecutionRequest) (*core.ExecutionResult, error) {
	cmd, err := l.CreateProcessStartInfo(ctx, request)
	if err != nil {
		return nil, err
	}

	return ExecuteProcess(
		ctx,
		l.Name(),
		cmd,
		request.StandardInput,
		l.profile.TimeoutSeconds,
	)
}

func (l *LocalExecutionBackend) CreateProcessStartInfo(ctx context.Context, req *core.ExecutionRequest) (*exec.Cmd, error) {
	workingDir, err := l.resolveWorkingDirectory(req)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(req.Command, req.Arguments...)
	cmd.Dir = workingDir

	envMap := make(map[string]string)
	maps.Copy(envMap, l.profile.Environment)
	maps.Copy(envMap, req.Environment)

	env := os.Environ()
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	return cmd, nil
}

func (l *LocalExecutionBackend) resolveWorkingDirectory(req *core.ExecutionRequest) (string, error) {
	effective := req.WorkingDirectory
	if effective == "" {
		effective = l.profile.WorkingDirectory
	}
	if effective == "" {
		effective = l.workspaceRoot
	}
	if effective == "" {
		var err error
		effective, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	if !req.RequireWorkspace {
		return effective, nil
	}

	workspaceRoot := l.workspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = l.profile.WorkingDirectory
	}
	if workspaceRoot == "" {
		workspaceRoot = l.profile.WorkspaceRoot
	}

	if strings.TrimSpace(workspaceRoot) == "" {
		return "", fmt.Errorf("execution backend 'local' requires a configured workspace root for this request")
	}

	resolvedWorkspaceRoot, err := l.resolveFullPath(workspaceRoot)
	if err != nil {
		return "", err
	}

	resolvedWorkingDirectory, err := l.resolveFullPath(effective)
	if err != nil {
		return "", err
	}

	isWindows := runtime.GOOS == "windows"
	cmp := func(a, b string) bool {
		if isWindows {
			return strings.EqualFold(a, b)
		}
		return a == b
	}

	if !cmp(resolvedWorkingDirectory, resolvedWorkspaceRoot) {
		workspacePrefix := resolvedWorkspaceRoot
		if !strings.HasSuffix(workspacePrefix, string(filepath.Separator)) {
			workspacePrefix += string(filepath.Separator)
		}

		hasPrefix := false
		if isWindows {
			hasPrefix = strings.HasPrefix(strings.ToLower(resolvedWorkingDirectory), strings.ToLower(workspacePrefix))
		} else {
			hasPrefix = strings.HasPrefix(resolvedWorkingDirectory, workspacePrefix)
		}

		if !hasPrefix {
			return "", fmt.Errorf("execution backend '%s' denied working directory '%s' because it is outside the configured workspace root", l.Name(), effective)
		}
	}

	return resolvedWorkingDirectory, nil
}

func (l *LocalExecutionBackend) resolveFullPath(val string) (string, error) {
	if strings.TrimSpace(val) == "" {
		return os.Getwd()
	}

	if strings.HasPrefix(val, "~/") || val == "~" {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		home := usr.HomeDir

		if val == "~" {
			val = home
		} else {
			val = filepath.Join(home, val[2:])
		}
	}

	return filepath.Abs(val)
}

var _ core.IExecutionBackend = (*OpenSandboxExecutionBackend)(nil)

type OpenSandboxExecutionBackend struct {
	name           string
	toolSandbox    core.IToolSandbox
	timeoutSeconds int
}

func NewOpenSandboxExecutionBackend(name string, toolSandbox core.IToolSandbox, timeoutSeconds int) *OpenSandboxExecutionBackend {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	backend := &OpenSandboxExecutionBackend{
		name:           name,
		toolSandbox:    toolSandbox,
		timeoutSeconds: timeoutSeconds,
	}
	return backend
}

func (o *OpenSandboxExecutionBackend) Name() string {
	return o.name
}

func (o *OpenSandboxExecutionBackend) Execute(ctx context.Context, request *core.ExecutionRequest) (*core.ExecutionResult, error) {
	start := time.Now()
	execCtx := ctx
	if o.timeoutSeconds > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(o.timeoutSeconds)*time.Second)
		defer cancel()
	}

	var ttlSeconds int
	if request.TimeToLiveSeconds != nil {
		ttlSeconds = *request.TimeToLiveSeconds
	}

	result, err := o.toolSandbox.Execute(execCtx, core.SandboxExecutionRequest{
		Command:           request.Command,
		Arguments:         request.Arguments,
		LeaseKey:          request.LeaseKey,
		Environment:       request.Environment,
		WorkingDirectory:  request.WorkingDirectory,
		Template:          request.Template,
		TimeToLiveSeconds: ttlSeconds,
	})

	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return &core.ExecutionResult{
				BackendName:  o.name,
				ExitCode:     -1,
				TimedOut:     true,
				FallbackUsed: false,
				DurationMs:   float64(time.Since(start).Milliseconds()),
			}, nil
		}

		return nil, err
	}

	return &core.ExecutionResult{
		BackendName:  o.name,
		ExitCode:     result.ExitCode,
		Stdout:       result.Stdout,
		Stderr:       result.Stderr,
		TimedOut:     false,
		FallbackUsed: false,
		DurationMs:   float64(time.Since(start).Milliseconds()),
	}, nil
}

var _ core.IExecutionBackend = (*SshExecutionBackend)(nil)
var _ IExecutionProcessBackend = (*SshExecutionBackend)(nil)

type SshExecutionBackend struct {
	ProcessExecutionBackendBase
	name    string
	profile core.ExecutionBackendProfileConfig
}

func NewSshExecutionBackend(name string, profile core.ExecutionBackendProfileConfig) *SshExecutionBackend {
	backend := &SshExecutionBackend{
		name:    name,
		profile: profile,
	}
	backend.ProcessExecutionBackendBase = NewProcessExecutionBackendBase(backend)
	return backend
}

func (s *SshExecutionBackend) Name() string {
	return s.name
}

func (s *SshExecutionBackend) Execute(ctx context.Context, request *core.ExecutionRequest) (*core.ExecutionResult, error) {
	cmd, err := s.CreateProcessStartInfo(ctx, request)
	if err != nil {
		return nil, err
	}

	return ExecuteProcess(
		ctx,
		s.Name(),
		cmd,
		request.StandardInput,
		s.profile.TimeoutSeconds,
	)
}

func (s *SshExecutionBackend) CreateProcessStartInfo(ctx context.Context, req *core.ExecutionRequest) (*exec.Cmd, error) {
	if strings.TrimSpace(s.profile.Host) == "" || strings.TrimSpace(s.profile.Username) == "" {
		return nil, fmt.Errorf("execution backend '%s' requires Host and Username", s.name)
	}

	var args []string

	port := s.profile.Port
	if port <= 0 {
		port = 22
	}
	args = append(args, "-p", fmt.Sprintf("%d", port))

	if strings.TrimSpace(s.profile.PrivateKeyPath) != "" {
		args = append(args, "-i", s.profile.PrivateKeyPath)
	}

	args = append(args, fmt.Sprintf("%s@%s", s.profile.Username, s.profile.Host))

	remoteCommand := req.Command
	if len(req.Arguments) > 0 {
		quotedArgs := make([]string, len(req.Arguments))
		for i, arg := range req.Arguments {
			quotedArgs[i] = quoteIfNeeded(arg)
		}
		remoteCommand += " " + strings.Join(quotedArgs, " ")
	}

	workDir := req.WorkingDirectory
	if workDir == "" {
		workDir = s.profile.WorkingDirectory
	}
	if strings.TrimSpace(workDir) != "" {
		remoteCommand = fmt.Sprintf("cd %s && %s", quoteIfNeeded(workDir), remoteCommand)
	}

	mergedEnv := make(map[string]string)
	maps.Copy(mergedEnv, s.profile.Environment)
	maps.Copy(mergedEnv, req.Environment)

	var envString strings.Builder
	for k, v := range mergedEnv {
		fmt.Fprintf(&envString, "%s=%s ", k, quoteIfNeeded(v))
	}
	if envString.String() != "" {
		remoteCommand = envString.String() + remoteCommand
	}

	args = append(args, remoteCommand)

	cmd := exec.Command("ssh", args...)

	return cmd, nil
}

func quoteIfNeeded(value string) string {
	if strings.ContainsAny(value, " \t\n\r") {
		escaped := strings.ReplaceAll(value, `"`, `\"`)
		return fmt.Sprintf(`"%s"`, escaped)
	}
	return value
}
