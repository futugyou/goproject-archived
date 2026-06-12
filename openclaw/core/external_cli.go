package core

import (
	"encoding/json"
	"time"
)

func NormalizeRiskLevel(val string) string {
	switch val {
	case "medium", "high":
		return val
	default:
		return RiskLevelLow
	}
}

// Const enums for Output Formats
const (
	OutputFormatJson   = "json"
	OutputFormatNdjson = "ndjson"
	OutputFormatCsv    = "csv"
	OutputFormatText   = "text"
	OutputFormatTable  = "table"
)

func NormalizeOutputFormat(val string) string {
	switch val {
	case "json", "ndjson", "csv", "table":
		return val
	default:
		return OutputFormatText
	}
}

// --- Options Structs ---

type ExternalCliOptions struct {
	Enabled                            bool                                   `json:"enabled"`
	DefaultTimeoutSeconds              int                                    `json:"default_timeout_seconds"`
	MaxStdoutBytes                     int                                    `json:"max_stdout_bytes"`
	MaxStderrBytes                     int                                    `json:"max_stderr_bytes"`
	RedactSecrets                      bool                                   `json:"redact_secrets"`
	AllowFreeformCommands              bool                                   `json:"allow_freeform_commands"`
	RequireApprovalForMutatingCommands bool                                   `json:"require_approval_for_mutating_commands"`
	Presets                            []string                               `json:"presets"`
	Connectors                         map[string]ExternalCliConnectorOptions `json:"connectors"`
}

func DefaultExternalCliOptions() ExternalCliOptions {
	return ExternalCliOptions{
		Enabled:                            false,
		DefaultTimeoutSeconds:              60,
		MaxStdoutBytes:                     262144,
		MaxStderrBytes:                     65536,
		RedactSecrets:                      true,
		AllowFreeformCommands:              false,
		RequireApprovalForMutatingCommands: true,
		Presets:                            []string{},
		Connectors:                         make(map[string]ExternalCliConnectorOptions),
	}
}

type ExternalCliConnectorOptions struct {
	Enabled             bool                                 `json:"enabled"`
	DisplayName         string                               `json:"display_name"`
	Executable          string                               `json:"executable"`
	DefaultArgs         []string                             `json:"default_args"`
	WorkingDirectory    *string                              `json:"working_directory,omitempty"`
	Environment         map[string]string                    `json:"environment"`
	StatusCommand       *ExternalCliStatusCommandOptions     `json:"status_command,omitempty"`
	VersionCommand      *ExternalCliStatusCommandOptions     `json:"version_command,omitempty"`
	DefaultOutputFormat string                               `json:"default_output_format"`
	RequiresApproval    bool                                 `json:"requires_approval"`
	RedactionRules      []string                             `json:"redaction_rules"`
	Commands            map[string]ExternalCliCommandOptions `json:"commands"`
}

func DefaultExternalCliConnectorOptions() ExternalCliConnectorOptions {
	return ExternalCliConnectorOptions{
		Enabled:             false,
		DisplayName:         "",
		Executable:          "",
		DefaultArgs:         []string{},
		Environment:         make(map[string]string),
		DefaultOutputFormat: OutputFormatText,
		RequiresApproval:    false,
		RedactionRules:      []string{},
		Commands:            make(map[string]ExternalCliCommandOptions),
	}
}

type ExternalCliStatusCommandOptions struct {
	Args           []string `json:"args"`
	TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
}

func DefaultExternalCliStatusCommandOptions() ExternalCliStatusCommandOptions {
	return ExternalCliStatusCommandOptions{
		Args: []string{},
	}
}

