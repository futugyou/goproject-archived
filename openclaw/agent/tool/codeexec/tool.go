package codeexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

const MaxOutputBytes int = 64 * 1024
const ShellProbeTimeoutMs int = 10_000

type CodeExecTool struct {
	config        core.CodeExecConfig
	toolingConfig *core.ToolingConfig
}

var BashProcessCommand *ResolvedProcessCommand

func resolveBashProcessCommand() *ResolvedProcessCommand {
	if runtime.GOOS == "windows" {
		if canRunProcess("wsl.exe", []string{"-e", "sh", "-lc", "exit 0"}) {
			return NewResolvedProcessCommand("wsl.exe", []string{"-e", "sh", "-lc"})
		}

		if canRunProcess("bash", []string{"-lc", "exit 0"}) {
			return NewResolvedProcessCommand("bash", []string{"-lc"})
		}

		return nil
	}

	if canRunProcess("bash", []string{"-lc", "exit 0"}) {
		return NewResolvedProcessCommand("bash", []string{"-lc"})
	}

	return nil
}
func init() {
	BashProcessCommand = resolveBashProcessCommand()
}

func New(config core.CodeExecConfig, toolingConfig *core.ToolingConfig) *CodeExecTool {
	return &CodeExecTool{
		config:        config,
		toolingConfig: toolingConfig,
	}
}

func (a *CodeExecTool) Name() string {
	return "code_exec"
}

func (a *CodeExecTool) Description() string {
	return "Execute a code snippet and return the output. Supports python, javascript, and bash. Use for calculations, data processing, and automation."
}

func (a *CodeExecTool) ParameterSchema() string {
	return `
	{
          "type": "object",
          "properties": {
            "language": {
              "type": "string",
              "description": "Programming language: python, javascript, or bash",
              "enum": ["python", "javascript", "bash"]
            },
            "code": {
              "type": "string",
              "description": "The code to execute"
            },
            "timeout_seconds": {
              "type": "integer",
              "description": "Execution timeout (default: from config)"
            }
          },
          "required": ["language", "code"]
        }
`
}

func (a *CodeExecTool) DefaultSandboxMode() core.ToolSandboxMode {
	return core.ToolSandboxMode_Prefer
}

