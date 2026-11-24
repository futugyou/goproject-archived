package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

func UpgradeCommandRun(args []string, output io.Writer, errWriter io.Writer) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		upgradeCommandPrintHelp(output)
		return 0, nil
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	rest := args[1:]
	switch subcommand {
	case "check":
		return upgradeCommandRunCheck(rest, output, errWriter)
	case "rollback":
		return upgradeCommandRunRollback(rest, output, errWriter)
	default:
		msg := fmt.Sprintf("Unknown upgrade subcommand: %s", subcommand)
		upgradeCommandPrintHelp(output)
		return 2, errors.New(msg)
	}
}

func buildRollbackVerifyArgs(configPath string, parsed *CliArgs, snapshot *core.UpgradeRollbackSnapshot) []string {
	args := []string{
		"--config",
		configPath,
	}

	if parsed.HasFlag("--offline") || snapshot.Offline {
		args = append(args, "--offline")
	}
	if parsed.HasFlag("--require-provider") || snapshot.RequireProvider {
		args = append(args, "--require-provider")
	}

	return args
}

func upgradeCommandRunRollback(args []string, output, errWriter io.Writer) (int, error) {
	parsed := CliArgsParse(args)
	if parsed.ShowHelp {
		upgradeCommandPrintHelp(output)
		return 0, nil
	}

	configPath, err := resolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	quotedConfigPath := core.GatewaySetupPathsIntance.QuoteIfNeeded(configPath)
	store := core.NewUpgradeRollbackSnapshotStore(configPath)
	snapshot, err := store.Load()
	if err != nil {
		fmt.Fprintf(errWriter, "%s Delete the broken snapshot and rerun 'openclaw upgrade check --config %s' before upgrading.\n", err.Error(), quotedConfigPath)
		return 1, err
	}

	if err := restoreSnapshot(snapshot, store, configPath); err != nil {
		fmt.Fprintf(errWriter, "Failed to restore rollback snapshot: %s\n", err.Error())
		return 1, err
	}

	fmt.Fprintln(output, "Restored last-known-good setup snapshot.")
	fmt.Fprintf(output, "Config: %s\n", configPath)
	fmt.Fprintf(output, "Captured at: %s\n", snapshot.CreatedAtUtc.Format("20060102150405"))
	fmt.Fprintf(output, "Captured by: openclaw %s\n", snapshot.CreatedByVersion)
	fmt.Fprintf(output, "Snapshot directory: %s\n", store.SnapshotDirectory())

	currentVersion := "0.0.0"
	if !strings.EqualFold(snapshot.CreatedByVersion, currentVersion) {
		fmt.Fprintf(output, "Current CLI version: openclaw %s\n", currentVersion)
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Re-running setup verification on the restored files...")

	verifyExitCode, err := SetupLifecycleCommandRunVerify(buildRollbackVerifyArgs(configPath, parsed, snapshot), output, errWriter)
	if verifyExitCode == 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Rollback completed successfully.")
	}

	return verifyExitCode, err
}

type UpgradeCheckSummary struct {
	Label       string
	Status      string
	Summary     string
	Details     []string
	NextActions []string
}

type RollbackCaptureItem struct {
	Kind                 string
	SourcePath           string
	TargetPath           string
	IsDirectory          bool
	Exists               bool
	SnapshotRelativePath string
}

type ValidatedRestoreArtifact struct {
	Kind                 string
	TargetPath           string
	IsDirectory          bool
	Exists               bool
	SnapshotRelativePath string
}

type UnixModeCopyStrategy string

var UnixModeCopyStrategyPreserveSource UnixModeCopyStrategy = "PreserveSource"
var UnixModeCopyStrategyOwnerOnlyPreserveExecute UnixModeCopyStrategy = "OwnerOnlyPreserveExecute"

func normalizeSeverity(severity string) string {
	if severity == "error" {
		return core.SetupCheckStatesFail
	}

	return core.SetupCheckStatesWarn
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case core.SetupCheckStatesFail:
		return core.SetupCheckStatesFail
	case core.SetupCheckStatesWarn:
		return core.SetupCheckStatesWarn
	case core.SetupCheckStatesSkip:
		return core.SetupCheckStatesSkip
	default:
		return core.SetupCheckStatesPass
	}
}

