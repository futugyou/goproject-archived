package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/futugyou/openclaw/client"
	"github.com/futugyou/openclaw/core"
)

const (
	DefaultBaseUrl = "http://127.0.0.1:18789"
	EnvBaseUrl     = "OPENCLAW_BASE_URL"
	EnvAuthToken   = "OPENCLAW_AUTH_TOKEN"
)

func ExternalCliCommandRun(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		externalCliCommandsPrintHelp()
		return 0
	}

	parsed := CliArgsParse(args)
	if parsed.ShowHelp || len(parsed.Positionals) == 0 {
		externalCliCommandsPrintHelp()
		return 0
	}

	command := parsed.Positionals[0]
	jsonOutput := parsed.HasFlag("--json")

	if strings.EqualFold(command, "presets") {
		response := &core.ExternalCliPresetListResponse{
			Items: core.ExternalCliPresetCatalogInstance.List(),
		}
		writeOutput(response, jsonOutput, textPresets)
		return 0
	}

	client, err := createClient(parsed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}

	switch command {
	case "list":
		response, err := client.ListExternalCliConnectors(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1
		}
		writeOutput(response, jsonOutput, textConnectors)
		return 0

	case "status":
		connector, err := requiredPosition(parsed, 1, "connector")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		response, err := client.GetExternalCliConnectorStatus(ctx, connector)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1
		}
		writeOutput(response, jsonOutput, textStatus)
		return 0

	case "commands":
		connector, err := requiredPosition(parsed, 1, "connector")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		response, err := client.ListExternalCliCommands(ctx, connector)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1
		}
		writeOutput(response, jsonOutput, textCommands)
		return 0

	case "preview":
		req, err := buildPreviewRequest(parsed)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		response, err := client.PreviewExternalCli(ctx, req)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1
		}
		writeOutput(response, jsonOutput, textPreview)
		return 0

	case "execute":
		req, err := buildPreviewRequest(parsed)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		req.ExecuteDryRun = false

		preview, err := client.PreviewExternalCli(ctx, req)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1
		}

		yesFlag := parsed.HasFlag("--yes")
		if preview.Preview.RequiresApproval && !yesFlag {
			fmt.Fprintln(os.Stderr, "External CLI command requires approval. Re-run with --yes after reviewing preview:")
			textPreview(preview)
			return 3
		}

		var approvedFingerprint string
		if preview.Preview.RequiresApproval {
			approvedFingerprint = preview.Preview.Fingerprint
		}

		reason := getOption(parsed, "--reason")
		execReq := core.ExternalCliExecuteRequest{
			Connector:           req.Connector,
			Command:             req.Command,
			Parameters:          req.Parameters,
			ApprovedFingerprint: approvedFingerprint,
			ApprovalReason:      reason,
		}

		response, err := client.ExecuteExternalCli(ctx, execReq)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1
		}

		writeOutput(response, jsonOutput, textExecution)
		if response.Success {
			return 0
		}
		return 1

	default:
		externalCliCommandsPrintHelp()
		return 2
	}
}

// Helpers
func buildPreviewRequest(parsed *CliArgs) (core.ExternalCliPreviewRequest, error) {
	connector, err := requiredPosition(parsed, 1, "connector")
	if err != nil {
		return core.ExternalCliPreviewRequest{}, err
	}
	command, err := requiredPosition(parsed, 2, "command")
	if err != nil {
		return core.ExternalCliPreviewRequest{}, err
	}
	params, err := parseParameters(parsed)
	if err != nil {
		return core.ExternalCliPreviewRequest{}, err
	}

	return core.ExternalCliPreviewRequest{
		Connector:     connector,
		Command:       command,
		Parameters:    params,
		ExecuteDryRun: parsed.HasFlag("--dry-run"),
	}, nil
}

func parseParameters(parsed *CliArgs) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)
	values, ok := parsed.Options["--param"]
	if !ok {
		return result, nil
	}

	for _, item := range values {
		index := strings.Index(item, "=")
		if index <= 0 {
			return nil, fmt.Errorf("--param must be in key=value form")
		}
		key := strings.TrimSpace(item[:index])
		val := item[index+1:]
		if key == "" {
			return nil, fmt.Errorf("--param key cannot be empty")
		}
		result[key] = []byte(val)
	}
	return result, nil
}

func requiredPosition(parsed *CliArgs, index int, name string) (string, error) {
	if len(parsed.Positionals) > index && strings.TrimSpace(parsed.Positionals[index]) != "" {
		return parsed.Positionals[index], nil
	}
	return "", fmt.Errorf("%s is required", name)
}

