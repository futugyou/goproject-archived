package main

import (
	"bufio"
	"context"
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
	"unicode"

	"github.com/futugyou/openclaw/cli"
	"github.com/futugyou/openclaw/client"
	"github.com/futugyou/openclaw/core"
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

	firstSpace := strings.Index(tail, " ")
	image := tail
	if firstSpace >= 0 {
		image = tail[:firstSpace]
	}

	prompt := "Describe this image."
	if firstSpace >= 0 {
		image = tail[firstSpace+1:]
	}

	if prompt == "" {
		prompt = "Describe this image."
	}

	return BuildUserContent(prompt, nil, []string{image})
}

func liveCmd(ctx context.Context, args []string) error          { return nil }
func tuiCmd(ctx context.Context, args []string) error           { return nil }
func insightsCmd(ctx context.Context, args []string) error      { return nil }
func setupCmd(ctx context.Context, args []string) error         { return nil }
func updateCmd(ctx context.Context, args []string) error        { return nil }
func upgradeCmd(ctx context.Context, args []string) error       { return nil }
func backupCmd(ctx context.Context, args []string) error        { return nil }
func maintenanceCmd(ctx context.Context, args []string) error   { return nil }
func paymentCmd(ctx context.Context, args []string) error       { return nil }
func externalCmd(ctx context.Context, args []string) error      { return nil }
func routingCmd(ctx context.Context, args []string) error       { return nil }
func memoryCmd(ctx context.Context, args []string) error        { return nil }
func testCmd(ctx context.Context, args []string) error          { return nil }
func harnessCmd(ctx context.Context, args []string) error       { return nil }
func regressionCmd(ctx context.Context, args []string) error    { return nil }
func initCmd(ctx context.Context, args []string) error          { return nil }
func migrateCmd(ctx context.Context, args []string) error       { return nil }
func pulseCmd(ctx context.Context, args []string) error         { return nil }
func heartbeatCmd(ctx context.Context, args []string) error     { return nil }
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
