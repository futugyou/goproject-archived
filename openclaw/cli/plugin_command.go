package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"

	cp "github.com/otiai10/copy"
)

type PluginInstallInspection struct {
	Success             bool
	CanInstall          bool
	ErrorMessage        string
	PluginId            string
	DisplayName         string
	Version             string
	Description         string
	EntryPath           string
	Format              string //"native"
	BundleFormat        string
	TrustLevel          string
	TrustReason         string
	CompatibilityStatus string
	ErrorCount          int
	WarningCount        int
	DeclaredSurface     string
	Diagnostics         []core.PluginCompatibilityDiagnostic
	Warnings            []string
}

func InspectUnsupportedRuntimeSurfaces(rootPath string, diagnostics []core.PluginCompatibilityDiagnostic) []core.PluginCompatibilityDiagnostic {
	result := []core.PluginCompatibilityDiagnostic{}
	for _, file := range enumeratePluginSourceFiles(rootPath) {
		source, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		result = append(result, addUnsupportedSurfaceDiagnostic(string(source), file, "registerGatewayMethod", "unsupported_gateway_method", diagnostics)...)
	}

	return result
}

func FailurePluginInstallInspection(msg string) *PluginInstallInspection {
	return &PluginInstallInspection{ErrorMessage: msg}
}

func addUnsupportedSurfaceDiagnostic(source, file, apiName, code string, diagnostics []core.PluginCompatibilityDiagnostic) []core.PluginCompatibilityDiagnostic {
	pattern := fmt.Sprintf(`\bapi\s*\.\s*%s\s*\(`, regexp.QuoteMeta(apiName))
	matched, err := regexp.MatchString(pattern, source)
	if err != nil {
		return diagnostics
	}

	result := !matched

	if !result {
		for _, item := range diagnostics {
			if item.Code == code {
				result = true
				break
			}
		}
	}

	if result {
		return diagnostics
	}

	return append(diagnostics, core.PluginCompatibilityDiagnostic{
		Severity: "error",
		Code:     code,
		Message:  fmt.Sprintf("Plugin source references api.%s()", apiName),
		Surface:  apiName,
		Path:     file,
	})
}

func enumeratePluginSourceFiles(rootPath string) []string {
	const maxDepth = 16
	var results []string

	type stackItem struct {
		dir   string
		depth int
	}

	pending := []stackItem{{dir: rootPath, depth: 0}}

	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		entries, err := os.ReadDir(current.dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}

			name := entry.Name()
			fullPath := filepath.Join(current.dir, name)

			if entry.IsDir() {
				if name != "node_modules" && name != ".git" {
					if current.depth < maxDepth {
						pending = append(pending, stackItem{
							dir:   fullPath,
							depth: current.depth + 1,
						})
					}
				}
			} else {
				ext := strings.ToLower(filepath.Ext(name))
				if ext == ".js" || ext == ".mjs" || ext == ".cjs" || ext == ".ts" {
					results = append(results, fullPath)
				}
			}
		}
	}

	return results
}

func pluginCommandPrintHelp() {
	fmt.Println(` openclaw plugins — Manage OpenClaw plugins

            Usage:
              openclaw plugins install <package|path|tarball>  Install a plugin from npm/ClawHub or local source
              openclaw plugins remove <plugin-name>            Remove an installed plugin
              openclaw plugins list                            List installed plugins
              openclaw plugins inspect <plugin|path> [--runtime] Inspect static and optional runtime compatibility
              openclaw plugins search <query>                  Search npm for OpenClaw plugins

            Options:
              -g, --global    Use global extensions directory (~/.openclaw/extensions)
              --dry-run       Inspect the plugin and print declared surfaces without installing it

            Examples:
              openclaw plugins install @sliverp/qqbot
              openclaw plugins install @opik/opik-openclaw
              openclaw plugins install ./my-local-plugin
              openclaw plugins install ./my-plugin.tgz
              openclaw plugins remove qqbot
              openclaw plugins list
              openclaw plugins inspect ./my-codex-bundle --runtime
              openclaw plugins search openclaw dingtalk`)
}

func PluginCommandRun(ctx context.Context, command string, args []string) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		pluginCommandPrintHelp()
		return 0, nil
	}

	subcommand := args[0]
	rest := []string{}
	if len(args) > 1 {
		rest = args[1:]
	}

	switch subcommand {
	case "install":
		return pluginCommandInstall(ctx, rest)
	case "remove", "uninstall":
		return pluginCommandRemove(ctx, rest)
	case "list", "ls":
		return pluginCommandList(ctx, rest)
	case "inspect":
		return pluginCommandInspect(ctx, rest)
	default:
		return unknownPluginCommand(subcommand)
	}
}

