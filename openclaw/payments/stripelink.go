package payments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

type StripeLinkOptions struct {
	ProviderId           string
	CliPath              string
	Mode                 string
	Timeout              time.Duration
	WorkingDirectory     string
	EnvironmentVariables map[string]string
}

func NewStripeLinkOptions() *StripeLinkOptions {
	return &StripeLinkOptions{
		ProviderId:           "stripe-link",
		CliPath:              "link-cli",
		Mode:                 "test",
		Timeout:              time.Second * 30,
		EnvironmentVariables: map[string]string{},
	}
}

type LinkCliCommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

type ILinkCliCommandRunner interface {
	Run(
		ctx context.Context,
		executable string,
		arguments []string,
		workingDirectory string,
		environment map[string]string,
		timeout time.Duration) (*LinkCliCommandResult, error)
}

type LinkCliProcessRunner struct {
	redactor PaymentSensitiveDataRedactor
	logger   *slog.Logger
}

func NewLinkCliProcessRunner(logger *slog.Logger) *LinkCliProcessRunner {
	if logger == nil {
		logger = slog.Default()
	}

	return &LinkCliProcessRunner{
		logger:   logger,
		redactor: PaymentSensitiveDataRedactor{},
	}
}

func (l *LinkCliProcessRunner) Run(ctx context.Context, executable string, arguments []string, workingDirectory string, environment map[string]string, timeout time.Duration) (*LinkCliCommandResult, error) {
	execCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 2. Prepare Command
	cmd := exec.CommandContext(execCtx, executable, arguments...)

	// Kills child process tree when context times out or is cancelled
	cmd.WaitDelay = 2 * time.Second

	if workingDirectory != "" {
		cmd.Dir = workingDirectory
	}

	// Environment variables setup
	if len(environment) > 0 {
		cmd.Env = os.Environ()
		for k, v := range environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Buffers to capture Stdout and Stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// 3. Start and Wait for execution
	err := cmd.Run()

	rawStdout := stdoutBuf.String()
	rawStderr := stderrBuf.String()

	// 4. Handle Timeout / Context Cancellation
	if execCtx.Err() == context.DeadlineExceeded {
		return &LinkCliCommandResult{
			ExitCode: -1,
			Stdout:   l.redactor.Redact(rawStdout),
			Stderr:   l.redactor.Redact(rawStderr),
			TimedOut: true,
		}, nil
	}

	// 5. Handle Errors and Exit Codes
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// Failed to launch process or binary not found
			return &LinkCliCommandResult{
				ExitCode: -1,
				Stderr:   err.Error(),
			}, nil
		}
	}

	stdout := l.redactor.Redact(rawStdout)
	stderr := l.redactor.Redact(rawStderr)

	if exitCode != 0 {
		l.logger.Warn("link-cli exited with non-zero status",
			"exitCode", exitCode,
			"stderr", stderr,
		)
	}

	return &LinkCliCommandResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		TimedOut: false,
	}, nil
}
