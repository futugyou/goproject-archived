package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

func MemoryCommandRun(ctx context.Context, args []string) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		memoryCommandPrintHelp()
		return 0, nil
	}

	parsed := CliArgsParse(args)
	if parsed.ShowHelp || len(parsed.Positionals) < 2 || parsed.Positionals[0] != "fractal" {
		memoryCommandPrintHelp()
		if parsed.ShowHelp {
			return 0, nil
		} else {
			return 2, nil
		}
	}

	command := parsed.Positionals[1]
	jsonOutput := parsed.HasFlag("--json")

	client, err := createClient(parsed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1, err
	}

	if command == "workflow" {
		operation, ok := memoryCommandTryGetPosition(parsed, 2, "operation")
		if !ok {
			return 2, nil
		}
		argumentsFile := parsed.GetOption("--arguments-file")
		inlineArguments := parsed.GetOption("--arguments")
		if argumentsFile != nil && inlineArguments != nil {
			return 1, errors.New("Use either --arguments or --arguments-file.")
		}

		arguments := ""
		if util.Deref(argumentsFile) == "" {
			if util.Deref(argumentsFile) == "" {
				arguments = "{}"
			} else {
				arguments = util.Deref(argumentsFile)
			}
		} else {
			data, err := os.ReadFile(*argumentsFile)
			if err != nil {
				return 1, err
			}
			arguments = string(data)
		}

		var doc map[string]any
		if err := json.Unmarshal([]byte(arguments), &doc); err != nil {
			return 1, err
		}

		result, err := client.ExecuteFractalMemoryWorkflow(ctx, operation, doc)
		if err != nil {
			return 2, err
		}
		writeOutput(result, jsonOutput, func(v *core.StructuredMemoryWorkflowResult) {
			if result.Success {
				fmt.Println(result.Data)
			} else {
				fmt.Println(result.Error)
			}
		})

		return 0, nil
	}
	if command == "status" {
		response, err := client.GetFractalMemoryStatus(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextStatus)
		if response.Enabled && !response.Available {
			return 1, nil
		}
		return 0, nil
	}
	if command == "search" {
		query, ok := memoryCommandTryGetPosition(parsed, 2, "query")
		if !ok {
			return 2, nil
		}
		response, err := client.SearchFractalMemory(ctx, query,
			getIntOption(parsed, "--limit", 10),
			util.Deref(parsed.GetOption("--scope")))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextSearch)
		if response.Success {
			return 1, nil
		}
		return 0, nil
	}

	if command == "open" {
		path, ok := memoryCommandTryGetPosition(parsed, 2, "path")
		if !ok {
			return 2, nil
		}
		response, err := client.OpenFractalMemory(ctx, path,
			util.Deref(getNullableIntOption(parsed, "--depth")),
			util.Deref(parsed.GetOption("--view")))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextOpen)
		if response.Success {
			return 1, nil
		}
		return 0, nil
	}

	if command == "export" {
		path, ok := memoryCommandTryGetPosition(parsed, 2, "path")
		if !ok {
			return 2, nil
		}
		response, err := client.ExportFractalMemory(ctx, path,
			util.Deref(parsed.GetOption("--mode")))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextExport)
		if response.Success {
			return 1, nil
		}
		return 0, nil
	}

	if command == "recent" {
		response, err := client.GetRecentFractalMemory(ctx,
			getIntOption(parsed, "--days", 30),
			getIntOption(parsed, "--limit", 10),
			util.Deref(parsed.GetOption("--scope")))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextRecent)
		if response.Success {
			return 1, nil
		}
		return 0, nil
	}

	if command == "validate" {
		response, err := client.ValidateFractalMemory(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextValidation)
		if response.Success && !response.HasErrors {
			return 1, nil
		}
		return 0, nil
	}

	if command == "index" && len(parsed.Positionals) > 2 && parsed.Positionals[2] == "refresh" {
		response, err := client.RefreshFractalMemoryIndex(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextValidation)
		if response.Success && !response.HasErrors {
			return 1, nil
		}
		return 0, nil
	}

	if command == "handoff" && len(parsed.Positionals) > 2 && parsed.Positionals[2] == "create" {
		path, ok := memoryCommandTryGetPosition(parsed, 2, "path")
		if !ok {
			return 2, nil
		}
		response, err := client.CreateFractalMemoryHandoff(ctx, path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return 1, err
		}
		writeOutput(response, jsonOutput, memoryCommandTextHandoff)
		if response.Success {
			return 1, nil
		}
		return 0, nil
	}

	memoryCommandPrintHelp()
	return 2, nil
}