func aggregateStatus(statuses []string) string {
	normalized := []string{}
	for _, status := range statuses {
		normalized = append(normalized, normalizeStatus(status))
	}
	if slices.Contains(normalized, core.SetupCheckStatesFail) {
		return core.SetupCheckStatesFail
	}
	if slices.Contains(normalized, core.SetupCheckStatesWarn) || slices.Contains(normalized, core.SetupCheckStatesSkip) {
		return core.SetupCheckStatesWarn
	}

	return core.SetupCheckStatesPass
}

func evaluateVerification(config *core.GatewayConfig, workspacePath string, offline bool) *UpgradeCheckSummary {
	localState := core.LocalSetupStateLoaderInstance.Load(config.Memory.StoragePath)

	verification, err := core.SetupVerificationServiceInstance.Verify(context.Background(), &core.SetupVerificationRequest{
		Config:               config,
		Policy:               localState.Policy,
		OperatorAccountCount: localState.OperatorAccountCount,
		Offline:              offline,
		RequireProvider:      !offline,
		WorkspacePath:        workspacePath,
		ModelDoctor:          core.ModelDoctorEvaluatorInstance.Build(config, nil, nil),
	}, nil)

	if err != nil {
		return &UpgradeCheckSummary{
			Label:       "Config and provider readiness",
			Status:      core.SetupCheckStatesFail,
			Summary:     err.Error(),
			Details:     []string{err.Error()},
			NextActions: []string{"SetupVerification Verify error "}}
	}

	detail := []string{}
	for _, check := range verification.Checks {
		if check.Status != core.SetupCheckStatesPass {
			detail = append(detail, fmt.Sprintf("[%s] %s: %s", check.Status, check.Label, check.Summary))
		}
	}

	summary := fmt.Sprintf("Setup verification reported %d checks with overall status '%s'.", len(verification.Checks), verification.OverallStatus)
	if offline {
		summary += " Provider smoke was skipped because offline mode is enabled."
	}

	return &UpgradeCheckSummary{
		Label:       "Config and provider readiness",
		Status:      verification.OverallStatus,
		Summary:     summary,
		Details:     detail,
		NextActions: verification.RecommendedNextActions}
}

func evaluatePlugins(config *core.GatewayConfig, workspacePath string) *UpgradeCheckSummary {
	result := core.PluginDiscoveryInstance.DiscoverWithDiagnostics(&config.Plugins, workspacePath)
	details := []string{}
	nextActions := []string{}
	status := core.SetupCheckStatesPass

	for _, report := range result.Reports {
		for _, diagnostic := range report.Diagnostics {
			severity := normalizeSeverity(diagnostic.Severity)
			status = aggregateStatus([]string{status, severity})
			details = append(details, fmt.Sprintf("[%s] %s: %s", severity, report.PluginId, diagnostic.Message))
		}
	}

	for _, plugin := range result.Plugins {
		manifestPath := filepath.Join(plugin.RootPath, "openclaw.plugin.json")
		packageJsonPath := filepath.Join(plugin.RootPath, "package.json")
		if util.FileExists(manifestPath) || util.FileExists(packageJsonPath) {
			inspection := InspectCandidate(plugin.RootPath, plugin.RootPath, false)
			if !inspection.Success {
				status = aggregateStatus([]string{status, core.SetupCheckStatesFail})
				details = append(details, fmt.Sprintf("[%s] %s: %s", core.SetupCheckStatesFail, plugin.Manifest.Id, inspection.ErrorMessage))
				continue
			}

			if !inspection.CanInstall {
				status = aggregateStatus([]string{status, core.SetupCheckStatesFail})
				details = append(details, fmt.Sprintf("[%s] %s: compatibility inspection reported blocking errors.", core.SetupCheckStatesFail, inspection.PluginId))
				for _, diagnostic := range inspection.Diagnostics {
					if diagnostic.Severity == "error" {
						details = append(details, fmt.Sprintf("[%s] %s: [%s] %s", core.SetupCheckStatesFail, inspection.PluginId, diagnostic.Code, diagnostic.Message))
					}
				}
				nextActions = append(nextActions, "Review plugin '%s' and resolve its compatibility errors before upgrading.", inspection.PluginId)
				continue
			}

			if inspection.WarningCount > 0 || len(inspection.Warnings) > 0 || strings.EqualFold(inspection.TrustLevel, "untrusted") {
				status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
				details = append(details, fmt.Sprintf("[%s] %s: trust=%s, warnings=%d.", core.SetupCheckStatesWarn, inspection.PluginId, inspection.TrustLevel, inspection.WarningCount+len(inspection.Warnings)))
			}
		} else {
			status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
			details = append(details, fmt.Sprintf("[%s] %s: entry-only plugin without a structured manifest-backed compatibility surface.", core.SetupCheckStatesWarn, plugin.Manifest.Id))
		}
	}

	if len(details) == 0 {
		details = append(details, "No blocking plugin compatibility issues were detected.")
	}

	if strings.EqualFold(status, core.SetupCheckStatesWarn) {
		details = append(details, "Review entry-only or warning-level plugins before upgrading the gateway/runtime.")
	}

	summary := "No workspace, global, or configured plugins were discovered."
	if len(result.Plugins) > 0 || len(result.Reports) > 0 {
		summary = fmt.Sprintf("Inspected %d plugin(s) with %d discovery diagnostic report(s).", len(result.Plugins), len(result.Reports))
	}

	return &UpgradeCheckSummary{
		Label:       "Plugin compatibility",
		Status:      status,
		Summary:     summary,
		Details:     details,
		NextActions: nextActions}
}

