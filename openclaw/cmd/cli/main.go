package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/cli"
	"github.com/futugyou/openclaw/client"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/tui"
	"github.com/futugyou/openclaw/util"
	"github.com/spf13/cobra"
)

const (
	DefaultBaseUrl = "http://127.0.0.1:18789"
	EnvBaseUrl     = "OPENCLAW_BASE_URL"
	EnvAuthToken   = "OPENCLAW_AUTH_TOKEN"
)

type cmdHandler func(ctx context.Context, args []string) error

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := runCLI(ctx, os.Args[1:])
	os.Exit(code)
}

func runCLI(ctx context.Context, argv []string) int {
	var showVersion bool

	rootCmd := &cobra.Command{
		Use:                "openclaw",
		Short:              "OpenClaw CLI",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				return printVersion(cmd.Context(), args)
			}
			if len(args) > 0 {
				return runPluginOrUnknown(cmd.Context(), args[0], args[1:])
			}
			return cmd.Help()
		},
	}

	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print version information")

	addSubCommand := func(use string, aliases []string, short string, handler cmdHandler) {
		sub := &cobra.Command{
			Use:                use,
			Aliases:            aliases,
			Short:              short,
			DisableFlagParsing: true,
			SilenceUsage:       true,
			SilenceErrors:      true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return handler(cmd.Context(), args)
			},
		}
		rootCmd.AddCommand(sub)
	}

	addSubCommand("start", nil, "Start service", startCmd)
	addSubCommand("run", nil, "Run task", runCmd)
	addSubCommand("chat", nil, "Start interactive chat", chatCmd)
	addSubCommand("live", nil, "Start live session", liveCmd)
	addSubCommand("tui", nil, "Launch terminal UI", tuiCmd)
	addSubCommand("insights", nil, "View insights", insightsCmd)
	addSubCommand("setup", nil, "Run setup", setupCmd)
	addSubCommand("update", nil, "Update components", updateCmd)
	addSubCommand("upgrade", nil, "Upgrade installation", upgradeCmd)
	addSubCommand("backup", nil, "Manage backups", backupCmd)
	addSubCommand("maintenance", nil, "Run maintenance tasks", maintenanceCmd)
	addSubCommand("payment", nil, "Manage payments", paymentCmd)
	addSubCommand("external", nil, "External CLI commands", externalCmd)
	addSubCommand("routing", nil, "Manage routing", routingCmd)
	addSubCommand("memory", nil, "Manage memory", memoryCmd)
	addSubCommand("test", nil, "Run tests", testCmd)
	addSubCommand("harness", nil, "Run test harness", harnessCmd)
	addSubCommand("regression", nil, "Run regression alias", regressionCmd)
	addSubCommand("init", nil, "Initialize workspace", initCmd)
	addSubCommand("migrate", nil, "Run migrations", migrateCmd)
	addSubCommand("pulse", nil, "Pulse diagnostics", pulseCmd)
	addSubCommand("heartbeat", nil, "Heartbeat status", heartbeatCmd)
	addSubCommand("models", nil, "Manage models", modelsCmd)
	addSubCommand("eval", nil, "Run evaluations", evalCmd)
	addSubCommand("accounts", nil, "Manage accounts", accountsCmd)
	addSubCommand("backends", nil, "Manage backends", backendsCmd)
	addSubCommand("admin", nil, "Administrative commands", adminCmd)
	addSubCommand("compatibility", []string{"compat"}, "Compatibility commands", compatibilityCmd)
	addSubCommand("plugins", nil, "Manage plugins", pluginsCmd)
	addSubCommand("skill", nil, "Skill kit commands", skillCmd)
	addSubCommand("skills", nil, "Manage skills", skillsCmd)
	addSubCommand("clawhub", nil, "ClawHub commands", clawHubCmd)
	addSubCommand("version", nil, "Print version information", printVersion)

	rootCmd.SetArgs(argv)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return 130
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	return 0
}

func startCmd(ctx context.Context, args []string) error {
	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		cli.StartCommandWriteHelp(os.Stdout)
		return nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	_, err = cli.StartCommandRun(args, os.Stdin, os.Stdout, os.Stderr, dir, !util.IsInputRedirected(), nil)
	return err

}

func resolveAuthToken(parsed *cli.CliArgs, errWrite io.Writer) string {
	cliToken := util.Deref(parsed.GetOption("--token"))
	if cliToken != "" {
		fmt.Fprintln(errWrite, "Warning: --token is deprecated because command-line arguments can be exposed in process listings. Prefer OPENCLAW_AUTH_TOKEN.")
		return cliToken
	}

	return os.Getenv(EnvAuthToken)
}

func ParseFloat(value *string) float64 {
	v := util.Deref(value)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}

	return f
}

func ParseInt(value *string) int64 {
	v := util.Deref(value)
	f, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}

	return f
}

func ReadAllStdin() *string {
	if !util.IsInputRedirected() {
		return nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil
	}

	text := string(data)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	trimmed := strings.TrimRightFunc(text, unicode.IsSpace)
	return &trimmed
}

func BuildUserContent(prompt string, files []string, images []string) string {
	if len(files) == 0 && len(images) == 0 {
		return prompt
	}

	parts := []string{prompt}
	for _, image := range images {
		parts = append(parts, BuildImageMarker(image))
	}

	for _, path := range files {
		fullPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		if IsImagePath(fullPath) {
			parts = append(parts, BuildImageMarker(fullPath))
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return prompt
		}

		parts = append(parts, fmt.Sprintf("\n\n--- file: %s ---\n```\n%s\n```\n", fullPath, string(data)))
	}

	return strings.Join(parts, "\n")
}

func IsImagePath(fullPath string) bool {
	extension := filepath.Ext(fullPath)
	return extension == ".png" || extension == ".jpg" || extension == ".jpeg" || extension == ".webp" || extension == ".gif"
}

func BuildImageMarker(image string) string {
	if u, err := url.Parse(image); err == nil && u.IsAbs() {
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "data":
			return fmt.Sprintf("[IMAGE_URL:%s]", image)
		case "file":
			localPath := filepath.FromSlash(u.Path)
			if u.Host != "" && u.Host != "localhost" {
				localPath = filepath.FromSlash("//" + u.Host + u.Path)
			} else if len(localPath) >= 3 && localPath[0] == '\\' && localPath[2] == ':' {
				localPath = localPath[1:]
			}
			absPath, err := filepath.Abs(localPath)
			if err != nil {
				absPath = localPath
			}
			return fmt.Sprintf("[IMAGE_PATH:%s]", absPath)
		}
	}

	absPath, err := filepath.Abs(image)
	if err != nil {
		absPath = image
	}
	return fmt.Sprintf("[IMAGE_PATH:%s]", absPath)
}

func BuildMessages(system, userContent string, priorConversation []core.OpenAiMessage) []core.OpenAiMessage {
	if len(priorConversation) == 0 {
		initial := []core.OpenAiMessage{}
		if system != "" {
			initial = append(initial, core.OpenAiMessage{Role: "system", Content: &core.OpenAiMessageContent{Text: system}})
		}
		initial = append(initial, core.OpenAiMessage{Role: "user", Content: &core.OpenAiMessageContent{Text: userContent}})
		return initial
	}

	messages := []core.OpenAiMessage{}
	messages = append(messages, priorConversation...)
	return messages
}

