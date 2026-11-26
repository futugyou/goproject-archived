package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type MigrationPlan struct {
	SourcePath           string
	TargetConfigPath     string
	DiscoveredConfigPath string
	ManagedSkillRootPath string
	PluginReviewPlanPath string
	TranslatedConfig     *core.GatewayConfig
	Compatibility        []core.UpstreamMigrationCompatibilityItem
	Skills               []core.UpstreamMigrationSkillItem
	Plugins              []core.UpstreamMigrationPluginItem
	Warnings             []string
	SkippedSettings      []string
}

func upstreamMigrationCommandPrintHelp(output io.Writer) {
	fmt.Fprintln(output,
		`
            openclaw migrate upstream

            Usage:
              openclaw migrate upstream --source <path> --target-config <path> [--apply] [--report <path>]

            Notes:
              - Dry-run is the default and emits a structured migration report.
              - Apply mode writes translated config, imports managed skills, and writes a plugin review plan.
              - Memory, session history, and workspace state are not migrated in v1.
            `)
}

func UpstreamMigrationCommandRun(args []string, output io.Writer, errwirter io.Writer, currentDirectory string) (int, error) {
	parsed := CliArgsParse(args)
	if parsed.ShowHelp {
		upstreamMigrationCommandPrintHelp(output)
		return 0, nil
	}

	configPath := util.Deref(parsed.GetOption("--source"))
	if configPath == "" {
		configPath = currentDirectory
	}

	expandPath := core.GatewaySetupPathsIntance.ExpandPath(configPath)
	sourcePath, err := filepath.Abs(expandPath)
	if err != nil {
		return 1, err
	}

	targetConfigPath := util.Deref(parsed.GetOption("--target-config"))
	if targetConfigPath == "" {
		msg := "--target-config is required."
		fmt.Fprintln(errwirter, msg)
		return 2, errors.New(msg)
	}

	if !util.DirectoryExists(sourcePath) {
		msg := fmt.Sprintf("Source path not found: %s", sourcePath)
		fmt.Fprintln(errwirter, msg)
		return 2, errors.New(msg)
	}

	resolvedTargetConfigPath, err := filepath.Abs(core.GatewaySetupPathsIntance.ExpandPath(targetConfigPath))
	if err != nil {
		return 1, err
	}

	reportPath := util.Deref(parsed.GetOption("--report"))
	apply := parsed.HasFlag("--apply")

	plan := upstreamMigrationCommandBuildPlan(sourcePath, resolvedTargetConfigPath)
	ctx := context.Background()
	report, err := upstreamMigrationCommandExecutePlan(ctx, plan, apply)
	if err != nil {
		return 1, err
	}

	if reportPath != "" {
		resolvedReportPath, err := filepath.Abs(core.GatewaySetupPathsIntance.ExpandPath(reportPath))
		if err != nil {
			return 1, err
		}
		directory := filepath.Dir(resolvedReportPath)
		if directory != "" {
			os.MkdirAll(directory, 0755)
		}

		data, _ := json.Marshal(report)
		if err := os.WriteFile(resolvedReportPath, data, 0644); err != nil {
			return 1, err
		}

		fmt.Fprintf(output, "Wrote migration report: %s\n", resolvedReportPath)
	} else {
		data, _ := json.Marshal(report)
		output.Write(data)
		fmt.Println()
	}

	if apply {
		fmt.Fprintf(output, "Applied translated config: %s\n", resolvedTargetConfigPath)
		if report.ManagedSkillRootPath != "" {
			fmt.Fprintf(output, "Imported skills into: %s\n", report.ManagedSkillRootPath)
		}
		if report.PluginReviewPlanPath != "" {
			fmt.Fprintf(output, "IWrote plugin review plan: %s\n", report.PluginReviewPlanPath)
		}
	}

	return 0, nil
}

