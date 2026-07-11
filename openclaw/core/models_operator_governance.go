package core

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// OperatorRoleNames Constants & Helpers
// ============================================================================

const (
	OperatorRoleNamesViewer   = "viewer"
	OperatorRoleNamesOperator = "operator"
	OperatorRoleNamesAdmin    = "admin"
)

func OperatorRoleNamesNormalize(role *string) string {
	if role == nil {
		return OperatorRoleNamesViewer
	}
	r := strings.ToLower(strings.TrimSpace(*role))
	switch r {
	case OperatorRoleNamesViewer:
		return OperatorRoleNamesViewer
	case OperatorRoleNamesOperator:
		return OperatorRoleNamesOperator
	case OperatorRoleNamesAdmin:
		return OperatorRoleNamesAdmin
	default:
		return OperatorRoleNamesViewer
	}
}

func OperatorRoleNamesCanAccess(grantedRole, requiredRole string) bool {
	return operatorRoleNamesRank(OperatorRoleNamesNormalize(&grantedRole)) >= operatorRoleNamesRank(OperatorRoleNamesNormalize(&requiredRole))
}

func operatorRoleNamesRank(role string) int {
	switch role {
	case OperatorRoleNamesAdmin:
		return 3
	case OperatorRoleNamesOperator:
		return 2
	default:
		return 1
	}
}

// ============================================================================
// OrganizationAuthModeNames Constants
// ============================================================================

const (
	OrganizationAuthModeNamesBootstrapToken = "bootstrap_token"
	OrganizationAuthModeNamesBrowserSession = "browser_session"
	OrganizationAuthModeNamesAccountToken   = "account_token"
)

// ============================================================================
// SetupCheckStates Constants
// ============================================================================

const (
	SetupCheckStatesPass   = "pass"
	SetupCheckStatesWarn   = "warn"
	SetupCheckStatesFail   = "fail"
	SetupCheckStatesSkip   = "skip"
	SetupCheckStatesNotRun = "not_run"
)

// ============================================================================
// DoctorCheckCategories Constants
// ============================================================================

const (
	DoctorCheckCategoriesConfig        = "config"
	DoctorCheckCategoriesWorkspace     = "workspace"
	DoctorCheckCategoriesSecurity      = "security"
	DoctorCheckCategoriesBrowser       = "browser"
	DoctorCheckCategoriesOperator      = "operator"
	DoctorCheckCategoriesModelDoctor   = "model_doctor"
	DoctorCheckCategoriesProviderSmoke = "provider_smoke"
	DoctorCheckCategoriesRuntime       = "runtime"
	DoctorCheckCategoriesStorage       = "storage"
	DoctorCheckCategoriesNetwork       = "network"
	DoctorCheckCategoriesPlugins       = "plugins"
	DoctorCheckCategoriesChannels      = "channels"
)

// ============================================================================
// SetupVerificationSources Constants
// ============================================================================

const (
	SetupVerificationSourcesCli           = "cli"
	SetupVerificationSourcesAdmin         = "admin"
	SetupVerificationSourcesLaunchStartup = "launch-startup"
)

// ============================================================================
// ToolResultStatuses Constants
// ============================================================================

const (
	ToolResultStatusesFailed  = "failed"
	ToolResultStatusesBlocked = "blocked"
)

// ============================================================================
// ToolFailureCodes Constants
// ============================================================================

const (
	ToolFailureCodesPresetBlocked                = "preset_blocked"
	ToolFailureCodesOperatorAuthRequired         = "operator_auth_required"
	ToolFailureCodesApprovalRequired             = "approval_required"
	ToolFailureCodesGovernanceDenied             = "governance_denied"
	ToolFailureCodesGovernanceUnavailable        = "governance_unavailable"
	ToolFailureCodesRuntimeCapabilityUnavailable = "runtime_capability_unavailable"
	ToolFailureCodesBrowserBackendMissing        = "browser_backend_missing"
	ToolFailureCodesTimeout                      = "timeout"
	ToolFailureCodesToolFailed                   = "tool_failed"
)

// ============================================================================
// Structs & Default Initializers
// ============================================================================

type OperatorIdentitySnapshot struct {
	AuthMode         string  `json:"auth_mode"`
	Role             string  `json:"role"`
	AccountId        *string `json:"account_id"`
	Username         *string `json:"username"`
	DisplayName      *string `json:"display_name"`
	IsBootstrapAdmin bool    `json:"is_bootstrap_admin"`
}