func runCmd(ctx context.Context, args []string) error {
	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		printHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)
	model := util.Deref(parsed.GetOption("--model"))
	system := util.Deref(parsed.GetOption("--system"))
	preset := util.Deref(parsed.GetOption("--preset"))

	stream := !parsed.HasFlag("--no-stream")
	temperature := ParseFloat(parsed.GetOption("--temperature"))
	maxTokens := ParseInt(parsed.GetOption("--max-tokens"))

	prompt := ""
	if len(parsed.Positionals) > 0 {
		prompt = strings.Join(parsed.Positionals, " ")
	}

	stdin := util.Deref(ReadAllStdin())
	if prompt == "" {
		prompt = stdin
	} else if stdin != "" {
		prompt = fmt.Sprintf("%s\n\n--- stdin ---\n%s", prompt, stdin)
	}

	if prompt == "" {
		msg := "Missing prompt. Provide <prompt> or pipe stdin."
		fmt.Fprintln(os.Stderr, msg)
		return errors.New(msg)
	}

	userContent := BuildUserContent(prompt, parsed.Files, parsed.Images)
	messages := BuildMessages(system, userContent, nil)

	httpClient, err := client.NewOpenClawHttpClient(baseUrl, token, nil)
	if err != nil {
		return err
	}

	request := core.OpenAiChatCompletionRequest{
		Model:       model,
		Stream:      stream,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Messages:    messages,
	}

	if stream {
		full, err := httpClient.StreamChatCompletion(ctx, request, func(s string) { fmt.Println(s) }, new(preset))
		if err != nil {
			return err
		}
		if full != "" && !strings.HasSuffix(full, "\n") {
			fmt.Println("")
		}

		return nil
	}

	response, err := httpClient.ChatCompletion(ctx, request, new(preset))
	if err != nil {
		return err
	}

	var text = response.Choices[0].Message.Content
	fmt.Println(text)
	return nil
}

func chatCmd(ctx context.Context, args []string) error {
	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		printHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)
	model := util.Deref(parsed.GetOption("--model"))
	system := util.Deref(parsed.GetOption("--system"))
	preset := util.Deref(parsed.GetOption("--preset"))

	temperature := ParseFloat(parsed.GetOption("--temperature"))
	maxTokens := ParseInt(parsed.GetOption("--max-tokens"))

	httpClient, err := client.NewOpenClawHttpClient(baseUrl, token, nil)
	if err != nil {
		return err
	}

	conversation := []core.OpenAiMessage{}
	if system != "" {
		if system != "" {
			conversation = append(conversation, core.OpenAiMessage{Role: "system", Content: &core.OpenAiMessageContent{Text: system}})
		}
	}

	fmt.Fprintf(os.Stdout, "openclaw chat — type /help for commands")
	scanner := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		fmt.Print("> ")
		line := scanner.Text()
		if line == "" {
			break
		}

		userContent := line
		if strings.HasPrefix(line, "/image ") {
			userContent = BuildImageCommandContent(line)
		}

		if userContent == "" {
			continue
		}

		if strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "/image ") {
			if HandleSlashCommand(line, &conversation, &model, &system) {
				break
			}
			continue
		}

		conversation = append(conversation, core.OpenAiMessage{Role: "user", Content: &core.OpenAiMessageContent{Text: userContent}})
		messages := BuildMessages(system, userContent, conversation)

		request := core.OpenAiChatCompletionRequest{
			Model:       model,
			Stream:      true,
			Temperature: temperature,
			MaxTokens:   maxTokens,
			Messages:    messages,
		}

		assistantText, err := httpClient.StreamChatCompletion(context.Background(), request, func(s string) {
			fmt.Println(s)
		}, new(preset))
		if err != nil {
			return err
		}
		if !strings.HasSuffix(assistantText, "\n") {
			fmt.Println("")
		}
		conversation = append(conversation, core.OpenAiMessage{Role: "assistant", Content: &core.OpenAiMessageContent{Text: assistantText}})
	}

	return scanner.Err()

}

func HandleSlashCommand(command string, conversation *[]core.OpenAiMessage, model, system *string) bool {
	if command == "/exit" || command == "/quit" {
		return true
	}

	if command == "/help" {
		fmt.Fprintln(os.Stdout, `Commands:
                  /help
                  /exit
                  /reset
                  /system <text>
                  /model <model>`)
		return false
	}

	if command == "/reset" {
		conversation = new([]core.OpenAiMessage{})
		if util.Deref(system) != "" {
			*conversation = append(*conversation, core.OpenAiMessage{Role: "system", Content: &core.OpenAiMessageContent{Text: *system}})
		}

		fmt.Fprintln(os.Stdout, "Reset conversation.")
		return false
	}

	if strings.HasPrefix(command, "/system ") {
		system = new(command[len("/system "):])
		c := []core.OpenAiMessage{}
		for _, m := range *conversation {
			if m.Role != "system" {
				c = append(c, m)
			}
		}

		if util.Deref(system) != "" {
			c = append(c, core.OpenAiMessage{Role: "system", Content: &core.OpenAiMessageContent{Text: *system}})
		}

		conversation = &c
		fmt.Fprintln(os.Stdout, "Set system prompt.")
		return false
	}

	if strings.HasPrefix(command, "/model ") {
		model = new(command[len("/model "):])
		if *model == "" {
			fmt.Fprintln(os.Stdout, "Cleared model override.")
		} else {
			fmt.Fprintf(os.Stdout, "Set model: %s\n", *model)
		}

		return false
	}

	fmt.Fprintf(os.Stdout, "Unknown command: %s\n", command)
	return false
}

func BuildImageCommandContent(command string) string {
	tail := command[len("/image "):]
	if len(tail) == 0 {
		fmt.Fprintln(os.Stdout, "Usage: /image <path|url> [prompt]")
		return ""
	}

	before, after, ok := strings.Cut(tail, " ")
	image := tail
	if ok {
		image = before
	}

	prompt := "Describe this image."
	if ok {
		image = after
	}

	if prompt == "" {
		prompt = "Describe this image."
	}

	return BuildUserContent(prompt, nil, []string{image})
}

func printLiveHelp() {
	fmt.Println(` openclaw live

            Usage:
              openclaw live [--url <url>] [--token <token>] [--provider <id>] [--model <id>] [--system <text>] [--voice <name>] [--modality <TEXT|AUDIO>]...

            Notes:
              - Opens a Gemini Live websocket session through the gateway.
              - Interactive commands: /interrupt, /audio-file <path> [mime], /exit`)
}

func liveCmd(ctx context.Context, args []string) error {
	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		printLiveHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)
	provider := util.Deref(parsed.GetOption("--provider"))
	model := util.Deref(parsed.GetOption("--model"))
	system := util.Deref(parsed.GetOption("--system"))
	voice := util.Deref(parsed.GetOption("--voice"))

	modalities, ok := parsed.Options["--modality"]
	if !ok {
		modalities = []string{"TEXT"}
	}

	return RunLiveConsole(baseUrl, token, provider, model, system, voice, modalities)

}