func getOption(parsed *CliArgs, name string) string {
	if vals, ok := parsed.Options[name]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func createClient(parsed *CliArgs) (*client.OpenClawHttpClient, error) {
	baseURL := getOption(parsed, "--url")
	if baseURL == "" {
		baseURL = os.Getenv(EnvBaseUrl)
	}
	if baseURL == "" {
		baseURL = DefaultBaseUrl
	}

	token := getOption(parsed, "--token")
	if token == "" {
		token = os.Getenv(EnvAuthToken)
	}
	return client.NewOpenClawHttpClient(baseURL, token, nil)
}

func writeOutput[T any](val T, isJSON bool, textFormatter func(T)) {
	if isJSON {
		data, _ := json.MarshalIndent(val, "", "  ")
		fmt.Println(string(data))
	} else {
		textFormatter(val)
	}
}

// Formatting Output Functions
func textConnectors(resp *core.ExternalCliConnectorListResponse) {
	for _, item := range resp.Items {
		status := "disabled"
		if item.Enabled {
			status = "enabled"
		}
		fmt.Printf("%s\t%s\t%s\tcommands=%d\n", item.Name, status, item.Executable, item.CommandCount)
	}
}

func textPresets(resp *core.ExternalCliPresetListResponse) {
	for _, item := range resp.Items {
		fmt.Printf("%s\tconnector=%s\tcommands=%s\t%s\n", item.Id, item.Connector, strings.Join(item.Commands, ","), item.Description)
	}
}

func textStatus(status *core.ExternalCliConnectorStatus) {
	enabledStr := "disabled"
	if status.Enabled {
		enabledStr = "enabled"
	}
	fmt.Printf("%s\t%s\texecutableFound=%t\tauth=%s\n", status.Connector, enabledStr, status.ExecutableFound, status.AuthenticationStatus)
	if status.ResolvedExecutablePath != "" {
		fmt.Printf("path: %s\n", status.ResolvedExecutablePath)
	}
	if status.Version != "" {
		fmt.Printf("version: %s\n", status.Version)
	}
	for _, warning := range status.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
}

func textCommands(resp *core.ExternalCliCommandListResponse) {
	for _, item := range resp.Items {
		fmt.Printf("%s\t%s\treadOnly=%t\tapproval=%t\t%s\n", item.Name, item.RiskLevel, item.ReadOnly, item.RequiresApproval, item.Description)
	}
}

func textPreview(resp *core.ExternalCliPreviewResponse) {
	p := resp.Preview
	fmt.Printf("%s/%s\n", p.Connector, p.Command)
	fmt.Printf("command: %s\n", p.RedactedCommandLine)
	fmt.Printf("risk: %s readOnly=%t approval=%t\n", p.RiskLevel, p.ReadOnly, p.RequiresApproval)
	fmt.Printf("fingerprint: %s\n", p.Fingerprint)
	if resp.DryRunResult != nil {
		textExecution(resp.DryRunResult)
	}
}

func textExecution(res *core.ExternalCliExecutionResult) {
	fmt.Printf("%s/%s\texit=%d\ttimedOut=%t\tdurationMs=%.0f\n", res.Preview.Connector, res.Preview.Command, res.ExitCode, res.TimedOut, res.DurationMs)
	if strings.TrimSpace(res.Stdout) != "" {
		fmt.Println(strings.TrimRight(res.Stdout, "\r\n"))
	}
	if strings.TrimSpace(res.Stderr) != "" {
		fmt.Fprintln(os.Stderr, strings.TrimRight(res.Stderr, "\r\n"))
	}
	if res.StdoutTruncated || res.StderrTruncated {
		fmt.Println("[output truncated]")
	}
	if strings.TrimSpace(res.ParseError) != "" {
		fmt.Printf("parseError: %s\n", res.ParseError)
	}
}

func externalCliCommandsPrintHelp() {
	fmt.Print(`openclaw external

Usage:
  openclaw external presets [--json]
  openclaw external list [--json]
  openclaw external status <connector> [--json]
  openclaw external commands <connector> [--json]
  openclaw external preview <connector> <command> [--param key=value]... [--dry-run] [--json]
  openclaw external execute <connector> <command> [--param key=value]... [--yes] [--reason text] [--json]

Notes:
  - The presets command is local; other commands talk to the gateway admin API.
  - Mutating or high-risk commands require --yes after preview review.
  - External CLI connectors are disabled unless OpenClaw:ExternalCli:Enabled=true.
`)
}
