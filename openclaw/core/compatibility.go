package core

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed public-smoke.json
var PublicCompatibilityCatalogEmbeddedFiles embed.FS
var (
	publicCompatibilityCatalogOnce     sync.Once
	publicCompatibilityCatalogInstance *CompatibilityCatalogResponse
	publicCompatibilityCatalogErr      error
)

const PublicCompatibilityCatalogResourceName = "public-smoke.json"

// 获取 Lazy 加载的单例数据
func getPublicCompatibilityCatalog() (*CompatibilityCatalogResponse, error) {
	publicCompatibilityCatalogOnce.Do(func() {
		publicCompatibilityCatalogInstance, publicCompatibilityCatalogErr = loadPublicCompatibilityCatalog()
	})
	return publicCompatibilityCatalogInstance, publicCompatibilityCatalogErr
}

type PublicCompatibilityCatalog struct{}

var PublicCompatibilityCatalogInstance = &PublicCompatibilityCatalog{}

func (*PublicCompatibilityCatalog) GetCatalog(compatibilityStatus, kind, category string) (*CompatibilityCatalogResponse, error) {
	all, err := getPublicCompatibilityCatalog()
	if err != nil {
		return nil, err
	}

	var filteredItems []CompatibilityCatalogEntry
	for _, item := range all.Items {
		if !matchesPublicCompatibilityCatalog(item.CompatibilityStatus, compatibilityStatus) {
			continue
		}
		if !matchesPublicCompatibilityCatalog(item.Kind, kind) {
			continue
		}
		if !matchesPublicCompatibilityCatalog(item.Category, category) {
			continue
		}
		filteredItems = append(filteredItems, item)
	}

	// 排序逻辑：CompatibilityStatus -> Subject -> Id (均为不区分大小写)
	sort.Slice(filteredItems, func(i, j int) bool {
		cmpStatus := strings.Compare(strings.ToLower(filteredItems[i].CompatibilityStatus), strings.ToLower(filteredItems[j].CompatibilityStatus))
		if cmpStatus != 0 {
			return cmpStatus < 0
		}

		cmpSubject := strings.Compare(strings.ToLower(filteredItems[i].Subject), strings.ToLower(filteredItems[j].Subject))
		if cmpSubject != 0 {
			return cmpSubject < 0
		}

		return strings.Compare(strings.ToLower(filteredItems[i].ID), strings.ToLower(filteredItems[j].ID)) < 0
	})

	return &CompatibilityCatalogResponse{
		Version: all.Version,
		Source:  all.Source,
		Items:   filteredItems,
	}, nil
}

func matchesPublicCompatibilityCatalog(value string, filter string) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	return strings.EqualFold(value, strings.TrimSpace(filter))
}

func loadPublicCompatibilityCatalog() (*CompatibilityCatalogResponse, error) {
	stream, err := PublicCompatibilityCatalogEmbeddedFiles.ReadFile(PublicCompatibilityCatalogResourceName)
	if err != nil {
		return nil, fmt.Errorf("embedded compatibility catalog '%s' was not found: %w", PublicCompatibilityCatalogResourceName, err)
	}

	var manifest CompatibilityCatalogManifest
	if err := json.Unmarshal(stream, &manifest); err != nil {
		return nil, fmt.Errorf("compatibility catalog manifest could not be parsed: %w", err)
	}

	return CreatePublicCompatibilityCatalog(manifest), nil
}

func CreatePublicCompatibilityCatalog(manifest CompatibilityCatalogManifest) *CompatibilityCatalogResponse {
	items := make([]CompatibilityCatalogEntry, len(manifest.Entries))
	for i, entry := range manifest.Entries {
		items[i] = *mapPublicCompatibilityCatalogEntry(entry)
	}

	return &CompatibilityCatalogResponse{
		Version: manifest.Version,
		Items:   items,
	}
}