func RunLiveConsole(baseUrl, token, provider, model, system, voice string, modalities []string) error {
	httpclient := client.NewOpenClawLiveClient(0)
	httpclient.OnEnvelopeReceived = func(envelope *core.LiveServerEnvelope) {
		switch envelope.Type {
		case "opened":
			fmt.Fprintf(os.Stdout, "[opened] %s\n", envelope.Text)
		case "text":
			fmt.Print(envelope.Text)
		case "turn_complete":
			fmt.Println("")
		case "audio":
			fmt.Fprintf(os.Stdout, "[audio] mime=%s bytes=%d\n", envelope.MimeType, len(envelope.Base64Data))
		case "input_transcription":
			fmt.Fprintf(os.Stdout, "[input] %s\n", envelope.Text)
		case "output_transcription":
			fmt.Fprintf(os.Stdout, "[output] %s\n", envelope.Text)
		case "interrupted":
			fmt.Fprintln(os.Stdout, "[interrupted]")
		case "error":
			fmt.Fprintf(os.Stdout, "[error] %s\n", envelope.Error)
		}
	}

	httpclient.OnError = func(err error) {
		fmt.Fprintf(os.Stdout, "[client-error] %s\n", err.Error())
	}

	ctx := context.Background()
	url, err := client.BuildWebSocketURI(baseUrl)
	if err != nil {
		return err
	}
	httpclient.Connect(
		ctx,
		url,
		token,
		core.LiveSessionOpenRequest{
			Provider:           provider,
			Model:              model,
			SystemInstruction:  system,
			VoiceName:          voice,
			ResponseModalities: modalities,
		})

	fmt.Fprintln(os.Stdout, "openclaw live — commands: /interrupt, /audio-file <path> [mime], /exit")

	scanner := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		fmt.Println("live> ")
		line := scanner.Text()
		if line == "" {
			break
		}

		if line == "/exit" {
			break
		}

		if line == "/interrupt" {
			if err := httpclient.Interrupt(context.Background()); err != nil {
				return nil
			}
			continue
		}

		if strings.HasPrefix(line, "/audio-file ") {
			tail := line[len("/audio-file "):]
			if len(tail) == 0 {
				fmt.Fprintln(os.Stdout, "Usage: /audio-file <path> [mime]")
				continue
			}

			parts := strings.Split(tail, " ")
			path, err := filepath.Abs(parts[0])
			if err != nil {
				return err
			}
			if !util.FileExists(path) {
				fmt.Fprintf(os.Stdout, "File not found: %s\n", path)
				continue
			}

			mime := "audio/pcm"
			if len(parts) > 1 {
				mime = parts[1]
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			base64 := base64.StdEncoding.EncodeToString(data)

			if err := httpclient.SendAudio(context.Background(), base64, mime, true); err != nil {
				return err
			}
			continue
		}

		if err := httpclient.SendText(context.Background(), line, true); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	return httpclient.CloseSession(context.Background())
}

func tuiCmd(ctx context.Context, args []string) error {
	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		printTuiHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)
	preset := util.Deref(parsed.GetOption("--preset"))

	if err := tui.Run(context.Background(), baseUrl, token, preset); err != nil {
		fmt.Fprintln(os.Stderr, "The TUI is not available in this build. Use the non-AOT build or the admin web UI.")
		return err
	}

	return nil
}

func printTuiHelp() {
	fmt.Println(`openclaw tui

            Usage:
              openclaw tui [--url <url>] [--token <token>]

            Notes:
              - Launches the Spectre.Console terminal UI for runtime status, sessions, search,
                automations, profiles, learning proposals, approvals, direct chat, and live sessions.`)
}

func insightsCmd(ctx context.Context, args []string) error {
	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		printInsightsHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)
	fromUtc := ParseDateTimeOffset(parsed.GetOption("--from"))
	toUtc := ParseDateTimeOffset(parsed.GetOption("--to"))

	httpClient, err := client.NewOpenClawHttpClient(baseUrl, token, nil)
	if err != nil {
		return err
	}

	insights, err := httpClient.GetOperatorInsights(context.Background(), fromUtc, toUtc)
	if err != nil {
		return err
	}
	if parsed.HasFlag("--json") {
		data, _ := json.Marshal(insights)
		fmt.Println(string(data))
		return nil
	}

	WriteInsights(insights)
	return nil
}

func ParseDateTimeOffset(value *string) *time.Time {
	if value == nil {
		return nil
	}

	d, err := time.Parse("20060102150405", *value)
	if err != nil {
		return nil
	}

	return &d
}

func printInsightsHelp() {
	fmt.Println(`
	openclaw insights

            Usage:
              openclaw insights [--from <iso8601>] [--to <iso8601>] [--json] [--url <url>] [--token <token>]

            Notes:
              - Summarizes provider usage, estimated token spend, tool frequency, and session counts.
              - Provider and tool usage are live runtime counters; session counts use the requested range.`)
}