func pluginCommandInspect(ctx context.Context, args []string) (int, error) {
	target := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			target = args[i]
			break
		}
	}

	if target == "" {
		return 2, errors.New("Usage: openclaw plugins inspect <plugin-name|local-directory> [--runtime]")
	}

	candidatePath := target
	if !util.DirectoryExists(candidatePath) &&
		!util.FileExists(candidatePath) &&
		!filepath.IsAbs(candidatePath) {
		extensionsDir := resolveExtensionsDir(slices.Contains(args, "--global") || slices.Contains(args, "-g"))
		candidatePath = filepath.Join(extensionsDir, target)
		if !util.DirectoryExists(candidatePath) {
			candidatePath = filepath.Join(extensionsDir, SanitizePackageName(target))
		}
	}

	if !util.DirectoryExists(candidatePath) && !util.FileExists(candidatePath) {
		return 1, fmt.Errorf("Plugin '%s' was not found.", target)
	}

	rootPath, err := filepath.Abs(candidatePath)
	if err != nil {
		return 1, err
	}
	if !util.DirectoryExists(candidatePath) {
		rootPath = filepath.Dir(rootPath)
	}

	inspection := InspectCandidate(rootPath, target, false)
	if !inspection.Success {
		return 1, errors.New(inspection.ErrorMessage)
	}

	pluginCommandPrintInspection(inspection)
	if !inspection.CanInstall {
		return 1, errors.New("can not install plugin")
	}
	if !slices.Contains(args, "--runtime") {
		return 0, nil
	}

	if inspection.Format == "bundle" {
		fmt.Println("Runtime: content bundle; no arbitrary bundle module was executed.")
		return 0, nil
	}

	runtime, err := InspectRuntime(ctx, inspection.EntryPath, inspection.PluginId)
	if err != nil {
		return 1, err
	}

	rt := "incompatible"
	if runtime.Compatible {
		rt = "compatible"
	}
	fmt.Printf("Runtime: %s", rt)
	fmt.Printf("Registered: tools=%d, channels=%d, commands=%d, cli=%d, providers=%d", runtime.ToolCount, runtime.ChannelCount, runtime.CommandCount, runtime.CliCommandCount, runtime.ProviderCount)
	for _, diagnostic := range runtime.Diagnostics {
		if diagnostic.Severity == "error" {
			fmt.Printf("Error: [%s] %s\n", diagnostic.Code, diagnostic.Message)
		} else {
			fmt.Printf("Warning: [%s] %s\n", diagnostic.Code, diagnostic.Message)
		}
	}

	if runtime.Compatible {
		return 0, nil
	}

	return 1, errors.New(runtime.ErrorMsg)
}

func pluginCommandList(_ context.Context, args []string) (int, error) {
	global := slices.Contains(args, "--global") || slices.Contains(args, "-g")
	extensionsDir := resolveExtensionsDir(global)

	if !util.DirectoryExists(extensionsDir) {
		return 0, errors.New("No plugins installed.")
	}

	plugins := core.PluginDiscoveryInstance.Discover(&core.PluginsConfig{
		Load: core.PluginLoadConfig{Paths: []string{extensionsDir}},
	}, "")

	if len(plugins) == 0 {
		return 0, errors.New("No plugins installed.")
	}

	fmt.Printf("Installed plugins (%d):\n", len(plugins))
	for _, plugin := range plugins {
		name := plugin.Manifest.Name
		if name == "" {
			name = plugin.Manifest.Id
		}
		if name == "" {
			name = filepath.Base(plugin.RootPath)
		}
		version := plugin.Manifest.Version
		if version == "" {
			version = "?"
		}
		desc := plugin.Manifest.Description
		hasStructuredSurface :=
			len(plugin.Manifest.Channels) > 0 ||
				len(plugin.Manifest.Providers) > 0 ||
				len(plugin.Manifest.Skills) > 0 ||
				plugin.Manifest.ConfigSchema != nil
		trustLevel := DetermineTrustLevel(plugin.RootPath, false, 0, hasStructuredSurface)
		fmt.Printf("  %s (%s) - %s\n", name, version, desc)
		fmt.Printf("    Path: %s\n", plugin.RootPath)
		fmt.Printf("    Format: %s\n", plugin.Format)
		if plugin.Format == "bundle" {
			fmt.Printf("    Bundle format: %s\n", plugin.BundleFormat)
		}
		fmt.Printf("    Trust: %s\n", trustLevel)
		fmt.Printf("    Trust reason: %s\n", DetermineTrustReason(trustLevel, 0, hasStructuredSurface))
		fmt.Printf("    Declared: %s\n", BuildDeclaredSurfaceSummary(plugin.Manifest, &plugin))
	}

	return 0, nil
}

func pluginCommandRemove(_ context.Context, args []string) (int, error) {
	if len(args) == 0 {
		return 2, errors.New("Usage: openclaw plugins remove <plugin-name>")
	}

	pluginName := args[0]
	global := slices.Contains(args, "--global") || slices.Contains(args, "-g")
	extensionsDir := resolveExtensionsDir(global)

	var targetDir = filepath.Join(extensionsDir, pluginName)
	if !util.DirectoryExists(targetDir) {
		// Try sanitized name
		targetDir = filepath.Join(extensionsDir, SanitizePackageName(pluginName))
	}

	if !util.DirectoryExists(targetDir) {
		return 1, fmt.Errorf("Plugin '%s' not found in %s", pluginName, extensionsDir)
	}

	os.Remove(targetDir)
	fmt.Printf("Removed '%s' from %s\n", pluginName, extensionsDir)
	fmt.Println("Restart the gateway to unload the plugin.")
	return 0, nil
}

func unknownPluginCommand(subcommand string) (int, error) {
	fmt.Printf("unknown subcommand: %s\n", subcommand)
	pluginCommandPrintHelp()
	return 2, nil
}

func resolveExtensionsDir(global bool) string {
	extensionsDir := func() string {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, ".openclaw", "extensions")
	}
	if global {
		return extensionsDir()
	}

	workspace := os.Getenv("OPENCLAW_WORKSPACE")
	if workspace != "" {
		return filepath.Join(workspace, ".openclaw", "extensions")
	}

	return extensionsDir()
}

func pluginCommandInstall(ctx context.Context, args []string) (int, error) {
	packageSpec := ""
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			packageSpec = arg
			break
		}
	}

	if packageSpec == "" {
		fmt.Println("Usage: openclaw plugins install <package-name|local-path>")
		return 2, nil
	}

	global := slices.Contains(args, "--global") || slices.Contains(args, "-g")
	dryRun := slices.Contains(args, "--dry-run")
	extensionsDir := resolveExtensionsDir(global)

	os.MkdirAll(extensionsDir, 0755)

	// Check if it's a local path
	if util.DirectoryExists(packageSpec) || util.FileExists(packageSpec) {
		return pluginCommandInstallFromLocal(ctx, packageSpec, extensionsDir, dryRun)
	}

	// Install from npm/ClawHub
	return pluginCommandInstallFromNpm(ctx, packageSpec, extensionsDir, dryRun)
}

