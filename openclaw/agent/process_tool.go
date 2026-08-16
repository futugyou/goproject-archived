package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
)

type ProcessTool struct {
	processes *ExecutionProcessService
	tooling   core.ToolingConfig
}

func NewProcessTool(
	processes *ExecutionProcessService,
	tooling *core.ToolingConfig) *ProcessTool {
	if tooling == nil {
		tooling = &core.ToolingConfig{}
	}
	return &ProcessTool{processes: processes, tooling: *tooling}
}

func (a *ProcessTool) Name() string {
	return "process"
}

func (a *ProcessTool) Description() string {
	return "Manage long-running background processes. Supports start, list, poll, log, wait, write, and kill actions."
}

func (a *ProcessTool) ParameterSchema() string {
	return `
{
      "type":"object",
      "properties":{
        "action":{"type":"string","enum":["start","list","poll","log","wait","write","kill"],"default":"start"},
        "command":{"type":"string","description":"Shell command to start when action=start."},
        "process_id":{"type":"string","description":"Background process id for poll/log/wait/write/kill."},
        "timeout_seconds":{"type":"integer","minimum":1,"maximum":3600},
        "pty":{"type":"boolean","default":false},
        "input":{"type":"string","description":"Input text to write when action=write."},
        "stdout_offset":{"type":"integer","minimum":0},
        "stderr_offset":{"type":"integer","minimum":0},
        "max_chars":{"type":"integer","minimum":1,"maximum":65536},
        "working_directory":{"type":"string"},
        "backend":{"type":"string"},
        "session_id":{"type":"string","description":"Optional owner session filter for list."}
      },
      "required":["action"]
    }`
}

func (a *ProcessTool) Execute(ctx context.Context, argumentsJson string) string {
	return "Error: process requires execution context."
}

