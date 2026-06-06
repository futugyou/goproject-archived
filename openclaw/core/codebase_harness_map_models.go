package core

import "time"

type CodebaseHarnessMap struct {
	ID                string                     `json:"id"`
	RepositoryRoot    string                     `json:"repository_root"`
	RepositoryName    string                     `json:"repository_name"`
	GeneratedAtUtc    time.Time                  `json:"generated_at_utc"`
	GeneratorVersion  string                     `json:"generator_version"`
	Summary           CodebaseMapSummary         `json:"summary"`
	Projects          []CodebaseProject          `json:"projects"`
	Modules           []CodebaseModule           `json:"modules"`
	Artifacts         []CodebaseArtifact         `json:"artifacts"`
	Endpoints         []CodebaseEndpoint         `json:"endpoints"`
	ToolSurfaces      []CodebaseToolSurface      `json:"tool_surfaces"`
	ProviderSurfaces  []CodebaseProviderSurface  `json:"provider_surfaces"`
	ChannelSurfaces   []CodebaseChannelSurface   `json:"channel_surfaces"`
	ConfigSurfaces    []CodebaseConfigSurface    `json:"config_surfaces"`
	TestSurfaces      []CodebaseTestSurface      `json:"test_surfaces"`
	EvidenceLinks     []CodebaseEvidenceLink     `json:"evidence_links"`
	ContractLinks     []CodebaseContractLink     `json:"contract_links"`
	SharedStateLinks  []CodebaseSharedStateLink  `json:"shared_state_links"`
	RuntimeTraceLinks []CodebaseRuntimeTraceLink `json:"runtime_trace_links"`
	Diagnostics       []CodebaseMapDiagnostic    `json:"diagnostics"`
	Tags              []string                   `json:"tags"`
	Metadata          map[string]string          `json:"metadata"`
}

func DefaultCodebaseHarnessMap() *CodebaseHarnessMap {
	return &CodebaseHarnessMap{
		GeneratedAtUtc:    time.Now().UTC(),
		Projects:          make([]CodebaseProject, 0),
		Modules:           make([]CodebaseModule, 0),
		Artifacts:         make([]CodebaseArtifact, 0),
		Endpoints:         make([]CodebaseEndpoint, 0),
		ToolSurfaces:      make([]CodebaseToolSurface, 0),
		ProviderSurfaces:  make([]CodebaseProviderSurface, 0),
		ChannelSurfaces:   make([]CodebaseChannelSurface, 0),
		ConfigSurfaces:    make([]CodebaseConfigSurface, 0),
		TestSurfaces:      make([]CodebaseTestSurface, 0),
		EvidenceLinks:     make([]CodebaseEvidenceLink, 0),
		ContractLinks:     make([]CodebaseContractLink, 0),
		SharedStateLinks:  make([]CodebaseSharedStateLink, 0),
		RuntimeTraceLinks: make([]CodebaseRuntimeTraceLink, 0),
		Diagnostics:       make([]CodebaseMapDiagnostic, 0),
		Tags:              make([]string, 0),
		Metadata:          make(map[string]string),
	}
}

// --- CodebaseMapSummary ---

type CodebaseMapSummary struct {
	SolutionFilesCount   int `json:"solution_files_count"`
	ProjectFilesCount    int `json:"project_files_count"`
	SourceFilesCount     int `json:"source_files_count"`
	TestProjectsCount    int `json:"test_projects_count"`
	EndpointCount        int `json:"endpoint_count"`
	ToolSurfaceCount     int `json:"tool_surface_count"`
	ChannelSurfaceCount  int `json:"channel_surface_count"`
	ProviderSurfaceCount int `json:"provider_surface_count"`
	ConfigFileCount      int `json:"config_file_count"`
	RecentChangeCount    int `json:"recent_change_count"`
	WarningCount         int `json:"warning_count"`
}

// --- CodebaseProject ---

type CodebaseProject struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Path              string   `json:"path"`
	ProjectType       string   `json:"project_type"`
	TargetFrameworks  []string `json:"target_frameworks"`
	IsTestProject     bool     `json:"is_test_project"`
	PackageReferences []string `json:"package_references"`
	ProjectReferences []string `json:"project_references"`
	Tags              []string `json:"tags"`
}

func DefaultCodebaseProject() *CodebaseProject {
	return &CodebaseProject{
		ProjectType:       "unknown",
		TargetFrameworks:  make([]string, 0),
		PackageReferences: make([]string, 0),
		ProjectReferences: make([]string, 0),
		Tags:              make([]string, 0),
	}
}

// --- CodebaseModule ---

type CodebaseModule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	ProjectID   *string  `json:"project_id,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags"`
}

func DefaultCodebaseModule() *CodebaseModule {
	return &CodebaseModule{
		Kind: "unknown",
		Tags: make([]string, 0),
	}
}

// --- CodebaseArtifact ---

type CodebaseArtifact struct {
	ID              string    `json:"id"`
	Path            string    `json:"path"`
	Kind            string    `json:"kind"`
	ProjectID       *string   `json:"project_id,omitempty"`
	ModuleID        *string   `json:"module_id,omitempty"`
	SizeBytes       int64     `json:"size_bytes"`
	LastModifiedUtc time.Time `json:"last_modified_utc"`
	Hash            *string   `json:"hash,omitempty"`
	Tags            []string  `json:"tags"`
	Summary         *string   `json:"summary,omitempty"`
}

func DefaultCodebaseArtifact() *CodebaseArtifact {
	return &CodebaseArtifact{
		Kind: "unknown",
		Tags: make([]string, 0),
	}
}

// --- CodebaseEndpoint ---