type SkillRootDto struct {
	Path   string
	Source core.SkillSource
}

func enumerateSkillRoots(config *core.GatewayConfig, workspacePath string) []SkillRootDto {
	seen := map[string]struct{}{}
	result := []SkillRootDto{}
	if config.Skills.Load.IncludeManaged {
		if homeDir, err := os.UserHomeDir(); err == nil {
			managed := filepath.Join(homeDir, ".openclaw", "skills")
			if !util.AddIfAbsent(seen, managed) {
				result = append(result, SkillRootDto{
					Path:   managed,
					Source: core.SkillSourceManaged,
				})
			}
		}
	}

	if config.Skills.Load.IncludeWorkspace && workspacePath != "" {
		workspaceSkills := filepath.Join(workspacePath, "skills")
		if _, ok := seen[workspaceSkills]; !ok {
			seen[workspaceSkills] = struct{}{}
			result = append(result, SkillRootDto{
				Path:   workspaceSkills,
				Source: core.SkillSourceWorkspace,
			})
		}
	}

	for _, extraDir := range config.Skills.Load.ExtraDirs {
		resolved := resolveConfiguredPath(extraDir)
		if _, ok := seen[resolved]; !ok && resolved != "" {
			seen[resolved] = struct{}{}
			result = append(result, SkillRootDto{
				Path:   resolved,
				Source: core.SkillSourceExtra,
			})
		}
	}
	return result
}

func isBinaryAvailable(binaryName string) bool {
	if binaryName == "" {
		return false
	}

	path := os.Getenv("PATH")
	if path == "" {
		return false
	}

	candidates := []string{binaryName}
	if runtime.GOOS == "windows" {
		candidates = []string{binaryName, binaryName + ".exe", binaryName + ".cmd", binaryName + ".bat"}
	}

	for _, directory := range strings.Split(path, string(os.PathSeparator)) {
		for _, candidate := range candidates {
			fullPath := filepath.Join(directory, candidate)
			if util.FileExists(fullPath) {
				return true
			}
		}
	}

	return false
}