func DetermineTrustLevel(sourceLabel string, sourceIsNpm bool, errorCount int, hasStructuredSurface bool) string {
	if sourceIsNpm && strings.HasPrefix(sourceLabel, "@openclaw/") {
		return "first-party"
	}

	if hasStructuredSurface && errorCount == 0 {
		return "upstream-compatible"
	}

	return "untrusted"
}

func DetermineTrustReason(trustLevel string, errorCount int, hasStructuredSurface bool) string {
	if trustLevel == "first-party" {
		return "Package source matches an official OpenClaw or ClawDotNet scope."
	}

	if trustLevel == "upstream-compatible" {
		return "Plugin declares structured OpenClaw surfaces and passed manifest, package metadata, and static compatibility checks."
	}

	if hasStructuredSurface && errorCount > 0 {
		return "Plugin declares OpenClaw surfaces, but compatibility verification reported blocking errors."
	}

	return "Plugin relies on entry discovery without a structured manifest-backed capability declaration."
}

func quote(path string) string {
	if strings.Contains(path, " ") {
		return fmt.Sprintf("\"%s\"", path)
	}
	return path
}

func FindEntryFile(rootPath string) string {
	packageJsonPath := filepath.Join(rootPath, "package.json")

	// 1. 检查 package.json 是否存在
	if fileData, err := os.ReadFile(packageJsonPath); err == nil {
		var root map[string]any
		// 解析 JSON
		if json.Unmarshal(fileData, &root) == nil {
			if openClaw, ok := root["openclaw"].(map[string]any); ok {
				var extensions []any

				// 优先检查 runtimeExtensions，其次检查 extensions
				if rtExts, exists := openClaw["runtimeExtensions"]; exists {
					if arr, ok := rtExts.([]any); ok {
						extensions = arr
					}
				}
				if extensions == nil {
					if srcExts, exists := openClaw["extensions"]; exists {
						if arr, ok := srcExts.([]any); ok {
							extensions = arr
						}
					}
				}

				// 遍历扩展路径
				for _, ext := range extensions {
					if relPath, ok := ext.(string); ok && relPath != "" {
						if resolvedPath, ok := core.PluginDiscoveryInstance.TryResolveContainedPath(rootPath, relPath); ok {
							if info, err := os.Stat(resolvedPath); err == nil && !info.IsDir() {
								return resolvedPath
							}
						}
					}
				}
			}
		}
	}

	// 2. 检查默认候选文件列表
	candidates := []string{
		"index.js", "index.mjs", "index.cjs", "index.ts",
		"src/index.js", "src/index.mjs", "src/index.cjs", "src/index.ts",
	}

	for _, candidate := range candidates {
		fullPath := filepath.Join(rootPath, candidate)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			return fullPath
		}
	}

	return ""
}

func SanitizePackageName(name string) string {
	replacer := strings.NewReplacer(
		"@", " ",
		"/", "-",
		" ", "-",
	)
	return replacer.Replace(name)
}

func ResolvePluginName(packageDir string) *string {
	// Try manifest (openclaw.plugin.json)
	manifestPath := filepath.Join(packageDir, "openclaw.plugin.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err == nil {
			// Check for "id" string property
			if id, ok := doc["id"].(string); ok && id != "" {
				return &id
			}
			// Check for "name" string property
			if name, ok := doc["name"].(string); ok && name != "" {
				sanitized := SanitizePackageName(name)
				return &sanitized
			}
		}
	}

	// Try package.json
	packageJsonPath := filepath.Join(packageDir, "package.json")
	if data, err := os.ReadFile(packageJsonPath); err == nil {
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err == nil {
			// Check for "name" string property
			if name, ok := doc["name"].(string); ok && name != "" {
				sanitized := SanitizePackageName(name)
				return &sanitized
			}
		}
	}

	return nil
}