func upstreamMigrationCommandExecutePlan(ctx context.Context, plan MigrationPlan, apply bool) (*core.UpstreamMigrationReport, error) {
	if apply {
		if err := core.GatewayConfigFileInstance.Save(plan.TranslatedConfig, plan.TargetConfigPath); err != nil {
			return nil, err
		}

		if err := upstreamMigrationCommandImportSkills(ctx, plan.Skills, plan.ManagedSkillRootPath); err != nil {
			return nil, err
		}
		if err := upstreamMigrationCommandWritePluginReviewPlan(ctx, plan.PluginReviewPlanPath, plan.Plugins); err != nil {
			return nil, err
		}
	}

	return &core.UpstreamMigrationReport{
		SourcePath:           plan.SourcePath,
		TargetConfigPath:     plan.TargetConfigPath,
		DiscoveredConfigPath: plan.DiscoveredConfigPath,
		ManagedSkillRootPath: plan.ManagedSkillRootPath,
		PluginReviewPlanPath: plan.PluginReviewPlanPath,
		Applied:              apply,
		Compatibility:        plan.Compatibility,
		Skills:               plan.Skills,
		Plugins:              plan.Plugins,
		Warnings:             plan.Warnings,
		SkippedSettings:      plan.SkippedSettings,
	}, nil
}

type PluginReviewPlanPayload struct {
	GeneratedAtUtc string                             `json:"generatedAtUtc"`
	Items          []core.UpstreamMigrationPluginItem `json:"items"`
}

func upstreamMigrationCommandWritePluginReviewPlan(ctx context.Context, path string, plugins []core.UpstreamMigrationPluginItem) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	payload := PluginReviewPlanPayload{
		GeneratedAtUtc: time.Now().UTC().Format(time.RFC3339Nano),
		Items:          plugins,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func upstreamMigrationCommandImportSkills(ctx context.Context, skills []core.UpstreamMigrationSkillItem, targetRoot string) error {
	os.MkdirAll(targetRoot, 0755)
	for _, skill := range skills {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		destination := filepath.Join(targetRoot, skill.TargetSlug)
		util.CopyDirectoryWithoutSymlink(skill.SourcePath, destination)
	}

	return nil
}

func upstreamMigrationCommandDiscoverSkills(sourcePath string) (results []core.UpstreamMigrationSkillItem, compatibility []core.UpstreamMigrationCompatibilityItem, warnings []string) {
	seenTargets := map[string]struct{}{}
	dirs, err := util.FindDirectoriesCantainsFileName(sourcePath, "SKILL.md")
	if err != nil {
		warnings = append(warnings, err.Error())
		return
	}
	for _, dir := range dirs {
		if strings.Contains(dir, "/node_modules") {
			continue
		}

		inspection := core.SkillInspectorInstance.InspectPath(dir, core.SkillSourceManaged)
		if !inspection.Success || inspection.Definition == nil {
			msg := inspection.ErrorMessage
			if msg == "" {
				msg = "Failed to parse SKILL.md."
			}

			compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
				Type:     "skill",
				Subject:  dir,
				Status:   "unsupported",
				Summary:  "Skill could not be parsed.",
				Warnings: []string{msg},
			})
			continue
		}

		slug := slugify(inspection.Definition.Name)
		if _, ok := seenTargets[slug]; !ok {
			seenTargets[slug] = struct{}{}
			warnings = append(warnings, fmt.Sprintf("Duplicate migrated skill slug '%s' from %s; keeping the first instance.", slug, dir))
			compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
				Type:     "skill",
				Subject:  inspection.Definition.Name,
				Status:   "partial",
				Summary:  "Skill was discovered more than once; only the first instance will be applied.",
				Warnings: []string{fmt.Sprintf("Duplicate source path skipped: %s", dir)},
			})
			continue
		}

		results = append(results, core.UpstreamMigrationSkillItem{
			Name:       inspection.Definition.Name,
			SourcePath: dir,
			TargetSlug: slug,
			Status:     "supported",
		})
		compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
			Type:    "skill",
			Subject: inspection.Definition.Name,
			Status:  "supported",
			Summary: fmt.Sprintf("Skill will be imported into managed skills as '%s'.", slug),
		})
	}

	slices.SortFunc(results, func(a, b core.UpstreamMigrationSkillItem) int {
		return strings.Compare(a.TargetSlug, b.TargetSlug)
	})

	return
}