func DefaultOperatorIdentitySnapshot() OperatorIdentitySnapshot {
	return OperatorIdentitySnapshot{
		AuthMode: "unauthorized",
		Role:     OperatorRoleNamesViewer,
	}
}

type OperatorAccountSummary struct {
	Id             string     `json:"id"`
	Username       string     `json:"username"`
	DisplayName    string     `json:"display_name"`
	Role           string     `json:"role"`
	Enabled        bool       `json:"enabled"`
	CreatedAtUtc   time.Time  `json:"created_at_utc"`
	UpdatedAtUtc   time.Time  `json:"updated_at_utc"`
	LastLoginAtUtc *time.Time `json:"last_login_at_utc"`
	TokenCount     int        `json:"token_count"`
}

func DefaultOperatorAccountSummary() OperatorAccountSummary {
	now := time.Now().UTC()
	return OperatorAccountSummary{
		DisplayName:  "",
		Role:         OperatorRoleNamesViewer,
		Enabled:      true,
		CreatedAtUtc: now,
		UpdatedAtUtc: now,
	}
}

type OperatorAccountTokenSummary struct {
	Id           string     `json:"id"`
	Label        string     `json:"label"`
	TokenPrefix  string     `json:"token_prefix"`
	CreatedAtUtc time.Time  `json:"created_at_utc"`
	ExpiresAtUtc *time.Time `json:"expires_at_utc"`
	RevokedAtUtc *time.Time `json:"revoked_at_utc"`
}

func DefaultOperatorAccountTokenSummary() OperatorAccountTokenSummary {
	return OperatorAccountTokenSummary{
		Label:        "",
		TokenPrefix:  "",
		CreatedAtUtc: time.Now().UTC(),
	}
}

type OperatorAccountListResponse struct {
	Items []OperatorAccountSummary `json:"items"`
}

func DefaultOperatorAccountListResponse() OperatorAccountListResponse {
	return OperatorAccountListResponse{
		Items: []OperatorAccountSummary{},
	}
}

type OperatorAccountDetailResponse struct {
	Account *OperatorAccountSummary       `json:"account"`
	Tokens  []OperatorAccountTokenSummary `json:"tokens"`
}

func DefaultOperatorAccountDetailResponse() OperatorAccountDetailResponse {
	return OperatorAccountDetailResponse{
		Tokens: []OperatorAccountTokenSummary{},
	}
}

type OperatorAccountCreateRequest struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	Role        string  `json:"role"`
	Password    *string `json:"password"`
	Enabled     bool    `json:"enabled"`
}

func DefaultOperatorAccountCreateRequest() OperatorAccountCreateRequest {
	return OperatorAccountCreateRequest{
		Role:    OperatorRoleNamesViewer,
		Enabled: true,
	}
}

type OperatorAccountUpdateRequest struct {
	DisplayName *string `json:"display_name"`
	Role        *string `json:"role"`
	Password    *string `json:"password"`
	Enabled     *bool   `json:"enabled"`
}

type OperatorAccountTokenCreateRequest struct {
	Label        *string    `json:"label"`
	ExpiresAtUtc *time.Time `json:"expires_at_utc"`
}

type OperatorAccountTokenCreateResponse struct {
	Account   *OperatorAccountSummary      `json:"account"`
	TokenInfo *OperatorAccountTokenSummary `json:"token_info"`
	Token     string                       `json:"token"`
}

func DefaultOperatorAccountTokenCreateResponse() OperatorAccountTokenCreateResponse {
	return OperatorAccountTokenCreateResponse{
		Token: "",
	}
}

type OrganizationPolicySnapshot struct {
	BootstrapTokenEnabled                       bool     `json:"bootstrap_token_enabled"`
	AllowedAuthModes                            []string `json:"allowed_auth_modes"`
	MinimumPluginTrustLevel                     string   `json:"minimum_plugin_trust_level"`
	ExportRetentionDays                         int      `json:"export_retention_days"`
	RequireInteractiveAdminForHighRiskMutations bool     `json:"require_interactive_admin_for_high_risk_mutations"`
	PublicDeploymentGuardrails                  bool     `json:"public_deployment_guardrails"`
}

