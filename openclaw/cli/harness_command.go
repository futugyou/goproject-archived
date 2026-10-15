package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/futugyou/openclaw/client"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/primitives"
	"github.com/futugyou/openclaw/util"
)

func HarnessCommandRun(args []string) (int, error) {
	return harnessCommandRunWithWriters(args, os.Stdout, os.Stderr)
}

func harnessCommandRunWithWriters(args []string, output, errOutput io.Writer) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		harnessCommandPrintHelp(output)
		return 0, nil
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	if subcommand == "state" {
		return runState(args[1:], output, errOutput)
	}
	if subcommand == "map" {
		return runMap(args[1:], output)
	}

	if subcommand != "test" && subcommand != "regression" {
		fmt.Fprintf(errOutput, "Unknown harness subcommand: %s\n", subcommand)
		harnessCommandPrintHelp(output)
		return 2, nil
	}

	parsed := CliArgsParse(args[1:])
	if parsed.ShowHelp {
		harnessCommandPrintHelp(output)
		return 0, nil
	}

	return runRegression(parsed, output)
}

func RunRegressionAlias(args []string, output, errOutput io.Writer) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		harnessPrintAliasHelp(output)
		return 0, nil
	}

	if !strings.EqualFold(args[0], "test") {
		fmt.Fprintf(errOutput, "Unknown regression subcommand: %s\n", args[0])
		harnessPrintAliasHelp(output)
		return 2, nil
	}

	forwarded := append([]string{"test"}, args[1:]...)
	return harnessCommandRunWithWriters(forwarded, output, errOutput)
}

func runRegression(parsed *CliArgs, output io.Writer) (int, error) {
	options := &primitives.HarnessRegressionOptions{
		ConfigPath: util.Deref(parsed.GetOption("--config")),
		Category:   util.Deref(parsed.GetOption("--category")),
		Offline:    true,
		Strict:     parsed.HasFlag("--strict"),
		ProposalId: util.Deref(parsed.GetOption("--proposal")),
		OutputPath: util.Deref(parsed.GetOption("--output")),
	}

	runner := primitives.NewHarnessRegressionRunner(nil)
	report, err := runner.Run(context.Background(), options)
	if err != nil {
		return 1, err
	}

	renderJson := parsed.HasFlag("--json")
	var rendered string
	if renderJson {
		rendered = report.ToJson()
	} else {
		rendered = report.ToText()
	}

	outputPath := parsed.GetOption("--output")
	if outputPath != nil {
		fullPath, err := filepath.Abs(*outputPath)
		if err == nil {
			directory := filepath.Dir(fullPath)
			if strings.TrimSpace(directory) != "" {
				_ = os.MkdirAll(directory, os.ModePerm)
			}
			_ = os.WriteFile(fullPath, []byte(rendered), 0644)
		}
	}

	fmt.Fprint(output, rendered)
	if !renderJson && outputPath != nil {
		fullPath, err := filepath.Abs(*outputPath)
		if err == nil {
			fmt.Fprintf(output, "Report written: %s\n", fullPath)
		}
	}

	return report.GetExitCode(), nil
}

func runMap(args []string, output io.Writer) (int, error) {
	parsed := CliArgsParse(args)
	if parsed.ShowHelp {
		printMapHelp(output)
		return 0, nil
	}

	category := util.Deref(parsed.GetOption("--category"))
	if category == "" {
		category = "all"
	}

	options := &core.CodebaseMapOptions{
		IncludeHashes: parsed.HasFlag("--include-hashes") || parsed.HasFlag("--hash"),
		RecentDays:    getIntOption(parsed, "--recent-days", 30),
		MaxFiles:      getIntOption(parsed, "--max-files", 5000),
		Category:      category,
	}

	root := util.Deref(parsed.GetOption("--root"))
	if root == "" {
		root, _ = os.Getwd()
	}

	service := core.NewCodebaseHarnessMapService()
	codebaseMap, err := service.Generate(context.Background(), root, options)
	if err != nil {
		return 1, err
	}

	isJson := parsed.HasFlag("--json")
	renderedData, err := json.Marshal(codebaseMap)
	var rendered string
	if err != nil {
		rendered = string(renderedData)
	} else {
		rendered = toMapText(codebaseMap)
	}

	outputPath := util.Deref(parsed.GetOption("--output"))
	if strings.TrimSpace(outputPath) != "" {
		fullPath, err := filepath.Abs(outputPath)
		if err == nil {
			directory := filepath.Dir(fullPath)
			if strings.TrimSpace(directory) != "" {
				_ = os.MkdirAll(directory, os.ModePerm)
			}
			_ = os.WriteFile(fullPath, []byte(rendered), 0644)
		}
	}

	fmt.Fprintln(output, rendered)
	if !isJson && strings.TrimSpace(outputPath) != "" {
		fullPath, err := filepath.Abs(outputPath)
		if err == nil {
			fmt.Fprintf(output, "Map written: %s\n", fullPath)
		}
	}

	hasError := false
	for _, diagnostic := range codebaseMap.Diagnostics {
		if diagnostic.Severity == core.CodebaseMapDiagnosticSeverityError {
			hasError = true
			break
		}
	}

	if hasError {
		return 1, nil
	}
	return 0, nil
}

