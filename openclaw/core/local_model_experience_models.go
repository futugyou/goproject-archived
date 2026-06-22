package core

import "time"

// --- SessionResponseModes ---
const (
	SessionResponseModesDefault    = "default"
	SessionResponseModesConciseOps = "concise_ops"
	SessionResponseModesFull       = "full"
)

// --- LocalModelPresetDefinition ---
type LocalModelPresetDefinition struct {
	Id                       string            `json:"id"`
	Label                    string            `json:"label"`
	Description              string            `json:"description"`
	Provider                 string            `json:"provider"`
	DefaultBaseUrl           string            `json:"default_base_url"`
	PackageId                *string           `json:"package_id,omitempty"`
	ModelId                  *string           `json:"model_id,omitempty"`
	Installable              bool              `json:"installable"`
	Tags                     []string          `json:"tags"`
	Capabilities             ModelCapabilities `json:"capabilities"`
	RecommendedContextTokens int               `json:"recommended_context_tokens"`
	RecommendedOutputTokens  int               `json:"recommended_output_tokens"`
	CompatibilityNotes       []string          `json:"compatibility_notes"`
	DoctorExpectations       []string          `json:"doctor_expectations"`
}

func DefaultLocalModelPresetDefinition() LocalModelPresetDefinition {
	return LocalModelPresetDefinition{
		Provider:           "ollama",
		DefaultBaseUrl:     "http://127.0.0.1:11434",
		Tags:               []string{},
		CompatibilityNotes: []string{},
		DoctorExpectations: []string{},
	}
}

// --- LocalModelPresetListResponse ---
type LocalModelPresetListResponse struct {
	Items []LocalModelPresetDefinition `json:"items"`
}

func DefaultLocalModelPresetListResponse() LocalModelPresetListResponse {
	return LocalModelPresetListResponse{
		Items: []LocalModelPresetDefinition{},
	}
}

// --- LocalModelRuntimeDefaults ---
type LocalModelRuntimeDefaults struct {
	Backend                     string `json:"backend"`
	Threads                     string `json:"threads"`
	GpuLayers                   string `json:"gpu_layers"`
	ContextSize                 int    `json:"context_size"`
	EnableJinja                 bool   `json:"enable_jinja"`
	ChatTemplate                string `json:"chat_template"`
	ChatTemplateFileName        string `json:"chat_template_file_name"`
	MultimodalProjectorFileName string `json:"multimodal_projector_file_name"`
	DraftModelFileName          string `json:"draft_model_file_name"`
	ReasoningMode               string `json:"reasoning_mode"`
	ReasoningBudget             int    `json:"reasoning_budget"`
}

func DefaultLocalModelRuntimeDefaults() LocalModelRuntimeDefaults {
	return LocalModelRuntimeDefaults{
		Backend:       "llama.cpp",
		Threads:       "auto",
		GpuLayers:     "auto",
		ContextSize:   4096,
		ReasoningMode: "auto",
	}
}

// --- LocalModelPackageFileRoles ---
const (
	LocalModelPackageFileRolesModel               = "model"
	LocalModelPackageFileRolesMultimodalProjector = "mmproj"
	LocalModelPackageFileRolesDraftModel          = "draft"
)

// --- LocalModelPackageFileDefinition ---
type LocalModelPackageFileDefinition struct {
	Role             string `json:"role"`
	FileName         string `json:"file_name"`
	DownloadUrl      string `json:"download_url"`
	ExpectedSha256   string `json:"expected_sha256"`
	Required         bool   `json:"required"`
	InstallByDefault bool   `json:"install_by_default"`
}

func DefaultLocalModelPackageFileDefinition() LocalModelPackageFileDefinition {
	return LocalModelPackageFileDefinition{
		Required:         true,
		InstallByDefault: true,
	}
}