func DefaultOrganizationPolicySnapshot() OrganizationPolicySnapshot {
	return OrganizationPolicySnapshot{
		BootstrapTokenEnabled: true,
		AllowedAuthModes: []string{
			OrganizationAuthModeNamesBootstrapToken,
			OrganizationAuthModeNamesBrowserSession,
			OrganizationAuthModeNamesAccountToken,
		},
		MinimumPluginTrustLevel: "untrusted",
		ExportRetentionDays:     30,
	}
}

type OrganizationPolicyResponse struct {
	Policy  OrganizationPolicySnapshot `json:"policy"`
	Message string                     `json:"message"`
}

func DefaultOrganizationPolicyResponse() OrganizationPolicyResponse {
	return OrganizationPolicyResponse{
		Policy:  DefaultOrganizationPolicySnapshot(),
		Message: "",
	}
}

type SetupArtifactStatusItem struct {
	Id     string  `json:"id"`
	Label  string  `json:"label"`
	Path   *string `json:"path"`
	Exists bool    `json:"exists"`
	Status string  `json:"status"`
}

func DefaultSetupArtifactStatusItem() SetupArtifactStatusItem {
	return SetupArtifactStatusItem{
		Status: "missing",
	}
}

type BrowserToolCapabilitySummary struct {
	ConfiguredEnabled          bool   `json:"configured_enabled"`
	LocalExecutionSupported    bool   `json:"local_execution_supported"`
	ExecutionBackendConfigured bool   `json:"execution_backend_configured"`
	Registered                 bool   `json:"registered"`
	Reason                     string `json:"reason"`
}

func DefaultBrowserToolCapabilitySummary() BrowserToolCapabilitySummary {
	return BrowserToolCapabilitySummary{
		Reason: "",
	}
}

type TailscaleServeStatusResponse struct {
	Mode                   string   `json:"mode"`
	LocalGatewayUrl        string   `json:"local_gateway_url"`
	SuggestedServeCommand  string   `json:"suggested_serve_command"`
	ServeDetected          string   `json:"serve_detected"`
	TailscaleCliDetected   bool     `json:"tailscale_cli_detected"`
	TailnetReachability    string   `json:"tailnet_reachability"`
	IdentityHeadersPresent bool     `json:"identity_headers_present"`
	PublicBind             bool     `json:"public_bind"`
	Warnings               []string `json:"warnings"`
}

func DefaultTailscaleServeStatusResponse() TailscaleServeStatusResponse {
	return TailscaleServeStatusResponse{
		Mode:                  "off",
		LocalGatewayUrl:       "http://127.0.0.1:18789",
		SuggestedServeCommand: "tailscale serve --bg http://127.0.0.1:18789",
		ServeDetected:         "unknown",
		TailnetReachability:   "unknown",
		Warnings:              []string{},
	}
}

type SetupStatusResponse struct {
	Profile                           string                        `json:"profile"`
	BindAddress                       string                        `json:"bind_address"`
	Port                              int                           `json:"port"`
	PublicBind                        bool                          `json:"public_bind"`
	AuthTokenConfigured               bool                          `json:"auth_token_configured"`
	BootstrapTokenEnabled             bool                          `json:"bootstrap_token_enabled"`
	AllowedAuthModes                  []string                      `json:"allowed_auth_modes"`
	MinimumPluginTrustLevel           string                        `json:"minimum_plugin_trust_level"`
	ReverseProxyRecommended           bool                          `json:"reverse_proxy_recommended"`
	ReachableBaseUrl                  string                        `json:"reachable_base_url"`
	WorkspacePath                     *string                       `json:"workspace_path"`
	WorkspaceExists                   bool                          `json:"workspace_exists"`
	HasOperatorAccounts               bool                          `json:"has_operator_accounts"`
	OperatorAccountCount              int                           `json:"operator_account_count"`
	ProviderConfigured                bool                          `json:"provider_configured"`
	ProviderSmokeStatus               string                        `json:"provider_smoke_status"`
	ModelDoctorStatus                 string                        `json:"model_doctor_status"`
	BrowserToolRegistered             bool                          `json:"browser_tool_registered"`
	BrowserExecutionBackendConfigured bool                          `json:"browser_execution_backend_configured"`
	BrowserCapabilityReason           string                        `json:"browser_capability_reason"`
	LastVerificationAtUtc             *time.Time                    `json:"last_verification_at_utc"`
	LastVerificationSource            *string                       `json:"last_verification_source"`
	LastVerificationStatus            string                        `json:"last_verification_status"`
	LastVerificationHasFailures       bool                          `json:"last_verification_has_failures"`
	LastVerificationHasWarnings       bool                          `json:"last_verification_has_warnings"`
	BootstrapGuidanceState            string                        `json:"bootstrap_guidance_state"`
	RecommendedNextActions            []string                      `json:"recommended_next_actions"`
	ChannelReadiness                  []ChannelReadinessDto         `json:"channel_readiness"`
	Artifacts                         []SetupArtifactStatusItem     `json:"artifacts"`
	Warnings                          []string                      `json:"warnings"`
	TailscaleServe                    *TailscaleServeStatusResponse `json:"tailscale_serve"`
	Reliability                       ReliabilitySnapshot           `json:"reliability"`
}