func evaluateSkillRequirements(config *core.GatewayConfig, definition *core.SkillDefinition) []string {
	issues := []string{}
	key := definition.Metadata.SkillKey
	if key == "" {
		key = definition.Name
	}
	entry, _ := config.Skills.Entries[key]
	if entry == nil {
		entry, _ = config.Skills.Entries[definition.Name]
	}
	entryenv := map[string]string{}
	entryconfig := map[string]string{}
	apiKey := ""
	if entry != nil {
		entryenv = entry.Env
		entryconfig = entry.Config
		apiKey = entry.ApiKey
	}

	for _, env := range definition.Metadata.RequireEnv {
		_, fromEntry := entryenv[env]
		fromApiKey := strings.EqualFold(definition.Metadata.PrimaryEnv, env) && apiKey != ""
		if !fromEntry && !fromApiKey && os.Getenv(env) == "" {
			issues = append(issues, fmt.Sprintf("required env '%s' is not set.", env))
		}
	}

	for _, binary := range definition.Metadata.RequireBins {
		if !isBinaryAvailable(binary) {
			issues = append(issues, fmt.Sprintf("required binary '%s' is not available on PATH.", binary))
		}
	}

	hasBinary := false
	for _, binary := range definition.Metadata.RequireAnyBins {
		if isBinaryAvailable(binary) {
			hasBinary = true
			break
		}
	}

	if len(definition.Metadata.RequireAnyBins) > 0 && hasBinary {
		issues = append(issues, fmt.Sprintf("none of the required fallback binaries are available on PATH: %s", strings.Join(definition.Metadata.RequireAnyBins, ", ")))
	}

	for _, configKey := range definition.Metadata.RequireConfig {
		if value, ok := entryconfig[configKey]; !ok || value == "" {
			issues = append(issues, fmt.Sprintf("required skills.entries config '%s' is not configured.", configKey))
		}
	}

	return issues
}

func evaluateSkills(config *core.GatewayConfig, workspacePath string) *UpgradeCheckSummary {
	details := []string{}
	nextActions := []string{}
	status := core.SetupCheckStatesPass
	totalInspected := 0

	for _, root := range enumerateSkillRoots(config, workspacePath) {
		for _, inspection := range core.SkillInspectorInstance.InspectInstalledRoot(root.Path, root.Source) {
			totalInspected++
			if !inspection.Success || inspection.Definition == nil {
				status = aggregateStatus([]string{status, core.SetupCheckStatesFail})
				msg := inspection.ErrorMessage
				if msg == "" {
					msg = "Failed to inspect skill."
				}
				details = append(details, fmt.Sprintf("[%s] %s: %s", core.SetupCheckStatesFail, root.Path, msg))
				continue
			}

			requirementIssues := evaluateSkillRequirements(config, inspection.Definition)
			if len(requirementIssues) > 0 {
				status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
				for _, issue := range requirementIssues {
					details = append(details, fmt.Sprintf("[%s] %s: %s", core.SetupCheckStatesWarn, inspection.Definition.Name, issue))
				}
			}
		}
	}

	if len(details) == 0 {
		details = append(details, "No blocking skill compatibility issues were detected.")
	}

	if strings.EqualFold(status, core.SetupCheckStatesWarn) {
		nextActions = append(nextActions, "Resolve missing skill requirements before upgrading so post-upgrade validation stays green.")
	}

	summary := "No managed, workspace, or extra skill directories were discovered."
	if totalInspected != 0 {
		summary = fmt.Sprintf("Inspected %d skill package(s) across the configured roots.", totalInspected)
	}

	return &UpgradeCheckSummary{
		Label:       "Skill compatibility",
		Status:      status,
		Summary:     summary,
		Details:     details,
		NextActions: nextActions}
}

