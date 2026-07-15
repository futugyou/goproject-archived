package core

import (
	"time"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
)

type ModelsConfig struct {
	DefaultProfile string               `json:"default_profile"`
	Profiles       []ModelProfileConfig `json:"profiles"`
}

func DefaultModelsConfig() ModelsConfig {
	return ModelsConfig{
		Profiles: make([]ModelProfileConfig, 0),
	}
}

// ==========================================
// ModelProfileConfig
// ==========================================

type ModelProfileConfig struct {
	Id                  string               `json:"id"`
	PresetId            string               `json:"preset_id"`
	Provider            string               `json:"provider"`
	Model               string               `json:"model"`
	BaseUrl             string               `json:"base_url"`
	ApiKey              string               `json:"api_key"`
	AuthMode            string               `json:"auth_mode"`
	SendRequestMetadata *bool                `json:"send_request_metadata,omitempty"`
	Tags                []string             `json:"tags"`
	FallbackProfileIds  []string             `json:"fallback_profile_ids"`
	FallbackModels      []string             `json:"fallback_models"`
	Capabilities        *ModelCapabilities   `json:"capabilities,omitempty"`
	PromptCaching       *PromptCachingConfig `json:"prompt_caching,omitempty"`
}

func DefaultModelProfileConfig() ModelProfileConfig {
	return ModelProfileConfig{
		Tags:               make([]string, 0),
		FallbackProfileIds: make([]string, 0),
		FallbackModels:     make([]string, 0),
	}
}

// ==========================================
// ModelCapabilities
// ==========================================

type ModelCapabilities struct {
	SupportsTools                  bool `json:"supports_tools"`
	SupportsVision                 bool `json:"supports_vision"`
	SupportsJsonSchema             bool `json:"supports_json_schema"`
	SupportsStructuredOutputs      bool `json:"supports_structured_outputs"`
	SupportsStreaming              bool `json:"supports_streaming"`
	SupportsParallelToolCalls      bool `json:"supports_parallel_tool_calls"`
	SupportsReasoningEffort        bool `json:"supports_reasoning_effort"`
	SupportsSystemMessages         bool `json:"supports_system_messages"`
	SupportsImageInput             bool `json:"supports_image_input"`
	SupportsVideoInput             bool `json:"supports_video_input"`
	SupportsAudioInput             bool `json:"supports_audio_input"`
	SupportsPromptCaching          bool `json:"supports_prompt_caching"`
	SupportsExplicitCacheRetention bool `json:"supports_explicit_cache_retention"`
	ReportsCacheReadTokens         bool `json:"reports_cache_read_tokens"`
	ReportsCacheWriteTokens        bool `json:"reports_cache_write_tokens"`
	MaxContextTokens               int  `json:"max_context_tokens"`
	MaxOutputTokens                int  `json:"max_output_tokens"`
}

func DefaultModelCapabilities() *ModelCapabilities {
	return &ModelCapabilities{
		SupportsStreaming:      true,
		SupportsSystemMessages: true,
	}
}

// ==========================================
// ModelSelectionRequirements
// ==========================================

type ModelSelectionRequirements struct {
	SupportsTools             bool `json:"supports_tools"`
	SupportsVision            bool `json:"supports_vision"`
	SupportsJsonSchema        bool `json:"supports_json_schema"`
	SupportsStructuredOutputs bool `json:"supports_structured_outputs"`
	SupportsStreaming         bool `json:"supports_streaming"`
	SupportsParallelToolCalls bool `json:"supports_parallel_tool_calls"`
	SupportsReasoningEffort   bool `json:"supports_reasoning_effort"`
	SupportsSystemMessages    bool `json:"supports_system_messages"`
	SupportsImageInput        bool `json:"supports_image_input"`
	SupportsVideoInput        bool `json:"supports_video_input"`
	SupportsAudioInput        bool `json:"supports_audio_input"`
	MinContextTokens          int  `json:"min_context_tokens"`
	MinOutputTokens           int  `json:"min_output_tokens"`
}

// ==========================================
// ModelProfile
// ==========================================

const ModelProfileAuthModeBearer = "bearer"

