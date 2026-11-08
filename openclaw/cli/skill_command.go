package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type SkillCommandInspection struct {
	Success       bool
	ErrorMessage  string
	Definition    core.SkillDefinition
	SkillRootPath string
	SkillFilePath string
	InstallSlug   string
	SourceLabel   string
	TrustLevel    string
	TrustReason   string
	Warnings      []string
}

func SkillCommandInspectionFailure(errorMessage string) SkillCommandInspection {
	return SkillCommandInspection{
		ErrorMessage: errorMessage,
		Definition: core.SkillDefinition{
			Source: core.SkillSourceExtra,
		},
		TrustLevel: "untrusted",
	}
}

const DefaultMetaRunProposalRollbackReason = "meta_run_proposal_operator_rollback"

func writeSkillsCommandError(asJson bool, command, errorCode, message string) {
	if !asJson {
		fmt.Println(message)
		return
	}

	data := map[string]string{}
	data["status"] = "error"
	data["command"] = command
	data["errorCode"] = errorCode
	data["message"] = message
	msg, _ := json.Marshal(data)

	fmt.Println([]byte(msg))
}

func skillCommandUnknownSubcommand(subcommand string, asJson bool) (int, error) {
	msg := fmt.Sprintf("Unknown skills subcommand: %s", subcommand)
	writeSkillsCommandError(asJson, "skills", "unknown_subcommand", msg)
	if !asJson {
		skillCommandPrintHelp()
	}
	return 2, errors.New(msg)
}

func skillCommandPrintHelp() {
	fmt.Println(`
openclaw skills — Inspect and install local OpenClaw skill packages

            Usage:
              openclaw skills inspect <path|tarball>
              openclaw skills install <path|tarball> [--dry-run] [--workdir <path> | --managed]
              openclaw skills list [--workdir <path> | --managed]
              openclaw skills catalog [--workdir <path> | --managed] [--kind <all|meta>] [--stable] [--json]
              openclaw skills create <name> [--kind <standard|meta>] [--description <text>] [--proposal-draft] [--workdir <path> | --managed] [--json] [--force]
              openclaw skills proposals <session-id> [--run <run-id>] [--storage <path>] [--json]
              openclaw skills proposals show <session-id> --proposal <id> [--storage <path>] [--json]
                            openclaw skills meta-runs list [--page <n>] [--page-size <n>] [--storage <path>] [--json]
                            openclaw skills meta-runs show <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs steps <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs failures [--since <5m|24h|7d>] [--name <skill>] [--limit <n>] [--storage <path>] [--json]
                            openclaw skills meta-runs <session-id> [--storage <path>] [--limit <count>] [--run <run-id>] [--verbose] [--json]
                            openclaw skills meta-runs replay <session-id> --run <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs reconstruct <session-id> --run <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs proposals <session-id> [--run <run-id>] [--storage <path>] [--json]
                            openclaw skills meta-runs proposals show <session-id> --proposal <id> [--storage <path>] [--json]
                            openclaw skills meta-runs proposals accept <session-id> --proposal <id> [--storage <path>] [--json]
                            openclaw skills meta-runs proposals dismiss <session-id> --proposal <id> [--reason <text>] [--storage <path>] [--json]
                            openclaw skills meta-runs proposals rollback <session-id> --proposal <id> [--reason <text>] [--storage <path>] [--json]
                            openclaw skills meta-runs proposals change <session-id> --proposal <id> --to <accept|dismiss> [--reason <text>] [--storage <path>] [--json]

            Notes:
              - Remote registry installs still go through "openclaw clawhub".
              - "install --dry-run" prints trust and requirement details without copying files.
                            - "meta-runs" reads persisted local session state from "./memory", " OPENCLAW_WORKSPACE/memory", or "--storage".
                            - "meta-runs --run <run-id>" limits output to one persisted run inside the session history.
                            - "meta-runs --verbose" expands per-step trace summaries for each persisted run.
                            - "meta-runs --json" emits machine-readable output for operators and scripts.
                            - "meta-runs steps" shows persisted per-step execution details for one run id.
                            - "meta-runs failures" supports "--since" and "--name" filters for operator triage windows.
                            - "meta-runs replay" is currently preview-only and reports whether persisted run history is sufficient for replay.
                                - "meta-runs reconstruct" builds an audit replay result from persisted run history and optional checkpoint state without re-executing tools or models.
                                - "meta-runs proposals" returns derived read-only proposal summaries from persisted meta-run evidence.
                                - "skills proposals" is a read-only shortcut to "skills meta-runs proposals" / "show".
                                - "skills catalog" lists installed skills for discovery and supports "--kind meta" filtering.
                                - "skills catalog --stable --kind meta" limits discovery to bundled first-party meta skills.
                                - "meta-runs proposals show" expands a single derived proposal without implying durable lifecycle state.
                                - "meta-runs proposals accept|dismiss|rollback|change" records lifecycle decisions only; it does not execute tools, models, or replay.`)
}