func evaluateMigrationImpact(configPath string, config *core.GatewayConfig) *UpgradeCheckSummary {
	details := []string{}
	nextActions := []string{}
	status := core.SetupCheckStatesPass

	if strings.Contains(configPath, "/src/OpenClaw.Gateway/appsettings") {
		status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
		details = append(details, fmt.Sprintf("[%s] Config points at a checked-in gateway appsettings file. Upgrades are safer when you use an external config generated by setup.", core.SetupCheckStatesWarn))
		nextActions = append(nextActions, "Move to an external config file generated by 'openclaw setup' before upgrading.")
	}

	if core.RuntimeOrchestratorNormalize(config.Runtime.Orchestrator) != core.RuntimeOrchestratorNative {
		status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
		details = append(details, fmt.Sprintf("[%s] Runtime orchestrator is '%s' instead of the default native path.", core.SetupCheckStatesWarn, core.RuntimeOrchestratorNormalize(config.Runtime.Orchestrator)))
	}

	if config.Plugins.DynamicNative.Enabled {
		status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
		details = append(details, "[%s] Dynamic native plugins are enabled, which increases upgrade compatibility risk.", core.SetupCheckStatesWarn)
	}

	if config.Plugins.Mcp.Enabled {
		status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
		details = append(details, "[%s] MCP plugin bridges are enabled. Re-check bridge compatibility after the upgrade.", core.SetupCheckStatesWarn)
	}

	if len(config.Plugins.Load.Paths) > 0 {
		status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
		details = append(details, fmt.Sprintf("[%s] Custom plugin load paths are configured: %s", core.SetupCheckStatesWarn, strings.Join(config.Plugins.Load.Paths, ", ")))
	}

	if len(config.Skills.Load.ExtraDirs) > 0 {
		status = aggregateStatus([]string{status, core.SetupCheckStatesWarn})
		details = append(details, fmt.Sprintf("[%s] Extra skill directories are configured: %s", core.SetupCheckStatesWarn, strings.Join(config.Skills.Load.ExtraDirs, ", ")))
	}

	if len(details) == 0 {
		details = append(details, "Config uses the default native runtime/orchestrator lane with no extra compatibility-surface overrides.")
	} else {
		nextActions = append(nextActions, "Treat this install as an elevated-risk upgrade lane and rerun setup verification immediately after upgrading.")
	}

	return &UpgradeCheckSummary{
		Label:       "Migration impact",
		Status:      status,
		Summary:     "Estimates how much the current install relies on non-default or expanded compatibility surfaces.",
		Details:     details,
		NextActions: nextActions}
}

func buildRollbackCaptureItems(configPath string) []RollbackCaptureItem {
	result := []RollbackCaptureItem{
		{
			Kind:                 "config",
			SourcePath:           configPath,
			TargetPath:           configPath,
			IsDirectory:          false,
			Exists:               util.FileExists(configPath),
			SnapshotRelativePath: "config.json",
		},
	}
	envExamplePath, err := core.GatewaySetupArtifactsInstance.BuildEnvExamplePath(configPath)
	if err == nil {
		result = append(result, RollbackCaptureItem{
			Kind:                 "env_example",
			SourcePath:           envExamplePath,
			TargetPath:           envExamplePath,
			IsDirectory:          false,
			Exists:               util.FileExists(envExamplePath),
			SnapshotRelativePath: "env.example",
		})
	}
	deployDir, err := getDeployDirectory(configPath)
	if err == nil {
		result = append(result, RollbackCaptureItem{
			Kind:                 "deploy",
			SourcePath:           deployDir,
			TargetPath:           deployDir,
			IsDirectory:          true,
			Exists:               util.FileExists(deployDir),
			SnapshotRelativePath: "deploy",
		})
	}

	return result
}