func runState(args []string, output, errOutput io.Writer) (int, error) {
	parsed := CliArgsParse(args)
	if parsed.ShowHelp || len(parsed.Positionals) == 0 {
		printStateHelp(output)
		if parsed.ShowHelp {
			return 0, nil
		}
		return 2, nil
	}

	isJson := parsed.HasFlag("--json")
	client, err := createClient(parsed)
	if err != nil {
		return 1, err
	}
	ctx := context.Background()

	switch strings.ToLower(parsed.Positionals[0]) {
	case "list":
		query := core.SharedHarnessStateListQuery{
			SessionId:         util.Deref(parsed.GetOption("--session")),
			ParentSessionId:   util.Deref(parsed.GetOption("--parent-session")),
			HarnessContractId: util.Deref(parsed.GetOption("--contract")),
			Status:            util.Deref(parsed.GetOption("--status")),
			Tag:               util.Deref(parsed.GetOption("--tag")),
			Limit:             getIntOption(parsed, "--limit", 100),
		}
		response, err := client.ListSharedHarnessState(ctx, query)
		if err != nil {
			return 1, err
		}
		writeList(output, response, isJson)
		return 0, nil

	case "show":
		id, ok := tryGetPosition(parsed, 1, "id", errOutput, output)
		if !ok {
			return 2, nil
		}
		response, err := client.GetSharedHarnessState(ctx, id)
		if err != nil {
			return 1, err
		}
		writeDetail(output, response, isJson)
		if response.State == nil {
			return 1, nil
		}
		return 0, nil

	case "session":
		sessionId, ok := tryGetPosition(parsed, 1, "session-id", errOutput, output)
		if !ok {
			return 2, nil
		}
		response, err := client.GetSharedHarnessStateForSession(ctx, sessionId)
		if err != nil {
			return 1, err
		}
		writeDetail(output, response, isJson)
		if response.State == nil {
			return 1, nil
		}
		return 0, nil

	case "conflicts":
		id, ok := tryGetPosition(parsed, 1, "id", errOutput, output)
		if !ok {
			return 2, nil
		}
		response, err := client.DetectSharedHarnessStateConflicts(ctx, id)
		if err != nil {
			return 1, err
		}
		writeMutation(output, response, isJson)
		if response.Success {
			return 0, nil
		}
		return 1, nil

	default:
		fmt.Fprintf(errOutput, "Unknown harness state subcommand: %s\n", parsed.Positionals[0])
		printStateHelp(output)
		return 2, nil
	}
}

func createClient(parsed *CliArgs) (*client.OpenClawHttpClient, error) {
	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := util.Deref(parsed.GetOption("--token"))
	if token == "" {
		token = os.Getenv(EnvAuthToken)
	}

	return client.NewOpenClawHttpClient(baseUrl, token, nil)
}

func getIntOption(parsed *CliArgs, name string, fallback int) int {
	valStr := util.Deref(parsed.GetOption(name))
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return fallback
}

func tryGetPosition(parsed *CliArgs, index int, name string, errOutput, output io.Writer) (string, bool) {
	if len(parsed.Positionals) > index && strings.TrimSpace(parsed.Positionals[index]) != "" {
		return parsed.Positionals[index], true
	}

	fmt.Fprintf(errOutput, "%s is required.\n", name)
	printStateHelp(output)
	return "", false
}

func writeList(output io.Writer, value *core.SharedHarnessStateListResponse, isJson bool) {
	if isJson {
		d, err := json.Marshal(value)
		if err != nil {
			return
		}
		fmt.Fprintln(output, string(d))
		return
	}
	textStateList(output, value)
}