type ExternalCliCommandOptions struct {
	Description            string                                 `json:"description"`
	ArgsTemplate           []string                               `json:"args_template"`
	Parameters             map[string]ExternalCliParameterOptions `json:"parameters"`
	AllowUnknownParameters bool                                   `json:"allow_unknown_parameters"`
	RiskLevel              string                                 `json:"risk_level"`
	ReadOnly               bool                                   `json:"read_only"`
	RequiresApproval       bool                                   `json:"requires_approval"`
	SupportsDryRun         bool                                   `json:"supports_dry_run"`
	DryRunArgsTemplate     []string                               `json:"dry_run_args_template"`
	StructuredOutput       string                                 `json:"structured_output"`
	TimeoutSeconds         *int                                   `json:"timeout_seconds,omitempty"`
	WorkingDirectory       *string                                `json:"working_directory,omitempty"`
	Environment            map[string]string                      `json:"environment"`
	RedactionRules         []string                               `json:"redaction_rules"`
	RequiredScopes         []string                               `json:"required_scopes"`
	RequiredIdentity       *string                                `json:"required_identity,omitempty"`
	Tags                   []string                               `json:"tags"`
}

func DefaultExternalCliCommandOptions() ExternalCliCommandOptions {
	return ExternalCliCommandOptions{
		Description:            "",
		ArgsTemplate:           []string{},
		Parameters:             make(map[string]ExternalCliParameterOptions),
		AllowUnknownParameters: false,
		RiskLevel:              RiskLevelLow,
		ReadOnly:               true,
		RequiresApproval:       false,
		SupportsDryRun:         false,
		DryRunArgsTemplate:     []string{},
		StructuredOutput:       "",
		Environment:            make(map[string]string),
		RedactionRules:         []string{},
		RequiredScopes:         []string{},
		Tags:                   []string{},
	}
}

type ExternalCliParameterOptions struct {
	Required      bool     `json:"required"`
	Description   string   `json:"description"`
	MaxLength     *int     `json:"max_length,omitempty"`
	Pattern       *string  `json:"pattern,omitempty"`
	AllowedValues []string `json:"allowed_values"`
}

func DefaultExternalCliParameterOptions() ExternalCliParameterOptions {
	return ExternalCliParameterOptions{
		Required:      false,
		Description:   "",
		AllowedValues: []string{},
	}
}

// --- Request Structs ---

type ExternalCliPreviewRequest struct {
	Connector     *string                    `json:"connector,omitempty"`
	Command       *string                    `json:"command,omitempty"`
	Parameters    map[string]json.RawMessage `json:"parameters"`
	ExecuteDryRun bool                       `json:"execute_dry_run"`
}

type ExternalCliExecuteRequest struct {
	Connector           *string                    `json:"connector,omitempty"`
	Command             *string                    `json:"command,omitempty"`
	Parameters          map[string]json.RawMessage `json:"parameters"`
	ApprovedFingerprint *string                    `json:"approved_fingerprint,omitempty"`
	ApprovalReason      *string                    `json:"approval_reason,omitempty"`
}

type ExternalCliToolRequest struct {
	Action              string                     `json:"action"`
	Connector           *string                    `json:"connector,omitempty"`
	Command             *string                    `json:"command,omitempty"`
	Parameters          map[string]json.RawMessage `json:"parameters"`
	ExecuteDryRun       bool                       `json:"execute_dry_run"`
	ApprovedFingerprint *string                    `json:"approved_fingerprint,omitempty"`
	ApprovalReason      *string                    `json:"approval_reason,omitempty"`
}

func DefaultExternalCliToolRequest() ExternalCliToolRequest {
	return ExternalCliToolRequest{
		Action:     "list_connectors",
		Parameters: make(map[string]json.RawMessage),
	}
}

// --- Response Structs ---

type ExternalCliConnectorSummary struct {
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Enabled      bool   `json:"enabled"`
	Executable   string `json:"executable"`
	CommandCount int    `json:"command_count"`
}

type ExternalCliConnectorListResponse struct {
	Items []ExternalCliConnectorSummary `json:"items"`
}

