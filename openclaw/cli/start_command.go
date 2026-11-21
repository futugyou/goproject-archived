package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/futugyou/openclaw/util"
)

type StartCommandHandlers struct {
	ConfigExists func(name string) bool
	RunSetup     func(args []string, input io.Reader, output io.Writer, errwirter io.Writer, currentDirectory string, canPrompt bool) (*SetupCommandResult, error)
	RunLaunch    func(args []string, output io.Writer, errWriter io.Writer) (int, error)
}

func DefaultStartCommandHandlers() *StartCommandHandlers {
	return &StartCommandHandlers{
		ConfigExists: util.FileExists,
		RunSetup:     SetupCommandRun,
		RunLaunch:    SetupLifecycleCommandRunLaunch,
	}
}

func ensureConfigArgument(args []string, configPath string) []string {
	rewritten := []string{}
	foundConfigArgument := false
	for i := range args {
		if args[i] == "--config" {
			foundConfigArgument = true
			rewritten = append(rewritten, "--config")
			rewritten = append(rewritten, configPath)
			if i+1 < len(args) {
				i++
			}
			continue
		}
		rewritten = append(rewritten, args[i])
	}

	if foundConfigArgument {
		return rewritten
	}

	return append([]string{"--config", configPath}, args...)
}

func StartCommandRun(args []string, input io.Reader, output io.Writer, errwirter io.Writer, currentDirectory string, canPrompt bool, handlers *StartCommandHandlers) (int, error) {
	parsed := CliArgsParse(args)
	if parsed.ShowHelp {
		StartCommandWriteHelp(output)
		return 0, nil
	}

	if handlers == nil {
		handlers = DefaultStartCommandHandlers()
	}

	configPath, err := resolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	launchArgs := ensureConfigArgument(args, configPath)

	if handlers.ConfigExists(configPath) {
		fmt.Fprintf(output, "Using config: %s\n", configPath)
		return handlers.RunLaunch(launchArgs, output, errwirter)
	}

	if canPrompt {
		fmt.Fprintf(output, "No config found at %s. Running guided setup.", configPath)
	} else {
		fmt.Fprintf(output, "No config found at %s. Running setup with the provided arguments.", configPath)
	}

	setupResult, err := handlers.RunSetup(args, input, output, errwirter, currentDirectory, canPrompt)
	if err != nil {
		return 1, err
	}
	if setupResult.ExitCode != 0 {
		return setupResult.ExitCode, errors.New("start up hander run error")
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Setup completed. Launching gateway...")
	launchConfigPath := setupResult.ConfigPath
	if launchConfigPath == "" {
		launchConfigPath = configPath
	}

	return handlers.RunLaunch(ensureConfigArgument(args, launchConfigPath), output, errwirter)
}

func StartCommandWriteHelp(output io.Writer) {
	msg := `openclaw start

            Usage:
              openclaw start [--config <path>] [--with-companion] [--open-browser] [--skip-verify] [--offline] [--require-provider]
                              [--profile <local|public>] [--non-interactive]
                              [--workspace <path>] [--provider <id>] [--model <id>] [--model-preset <id>] [--api-key <secret-or-envref>]
                              [--bind <address>] [--port <n>] [--auth-token <token>]
                              [--docker-image <image>] [--opensandbox-endpoint <url>] [--ssh-host <host>] [--ssh-user <user>] [--ssh-key <path>]

            Notes:
              - This is the primary supported local entrypoint.
              - If the config already exists, it launches the gateway with verification.
              - If the config is missing, it runs setup first and then launches.
              - Use --non-interactive together with explicit setup inputs for automation or CI.`
	fmt.Fprintln(output, msg)
}