func writeDetail(output io.Writer, value *core.SharedHarnessStateDetailResponse, isJson bool) {
	if isJson {
		d, err := json.Marshal(value)
		if err != nil {
			return
		}
		fmt.Fprintln(output, string(d))
		return
	}
	textStateDetail(output, value)
}

func writeMutation(output io.Writer, value *core.SharedHarnessStateMutationResponse, isJson bool) {
	if isJson {
		d, err := json.Marshal(value)
		if err != nil {
			return
		}
		fmt.Fprintln(output, string(d))
		return
	}
	textStateMutation(output, value)
}

func textStateList(output io.Writer, response *core.SharedHarnessStateListResponse) {
	fmt.Fprintln(output, "Shared Harness State")
	fmt.Fprintf(output, "Items: %d\n", len(response.Items))
	for _, item := range response.Items {
		sessionId := "-"
		if item.SessionId != "" {
			sessionId = item.SessionId
		}
		fmt.Fprintf(output, "- %s session=%s status=%s participants=%d actions=%d conflicts=%d\n",
			item.Id, sessionId, item.Status, len(item.Participants), len(item.Actions), len(item.Conflicts))
	}
}

func textStateDetail(output io.Writer, response *core.SharedHarnessStateDetailResponse) {
	if response.State == nil {
		fmt.Fprintln(output, "Shared harness state not found.")
		return
	}

	state := response.State
	sessionId := "-"
	if state.SessionId != "" {
		sessionId = state.SessionId
	}

	fmt.Fprintf(output, "Shared Harness State: %s\n", state.Id)
	fmt.Fprintf(output, "Session: %s\n", sessionId)
	fmt.Fprintf(output, "Status: %s\n", state.Status)
	fmt.Fprintf(output, "Goal: %s\n", state.Goal)
	fmt.Fprintf(output, "Participants: %d\n", len(state.Participants))
	for _, participant := range state.Participants {
		pSessionId := "-"
		if participant.SessionId != "" {
			pSessionId = participant.SessionId
		}
		fmt.Fprintf(output, "- %s role=%s session=%s status=%s\n",
			participant.Id, participant.Role, pSessionId, participant.Status)
	}
	fmt.Fprintf(output, "Actions: %d\n", len(state.Actions))
	for _, action := range state.Actions {
		participantId := "-"
		if action.ParticipantId != "" {
			participantId = action.ParticipantId
		}
		fmt.Fprintf(output, "- %s participant=%s status=%s reads=%d writes=%d\n",
			action.Id, participantId, action.Status, len(action.ReadSet), len(action.WriteSet))
	}
	fmt.Fprintf(output, "Conflicts: %d\n", len(state.Conflicts))
	for _, conflict := range state.Conflicts {
		fmt.Fprintf(output, "- %s %s: %s\n", conflict.Type, conflict.Severity, conflict.Summary)
	}
}

func textStateMutation(output io.Writer, response *core.SharedHarnessStateMutationResponse) {
	if !response.Success {
		fmt.Fprintf(output, "Error: %s\n", response.Error)
		return
	}

	fmt.Fprintln(output, response.Message)
	if response.State != nil {
		textStateDetail(output, &core.SharedHarnessStateDetailResponse{State: response.State})
	}
}