// --- LocalModelPackageDefinition ---
type LocalModelPackageDefinition struct {
	Id                        string                            `json:"id"`
	PresetId                  string                            `json:"preset_id"`
	DisplayName               string                            `json:"display_name"`
	Description               string                            `json:"description"`
	Provider                  string                            `json:"provider"`
	ModelId                   string                            `json:"model_id"`
	Family                    string                            `json:"family"`
	Format                    string                            `json:"format"`
	Quantization              string                            `json:"quantization"`
	FileName                  string                            `json:"file_name"`
	DownloadUrl               string                            `json:"download_url"`
	ExpectedSha256            string                            `json:"expected_sha256"`
	ModelPageUrl              string                            `json:"model_page_url"`
	LicenseUrl                string                            `json:"license_url"`
	RequiresLicenseAcceptance bool                              `json:"requires_license_acceptance"`
	RequiresDownloadToken     bool                              `json:"requires_download_token"`
	Experimental              bool                              `json:"experimental"`
	MinRamGb                  int                               `json:"min_ram_gb"`
	RecommendedRamGb          int                               `json:"recommended_ram_gb"`
	ContextWindow             int                               `json:"context_window"`
	MaxOutputTokens           int                               `json:"max_output_tokens"`
	Tags                      []string                          `json:"tags"`
	Files                     []LocalModelPackageFileDefinition `json:"files"`
	Capabilities              ModelCapabilities                 `json:"capabilities"`
	Runtime                   LocalModelRuntimeDefaults         `json:"runtime"`
}

func DefaultLocalModelPackageDefinition() LocalModelPackageDefinition {
	return LocalModelPackageDefinition{
		Provider:        "embedded",
		Format:          "gguf",
		FileName:        "model.gguf",
		ContextWindow:   4096,
		MaxOutputTokens: 1024,
		Tags:            []string{},
		Files:           []LocalModelPackageFileDefinition{},
		Runtime:         DefaultLocalModelRuntimeDefaults(),
	}
}

// --- LocalModelInstallFileManifest ---
type LocalModelInstallFileManifest struct {
	Role     string `json:"role"`
	FileName string `json:"file_name"`
	Sha256   string `json:"sha256"`
	Source   string `json:"source"`
}

// --- LocalModelInstallManifest ---
type LocalModelInstallManifest struct {
	SchemaVersion   int                             `json:"schema_version"`
	PackageId       string                          `json:"package_id"`
	PresetId        string                          `json:"preset_id"`
	ModelId         string                          `json:"model_id"`
	FileName        string                          `json:"file_name"`
	Sha256          string                          `json:"sha256"`
	Source          string                          `json:"source"`
	LicenseUrl      string                          `json:"license_url"`
	LicenseAccepted bool                            `json:"license_accepted"`
	InstalledAtUtc  time.Time                       `json:"installed_at_utc"`
	Files           []LocalModelInstallFileManifest `json:"files"`
}

func DefaultLocalModelInstallManifest() LocalModelInstallManifest {
	return LocalModelInstallManifest{
		SchemaVersion:  1,
		InstalledAtUtc: time.Now().UTC(),
		Files:          []LocalModelInstallFileManifest{},
	}
}

// --- LocalModelPackageFileStatus ---
type LocalModelPackageFileStatus struct {
	Role      string `json:"role"`
	FileName  string `json:"file_name"`
	Required  bool   `json:"required"`
	Installed bool   `json:"installed"`
	Verified  bool   `json:"verified"`
	Path      string `json:"path"`
	Sha256    string `json:"sha256"`
	Issue     string `json:"issue"`
}

// --- LocalModelPackageStatus ---
type LocalModelPackageStatus struct {
	PackageId   string                        `json:"package_id"`
	PresetId    string                        `json:"preset_id"`
	ModelId     string                        `json:"model_id"`
	DisplayName string                        `json:"display_name"`
	Installed   bool                          `json:"installed"`
	Verified    bool                          `json:"verified"`
	ModelPath   string                        `json:"model_path"`
	Sha256      string                        `json:"sha256"`
	Issue       string                        `json:"issue"`
	Files       []LocalModelPackageFileStatus `json:"files"`
}

