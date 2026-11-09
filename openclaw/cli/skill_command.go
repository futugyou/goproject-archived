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
	case "install":
		return skillCommandInstall(rest)
	case "list", "ls":
		return skillCommandListInstalled(rest)
	case "catalog":
		return skillCommandListCatalog(rest)
	case "create":
		return skillCommandCreateSkillScaffold(rest)
	default:
		return skillCommandUnknownSubcommand(subcommand, asJson)
	}
}

type ProposalDraftQuality struct {
	ChecksPassed int
	ChecksTotal  int
	Checks       []ProposalDraftQualityCheck
	Warnings     []string
}

type ProposalDraftQualityCheck struct {
	Id             string
	Status         string
	Message        string
	Recommendation string
}

func buildProposalDraftQuality(name, description, kind string) ProposalDraftQuality {
	checks := []ProposalDraftQualityCheck{}
	warnings := []string{}

	hasName := name != ""
	check := ProposalDraftQualityCheck{
		Id:             "name_present",
		Status:         "fail",
		Message:        "Skill name is empty.",
		Recommendation: "Provide a non-empty skill name.",
	}
	if hasName {
		check.Status = "pass"
		check.Message = "Skill name is present."
		check.Recommendation = ""
	}

	checks = append(checks, check)

	hasDescription := description != ""
	descriptionStatus := "fail"
	descriptionMessage := "Skill description is empty."
	descriptionRecommendation := "Add a meaningful description that explains intent and outcomes."
	if hasDescription {
		descriptionStatus = "pass"
		descriptionMessage = "Skill description is present."
		descriptionRecommendation = ""
	}

	if hasDescription && len(description) < 16 {
		descriptionStatus = "warn"
		descriptionMessage = "Skill description is present but too short to guide proposal review."
		descriptionRecommendation = "Expand description to include expected behavior and boundaries."
		warnings = append(warnings, "description_too_short")
	}

	checks = append(checks, ProposalDraftQualityCheck{
		Id:             "description_present",
		Status:         descriptionStatus,
		Message:        descriptionMessage,
		Recommendation: descriptionRecommendation,
	})

	check = ProposalDraftQualityCheck{
		Id:             "meta_composition_seeded",
		Status:         "fail",
		Message:        "Meta composition scaffold is missing.",
		Recommendation: "Use --kind meta to seed composition scaffolding.",
	}
	if kind == "meta" {
		check.Status = "pass"
		check.Message = "Meta composition scaffold is seeded."
		check.Recommendation = ""
	}

	checks = append(checks, check)
	checksPassed := 0
	for _, v := range checks {
		if v.Status == "pass" {
			checksPassed++
		}
	}
	return ProposalDraftQuality{
		ChecksPassed: checksPassed,
		ChecksTotal:  len(checks),
		Checks:       checks,
		Warnings:     warnings,
	}
}

func isProposalDraftQualityBlocked(quality ProposalDraftQuality) bool {
	for _, check := range quality.Checks {
		if check.Status != "pass" {
			return true
		}
	}

	return false
}

func writeCreateProposalDraftQualityError(asJson bool, quality ProposalDraftQuality) string {
	var blockingChecks []ProposalDraftQualityCheck
	for _, check := range quality.Checks {
		if check.Status != "pass" {
			blockingChecks = append(blockingChecks, check)
		}
	}

	message := fmt.Sprintf("Proposal draft quality gate failed: %d/%d checks passed.", quality.ChecksPassed, quality.ChecksTotal)

	if !asJson {
		fmt.Fprintln(os.Stderr, message)
		for _, check := range blockingChecks {
			fmt.Fprintf(os.Stderr, "  - %s [%s]: %s\n", check.Id, check.Status, check.Message)
		}
		return message
	}

	blockingChecksList := make([]map[string]any, len(blockingChecks))
	for i, check := range blockingChecks {
		bc := map[string]any{
			"id":      check.Id,
			"status":  check.Status,
			"message": check.Message,
		}
		if check.Recommendation != "" {
			bc["recommendation"] = check.Recommendation
		}
		blockingChecksList[i] = bc
	}

	data := map[string]any{
		"status":    "error",
		"command":   "skills create",
		"errorCode": "proposal_draft_quality_gate_failed",
		"message":   message,
		"quality": map[string]any{
			"checksPassed":        quality.ChecksPassed,
			"checksTotal":         quality.ChecksTotal,
			"minimumChecksPassed": quality.ChecksTotal,
			"blockingChecks":      blockingChecksList,
			"warnings":            quality.Warnings,
		},
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to marshal error JSON:", err)
		return err.Error()
	}

	msg := string(jsonBytes)
	fmt.Fprintln(os.Stderr, msg)
	return msg
}