func memoryCommandPrintHelp() {
	fmt.Println(
		`
            openclaw memory fractal <command>

            Commands:
              status
              search <query> [--limit <n>] [--scope <path>] [--json]
              open <path> [--depth <0-3>] [--view index|state|timeline|decisions|children] [--json]
              export <path> [--mode compact|standard|verbose] [--json]
              recent [--days <n>] [--limit <n>] [--scope <path>] [--json]
              handoff create <path> [--json]
              validate [--json]
              index refresh [--json]
              workflow <operation> [--arguments <json> | --arguments-file <path>] [--json]

            Workflow operations:
              read, list, attention, decisions, context, resume, handoff_list, handoff_read,
              node_create, update, append, review, doctor, import

            Common options:
              --url <url>
              --token <token>
            `)
}

func memoryCommandTextValidation(response *core.StructuredMemoryValidationResult) {
	if !response.Success {
		fmt.Printf("Error: %s\n", response.Error)
		return
	}

	msg := response.Summary
	if msg == "" {
		if response.HasErrors {
			msg = "Validation reported errors."
		} else {
			msg = "Validation passed."
		}
	}
	fmt.Println(msg)
	for _, issue := range response.Issues {
		fmt.Printf("- %s: %s %s\n", issue.Severity, issue.Path, issue.Message)
	}
}

func memoryCommandTextHandoff(response *core.StructuredMemoryHandoffResult) {
	if !response.Success {
		fmt.Printf("Error: %s\n", response.Error)
		return
	}

	msg := response.HandoffFilePath
	if msg == "" {
		msg = response.Path
	}
	fmt.Println(msg)
	if response.Content != "" {
		fmt.Println(response.Content)
	}
}

func memoryCommandTextExport(response *core.StructuredMemoryExportResult) {
	if !response.Success {
		fmt.Printf("Error: %s\n", response.Error)
		return
	}

	fmt.Printf("Source: %s\n", response.Path)
	fmt.Printf("Mode: %s\n", response.Mode)
	if response.Content != "" {
		fmt.Println(response.Content)
	}
}

func memoryCommandTextOpen(response *core.StructuredMemoryOpenResult) {
	if !response.Success {
		fmt.Printf("Error: %s\n", response.Error)
		return
	}

	fmt.Printf("%s %s\n", response.Path, response.Title)
	if response.Content != "" {
		fmt.Println(response.Content)
	}
}

func memoryCommandTextRecent(response *core.StructuredMemoryRecentResult) {
	if !response.Success {
		fmt.Printf("Error: %s\n", response.Error)
		return
	}

	fmt.Printf("Recent nodes: %d\n", len(response.Items))
	for _, item := range response.Items {
		fmt.Printf("- %s %s\n", item.Path, item.Title)
	}
}

func memoryCommandTextSearch(response *core.StructuredMemorySearchResult) {
	if !response.Success {
		fmt.Printf("Error: %s\n", response.Error)
		return
	}

	fmt.Printf("Matches: %d\n", len(response.Items))

	for _, item := range response.Items {
		fmt.Printf("- %s %s\n", item.Path, item.Title)
	}
}

func memoryCommandTextStatus(response *core.StructuredMemoryStatusResponse) {
	fmt.Println("Fractal Memory")
	fmt.Printf("Status: %s\n", response.Status)
	fmt.Printf("Enabled: %t\n", response.Enabled)
	fmt.Printf("Mode: %s\n", response.Mode)
	fmt.Printf("Repository: %s\n", response.ResolvedRepositoryRoot)
	fmt.Printf("MCP: %s\n", response.McpCommand)
	fmt.Printf("Auto context: %s\n", response.AutoContextMode)
	msg := "disabled"
	if response.WriteToolsAvailable {
		msg = "enabled"
	}
	fmt.Printf("Writes: %s\n", msg)
	for _, warning := range response.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}

	if response.Error != "" {
		fmt.Printf("Error: %s\n", response.Error)
	}
}

func getNullableIntOption(parsed *CliArgs, name string) *int {
	value := parsed.GetOption(name)
	if value == nil {
		return nil
	}

	v, err := strconv.Atoi(*value)
	if err != nil {
		return nil
	}

	return &v
}

func memoryCommandTryGetPosition(parsed *CliArgs, index int, name string) (string, bool) {
	if len(parsed.Positionals) > index && parsed.Positionals[index] != "" {
		return parsed.Positionals[index], true
	}

	fmt.Printf("%s is required.\n", name)
	memoryCommandPrintHelp()
	return "", false
}