func DefaultSetupStatusResponse() SetupStatusResponse {
	return SetupStatusResponse{
		Profile:                 "local",
		MinimumPluginTrustLevel: "untrusted",
		ProviderSmokeStatus:     SetupCheckStatesNotRun,
		ModelDoctorStatus:       SetupCheckStatesSkip,
		LastVerificationStatus:  SetupCheckStatesNotRun,
		BootstrapGuidanceState:  "not_applicable",
		RecommendedNextActions:  []string{},
		ChannelReadiness:        []ChannelReadinessDto{},
		Artifacts:               []SetupArtifactStatusItem{},
		Warnings:                []string{},
	}
}

type SetupVerificationCheck struct {
	Id       string  `json:"id"`
	Label    string  `json:"label"`
	Category string  `json:"category"`
	Status   string  `json:"status"`
	Summary  string  `json:"summary"`
	Detail   *string `json:"detail"`
	NextStep *string `json:"next_step"`
}

func DefaultSetupVerificationCheck() SetupVerificationCheck {
	return SetupVerificationCheck{
		Category: DoctorCheckCategoriesConfig,
		Status:   SetupCheckStatesPass,
		Summary:  "",
	}
}

type SetupVerificationResponse struct {
	GeneratedAtUtc         time.Time                `json:"generated_at_utc"`
	OverallStatus          string                   `json:"overall_status"`
	HasFailures            bool                     `json:"has_failures"`
	HasWarnings            bool                     `json:"has_warnings"`
	HasSkips               bool                     `json:"has_skips"`
	Checks                 []SetupVerificationCheck `json:"checks"`
	RecommendedNextActions []string                 `json:"recommended_next_actions"`
}

func DefaultSetupVerificationResponse() SetupVerificationResponse {
	return SetupVerificationResponse{
		GeneratedAtUtc:         time.Now().UTC(),
		OverallStatus:          SetupCheckStatesPass,
		Checks:                 []SetupVerificationCheck{},
		RecommendedNextActions: []string{},
	}
}

type SetupVerificationSnapshot struct {
	RecordedAtUtc   time.Time                 `json:"recorded_at_utc"`
	Source          string                    `json:"source"`
	Offline         bool                      `json:"offline"`
	RequireProvider bool                      `json:"require_provider"`
	Verification    SetupVerificationResponse `json:"verification"`
}

func DefaultSetupVerificationSnapshot() SetupVerificationSnapshot {
	return SetupVerificationSnapshot{
		RecordedAtUtc: time.Now().UTC(),
		Source:        "",
		Verification:  DefaultSetupVerificationResponse(),
	}
}

type UpgradeRollbackSnapshotArtifact struct {
	Kind                 string  `json:"kind"`
	TargetPath           string  `json:"target_path"`
	Exists               bool    `json:"exists"`
	IsDirectory          bool    `json:"is_directory"`
	SnapshotRelativePath *string `json:"snapshot_relative_path"`
}

type UpgradeRollbackSnapshot struct {
	SchemaVersion      int                               `json:"schema_version"`
	SnapshotId         string                            `json:"snapshot_id"`
	CreatedAtUtc       time.Time                         `json:"created_at_utc"`
	CreatedByVersion   string                            `json:"created_by_version"`
	ConfigPath         string                            `json:"config_path"`
	WorkspacePath      *string                           `json:"workspace_path"`
	VerificationStatus string                            `json:"verification_status"`
	Offline            bool                              `json:"offline"`
	RequireProvider    bool                              `json:"require_provider"`
	Artifacts          []UpgradeRollbackSnapshotArtifact `json:"artifacts"`
}