func InspectCandidate(rootPath, sourceLabel string, sourceIsNpm bool) PluginInstallInspection {
	manifestPath := filepath.Join(rootPath, "openclaw.plugin.json")
	var manifest *core.PluginManifest
	var packageName *string
	var version *string
	var description *string
	hasManifest := false
	hasExtensionsConfig := false

	// 1. Read openclaw.plugin.json if it exists
	if _, err := os.Stat(manifestPath); err == nil {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return PluginInstallInspectionFailure(fmt.Sprintf("Invalid plugin manifest at %s: %s", manifestPath, err.Error()))
		}
		manifest = &core.PluginManifest{}
		if err := json.Unmarshal(data, manifest); err != nil {
			return PluginInstallInspectionFailure(fmt.Sprintf("Invalid plugin manifest at %s: %s", manifestPath, err.Error()))
		}
		hasManifest = manifest != nil
	}

	// 2. Read package.json if it exists
	packageJsonPath := filepath.Join(rootPath, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		data, err := os.ReadFile(packageJsonPath)
		if err != nil {
			return PluginInstallInspectionFailure(fmt.Sprintf("Invalid package.json at %s: %s", packageJsonPath, err.Error()))
		}
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			return PluginInstallInspectionFailure(fmt.Sprintf("Invalid package.json at %s: %s", packageJsonPath, err.Error()))
		}

		if name, ok := root["name"].(string); ok {
			packageName = &name
		}
		if ver, ok := root["version"].(string); ok {
			version = &ver
		}
		if desc, ok := root["description"].(string); ok {
			description = &desc
		}

		if openClaw, ok := root["openclaw"].(map[string]any); ok {
			hasExtensionsConfig = checkExtensionsConfig(openClaw)
		}
	}

	// 3. Plugin discovery & bundle checking
	discovery := core.PluginDiscoveryInstance.DiscoverWithDiagnostics(&core.PluginsConfig{
		Load: core.PluginLoadConfig{Paths: []string{rootPath}},
	}, "")

	var discoveredPlugin *core.DiscoveredPlugin
	if len(discovery.Plugins) > 0 {
		discoveredPlugin = &discovery.Plugins[0]
	}

	isBundle := discoveredPlugin != nil && discoveredPlugin.Format == "bndle"
	var entryPath string
	if isBundle {
		entryPath = rootPath
	} else if discoveredPlugin != nil && discoveredPlugin.EntryPath != "" {
		entryPath = discoveredPlugin.EntryPath
	} else {
		entryPath = FindEntryFile(rootPath)
	}

	if entryPath == "" {
		return PluginInstallInspectionFailure(fmt.Sprintf("No plugin entry file or compatible Codex/Claude/Cursor bundle was found under %s.", rootPath))
	}

	// 4. Effective Manifest resolution
	var effectiveManifest core.PluginManifest
	if manifest != nil {
		effectiveManifest = *manifest
	} else if discoveredPlugin != nil {
		effectiveManifest = discoveredPlugin.Manifest
	} else {
		resolvedName := ResolvePluginName(rootPath)
		fallbackName := ""
		if packageName != nil {
			fallbackName = SanitizePackageName(*packageName)
		} else {
			fallbackName = SanitizePackageName(filepath.Base(rootPath))
		}
		if resolvedName != nil {
			fallbackName = *resolvedName
		}
		effectiveManifest = core.PluginManifest{
			Id:          fallbackName,
			Name:        util.Deref(packageName),
			Description: util.Deref(description),
			Version:     util.Deref(version),
		}
	}

	var warnings []string
	var diagnostics []core.PluginCompatibilityDiagnostic
	for _, report := range discovery.Reports {
		diagnostics = append(diagnostics, report.Diagnostics...)
	}

	if !hasManifest && !isBundle {
		warnings = append(warnings, "No openclaw.plugin.json manifest was found. Install is allowed, but declared capabilities and config validation metadata are limited.")
	}
	if !hasExtensionsConfig && !hasManifest && !isBundle {
		warnings = append(warnings, "Package relies on standalone entry-file discovery. Review the source before enabling it on a   bind.")
	}

	declaredChannels := effectiveManifest.Channels
	if declaredChannels == nil {
		declaredChannels = []string{}
	}
	declaredProviders := effectiveManifest.Providers
	if declaredProviders == nil {
		declaredProviders = []string{}
	}
	declaredSkills := effectiveManifest.Skills
	if declaredSkills == nil {
		declaredSkills = []string{}
	}

	if !isBundle && effectiveManifest.ConfigSchema != nil {
		diagnostics = append(diagnostics, core.PluginConfigValidatorinstance.Validate(effectiveManifest, nil)...)
	}

	if discoveredPlugin != nil && !isBundle {
		diagnostics = append(diagnostics, core.PluginPackageCompatibilityInstance.Validate(*discoveredPlugin)...)
	}

	if !isBundle {
		diagnostics = append(diagnostics, InspectUnsupportedRuntimeSurfaces(rootPath, diagnostics)...)
	}

	if isBundle && discoveredPlugin != nil {
		for _, capability := range discoveredPlugin.BundleDetectedCapabilities {
			diagnostics = append(diagnostics, core.PluginCompatibilityDiagnostic{
				Severity: "warning",
				Code:     "bundle_capability_detected_only",
				Message:  fmt.Sprintf("Bundle capability '%s' is detected", capability),
				Surface:  capability,
				Path:     rootPath,
			})
		}
	}

	// 5. Skill validation
	for _, skillDir := range declaredSkills {
		resolvedSkillDir, ok := core.PluginDiscoveryInstance.TryResolveContainedPath(rootPath, skillDir)
		if !ok {
			diagnostics = append(diagnostics, core.PluginCompatibilityDiagnostic{
				Severity: "error",
				Code:     "skill_path_outside_root",
				Message:  fmt.Sprintf("Skill directory '%s' resolves outside the plugin root.", skillDir),
				Surface:  "skills",
				Path:     rootPath,
			})
			continue
		}

		if _, err := os.Stat(resolvedSkillDir); os.IsNotExist(err) {
			diagnostics = append(diagnostics, core.PluginCompatibilityDiagnostic{
				Severity: "error",
				Code:     "skill_directory_missing",
				Message:  fmt.Sprintf("Declared skill directory '%s' does not exist.", skillDir),
				Surface:  "skills",
				Path:     resolvedSkillDir,
			})
			continue
		}

		rootSkillFile := filepath.Join(resolvedSkillDir, "SKILL.md")
		nestedSkillFiles, _ := filepath.Glob(filepath.Join(resolvedSkillDir, "**/SKILL.md"))

		pathBase := filepath.Base(strings.TrimSuffix(resolvedSkillDir, string(filepath.Separator)))
		isBundleCommandRoot := isBundle && strings.EqualFold(pathBase, "commands")

		hasBundleCommands := false
		if isBundleCommandRoot {
			mdFiles, _ := filepath.Glob(filepath.Join(resolvedSkillDir, "**/*.md"))
			hasBundleCommands = len(mdFiles) > 0
		}

		if _, err := os.Stat(rootSkillFile); os.IsNotExist(err) && len(nestedSkillFiles) == 0 && !hasBundleCommands {
			diagnostics = append(diagnostics, core.PluginCompatibilityDiagnostic{
				Severity: "warning",
				Code:     "skill_directory_empty",
				Message:  fmt.Sprintf("Declared skill directory '%s' does not currently contain a SKILL.md file.", skillDir),
				Surface:  "skills",
				Path:     resolvedSkillDir,
			})
		}
	}

	// 6. Metrics & Status calculation
	errorCount := 0
	for _, d := range diagnostics {
		if strings.EqualFold(d.Severity, "error") {
			errorCount++
		}
	}
	warningCount := len(diagnostics) - errorCount

	compatibilityStatus := "manifest-valid"
	if errorCount > 0 {
		compatibilityStatus = "errors"
	} else if warningCount > 0 {
		compatibilityStatus = "warnings"
	}

	hasStructuredSurface := (isBundle && discoveredPlugin != nil && len(discoveredPlugin.BundleMappedCapabilities) > 0) ||
		len(declaredChannels) > 0 ||
		len(declaredProviders) > 0 ||
		len(declaredSkills) > 0 ||
		effectiveManifest.ConfigSchema != nil

	trustLevel := DetermineTrustLevel(sourceLabel, sourceIsNpm, errorCount, hasStructuredSurface)
	trustReason := DetermineTrustReason(trustLevel, errorCount, hasStructuredSurface)

	bundleFormatPtr := "bundle"
	if discoveredPlugin != nil {
		bundleFormatPtr = discoveredPlugin.BundleFormat
	}

	displayName := effectiveManifest.Id
	if effectiveManifest.Name != "" {
		displayName = effectiveManifest.Name
	}

	ver := "?"
	if effectiveManifest.Version != "" {
		ver = effectiveManifest.Version
	} else if version != nil {
		ver = *version
	}

	desc := ""
	if effectiveManifest.Description != "" {
		desc = effectiveManifest.Description
	} else if description != nil {
		desc = *description
	}

	format := "native"
	if discoveredPlugin != nil {
		format = discoveredPlugin.Format
	}

	return PluginInstallInspection{
		Success:             true,
		CanInstall:          errorCount == 0,
		PluginId:            effectiveManifest.Id,
		DisplayName:         displayName,
		Version:             ver,
		Description:         desc,
		EntryPath:           entryPath,
		Format:              format,
		BundleFormat:        bundleFormatPtr,
		TrustLevel:          trustLevel,
		TrustReason:         trustReason,
		CompatibilityStatus: compatibilityStatus,
		ErrorCount:          errorCount,
		WarningCount:        warningCount,
		DeclaredSurface:     BuildDeclaredSurfaceSummary(effectiveManifest, discoveredPlugin),
		Diagnostics:         diagnostics,
		Warnings:            warnings,
	}
}