func formatCost(val float64) string {
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func WriteInsights(insights *core.OperatorInsightsResponse) {
	fmt.Printf("window: %s..%s\n", insights.StartUtc.UTC().Format(time.RFC3339Nano), insights.EndUtc.UTC().Format(time.RFC3339Nano))

	fmt.Printf("sessions: active=%d persisted=%d total=%d range=%d 24h=%d 7d=%d\n",
		insights.Sessions.Active,
		insights.Sessions.Persisted,
		insights.Sessions.UniqueTotal,
		insights.Sessions.InRange,
		insights.Sessions.Last24Hours,
		insights.Sessions.Last7Days,
	)

	fmt.Printf("provider_usage: requests=%d errors=%d tokens=%d input=%d output=%d cache_read=%d cache_write=%d estimated_cost_usd=%s\n",
		insights.Totals.ProviderRequests,
		insights.Totals.ProviderErrors,
		insights.Totals.TotalTokens(),
		insights.Totals.InputTokens,
		insights.Totals.OutputTokens,
		insights.Totals.CacheReadTokens,
		insights.Totals.CacheWriteTokens,
		formatCost(insights.Totals.EstimatedCostUsd),
	)

	fmt.Printf("tool_calls: %d\n", insights.Totals.ToolCalls)

	fmt.Println()
	fmt.Println("Providers")
	if len(insights.Providers) == 0 {
		fmt.Println("- none")
	} else {
		limit := min(len(insights.Providers), 10)
		for _, provider := range insights.Providers[:limit] {
			fmt.Printf("- %s/%s: requests=%d tokens=%d input=%d output=%d retries=%d errors=%d estimated_cost_usd=%s\n",
				provider.ProviderId,
				provider.ModelId,
				provider.Requests,
				provider.TotalTokens(),
				provider.InputTokens,
				provider.OutputTokens,
				provider.Retries,
				provider.Errors,
				formatCost(provider.EstimatedCostUsd),
			)
		}
	}

	fmt.Println()
	fmt.Println("Tools")
	if len(insights.Tools) == 0 {
		fmt.Println("- none")
	} else {
		limit := min(len(insights.Tools), 10)
		for _, tool := range insights.Tools[:limit] {
			fmt.Printf("- %s: calls=%d failures=%d timeouts=%d avg_ms=%.1f\n",
				tool.ToolName,
				tool.Calls,
				tool.Failures,
				tool.Timeouts,
				tool.AverageDurationMs,
			)
		}
	}

	if len(insights.Sessions.ByChannel) > 0 {
		channels := make([]string, len(insights.Sessions.ByChannel))
		for i, item := range insights.Sessions.ByChannel {
			channels[i] = fmt.Sprintf("%s=%d", item.Label, item.Count)
		}
		fmt.Printf("session_channels: %s\n", strings.Join(channels, ", "))
	}

	if len(insights.Sessions.ByState) > 0 {
		states := make([]string, len(insights.Sessions.ByState))
		for i, item := range insights.Sessions.ByState {
			states[i] = fmt.Sprintf("%s=%d", item.Label, item.Count)
		}
		fmt.Printf("session_states: %s\n", strings.Join(states, ", "))
	}

	for _, warning := range insights.Warnings {
		fmt.Printf("note: %s\n", warning)
	}
}

func setupCmd(ctx context.Context, args []string) error {
	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		printSetupHelp()
		return nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	_, err = cli.SetupCommandRun(args, os.Stdin, os.Stdout, os.Stderr, dir, util.IsInputRedirected())
	if err != nil {
		return err
	}

	return nil
}

func printSetupHelp() {
	fmt.Println(`
            openclaw setup

            Usage:
              openclaw setup [--profile <local|public|tailscale-serve>] [--non-interactive]
                              [--config <path>] [--workspace <path>] [--provider <id>] [--model <id>] [--model-preset <id>] [--api-key <secret-or-envref>]
                              [--bind <address>] [--port <n>] [--auth-token <token>]
                              [--docker-image <image>] [--opensandbox-endpoint <url>] [--ssh-host <host>] [--ssh-user <user>] [--ssh-key <path>]
              openclaw setup launch [--config <path>] [--with-companion] [--open-browser] [--skip-verify] [--offline] [--require-provider]
              openclaw setup service [--config <path>] [--platform <linux|macos|all>]
              openclaw setup status [--config <path>]
              openclaw setup verify [--config <path>] [--offline] [--require-provider] [--json]
              openclaw setup channel <telegram|slack|discord|teams|whatsapp> [--config <path>] [--non-interactive] [...]
              openclaw setup tailscale serve [--config <path>] [--local-url <url>] [--non-interactive]

            Notes:
              - Prefer 'openclaw start' for the one-command local path.
              - Bare 'openclaw setup' launches a guided onboarding flow.
              - 'openclaw setup launch' starts the gateway in the current repo checkout, runs verification, and streams logs until Ctrl-C.
              - Use --with-companion to start Companion too.
              - If you start the gateway directly and hit local startup friction, use: dotnet run --project src/OpenClaw.Gateway -c Release -- --quickstart
              - 'openclaw setup service' writes systemd/launchd/Caddy deployment artifacts next to the config.
              - 'openclaw setup status' summarizes bind/auth posture and deploy artifact presence.
              - 'openclaw setup verify' runs the first-run verification checks without launching the gateway.
              - 'openclaw setup channel ...' updates an existing external config with channel-specific settings.
              - 'openclaw setup tailscale serve' prints private tailnet Serve instructions without enabling public bind or changing providers.
              - Use --non-interactive for automation or CI.
              - Writes an external JSON config file plus an adjacent env example.
              - Prints gateway, companion, doctor, and admin posture commands.`)
}

func printUpgradeHelp() {
	fmt.Println(`
            openclaw upgrade

            Usage:
              openclaw upgrade check [--config <path>] [--offline]
              openclaw upgrade rollback [--config <path>] [--offline] [--require-provider]

            Notes:
              - Runs preflight checks before an upgrade.
              - Combines setup verification, provider readiness, plugin compatibility,
                skill compatibility, and migration-risk heuristics into one report.
              - Captures a last-known-good config/env/deploy snapshot when preflight succeeds.
              - 'rollback' restores the saved snapshot and reruns setup verification.
              - Returns a non-zero exit code when blocking issues are found.`)
}

func updateCmd(ctx context.Context, args []string) error {
	_, err := cli.UpdateCommandRun(args)
	if err != nil {
		return err
	}

	return nil
}

func upgradeCmd(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printUpgradeHelp()
		return nil
	}

	_, err := cli.UpgradeCommandRun(args, os.Stdout, os.Stderr)
	return err
}

func backupCmd(ctx context.Context, args []string) error {
	_, err := cli.BackupCommandsRun(args)
	return err
}

func maintenanceCmd(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printMaintenanceHelp()
		return nil
	}

	_, err := cli.MaintenanceCommandsRun(args, os.Stdout, os.Stderr)
	return err
}

func printMaintenanceHelp() {
	fmt.Println(`
            openclaw maintenance

            Usage:
              openclaw maintenance scan [--config <path>] [--json]
              openclaw maintenance fix [--config <path>] [--dry-run] [--json] [--apply <all|retention|metadata|artifacts>]`)
}

func paymentCmd(ctx context.Context, args []string) error {
	_, err := cli.PaymentCommandRun(args)
	return err
}

func externalCmd(ctx context.Context, args []string) error {
	_, err := cli.ExternalCliCommandRun(ctx, args)
	return err
}

func routingCmd(ctx context.Context, args []string) error {
	_, err := cli.RoutingCommandRun(args)
	return err
}

func memoryCmd(ctx context.Context, args []string) error {
	_, err := cli.MemoryCommandRun(ctx, args)
	return err
}

func testCmd(ctx context.Context, args []string) error {
	_, err := cli.TestingCommandRun(args, os.Stdout, os.Stderr)
	return err
}

func harnessCmd(ctx context.Context, args []string) error {
	_, err := cli.HarnessCommandRun(args)
	return err
}

func regressionCmd(ctx context.Context, args []string) error {
	_, err := cli.HarnessCommandRunRegressionAlias(args, os.Stdout, os.Stderr)
	return err
}

func initCmd(ctx context.Context, args []string) error {
	_, err := cli.InitCommandRun(args)
	return err
}

func migrateCmd(ctx context.Context, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	if len(args) > 0 && args[0] == "upstream" {
		_, err := cli.UpstreamMigrationCommandRun(args[1:], os.Stdout, os.Stderr, dir)
		return err
	}
	if len(args) > 0 && args[0] == "legacy" {
		args = args[1:]
	}

	parsed := cli.CliArgsParse(args)
	if parsed.ShowHelp {
		printMigrateHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)
	apply := parsed.HasFlag("--apply")

	httpclient, err := client.NewOpenClawHttpClient(baseUrl, token, nil)
	if err != nil {
		return err
	}

	migrated, err := httpclient.MigrateAutomations(ctx, apply)
	if err != nil {
		return err
	}

	if apply {
		fmt.Println("Applied legacy automation migration.")
	} else {
		fmt.Println("Previewed legacy automation migration.")
	}

	fmt.Printf("Automations: %d\n", len(migrated.Items))
	for _, item := range migrated.Items {
		fmt.Printf("- %s | %s | %s | enabled=%t draft=%t\n",
			item.Id, item.Name, item.Schedule, item.Enabled, item.IsDraft)
	}

	return nil
}