func mapPublicCompatibilityCatalogEntry(entry CompatibilityCatalogManifestEntry) *CompatibilityCatalogEntry {
	compatibilityStatus := resolvePublicCompatibilityCatalogStatus(entry)

	installSurface := "npm"
	if entry.Kind == "clawhub-skill" {
		installSurface = "clawhub"
	}

	var subject string
	if entry.Slug != "" {
		subject = entry.Slug
	} else if entry.PackageName != "" {
		subject = entry.PackageName
	} else if entry.PluginId != "" {
		subject = entry.PluginId
	} else {
		subject = entry.Id
	}

	scenarioType := "negative"
	if strings.EqualFold(compatibilityStatus, "compatible") {
		scenarioType = "positive"
	}

	// 保证切片不为 nil
	extraPackages := entry.InstallExtraPackages
	if extraPackages == nil {
		extraPackages = []string{}
	}
	toolNames := entry.ExpectedToolNames
	if toolNames == nil {
		toolNames = []string{}
	}
	skillNames := entry.ExpectedSkillNames
	if skillNames == nil {
		skillNames = []string{}
	}
	diagnosticCodes := entry.ExpectedDiagnosticCodes
	if diagnosticCodes == nil {
		diagnosticCodes = []string{}
	}

	return &CompatibilityCatalogEntry{
		ID:                      entry.Id,
		Category:                entry.Category,
		Kind:                    entry.Kind,
		Subject:                 subject,
		ScenarioType:            scenarioType,
		CompatibilityStatus:     compatibilityStatus,
		InstallSurface:          installSurface,
		InstallCommand:          buildPublicCompatibilityCatalogInstallCommand(entry),
		Summary:                 buildPublicCompatibilityCatalogSummary(entry, compatibilityStatus),
		PackageSpec:             entry.Spec,
		PackageName:             entry.PackageName,
		PluginID:                entry.PluginId,
		SkillSlug:               entry.Slug,
		PackageVersion:          entry.Version,
		ExpectedRelativePath:    entry.ExpectedRelativePath,
		ConfigJSONExample:       entry.ConfigJson,
		InstallExtraPackages:    extraPackages,
		ExpectedToolNames:       toolNames,
		ExpectedSkillNames:      skillNames,
		ExpectedDiagnosticCodes: diagnosticCodes,
		Guidance:                buildPublicCompatibilityCatalogGuidance(entry, compatibilityStatus),
	}
}

func buildPublicCompatibilityCatalogInstallCommand(entry CompatibilityCatalogManifestEntry) string {
	if entry.Kind == "clawhub-skill" {
		slug := requirePublicCompatibilityCatalogField(entry.Slug, entry, "slug")
		suffix := ""
		if strings.TrimSpace(entry.Version) != "" {
			suffix = fmt.Sprintf(" --version %s", entry.Version)
		}
		return fmt.Sprintf("openclaw clawhub install %s%s", slug, suffix)
	}

	spec := requirePublicCompatibilityCatalogField(entry.Spec, entry, "spec")
	return fmt.Sprintf("openclaw plugins install %s --dry-run", spec)
}

func buildPublicCompatibilityCatalogSummary(entry CompatibilityCatalogManifestEntry, compatibilityStatus string) string {
	isCompatible := strings.EqualFold(compatibilityStatus, "compatible")

	switch entry.Category {
	case "pure-skill":
		return "Pinned standalone skill package expected to install and parse through the upstream SKILL.md flow."
	case "js-tool-plugin":
		if isCompatible {
			return "Pinned JavaScript bridge plugin expected to load and expose its declared tools and skills."
		}
	case "ts-jiti-plugin":
		if isCompatible {
			return "Pinned TypeScript bridge plugin expected to load when jiti is present in the plugin dependency tree."
		}
	case "config-schema-plugin":
		return "Negative compatibility scenario proving that invalid plugin config fails fast with structured diagnostics."
	case "unsupported-surface-plugin":
		return "Negative compatibility scenario proving that unsupported upstream plugin surfaces fail explicitly instead of loading partially."
	}

	if isCompatible {
		return "Pinned compatibility scenario expected to load successfully under the documented OpenClaw.NET subset."
	}
	return "Pinned negative compatibility scenario expected to fail with explicit diagnostics."
}