type CodebaseEndpoint struct {
	ID           string   `json:"id"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	SourceFile   string   `json:"source_file"`
	AuthRequired *bool    `json:"auth_required,omitempty"`
	Scope        *string  `json:"scope,omitempty"`
	Tags         []string `json:"tags"`
}

func DefaultCodebaseEndpoint() *CodebaseEndpoint {
	return &CodebaseEndpoint{
		Tags: make([]string, 0),
	}
}

// --- CodebaseToolSurface ---

type CodebaseToolSurface struct {
	Name             string   `json:"name"`
	SourceFile       string   `json:"source_file"`
	Category         *string  `json:"category,omitempty"`
	ReadOnly         bool     `json:"read_only"`
	Mutating         bool     `json:"mutating"`
	ApprovalRequired bool     `json:"approval_required"`
	SandboxCapable   bool     `json:"sandbox_capable"`
	Tags             []string `json:"tags"`
}

func DefaultCodebaseToolSurface() *CodebaseToolSurface {
	return &CodebaseToolSurface{
		Tags: make([]string, 0),
	}
}

// --- CodebaseProviderSurface ---

type CodebaseProviderSurface struct {
	Name              string   `json:"name"`
	SourceFile        string   `json:"source_file"`
	ProviderType      *string  `json:"provider_type,omitempty"`
	SupportsStreaming *bool    `json:"supports_streaming,omitempty"`
	SupportsTools     *bool    `json:"supports_tools,omitempty"`
	Tags              []string `json:"tags"`
}

func DefaultCodebaseProviderSurface() *CodebaseProviderSurface {
	return &CodebaseProviderSurface{
		Tags: make([]string, 0),
	}
}

// --- CodebaseChannelSurface ---

type CodebaseChannelSurface struct {
	Name                    string   `json:"name"`
	SourceFile              string   `json:"source_file"`
	Direction               *string  `json:"direction,omitempty"`
	AuthOrSignatureRequired *bool    `json:"auth_or_signature_required,omitempty"`
	Tags                    []string `json:"tags"`
}

func DefaultCodebaseChannelSurface() *CodebaseChannelSurface {
	return &CodebaseChannelSurface{
		Tags: make([]string, 0),
	}
}

// --- CodebaseConfigSurface ---

type CodebaseConfigSurface struct {
	Path        string  `json:"path"`
	Section     *string `json:"section,omitempty"`
	Key         string  `json:"key"`
	Description *string `json:"description,omitempty"`
	Sensitive   bool    `json:"sensitive"`
}

type CodebaseTestSurface struct {
	ProjectName   string  `json:"project_name"`
	ProjectPath   string  `json:"project_path"`
	TestFramework *string `json:"test_framework,omitempty"`
	RelatedModule *string `json:"related_module,omitempty"`
}
type CodebaseEvidenceLink struct {
	EvidenceBundleId string  `json:"evidence_bundle_id"`
	Path             *string `json:"path"`
	Summary          *string `json:"summary"`
}

type CodebaseContractLink struct {
	HarnessContractId string  `json:"harness_contract_id"`
	Path              *string `json:"path"`
	Summary           *string `json:"summary"`
}

type CodebaseSharedStateLink struct {
	SharedStateId string  `json:"shared_state_id"`
	SessionId     *string `json:"session_id"`
	Path          *string `json:"path"`
	Summary       *string `json:"summary"`
}

type CodebaseRuntimeTraceLink struct {
	RuntimeEventId string  `json:"runtime_event_id"`
	Component      *string `json:"component"`
	Action         *string `json:"action"`
	Path           *string `json:"path"`
	Summary        *string `json:"summary"`
}

type CodebaseMapDiagnostic struct {
	Severity       string  `json:"severity"`
	Code           string  `json:"code"`
	Message        string  `json:"message"`
	Path           *string `json:"path"`
	Recommendation *string `json:"recommendation"`
}

func DefaultCodebaseMapDiagnostic() CodebaseMapDiagnostic {
	return CodebaseMapDiagnostic{
		Severity: "warning",
	}
}

type CodebaseMapOptions struct {
	IncludeHashes           bool   `json:"include_hashes"`
	IncludeRecentChanges    bool   `json:"include_recent_changes"`
	IncludeEndpoints        bool   `json:"include_endpoints"`
	IncludeToolSurfaces     bool   `json:"include_tool_surfaces"`
	IncludeProviderSurfaces bool   `json:"include_provider_surfaces"`
	IncludeChannelSurfaces  bool   `json:"include_channel_surfaces"`
	IncludeConfigSurfaces   bool   `json:"include_config_surfaces"`
	IncludeTests            bool   `json:"include_tests"`
	IncludeDocs             bool   `json:"include_docs"`
	MaxFiles                int    `json:"max_files"`
	MaxDepth                int    `json:"max_depth"`
	RecentDays              int    `json:"recent_days"`
	Category                string `json:"category"`
}

func DefaultCodebaseMapOptions() CodebaseMapOptions {
	return CodebaseMapOptions{
		IncludeRecentChanges:    true,
		IncludeEndpoints:        true,
		IncludeToolSurfaces:     true,
		IncludeProviderSurfaces: true,
		IncludeChannelSurfaces:  true,
		IncludeConfigSurfaces:   true,
		IncludeTests:            true,
		IncludeDocs:             true,
		MaxFiles:                5000,
		MaxDepth:                12,
		RecentDays:              30,
		Category:                "All",
	}
}

type CodebaseMapQuery struct {
	Root          *string `json:"root"`
	Category      *string `json:"category"`
	IncludeHashes bool    `json:"include_hashes"`
	RecentDays    int     `json:"recent_days"`
	MaxFiles      int     `json:"max_files"`
}

func DefaultCodebaseMapQuery() CodebaseMapQuery {
	return CodebaseMapQuery{
		RecentDays: 30,
		MaxFiles:   5000,
	}
}