func upstreamMigrationCommandBuildPackageSpec(pluginRoot string, inspection PluginInstallInspection) string {
	packageJsonPath := filepath.Join(pluginRoot, "package.json")
	if util.FileExists(packageJsonPath) {
		data, err := os.ReadFile(packageJsonPath)
		if err == nil {
			type innerDto struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}

			var dto innerDto
			err := json.Unmarshal(data, &dto)
			if err == nil {
				if dto.Name != "" {
					if dto.Version == "" {
						return dto.Name
					} else {
						return fmt.Sprintf("%s@%s", dto.Name, dto.Version)
					}

				}
			}
		}
	}

	if inspection.DisplayName != "" && inspection.Version != "" {
		return fmt.Sprintf("%s@%s", inspection.DisplayName, inspection.Version)
	}

	return inspection.PluginId
}

func upstreamMigrationCommandMapCatalogStatus(status string) string {
	if status == "compatible" {
		return "supported"
	}
	return "unsupported"
}

func upstreamMigrationCommandDiscoverPlugins(sourcePath string) (results []core.UpstreamMigrationPluginItem, compatibility []core.UpstreamMigrationCompatibilityItem, warnings []string) {
	catalog, err := core.PublicCompatibilityCatalogInstance.GetCatalog("", "npm-plugin", "")
	if err != nil {
		warnings = append(warnings, err.Error())
		return
	}

	manifestPaths, err := util.FindFiles(sourcePath, "openclaw.plugin.json")
	if err != nil {
		warnings = append(warnings, err.Error())
		return
	}

	for _, manifestPath := range manifestPaths {
		if strings.Contains(manifestPath, "/node_modules") {
			continue
		}

		pluginRoot := filepath.Dir(manifestPath)

		inspection := InspectCandidate(pluginRoot, pluginRoot, false)
		if !inspection.Success {
			msg := inspection.ErrorMessage
			if msg == "" {
				msg = "Plugin inspection failed."
			}
			compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
				Type:     "plugin",
				Subject:  manifestPath,
				Status:   "unsupported",
				Summary:  "Plugin manifest could not be inspected.",
				Warnings: []string{msg},
			})
			continue
		}

		packageSpec := upstreamMigrationCommandBuildPackageSpec(pluginRoot, inspection)
		var matchedCatalog *core.CompatibilityCatalogEntry
		for i := range catalog.Items {
			if catalog.Items[i].PluginID == inspection.PluginId || catalog.Items[i].PackageName == packageSpec || (catalog.Items[i].PackageSpec != "" && catalog.Items[i].PackageSpec == packageSpec) {
				matchedCatalog = &catalog.Items[i]
				break
			}
		}

		status := "unsupported"
		if inspection.CanInstall {
			if matchedCatalog == nil {
				status = "partial"
			} else {
				status = upstreamMigrationCommandMapCatalogStatus(matchedCatalog.CompatibilityStatus)
			}
		}

		guidance := []string{}
		if packageSpec != "" {
			guidance = append(guidance, fmt.Sprintf("Install review command: openclaw plugins install %s --dry-run", packageSpec))
		}

		guidance = append(guidance, fmt.Sprintf("Trust level: %s", inspection.TrustLevel))
		guidance = append(guidance, fmt.Sprintf("Declared surface: %s", inspection.DeclaredSurface))
		guidance = append(guidance, inspection.Warnings...)

		if matchedCatalog != nil {
			guidance = append(guidance, matchedCatalog.Guidance...)
		}

		if len(inspection.Diagnostics) > 0 {
			for _, item := range inspection.Diagnostics {
				guidance = append(guidance, fmt.Sprintf("%s: %s", item.Code, item.Message))
			}
		}

		subject := inspection.PluginId
		if subject == "" {
			subject = util.GetFileNameWithoutExtension(pluginRoot)
		}
		results = append(results, core.UpstreamMigrationPluginItem{
			Subject:     subject,
			PackageSpec: packageSpec,
			Status:      status,
			Guidance:    guidance,
		})

		subject = inspection.PluginId
		if subject == "" {
			subject = manifestPath
		}
		summary := "Plugin manifest was discovered and converted into a pending review/install plan."
		if inspection.CanInstall {
			summary = "Plugin manifest was discovered but failed local compatibility inspection."
		}
		compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
			Type:     "plugin",
			Subject:  subject,
			Status:   status,
			Summary:  summary,
			Warnings: guidance,
		})
	}

	if len(results) == 0 {
		warnings = append(warnings, "No upstream plugin manifests were discovered under the source tree.")
	}

	slices.SortFunc(results, func(a, b core.UpstreamMigrationPluginItem) int {
		return strings.Compare(a.Subject, b.Subject)
	})

	return
}