func evaluateRollbackSnapshot(configPath, workspacePath string, offline, requireProvider bool, preflightStatus string) *UpgradeCheckSummary {
	if strings.EqualFold(preflightStatus, core.SetupCheckStatesFail) {
		return &UpgradeCheckSummary{
			Label:       "Rollback snapshot",
			Status:      core.SetupCheckStatesSkip,
			Summary:     "Snapshot not updated because the preflight already found blocking issues.",
			Details:     []string{"[skip] Resolve the blocking issues and rerun 'openclaw upgrade check' to refresh the last-known-good snapshot."},
			NextActions: []string{"Resolve the blocking upgrade issues before relying on rollback protection."}}
	}

	captureItems := buildRollbackCaptureItems(configPath)
	artifacts := []core.UpgradeRollbackSnapshotArtifact{}
	for _, item := range captureItems {
		artifacts = append(artifacts, core.UpgradeRollbackSnapshotArtifact{
			Kind:                 item.Kind,
			TargetPath:           item.TargetPath,
			Exists:               item.Exists,
			IsDirectory:          item.IsDirectory,
			SnapshotRelativePath: item.SnapshotRelativePath,
		})
	}
	snapshot := &core.UpgradeRollbackSnapshot{
		CreatedByVersion:   "0.0.0",
		ConfigPath:         configPath,
		WorkspacePath:      workspacePath,
		VerificationStatus: preflightStatus,
		Offline:            offline,
		RequireProvider:    requireProvider,
		Artifacts:          artifacts,
	}

	store := core.NewUpgradeRollbackSnapshotStore(configPath)
	if err := store.Save(snapshot, func(payloadRoot string) error {
		return captureSnapshotArtifacts(captureItems, payloadRoot)
	}); err != nil {
		return &UpgradeCheckSummary{
			Label:       "Rollback snapshot",
			Status:      core.SetupCheckStatesFail,
			Summary:     "Failed to save the last-known-good rollback snapshot.",
			Details:     []string{fmt.Sprintf("[%s] %s", core.SetupCheckStatesFail, err.Error())},
			NextActions: []string{"Fix snapshot storage permissions or disk space, then rerun 'openclaw upgrade check' before upgrading."}}
	}

	existsCount := 0
	absentCount := 0
	for _, artifact := range snapshot.Artifacts {
		if artifact.Exists {
			existsCount++
		} else {
			absentCount++
		}
	}
	artifactSummary := fmt.Sprintf("%d artifact(s) captured; %d tracked as absent.", existsCount, absentCount)
	configPath = core.GatewaySetupPathsIntance.QuoteIfNeeded(configPath)
	return &UpgradeCheckSummary{
		Label:   "Rollback snapshot",
		Status:  core.SetupCheckStatesPass,
		Summary: fmt.Sprintf("Saved the last-known-good setup snapshot. %s", artifactSummary),
		Details: []string{fmt.Sprintf("[%s] Snapshot directory: %s", core.SetupCheckStatesPass, store.SnapshotDirectory()),
			fmt.Sprintf("[%s] Rollback command: openclaw upgrade rollback --config %s", core.SetupCheckStatesPass, configPath),
		},
		NextActions: []string{fmt.Sprintf("Use 'openclaw upgrade rollback --config %s' to restore this snapshot if the upgrade regresses your setup.", configPath)}}
}

func WriteCheck(output io.Writer, check *UpgradeCheckSummary) {
	fmt.Fprintf(output, "- [%s] %s: %s\n", check.Status, check.Label, check.Summary)
	for _, detail := range check.Details {
		fmt.Fprintf(output, "  %s\n", detail)
	}
}

func captureSnapshotArtifacts(items []RollbackCaptureItem, payloadRoot string) error {
	for _, item := range items {
		if !item.Exists || item.SnapshotRelativePath == "" {
			continue
		}

		destination := filepath.Join(payloadRoot, item.SnapshotRelativePath)
		if item.IsDirectory {
			if err := util.CopyDirectoryWithOptions(item.SourcePath, destination, util.CopyOptions{
				UnixModeStrategy: util.UnixModeOwnerOnlyPreserveExecute,
			}); err != nil {
				return err
			}
		} else {
			if err := util.CopyFile(item.SourcePath, destination); err != nil {
				return err
			}
		}
	}

	return nil
}

