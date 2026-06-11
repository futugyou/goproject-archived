package core

type CompatibilityCatalogResponse struct {
	Version int                         `json:"version"`
	Source  string                      `json:"source"`
	Items   []CompatibilityCatalogEntry `json:"items"`
}

func DefaultCompatibilityCatalogResponse() *CompatibilityCatalogResponse {
	return &CompatibilityCatalogResponse{
		Version: 0,
		Source:  "public-smoke.json",
		Items:   make([]CompatibilityCatalogEntry, 0),
	}
}

type CompatibilityCatalogEntry struct {
	ID                      string   `json:"id"`
	Category                string   `json:"category"`
	Kind                    string   `json:"kind"`
	Subject                 string   `json:"subject"`
	ScenarioType            string   `json:"scenario_type"`
	CompatibilityStatus     string   `json:"compatibility_status"`
	InstallSurface          string   `json:"install_surface"`
	InstallCommand          string   `json:"install_command"`
	Summary                 string   `json:"summary"`
	PackageSpec             *string  `json:"package_spec,omitempty"`
	PackageName             *string  `json:"package_name,omitempty"`
	PluginID                *string  `json:"plugin_id,omitempty"`
	SkillSlug               *string  `json:"skill_slug,omitempty"`
	PackageVersion          *string  `json:"package_version,omitempty"`
	ExpectedRelativePath    *string  `json:"expected_relative_path,omitempty"`
	ConfigJSONExample       *string  `json:"config_json_example,omitempty"`
	InstallExtraPackages    []string `json:"install_extra_packages"`
	ExpectedToolNames       []string `json:"expected_tool_names"`
	ExpectedSkillNames      []string `json:"expected_skill_names"`
	ExpectedDiagnosticCodes []string `json:"expected_diagnostic_codes"`
	Guidance                []string `json:"guidance"`
}

func DefaultCompatibilityCatalogEntry() *CompatibilityCatalogEntry {
	return &CompatibilityCatalogEntry{
		ScenarioType:            "positive",
		CompatibilityStatus:     "unknown",
		InstallSurface:          "",
		InstallCommand:          "",
		Summary:                 "",
		InstallExtraPackages:    make([]string, 0),
		ExpectedToolNames:       make([]string, 0),
		ExpectedSkillNames:      make([]string, 0),
		ExpectedDiagnosticCodes: make([]string, 0),
		Guidance:                make([]string, 0),
	}
}
