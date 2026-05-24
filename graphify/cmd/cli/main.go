package main

import (
	"fmt"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyousuzu/graphify"
)

var (
	outputOpt   string
	formatOpt   string
	verboseOpt  bool
	providerOpt string
	endpointOpt string
	apiKeyOpt   string
	modelOpt    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "graphify-dotnet",
		Short: "graphify: AI-powered knowledge graph builder for codebases",
	}

	cp := &graphify.ConfigPersistence{}

	var useConfigWizard bool

	runCmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Run the full extraction and graph-building pipeline",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolvePath(args)

			output := outputOpt
			format := formatOpt

			savedConfig := cp.Load()
			if savedConfig != nil {
				if path == "." && savedConfig.WorkingFolder != "" {
					path = savedConfig.WorkingFolder
				}

				if output == "graphify-out" && savedConfig.OutputFolder != "" {
					output = savedConfig.OutputFolder
				}

				if format == "json,html,report" && savedConfig.ExportFormats != "" {
					format = savedConfig.ExportFormats
				}
			}

			if useConfigWizard {
				if err := runConfigWizard(cp); err != nil {
					return err
				}
				fmt.Println()
			}

			formats := parseFormats(format)

			chatClient, err := resolveProvider(
				providerOpt,
				endpointOpt,
				apiKeyOpt,
				modelOpt,
			)
			if err != nil {
				return err
			}

			runner := graphify.NewPipelineRunner(
				os.Stdout,
				&verboseOpt,
				chatClient,
			)

			_, err = runner.Run(
				cmd.Context(),
				path,
				output,
				formats,
				verboseOpt,
			)

			return err
		},
	}

	runCmd.Flags().BoolVarP(
		&useConfigWizard,
		"config",
		"c",
		false,
		"Launch interactive configuration wizard before running",
	)

	addPipelineOptions(runCmd)
	rootCmd.AddCommand(runCmd)

	watchCmd := &cobra.Command{
		Use:   "watch [path]",
		Short: "Watch for changes and re-process",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolvePath(args)
			formats := parseFormats(formatOpt)

			chatClient, err := resolveProvider(
				providerOpt,
				endpointOpt,
				apiKeyOpt,
				modelOpt,
			)
			if err != nil {
				return err
			}

			pterm.Info.Println("Running initial pipeline...")
			fmt.Println()

			runner := graphify.NewPipelineRunner(
				os.Stdout,
				&verboseOpt,
				chatClient,
			)

			graph, err := runner.Run(
				cmd.Context(),
				path,
				outputOpt,
				formats,
				verboseOpt,
			)
			if err != nil {
				pterm.Error.Println("Initial pipeline failed. Aborting watch.")
				return err
			}

			watcher := graphify.NewWatchMode(os.Stdout, verboseOpt)
			watcher.SetInitialGraph(graph)

			watcher.Watch(
				cmd.Context(),
				path,
				outputOpt,
				formats,
			)

			return nil
		},
	}

	addPipelineOptions(watchCmd)
	rootCmd.AddCommand(watchCmd)

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := []string{
				"📋 View current configuration",
				"🔧 Set up AI provider",
				"📂 Set folder to analyze",
			}

			selected, err := pterm.DefaultInteractiveSelect.
				WithOptions(options).
				WithDefaultText("What would you like to do?").
				Show()

			if err != nil {
				return err
			}

			switch {
			case strings.HasPrefix(selected, "📋"):
				showStyledConfig(cp)

			case strings.HasPrefix(selected, "🔧"),
				strings.HasPrefix(selected, "📂"):
				return runConfigWizard(cp)
			}

			return nil
		},
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Display resolved provider settings",
		Run: func(cmd *cobra.Command, args []string) {
			showStyledConfig(cp)
		},
	}

	configSetCmd := &cobra.Command{
		Use:   "set",
		Short: "Set up AI provider interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigWizard(cp)
		},
	}

	configFolderCmd := &cobra.Command{
		Use:   "folder",
		Short: "Set the default project folder to analyze",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigWizard(cp)
		},
	}

	configCmd.AddCommand(
		configShowCmd,
		configSetCmd,
		configFolderCmd,
	)

	rootCmd.AddCommand(configCmd)

	if err := rootCmd.Execute(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}

func addPipelineOptions(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&outputOpt,
		"output",
		"o",
		"graphify-out",
		"Output directory",
	)

	cmd.Flags().StringVarP(
		&formatOpt,
		"format",
		"f",
		"json,html,report",
		"Export formats (comma-separated): json, html, svg, neo4j, ladybug, obsidian, wiki, report",
	)

	cmd.Flags().BoolVarP(
		&verboseOpt,
		"verbose",
		"v",
		false,
		"Enable verbose output",
	)

	cmd.Flags().StringVarP(
		&providerOpt,
		"provider",
		"p",
		"",
		"AI provider: openai",
	)

	cmd.Flags().StringVar(
		&endpointOpt,
		"endpoint",
		"",
		"AI service endpoint URL",
	)

	cmd.Flags().StringVar(
		&apiKeyOpt,
		"api-key",
		"",
		"API key for the AI provider",
	)

	cmd.Flags().StringVar(
		&modelOpt,
		"model",
		"",
		"Model ID (e.g., gpt-4o)",
	)
}