type ModelProfile struct {
	Id                  string               `json:"id"`
	PresetId            string               `json:"preset_id,omitempty"`
	ProviderId          string               `json:"provider_id"`
	ModelId             string               `json:"model_id"`
	BaseUrl             string               `json:"base_url,omitempty"`
	ApiKey              string               `json:"api_key,omitempty"`
	AuthMode            string               `json:"auth_mode"`
	SendRequestMetadata bool                 `json:"send_request_metadata"`
	Tags                []string             `json:"tags"`
	FallbackProfileIds  []string             `json:"fallback_profile_ids"`
	FallbackModels      []string             `json:"fallback_models"`
	Capabilities        *ModelCapabilities   `json:"capabilities"`
	PromptCaching       *PromptCachingConfig `json:"prompt_caching"`
	IsImplicit          bool                 `json:"is_implicit"`
}

func DefaultModelProfile() ModelProfile {
	return ModelProfile{
		AuthMode:           ModelProfileAuthModeBearer,
		Tags:               make([]string, 0),
		FallbackProfileIds: make([]string, 0),
		FallbackModels:     make([]string, 0),
		Capabilities:       DefaultModelCapabilities(),
		PromptCaching:      DefaultPromptCachingConfig(),
	}
}

// ==========================================
// ModelProfileStatus
// ==========================================

const ModelProfileStatusAuthModeBearer = "bearer"

type ModelProfileStatus struct {
	Id                         string               `json:"id"`
	PresetId                   string               `json:"preset_id"`
	ProviderId                 string               `json:"provider_id"`
	ModelId                    string               `json:"model_id"`
	IsDefault                  bool                 `json:"is_default"`
	IsImplicit                 bool                 `json:"is_implicit"`
	IsAvailable                bool                 `json:"is_available"`
	ProviderGateway            string               `json:"provider_gateway"`
	AuthMode                   string               `json:"auth_mode"`
	SendRequestMetadata        bool                 `json:"send_request_metadata"`
	Tags                       []string             `json:"tags"`
	Capabilities               *ModelCapabilities   `json:"capabilities"`
	PromptCaching              *PromptCachingConfig `json:"prompt_caching"`
	ValidationIssues           []string             `json:"validation_issues"`
	FallbackProfileIds         []string             `json:"fallback_profile_ids"`
	FallbackModels             []string             `json:"fallback_models"`
	CompatibilityNotes         []string             `json:"compatibility_notes"`
	UsesCompatibilityTransport bool                 `json:"uses_compatibility_transport"`
}

func DefaultModelProfileStatus() ModelProfileStatus {
	return ModelProfileStatus{
		AuthMode:           ModelProfileStatusAuthModeBearer,
		Tags:               make([]string, 0),
		Capabilities:       DefaultModelCapabilities(),
		PromptCaching:      DefaultPromptCachingConfig(),
		ValidationIssues:   make([]string, 0),
		FallbackProfileIds: make([]string, 0),
		FallbackModels:     make([]string, 0),
		CompatibilityNotes: make([]string, 0),
	}
}

type PromptCachingConfig struct {
	Enabled                 *bool  `json:"enabled"`
	Retention               string `json:"retention"`
	Dialect                 string `json:"dialect"`
	KeepWarmEnabled         *bool  `json:"keep_warm_enabled"`
	KeepWarmIntervalMinutes int    `json:"keep_warm_interval_minutes"`
	TraceEnabled            *bool  `json:"trace_enabled"`
	TraceFilePath           string `json:"trace_file_path"`
}

func DefaultPromptCachingConfig() *PromptCachingConfig {
	return &PromptCachingConfig{
		KeepWarmIntervalMinutes: 55,
	}
}

// ==========================================
// ModelSelectionDescriptor
// ==========================================

type ModelSelectionDescriptor struct {
	ProfileId          string                     `json:"profile_id,omitempty"`
	PreferredTags      []string                   `json:"preferred_tags"`
	FallbackProfileIds []string                   `json:"fallback_profile_ids"`
	Requirements       ModelSelectionRequirements `json:"requirements"`
}

func DefaultModelSelectionDescriptor() ModelSelectionDescriptor {
	return ModelSelectionDescriptor{
		PreferredTags:      make([]string, 0),
		FallbackProfileIds: make([]string, 0),
	}
}

// ==========================================
// ModelProfilesStatusResponse
// ==========================================

type ModelProfilesStatusResponse struct {
	DefaultProfileId string               `json:"default_profile_id,omitempty"`
	Profiles         []ModelProfileStatus `json:"profiles"`
}

func DefaultModelProfilesStatusResponse() ModelProfilesStatusResponse {
	return ModelProfilesStatusResponse{
		Profiles: make([]ModelProfileStatus, 0),
	}
}

// ==========================================
// ModelSelectionDoctorResponse
// ==========================================

type ModelSelectionDoctorResponse struct {
	DefaultProfileId string               `json:"default_profile_id"`
	Errors           []string             `json:"errors"`
	Warnings         []string             `json:"warnings"`
	Profiles         []ModelProfileStatus `json:"profiles"`
}