func BuildDeclaredSurfaceSummary(manifest core.PluginManifest, plugin *core.DiscoveredPlugin) string {
	items := []string{}
	if plugin.Format == "bundle" {
		items = append(items, fmt.Sprintf("bundle=%s", plugin.BundleFormat))
		if len(plugin.BundleMappedCapabilities) > 0 {
			items = append(items, fmt.Sprintf("mapped=%s", strings.Join(plugin.BundleMappedCapabilities, ",")))
		}
		if len(plugin.BundleDetectedCapabilities) > 0 {
			items = append(items, fmt.Sprintf("detected_only=%s", strings.Join(plugin.BundleDetectedCapabilities, ",")))
		}
	}

	channels := manifest.Channels
	providers := manifest.Providers
	skills := manifest.Skills
	if len(channels) > 0 {
		items = append(items, fmt.Sprintf("channels=%s", strings.Join(channels, ",")))
	}
	if len(providers) > 0 {
		items = append(items, fmt.Sprintf("providers=%s", strings.Join(providers, ",")))
	}
	if len(skills) > 0 {
		items = append(items, fmt.Sprintf("skills=%d", len(skills)))
	}
	if manifest.ConfigSchema != nil {
		items = append(items, "config_schema")
	}

	if len(items) == 0 {
		return "entry-only"
	}
	return strings.Join(items, " | ")
}

// --- Supporting Types and Helper Stubs ---

func checkExtensionsConfig(openClaw map[string]any) bool {
	checkArray := func(key string) bool {
		if val, ok := openClaw[key].([]any); ok && len(val) > 0 {
			return true
		}
		return false
	}
	return checkArray("runtimeExtensions") || checkArray("extensions")
}

func PluginInstallInspectionFailure(message string) PluginInstallInspection {
	return PluginInstallInspection{
		Success:    false,
		CanInstall: false,
		Warnings:   []string{message},
	}
}

func pluginCommandPrintInspection(inspection PluginInstallInspection) {
	fmt.Printf("Plugin: %s (%s)\n", inspection.DisplayName, inspection.PluginId)
	fmt.Printf("Version: %s\n", inspection.Version)
	fmt.Printf("Format: %s\n", inspection.Format)

	if inspection.Format == "bundle" {
		fmt.Printf("Bundle format: %s\n", inspection.BundleFormat)
	}
	if strings.TrimSpace(inspection.Description) != "" {
		fmt.Printf("Description: %s\n", inspection.Description)
	}

	fmt.Printf("Trust: %s\n", inspection.TrustLevel)
	fmt.Printf("Trust reason: %s\n", inspection.TrustReason)
	fmt.Printf("Compatibility: %s (errors=%d, warnings=%d)\n", inspection.CompatibilityStatus, inspection.ErrorCount, inspection.WarningCount)
	fmt.Printf("Declared: %s\n", inspection.DeclaredSurface)
	fmt.Printf("Entry: %s\n", inspection.EntryPath)

	for _, warning := range inspection.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}

	for _, diagnostic := range inspection.Diagnostics {
		severity := "Warning"
		if strings.EqualFold(diagnostic.Severity, "error") {
			severity = "Error"
		}
		fmt.Printf("%s: [%s] %s\n", severity, diagnostic.Code, diagnostic.Message)
	}

	if !inspection.CanInstall {
		fmt.Println("Install blocked: compatibility verification reported one or more errors.")
	}
}