func SkillCommandRun(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		skillCommandPrintHelp()
		return 0, nil
	}

	subcommand := args[0]
	rest := []string{}
	if len(args) > 1 {
		rest = args[1:]
	}

	switch subcommand {
	case "inspect":
		return skillCommandInspect(rest)
	default:
		return skillCommandUnknownSubcommand(subcommand, asJson)
	}
}

func skillCommandInspect(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	sourcePath := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sourcePath = args[i]
			break
		}
	}

	if sourcePath == "" {
		writeSkillsCommandError(asJson, "skills inspect", "missing_source_path", "Usage: openclaw skills inspect <path|tarball>")
		return 2, errors.New("Usage: openclaw skills inspect <path|tarball>")
	}

	return skillCommandInspectSource(sourcePath, false, asJson)
}

func buildRequirementsSummary(definition core.SkillDefinition) string {
	items := []string{}
	if len(definition.Metadata.RequireBins) > 0 {
		items = append(items, "bins=%s", strings.Join(definition.Metadata.RequireBins, ","))
	}
	if len(definition.Metadata.RequireAnyBins) > 0 {
		items = append(items, "anyBins=%s", strings.Join(definition.Metadata.RequireAnyBins, ","))
	}
	if len(definition.Metadata.RequireEnv) > 0 {
		items = append(items, "env=%s", strings.Join(definition.Metadata.RequireEnv, ","))
	}
	if len(definition.Metadata.RequireConfig) > 0 {
		items = append(items, "config=%s", strings.Join(definition.Metadata.RequireConfig, ","))
	}
	if definition.Metadata.Always {
		items = append(items, "always")
	}

	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, " | ")
}

func skillCommandPrintInspection(inspection SkillCommandInspection, installDirectory string) {
	fmt.Printf("Skill: %s\n", inspection.Definition.Name)
	fmt.Printf("Description: %s\n", inspection.Definition.Description)
	fmt.Printf("Trust: %s\n", inspection.TrustLevel)
	fmt.Printf("Trust reason: %s\n", inspection.TrustReason)
	fmt.Printf("Source: %s\n", inspection.SourceLabel)
	fmt.Printf("Path: %s\n", inspection.SkillRootPath)
	fmt.Printf("Install slug: %s\n", inspection.InstallSlug)
	fmt.Printf("User invocable: %t\n", inspection.Definition.UserInvocable)
	fmt.Printf("Disable model invocation: %t\n", inspection.Definition.DisableModelInvocation)
	if inspection.Definition.CommandDispatch != "" {
		fmt.Printf("Command dispatch: %s\n", inspection.Definition.CommandDispatch)
	}
	if inspection.Definition.CommandTool != "" {
		fmt.Printf("Command tool: %s\n", inspection.Definition.CommandTool)
	}
	if inspection.Definition.CommandArgMode != "" {
		fmt.Printf("Command arg mode: %s\n", inspection.Definition.CommandArgMode)
	}
	if inspection.Definition.Metadata.Homepage != "" {
		fmt.Printf("Homepage: %s\n", inspection.Definition.Metadata.Homepage)
	}
	if inspection.Definition.Metadata.PrimaryEnv != "" {
		fmt.Printf("Primary env: %s\n", inspection.Definition.Metadata.PrimaryEnv)
	}
	fmt.Printf("Requirements: %s\n", buildRequirementsSummary(inspection.Definition))
	for _, warning := range inspection.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}

	if installDirectory != "" {
		fmt.Printf("Install target: %s\n", filepath.Join(installDirectory, inspection.InstallSlug))
	}
}

func resolveSkillsDirectory(managed bool, workdir string) (string, error) {
	if workdir != "" {
		path, err := filepath.Abs(workdir)
		if err != nil {
			return "", fmt.Errorf("Missing %s. Set %s or pass --workdir or --managed.", envWorkspace, envWorkspace)
		}
		return filepath.Join(path, "skills"), nil
	}

	if managed {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".openclaw", "skills"), nil
	}

	workspace := os.Getenv(envWorkspace)
	if workspace != "" {
		path, err := filepath.Abs(workspace)
		if err != nil {
			return "", fmt.Errorf("Missing %s. Set %s or pass --workdir or --managed.", envWorkspace, envWorkspace)
		}
		return filepath.Join(path, "skills"), nil
	}

	return "", fmt.Errorf("Missing %s. Set %s or pass --workdir or --managed.", envWorkspace, envWorkspace)
}

