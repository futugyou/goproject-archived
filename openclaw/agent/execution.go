package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
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
	req core.ExecutionProcessStartRequest,
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
	CreateCmd(ctx context.Context, req core.ExecutionRequest) (*exec.Cmd, error)
}

type ProcessExecutionBackendBase struct {
	capabilities core.ExecutionBackendCapabilities
	builder      IProcessCommandBuilder
}

func NewProcessExecutionBackendBase(builder IProcessCommandBuilder) ProcessExecutionBackendBase {
	return ProcessExecutionBackendBase{
		builder: builder,
		capabilities: core.ExecutionBackendCapabilities{
			SupportsOneShotCommands:  true,
			SupportsProcesses:        true,
			SupportsPty:              false,
			SupportsInteractiveInput: true,
		},
	}
}

func (b *ProcessExecutionBackendBase) Capabilities() core.ExecutionBackendCapabilities {
	return b.capabilities
}

func (b *ProcessExecutionBackendBase) StartProcess(ctx context.Context, request core.ExecutionProcessStartRequest) (*ManagedExecutionProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	execReq := core.ExecutionRequest{
		ToolName:         request.ToolName,
		BackendName:      request.BackendName,
		Command:          request.Command,
		Arguments:        request.Arguments,
		WorkingDirectory: request.WorkingDirectory,
		Environment:      request.Environment,
		Template:         request.Template,
		RequireWorkspace: request.RequireWorkspace,
	}

	cmd, err := b.builder.CreateCmd(ctx, execReq)
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
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
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