func printMigrateHelp() {
	fmt.Println(`
            openclaw migrate

            Usage:
              openclaw migrate [--apply] [--url <url>] [--token <token>]
              openclaw migrate legacy [--apply] [--url <url>] [--token <token>]
              openclaw migrate upstream --source <path> --target-config <path> [--apply] [--report <path>]

            Notes:
              - Bare 'openclaw migrate' remains the legacy automation migration alias.
              - 'openclaw migrate upstream' performs upstream config + skill + plugin-manifest migration.
              - Upstream dry-run is the default; apply mode writes translated config, managed skills, and a plugin review plan.`)
}

func pulseCmd(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printPulseHelp()
		return nil
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	parsed := cli.CliArgsParse(args[1:])
	if parsed.ShowHelp {
		printPulseHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)

	httpclient, err := client.NewOpenClawHttpClient(baseUrl, token, nil)
	if err != nil {
		return err
	}

	switch subcommand {
	case "status":
		return pulseStatus(ctx, httpclient)
	case "run":
		return pulseRun(ctx, httpclient, parsed)
	case "enable":
		return pulseEnable(ctx, httpclient)
	case "disable":
		return pulseDisable(ctx, httpclient)
	case "events":
		return pulseEvents(ctx, httpclient, parsed)
	case "doctor":
		return pulseDoctor(ctx, httpclient)
	default:
		return fmt.Errorf("Unknown pulse command: %s", subcommand)
	}
}

func printPulseHelp() {
	fmt.Println(`
            openclaw pulse

            Usage:
              openclaw pulse status [--url <url>] [--token <token>]
              openclaw pulse run [--text <text>] [--mode now|next-heartbeat] [--url <url>] [--token <token>]
              openclaw pulse enable [--url <url>] [--token <token>]
              openclaw pulse disable [--url <url>] [--token <token>]
              openclaw pulse events [--limit <n>] [--url <url>] [--token <token>]
              openclaw pulse doctor [--url <url>] [--token <token>]

            Notes:
              - Runtime Pulse is a scheduled heartbeat turn, not cron automation.
              - OK replies are suppressed by default; alerts remain operator-visible.`)
}

func pulseDoctor(ctx context.Context, httpclient *client.OpenClawHttpClient) error {
	status, err := httpclient.GetPulseStatus(ctx)
	if err != nil {
		return err
	}

	writePulseStatus(status)
	fmt.Println("")
	fmt.Println("doctor:")
	if !status.Enabled {
		fmt.Println("- pulse is disabled or has a zero interval")
	}

	if !status.Config.Visibility.ShowOk && !status.Config.Visibility.ShowAlerts && !status.Config.Visibility.UseIndicator {
		fmt.Println("- all visibility controls are disabled; scheduled pulse calls are skipped")
	}

	if status.HeartbeatExists && status.HeartbeatEmpty {
		fmt.Println("- HEARTBEAT.md exists but has no actionable content")
	}
	if strings.EqualFold(status.Config.Target, "last") {
		fmt.Println("- target=last may send alerts externally")
	}
	if status.Config.IncludeReasoning {
		fmt.Println("- includeReasoning may expose more detail in human-facing channels")
	}
	return nil
}

func pulseEvents(ctx context.Context, httpclient *client.OpenClawHttpClient, parsed *cli.CliArgs) error {
	limit, _ := strconv.ParseInt(util.Deref(parsed.GetOption("--limit")), 10, 64)
	if limit <= 0 {
		limit = 50
	}

	events, err := httpclient.GetPulseEvents(ctx, int(limit))
	if err != nil {
		return err
	}

	for _, item := range events.Items {
		fmt.Printf("%s %s %s %s", item.TimestampUtc.Format("20060102150405"), item.Severity, item.Action, item.Summary)
	}
	return nil
}

func pulseDisable(ctx context.Context, httpclient *client.OpenClawHttpClient) error {
	status, err := httpclient.DisablePulse(ctx)
	writePulseStatus(status)
	return err
}

func pulseEnable(ctx context.Context, httpclient *client.OpenClawHttpClient) error {
	status, err := httpclient.EnablePulse(ctx)
	writePulseStatus(status)
	return err
}

func pulseRun(ctx context.Context, httpclient *client.OpenClawHttpClient, parsed *cli.CliArgs) error {
	text := util.Deref(parsed.GetOption("--text"))
	mode := util.Deref(parsed.GetOption("--mode"))
	if mode == "" {
		mode = "now"
	}
	result, err := httpclient.RunPulse(ctx, core.PulseRunRequest{
		Text: text,
		Mode: mode,
	})
	if err != nil {
		return err
	}

	fmt.Printf("outcome: %s\n", result.Outcome)
	if result.SkipReason != "" {
		fmt.Printf("skip_reason: %s\n", result.SkipReason)
	}
	if result.SessionId != "" {
		fmt.Printf("session: %s\n", result.SessionId)
	}
	if result.MessagePreview != "" {
		fmt.Println(result.MessagePreview)
	}

	return nil
}

func pulseStatus(ctx context.Context, httpclient *client.OpenClawHttpClient) error {
	status, err := httpclient.GetPulseStatus(ctx)
	writePulseStatus(status)
	return err
}

func writePulseStatus(status *core.PulseStatusResponse) {
	if status == nil {
		return
	}

	fmt.Println("Runtime Pulse")
	fmt.Printf("Enabled: %t\n", status.Enabled)
	fmt.Printf("Interval: %v\n", status.Interval)
	fmt.Printf("HEARTBEAT path: %s\n", status.HeartbeatPath)
	fmt.Printf("HEARTBEAT exists: %t\n", status.HeartbeatExists)
	fmt.Printf("HEARTBEAT empty: %t\n", status.HeartbeatEmpty)
	fmt.Printf("Last result: %s\n", status.LastResult)

	if strings.TrimSpace(status.LastSkipReason) != "" {
		fmt.Printf("Last skip reason: %s\n", status.LastSkipReason)
	}
	if status.LastRunAtUtc != nil {
		fmt.Printf("Last run: %s\n", status.LastRunAtUtc.UTC().Format(time.RFC3339Nano))
	}
	if status.NextRunAtUtc != nil {
		fmt.Printf("Next run: %s\n", status.NextRunAtUtc.UTC().Format(time.RFC3339Nano))
	}

	fmt.Printf("Target: %s\n", status.Config.Target)
	fmt.Printf("Session: %s\n", status.Config.Session)
	fmt.Printf("Light context: %t\n", status.Config.LightContext)
	fmt.Printf("Isolated session: %t\n", status.Config.IsolatedSession)
	fmt.Printf("Recent OKs: %d\n", status.RecentOkCount)
	fmt.Printf("Recent alerts: %d\n", status.RecentAlertCount)

	limit := min(len(status.RecentAlerts), 5)
	for _, alert := range status.RecentAlerts[:limit] {
		fmt.Printf("- %s [%s] %s\n", alert.TimestampUtc.UTC().Format(time.RFC3339Nano), alert.Severity, alert.Text)
	}
}