func upstreamMigrationDiscoverConfigPath(sourcePath string) string {
	candidates := []string{
		filepath.Join(sourcePath, "openclaw.json"),
		filepath.Join(sourcePath, "openclaw.config.json"),
		filepath.Join(sourcePath, "openclaw.settings.json"),
		filepath.Join(sourcePath, "config", "openclaw.json"),
		filepath.Join(sourcePath, "config", "openclaw.config.json"),
		filepath.Join(sourcePath, "config", "openclaw.settings.json"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	var discoveredPath string

	_ = filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(sourcePath, path)
		if err == nil && rel != "." {
			depth := len(strings.Split(rel, string(filepath.Separator)))
			if depth > 3 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			nameLower := strings.ToLower(d.Name())
			if strings.Contains(nameLower, "openclaw") || strings.Contains(nameLower, "claw") {
				discoveredPath = path
				return fs.SkipAll
			}
		}

		return nil
	})

	return discoveredPath
}

func upstreamMigrationTranslateConfig(sourcePath, discoveredConfigPath string) (config core.GatewayConfig, compatibility []core.UpstreamMigrationCompatibilityItem, warnings []string, skippedSettings []string) {
	defaultconfig := core.GatewayConfig{
		Tooling: core.ToolingConfig{
			WorkspaceRoot:     sourcePath,
			WorkspaceOnly:     true,
			AllowedReadRoots:  []string{sourcePath},
			AllowedWriteRoots: []string{sourcePath},
		},
	}

	if discoveredConfigPath == "" {
		config = defaultconfig
		compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
			Type:     "config",
			Subject:  "gateway-config",
			Status:   "partial",
			Summary:  "No upstream JSON config file was discovered. A default OpenClaw config will be written instead.",
			Warnings: []string{"Provide provider/model/auth settings manually if the source tree did not include a compatible JSON config."},
		})
		return
	}

	data, err := os.ReadFile(discoveredConfigPath)
	if err != nil {
		config = defaultconfig
		compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
			Type:     "config",
			Subject:  discoveredConfigPath,
			Status:   "unsupported",
			Summary:  "The discovered upstream config could not be parsed as JSON.",
			Warnings: []string{err.Error()},
		})
		warnings = append(warnings, fmt.Sprintf("Config parse failed for %s: %s", discoveredConfigPath, err.Error()))
		return
	}

	var rawRoot map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawRoot); err != nil {
		config = defaultconfig
		compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
			Type:     "config",
			Subject:  discoveredConfigPath,
			Status:   "unsupported",
			Summary:  "The discovered upstream config could not be parsed as JSON.",
			Warnings: []string{err.Error()},
		})
		warnings = append(warnings, fmt.Sprintf("Config parse failed for %s: %s", discoveredConfigPath, err.Error()))
		return
	}

	var targetNode map[string]json.RawMessage
	if openClawRaw, ok := rawRoot["OpenClaw"]; ok {
		if err := json.Unmarshal(openClawRaw, &targetNode); err != nil {
			targetNode = rawRoot
		}
	} else {
		targetNode = rawRoot
	}

	targetBytes, _ := json.Marshal(targetNode)
	if err := json.Unmarshal(targetBytes, &config); err != nil {
		config = defaultconfig
		compatibility = append(compatibility, core.UpstreamMigrationCompatibilityItem{
			Type:     "config",
			Subject:  discoveredConfigPath,
			Status:   "unsupported",
			Summary:  "The discovered upstream config could not be parsed as JSON.",
			Warnings: []string{err.Error()},
		})
		warnings = append(warnings, fmt.Sprintf("Config parse failed for %s: %s", discoveredConfigPath, err.Error()))
	}

	mappedRootKeys := map[string]bool{
		"bindAddress": true,
		"port":        true,
		"authToken":   true,
		"llm":         true,
		"tooling":     true,
		"memory":      true,
		"plugins":     true,
		"skills":      true,
		"channels":    true,
	}

	for key := range targetNode {
		if !mappedRootKeys[key] {
			skippedSettings = append(skippedSettings, fmt.Sprintf("%s: not translated by upstream migration v1", key))
		}
	}

	if _, ok := targetNode["tooling"]; ok {
		value := config.Tooling.WorkspaceRoot
		if !filepath.IsAbs(value) {
			resolved, err := filepath.Abs(filepath.Join(sourcePath, value))
			if err == nil {
				value = resolved
			}
		}
		config.Tooling.WorkspaceRoot = value
		config.Tooling.AllowedReadRoots = []string{value}
		config.Tooling.AllowedWriteRoots = []string{value}
	} else {
		config.Tooling = defaultconfig.Tooling
	}

	if _, ok := targetNode["memory"]; ok {
		value := config.Memory.StoragePath
		if !filepath.IsAbs(value) {
			dir := filepath.Dir(discoveredConfigPath)
			if dir == "" {
				dir = sourcePath
			}
			resolved, err := filepath.Abs(filepath.Join(dir, value))
			if err == nil {
				value = resolved
			}
		}
		config.Memory.StoragePath = value
	}

	return
}