func getDistinctDiagnosticCodes(diagnostics []core.PluginCompatibilityDiagnostic) []string {
	var codes []string
	seen := make(map[string]bool)
	for _, item := range diagnostics {
		if !seen[item.Code] {
			seen[item.Code] = true
			codes = append(codes, item.Code)
		}
	}
	return codes
}

func hasLocalJiti(entryPath, rootDir string) bool {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return false
	}
	absRoot = filepath.Clean(absRoot)

	absEntry, err := filepath.Abs(entryPath)
	if err != nil {
		return false
	}

	current := filepath.Dir(absEntry)

	isWindows := runtime.GOOS == "windows"

	equals := func(a, b string) bool {
		if isWindows {
			return strings.EqualFold(a, b)
		}
		return a == b
	}

	startsWith := func(s, prefix string) bool {
		if isWindows {
			return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
		}
		return strings.HasPrefix(s, prefix)
	}

	sepStr := string(filepath.Separator)
	rootWithSep := absRoot + sepStr

	for current != "" && (equals(current, absRoot) || startsWith(current, rootWithSep)) {
		jitiPath := filepath.Join(current, "node_modules", "jiti")
		if info, err := os.Stat(jitiPath); err == nil && info.IsDir() {
			return true
		}

		if equals(current, absRoot) {
			break
		}

		parent := filepath.Dir(current)
		if equals(parent, current) {
			break
		}
		current = parent
	}

	return false
}

type PluginRuntimeInspection struct {
	Compatible      bool
	ErrorMsg        string
	Diagnostics     []core.PluginCompatibilityDiagnostic
	ToolCount       int
	ChannelCount    int
	CommandCount    int
	CliCommandCount int
	ProviderCount   int
}

func FailurePluginRuntimeInspection(msg string) *PluginRuntimeInspection {
	return &PluginRuntimeInspection{ErrorMsg: msg}
}

func InspectRuntime(ctx context.Context, entryPath, pluginId string) (*PluginRuntimeInspection, error) {
	bridgeScript := ResolveBridgeScriptPath()
	if bridgeScript == "" {
		return nil, errors.New("plugin bridge script was not found. Reinstall or republish the OpenClaw CLI")
	}

	nodeExecutable := "node"
	if runtime.GOOS == "windows" {
		nodeExecutable = "node.exe"
	}

	absEntryPath, err := filepath.Abs(entryPath)
	if err != nil {
		absEntryPath = entryPath
	}

	initRequest := core.BridgeInitRequest{
		EntryPath: absEntryPath,
		PluginId:  pluginId,
		Config:    new(json.RawMessage("{}")),
		Transport: &core.BridgeTransportRuntimeConfig{Mode: "stdio"},
	}

	initParams, err := json.Marshal(initRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal init request: %w", err)
	}

	bridgeReq := core.BridgeRequest{
		Id:     "inspection-1",
		Method: "init",
		Params: json.RawMessage(initParams),
	}

	reqJson, err := json.Marshal(bridgeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bridge request: %w", err)
	}
	// 加上换行符，对应 Node 端按行读取
	reqJson = append(reqJson, '\n')

	workDir := filepath.Dir(entryPath)
	// 直接调用带有 stdin 支持的通用方法
	res := util.RunProcessWithStdin(ctx, nodeExecutable, []string{"--experimental-vm-modules", bridgeScript}, workDir, reqJson, 20, 1024*1024, 1024*1024)

	if res.Error != "" {
		if res.Error == "Error: Execution timed out." {
			return nil, errors.New("plugin runtime inspection timed out after 20 seconds")
		}
		if strings.TrimSpace(res.StdoutText) == "" && res.StderrText != "" {
			return nil, fmt.Errorf("plugin bridge exited without an inspection response. %s", strings.TrimSpace(res.StderrText))
		}
		return nil, errors.New(res.Error)
	}

	responseLine := strings.TrimSpace(res.StdoutText)
	if responseLine == "" {
		if res.StderrText != "" {
			return nil, fmt.Errorf("plugin bridge exited without an inspection response. %s", strings.TrimSpace(res.StderrText))
		}
		return nil, errors.New("plugin bridge exited without an inspection response.")
	}

	var bridgeResp core.BridgeResponse
	if err := json.Unmarshal([]byte(responseLine), &bridgeResp); err != nil {
		return nil, fmt.Errorf("plugin bridge returned an unreadable inspection result: %w", err)
	}

	if bridgeResp.Error != nil {
		return nil, errors.New(bridgeResp.Error.Message)
	}
	if bridgeResp.Result == nil || len(*bridgeResp.Result) == 0 {
		return nil, errors.New("plugin bridge returned an empty inspection result.")
	}

	var initResult core.BridgeInitResult
	if err := json.Unmarshal(*bridgeResp.Result, &initResult); err != nil {
		return nil, errors.New("plugin bridge returned an unreadable inspection result.")
	}

	return &PluginRuntimeInspection{
		Compatible:      initResult.Compatible,
		Diagnostics:     initResult.Diagnostics,
		ToolCount:       len(initResult.Tools),
		ChannelCount:    len(initResult.Channels),
		CommandCount:    len(initResult.Commands),
		CliCommandCount: len(initResult.CliCommands),
		ProviderCount:   len(initResult.Providers),
	}, nil
}

