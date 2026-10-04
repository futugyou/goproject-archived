package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/core"
)

func CompatibilityCommandRun(args []string) int {
	return RunWithWriters(args, os.Stdout, os.Stderr)
}

func RunWithWriters(args []string, output, errWriter io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printHelp(output)
		return 0
	}

	subcommand := args[0]
	rest := args[1:]

	switch subcommand {
	case "catalog", "list":
		return listCatalog(rest, output)
	default:
		return unknownSubcommand(subcommand, output, errWriter)
	}
}

func listCatalog(args []string, output io.Writer) int {
	status := getOptionValue(args, "--status")
	kind := getOptionValue(args, "--kind")
	category := getOptionValue(args, "--category")
	asJSON := containsOption(args, "--json")

	catalog, err := core.PublicCompatibilityCatalogInstance.GetCatalog(status, kind, category)
	if err != nil {
		fmt.Fprintln(output, err.Error())
		return 0
	}
	if asJSON {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(catalog); err != nil {
			return 1
		}
		return 0
	}

	fmt.Fprintf(output, "Compatibility catalog v%d (%d scenarios)\n", catalog.Version, len(catalog.Items))
	if len(catalog.Items) == 0 {
		fmt.Fprintln(output, "No compatibility scenarios matched the requested filters.")
		return 0
	}

	for _, item := range catalog.Items {
		fmt.Fprintf(output, "- %s [%s] %s\n", item.ID, item.Kind, item.CompatibilityStatus)
		fmt.Fprintf(output, "  Subject: %s\n", item.Subject)
		fmt.Fprintf(output, "  Summary: %s\n", item.Summary)
		fmt.Fprintf(output, "  Install: %s\n", item.InstallCommand)

		if strings.TrimSpace(item.ConfigJSONExample) != "" {
			fmt.Fprintf(output, "  Config: %s\n", item.ConfigJSONExample)
		}
		if len(item.InstallExtraPackages) > 0 {
			fmt.Fprintf(output, "  Extras: %s\n", strings.Join(item.InstallExtraPackages, ", "))
		}
		if len(item.ExpectedToolNames) > 0 {
			fmt.Fprintf(output, "  Tools: %s\n", strings.Join(item.ExpectedToolNames, ", "))
		}
		if len(item.ExpectedSkillNames) > 0 {
			fmt.Fprintf(output, "  Skills: %s\n", strings.Join(item.ExpectedSkillNames, ", "))
		}
		if len(item.ExpectedDiagnosticCodes) > 0 {
			fmt.Fprintf(output, "  Diagnostics: %s\n", strings.Join(item.ExpectedDiagnosticCodes, ", "))
		}
		for _, guidance := range item.Guidance {
			fmt.Fprintf(output, "  Note: %s\n", guidance)
		}
	}

	return 0
}

func getOptionValue(args []string, optionName string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == optionName {
			val := args[i+1]
			return val
		}
	}
	return ""
}

func containsOption(args []string, optionName string) bool {
	return slices.Contains(args, optionName)
}

func unknownSubcommand(subcommand string, output, errWriter io.Writer) int {
	fmt.Fprintf(errWriter, "Unknown compatibility subcommand: %s\n", subcommand)
	printHelp(output)
	return 2
}

func printHelp(output io.Writer) {
	helpText := `    openclaw compatibility

    Usage:
      openclaw compatibility catalog [--status <compatible|incompatible>] [--kind <kind>] [--category <category>] [--json]
      openclaw compat catalog [--status <compatible|incompatible>] [--kind <kind>] [--category <category>] [--json]

    Notes:
      - The catalog is backed by the pinned public compatibility smoke manifest shipped with OpenClaw.NET.
      - Use --json when you want machine-readable output.
`
	fmt.Fprint(output, helpText)
}