func writeCreateError(asJson bool, errorCode string, message string) {
	if !asJson {
		fmt.Fprintln(os.Stderr, message)
		return
	}

	data := map[string]any{
		"status":    "error",
		"command":   "skills create",
		"errorCode": errorCode,
		"message":   message,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to marshal error JSON:", err)
		return
	}

	fmt.Fprintln(os.Stderr, string(jsonBytes))
}

func buildSkillScaffoldMarkdown(name, description, kind string) string {
	lines := []string{
		"---",
		fmt.Sprintf("name: %s", name),
		fmt.Sprintf("description: %s", description),
	}

	if kind == "meta" {
		lines = append(lines, "kind: meta")
		lines = append(lines, "composition: {\"steps\":[{\"id\":\"draft\",\"kind\":\"llm_chat\",\"with\":{\"prompt\":\"Summarize the user intent and propose next steps.\"}}]}")
	}

	lines = append(lines, "---")
	lines = append(lines, "")
	lines = append(lines, "Describe what this skill should do and how it should be used.")

	return strings.Join(lines, "\n") + "\n"
}

func skillCommandCreateSkillScaffold(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	force := slices.Contains(args, "--force")
	proposalDraftRequested := slices.Contains(args, "--proposal-draft")
	managed := slices.Contains(args, "--managed")
	workdir := getOptionValue(args, "--workdir")
	name := ""
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			name = args[i]
			break
		}
	}
	if name == "" {
		msg := "Usage: openclaw skills create <name> [--kind <standard|meta>] [--description <text>] [--proposal-draft] [--workdir <path> | --managed] [--json] [--force]"
		writeCreateError(asJson, "invalid_create_usage", msg)
		return 2, errors.New(msg)
	}

	kind := getOptionValue(args, "--kind")
	if kind == "" {
		kind = "standard"
	}
	if kind != "standard" && kind != "meta" {
		writeCreateError(asJson, "invalid_kind", "--kind must be one of: standard, meta.")
		return 2, errors.New("--kind must be one of: standard, meta.")
	}

	normalizeSingleLineValue := func(value string) string {

		replacer := strings.NewReplacer(
			"\r", " ",
			"\n", " ",
		)
		return replacer.Replace(value)
	}

	name = normalizeSingleLineValue(name)
	if name == "" {
		writeCreateError(asJson, "invalid_skill_name", "Skill name cannot be empty.")
		return 2, errors.New("Skill name cannot be empty.")
	}

	slug := slugify(name)
	if slug == "" {
		writeCreateError(asJson, "invalid_skill_name", "Skill name does not contain any letters or digits.")
		return 2, errors.New("Skill name does not contain any letters or digits.")
	}

	description := getOptionValue(args, "--description")
	if description == "" {
		description = fmt.Sprintf("%s workflow scaffold.", name)
	}
	description = normalizeSingleLineValue(description)
	skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}

	skillDirectory := filepath.Join(skillsDirectory, slug)
	exists := util.DirectoryExists(skillDirectory)
	if exists && !force {
		msg := fmt.Sprintf("Skill scaffold already exists: %s. Use --force to overwrite SKILL.md.", skillDirectory)
		writeCreateError(asJson, "skill_already_exists", msg)
		return 1, errors.New(msg)
	}

	kindValue := "standard"
	if kind == "meta" {
		kindValue = "meta"
	}

	if proposalDraftRequested && kindValue != "meta" {
		writeCreateError(asJson, "invalid_proposal_draft_kind", "--proposal-draft is only supported for --kind meta.")
		return 2, errors.New("--proposal-draft is only supported for --kind meta.")
	}

	var proposalDraftQuality *ProposalDraftQuality
	if proposalDraftRequested {
		proposalDraftQuality = new(buildProposalDraftQuality(name, description, kindValue))
		if isProposalDraftQualityBlocked(*proposalDraftQuality) {
			msg := writeCreateProposalDraftQualityError(asJson, *proposalDraftQuality)
			return 1, errors.New(msg)
		}
	}

	os.MkdirAll(skillDirectory, 0755)

	skillMarkdown := buildSkillScaffoldMarkdown(name, description, kindValue)
	os.WriteFile(filepath.Join(skillDirectory, "SKILL.md"), []byte(skillMarkdown), 0644)
	proposalDraftId := slug
	proposalDraftKind := "meta_skill_creator_draft"
	proposalDraftStatus := "draft"
	proposalDraftTitle := fmt.Sprintf("Meta skill draft proposal: %s", name)
	proposalDraftSummary := "Draft proposal prepared from scaffold metadata. Review and refine before lifecycle actions."

	if asJson {
		response := map[string]any{
			"name":      name,
			"slug":      slug,
			"kind":      kindValue,
			"path":      skillDirectory,
			"created":   true,
			"overwrote": exists,
		}

		if proposalDraftRequested {
			if proposalDraftQuality == nil {
				return 1, errors.New("Proposal draft quality must be available when proposal draft is requested.")
			}
			var checks []map[string]any
			for _, check := range proposalDraftQuality.Checks {
				c := map[string]any{
					"id":      check.Id,
					"status":  check.Status,
					"message": check.Message,
				}
				if strings.TrimSpace(check.Recommendation) != "" {
					c["recommendation"] = check.Recommendation
				}
				checks = append(checks, c)
			}

			response["proposalDraft"] = map[string]any{
				"available": true,
				"id":        proposalDraftId,
				"kind":      proposalDraftKind,
				"status":    proposalDraftStatus,
				"title":     proposalDraftTitle,
				"summary":   proposalDraftSummary,
				"quality": map[string]any{
					"checksPassed":        proposalDraftQuality.ChecksPassed,
					"checksTotal":         proposalDraftQuality.ChecksTotal,
					"minimumChecksPassed": proposalDraftQuality.ChecksTotal,
					"checks":              checks,
					"warnings":            proposalDraftQuality.Warnings,
				},
			}
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			panic(err)
		}

		fmt.Println(string(jsonData))
		return 0, nil
	}

	fmt.Printf("Created skill scaffold: %s [%s]\n", name, kindValue)
	fmt.Printf("Path: %s\n", skillDirectory)
	if exists {
		fmt.Println("Overwrote existing SKILL.md via --force.")
	}

	if proposalDraftRequested {
		if proposalDraftQuality == nil {
			return 1, errors.New("Proposal draft quality must be available when proposal draft is requested.")
		}

		fmt.Printf("Proposal draft: %s\n", proposalDraftStatus)
		fmt.Printf("Proposal kind: %s\n", proposalDraftKind)
		fmt.Printf("Proposal id: %s\n", proposalDraftId)
		fmt.Printf("Proposal quality: %d/%d checks passed\n", proposalDraftQuality.ChecksPassed, proposalDraftQuality.ChecksTotal)
	}

	return 0, nil
}