func DefaultLocalModelPackageStatus() LocalModelPackageStatus {
	return LocalModelPackageStatus{
		Files: []LocalModelPackageFileStatus{},
	}
}

// --- MaintenanceFindingSeverities ---
const (
	MaintenanceFindingSeveritiesInfo = "info"
	MaintenanceFindingSeveritiesWarn = "warn"
	MaintenanceFindingSeveritiesFail = "fail"
)

// --- MaintenanceFindingCategories ---
const (
	MaintenanceFindingCategoriesStorage      = "storage"
	MaintenanceFindingCategoriesPromptBudget = "prompt_budget"
	MaintenanceFindingCategoriesDrift        = "drift"
	MaintenanceFindingCategoriesReliability  = "reliability"
)

// --- MaintenanceFinding ---
type MaintenanceFinding struct {
	Id                 string  `json:"id"`
	Category           string  `json:"category"`
	Severity           string  `json:"severity"`
	Summary            string  `json:"summary"`
	Detail             *string `json:"detail,omitempty"`
	Recommendation     *string `json:"recommendation,omitempty"`
	RecommendedCommand *string `json:"recommended_command,omitempty"`
	NumericValue       int64   `json:"numeric_value"`
}

func DefaultMaintenanceFinding() MaintenanceFinding {
	return MaintenanceFinding{
		Category: MaintenanceFindingCategoriesStorage,
		Severity: MaintenanceFindingSeveritiesInfo,
	}
}

// --- MaintenancePromptBudgetSnapshot ---
type MaintenancePromptBudgetSnapshot struct {
	RecentTurnsAnalyzed int64 `json:"recent_turns_analyzed"`
	P50InputTokens      int64 `json:"p50_input_tokens"`
	P95InputTokens      int64 `json:"p95_input_tokens"`
	SystemPromptTokens  int64 `json:"system_prompt_tokens"`
	SkillsTokens        int64 `json:"skills_tokens"`
	HistoryTokens       int64 `json:"history_tokens"`
	ToolOutputsTokens   int64 `json:"tool_outputs_tokens"`
	UserInputTokens     int64 `json:"user_input_tokens"`
	AgentsFileBytes     int   `json:"agents_file_bytes"`
	SoulFileBytes       int   `json:"soul_file_bytes"`
	LoadedSkillCount    int   `json:"loaded_skill_count"`
}

// --- MaintenanceStorageSnapshot ---
type MaintenanceStorageSnapshot struct {
	MemoryBytes                    int64 `json:"memory_bytes"`
	ArchiveBytes                   int64 `json:"archive_bytes"`
	OrphanedSessionMetadataEntries int   `json:"orphaned_session_metadata_entries"`
	ModelEvaluationArtifacts       int   `json:"model_evaluation_artifacts"`
	PromptCacheTraceArtifacts      int   `json:"prompt_cache_trace_artifacts"`
}

// --- MaintenanceDriftSnapshot ---
type MaintenanceDriftSnapshot struct {
	ProviderRetries        int64 `json:"provider_retries"`
	ProviderErrors         int64 `json:"provider_errors"`
	DegradedAutomations    int   `json:"degraded_automations"`
	QuarantinedAutomations int   `json:"quarantined_automations"`
	RetentionFailures      int64 `json:"retention_failures"`
	ChannelDriftCount      int   `json:"channel_drift_count"`
	PluginWarningCount     int   `json:"plugin_warning_count"`
	PluginErrorCount       int   `json:"plugin_error_count"`
	PromptP95Delta         int64 `json:"prompt_p95_delta"`
}

// --- MaintenanceReportResponse ---
type MaintenanceReportResponse struct {
	GeneratedAtUtc time.Time                       `json:"generated_at_utc"`
	OverallStatus  string                          `json:"overall_status"`
	Storage        MaintenanceStorageSnapshot      `json:"storage"`
	PromptBudget   MaintenancePromptBudgetSnapshot `json:"prompt_budget"`
	Drift          MaintenanceDriftSnapshot        `json:"drift"`
	Findings       []MaintenanceFinding            `json:"findings"`
	Reliability    ReliabilitySnapshot             `json:"reliability"`
}