func toMapText(codebaseMap *core.CodebaseHarnessMap) string {
	var sb strings.Builder
	sb.WriteString("OpenClaw Codebase Harness Map\n")
	fmt.Fprintf(&sb, "Root: %s\n", codebaseMap.RepositoryRoot)
	fmt.Fprintf(&sb, "Generated: %s\n\n", codebaseMap.GeneratedAtUtc.Format("2006-01-02T15:04:05.9999999Z07:00"))

	sb.WriteString("Summary:\n")
	fmt.Fprintf(&sb, "- Solutions: %d\n", codebaseMap.Summary.SolutionFilesCount)
	fmt.Fprintf(&sb, "- Projects: %d\n", codebaseMap.Summary.ProjectFilesCount)
	fmt.Fprintf(&sb, "- Test projects: %d\n", codebaseMap.Summary.TestProjectsCount)
	fmt.Fprintf(&sb, "- Source files: %d\n", codebaseMap.Summary.SourceFilesCount)
	fmt.Fprintf(&sb, "- Endpoints: %d\n", codebaseMap.Summary.EndpointCount)
	fmt.Fprintf(&sb, "- Tool surfaces: %d\n", codebaseMap.Summary.ToolSurfaceCount)
	fmt.Fprintf(&sb, "- Channel surfaces: %d\n", codebaseMap.Summary.ChannelSurfaceCount)
	fmt.Fprintf(&sb, "- Provider surfaces: %d\n", codebaseMap.Summary.ProviderSurfaceCount)
	fmt.Fprintf(&sb, "- Config surfaces: %d\n", len(codebaseMap.ConfigSurfaces))
	fmt.Fprintf(&sb, "- Recent changes: %d\n\n", codebaseMap.Summary.RecentChangeCount)

	sb.WriteString("Modules:\n")
	limitModules := min(len(codebaseMap.Modules), 24)
	for i := 0; i < limitModules; i++ {
		module := codebaseMap.Modules[i]
		fmt.Fprintf(&sb, "- %s: %s (%s)\n", module.Name, module.Path, module.Kind)
	}
	if len(codebaseMap.Modules) > 24 {
		fmt.Fprintf(&sb, "- ... %d more\n", len(codebaseMap.Modules)-24)
	}

	if len(codebaseMap.Endpoints) > 0 {
		sb.WriteString("\nEndpoints:\n")
		limitEndpoints := min(len(codebaseMap.Endpoints), 24)
		for i := range limitEndpoints {
			endpoint := codebaseMap.Endpoints[i]
			fmt.Fprintf(&sb, "- %s %s (%s)\n", endpoint.Method, endpoint.Path, endpoint.SourceFile)
		}
		if len(codebaseMap.Endpoints) > 24 {
			fmt.Fprintf(&sb, "- ... %d more\n", len(codebaseMap.Endpoints)-24)
		}
	}

	var warnings []core.CodebaseMapDiagnostic
	for _, item := range codebaseMap.Diagnostics {
		if item.Severity == core.CodebaseMapDiagnosticSeverityWarning || item.Severity == core.CodebaseMapDiagnosticSeverityError {
			warnings = append(warnings, item)
		}
	}

	if len(warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		limitWarnings := min(len(warnings), 24)
		for i := range limitWarnings {
			warning := warnings[i]
			fmt.Fprintf(&sb, "- %s: %s\n", warning.Code, warning.Message)
		}
		if len(warnings) > 24 {
			fmt.Fprintf(&sb, "- ... %d more\n", len(warnings)-24)
		}
	}

	return sb.String()
}

func harnessCommandPrintHelp(output io.Writer) {
	fmt.Fprintln(output, `openclaw harness

Usage:
  openclaw harness test [--config <path>] [--category <name>] [--json] [--offline] [--strict] [--proposal <id>] [--output <path>]
  openclaw harness regression [--config <path>] [--category <name>] [--json] [--offline] [--strict] [--proposal <id>] [--output <path>]
  openclaw harness map [--root <path>] [--json] [--output <path>] [--include-hashes|--hash] [--recent-days <n>] [--max-files <n>] [--category <name>]
  openclaw harness state <list|show|session|conflicts> [options]

Categories:
  onboarding, security, approvals, memory, providers, tools, mcp,
  openai_compat, sessions, harness, deployment, docs

Notes:
  - Runs offline by default and does not require provider keys.
  - Provider/network checks are structural only unless a future online mode is added.
  - Strict mode treats required warnings and skips as failures.`)
}

func printStateHelp(output io.Writer) {
	fmt.Fprintln(output, `openclaw harness state

Usage:
  openclaw harness state list [--session <id>] [--parent-session <id>] [--contract <id>] [--status <name>] [--tag <tag>] [--limit <n>] [--json]
  openclaw harness state show <id> [--json]
  openclaw harness state session <session-id> [--json]
  openclaw harness state conflicts <id> [--json]

Common options:
  --url <url>
  --token <token>`)
}

func printMapHelp(output io.Writer) {
	fmt.Fprintln(output, `openclaw harness map

Usage:
  openclaw harness map [--root <path>] [--json] [--output <path>] [--include-hashes|--hash] [--recent-days <n>] [--max-files <n>] [--category <name>]

Categories:
  all, projects, endpoints, tools, providers, channels, config, tests

Notes:
  - Runs local static scanning only.
  - Does not build projects or execute repository code.`)
}

func harnessPrintAliasHelp(output io.Writer) {
	fmt.Fprintln(output, `openclaw regression

Usage:
  openclaw regression test [--config <path>] [--category <name>] [--json] [--offline] [--strict] [--proposal <id>] [--output <path>]`)
}