func skillCommandListCatalog(args []string) (int, error) {
	managed := slices.Contains(args, "--managed")
	stable := slices.Contains(args, "--stable")
	workdir := getOptionValue(args, "--workdir")
	asJson := slices.Contains(args, "--json")
	kind := getOptionValue(args, "--kind")

	if kind != "" && kind != "all" && kind != "meta" {
		writeSkillsCommandError(asJson, "skills catalog", "invalid_kind", "--kind supports only 'all' or 'meta'.")
		return 2, errors.New("--kind supports only 'all' or 'meta'.")
	}

	catalog := []SkillCommandInspection{}
	var skillsRootPath string
	var source core.SkillSource
	if stable {
		bundledDir := filepath.Join(".", "skills")
		if !util.DirectoryExists(bundledDir) {
			if asJson {
				fmt.Println("{\"count\":0,\"skills\":[]}")
				return 0, nil
			}

			fmt.Println("Stable meta catalog (0):")
			return 0, nil
		}
		skillsRootPath = bundledDir
		source = core.SkillSourceBundled
	} else {
		skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
		if err != nil {
			return 1, err
		}
		if !util.DirectoryExists(skillsDirectory) {
			if asJson {
				fmt.Println("{\"count\":0,\"skills\":[]}")
				return 0, nil
			}

			fmt.Println("No skills installed.")
			return 0, nil
		}

		skillsRootPath = skillsDirectory
	}

	inspectionResults := core.SkillInspectorInstance.InspectInstalledRoot(skillsRootPath, source)
	for _, v := range inspectionResults {
		if v.Success && v.Definition != nil {
			inspection := createInspection(&v)
			if kind == "" || kind == "all" || (kind == "meta" && inspection.Definition.Kind == core.SkillKind_Meta) {
				catalog = append(catalog, inspection)
			}
		}
	}

	slices.SortFunc(catalog, func(a, b SkillCommandInspection) int {
		return strings.Compare(a.Definition.Name, b.Definition.Name)
	})

	if asJson {
		type innerDtoItem struct {
			Name        string `json:"name"`
			Kind        string `json:"kind"`
			Description string `json:"description"`
			Trust       string `json:"trust"`
			Source      string `json:"source"`
			Path        string `json:"path"`
		}
		type innerDto struct {
			Count  int            `json:"count"`
			Skills []innerDtoItem `json:"skill"`
		}
		dto := innerDto{
			Count: len(catalog),
		}
		for _, item := range catalog {
			dto.Skills = append(dto.Skills, innerDtoItem{
				Name:        item.Definition.Name,
				Kind:        strings.ToLower(item.Definition.Kind.ToString()),
				Description: item.Definition.Description,
				Trust:       item.TrustLevel,
				Source:      item.SourceLabel,
				Path:        item.SkillRootPath,
			})
		}

		raw, _ := json.Marshal(dto)
		fmt.Println(string(raw))
		return 0, nil
	}

	header := "Skill catalog"
	if stable {
		header = "Stable meta catalog"
	}

	fmt.Printf("%s (%d):\n", header, len(catalog))
	for _, item := range catalog {
		fmt.Printf("  %s [%s] - %s\n", item.Definition.Name, strings.ToLower(item.Definition.Kind.ToString()), item.Definition.Description)
		fmt.Printf("    Trust: %s\n", item.TrustLevel)
		fmt.Printf("    Source: %s\n", item.SourceLabel)
		fmt.Printf("    Path: %s\n", item.SkillRootPath)
	}

	return 0, nil
}