func DefaultMaintenanceReportResponse() MaintenanceReportResponse {
	return MaintenanceReportResponse{
		GeneratedAtUtc: time.Now().UTC(),
		OverallStatus:  "pass", // 假设 SetupCheckStates.Pass 的值为 "pass"
		Findings:       []MaintenanceFinding{},
		Reliability:    DefaultReliabilitySnapshot(),
	}
}

// --- MaintenanceFixRequest ---
type MaintenanceFixRequest struct {
	DryRun bool   `json:"dry_run"`
	Apply  string `json:"apply"`
}

func DefaultMaintenanceFixRequest() MaintenanceFixRequest {
	return MaintenanceFixRequest{
		DryRun: true,
		Apply:  "all",
	}
}

// --- MaintenanceFixAction ---
type MaintenanceFixAction struct {
	Id           string `json:"id"`
	Applied      bool   `json:"applied"`
	Summary      string `json:"summary"`
	NumericValue int64  `json:"numeric_value"`
}

// --- MaintenanceFixResponse ---
type MaintenanceFixResponse struct {
	DryRun      bool                   `json:"dry_run"`
	Success     bool                   `json:"success"`
	Actions     []MaintenanceFixAction `json:"actions"`
	Warnings    []string               `json:"warnings"`
	Reliability ReliabilitySnapshot    `json:"reliability"`
}

func DefaultMaintenanceFixResponse() MaintenanceFixResponse {
	return MaintenanceFixResponse{
		DryRun:      true,
		Actions:     []MaintenanceFixAction{},
		Warnings:    []string{},
		Reliability: DefaultReliabilitySnapshot(),
	}
}

// --- ReliabilityStates ---
const (
	ReliabilityStatesHealthy      = "healthy"
	ReliabilityStatesWatch        = "watch"
	ReliabilityStatesActionNeeded = "action_needed"
)

// --- ReliabilityFactor ---
type ReliabilityFactor struct {
	Id       string   `json:"id"`
	Label    string   `json:"label"`
	Weight   int      `json:"weight"`
	Score    int      `json:"score"`
	Status   string   `json:"status"`
	Findings []string `json:"findings"`
}

func DefaultReliabilityFactor() ReliabilityFactor {
	return ReliabilityFactor{
		Status:   ReliabilityStatesHealthy,
		Findings: []string{},
	}
}

// --- ReliabilityRecommendation ---
type ReliabilityRecommendation struct {
	Id       string `json:"id"`
	Summary  string `json:"summary"`
	Command  string `json:"command"`
	Priority int    `json:"priority"`
}

// --- ReliabilitySnapshot ---
type ReliabilitySnapshot struct {
	Score           int                         `json:"score"`
	Status          string                      `json:"status"`
	Factors         []ReliabilityFactor         `json:"factors"`
	Recommendations []ReliabilityRecommendation `json:"recommendations"`
}

func DefaultReliabilitySnapshot() ReliabilitySnapshot {
	return ReliabilitySnapshot{
		Status:          ReliabilityStatesHealthy,
		Factors:         []ReliabilityFactor{},
		Recommendations: []ReliabilityRecommendation{},
	}
}

// --- MaintenanceHistorySnapshot ---
type MaintenanceHistorySnapshot struct {
	GeneratedAtUtc time.Time                 `json:"generated_at_utc"`
	Report         MaintenanceReportResponse `json:"report"`
}

func DefaultMaintenanceHistorySnapshot() MaintenanceHistorySnapshot {
	return MaintenanceHistorySnapshot{
		GeneratedAtUtc: time.Now().UTC(),
		Report:         DefaultMaintenanceReportResponse(),
	}
}
