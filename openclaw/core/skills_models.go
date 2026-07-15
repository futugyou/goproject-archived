package core

type MetaSkillPolicyConfig struct {
	Enabled              bool     `json:"enabled"`
	AllowedRiskLevels    []string `json:"allowed_risk_levels"`
	RequiredCapabilities []string `json:"required_capabilities"`
}

type SkillsConfig struct {
	Enabled              bool                         `json:"enabled"`
	MetaSkill            MetaSkillPolicyConfig        `json:"meta_skill"`
	Load                 SkillLoadConfig              `json:"load"`
	Entries              map[string]*SkillEntryConfig `json:"entries"`
	AllowBundled         []string                     `json:"allow_bundled"`
	InstructionPrompt    string                       `json:"instruction_prompt,omitempty"`
	MaxResourceReadBytes int                          `json:"max_resource_read_bytes"`
}

// DefaultSkillsConfig 返回带默认值的 SkillsConfig 实例
func DefaultSkillsConfig() *SkillsConfig {
	return &SkillsConfig{
		Enabled:              true,
		Load:                 *DefaultSkillLoadConfig(),
		Entries:              make(map[string]*SkillEntryConfig),
		AllowBundled:         []string{},
		MaxResourceReadBytes: 256 * 1024, // 256 KB
	}
}

type SkillLoadConfig struct {
	ExtraDirs          []string `json:"extra_dirs"`
	IncludeBundled     bool     `json:"include_bundled"`
	IncludeManaged     bool     `json:"include_managed"`
	ManagedRoot        string   `json:"managed_root"`
	IncludeWorkspace   bool     `json:"include_workspace"`
	Watch              bool     `json:"watch"`
	WatchDebounceMs    int      `json:"watch_debounce_ms"`
	ScanSubdirectories bool     `json:"scan_subdirectories"`
}

// DefaultSkillLoadConfig 返回带默认值的 SkillLoadConfig 实例
func DefaultSkillLoadConfig() *SkillLoadConfig {
	return &SkillLoadConfig{
		ExtraDirs:        []string{},
		IncludeBundled:   true,
		IncludeManaged:   true,
		IncludeWorkspace: true,
		Watch:            false,
		WatchDebounceMs:  250,
	}
}

type SkillEntryConfig struct {
	Enabled bool              `json:"enabled"`
	ApiKey  string            `json:"api_key"`
	Env     map[string]string `json:"env"`
	Config  map[string]string `json:"config"`
}

// DefaultSkillEntryConfig 返回带默认值的 SkillEntryConfig 实例
func DefaultSkillEntryConfig() *SkillEntryConfig {
	return &SkillEntryConfig{
		Enabled: true,
		Env:     make(map[string]string),
		Config:  make(map[string]string),
	}
}

type SkillDefinition struct {
	Name                   string                `json:"name"`
	Description            string                `json:"description"`
	Instructions           string                `json:"instructions"`
	Location               string                `json:"location"`
	Source                 SkillSource           `json:"source"`
	Metadata               *SkillMetadata        `json:"metadata,omitempty"`
	Kind                   SkillKind             `json:"kind"`
	Triggers               []string              `json:"triggers"`
	MetaPriority           int                   `json:"meta_priority"`
	FinalTextMode          string                `json:"final_text_mode"`
	Composition            *MetaSkillComposition `json:"composition,omitempty"`
	UserInvocable          bool                  `json:"user_invocable"`
	DisableModelInvocation bool                  `json:"disable_model_invocation"`
	CommandDispatch        string                `json:"command_dispatch,omitempty"`
	CommandTool            string                `json:"command_tool,omitempty"`
	CommandArgMode         string                `json:"command_arg_mode,omitempty"`
	Resources              []SkillResource       `json:"resources,omitempty"`
}

// DefaultSkillDefinition 返回带默认值的 SkillDefinition 实例
func DefaultSkillDefinition() *SkillDefinition {
	return &SkillDefinition{
		UserInvocable:          true,
		DisableModelInvocation: false,
		Metadata:               DefaultSkillMetadata(),
		Resources:              []SkillResource{},
		Kind:                   SkillKind_Standard,
	}
}

type MetaSkillComposition struct {
	ToolArgsJson string
	Steps        []MetaSkillStepDefinition
}

type SkillMetadata struct {
	Always         bool     `json:"always"`
	Emoji          string   `json:"emoji,omitempty"`
	Homepage       string   `json:"homepage"`
	Os             []string `json:"os"`
	RequireBins    []string `json:"require_bins"`
	RequireAnyBins []string `json:"require_any_bins"`
	RequireEnv     []string `json:"require_env"`
	RequireConfig  []string `json:"require_config"`
	PrimaryEnv     string   `json:"primary_env"`
	SkillKey       string   `json:"skill_key"`
	Risk           string   `json:"risk"`
	Capabilities   []string `json:"capabilities"`
}

// DefaultSkillMetadata 返回带默认值的 SkillMetadata 实例
func DefaultSkillMetadata() *SkillMetadata {
	return &SkillMetadata{
		Os:             []string{},
		RequireBins:    []string{},
		RequireAnyBins: []string{},
		RequireEnv:     []string{},
		RequireConfig:  []string{},
	}
}

type SkillResource struct {
	Name         string            `json:"name"`
	RelativePath string            `json:"relative_path"`
	AbsolutePath string            `json:"absolute_path"`
	Kind         SkillResourceKind `json:"kind"`
}

type SkillSource uint8

const (
	SkillSource_Bundled SkillSource = iota
	SkillSource_Managed
	SkillSource_Workspace
	SkillSource_Extra
	SkillSource_Plugin
)

func (s SkillSource) ToString() string {
	switch s {
	case SkillSource_Bundled:
		return "bundled"
	case SkillSource_Managed:
		return "managed"
	case SkillSource_Workspace:
		return "workspace"
	case SkillSource_Extra:
		return "extra"
	case SkillSource_Plugin:
		return "plugin"
	}
	return "bundled"
}

type SkillResourceKind uint8

const (
	SkillResourceKind_Reference SkillResourceKind = iota
	SkillResourceKind_Script
)

type SkillKind uint8

const (
	SkillKind_Standard SkillKind = iota
	SkillKind_Meta
)

func (s SkillKind) ToString() string {
	switch s {
	case SkillKind_Standard:
		return "standard"
	case SkillKind_Meta:
		return "meta"
	}
	return "standard"
}