func buildPublicCompatibilityCatalogGuidance(entry CompatibilityCatalogManifestEntry, compatibilityStatus string) []string {
	var guidance []string

	if entry.Kind == "clawhub-skill" {
		guidance = append(guidance, "Remote upstream skills use the ClawHub install flow; local copies can be installed with `openclaw skills install`.")
		if strings.TrimSpace(entry.ExpectedRelativePath) != "" {
			guidance = append(guidance, fmt.Sprintf("Expected installed file: %s.", entry.ExpectedRelativePath))
		}
		return guidance
	}

	guidance = append(guidance, "Run the dry-run installer first so manifest validation and declared surfaces are reported before the package is copied into extensions.")

	if len(entry.InstallExtraPackages) > 0 {
		guidance = append(guidance, fmt.Sprintf("Install extra packages in the plugin dependency tree before load: %s.", strings.Join(entry.InstallExtraPackages, ", ")))
	}

	if strings.TrimSpace(entry.ConfigJson) != "" {
		guidance = append(guidance, "This scenario pins a specific plugin config example; use it as a starting point when comparing your own configuration.")
	}

	if len(entry.ExpectedToolNames) > 0 {
		guidance = append(guidance, fmt.Sprintf("Expected tool surfaces: %s.", strings.Join(entry.ExpectedToolNames, ", ")))
	}

	if len(entry.ExpectedSkillNames) > 0 {
		guidance = append(guidance, fmt.Sprintf("Expected bundled skills: %s.", strings.Join(entry.ExpectedSkillNames, ", ")))
	}

	isIncompatible := strings.EqualFold(compatibilityStatus, "incompatible")
	if isIncompatible && len(entry.ExpectedDiagnosticCodes) > 0 {
		guidance = append(guidance, fmt.Sprintf("Expected failure diagnostics: %s.", strings.Join(entry.ExpectedDiagnosticCodes, ", ")))
	}

	hasUnsupportedCli := false
	hasConfigMismatch := false
	for _, code := range entry.ExpectedDiagnosticCodes {
		if code == "unsupported_cli_registration" {
			hasUnsupportedCli = true
		}
		if code == "config_one_of_mismatch" {
			hasConfigMismatch = true
		}
	}

	if hasUnsupportedCli {
		guidance = append(guidance, "This package depends on `api.registerCli()`, which OpenClaw does not bridge today.")
	}
	if hasConfigMismatch {
		guidance = append(guidance, "Adjust the plugin config to the supported JSON-schema subset; this scenario intentionally demonstrates a failing shape.")
	}

	return guidance
}

func resolvePublicCompatibilityCatalogStatus(entry CompatibilityCatalogManifestEntry) string {
	if strings.TrimSpace(entry.ExpectedStatus) != "" {
		return entry.ExpectedStatus
	}

	if entry.Kind == "clawhub-skill" {
		return "compatible"
	}

	panic(fmt.Sprintf("Compatibility catalog entry '%s' of kind '%s' must declare expectedStatus.", entry.Id, entry.Kind))
}

func requirePublicCompatibilityCatalogField(value string, entry CompatibilityCatalogManifestEntry, fieldName string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}

	panic(fmt.Sprintf("Compatibility catalog entry '%s' of kind '%s' must declare '%s'.", entry.Id, entry.Kind, fieldName))
}

type CompatibilityCatalogManifest struct {
	Version int
	Entries []CompatibilityCatalogManifestEntry
}

type CompatibilityCatalogManifestEntry struct {
	Id                      string
	Category                string
	Kind                    string
	Spec                    string
	PackageName             string
	PluginId                string
	Slug                    string
	Version                 string
	ExpectedStatus          string
	ExpectedRelativePath    string
	ConfigJson              string
	InstallExtraPackages    []string
	ExpectedToolNames       []string
	ExpectedSkillNames      []string
	ExpectedDiagnosticCodes []string
}