func skillCommandListInstalled(args []string) (int, error) {
	managed := slices.Contains(args, "--managed")
	workdir := getOptionValue(args, "--workdir")
	skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}

	if !util.DirectoryExists(skillsDirectory) {
		fmt.Println("No skills installed.")
		return 0, nil
	}

	source := core.SkillSourceWorkspace
	if managed {
		source = core.SkillSourceManaged
	}
	inspectionResults := core.SkillInspectorInstance.InspectInstalledRoot(skillsDirectory, source)

	inspections := []SkillCommandInspection{}
	for _, inspection := range inspectionResults {
		if inspection.Success && inspection.Definition != nil {
			inspections = append(inspections, createInspection(&inspection))
		}
	}

	slices.SortFunc(inspections, func(a, b SkillCommandInspection) int {
		return strings.Compare(a.Definition.Name, b.Definition.Name)
	})

	if len(inspections) == 0 {
		fmt.Println("No skills installed.")
		return 0, nil
	}

	fmt.Printf("Installed skills (%d):\n", len(inspections))
	for i := 0; i < len(inspections); i++ {
		inspection := inspections[i]
		fmt.Printf("  %s - %s\n", inspection.Definition.Name, inspection.Definition.Description)
		fmt.Printf("    Trust: %s\n", inspection.TrustLevel)
		fmt.Printf("    Source: %s\n", inspection.SourceLabel)
		fmt.Printf("    Path: %s\n", inspection.SkillRootPath)
	}

	return 0, nil
}

func skillCommandInstall(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	sourcePath := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sourcePath = args[i]
			break
		}
	}

	if sourcePath == "" {
		writeSkillsCommandError(asJson, "skills install", "missing_source_path", "Usage: openclaw skills install <path|tarball>")
		return 2, errors.New("Usage: openclaw skills install <path|tarball>")
	}

	dryRun := slices.Contains(args, "--dry-run")
	managed := slices.Contains(args, "--managed")
	workdir := getOptionValue(args, "--workdir")

	inspected, tempDirectory := skillCommandInspectResolvedSource(sourcePath, true)
	if !inspected.Success {
		msg := inspected.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("Failed to inspect source path: %s", sourcePath)
		}
		writeSkillsCommandError(asJson, "skills install", "install_failed", msg)
		return 1, errors.New(msg)
	}

	if tempDirectory != "" {
		defer os.RemoveAll(tempDirectory)
	}

	path, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}
	skillCommandPrintInspection(inspected, path)
	if dryRun {
		return 0, nil
	}

	skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}

	os.MkdirAll(skillsDirectory, 0755)

	targetDir := filepath.Join(skillsDirectory, inspected.InstallSlug)
	if util.DirectoryExists(targetDir) {
		os.RemoveAll(targetDir)
	}

	if err := util.CopyDirectoryWithoutSymlink(inspected.SkillRootPath, targetDir); err != nil {
		writeSkillsCommandError(asJson, "skills install", "install_failed", err.Error())
		return 1, err
	}
	fmt.Printf("Installed skill '%s' to %s\n", inspected.Definition.Name, targetDir)
	return 0, nil
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
		inspection := core.SkillInspectorInstance.InspectPath(resolvedSourcePath, core.SkillSourceExtra)
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
		defer os.RemoveAll(tempDir)

		result := util.RunProcess(context.Background(), "tar", []string{"--extract", "--gzip", "--file", resolvedSourcePath, "--directory", tempDir}, tempDir, 0, 0, 0)
		path := ""
		if retainExtractedDirectory {
			path = tempDir
		}
		if result.ExitCode != 0 {
			return SkillCommandInspectionFailure(fmt.Sprintf("Failed to extract skill tarball: %s", result.Error)), path
		}

		inspection := core.SkillInspectorInstance.InspectPath(tempDir, core.SkillSourceExtra)
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