func InstallPreparedDirectory(ctx context.Context, sourceDir, targetDir, sourceLabel string, sourceIsNpm bool) (bool, error) {
	parentDir := filepath.Dir(targetDir)
	if parentDir == "" || parentDir == targetDir {
		return false, fmt.Errorf("plugin target '%s' has no parent directory", targetDir)
	}

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return false, err
	}

	suffix := util.CleanUUID()[:12]
	targetBase := filepath.Base(targetDir)
	stagingDir := filepath.Join(parentDir, fmt.Sprintf(".%s.installing-%s", targetBase, suffix))
	backupDir := filepath.Join(parentDir, fmt.Sprintf(".%s.backup-%s", targetBase, suffix))
	defer func() {
		if _, err := os.Stat(stagingDir); err == nil {
			if removeErr := os.RemoveAll(stagingDir); removeErr != nil {
				fmt.Printf("Unable to delete plugin staging directory '%s': %v\n", stagingDir, removeErr)
			}
		}

		if _, err := os.Stat(backupDir); err == nil {
			if _, targetErr := os.Stat(targetDir); targetErr == nil {
				if removeErr := os.RemoveAll(backupDir); removeErr != nil {
					fmt.Printf("Unable to delete plugin backup directory '%s': %v\n", backupDir, removeErr)
				}
			}
		}
	}()

	opt := cp.Options{
		Skip: func(srcInfo os.FileInfo, src, dest string) (bool, error) {
			if srcInfo.IsDir() && (srcInfo.Name() == "node_modules" || srcInfo.Name() == ".git") {
				return true, nil
			}
			return false, nil
		},
	}

	if err := cp.Copy("source_dir", "destination_dir", opt); err != nil {
		return false, fmt.Errorf("plugin installation failed; the existing plugin was preserved: %w", err)
	}

	stagedInspection := InspectCandidate(stagingDir, sourceLabel, sourceIsNpm)
	if !stagedInspection.Success {
		return false, fmt.Errorf("staged plugin inspection failed; the existing plugin was preserved: %s", stagedInspection.ErrorMessage)
	}
	if !stagedInspection.CanInstall {
		codes := getDistinctDiagnosticCodes(stagedInspection.Diagnostics)
		return false, fmt.Errorf("staged plugin compatibility failed; the existing plugin was preserved. Diagnostics: %s", strings.Join(codes, ", "))
	}

	packageJson := filepath.Join(stagingDir, "package.json")
	if util.FileExists(packageJson) && stagedInspection.Format != "bundle" {
		fmt.Println("Installing dependencies in staging...")
		npmInstall := util.RunProcess(ctx, "npm", []string{"install --ignore-scripts --omit=dev --omit=optional"}, stagingDir, 0, 0, 0)
		if npmInstall.ExitCode != 0 {
			return false, fmt.Errorf("dependency installation failed; the existing plugin was preserved: %s", npmInstall.StderrError)
		}

		stagedInspection = InspectCandidate(stagingDir, sourceLabel, sourceIsNpm)
		if !stagedInspection.Success {
			return false, fmt.Errorf("post-dependency inspection failed; the existing plugin was preserved: %s", stagedInspection.ErrorMessage)
		}
		if !stagedInspection.CanInstall {
			codes := getDistinctDiagnosticCodes(stagedInspection.Diagnostics)
			return false, fmt.Errorf("post-dependency compatibility inspection failed; the existing plugin was preserved. Diagnostics: %s", strings.Join(codes, ", "))
		}
	}

	if strings.EqualFold(filepath.Ext(stagedInspection.EntryPath), ".ts") &&
		stagedInspection.Format != "bundle" &&
		!hasLocalJiti(stagedInspection.EntryPath, stagingDir) {
		fmt.Println("Installing the TypeScript runtime dependency jiti in staging...")
		jitiInstall := util.RunProcess(ctx, "npm", []string{"install --ignore-scripts --no-save --omit=dev --omit=optional jiti"}, stagingDir, 0, 0, 0)
		if jitiInstall.ExitCode != 0 {
			return false, fmt.Errorf("TypeScript runtime dependency installation failed; the existing plugin was preserved: %s", jitiInstall.StderrError)
		}
	}

	if stagedInspection.Format != "bundle" {
		runtimeInspection, err := InspectRuntime(ctx, stagedInspection.EntryPath, stagedInspection.PluginId)
		if err != nil {
			return false, fmt.Errorf("plugin runtime inspection failed; the existing plugin was preserved: %w", err)
		}
		if !runtimeInspection.Compatible {
			var details string
			if len(runtimeInspection.Diagnostics) == 0 {
				if runtimeInspection.ErrorMsg != "" {
					details = runtimeInspection.ErrorMsg
				} else {
					details = "unknown runtime inspection failure"
				}
			} else {
				details = strings.Join(getDistinctDiagnosticCodes(runtimeInspection.Diagnostics), ", ")
			}
			return false, fmt.Errorf("plugin runtime inspection failed; the existing plugin was preserved. Diagnostics: %s", details)
		}
	}

	if _, err := os.Stat(targetDir); err == nil {
		fmt.Printf("Replacing existing plugin '%s' atomically...\n", filepath.Base(targetDir))
		if err := os.Rename(targetDir, backupDir); err != nil {
			return false, fmt.Errorf("plugin installation failed; the existing plugin was preserved: %w", err)
		}
	}

	if err := os.Rename(stagingDir, targetDir); err != nil {
		if _, statErr := os.Stat(backupDir); statErr == nil {
			if _, targetErr := os.Stat(targetDir); os.IsNotExist(targetErr) {
				_ = os.Rename(backupDir, targetDir)
			}
		}
		return false, fmt.Errorf("plugin installation failed; the existing plugin was preserved: %w", err)
	}

	if _, err := os.Stat(backupDir); err == nil {
		if err := os.RemoveAll(backupDir); err != nil {
			fmt.Printf("Unable to delete stale plugin backup '%s': %v\n", backupDir, err)
		}
	}

	return true, nil
}