type ProcessModel struct {
	Action           string `json:"action"`
	Command          string `json:"command"`
	ProcessId        string `json:"process_id"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
	Pty              bool   `json:"pty"`
	Input            string `json:"input"`
	StdoutOffset     int    `json:"stdout_offset"`
	StderrOffset     int    `json:"stderr_offset"`
	MaxChars         int    `json:"max_chars"`
	WorkingDirectory string `json:"working_directory"`
	Backend          string `json:"backend"`
	SessionId        string `json:"session_id"`
}

func (a *ProcessTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) string {
	if a.tooling.ReadOnlyMode {
		return "Error: process is disabled because Tooling.ReadOnlyMode is enabled."
	}
	if !a.tooling.AllowShell {
		return "Error: process is disabled because shell execution is disabled by configuration."
	}

	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	var model ProcessModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if model.Action == "" {
		model.Action = "start"
	}

	switch model.Action {
	case "start":
		return a.start(ctx, model, toolContext)
	case "list":
		return a.list(ctx, model, toolContext)
	case "poll":
		return a.poll(ctx, model, toolContext)
	case "log":
		return a.log(ctx, model, toolContext)
	case "wait":
		return a.wait(ctx, model, toolContext)
	case "write":
		return a.write(ctx, model, toolContext)
	case "kill":
		return a.kill(ctx, model, toolContext)
	default:
		return "Error: Unknown action. Valid actions are start, list, poll, log, wait, write, and kill."
	}
}

func (a *ProcessTool) start(ctx context.Context, model ProcessModel, toolContext core.ToolExecutionContext) string {
	var subcommand = model.Command
	if subcommand == "" {
		return "Error: command is required for process start."
	}

	command := "cmd.exe"
	arguments := []string{"/c", subcommand}
	if runtime.GOOS != "windows" {
		command = "/bin/sh"
		arguments = []string{"-lc", subcommand}
	}

	handle, err := a.processes.Start(ctx, &core.ExecutionProcessStartRequest{
		ToolName:         a.Name(),
		BackendName:      model.Backend,
		OwnerSessionId:   toolContext.Session.Id,
		OwnerChannelId:   toolContext.Session.ChannelId,
		OwnerSenderId:    toolContext.Session.SenderId,
		WorkingDirectory: model.WorkingDirectory,
		RequireWorkspace: true,
		Pty:              model.Pty,
		Command:          command,
		Arguments:        arguments,
	})
	if err != nil {
		return err.Error()
	}

	return fmt.Sprintf("Started process %s\nbackend: %s\ncommand: %s",
		handle.ProcessId,
		handle.BackendName,
		handle.CommandPreview,
	)
}

func (a *ProcessTool) list(_ context.Context, model ProcessModel, toolContext core.ToolExecutionContext) string {
	ownerSessionId := model.SessionId
	if ownerSessionId == "" {
		ownerSessionId = toolContext.Session.Id
	}
	var items = a.processes.List(ownerSessionId)
	if len(items) == 0 {
		return fmt.Sprintf("No background processes found for session %s.", ownerSessionId)
	}

	var sb = strings.Builder{}
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("%s [%s] %s", item.ProcessId, item.State, item.CommandPreview))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (a *ProcessTool) poll(_ context.Context, model ProcessModel, toolContext core.ToolExecutionContext) string {
	var processId = model.ProcessId
	if processId == "" {
		return "Error: process_id is required."
	}
	var status = a.processes.GetStatus(processId, toolContext.Session.Id)
	if status == nil {
		return fmt.Sprintf("Error: process '%s' was not found.", processId)
	}

	exit := "-"
	if status.ExitCode != nil {
		exit = strconv.Itoa(*status.ExitCode)
	}
	return fmt.Sprintf("%s [%s] exit=%s stdout=%d stderr=%d",
		status.ProcessId,
		status.State,
		exit,
		status.StdoutBytes,
		status.StderrBytes,
	)
}

func (a *ProcessTool) log(_ context.Context, model ProcessModel, toolContext core.ToolExecutionContext) string {
	var processId = model.ProcessId
	if processId == "" {
		return "Error: process_id is required."
	}

	log := a.processes.ReadLog(&core.ExecutionProcessLogRequest{
		ProcessId:      processId,
		OwnerSessionId: toolContext.Session.Id,
		StdoutOffset:   model.StdoutOffset,
		StderrOffset:   model.StderrOffset,
		MaxChars:       model.MaxChars,
	})

	if log == nil {
		return fmt.Sprintf("Error: process '%s' was not found.", processId)
	}

	return fmt.Sprintf("[stdout]\n%s\n[stderr]\n%s\n[next_stdout_offset=%d next_stderr_offset=%d]",
		log.Stdout,
		log.Stderr,
		log.NextStdoutOffset,
		log.NextStderrOffset,
	)
}

func (a *ProcessTool) wait(ctx context.Context, model ProcessModel, toolContext core.ToolExecutionContext) string {
	var processId = model.ProcessId
	if processId == "" {
		return "Error: process_id is required."
	}

	if model.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(model.TimeoutSeconds*int(time.Second)))
		defer cancel()
	}

	status, err := a.processes.Wait(ctx, processId, toolContext.Session.Id)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			var status = a.processes.GetStatus(processId, toolContext.Session.Id)
			if status == nil {
				return fmt.Sprintf("Error: process '%s' was not found.", processId)
			}

			if status.State == "running" {
				return fmt.Sprintf("%s still running state=%s", status.ProcessId, status.State)
			} else {
				return fmt.Sprintf("%s completed with state=%s", status.ProcessId, status.State)
			}
		}
		return err.Error()
	}
	if status != nil {
		return fmt.Sprintf("%s completed with state=%s", status.ProcessId, status.State)
	} else {
		return fmt.Sprintf("Error: process '%s' was not found.", processId)
	}
}

func (a *ProcessTool) write(ctx context.Context, model ProcessModel, toolContext core.ToolExecutionContext) string {
	var processId = model.ProcessId
	if processId == "" {
		return "Error: process_id is required."
	}
	input := model.Input
	if input == "" {
		return "Error: input is required."
	}

	f, err := a.processes.Write(ctx, core.ExecutionProcessInputRequest{
		ProcessId:      processId,
		OwnerSessionId: toolContext.Session.Id,
		Data:           input,
	})
	if err != nil {
		return err.Error()
	}
	if f {
		return fmt.Sprintf("Input written to process %s", processId)
	} else {
		return fmt.Sprintf("Error: process '%s' was not found.", processId)
	}
}

func (a *ProcessTool) kill(ctx context.Context, model ProcessModel, toolContext core.ToolExecutionContext) string {
	var processId = model.ProcessId
	if processId == "" {
		return "Error: process_id is required."
	}

	f, err := a.processes.Kill(ctx, processId, toolContext.Session.Id)
	if err != nil {
		return err.Error()
	}
	if f {
		return fmt.Sprintf("Process %s terminated.", processId)
	} else {
		return fmt.Sprintf("Error: process '%s' was not found.", processId)
	}
}