func DefaultModelSelectionDoctorResponse() ModelSelectionDoctorResponse {
	return ModelSelectionDoctorResponse{
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Profiles: make([]ModelProfileStatus, 0),
	}
}

// ==========================================
// ModelEvaluationRequest
// ==========================================

type ModelEvaluationRequest struct {
	ProfileId       string   `json:"profile_id"`
	ProfileIds      []string `json:"profile_ids"`
	ScenarioIds     []string `json:"scenario_ids"`
	IncludeMarkdown bool     `json:"include_markdown"`
}

func DefaultModelEvaluationRequest() ModelEvaluationRequest {
	return ModelEvaluationRequest{
		ProfileIds:      make([]string, 0),
		ScenarioIds:     make([]string, 0),
		IncludeMarkdown: true,
	}
}

// ==========================================
// ModelEvaluationScenarioResult
// ==========================================

const ModelEvaluationScenarioResultStatusUnknown = "unknown"

type ModelEvaluationScenarioResult struct {
	ScenarioId    string `json:"scenario_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Summary       string `json:"summary,omitempty"`
	LatencyMs     int64  `json:"latency_ms"`
	InputTokens   int64  `json:"input_tokens"`
	OutputTokens  int64  `json:"output_tokens"`
	MalformedJson bool   `json:"malformed_json"`
	ToolCalls     int    `json:"tool_calls"`
	Error         string `json:"error,omitempty"`
}

func DefaultModelEvaluationScenarioResult() ModelEvaluationScenarioResult {
	return ModelEvaluationScenarioResult{
		Status: ModelEvaluationScenarioResultStatusUnknown,
	}
}

// ==========================================
// ModelEvaluationProfileReport
// ==========================================

type ModelEvaluationProfileReport struct {
	ProfileId      string                          `json:"profile_id"`
	ProviderId     string                          `json:"provider_id"`
	ModelId        string                          `json:"model_id"`
	StartedAtUtc   time.Time                       `json:"started_at_utc"`
	CompletedAtUtc time.Time                       `json:"completed_at_utc"`
	Scenarios      []ModelEvaluationScenarioResult `json:"scenarios"`
}

func DefaultModelEvaluationProfileReport() ModelEvaluationProfileReport {
	return ModelEvaluationProfileReport{
		Scenarios: make([]ModelEvaluationScenarioResult, 0),
	}
}

// ==========================================
// ModelEvaluationReport
// ==========================================

type ModelEvaluationReport struct {
	RunId          string                         `json:"run_id"`
	StartedAtUtc   time.Time                      `json:"started_at_utc"`
	CompletedAtUtc time.Time                      `json:"completed_at_utc"`
	ScenarioIds    []string                       `json:"scenario_ids"`
	Profiles       []ModelEvaluationProfileReport `json:"profiles"`
	JsonPath       string                         `json:"json_path,omitempty"`
	MarkdownPath   string                         `json:"markdown_path,omitempty"`
	Markdown       string                         `json:"markdown,omitempty"`
}

func DefaultModelEvaluationReport() ModelEvaluationReport {
	return ModelEvaluationReport{
		ScenarioIds: make([]string, 0),
		Profiles:    make([]ModelEvaluationProfileReport, 0),
	}
}

type ModelSelectionRequest struct {
	ExplicitProfileId    string                       `json:"explicit_profile_id"`
	Session              *Session                     `json:"session"`
	Messages             []chatcompletion.ChatMessage `json:"messages"`
	Options              *chatcompletion.ChatOptions  `json:"options"`
	Streaming            bool                         `json:"streaming"`
	EstimatedInputTokens int64                        `json:"estimated_input_tokens"`
	ReservedOutputTokens int                          `json:"reserved_output_tokens"`
}

type ModelSelectionCandidate struct {
	Profile        *ModelProfile `json:"profile"`
	FallbackModels []string      `json:"fallback_models"`
}

type ModelSelectionResult struct {
	RequestedProfileId string                      `json:"requested_profile_id"`
	SelectedProfileId  string                      `json:"selected_profile_id"`
	ProviderId         string                      `json:"provider_id"`
	ModelId            string                      `json:"mode_id"`
	Requirements       *ModelSelectionRequirements `json:"requirements"`
	Candidates         []ModelSelectionCandidate   `json:"candidates"`
	PreferredTags      []string                    `json:"preferred_tags"`
	Explanation        string                      `json:"explanation"`
}