type ExternalCliPresetSummary struct {
	Id          string   `json:"id"`
	Connector   string   `json:"connector"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Commands    []string `json:"commands"`
}

type ExternalCliPresetListResponse struct {
	Items []ExternalCliPresetSummary `json:"items"`
}

type ExternalCliCommandSummary struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	RiskLevel        string   `json:"risk_level"`
	ReadOnly         bool     `json:"read_only"`
	RequiresApproval bool     `json:"requires_approval"`
	SupportsDryRun   bool     `json:"supports_dry_run"`
	StructuredOutput string   `json:"structured_output"`
	Tags             []string `json:"tags"`
}

func DefaultExternalCliCommandSummary() ExternalCliCommandSummary {
	return ExternalCliCommandSummary{
		RiskLevel:        RiskLevelLow,
		ReadOnly:         true,
		StructuredOutput: OutputFormatText,
		Tags:             []string{},
	}
}

type ExternalCliCommandListResponse struct {
	Connector string                      `json:"connector"`
	Items     []ExternalCliCommandSummary `json:"items"`
}

type ExternalCliCommandSchemaResponse struct {
	Connector          string                                 `json:"connector"`
	Command            string                                 `json:"command"`
	Description        string                                 `json:"description"`
	Parameters         map[string]ExternalCliParameterOptions `json:"parameters"`
	RequiredParameters []string                               `json:"required_parameters"`
	RiskLevel          string                                 `json:"risk_level"`
	ReadOnly           bool                                   `json:"read_only"`
	RequiresApproval   bool                                   `json:"requires_approval"`
	SupportsDryRun     bool                                   `json:"supports_dry_run"`
	StructuredOutput   string                                 `json:"structured_output"`
}

func DefaultExternalCliCommandSchemaResponse() ExternalCliCommandSchemaResponse {
	return ExternalCliCommandSchemaResponse{
		Parameters:         make(map[string]ExternalCliParameterOptions),
		RequiredParameters: []string{},
		RiskLevel:          RiskLevelLow,
		ReadOnly:           true,
		StructuredOutput:   OutputFormatText,
	}
}

type ExternalCliInvocationPreview struct {
	Connector           string   `json:"connector"`
	Command             string   `json:"command"`
	Executable          string   `json:"executable"`
	Arguments           []string `json:"arguments"`
	RedactedArguments   []string `json:"redacted_arguments"`
	RedactedCommandLine string   `json:"redacted_command_line"`
	RiskLevel           string   `json:"risk_level"`
	ReadOnly            bool     `json:"read_only"`
	RequiresApproval    bool     `json:"requires_approval"`
	SupportsDryRun      bool     `json:"supports_dry_run"`
	IsDryRun            bool     `json:"is_dry_run"`
	StructuredOutput    string   `json:"structured_output"`
	RequiredScopes      []string `json:"required_scopes"`
	RequiredIdentity    *string  `json:"required_identity,omitempty"`
	WorkingDirectory    *string  `json:"working_directory,omitempty"`
	TimeoutSeconds      int      `json:"timeout_seconds"`
	Fingerprint         string   `json:"fingerprint"`
	ParametersHash      string   `json:"parameters_hash"`
	Warnings            []string `json:"warnings"`
}

func DefaultExternalCliInvocationPreview() ExternalCliInvocationPreview {
	return ExternalCliInvocationPreview{
		Arguments:         []string{},
		RedactedArguments: []string{},
		RiskLevel:         RiskLevelLow,
		ReadOnly:          true,
		StructuredOutput:  OutputFormatText,
		RequiredScopes:    []string{},
		Warnings:          []string{},
	}
}

type ExternalCliPreviewResponse struct {
	Preview      ExternalCliInvocationPreview `json:"preview"`
	DryRunResult *ExternalCliExecutionResult  `json:"dry_run_result,omitempty"`
}

type ExternalCliExecutionResult struct {
	Preview         ExternalCliInvocationPreview `json:"preview"`
	Success         bool                         `json:"success"`
	ExitCode        int                          `json:"exit_code"`
	Stdout          string                       `json:"stdout"`
	Stderr          string                       `json:"stderr"`
	StdoutTruncated bool                         `json:"stdout_truncated"`
	StderrTruncated bool                         `json:"stderr_truncated"`
	TimedOut        bool                         `json:"timed_out"`
	DurationMs      float64                      `json:"duration_ms"`
	StartedAtUtc    time.Time                    `json:"started_at_utc"`
	CompletedAtUtc  time.Time                    `json:"completed_at_utc"`
	ParsedJson      *json.RawMessage             `json:"parsed_json,omitempty"`
	ParseError      *string                      `json:"parse_error,omitempty"`
	ErrorMessage    *string                      `json:"error_message,omitempty"`
}

type ExternalCliConnectorStatus struct {
	Connector              string    `json:"connector"`
	DisplayName            string    `json:"display_name"`
	Enabled                bool      `json:"enabled"`
	Executable             string    `json:"executable"`
	ExecutableFound        bool      `json:"executable_found"`
	ResolvedExecutablePath *string   `json:"resolved_executable_path,omitempty"`
	Version                *string   `json:"version,omitempty"`
	Authenticated          *bool     `json:"authenticated,omitempty"`
	AuthenticationStatus   string    `json:"authentication_status"`
	IdentitySummary        *string   `json:"identity_summary,omitempty"`
	GrantedScopes          []string  `json:"granted_scopes"`
	Warnings               []string  `json:"warnings"`
	LastCheckedAtUtc       time.Time `json:"last_checked_at_utc"`
}

func DefaultExternalCliConnectorStatus() ExternalCliConnectorStatus {
	return ExternalCliConnectorStatus{
		AuthenticationStatus: "unknown",
		GrantedScopes:        []string{},
		Warnings:             []string{},
		LastCheckedAtUtc:     time.Now().UTC(),
	}
}

// --- Audit & Diagnostics Structs ---

type ExternalCliAuditEntry struct {
	Id                  string    `json:"id"`
	TimestampUtc        time.Time `json:"timestamp_utc"`
	SessionId           string    `json:"session_id"`
	ChannelId           string    `json:"channel_id"`
	SenderId            string    `json:"sender_id"`
	ActorId             string    `json:"actor_id"`
	Connector           string    `json:"connector"`
	Command             string    `json:"command"`
	Executable          string    `json:"executable"`
	ArgsHash            string    `json:"args_hash"`
	RedactedArgsPreview string    `json:"redacted_args_preview"`
	ParametersHash      string    `json:"parameters_hash"`
	ApprovalId          *string   `json:"approval_id,omitempty"`
	ApprovalFingerprint *string   `json:"approval_fingerprint,omitempty"`
	ExitCode            int       `json:"exit_code"`
	DurationMs          float64   `json:"duration_ms"`
	TimedOut            bool      `json:"timed_out"`
	Failed              bool      `json:"failed"`
	StdoutTruncated     bool      `json:"stdout_truncated"`
	StderrTruncated     bool      `json:"stderr_truncated"`
	RiskLevel           string    `json:"risk_level"`
	ReadOnly            bool      `json:"read_only"`
	WorkingDirectory    *string   `json:"working_directory,omitempty"`
}

func DefaultExternalCliAuditEntry() ExternalCliAuditEntry {
	return ExternalCliAuditEntry{
		TimestampUtc: time.Now().UTC(),
		RiskLevel:    RiskLevelLow,
	}
}

type ExternalCliRuntimeEvent struct {
	SessionId string            `json:"session_id"`
	ChannelId string            `json:"channel_id"`
	SenderId  string            `json:"sender_id"`
	Action    string            `json:"action"`
	Severity  string            `json:"severity"`
	Summary   string            `json:"summary"`
	Metadata  map[string]string `json:"metadata"`
}

func DefaultExternalCliRuntimeEvent() ExternalCliRuntimeEvent {
	return ExternalCliRuntimeEvent{
		Severity: "info",
		Metadata: make(map[string]string),
	}
}

const (
	ExternalCliPresetCatalogRepoPattern       = "^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$"
	ExternalCliPresetCatalogNumberPattern     = "^[0-9]+$"
	ExternalCliPresetCatalogSimpleNamePattern = "^[A-Za-z0-9_.:-]+$"
)

type ExternalCliPresetCatalog struct{}
