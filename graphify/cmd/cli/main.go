package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyousuzu/graphify"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
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
	var rootCmd = &cobra.Command{
		Use:   "graphify-dotnet",
		Short: "graphify: AI-powered knowledge graph builder for codebases",
	}

	cp := &graphify.ConfigPersistence{}
	var useConfigWizard bool
	var runCmd = &cobra.Command{
		Use:   "run [path]",
		Short: "Run the full extraction and graph-building pipeline",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			output := outputOpt
			format := formatOpt
			formats := strings.Split(format, ",")

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
					formats = strings.Split(format, ",")
				}
			}

			if useConfigWizard {
				existingConfig := cp.Load()
				wizzard := &graphify.ConfigWizard{}
				wizardConfig := wizzard.Run(existingConfig)
				cp.Save(wizardConfig)
				fmt.Println()
			}

			chatClient, verbose := resolveProvider(verboseOpt, providerOpt, endpointOpt, apiKeyOpt, modelOpt)

			runner := graphify.NewPipelineRunner(os.Stdout, &verbose, chatClient)
			_, err := runner.Run(cmd.Context(), path, output, formats, verbose)
			if err != nil {
				os.Exit(1)
			}
			return nil
		},
	}
	runCmd.Flags().BoolVarP(&useConfigWizard, "config", "c", false, "Launch interactive configuration wizard before running")
	addPipelineOptions(runCmd)
	rootCmd.AddCommand(runCmd)

	var watchCmd = &cobra.Command{
		Use:   "watch [path]",
		Short: "Watch for changes and re-process",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			formats := strings.Split(formatOpt, ",")

			chatClient, verbose := resolveProvider(verboseOpt, providerOpt, endpointOpt, apiKeyOpt, modelOpt)

			pterm.Info.Println("Running initial pipeline...")
			fmt.Println()

			runner := graphify.NewPipelineRunner(os.Stdout, &verbose, chatClient)
			var graph *graphify.KnowledgeGraph
			var err error
			if graph, err = runner.Run(cmd.Context(), path, outputOpt, formats, verbose); err != nil {
				pterm.Error.Println("Initial pipeline failed. Aborting watch.")
				os.Exit(1)
			}

			watcher := graphify.NewWatchMode(os.Stdout, verbose)
			watcher.SetInitialGraph(graph)
			watcher.Watch(cmd.Context(), path, outputOpt, formats)
			return nil
		},
	}
	addPipelineOptions(watchCmd)
	rootCmd.AddCommand(watchCmd)

	var configCmd = &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := []string{"📋 View current configuration", "🔧 Set up AI provider", "📂 Set folder to analyze"}
			selected, _ := pterm.DefaultInteractiveSelect.WithOptions(options).WithDefaultText("What would you like to do?").Show()

			if strings.HasPrefix(selected, "📋") {
				showStyledConfig(cp)
			} else if strings.HasPrefix(selected, "📂") {
				existingConfig := cp.Load()
				wizzard := &graphify.ConfigWizard{}
				wizardConfig := wizzard.Run(existingConfig)
				cp.Save(wizardConfig)
			} else {
				existingConfig := cp.Load()
				wizzard := &graphify.ConfigWizard{}
				wizardConfig := wizzard.Run(existingConfig)
				cp.Save(wizardConfig)
			}
			return nil
		},
	}

	var configShowCmd = &cobra.Command{
		Use:   "show",
		Short: "Display resolved provider settings",
		Run: func(cmd *cobra.Command, args []string) {
			showStyledConfig(cp)
		},
	}

	var configSetCmd = &cobra.Command{
		Use:   "set",
		Short: "Set up AI provider interactively",
		Run: func(cmd *cobra.Command, args []string) {
			existingConfig := cp.Load()
			wizzard := &graphify.ConfigWizard{}
			wizardConfig := wizzard.Run(existingConfig)
			cp.Save(wizardConfig)
		},
	}

	var configFolderCmd = &cobra.Command{
		Use:   "folder",
		Short: "Set the default project folder to analyze",
		Run: func(cmd *cobra.Command, args []string) {
			existingConfig := cp.Load()
			wizzard := &graphify.ConfigWizard{}
			wizardConfig := wizzard.Run(existingConfig)
			cp.Save(wizardConfig)
		},
	}

	configCmd.AddCommand(configShowCmd, configSetCmd, configFolderCmd)
	rootCmd.AddCommand(configCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func addPipelineOptions(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&outputOpt, "output", "o", "graphify-out", "Output directory")
	cmd.Flags().StringVarP(&formatOpt, "format", "f", "json,html,report", "Export formats (comma-separated): json, html, svg, neo4j, ladybug, obsidian, wiki, report")
	cmd.Flags().BoolVarP(&verboseOpt, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().StringVarP(&providerOpt, "provider", "p", "", "AI provider: azureopenai")
	cmd.Flags().StringVar(&endpointOpt, "endpoint", "", "AI service endpoint URL")
	cmd.Flags().StringVar(&apiKeyOpt, "api-key", "", "API key for the AI provider")
	cmd.Flags().StringVar(&modelOpt, "model", "", "Model ID (e.g., gpt-4o)")
}

func resolveProvider(verbose bool, provider, endpoint, apiKey, model string) (chatcompletion.IChatClient, bool) {
	graphifyConfig := &graphify.GraphifyConfig{
		Provider: provider,
		OpenAI: &graphify.OpenAIConfig{
			Endpoint: endpoint,
			ModelId:  model,
			ApiKey:   apiKey,
		},
	}

	pterm.Success.Printf("AI provider: %s\n", graphifyConfig.Provider)

	var chatClient chatcompletion.IChatClient

	prov := strings.ToLower(graphifyConfig.Provider)
	if prov == "openai" {
		chatClient = graphify.ChatClientResolver(graphifyConfig)
		pterm.Warning.Printf("Note: Source code contents will be sent to %s for semantic analysis. Use AST-only mode for local-only analysis.\n", graphifyConfig.Provider)
	}

	return chatClient, verbose
}

func showStyledConfig(cp *graphify.ConfigPersistence) {
	config := &graphify.GraphifyConfig{
		Provider: "openai",
		OpenAI: &graphify.OpenAIConfig{
			Endpoint: "https://my-azure-openai.openai.azure.com/",
			ModelId:  "gpt-4o",
			ApiKey:   "sk-1234567890abcdef",
		},
	}
	savedConfig := cp.Load()

	pterm.DefaultSection.WithLevel(1).Println("Graphify Configuration (resolved)")

	var providerText string
	if config.Provider != "" {
		providerText = pterm.Green(config.Provider)
	} else {
		providerText = pterm.Gray("(not set — AST-only mode)")
	}
	pterm.Println(pterm.Bold.Sprint("Provider: ") + providerText)
	fmt.Println()

	projectData := pterm.TableData{
		{"Setting", "Value"},
		{"Working Folder", formatValue(savedConfig.WorkingFolder)},
		{"Output Folder", formatValue(savedConfig.OutputFolder)},
		{"Export Formats", formatValue(savedConfig.ExportFormats)},
	}
	projectTableStr, _ := pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(projectData).Srender()
	pterm.DefaultBox.WithTitle("Project Settings").Println(projectTableStr)

	openaiData := pterm.TableData{
		{"Setting", "Value"},
		{"Endpoint", formatValue(config.OpenAI.Endpoint)},
		{"Model", formatValue(config.OpenAI.ModelId)},
		{"API Key", maskSecret(config.OpenAI.ApiKey)},
	}
	openaiTableStr, _ := pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(openaiData).Srender()
	pterm.DefaultBox.WithTitle("OpenAI / Azure OpenAI").Println(openaiTableStr)
	fmt.Println()

	panelContent := pterm.FgLightCyan.Sprint("1. ") + "CLI arguments (--provider, --endpoint, etc.)\n" +
		pterm.FgGray.Sprint("2. ") + "Environment variables (GRAPHIFY__*)\n"

	pterm.DefaultBox.WithTitle("Configuration sources (highest priority first)").Println(panelContent)
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