func heartbeatCmd(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printHeartbeatHelp()
		return nil
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	parsed := cli.CliArgsParse(args[1:])
	if parsed.ShowHelp {
		printHeartbeatHelp()
		return nil
	}

	baseUrl := util.Deref(parsed.GetOption("--url"))
	if baseUrl == "" {
		baseUrl = os.Getenv(EnvBaseUrl)
	}
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	token := resolveAuthToken(parsed, os.Stderr)

	httpclient, err := client.NewOpenClawHttpClient(baseUrl, token, nil)
	if err != nil {
		return err
	}

	switch subcommand {
	case "status":
		return heartbeatStatus(ctx, httpclient)
	case "preview":
		return heartbeatPreview(ctx, httpclient)
	case "wizard":
		return heartbeatWizard(ctx, httpclient)
	case "run":
		return pulseRun(ctx, httpclient, parsed)
	default:
		return fmt.Errorf("Unknown heartbeat command: %s", subcommand)
	}
}

func heartbeatWizard(ctx context.Context, httpclient *client.OpenClawHttpClient) error {
	current, err := httpclient.GetHeartbeat(ctx)
	if err != nil {
		return err
	}

	status := "disabled"
	if current.Config.Enabled {
		status = "enabled"
	}

	fmt.Printf("Config path: %s\n", current.ConfigPath)
	fmt.Printf("HEARTBEAT path: %s\n", current.HeartbeatPath)
	fmt.Printf("Current status: %s\n", status)
	fmt.Println()

	config := core.HeartbeatConfigDto{
		Enabled:             promptBool("Enable heartbeat", current.Config.Enabled),
		CronExpression:      prompt("Cron expression", defaultString(current.Config.CronExpression, "@hourly")),
		Timezone:            prompt("Timezone", defaultString(current.Config.Timezone, "UTC")),
		DeliveryChannelId:   strings.ToLower(prompt("Delivery channel", defaultString(current.Config.DeliveryChannelId, "cron"))),
		DeliveryRecipientId: promptOptional("Delivery recipient", current.Config.DeliveryRecipientId),
		DeliverySubject:     promptOptional("Delivery subject", current.Config.DeliverySubject),
		ModelId:             promptOptional("Model override", current.Config.ModelId),
		Tasks:               promptTasks(current),
	}

	preview, err := httpclient.PreviewHeartbeat(ctx, config)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Preview")
	fmt.Println("-------")
	writeHeartbeatPreview(preview)

	hasErrors := false
	for _, issue := range preview.Issues {
		if strings.EqualFold(issue.Severity, "error") {
			hasErrors = true
			break
		}
	}
	if hasErrors {
		msg := "Heartbeat config has validation errors and was not saved."
		fmt.Println(msg)
		return errors.New(msg)
	}

	if !promptBool("Save this heartbeat config", true) {
		fmt.Println("Heartbeat config not saved.")
		return nil
	}

	saved, err := httpclient.SaveHeartbeat(ctx, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save heartbeat config: %v\n", err)
		return err
	}

	fmt.Println("Heartbeat config saved.")
	writeHeartbeatPreview(saved)
	return nil
}

func promptTasks(current *core.HeartbeatPreviewResponse) []core.HeartbeatTaskDto {
	if current == nil {
		return nil
	}

	templates := []core.HeartbeatTemplateDto{}
	for _, template := range current.AvailableTemplates {
		if template.Available {
			templates = append(templates, template)
		}
	}

	if len(templates) == 0 {
		fmt.Println("No heartbeat templates are currently available from the gateway.")
		return current.Config.Tasks
	}

	fmt.Println("Available templates:")
	for i := range templates {
		fmt.Printf("  %d. %s (%s)\n", i+1, templates[i].Label, templates[i].Key)
	}

	if len(current.Suggestions) > 0 {
		fmt.Println("")
		fmt.Println("Suggestions:")
		for i := range current.Suggestions {
			fmt.Printf("  - %s [%s] — %s\n", current.Suggestions[i].Title, current.Suggestions[i].TemplateKey, current.Suggestions[i].Reason)
		}
	}

	selected := prompt("Template numbers (comma separated, blank keeps current tasks)", "")
	if selected == "" {
		return current.Config.Tasks
	}

	tasks := []core.HeartbeatTaskDto{}
	picks := strings.Split(selected, ",")
	for _, pick := range picks {
		index, _ := strconv.ParseInt(pick, 10, 64)
		if index < 1 || index > int64(len(templates)) {
			continue
		}

		template := templates[index-1]
		defaultTitle := template.Label
		title := prompt("Task title", defaultTitle)
		target := promptOptional("Task target", "")
		instruction := promptOptional("Task instruction", "")
		priority := strings.ToLower(prompt("Priority (low|normal|high)", "normal"))
		conditionMode := strings.ToLower(prompt("Condition mode (and|or)", "and"))
		conditions := promptConditions()

		tasks = append(tasks, core.HeartbeatTaskDto{
			Id:            fmt.Sprintf("task-%s", util.CleanUUID())[:12],
			TemplateKey:   template.Key,
			Title:         title,
			Target:        target,
			Instruction:   instruction,
			Priority:      priority,
			Enabled:       true,
			ConditionMode: conditionMode,
			Conditions:    conditions,
		})

	}

	return tasks
}

func promptConditions() []core.HeartbeatConditionDto {
	conditions := []core.HeartbeatConditionDto{}

	for {
		if !promptBool("Add a condition", false) {
			break
		}

		field := prompt("Condition field", "subject")
		op := prompt("Condition operator (contains|equals|any_of|is_true)", "contains")

		values := []string{}
		if !strings.EqualFold(op, "is_true") {
			values = strings.Split(prompt("Condition values (comma separated)", ""), ",")
		}
		conditions = append(conditions, core.HeartbeatConditionDto{
			Field:    field,
			Operator: op,
			Values:   values,
		})
	}

	return conditions
}

func promptOptional(label, defaultValue string) string {
	suffix := ""
	if defaultValue != "" {
		suffix = fmt.Sprintf(" [%s]", defaultValue)
	}

	fmt.Printf("%s%s: ", label, suffix)
	value := strings.TrimSpace(util.ReadConsoleLine())
	if value == "" {
		return defaultValue
	}

	return value
}

func defaultString(value, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}

	return value
}

func prompt(label, defaultValue string) string {
	fmt.Printf("%s [%s]: ", label, defaultValue)
	value := strings.TrimSpace(util.ReadConsoleLine())
	if value == "" {
		return defaultValue
	}

	return value
}

func promptBool(label string, defaultValue bool) bool {
	if defaultValue {
		fmt.Printf("%s [Y/n]: ", label)
	} else {
		fmt.Printf("%s [y/N]: ", label)
	}

	value := strings.ToLower(strings.TrimSpace(util.ReadConsoleLine()))
	if value == "" {
		return defaultValue
	}

	return value == "y" || value == "yes" || value == "true"
}

func heartbeatPreview(ctx context.Context, httpclient *client.OpenClawHttpClient) error {
	preview, err := httpclient.GetHeartbeat(ctx)
	writeHeartbeatPreview(preview)
	return err
}