func resolveProvider(
	provider,
	endpoint,
	apiKey,
	model string,
) (chatcompletion.IChatClient, error) {
	graphifyConfig := &graphify.GraphifyConfig{
		Provider: provider,
		OpenAI: &graphify.OpenAIConfig{
			Endpoint: endpoint,
			ModelId:  model,
			ApiKey:   apiKey,
		},
	}

	if graphifyConfig.Provider == "" {
		pterm.Warning.Println("No AI provider configured. Running in AST-only mode.")
		return nil, nil
	}

	pterm.Success.Printf(
		"AI provider: %s\n",
		graphifyConfig.Provider,
	)

	switch strings.ToLower(graphifyConfig.Provider) {
	case "openai":
		pterm.Warning.Printf(
			"Note: Source code contents will be sent to %s for semantic analysis. Use AST-only mode for local-only analysis.\n",
			graphifyConfig.Provider,
		)

		return graphify.ChatClientResolver(graphifyConfig), nil

	default:
		return nil, fmt.Errorf(
			"unsupported provider: %s",
			graphifyConfig.Provider,
		)
	}
}

func runConfigWizard(cp *graphify.ConfigPersistence) error {
	existingConfig := cp.Load()

	wizard := &graphify.ConfigWizard{}
	wizardConfig := wizard.Run(existingConfig)

	cp.Save(wizardConfig)

	return nil
}

func resolvePath(args []string) string {
	if len(args) > 0 {
		return args[0]
	}

	return "."
}

func parseFormats(format string) []string {
	parts := strings.Split(format, ",")

	var result []string

	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))

		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

func readEnv() *graphify.GraphifyConfig {
	c := &graphify.GraphifyConfig{
		Provider:      os.Getenv("Provider"),
		WorkingFolder: os.Getenv("WorkingFolder"),
		OutputFolder:  os.Getenv("OutputFolder"),
		ExportFormats: os.Getenv("Formats"),
	}

	if c.Provider == "openai" {
		c.OpenAI = &graphify.OpenAIConfig{
			Endpoint: os.Getenv("Endpoint"),
			ModelId:  os.Getenv("ModelId"),
			ApiKey:   os.Getenv("ApiKey"),
		}
	}
	return c
}

func showStyledConfig(cp *graphify.ConfigPersistence) {
	config := readEnv()
	savedConfig := cp.Load()

	pterm.DefaultSection.
		WithLevel(1).
		Println("Graphify Configuration (resolved)")

	var providerText string

	if config.Provider != "" {
		providerText = pterm.Green(config.Provider)
	} else {
		providerText = pterm.Gray("(not set — AST-only mode)")
	}

	pterm.Println(
		pterm.Bold.Sprint("Provider: ") + providerText,
	)

	fmt.Println()

	projectData := pterm.TableData{
		{"Setting", "Value"},
		{"Working Folder", formatValue(savedConfig.WorkingFolder)},
		{"Output Folder", formatValue(savedConfig.OutputFolder)},
		{"Export Formats", formatValue(savedConfig.ExportFormats)},
	}

	projectTableStr, _ := pterm.DefaultTable.
		WithHasHeader().
		WithBoxed().
		WithData(projectData).
		Srender()

	pterm.DefaultBox.
		WithTitle("Project Settings").
		Println(projectTableStr)

	openaiData := pterm.TableData{
		{"Setting", "Value"},
		{"Endpoint", formatValue(config.OpenAI.Endpoint)},
		{"Model", formatValue(config.OpenAI.ModelId)},
		{"API Key", maskSecret(config.OpenAI.ApiKey)},
	}

	openaiTableStr, _ := pterm.DefaultTable.
		WithHasHeader().
		WithBoxed().
		WithData(openaiData).
		Srender()

	pterm.DefaultBox.
		WithTitle("OpenAI / Azure OpenAI").
		Println(openaiTableStr)

	fmt.Println()

	panelContent :=
		pterm.FgLightCyan.Sprint("1. ") +
			"CLI arguments (--provider, --endpoint, etc.)\n" +
			pterm.FgGray.Sprint("2. ") +
			"Environment variables (GRAPHIFY__*)\n"

	pterm.DefaultBox.
		WithTitle("Configuration sources (highest priority first)").
		Println(panelContent)
}

func formatValue(val string) string {
	if val != "" {
		return pterm.Green(val)
	}

	return pterm.Gray("(not set)")
}

func maskSecret(val string) string {
	if val == "" {
		return pterm.Gray("(not set)")
	}

	if len(val) <= 4 {
		return pterm.Yellow("****")
	}

	return pterm.Yellow("****" + val[len(val)-4:])
}