func upstreamMigrationResolveManagedSkillRoot(targetConfigPath string) string {
	configDir := filepath.Dir(targetConfigPath)
	if configDir == "." || targetConfigPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			configDir = cwd
		}
	}
	parent := filepath.Dir(configDir)
	baseDir := configDir
	if parent != configDir {
		baseDir = parent
	}
	return filepath.Join(baseDir, "skills")
}

func upstreamMigrationBuildPluginReviewPlanPath(targetConfigPath string) string {
	configDir := filepath.Dir(targetConfigPath)
	if configDir == "." || targetConfigPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			configDir = cwd
		}
	}
	var fileName = util.GetFileNameWithoutExtension(targetConfigPath)
	return filepath.Join(configDir, fmt.Sprintf("%s.plugin-review-plan.json", fileName))
}

func upstreamMigrationCommandBuildPlan(sourcePath, targetConfigPath string) MigrationPlan {
	skill, compatibility, warnings := upstreamMigrationCommandDiscoverSkills(sourcePath)
	plugins, compatibility1, warnings1 := upstreamMigrationCommandDiscoverPlugins(sourcePath)
	discoveredConfigPath := upstreamMigrationDiscoverConfigPath(sourcePath)
	translatedConfig, compatibility2, warnings2, skippedSettings := upstreamMigrationTranslateConfig(sourcePath, discoveredConfigPath)
	managedSkillRootPath := upstreamMigrationResolveManagedSkillRoot(targetConfigPath)
	pluginReviewPlanPath := upstreamMigrationBuildPluginReviewPlanPath(targetConfigPath)

	return MigrationPlan{
		SourcePath:           sourcePath,
		TargetConfigPath:     targetConfigPath,
		DiscoveredConfigPath: discoveredConfigPath,
		ManagedSkillRootPath: managedSkillRootPath,
		PluginReviewPlanPath: pluginReviewPlanPath,
		TranslatedConfig:     &translatedConfig,
		Compatibility:        append(append(compatibility, compatibility1...), compatibility2...),
		Skills:               skill,
		Plugins:              plugins,
		Warnings:             append(append(warnings, warnings1...), warnings2...),
		SkippedSettings:      skippedSettings,
	}
}