func upgradeCommandRunCheck(args []string, output, errWriter io.Writer) (int, error) {
	parsed := CliArgsParse(args)
	if parsed.ShowHelp {
		upgradeCommandPrintHelp(output)
		return 0, nil
	}

	configPath, err := resolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(configPath)
	if err != nil {
		fmt.Fprintln(errWriter, err.Error())
		return 1, err
	}

	workspacePath := resolveWorkspacePath(config)
	offline := parsed.HasFlag("--offline")
	verification := evaluateVerification(config, workspacePath, offline)
	plugins := evaluatePlugins(config, workspacePath)
	skills := evaluateSkills(config, workspacePath)
	migrationImpact := evaluateMigrationImpact(configPath, config)
	rollbackSnapshot := evaluateRollbackSnapshot(configPath, workspacePath, offline, !offline, aggregateStatus([]string{
		verification.Status,
		plugins.Status,
		skills.Status,
		migrationImpact.Status}))

	overall := aggregateStatus([]string{
		verification.Status,
		plugins.Status,
		skills.Status,
		migrationImpact.Status,
		rollbackSnapshot.Status})
	if workspacePath == "" {
		workspacePath = "not configured"
	}

	fmt.Fprintf(output, "OpenClaw upgrade preflight\nConfig: %s\nWorkspace: %s\nOverall result: %s\n\n", configPath, workspacePath, overall)
	WriteCheck(output, verification)
	WriteCheck(output, plugins)
	WriteCheck(output, skills)
	WriteCheck(output, migrationImpact)
	WriteCheck(output, rollbackSnapshot)

	nextActions := append(verification.NextActions, append(plugins.NextActions, append(skills.NextActions, append(migrationImpact.NextActions, rollbackSnapshot.NextActions...)...)...)...)

	if len(nextActions) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Recommended next actions:")
		for _, action := range nextActions {
			fmt.Fprintf(output, "- %s\n", action)
		}
	}

	if strings.EqualFold(overall, core.SetupCheckStatesFail) {
		msg := "Upgrade preflight failed. Resolve the blocking issues before upgrading."
		fmt.Fprintln(output)
		fmt.Fprintln(output, msg)
		return 1, errors.New(msg)
	}

	return 0, nil
}