func DefaultUpgradeRollbackSnapshot() UpgradeRollbackSnapshot {
	return UpgradeRollbackSnapshot{
		SchemaVersion:      1,
		SnapshotId:         uuid.NewString(),
		CreatedAtUtc:       time.Now().UTC(),
		CreatedByVersion:   "",
		ConfigPath:         "",
		VerificationStatus: SetupCheckStatesPass,
		Artifacts:          []UpgradeRollbackSnapshotArtifact{},
	}
}

type DoctorCheckItem struct {
	Id       string  `json:"id"`
	Label    string  `json:"label"`
	Category string  `json:"category"`
	Status   string  `json:"status"`
	Summary  string  `json:"summary"`
	Detail   *string `json:"detail"`
	NextStep *string `json:"next_step"`
}

func DefaultDoctorCheckItem() DoctorCheckItem {
	return DoctorCheckItem{
		Category: DoctorCheckCategoriesConfig,
		Status:   SetupCheckStatesPass,
	}
}

type DoctorReportResponse struct {
	GeneratedAtUtc         time.Time         `json:"generated_at_utc"`
	OverallStatus          string            `json:"overall_status"`
	HasFailures            bool              `json:"has_failures"`
	HasWarnings            bool              `json:"has_warnings"`
	HasSkips               bool              `json:"has_skips"`
	Checks                 []DoctorCheckItem `json:"checks"`
	RecommendedNextActions []string          `json:"recommended_next_actions"`
}

func DefaultDoctorReportResponse() DoctorReportResponse {
	return DoctorReportResponse{
		GeneratedAtUtc:         time.Now().UTC(),
		OverallStatus:          SetupCheckStatesPass,
		Checks:                 []DoctorCheckItem{},
		RecommendedNextActions: []string{},
	}
}

const (
	UpstreamMigrationSkillItemStatusSupported = "supported"
	UpstreamMigrationPluginItemStatusPartial  = "partial"
)

type ObservabilityMetricPoint struct {
	TimestampUtc       time.Time `json:"timestamp_utc"`
	ApprovalDecisions  int       `json:"approval_decisions"`
	ApprovalPending    int       `json:"approval_pending"`
	AutomationRuns     int       `json:"automation_runs"`
	AutomationFailures int       `json:"automation_failures"`
	ProviderErrors     int       `json:"provider_errors"`
	ProviderRetries    int       `json:"provider_retries"`
	RuntimeWarnings    int       `json:"runtime_warnings"`
	RuntimeErrors      int       `json:"runtime_errors"`
	DeadLetters        int       `json:"dead_letters"`
	ActiveSessions     int       `json:"active_sessions"`
	ChannelDrift       int       `json:"channel_drift"`
	OperatorActions    int       `json:"operator_actions"`
}

type ObservabilitySummaryCard struct {
	Id    string `json:"id"`
	Label string `json:"label"`
	Value int    `json:"value"`
	Note  string `json:"note"`
}

type ObservabilitySummaryResponse struct {
	GeneratedAtUtc           time.Time                  `json:"generated_at_utc"`
	Cards                    []ObservabilitySummaryCard `json:"cards"`
	ApprovalLatencyBuckets   []DashboardNamedMetric     `json:"approval_latency_buckets"`
	ProviderErrorsByRoute    []DashboardNamedMetric     `json:"provider_errors_by_route"`
	ProviderRetriesByRoute   []DashboardNamedMetric     `json:"provider_retries_by_route"`
	OperatorActions          []DashboardNamedMetric     `json:"operator_actions"`
	OperatorActionsByRole    []DashboardNamedMetric     `json:"operator_actions_by_role"`
	OperatorActionsByAccount []DashboardNamedMetric     `json:"operator_actions_by_account"`
	ChannelDrift             []DashboardNamedMetric     `json:"channel_drift"`
}

func DefaultObservabilitySummaryResponse() ObservabilitySummaryResponse {
	return ObservabilitySummaryResponse{
		GeneratedAtUtc:           time.Now().UTC(),
		Cards:                    []ObservabilitySummaryCard{},
		ApprovalLatencyBuckets:   []DashboardNamedMetric{},
		ProviderErrorsByRoute:    []DashboardNamedMetric{},
		ProviderRetriesByRoute:   []DashboardNamedMetric{},
		OperatorActions:          []DashboardNamedMetric{},
		OperatorActionsByRole:    []DashboardNamedMetric{},
		OperatorActionsByAccount: []DashboardNamedMetric{},
		ChannelDrift:             []DashboardNamedMetric{},
	}
}

