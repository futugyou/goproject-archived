package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	envWorkspace               = "OPENCLAW_WORKSPACE"
	envClawHubWorkdir          = "CLAWHUB_WORKDIR"
	envClawHubDisableTelemetry = "CLAWHUB_DISABLE_TELEMETRY"
)

type TelemetryMode int

const (
	TelemetryOff TelemetryMode = iota
	TelemetryOn
)

type WrapperArgs struct {
	ShowHelp    bool
	Workdir     string
	UseManaged  bool
	Telemetry   TelemetryMode
	ForwardArgs []string
}

type InvocationSpec struct {
	FileName  string
	Arguments []string
	Workdir   string
	Telemetry TelemetryMode
}

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

func ClawHubCommandRun(ctx context.Context, args []string) int {
	parsed, err := ParseArgs(args)
	if err != nil {
		if usageErr, ok := errors.AsType[*UsageError](err); ok {
			fmt.Fprintln(os.Stderr, usageErr.Error())
			fmt.Fprintln(os.Stderr, "Run: openclaw clawhub --help")
			return 2
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	if parsed.ShowHelp {
		ClawHubPrintHelp()
		return 0
	}

	home, err := getHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine user home directory: %v\n", err)
		return 1
	}

	workspaceEnv := os.Getenv(envWorkspace)
	workdir, err := ResolveWorkdir(parsed, workspaceEnv, home)
	if err != nil {
		if usageErr, ok := errors.AsType[*UsageError](err); ok {
			fmt.Fprintln(os.Stderr, usageErr.Error())
			fmt.Fprintln(os.Stderr, "Run: openclaw clawhub --help")
			return 2
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	if err := os.MkdirAll(workdir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create workdir '%s': %v\n", workdir, err)
		return 2
	}

	spec, err := BuildInvocationSpec(parsed, workdir)
	if err != nil {
		if usageErr, ok := errors.AsType[*UsageError](err); ok {
			fmt.Fprintln(os.Stderr, usageErr.Error())
			fmt.Fprintln(os.Stderr, "Run: openclaw clawhub --help")
			return 2
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	return RunClawHub(ctx, spec)
}

func ParseArgs(args []string) (*WrapperArgs, error) {
	var (
		showHelp   bool
		workdir    string
		useManaged bool
		telemetry  = TelemetryOff
		forward    []string
	)

	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "-h" || a == "--help" {
			showHelp = true
			continue
		}

		if a == "--" {
			if i+1 < len(args) {
				forward = append(forward, args[i+1:]...)
			}
			break
		}

		if a == "--workdir" {
			if i+1 >= len(args) {
				return nil, &UsageError{Message: "Missing value for --workdir"}
			}
			i++
			workdir = args[i]
			continue
		}

		if a == "--managed" {
			useManaged = true
			continue
		}

		if a == "--telemetry" {
			if i+1 >= len(args) {
				return nil, &UsageError{Message: "Missing value for --telemetry (expected: on|off)"}
			}
			i++
			v := strings.ToLower(strings.TrimSpace(args[i]))
			switch v {
			case "on":
				telemetry = TelemetryOn
			case "off":
				telemetry = TelemetryOff
			default:
				return nil, &UsageError{Message: "Invalid value for --telemetry (expected: on|off)"}
			}
			continue
		}

		if strings.HasPrefix(a, "--") {
			return nil, &UsageError{Message: fmt.Sprintf("Unknown option: %s (use -- to forward flags to ClawHub)", a)}
		}

		// First non-wrapper token => start forwarding, including this one.
		forward = append(forward, args[i:]...)
		break
	}

	if useManaged && strings.TrimSpace(workdir) != "" {
		return nil, &UsageError{Message: "Cannot use --managed together with --workdir"}
	}

	return &WrapperArgs{
		ShowHelp:    showHelp,
		Workdir:     workdir,
		UseManaged:  useManaged,
		Telemetry:   telemetry,
		ForwardArgs: forward,
	}, nil
}

func ResolveWorkdir(args *WrapperArgs, openclawWorkspace, userProfilePath string) (string, error) {
	if strings.TrimSpace(args.Workdir) != "" {
		abs, err := filepath.Abs(args.Workdir)
		if err != nil {
			return "", fmt.Errorf("invalid workdir path: %w", err)
		}
		return abs, nil
	}

	if args.UseManaged {
		return filepath.Join(userProfilePath, ".openclaw"), nil
	}

	if strings.TrimSpace(openclawWorkspace) != "" {
		abs, err := filepath.Abs(openclawWorkspace)
		if err != nil {
			return "", fmt.Errorf("invalid workspace path: %w", err)
		}
		return abs, nil
	}

	return "", &UsageError{
		Message: fmt.Sprintf("Missing %s. Set %s or pass --workdir or --managed.", envWorkspace, envWorkspace),
	}
}

func BuildInvocationSpec(args *WrapperArgs, resolvedWorkdir string) (*InvocationSpec, error) {
	if len(args.ForwardArgs) == 0 {
		return nil, &UsageError{Message: "Missing ClawHub args. Example: openclaw clawhub search \"calendar\""}
	}

	if runtime.GOOS == "windows" {
		// Prefer cmd.exe so npm-installed clawhub.cmd shims work.
		cmdArgs := []string{"/d", "/s", "/c", "clawhub"}
		cmdArgs = append(cmdArgs, args.ForwardArgs...)
		return &InvocationSpec{
			FileName:  "cmd.exe",
			Arguments: cmdArgs,
			Workdir:   resolvedWorkdir,
			Telemetry: args.Telemetry,
		}, nil
	}

	return &InvocationSpec{
		FileName:  "clawhub",
		Arguments: args.ForwardArgs,
		Workdir:   resolvedWorkdir,
		Telemetry: args.Telemetry,
	}, nil
}

func RunClawHub(ctx context.Context, spec *InvocationSpec) int {
	cmd := exec.CommandContext(ctx, spec.FileName, spec.Arguments...)

	// Configure environment variables
	envMap := BuildChildEnvironment(spec.Workdir, spec.Telemetry)
	cmd.Env = mergeEnvironment(os.Environ(), envMap)

	// Stream stdout and stderr directly
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return exitErr.ExitCode()
		}

		// Check if binary is missing (equivalent to Win32Exception NativeErrorCode == 2)
		if errors.Is(err, exec.ErrNotFound) {
			PrintMissingClawHub(spec.FileName)
			return 127
		}

		fmt.Fprintf(os.Stderr, "Failed to execute ClawHub: %v\n", err)
		return 1
	}

	return 0
}

func BuildChildEnvironment(workdir string, telemetry TelemetryMode) map[string]*string {
	env := make(map[string]*string)

	env[envClawHubWorkdir] = &workdir

	if telemetry == TelemetryOff {
		val := "1"
		env[envClawHubDisableTelemetry] = &val
	} else {
		// Nil indicates that the variable should be unset
		env[envClawHubDisableTelemetry] = nil
	}

	return env
}

func mergeEnvironment(currentEnv []string, overrides map[string]*string) []string {
	envMap := make(map[string]string)
	for _, kv := range currentEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	for k, v := range overrides {
		if v == nil {
			delete(envMap, k)
		} else {
			envMap[k] = *v
		}
	}

	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

func getHomeDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err == nil {
		return dir, nil
	}

	u, err := user.Current()
	if err == nil {
		return u.HomeDir, nil
	}

	return "", err
}

func PrintMissingClawHub(attemptedExecutable string) {
	fmt.Fprintf(os.Stderr, "ClawHub CLI not found on PATH (tried '%s').\n", attemptedExecutable)
	fmt.Fprintln(os.Stderr, "Install it with one of:")
	fmt.Fprintln(os.Stderr, "  npm i -g clawhub")
	fmt.Fprintln(os.Stderr, "  pnpm add -g clawhub")
	fmt.Fprintln(os.Stderr, "Then re-run: openclaw clawhub -- --help")
}

func ClawHubPrintHelp() {
	helpText := fmt.Sprintf(`openclaw clawhub — ClawHub CLI wrapper

Usage:
  openclaw clawhub [wrapper options] [--] <clawhub args...>

Wrapper options:
  --help, -h         Show this help.
  --workdir <path>   Set %s for the child process.
  --managed          Use ~/.openclaw as workdir (installs into ~/.openclaw/skills).
  --telemetry on|off Default: off. When off, sets %s=1 for the child.
  --                 Forward all remaining args verbatim to ClawHub.

Workdir resolution (when no --workdir/--managed):
  - Uses %s as the workdir.

Examples:
  # Forward --help to ClawHub itself:
  openclaw clawhub -- --help

  # Search and install into $OPENCLAW_WORKSPACE/skills:
  openclaw clawhub search "calendar"
  openclaw clawhub install <skill-slug>

  # Install into ~/.openclaw/skills:
  openclaw clawhub --managed install <skill-slug>
`, envClawHubWorkdir, envClawHubDisableTelemetry, envWorkspace)

	fmt.Print(helpText)
}