func writeHeartbeatPreview(preview *core.HeartbeatPreviewResponse) {
	if preview == nil {
		return
	}

	fmt.Printf("Config path: %s\n", preview.ConfigPath)
	fmt.Printf("HEARTBEAT path: %s\n", preview.HeartbeatPath)
	fmt.Printf("memory.md path: %s\n", preview.MemoryMarkdownPath)
	fmt.Printf("Managed job active: %t\n", preview.ManagedJobActive)
	fmt.Printf("Drift detected: %t\n", preview.DriftDetected)
	fmt.Printf("Model: %s:%s\n", preview.CostEstimate.ProviderId, preview.CostEstimate.ModelId)
	fmt.Printf("Estimated input tokens/run: %d\n", preview.CostEstimate.EstimatedInputTokensPerRun)
	fmt.Printf("Estimated monthly runs: %d\n", preview.CostEstimate.EstimatedRunsPerMonth)
	fmt.Printf("Estimated OK cost/month: $%.4f\n", preview.CostEstimate.EstimatedOkCostUsdPerMonth)
	fmt.Printf("Estimated alert cost/month: $%.4f\n", preview.CostEstimate.EstimatedAlertCostUsdPerMonth)
	fmt.Println()

	if len(preview.Issues) > 0 {
		fmt.Println("Issues:")
		for _, issue := range preview.Issues {
			fmt.Printf("- %s: %s\n", issue.Severity, issue.Message)
		}
		fmt.Println()
	}

	fmt.Println(preview.HeartbeatMarkdown)
}

func heartbeatStatus(ctx context.Context, httpclient *client.OpenClawHttpClient) error {
	status, err := httpclient.GetHeartbeatStatus(ctx)
	writeHeartbeatStatus(status)
	return err
}

func writeHeartbeatStatus(status *core.HeartbeatStatusResponse) {
	if status == nil {
		return
	}
	fmt.Printf("Config path: %s\n", status.ConfigPath)
	fmt.Printf("HEARTBEAT path: %s\n", status.HeartbeatPath)
	fmt.Printf("memory.md path: %s\n", status.MemoryMarkdownPath)
	fmt.Printf("Config exists: %t\n", status.ConfigExists)
	fmt.Printf("HEARTBEAT exists: %t\n", status.HeartbeatExists)
	fmt.Printf("Enabled: %t\n", status.Config.Enabled)
	fmt.Printf("Drift detected: %t\n", status.DriftDetected)
	fmt.Printf("Monthly runs: %d\n", status.CostEstimate.EstimatedRunsPerMonth)
	fmt.Printf("Estimated OK cost/month: $%.4f\n", status.CostEstimate.EstimatedOkCostUsdPerMonth)
	fmt.Printf("Estimated alert cost/month: $%.4f\n", status.CostEstimate.EstimatedAlertCostUsdPerMonth)

	if status.LastRun != nil {
		fmt.Printf("Last run: %s\n", status.LastRun.LastRunAtUtc.UTC().Format(time.RFC3339Nano))
		fmt.Printf("Last outcome: %s\n", status.LastRun.Outcome)
		fmt.Printf("Delivery suppressed: %t\n", status.LastRun.DeliverySuppressed)
		fmt.Printf("Last run tokens: in %d / out %d\n", status.LastRun.InputTokens, status.LastRun.OutputTokens)
		if strings.TrimSpace(status.LastRun.MessagePreview) != "" {
			fmt.Printf("Last preview: %s\n", status.LastRun.MessagePreview)
		}
	}

	if len(status.Issues) > 0 {
		fmt.Println()
		fmt.Println("Issues:")
		for _, issue := range status.Issues {
			fmt.Printf("- %s: %s\n", issue.Severity, issue.Message)
		}
	}
}

func printHeartbeatHelp() {
	fmt.Println(`
            openclaw heartbeat

            Usage:
              openclaw heartbeat status [--url <url>] [--token <token>]
              openclaw heartbeat preview [--url <url>] [--token <token>]
              openclaw heartbeat wizard [--url <url>] [--token <token>]
              openclaw heartbeat run --text <text> [--mode now|next-heartbeat] [--url <url>] [--token <token>]

            Notes:
              - Wizard/preview/status manage the legacy cron-backed heartbeat wizard.
              - Run is an alias for Runtime Pulse manual wake.
              - Prefer OPENCLAW_BASE_URL / OPENCLAW_AUTH_TOKEN over command-line tokens.`)
}

func modelsCmd(ctx context.Context, args []string) error        { return nil }
func evalCmd(ctx context.Context, args []string) error          { return nil }
func accountsCmd(ctx context.Context, args []string) error      { return nil }
func backendsCmd(ctx context.Context, args []string) error      { return nil }
func adminCmd(ctx context.Context, args []string) error         { return nil }
func compatibilityCmd(ctx context.Context, args []string) error { return nil }
func pluginsCmd(ctx context.Context, args []string) error       { return nil }
func skillCmd(ctx context.Context, args []string) error         { return nil }
func skillsCmd(ctx context.Context, args []string) error        { return nil }
func clawHubCmd(ctx context.Context, args []string) error       { return nil }

func printVersion(ctx context.Context, args []string) error {
	fmt.Println("1.0.0")
	return nil
}

func runPluginOrUnknown(ctx context.Context, command string, args []string) error {
	code, err := cli.PluginCliCommandRun(ctx, command, args)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("unknown command or plugin: %s", command)
	}

	return nil
}

