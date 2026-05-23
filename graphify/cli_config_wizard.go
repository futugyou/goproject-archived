package graphify

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
)

type ConfigWizard struct{}

func (cw ConfigWizard) Run(existing *GraphifyConfig) *GraphifyConfig {
	config := existing
	if config == nil {
		config = &GraphifyConfig{}
	}

	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgBlue)).Println("🔧 Graphify Configuration")
	fmt.Println()

	providerChoice, _ := pterm.DefaultInteractiveSelect.
		WithOptions([]string{
			"OpenAI",
			"None (AST-only mode)",
		}).
		Show("Select AI provider")

	switch providerChoice {
	case "OpenAI":
		config.Provider = "OpenAI"
		cw.promptOpenAI(config)
	case "None (AST-only mode)":
		config.Provider = ""
	}

	fmt.Println()
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgCyan)).Println("Export Settings")
	fmt.Println()

	allFormats := []string{"json", "html", "svg", "neo4j", "obsidian", "wiki", "report"}
	defaultFormats := cw.parseSelectedFormats(config.ExportFormats)

	selectedFormats, _ := pterm.DefaultInteractiveMultiselect.
		WithOptions(allFormats).
		WithDefaultOptions(defaultFormats).
		Show("Export formats (Use Space to select, Enter to confirm)")

	config.ExportFormats = strings.Join(selectedFormats, ",")

	fmt.Println()
	cw.showSummary(config)

	return config
}

func (cw ConfigWizard) RunFolderWizard(existing *GraphifyConfig) *GraphifyConfig {
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgBlue)).Println("📂 Project Folder Settings")
	fmt.Println()

	config := existing
	if config == nil {
		config = &GraphifyConfig{}
	}

	workingFolderDefault := "."
	if config.WorkingFolder != "" {
		workingFolderDefault = config.WorkingFolder
	}
	workingFolder, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(workingFolderDefault).
		Show("Project folder to analyze")

	if strings.TrimSpace(workingFolder) == "" {
		pterm.Error.Println("Folder path is required")
		return config
	}
	config.WorkingFolder = workingFolder

	outputFolderDefault := "graphify-out"
	if config.OutputFolder != "" {
		outputFolderDefault = config.OutputFolder
	}
	outputFolder, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(outputFolderDefault).
		Show("Output directory")
	config.OutputFolder = outputFolder

	allFormats := []string{"json", "html", "svg", "neo4j", "obsidian", "wiki", "report"}
	defaultFormats := cw.parseSelectedFormats(config.ExportFormats)

	formatChoices, _ := pterm.DefaultInteractiveMultiselect.
		WithOptions(allFormats).
		WithDefaultOptions(defaultFormats).
		Show("Export formats")

	config.ExportFormats = strings.Join(formatChoices, ",")

	fmt.Println()
	cw.showFolderSummary(config)

	return config
}

func (cw ConfigWizard) promptOpenAI(config *GraphifyConfig) {
	fmt.Println()
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgCyan)).Println("Azure OpenAI Settings")
	fmt.Println()

	// Endpoint
	endpointDefault := ""
	if config.OpenAI.Endpoint != "" {
		endpointDefault = config.OpenAI.Endpoint
	}
	endpoint, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(endpointDefault).
		Show("Endpoint URL")
	config.OpenAI.Endpoint = endpoint

	// API Key
	apiKey, _ := pterm.DefaultInteractiveTextInput.
		WithMask("*").
		Show("API Key")
	config.OpenAI.ApiKey = apiKey

	// Model ID
	modelDefault := "gpt-4o"
	if config.OpenAI.ModelId != "" {
		modelDefault = config.OpenAI.ModelId
	}
	model, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultValue(modelDefault).
		Show("Model ID")
	config.OpenAI.ModelId = model
}

func (cw ConfigWizard) showSummary(config *GraphifyConfig) {
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgGreen)).Println("✅ Configuration Summary")

	tableData := pterm.TableData{
		{"Setting", "Value"},
	}

	provider := config.Provider
	if provider == "" {
		provider = "None (AST-only)"
	}
	tableData = append(tableData, []string{"Provider", pterm.LightGreen(provider)})

	switch strings.ToLower(config.Provider) {
	case "OpenAI":
		tableData = append(tableData, []string{"Endpoint", config.OpenAI.Endpoint})
		tableData = append(tableData, []string{"API Key", cw.maskSecret(config.OpenAI.ApiKey)})
		tableData = append(tableData, []string{"Model", config.OpenAI.ModelId})
	}

	if config.ExportFormats != "" {
		tableData = append(tableData, []string{"Export Formats", config.ExportFormats})
	}

	_ = pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()
}

func (cw ConfigWizard) showFolderSummary(config *GraphifyConfig) {
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgGreen)).Println("✅ Folder Settings Summary")

	tableData := pterm.TableData{
		{"Setting", "Value"},
		{"Working Folder", config.WorkingFolder},
		{"Output Folder", config.OutputFolder},
		{"Export Formats", config.ExportFormats},
	}

	_ = pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()
}

func (cw ConfigWizard) parseSelectedFormats(formats string) []string {
	if strings.TrimSpace(formats) == "" {
		return []string{"json", "html", "report"}
	}
	parts := strings.Split(formats, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func (cw ConfigWizard) maskSecret(value string) string {
	if value == "" {
		return "(not set)"
	}
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}
