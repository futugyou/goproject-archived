package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/futugyou/openclaw/cli"
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
func runCmd(ctx context.Context, args []string) error           { return nil }
func chatCmd(ctx context.Context, args []string) error          { return nil }
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
