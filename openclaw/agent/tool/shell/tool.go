package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type ShellTool struct {
	config *core.ToolingConfig
}

func New(config *core.ToolingConfig) *ShellTool {
	if config == nil {
		config = &core.ToolingConfig{}
	}
	return &ShellTool{config: config}
}

func (a *ShellTool) Name() string {
	return "shell"
}

func (a *ShellTool) Description() string {
	return "Execute a shell command on the local machine. Use for file operations, system queries, and automation."
}

func (a *ShellTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"command": {
			"type": "string",
			"description": "The shell command to execute"
		},
		"timeout_seconds": {
			"type": "integer",
			"default": 30
		}
	},
	"required": ["command"]
}`
}

type ShellModel struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func (a *ShellTool) DefaultSandboxMode() core.ToolSandboxMode {
	return core.ToolSandboxMode_Prefer
}

func (a *ShellTool) CreateSandboxRequest(argumentsJson string) (*core.SandboxExecutionRequest, error) {
	if argumentsJson == "" {
		return nil, errors.New("Error: arguments payload is empty.")
	}

	var model ShellModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return nil, err
	}

	if model.Command == "" {
		return nil, errors.New("Error: command is required")
	}

	if model.TimeoutSeconds <= 0 {
		model.TimeoutSeconds = 30
	}

	model.TimeoutSeconds = util.Clamp(model.TimeoutSeconds, 1, 600)

	return &core.SandboxExecutionRequest{
		Command: "/bin/sh",
		Arguments: []string{
			"-lc",
			core.SandboxCommandLineInstance.WrapWithTimeout("/bin/sh", []string{"-lc", model.Command}, model.TimeoutSeconds),
		},
	}, nil
}

func (a *ShellTool) FormatSandboxResult(argumentsJson string, result core.SandboxResult) string {
	if result.ExitCode == 124 {
		return "[exit: timeout]\n[truncated]"
	}

	output := result.Stdout
	if result.Stderr != "" {
		output = fmt.Sprintf("%s\n[stderr]: %s", result.Stdout, result.Stderr)
	}

	return fmt.Sprintf("[exit: %d]\n%s", result.ExitCode, output)
}

func (a *ShellTool) Execute(ctx context.Context, argumentsJson string) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	var model ShellModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if model.Command == "" {
		return "Error: command is required"
	}

	if model.TimeoutSeconds <= 0 {
		model.TimeoutSeconds = 30
	}

	model.TimeoutSeconds = util.Clamp(model.TimeoutSeconds, 1, 600)

	exe := "cmd.exe"
	args := []string{"/c", model.Command}

	if runtime.GOOS != "windows" {
		exe = "/bin/sh"
		args = []string{"-c", model.Command}
	}

	result := util.RunProcess(ctx, exe, args, "", int64(model.TimeoutSeconds), 64*1024, 64*1024)

	if result.Error != "" {
		return result.Error
	}

	output := result.StdoutText
	if result.StderrText != "" {
		output = fmt.Sprintf("%s\n[stderr]: %s", result.StdoutText, result.StderrText)
	}

	return fmt.Sprintf("[exit: %d]\n%s", result.ExitCode, output)
}