type OperatorInsightsResponse struct {
	GeneratedAtUtc time.Time                       `json:"generated_at_utc"`
	StartUtc       time.Time                       `json:"start_utc"`
	EndUtc         time.Time                       `json:"end_utc"`
	Totals         OperatorInsightsTotals          `json:"totals"`
	Sessions       OperatorInsightsSessionCounts   `json:"sessions"`
	Providers      []OperatorInsightsProviderUsage `json:"providers"`
	Tools          []OperatorInsightsToolFrequency `json:"tools"`
	Warnings       []string                        `json:"warnings"`
}

func DefaultOperatorInsightsResponse() OperatorInsightsResponse {
	return OperatorInsightsResponse{
		GeneratedAtUtc: time.Now().UTC(),
		Providers:      []OperatorInsightsProviderUsage{},
		Tools:          []OperatorInsightsToolFrequency{},
		Warnings:       []string{},
	}
}

type OperatorInsightsTotals struct {
	ProviderRequests int64   `json:"provider_requests"`
	ProviderErrors   int64   `json:"provider_errors"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	EstimatedCostUsd float64 `json:"estimated_cost_usd"`
	ToolCalls        int64   `json:"tool_calls"`
}

func (o OperatorInsightsTotals) TotalTokens() int64 {
	return o.InputTokens + o.OutputTokens
}

type OperatorInsightsSessionCounts struct {
	Active      int                    `json:"active"`
	Persisted   int                    `json:"persisted"`
	UniqueTotal int                    `json:"unique_total"`
	Last24Hours int                    `json:"last24_hours"`
	Last7Days   int                    `json:"last7_days"`
	InRange     int                    `json:"in_range"`
	ByChannel   []DashboardNamedMetric `json:"by_channel"`
	ByState     []DashboardNamedMetric `json:"by_state"`
}

func DefaultOperatorInsightsSessionCounts() OperatorInsightsSessionCounts {
	return OperatorInsightsSessionCounts{
		ByChannel: []DashboardNamedMetric{},
		ByState:   []DashboardNamedMetric{},
	}
}

type OperatorInsightsProviderUsage struct {
	ProviderId       string  `json:"provider_id"`
	ModelId          string  `json:"model_id"`
	Requests         int64   `json:"requests"`
	Retries          int64   `json:"retries"`
	Errors           int64   `json:"errors"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	EstimatedCostUsd float64 `json:"estimated_cost_usd"`
}

func (o OperatorInsightsProviderUsage) TotalTokens() int64 {
	return o.InputTokens + o.OutputTokens
}

type OperatorInsightsToolFrequency struct {
	ToolName          string  `json:"tool_name"`
	Calls             int64   `json:"calls"`
	Failures          int64   `json:"failures"`
	Timeouts          int64   `json:"timeouts"`
	AverageDurationMs float64 `json:"average_duration_ms"`
}

type ObservabilitySeriesResponse struct {
	GeneratedAtUtc time.Time                  `json:"generated_at_utc"`
	StartUtc       time.Time                  `json:"start_utc"`
	EndUtc         time.Time                  `json:"end_utc"`
	BucketMinutes  int                        `json:"bucket_minutes"`
	Points         []ObservabilityMetricPoint `json:"points"`
}

func DefaultObservabilitySeriesResponse() ObservabilitySeriesResponse {
	return ObservabilitySeriesResponse{
		GeneratedAtUtc: time.Now().UTC(),
		BucketMinutes:  60,
		Points:         []ObservabilityMetricPoint{},
	}
}

type AuditExportManifest struct {
	SchemaVersion                  int                         `json:"schema_version"`
	GeneratedAtUtc                 time.Time                   `json:"generated_at_utc"`
	StartUtc                       *time.Time                  `json:"start_utc,omitempty"`
	EndUtc                         *time.Time                  `json:"end_utc,omitempty"`
	Files                          []string                    `json:"files"`
	Policy                         *OrganizationPolicySnapshot `json:"policy,omitempty"`
	RetentionDays                  int                         `json:"retention_days"`
	OperatorAuditSequenceStart     *int64                      `json:"operator_audit_sequence_start,omitempty"`
	OperatorAuditSequenceEnd       *int64                      `json:"operator_audit_sequence_end,omitempty"`
	OperatorAuditPreviousEntryHash *string                     `json:"operator_audit_previous_entry_hash,omitempty"`
	OperatorAuditLastEntryHash     *string                     `json:"operator_audit_last_entry_hash,omitempty"`
	FileEntryCounts                map[string]int              `json:"file_entry_counts"`
	Warnings                       []string                    `json:"warnings"`
}