func upgradeCommandPrintHelp(output io.Writer) {
	fmt.Fprintln(output, `
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

func validateSnapshotRelativePath(snapshotRelativePath, artifactKind string) (string, error) {
	if snapshotRelativePath == "" {
		return "", fmt.Errorf("Snapshot artifact '%s' is missing its payload path.", artifactKind)
	}

	if filepath.IsAbs(snapshotRelativePath) {
		return "", fmt.Errorf("Snapshot artifact '%s' has an invalid rooted payload path '%s'", artifactKind, snapshotRelativePath)
	}

	normalizedPath := strings.ReplaceAll(snapshotRelativePath, "/", string(os.PathSeparator))
	segments := strings.Split(normalizedPath, string(os.PathSeparator))
	hasdot := false
	for _, segment := range segments {
		if segment == "." || segment == ".." {
			hasdot = true
			break
		}
	}
	if len(segments) == 0 || hasdot {
		return "", fmt.Errorf("Snapshot artifact '%s' has an invalid payload path '%s'", artifactKind, snapshotRelativePath)
	}

	return strings.Join(segments, string(os.PathSeparator)), nil
}

func validateRestoreArtifacts(snapshot *core.UpgradeRollbackSnapshot, configPath string) ([]ValidatedRestoreArtifact, error) {
	expectedByKind := map[string]RollbackCaptureItem{}
	for _, item := range buildRollbackCaptureItems(configPath) {
		expectedByKind[item.Kind] = item
	}

	seen := map[string]struct{}{}

	validated := []ValidatedRestoreArtifact{}
	for _, artifact := range snapshot.Artifacts {
		expected, ok := expectedByKind[artifact.Kind]
		if !ok {
			return nil, fmt.Errorf("Snapshot artifact '%s' is not a supported rollback artifact kind.", artifact.Kind)
		}

		if ok := util.AddIfAbsent(seen, artifact.Kind); !ok {
			return nil, fmt.Errorf("Snapshot artifact '%s' appears multiple times in the rollback manifest.", artifact.Kind)
		}

		if artifact.IsDirectory != expected.IsDirectory {
			return nil, fmt.Errorf("Snapshot artifact '%s' has an unexpected directory/file shape.", artifact.Kind)
		}

		expectedTargetPath, err := filepath.Abs(expected.TargetPath)
		if err != nil {
			return nil, err
		}

		manifestTargetPath, err := filepath.Abs(artifact.TargetPath)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(expectedTargetPath, manifestTargetPath) {
			return nil, fmt.Errorf("Snapshot artifact '%s' has an unexpected restore target path '%s'", artifact.Kind, artifact.TargetPath)
		}

		if !artifact.Exists {
			if artifact.SnapshotRelativePath != "" {
				return nil, fmt.Errorf("Snapshot artifact '%s' is marked absent but still includes a payload path.", artifact.Kind)
			}

			art := ValidatedRestoreArtifact{
				Kind:        artifact.Kind,
				TargetPath:  expectedTargetPath,
				IsDirectory: artifact.IsDirectory,
			}
			validated = append(validated, art)
			continue
		}

		snapshotRelativePath, err := validateSnapshotRelativePath(artifact.SnapshotRelativePath, artifact.Kind)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(snapshotRelativePath, expected.SnapshotRelativePath) {
			return nil, fmt.Errorf("Snapshot artifact '%s' has an unexpected payload path '%s'", artifact.Kind, artifact.SnapshotRelativePath)
		}

		art := ValidatedRestoreArtifact{
			Kind:                 artifact.Kind,
			TargetPath:           expectedTargetPath,
			IsDirectory:          artifact.IsDirectory,
			Exists:               true,
			SnapshotRelativePath: snapshotRelativePath,
		}
		validated = append(validated, art)
	}

	missingKinds := []string{}
	for key := range expectedByKind {
		if _, ok := seen[key]; !ok {
			missingKinds = append(missingKinds, key)
		}
	}

	if len(missingKinds) > 0 {
		return nil, fmt.Errorf("Rollback snapshot manifest is missing required artifact(s): %s", strings.Join(missingKinds, ", "))
	}

	return validated, nil
}

func createSiblingDirectoryPath(path, suffix string) (string, error) {
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	parent := filepath.Dir(fullPath)

	name := filepath.Base(fullPath)
	return filepath.Join(parent, fmt.Sprintf("%s.%s.%s", name, suffix, util.CleanUUID())), nil
}

func replaceDirectory(sourcePath, destinationPath string) error {
	fullDestinationPath, err := filepath.Abs(destinationPath)
	if err != nil {
		return err
	}

	destinationParent := filepath.Dir(fullDestinationPath)
	os.MkdirAll(destinationParent, 0755)

	stagedPath, err := createSiblingDirectoryPath(fullDestinationPath, "staged")
	if err != nil {
		return err
	}
	backupPath, err := createSiblingDirectoryPath(fullDestinationPath, "backup")
	if err != nil {
		return err
	}
	backupCreated := false
	replaced := false

	defer func() {
		if util.DirectoryExists(stagedPath) {
			os.Remove(stagedPath)
		}
		if replaced && backupCreated && util.DirectoryExists(backupPath) {
			os.Remove(backupPath)
		}
	}()

	if err := util.CopyDirectoryWithOptions(sourcePath, stagedPath, util.CopyOptions{
		UnixModeStrategy: util.UnixModePreserveSource,
	}); err != nil {
		return err
	}

	if util.DirectoryExists(fullDestinationPath) {
		if err := os.Rename(fullDestinationPath, backupPath); err != nil {
			return err
		}
		backupCreated = true
	}

	return nil
}

func restoreSnapshot(snapshot *core.UpgradeRollbackSnapshot, store *core.UpgradeRollbackSnapshotStore, configPath string) error {
	artifacts, err := validateRestoreArtifacts(snapshot, configPath)
	if err != nil {
		return err
	}

	for _, artifact := range artifacts {
		if artifact.Exists {
			payloadPath, err := store.ResolvePayloadPath(artifact.SnapshotRelativePath)
			if err != nil {
				return err
			}
			if artifact.IsDirectory {
				err = replaceDirectory(payloadPath, artifact.TargetPath)
			} else {
				err = util.CopyFile(payloadPath, artifact.TargetPath)
			}
			if err != nil {
				return err
			}
			continue
		}

		if artifact.IsDirectory {
			if util.DirectoryExists(artifact.TargetPath) {
				util.DeleteDirectory(artifact.TargetPath)
			}
		} else if util.FileExists(artifact.TargetPath) {
			util.DeleteOneFile(artifact.TargetPath)
		}
	}

	return nil
}