func printHelp() {
	fmt.Println(`
            openclaw — OpenClaw.NET CLI

            Usage:
              openclaw start [options]
              openclaw run [options] <prompt>
              openclaw chat [options]
              openclaw live [options]
              openclaw tui [options]
              openclaw insights [options]
              openclaw setup [options]
              openclaw setup <launch|service|status|verify|channel|provider|tailscale> [options]
              openclaw update <trust|check|install|rollback|launch> [options]
              openclaw upgrade <check|rollback> [options]
              openclaw backup <create|validate|restore> [options]
              openclaw init [options]
              openclaw migrate [options]
              openclaw migrate <legacy|upstream> [options]
              openclaw pulse <status|run|enable|disable|events|doctor> [options]
              openclaw heartbeat <wizard|preview|status|run> [options]
              openclaw models <list|doctor|presets> [options]
              openclaw maintenance <scan|fix> [options]
              openclaw payment <setup|funding list|virtual-card issue|execute|status> [options]
              openclaw external <list|status|commands|preview|execute> [options]
              openclaw routing <onboard|configure|providers|status|diagnostics> [options]
              openclaw memory fractal <status|search|open|export|recent|handoff create|validate|index refresh> [options]
              openclaw test <init|run|report|gates> [options]
              openclaw harness <test|regression|map|state> [options]
              openclaw regression test [options]
              openclaw eval <run|compare> [options]
              openclaw accounts <list|add|remove|probe> [options]
              openclaw backends <list|probe|run|session send> [options]
              openclaw admin <posture|incident export|trajectory export|approvals simulate> [options]
              openclaw compatibility <catalog> [options]
              openclaw plugins <install|remove|list|search> [options]
              openclaw skill <new|critique|generate|validate|run|package|list> [options]
              openclaw skills <inspect|install|list|catalog|create|proposals|meta-runs> [options]
              openclaw skills catalog [--workdir <path> | --managed] [--kind <all|meta>] [--json]
              openclaw skills create <name> [--kind <standard|meta>] [--description <text>] [--proposal-draft] [--workdir <path> | --managed] [--json] [--force]
              openclaw skills proposals <session-id> [--run <run-id>] [--storage <path>] [--json]
              openclaw skills proposals show <session-id> --proposal <id> [--storage <path>] [--json]
              openclaw skills meta-runs <session-id> [--storage <path>] [--limit <count>] [--run <run-id>] [--verbose] [--json]
              openclaw skills meta-runs replay <session-id> --run <run-id> [--storage <path>] [--json]
              openclaw skills meta-runs reconstruct <session-id> --run <run-id> [--storage <path>] [--json]
              openclaw skills meta-runs proposals <session-id> [--run <run-id>] [--storage <path>] [--json]
              openclaw skills meta-runs proposals show <session-id> --proposal <id> [--storage <path>] [--json]
              openclaw skills meta-runs proposals accept <session-id> --proposal <id> [--storage <path>] [--json]
              openclaw skills meta-runs proposals dismiss <session-id> --proposal <id> [--reason <text>] [--storage <path>] [--json]
              openclaw skills meta-runs proposals rollback <session-id> --proposal <id> [--reason <text>] [--storage <path>] [--json]
              openclaw skills meta-runs proposals change <session-id> --proposal <id> --to <accept|dismiss> [--reason <text>] [--storage <path>] [--json]
              openclaw clawhub [wrapper options] [--] <clawhub args...>

            Common options:
              --url <url>        Base URL (default: OPENCLAW_BASE_URL or http://127.0.0.1:18789)
              --token <token>    Auth token (deprecated: prefer OPENCLAW_AUTH_TOKEN)
              --model <model>    Model override (optional)
              --system <text>    System prompt (optional)
              --preset <id>      Tool preset / platform policy bundle (optional)

            run options:
              --file <path>      Attach file contents (repeatable)
              --image <path|url> Attach image input (repeatable)
              --no-stream        Disable SSE streaming
              --temperature <n>  Temperature (optional)
              --max-tokens <n>   Max tokens (optional)

            chat commands:
              /help, /exit, /reset
              /system <text>
              /model <model>
              /image <path|url> [prompt]

            Examples:
              openclaw start
              openclaw start --with-companion --open-browser
              openclaw start --non-interactive --profile local --workspace ./workspace --provider openai --model gpt-4o --api-key env:MODEL_PROVIDER_KEY
              openclaw start --non-interactive --profile local --workspace ./workspace --provider deepseek --model deepseek-v4-flash --api-key env:DEEPSEEK_API_KEY
              openclaw start --non-interactive --profile local --workspace ./workspace --provider ollama --model llama3.2 --model-preset ollama-general
              openclaw run "summarize this README" --file ./README.md
              OPENCLAW_AUTH_TOKEN=... openclaw run "summarize this README" --file ./README.md
              cat error.log | openclaw run "what went wrong?"
              openclaw chat --system "Be concise."
              openclaw live --model gemini-2.0-flash-live-001 --system "Be concise."
              openclaw tui
              openclaw insights
              openclaw setup
              openclaw setup tailscale serve
              openclaw setup provider aperture --config ~/.openclaw/config/openclaw.settings.json --endpoint https://YOUR_APERTURE_ENDPOINT --model YOUR_APERTURE_MODEL_ROUTE --auth-mode bearer --env-var OPENCLAW_APERTURE_TOKEN
              openclaw upgrade check
              openclaw upgrade check --config ~/.openclaw/config/openclaw.settings.json --offline
              openclaw upgrade rollback --config ~/.openclaw/config/openclaw.settings.json --offline
              openclaw setup --non-interactive --profile local --workspace ./workspace --provider openai --model gpt-4o --api-key env:MODEL_PROVIDER_KEY
              openclaw setup --non-interactive --profile local --workspace ./workspace --provider deepseek --model deepseek-v4-flash --api-key env:DEEPSEEK_API_KEY
              openclaw setup --non-interactive --profile local --workspace ./workspace --provider ollama --model llama3.2 --model-preset ollama-general
              openclaw setup provider aperture --endpoint https://YOUR_APERTURE_ENDPOINT --model YOUR_APERTURE_MODEL_ROUTE --auth-mode tailnet-identity
              openclaw setup verify --config ~/.openclaw/config/openclaw.settings.json
              openclaw setup launch --config ~/.openclaw/config/openclaw.settings.json --with-companion --open-browser
              openclaw setup service --config ~/.openclaw/config/openclaw.settings.json --platform all
              openclaw setup status --config ~/.openclaw/config/openclaw.settings.json
              openclaw init --preset public
              openclaw migrate --apply
              openclaw migrate upstream --source ./upstream-agent --target-config ~/.openclaw/config/openclaw.settings.json --report ./migration-report.json
              openclaw heartbeat status
              openclaw pulse status
              openclaw pulse run --text "Check for urgent follow-ups"
              openclaw external list
              openclaw routing --help
              openclaw memory fractal status
              openclaw memory fractal search "context bloat"
              openclaw test run
              openclaw test gates
              openclaw harness test
              openclaw harness test --category security --strict
              openclaw harness map
              openclaw harness state list
              openclaw models list
              openclaw models presets
              openclaw models doctor
              openclaw maintenance scan
              openclaw maintenance fix --dry-run
              openclaw eval run --profile gemma4-prod
              openclaw eval compare --profiles gemma4-prod,frontier-tools
              openclaw accounts list
              openclaw accounts add codex --display-name "Local Codex" --secret-ref env:OPENAI_API_KEY
              openclaw backends list
              openclaw backends run codex-cli --workspace . --prompt "summarize this repository"
              openclaw heartbeat wizard
              openclaw admin posture
              openclaw admin approvals simulate --tool shell --args "{\"command\":\"pwd\"}"
              openclaw admin incident export
              openclaw compatibility catalog --status compatible
              openclaw skill new "Community Research Insight Extractor" --category research
              openclaw skill validate community.research_insight
              openclaw skill run community.research_insight --input transcript.md --dry-run

            Gateway direct-start fallback:
              dotnet run --project src/OpenClaw.Gateway -c Release -- --quickstart
              # Uses a minimal local loopback profile, prompts for missing provider inputs,
              # retries in-process on common startup failures, and can save the working setup.

            Plugin management:
              openclaw plugins install <package-name>    Install from npm/ClawHub
              openclaw plugins install ./local-plugin     Install from local path
              openclaw plugins remove <plugin-name>       Remove a plugin
              openclaw plugins list                       List installed plugins
              openclaw plugins search <query>             Search npm for plugins

              Installed plugins may also register root commands. Built-in commands
              take precedence; disabled or quarantined plugins are not dispatched.
              openclaw <plugin-command> --help            Show plugin command help

            Skill management:
              openclaw skill new "Community Research Insight Extractor" --category research
              openclaw skill validate community.research_insight
              openclaw skill package community.research_insight
              openclaw skills inspect ./my-skill          Inspect a local skill package
              openclaw skills install ./my-skill --dry-run
              openclaw skills install ./my-skill --managed
              openclaw skills list --managed

            Compatibility catalog:
              openclaw compatibility catalog              List pinned upstream compatibility scenarios
              openclaw compat catalog --json             Emit catalog JSON

            ClawHub wrapper:
              # Forward --help to ClawHub itself:
              openclaw clawhub -- --help
              # Install skills into $OPENCLAW_WORKSPACE/skills (default):
              openclaw clawhub install <skill-slug>
              # Install into ~/.openclaw/skills:
              openclaw clawhub --managed install <skill-slug>
            `)
}