func pluginCommandInstallFromLocal(ctx context.Context, localPath, extensionsDir string, dryRun bool) (int, error) {
	sourcePath, err := filepath.Abs(localPath)
	if err != nil {
		return 1, err
	}

	if util.FileExists(sourcePath) && strings.HasPrefix(sourcePath, ".tgz") {
		// Extract tarball
		tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("openclaw-install-%s", util.CleanUUID())[:24])
		os.MkdirAll(tempDir, 0755)
		defer os.Remove(tempDir)

		extractResult := util.RunProcess(ctx, "tar", []string{fmt.Sprintf("xzf %s -C %s", quote(sourcePath), quote(tempDir))}, tempDir, 0, 0, 0)
		if extractResult.ExitCode != 0 {
			return 1, fmt.Errorf("Failed to extract: %s", extractResult.Error)
		}

		var packageDir = filepath.Join(tempDir, "package")
		if !util.DirectoryExists(packageDir) {
			dirs := util.EnumerateAllFiles(tempDir)
			packageDir = tempDir
			if len(dirs) > 0 {
				packageDir = dirs[0]
			}
		}

		var inspection = InspectCandidate(packageDir, localPath, false)
		if !inspection.Success {
			return 1, errors.New(inspection.ErrorMessage)
		}

		pluginCommandPrintInspection(inspection)
		if !inspection.CanInstall {
			return 1, nil
		}
		if dryRun {
			return 0, nil
		}

		pluginName := ResolvePluginName(packageDir)
		if pluginName == nil {
			pluginName = new(util.GetFileNameWithoutExtension(packageDir))
		}

		targetDir := filepath.Join(extensionsDir, *pluginName)
		installResult, err := InstallPreparedDirectory(ctx, packageDir, targetDir, localPath, false)
		if !installResult {
			return 1, errors.New(err.Error())
		}

		fmt.Printf("Installed '%s' from tarball to %s\n", *pluginName, targetDir)
		return 0, nil
	}

	if util.DirectoryExists(sourcePath) {
		inspection := InspectCandidate(sourcePath, localPath, false)
		if !inspection.Success {
			return 1, errors.New(inspection.ErrorMessage)
		}

		pluginCommandPrintInspection(inspection)
		if !inspection.CanInstall {
			return 1, errors.New("can not install plugin")
		}
		if dryRun {
			return 0, nil
		}

		pluginName := ResolvePluginName(sourcePath)
		if pluginName == nil {
			pluginName = new(filepath.Base(sourcePath))
		}

		var targetDir = filepath.Join(extensionsDir, *pluginName)
		installResult, err := InstallPreparedDirectory(ctx, sourcePath, targetDir, localPath, false)
		if !installResult {
			return 1, errors.New(err.Error())
		}

		fmt.Printf("Installed '%s' from local directory to %s\n", *pluginName, targetDir)
		return 0, nil
	}

	return 1, fmt.Errorf("Path not found: %s", localPath)
}

func pluginCommandInstallFromNpm(ctx context.Context, packageSpec, extensionsDir string, dryRun bool) (int, error) {
	if dryRun {
		fmt.Printf("Dry-run install for %s from npm...\n", packageSpec)
	} else {
		fmt.Printf("Installing %s from npm...\n", packageSpec)
	}

	// Use npm pack to download the tarball, then extract into extensions dir
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("openclaw-install-%s", util.CleanUUID())[:24])
	os.MkdirAll(tempDir, 0755)
	defer os.Remove(tempDir)
	// Step 1: npm pack to download tarball
	packResult := util.RunProcess(ctx, "npm", []string{fmt.Sprintf("pack %s --pack-destination %s", packageSpec, quote(tempDir))}, tempDir, 0, 0, 0)
	if packResult.ExitCode != 0 {
		return 1, fmt.Errorf("Failed to download package: %s", packResult.Error)
	}
	// Find the downloaded tarball
	tarballs, _ := filepath.Glob(filepath.Join(tempDir, "*.tgz"))
	if len(tarballs) == 0 {
		return 1, errors.New("No tarball downloaded.")
	}

	tarball := tarballs[0]

	// Step 2: Extract tarball into a temp staging directory
	stagingDir := filepath.Join(tempDir, "staging")
	os.MkdirAll(stagingDir, 0755)

	extractResult := util.RunProcess(ctx, "tar", []string{fmt.Sprintf("xzf %s -C %s", quote(tarball), quote(stagingDir))}, tempDir, 0, 0, 0)
	if extractResult.ExitCode != 0 {
		return 1, fmt.Errorf("Failed to extract package: %s", extractResult.Error)
	}

	// npm pack creates a 'package' directory inside the tarball
	var packageDir = filepath.Join(stagingDir, "package")
	if !util.DirectoryExists(packageDir) {
		dirs := util.EnumerateAllFiles(stagingDir)
		packageDir = tempDir
		if len(dirs) > 0 {
			packageDir = dirs[0]
		}
	}

	// Step 3: Inspect package before copying into extensions
	inspection := InspectCandidate(packageDir, packageSpec, true)
	if !inspection.Success {
		return 1, errors.New(inspection.ErrorMessage)
	}

	pluginCommandPrintInspection(inspection)
	if !inspection.CanInstall {
		return 1, errors.New("can not install plugin")
	}
	if dryRun {
		return 0, nil
	}

	// Step 4: Determine plugin name from manifest or package.json
	pluginName := ResolvePluginName(packageDir)
	if pluginName == nil {
		pluginName = new(SanitizePackageName(packageSpec))
	}

	// Step 5: stage dependencies and atomically replace the installed plugin
	var targetDir = filepath.Join(extensionsDir, *pluginName)
	installResult, err := InstallPreparedDirectory(ctx, packageDir, targetDir, packageSpec, true)
	if !installResult {
		return 1, errors.New(err.Error())
	}

	fmt.Printf("Installed '%s' to %s\n", *pluginName, targetDir)
	fmt.Println("Restart the gateway to load the plugin.")
	return 0, nil
}