func skillCommandInspectResolvedSource(sourcePath string, retainExtractedDirectory bool) (SkillCommandInspection, string) {
	resolvedSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return SkillCommandInspectionFailure(err.Error()), ""
	}

	if util.DirectoryExists(resolvedSourcePath) {
		inspection := core.SkillInspectorInstace.InspectPath(resolvedSourcePath, core.SkillSourceExtra)
		if inspection.Success {
			return createInspection(inspection), ""
		}

		msg := inspection.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("Failed to inspect %s", resolvedSourcePath)
		}
		return SkillCommandInspectionFailure(msg), ""
	}

	if util.FileExists(resolvedSourcePath) && strings.HasSuffix(resolvedSourcePath, ".tgz") {
		tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("openclaw-skill-install-%s", util.CleanUUID())[:24])
		os.MkdirAll(tempDir, 0755)
		defer os.Remove(tempDir)

		result := util.RunProcess(context.Background(), "tar", []string{"--extract", "--gzip", "--file", resolvedSourcePath, "--directory", tempDir}, tempDir, 0, 0, 0)
		path := ""
		if retainExtractedDirectory {
			path = tempDir
		}
		if result.ExitCode != 0 {
			return SkillCommandInspectionFailure(fmt.Sprintf("Failed to extract skill tarball: %s", result.Error)), path
		}

		inspection := core.SkillInspectorInstace.InspectPath(tempDir, core.SkillSourceExtra)
		if !inspection.Success {
			msg := inspection.ErrorMessage
			if msg == "" {
				msg = fmt.Sprintf("Failed to inspect extracted tarball %s", resolvedSourcePath)
			}
			return SkillCommandInspectionFailure(msg), ""
		}

		return createInspection(inspection), path
	}

	return SkillCommandInspectionFailure(fmt.Sprintf("Skill path not found: %s", sourcePath)), ""
}

func slugify(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, value)

	fields := strings.FieldsFunc(mapped, func(r rune) bool {
		return r == '-'
	})

	return strings.Join(fields, "-")
}

func createInspection(inspection *core.SkillInspectionResult) SkillCommandInspection {
	definition := inspection.Definition
	install := definition.Metadata.SkillKey
	if install == "" {
		install = definition.Name
	}
	installSlug := slugify(install)
	trustLevel := "upstream-compatible"
	if definition.Source == core.SkillSourceBundled {
		trustLevel = "first-party"
	}
	trustReason := "Skill document parsed successfully and uses the OpenClaw skill format."
	if definition.Source == core.SkillSourceBundled {
		trustReason = "Skill ships with OpenClaw."
	}

	warnings := []string{}
	if definition.Description != "" {
		warnings = append(warnings, "Skill description is empty.")
	}
	if len(definition.Metadata.RequireBins) == 0 &&
		len(definition.Metadata.RequireAnyBins) == 0 &&
		len(definition.Metadata.RequireEnv) == 0 &&
		len(definition.Metadata.RequireConfig) == 0 {
		warnings = append(warnings, "Skill does not declare host requirements.")
	}

	return SkillCommandInspection{
		Success:       true,
		Definition:    *definition,
		SkillRootPath: inspection.SkillRootPath,
		SkillFilePath: inspection.SkillFilePath,
		InstallSlug:   installSlug,
		SourceLabel:   strings.ToLower(definition.Source.ToString()),
		TrustLevel:    trustLevel,
		TrustReason:   trustReason,
		Warnings:      warnings,
	}
}

func skillCommandInspectSource(sourcePath string, printInstallTarget bool, asJson bool) (int, error) {
	inspected, _ := skillCommandInspectResolvedSource(sourcePath, false)
	if !inspected.Success {
		msg := inspected.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("Failed to inspect source path: %s", sourcePath)
		}
		writeSkillsCommandError(asJson, "skills inspect", "inspect_failed", msg)
		return 1, errors.New(msg)
	}

	installDirectory := ""
	if printInstallTarget {
		path, err := resolveSkillsDirectory(false, "")
		if err != nil {
			return 1, err
		}
		installDirectory = path
	}

	skillCommandPrintInspection(inspected, installDirectory)
	return 0, nil
}