func DefaultAuditExportManifest() AuditExportManifest {
	return AuditExportManifest{
		SchemaVersion:  1,
		GeneratedAtUtc: time.Now().UTC(),
		Files:          []string{},
		Warnings:       []string{},
	}
}

type TrajectoryExportRecord struct {
	SchemaVersion         int                    `json:"schema_version"`
	Type                  string                 `json:"type"`
	TimestampUtc          time.Time              `json:"timestamp_utc"`
	SessionId             string                 `json:"session_id"`
	ChannelId             string                 `json:"channel_id"`
	SenderId              string                 `json:"sender_id"`
	TurnIndex             int                    `json:"turn_index"`
	Role                  *string                `json:"role,omitempty"`
	Content               *string                `json:"content,omitempty"`
	ToolName              *string                `json:"tool_name,omitempty"`
	CallId                *string                `json:"call_id,omitempty"`
	Arguments             *string                `json:"arguments,omitempty"`
	Result                *string                `json:"result,omitempty"`
	DurationMs            *int64                 `json:"duration_ms,omitempty"`
	ResultStatus          *string                `json:"result_status,omitempty"`
	FailureCode           *string                `json:"failure_code,omitempty"`
	FailureMessage        *string                `json:"failure_message,omitempty"`
	EvidenceBundle        *EvidenceBundle        `json:"evidence_bundle,omitempty"`
	GovernanceLedgerEntry *GovernanceLedgerEntry `json:"governance_ledger_entry,omitempty"`
	Anonymized            bool                   `json:"anonymized"`
}

func DefaultTrajectoryExportRecord() TrajectoryExportRecord {
	return TrajectoryExportRecord{
		SchemaVersion: 1,
	}
}

type UpstreamMigrationCompatibilityItem struct {
	Type     string   `json:"type"`
	Subject  string   `json:"subject"`
	Status   string   `json:"status"`
	Summary  string   `json:"summary"`
	Warnings []string `json:"warnings"`
}

type UpstreamMigrationSkillItem struct {
	Name       string `json:"name"`
	SourcePath string `json:"source_path"`
	TargetSlug string `json:"target_slug"`
	Status     string `json:"status"`
}

func DefaultUpstreamMigrationSkillItem() UpstreamMigrationSkillItem {
	return UpstreamMigrationSkillItem{
		Status: UpstreamMigrationSkillItemStatusSupported,
	}
}

type UpstreamMigrationPluginItem struct {
	Subject     string   `json:"subject"`
	PackageSpec *string  `json:"package_spec,omitempty"`
	Status      string   `json:"status"`
	Guidance    []string `json:"guidance"`
}

func DefaultUpstreamMigrationPluginItem() UpstreamMigrationPluginItem {
	return UpstreamMigrationPluginItem{
		Status:   UpstreamMigrationPluginItemStatusPartial,
		Guidance: []string{},
	}
}

type UpstreamMigrationReport struct {
	GeneratedAtUtc       time.Time                            `json:"generated_at_utc"`
	SourcePath           string                               `json:"source_path"`
	TargetConfigPath     string                               `json:"target_config_path"`
	DiscoveredConfigPath *string                              `json:"discovered_config_path,omitempty"`
	ManagedSkillRootPath *string                              `json:"managed_skill_root_path,omitempty"`
	PluginReviewPlanPath *string                              `json:"plugin_review_plan_path,omitempty"`
	Applied              bool                                 `json:"applied"`
	Compatibility        []UpstreamMigrationCompatibilityItem `json:"compatibility"`
	Skills               []UpstreamMigrationSkillItem         `json:"skills"`
	Plugins              []UpstreamMigrationPluginItem        `json:"plugins"`
	Warnings             []string                             `json:"warnings"`
	SkippedSettings      []string                             `json:"skipped_settings"`
}

func DefaultUpstreamMigrationReport() UpstreamMigrationReport {
	return UpstreamMigrationReport{
		GeneratedAtUtc:  time.Now().UTC(),
		Compatibility:   []UpstreamMigrationCompatibilityItem{},
		Skills:          []UpstreamMigrationSkillItem{},
		Plugins:         []UpstreamMigrationPluginItem{},
		Warnings:        []string{},
		SkippedSettings: []string{},
	}
}