type ArgumentModel struct {
	Language       string `json:"language"`
	Code           string `json:"code"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func (a *CodeExecTool) tryParseArguments(argumentsJson string) (language string, code string, timeoutSec int, errstr string, result bool) {
	timeoutSec = a.config.TimeoutSeconds

	if a.toolingConfig != nil || a.toolingConfig.ReadOnlyMode {
		errstr = "Error: code_exec is disabled because Tooling.ReadOnlyMode is enabled."
		return
	}

	var args ArgumentModel
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		errstr = err.Error()
		return
	}

	if args.Language == "" {
		errstr = "Error: 'language' is required."
		return
	}
	language = strings.ToLower(args.Language)
	if len(a.config.AllowedLanguages) > 0 && !slices.Contains(a.config.AllowedLanguages, language) {
		errstr = fmt.Sprintf("Error: Language '%s' is not allowed. Allowed: %s", language, strings.Join(a.config.AllowedLanguages, ", "))
		return
	}

	if args.Code == "" {
		errstr = "Error: 'code' is required."
		return
	}
	code = args.Code

	timeoutSec = args.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = a.config.TimeoutSeconds
	}

	timeoutSec = util.Clamp(timeoutSec, 1, 300)

	result = true
	return
}

func getSandboxCommand(language, code string) (string, []string) {
	switch language {
	case "python":
		return "python3", []string{"-c", code}
	case "javascript":
		return "node", []string{"-e", code}
	case "bash":
		return "bash", []string{"-lc", code}
	default:
		return "", nil
	}
}

func (a *CodeExecTool) CreateSandboxRequest(argumentsJson string) (*core.SandboxExecutionRequest, error) {
	language, code, timeoutSec, errstr, ok := a.tryParseArguments(argumentsJson)
	if !ok {
		return nil, errors.New(errstr)
	}

	interpreter, arguments := getSandboxCommand(language, code)
	if interpreter == "" || len(arguments) == 0 {
		return nil, fmt.Errorf("Error: Unsupported language '%s'.", language)
	}

	return &core.SandboxExecutionRequest{
		Command: "/bin/sh",
		Arguments: []string{
			"-lc",
			core.SandboxCommandLineInstance.WrapWithTimeout(interpreter, arguments, timeoutSec),
		},
	}, nil
}

func (a *CodeExecTool) FormatSandboxResult(argumentsJson string, result core.SandboxResult) string {
	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "Exit code: %d\n", result.ExitCode)

	if result.Stdout != "" {
		sb.WriteString("--- stdout ---\n")
		sb.WriteString(result.Stdout)
	}

	if result.Stderr != "" {
		if sb.Len() > 0 && sb.String()[sb.Len()-1] != '\n' {
			sb.WriteString("\n")
		}

		sb.WriteString("--- stderr ---\n")
		sb.WriteString(result.Stderr)
	}

	return sb.String()
}

func (a *CodeExecTool) runProcess(ctx context.Context, exe string, args []string, timeoutSec int) string {
	result := util.RunProcess(ctx, exe, args, "", int64(timeoutSec), int64(a.config.MaxOutputBytes), 8192)
	if result.Error != "" {
		return result.Error
	}
	// 拼接输出结果
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Exit code: %d\n", result.ExitCode))

	stdout := strings.TrimSpace(result.StdoutText)
	if stdout != "" {
		sb.WriteString("--- stdout ---\n")
		sb.WriteString(result.StdoutText)
		sb.WriteString("\n")
	}

	stderr := strings.TrimSpace(result.StderrText)
	if stderr != "" {
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("--- stderr ---\n")
		sb.WriteString(result.StderrText)
	}

	return sb.String()
}

func canRunProcess(executable string, arguments []string) bool {
	// 创建超时 Context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ShellProbeTimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, arguments...)

	err := cmd.Run()
	if err != nil {
		return false
	}

	return cmd.ProcessState.ExitCode() == 0
}

type ResolvedProcessCommand struct {
	Executable      string
	PrefixArguments []string
}

func NewResolvedProcessCommand(executable string, pefixArguments []string) *ResolvedProcessCommand {
	return &ResolvedProcessCommand{
		Executable:      executable,
		PrefixArguments: pefixArguments,
	}
}

func getInterpreter(language string) (string, string) {
	switch language {
	case "python":
		return "python3", ""
	case "javascript":
		return "node", ""
	case "bash":
		return "pytbashhon3", ""
	default:
		return "", ""
	}
}

func addArgumentTokens(input []string, flags string) (output []string) {
	output = input

	if strings.TrimSpace(flags) == "" {
		return output
	}

	var currentToken = strings.Builder{}
	var inQuotes = false
	quoteChar := '\x00'

	for _, c := range flags {
		if inQuotes {
			if c == quoteChar {
				inQuotes = false
			} else {
				currentToken.WriteRune(c)
			}
		} else {
			if c == '"' || c == '\'' {
				inQuotes = true
				quoteChar = c
			} else if unicode.IsSpace(c) {
				if currentToken.Len() > 0 {
					output = append(output, currentToken.String())
					currentToken.Reset()
				}
			} else {
				currentToken.WriteRune(c)
			}
		}
	}

	if currentToken.Len() > 0 {
		output = append(output, currentToken.String())
	}

	return
}

func (a *CodeExecTool) runInDocker(ctx context.Context, language, code string, timeoutSec int) string {
	interpreter, flags := getInterpreter(language)
	if interpreter == "" {
		return fmt.Sprintf("Error: Unsupported language '%s'.", language)
	}

	ext := ""
	switch language {
	case "python":
		ext = ".py"
	case "javascript":
		ext = ".js"
	case "bash":
		ext = ".sh"
	default:
		ext = ".txt"
	}

	tmpFile, err := os.CreateTemp("", "code-*"+ext)
	if err != nil {
		return "failed to create temp file"
	}

	codeFilePath := tmpFile.Name()

	defer func() {
		tmpFile.Close()
		_ = os.Remove(codeFilePath)
	}()
	// 2. 将代码写入临时文件
	if _, err := tmpFile.WriteString(code); err != nil {
		return "failed to write code to temp file"
	}

	_ = tmpFile.Close()

	mountPath := fmt.Sprintf("%s:/code%s:ro", codeFilePath, ext)
	codeInContainer := fmt.Sprintf("/code%s", ext)

	dockerArgs := []string{
		"run",
		"--rm",
		"--network", "none",
		"--memory=256m",
		"--cpus=1",
		"-v", mountPath,
		"-w", "/tmp",
		a.config.DockerImage,
		interpreter,
	}

	// 追加解释器参数标志 (Flags)
	dockerArgs = addArgumentTokens(dockerArgs, flags)
	// 追加容器内的脚本文件路径
	dockerArgs = append(dockerArgs, codeInContainer)
	return a.runProcess(ctx, "docker", dockerArgs, timeoutSec)
}

func (a *CodeExecTool) Execute(ctx context.Context, argumentsJson string) string {
	language, code, timeoutSec, errstr, ok := a.tryParseArguments(argumentsJson)
	if !ok {
		return errstr
	}
	backend := strings.ToLower(a.config.Backend)
	switch backend {
	case "docker":
		return a.runInDocker(ctx, language, code, timeoutSec)
	case "process":
		return a.runInProcess(ctx, language, code, timeoutSec)
	default:
		return fmt.Sprintf("Error: Unsupported backend '%s'. Use 'docker' or 'process'.", a.config.Backend)
	}
}

func (a *CodeExecTool) runInProcess(ctx context.Context, language string, code string, timeoutSec int) string {
	if language == "bash" {
		var command = BashProcessCommand
		if command == nil {
			return "Error: Bash execution is not available on this host."
		}

		return a.runProcess(ctx, command.Executable, append(command.PrefixArguments, code), timeoutSec)
	}

	interpreter, flags := getInterpreter(language)
	if interpreter == "" {
		return fmt.Sprintf("Error: Unsupported language '%s'.", language)
	}

	// Write code to a temp file
	ext := ""
	switch language {
	case "python":
		ext = ".py"
	case "javascript":
		ext = ".js"
	case "bash":
		ext = ".sh"
	default:
		ext = ".txt"
	}

	tmpFile, err := os.CreateTemp("", "openclaw-exec-"+util.CleanUUID()+ext)
	if err != nil {
		return "failed to create temp file"
	}

	codeFilePath := tmpFile.Name()

	defer func() {
		tmpFile.Close()
		_ = os.Remove(codeFilePath)
	}()
	if _, err := tmpFile.WriteString(code); err != nil {
		return "failed to write code to temp file"
	}

	_ = tmpFile.Close()

	args := addArgumentTokens([]string{}, flags)
	args = append(args, codeFilePath)

	return a.runProcess(ctx, interpreter, args, timeoutSec)
}
