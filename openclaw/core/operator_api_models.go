package core

import "time"

const (
	ProviderRouteHealthSnapshotCircuitStateClosed  = "Closed"
	PluginHealthSnapshotTrustLevelUntrusted        = "untrusted"
	PluginHealthSnapshotCompatibilityStatusUnknown = "unknown"
	SkillHealthSnapshotTrustLevelUntrusted         = "untrusted"
)

type MutationResponse struct {
	Success         bool    `json:"success"`
	Message         string  `json:"message"`
	Error           *string `json:"error"`
	RestartRequired bool    `json:"restart_required"`
}

type InputTokenComponentEstimate struct {
	SystemPrompt int64 `json:"system_prompt"`
	Skills       int64 `json:"skills"`
	History      int64 `json:"history"`
	ToolOutputs  int64 `json:"tool_outputs"`
	UserInput    int64 `json:"user_input"`
}

type ProviderPolicyRule struct {
	Id              string    `json:"id"`
	Priority        int       `json:"priority"`
	Enabled         bool      `json:"enabled"`
	ChannelId       *string   `json:"channel_id"`
	SenderId        *string   `json:"sender_id"`
	SessionId       *string   `json:"session_id"`
	ProviderId      string    `json:"provider_id"`
	ModelId         string    `json:"model_id"`
	FallbackModels  []string  `json:"fallback_models"`
	MaxInputTokens  int       `json:"max_input_tokens"`
	MaxOutputTokens int       `json:"max_output_tokens"`
	MaxTotalTokens  int       `json:"max_total_tokens"`
	CreatedAtUtc    time.Time `json:"created_at_utc"`
}

func NewDefaultProviderPolicyRule() *ProviderPolicyRule {
	return &ProviderPolicyRule{
		Enabled:        true,
		FallbackModels: []string{},
		CreatedAtUtc:   time.Now().UTC(),
	}
}

type ProviderPolicyListResponse struct {
	Items []*ProviderPolicyRule `json:"items"`
}

func NewDefaultProviderPolicyListResponse() *ProviderPolicyListResponse {
	return &ProviderPolicyListResponse{
		Items: []*ProviderPolicyRule{},
	}
}

type ProviderRouteHealthSnapshot struct {
	ProfileId        *string    `json:"profile_id"`
	ProviderId       string     `json:"provider_id"`
	ModelId          string     `json:"model_id"`
	IsDefaultRoute   bool       `json:"is_default_route"`
	IsDynamic        bool       `json:"is_dynamic"`
	OwnerId          *string    `json:"owner_id"`
	Tags             []string   `json:"tags"`
	ValidationIssues []string   `json:"validation_issues"`
	CircuitState     string     `json:"circuit_state"`
	Requests         int64      `json:"requests"`
	Retries          int64      `json:"retries"`
	Errors           int64      `json:"errors"`
	LastErrorAtUtc   *time.Time `json:"last_error_at_utc"`
	LastError        *string    `json:"last_error"`
}

func NewDefaultProviderRouteHealthSnapshot() *ProviderRouteHealthSnapshot {
	return &ProviderRouteHealthSnapshot{
		Tags:             []string{},
		ValidationIssues: []string{},
		CircuitState:     ProviderRouteHealthSnapshotCircuitStateClosed,
	}
}

type ProviderTurnUsageEntry struct {
	TimestampUtc                    time.Time                    `json:"timestamp_utc"`
	SessionId                       string                       `json:"session_id"`
	ChannelId                       string                       `json:"channel_id"`
	ProviderId                      string                       `json:"provider_id"`
	ModelId                         string                       `json:"model_id"`
	InputTokens                     int64                        `json:"input_tokens"`
	OutputTokens                    int64                        `json:"output_tokens"`
	CacheReadTokens                 int64                        `json:"cache_read_tokens"`
	CacheWriteTokens                int64                        `json:"cache_write_tokens"`
	EstimatedInputTokensByComponent *InputTokenComponentEstimate `json:"estimated_input_tokens_by_component"`
}

func NewDefaultProviderTurnUsageEntry() *ProviderTurnUsageEntry {
	return &ProviderTurnUsageEntry{
		TimestampUtc:                    time.Now().UTC(),
		EstimatedInputTokensByComponent: &InputTokenComponentEstimate{},
	}
}

type ProviderAdminResponse struct {
	Routes        []*ProviderRouteHealthSnapshot `json:"routes"`
	ModelProfiles *ModelProfilesStatusResponse   `json:"model_profiles"`
	Usage         []*ProviderUsageSnapshot       `json:"usage"`
	Policies      []*ProviderPolicyRule          `json:"policies"`
	RecentTurns   []*ProviderTurnUsageEntry      `json:"recent_turns"`
}

func NewDefaultProviderAdminResponse() *ProviderAdminResponse {
	return &ProviderAdminResponse{
		Routes:      []*ProviderRouteHealthSnapshot{},
		Usage:       []*ProviderUsageSnapshot{},
		Policies:    []*ProviderPolicyRule{},
		RecentTurns: []*ProviderTurnUsageEntry{},
	}
}

type RuntimeEventQuery struct {
	Limit     int        `json:"limit"`
	SessionId *string    `json:"session_id"`
	ChannelId *string    `json:"channel_id"`
	SenderId  *string    `json:"sender_id"`
	Component *string    `json:"component"`
	Action    *string    `json:"action"`
	FromUtc   *time.Time `json:"from_utc"`
	ToUtc     *time.Time `json:"to_utc"`
}

func NewDefaultRuntimeEventQuery() *RuntimeEventQuery {
	return &RuntimeEventQuery{
		Limit: 100,
	}
}

type RuntimeEventEntry struct {
	Id            string            `json:"id"`
	TimestampUtc  time.Time         `json:"timestamp_utc"`
	SessionId     *string           `json:"session_id"`
	ChannelId     *string           `json:"channel_id"`
	SenderId      *string           `json:"sender_id"`
	CorrelationId *string           `json:"correlation_id"`
	Component     string            `json:"component"`
	Action        string            `json:"action"`
	Severity      string            `json:"severity"`
	Summary       string            `json:"summary"`
	Metadata      map[string]string `json:"metadata"`
}

func NewDefaultRuntimeEventEntry() *RuntimeEventEntry {
	return &RuntimeEventEntry{
		TimestampUtc: time.Now().UTC(),
		Summary:      "",
	}
}

type RuntimeEventListResponse struct {
	Items []*RuntimeEventEntry `json:"items"`
}

func NewDefaultRuntimeEventListResponse() *RuntimeEventListResponse {
	return &RuntimeEventListResponse{
		Items: []*RuntimeEventEntry{},
	}
}

type PluginOperatorState struct {
	PluginId         string    `json:"plugin_id"`
	Disabled         bool      `json:"disabled"`
	Quarantined      bool      `json:"quarantined"`
	QuarantineSource *string   `json:"quarantine_source"`
	Reviewed         bool      `json:"reviewed"`
	Reason           *string   `json:"reason"`
	ReviewNotes      *string   `json:"review_notes"`
	UpdatedAtUtc     time.Time `json:"updated_at_utc"`
}

func NewDefaultPluginOperatorState() *PluginOperatorState {
	return &PluginOperatorState{
		UpdatedAtUtc: time.Now().UTC(),
	}
}

type PluginHealthSnapshot struct {
	PluginId              string                           `json:"plugin_id"`
	Origin                string                           `json:"origin"`
	Loaded                bool                             `json:"loaded"`
	BlockedByRuntimeMode  bool                             `json:"blocked_by_runtime_mode"`
	Disabled              bool                             `json:"disabled"`
	Quarantined           bool                             `json:"quarantined"`
	QuarantineSource      *string                          `json:"quarantine_source"`
	Reviewed              bool                             `json:"reviewed"`
	PendingReason         *string                          `json:"pending_reason"`
	ReviewNotes           *string                          `json:"review_notes"`
	EffectiveRuntimeMode  *string                          `json:"effective_runtime_mode"`
	TrustLevel            string                           `json:"trust_level"`
	TrustReason           string                           `json:"trust_reason"`
	CompatibilityStatus   string                           `json:"compatibility_status"`
	ErrorCount            int                              `json:"error_count"`
	WarningCount          int                              `json:"warning_count"`
	DeclaredSurface       string                           `json:"declared_surface"`
	SourcePath            *string                          `json:"source_path"`
	EntryPath             *string                          `json:"entry_path"`
	RequestedCapabilities []string                         `json:"requested_capabilities"`
	SkillDirectories      []string                         `json:"skill_directories"`
	LastError             *string                          `json:"last_error"`
	LastActivityAtUtc     *time.Time                       `json:"last_activity_at_utc"`
	RestartCount          int                              `json:"restart_count"`
	WorkingSetBytes       *int64                           `json:"working_set_bytes"`
	PrivateMemoryBytes    *int64                           `json:"private_memory_bytes"`
	ToolCount             int                              `json:"tool_count"`
	ChannelCount          int                              `json:"channel_count"`
	CommandCount          int                              `json:"command_count"`
	ProviderCount         int                              `json:"provider_count"`
	BudgetViolations      []string                         `json:"budget_violations"`
	Diagnostics           []*PluginCompatibilityDiagnostic `json:"diagnostics"`
}

func NewDefaultPluginHealthSnapshot() *PluginHealthSnapshot {
	return &PluginHealthSnapshot{
		TrustLevel:            PluginHealthSnapshotTrustLevelUntrusted,
		TrustReason:           "",
		CompatibilityStatus:   PluginHealthSnapshotCompatibilityStatusUnknown,
		DeclaredSurface:       "",
		RequestedCapabilities: []string{},
		SkillDirectories:      []string{},
		BudgetViolations:      []string{},
		Diagnostics:           []*PluginCompatibilityDiagnostic{},
	}
}

type PluginListResponse struct {
	Items []*PluginHealthSnapshot `json:"items"`
}

func NewDefaultPluginListResponse() *PluginListResponse {
	return &PluginListResponse{
		Items: []*PluginHealthSnapshot{},
	}
}

type SkillHealthSnapshot struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Source                 string   `json:"source"`
	Location               string   `json:"location"`
	TrustLevel             string   `json:"trust_level"`
	TrustReason            string   `json:"trust_reason"`
	Always                 bool     `json:"always"`
	UserInvocable          bool     `json:"user_invocable"`
	DisableModelInvocation bool     `json:"disable_model_invocation"`
	CommandDispatch        *string  `json:"command_dispatch"`
	CommandTool            *string  `json:"command_tool"`
	CommandArgMode         *string  `json:"command_arg_mode"`
	Homepage               *string  `json:"homepage"`
	PrimaryEnv             *string  `json:"primary_env"`
	RequiredBins           []string `json:"required_bins"`
	RequiredAnyBins        []string `json:"required_any_bins"`
	RequiredEnv            []string `json:"required_env"`
	RequiredConfig         []string `json:"required_config"`
	Warnings               []string `json:"warnings"`
}

func NewDefaultSkillHealthSnapshot() *SkillHealthSnapshot {
	return &SkillHealthSnapshot{
		Description:     "",
		TrustLevel:      SkillHealthSnapshotTrustLevelUntrusted,
		TrustReason:     "",
		UserInvocable:   true,
		RequiredBins:    []string{},
		RequiredAnyBins: []string{},
		RequiredEnv:     []string{},
		RequiredConfig:  []string{},
		Warnings:        []string{},
	}
}

type SkillListResponse struct {
	Items []*SkillHealthSnapshot `json:"items"`
}

func NewDefaultSkillListResponse() *SkillListResponse {
	return &SkillListResponse{
		Items: []*SkillHealthSnapshot{},
	}
}

type SkillCostBreakdown struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	EagerCharacters    int    `json:"eager_characters"`
	IndexCharacters    int    `json:"index_characters"`
	ResourceCount      int    `json:"resource_count"`
	InstructionsLength int    `json:"instructions_length"`
	ExcludedFromModel  bool   `json:"excluded_from_model"`
}

func NewDefaultSkillCostBreakdown() *SkillCostBreakdown {
	return &SkillCostBreakdown{
		Description: "",
	}
}

type SkillCostEstimateResponse struct {
	TotalSkills          int                   `json:"total_skills"`
	ModelInvocableSkills int                   `json:"model_invocable_skills"`
	EagerCharacters      int                   `json:"eager_characters"`
	IndexCharacters      int                   `json:"index_characters"`
	CharactersSaved      int                   `json:"characters_saved"`
	SavedRatio           float64               `json:"saved_ratio"`
	EagerTokensEstimate  int                   `json:"eager_tokens_estimate"`
	IndexTokensEstimate  int                   `json:"index_tokens_estimate"`
	Items                []*SkillCostBreakdown `json:"items"`
	GeneratedAt          time.Time             `json:"generated_at"`
}

func NewDefaultSkillCostEstimateResponse() *SkillCostEstimateResponse {
	return &SkillCostEstimateResponse{
		Items: []*SkillCostBreakdown{},
	}
}

const (
	MemoryNoteClassGeneral            = "general"
	MemoryNoteClassProjectFact        = "project_fact"
	MemoryNoteClassOperationalRunbook = "operational_runbook"
	MemoryNoteClassApprovedSkill      = "approved_skill"
	MemoryNoteClassApprovedAutomation = "approved_automation"
)

// ==========================================
// ChannelAuthStatusResponse
// ==========================================

type ChannelAuthStatusResponse struct {
	Items []ChannelAuthStatusItem `json:"items"`
}

func NewChannelAuthStatusResponse() *ChannelAuthStatusResponse {
	return &ChannelAuthStatusResponse{
		Items: make([]ChannelAuthStatusItem, 0),
	}
}

// ==========================================
// ChannelAuthStatusItem
// ==========================================

type ChannelAuthStatusItem struct {
	ChannelId    string    `json:"channel_id"`
	State        string    `json:"state"`
	Data         *string   `json:"data,omitempty"`
	AccountId    *string   `json:"account_id,omitempty"`
	UpdatedAtUtc time.Time `json:"updated_at_utc"`
}

func NewChannelAuthStatusItem() *ChannelAuthStatusItem {
	return &ChannelAuthStatusItem{
		UpdatedAtUtc: time.Now().UTC(),
	}
}

// ==========================================
// WhatsAppSetupRequest
// ==========================================

type WhatsAppSetupRequest struct {
	Enabled                      bool                            `json:"enabled"`
	Type                         string                          `json:"type"`
	DmPolicy                     string                          `json:"dm_policy"`
	WebhookPath                  string                          `json:"webhook_path"`
	WebhookPublicBaseUrl         *string                         `json:"webhook_public_base_url,omitempty"`
	WebhookVerifyToken           string                          `json:"webhook_verify_token"`
	WebhookVerifyTokenRef        string                          `json:"webhook_verify_token_ref"`
	ValidateSignature            bool                            `json:"validate_signature"`
	WebhookAppSecret             *string                         `json:"webhook_app_secret,omitempty"`
	WebhookAppSecretRef          string                          `json:"webhook_app_secret_ref"`
	CloudApiToken                *string                         `json:"cloud_api_token,omitempty"`
	CloudApiTokenRef             string                          `json:"cloud_api_token_ref"`
	PhoneNumberId                *string                         `json:"phone_number_id,omitempty"`
	BusinessAccountId            *string                         `json:"business_account_id,omitempty"`
	BridgeUrl                    *string                         `json:"bridge_url,omitempty"`
	BridgeToken                  *string                         `json:"bridge_token,omitempty"`
	BridgeTokenRef               string                          `json:"bridge_token_ref"`
	BridgeSuppressSendExceptions bool                            `json:"bridge_suppress_send_exceptions"`
	PluginId                     *string                         `json:"plugin_id,omitempty"`
	PluginConfigJson             *string                         `json:"plugin_config_json,omitempty"`
	FirstPartyWorker             *WhatsAppFirstPartyWorkerConfig `json:"first_party_worker,omitempty"`
	FirstPartyWorkerConfigJson   *string                         `json:"first_party_worker_config_json,omitempty"`
}

func NewWhatsAppSetupRequest() *WhatsAppSetupRequest {
	return &WhatsAppSetupRequest{
		Type:                         "official",
		DmPolicy:                     "pairing",
		WebhookPath:                  "/whatsapp/inbound",
		WebhookVerifyToken:           "openclaw-verify",
		WebhookVerifyTokenRef:        "env:WHATSAPP_VERIFY_TOKEN",
		WebhookAppSecretRef:          "env:WHATSAPP_APP_SECRET",
		CloudApiTokenRef:             "env:WHATSAPP_CLOUD_API_TOKEN",
		BridgeTokenRef:               "env:WHATSAPP_BRIDGE_TOKEN",
		BridgeSuppressSendExceptions: false,
	}
}

// ==========================================
// WhatsAppSetupResponse
// ==========================================

type WhatsAppSetupResponse struct {
	ActiveBackend                    string                          `json:"active_backend"`
	ConfiguredType                   string                          `json:"configured_type"`
	Message                          string                          `json:"message"`
	RestartRequired                  bool                            `json:"restart_required"`
	Enabled                          bool                            `json:"enabled"`
	DmPolicy                         string                          `json:"dm_policy"`
	WebhookPath                      string                          `json:"webhook_path"`
	WebhookPublicBaseUrl             *string                         `json:"webhook_public_base_url,omitempty"`
	WebhookVerifyToken               string                          `json:"webhook_verify_token"`
	WebhookVerifyTokenConfigured     bool                            `json:"webhook_verify_token_configured"`
	WebhookVerifyTokenRef            string                          `json:"webhook_verify_token_ref"`
	ValidateSignature                bool                            `json:"validate_signature"`
	WebhookAppSecret                 *string                         `json:"webhook_app_secret,omitempty"`
	WebhookAppSecretConfigured       bool                            `json:"webhook_app_secret_configured"`
	WebhookAppSecretRef              string                          `json:"webhook_app_secret_ref"`
	CloudApiToken                    *string                         `json:"cloud_api_token,omitempty"`
	CloudApiTokenConfigured          bool                            `json:"cloud_api_token_configured"`
	CloudApiTokenRef                 string                          `json:"cloud_api_token_ref"`
	PhoneNumberId                    *string                         `json:"phone_number_id,omitempty"`
	BusinessAccountId                *string                         `json:"business_account_id,omitempty"`
	BridgeUrl                        *string                         `json:"bridge_url,omitempty"`
	BridgeToken                      *string                         `json:"bridge_token,omitempty"`
	BridgeTokenConfigured            bool                            `json:"bridge_token_configured"`
	BridgeTokenRef                   string                          `json:"bridge_token_ref"`
	BridgeSuppressSendExceptions     bool                            `json:"bridge_suppress_send_exceptions"`
	FirstPartyWorker                 *WhatsAppFirstPartyWorkerConfig `json:"first_party_worker,omitempty"`
	FirstPartyWorkerConfigJson       *string                         `json:"first_party_worker_config_json,omitempty"`
	FirstPartyWorkerConfigSchemaJson *string                         `json:"first_party_worker_config_schema_json,omitempty"`
	PluginDetected                   bool                            `json:"plugin_detected"`
	PluginId                         *string                         `json:"plugin_id,omitempty"`
	PluginConfigJson                 *string                         `json:"plugin_config_json,omitempty"`
	PluginConfigSchemaJson           *string                         `json:"plugin_config_schema_json,omitempty"`
	PluginUiHintsJson                *string                         `json:"plugin_ui_hints_json,omitempty"`
	PluginWarning                    *string                         `json:"plugin_warning,omitempty"`
	RestartSupported                 bool                            `json:"restart_supported"`
	RestartHint                      *string                         `json:"restart_hint,omitempty"`
	DerivedWebhookUrl                *string                         `json:"derived_webhook_url,omitempty"`
	Readiness                        *ChannelReadinessDto            `json:"readiness,omitempty"`
	AuthStates                       []ChannelAuthStatusItem         `json:"auth_states"`
	Warnings                         []string                        `json:"warnings"`
	ValidationErrors                 []string                        `json:"validation_errors"`
}

func NewWhatsAppSetupResponse() *WhatsAppSetupResponse {
	return &WhatsAppSetupResponse{
		Message:               "",
		DmPolicy:              "pairing",
		WebhookPath:           "/whatsapp/inbound",
		WebhookVerifyToken:    "openclaw-verify",
		WebhookVerifyTokenRef: "env:WHATSAPP_VERIFY_TOKEN",
		WebhookAppSecretRef:   "env:WHATSAPP_APP_SECRET",
		CloudApiTokenRef:      "env:WHATSAPP_CLOUD_API_TOKEN",
		BridgeTokenRef:        "env:WHATSAPP_BRIDGE_TOKEN",
		AuthStates:            make([]ChannelAuthStatusItem, 0),
		Warnings:              make([]string, 0),
		ValidationErrors:      make([]string, 0),
	}
}

// ==========================================
// PluginMutationRequest
// ==========================================

type PluginMutationRequest struct {
	Reason *string `json:"reason,omitempty"`
}

// ==========================================
// ToolApprovalGrant
// ==========================================

type ToolApprovalGrant struct {
	Id            string     `json:"id"`
	Scope         string     `json:"scope"`
	ChannelId     *string    `json:"channel_id,omitempty"`
	SenderId      *string    `json:"sender_id,omitempty"`
	SessionId     *string    `json:"session_id,omitempty"`
	ToolName      string     `json:"tool_name"`
	CreatedAtUtc  time.Time  `json:"created_at_utc"`
	ExpiresAtUtc  *time.Time `json:"expires_at_utc,omitempty"`
	GrantedBy     string     `json:"granted_by"`
	GrantSource   string     `json:"grant_source"`
	RemainingUses int        `json:"remaining_uses"`
}

func NewToolApprovalGrant() *ToolApprovalGrant {
	return &ToolApprovalGrant{
		CreatedAtUtc:  time.Now().UTC(),
		RemainingUses: 1,
	}
}

// ==========================================
// ApprovalGrantListResponse
// ==========================================

type ApprovalGrantListResponse struct {
	Items []ToolApprovalGrant `json:"items"`
}

func NewApprovalGrantListResponse() *ApprovalGrantListResponse {
	return &ApprovalGrantListResponse{
		Items: make([]ToolApprovalGrant, 0),
	}
}

// ==========================================
// OperatorAuditQuery
// ==========================================

type OperatorAuditQuery struct {
	Limit      int        `json:"limit"`
	ActorId    *string    `json:"actor_id,omitempty"`
	ActionType *string    `json:"action_type,omitempty"`
	TargetId   *string    `json:"target_id,omitempty"`
	FromUtc    *time.Time `json:"from_utc,omitempty"`
	ToUtc      *time.Time `json:"to_utc,omitempty"`
}

func NewOperatorAuditQuery() *OperatorAuditQuery {
	return &OperatorAuditQuery{
		Limit: 100,
	}
}

// ==========================================
// OperatorAuditEntry
// ==========================================

type OperatorAuditEntry struct {
	Id                string    `json:"id"`
	Sequence          int64     `json:"sequence"`
	TimestampUtc      time.Time `json:"timestamp_utc"`
	ActorId           string    `json:"actor_id"`
	ActorRole         string    `json:"actor_role"`
	ActorDisplayName  *string   `json:"actor_display_name,omitempty"`
	AuthMode          string    `json:"auth_mode"`
	ActionType        string    `json:"action_type"`
	TargetId          string    `json:"target_id"`
	Summary           string    `json:"summary"`
	PreviousEntryHash *string   `json:"previous_entry_hash,omitempty"`
	EntryHash         *string   `json:"entry_hash,omitempty"`
	Before            *string   `json:"before,omitempty"`
	After             *string   `json:"after,omitempty"`
	Success           bool      `json:"success"`
}

func NewOperatorAuditEntry() *OperatorAuditEntry {
	return &OperatorAuditEntry{
		TimestampUtc: time.Now().UTC(),
		ActorRole:    "viewer",
	}
}

// ==========================================
// OperatorAuditListResponse
// ==========================================

type OperatorAuditListResponse struct {
	Items []OperatorAuditEntry `json:"items"`
}

func NewOperatorAuditListResponse() *OperatorAuditListResponse {
	return &OperatorAuditListResponse{
		Items: make([]OperatorAuditEntry, 0),
	}
}

// ==========================================
// MemoryNoteItem
// ==========================================

type MemoryNoteItem struct {
	Key          string    `json:"key"`
	DisplayKey   string    `json:"display_key"`
	MemoryClass  string    `json:"memory_class"`
	ProjectId    *string   `json:"project_id,omitempty"`
	Preview      string    `json:"preview"`
	Content      *string   `json:"content,omitempty"`
	UpdatedAtUtc time.Time `json:"updated_at_utc"`
}

func NewMemoryNoteItem() *MemoryNoteItem {
	return &MemoryNoteItem{
		MemoryClass:  MemoryNoteClassGeneral,
		Preview:      "",
		UpdatedAtUtc: time.Now().UTC(),
	}
}

// ==========================================
// MemoryNoteListResponse
// ==========================================

type MemoryNoteListResponse struct {
	Prefix      *string          `json:"prefix,omitempty"`
	Query       *string          `json:"query,omitempty"`
	MemoryClass *string          `json:"memory_class,omitempty"`
	ProjectId   *string          `json:"project_id,omitempty"`
	Items       []MemoryNoteItem `json:"items"`
}

func NewMemoryNoteListResponse() *MemoryNoteListResponse {
	return &MemoryNoteListResponse{
		Items: make([]MemoryNoteItem, 0),
	}
}

// ==========================================
// MemoryNoteDetailResponse
// ==========================================

type MemoryNoteDetailResponse struct {
	Note *MemoryNoteItem `json:"note,omitempty"`
}

// ==========================================
// MemoryNoteUpsertRequest
// ==========================================

type MemoryNoteUpsertRequest struct {
	Key         *string `json:"key,omitempty"`
	MemoryClass *string `json:"memory_class,omitempty"`
	ProjectId   *string `json:"project_id,omitempty"`
	Content     string  `json:"content"`
}

// ==========================================
// MemoryConsoleExportBundle
// ==========================================

type MemoryConsoleExportBundle struct {
	ExportedAtUtc time.Time              `json:"exported_at_utc"`
	Notes         []MemoryNoteItem       `json:"notes"`
	Profiles      []UserProfile          `json:"profiles"`
	Proposals     []LearningProposal     `json:"proposals"`
	Automations   []AutomationDefinition `json:"automations"`
}

func NewMemoryConsoleExportBundle() *MemoryConsoleExportBundle {
	return &MemoryConsoleExportBundle{
		ExportedAtUtc: time.Now().UTC(),
		Notes:         make([]MemoryNoteItem, 0),
		Profiles:      make([]UserProfile, 0),
		Proposals:     make([]LearningProposal, 0),
		Automations:   make([]AutomationDefinition, 0),
	}
}

// ==========================================
// MemoryConsoleImportResponse
// ==========================================

type MemoryConsoleImportResponse struct {
	Success             bool   `json:"success"`
	NotesImported       int    `json:"notes_imported"`
	ProfilesImported    int    `json:"profiles_imported"`
	ProposalsImported   int    `json:"proposals_imported"`
	AutomationsImported int    `json:"automations_imported"`
	Message             string `json:"message"`
}

func NewMemoryConsoleImportResponse() *MemoryConsoleImportResponse {
	return &MemoryConsoleImportResponse{
		Message: "",
	}
}

// ==========================================
// ManagedSkillBundleItem
// ==========================================

type ManagedSkillBundleItem struct {
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	Description  string     `json:"description"`
	Content      string     `json:"content"`
	RootPath     string     `json:"root_path"`
	UpdatedAtUtc *time.Time `json:"updated_at_utc,omitempty"`
}

func NewManagedSkillBundleItem() *ManagedSkillBundleItem {
	return &ManagedSkillBundleItem{
		Description: "",
		RootPath:    "",
	}
}

// ==========================================
// AgentBundleExportBundle
// ==========================================

type AgentBundleExportBundle struct {
	Format           string                   `json:"format"`
	Version          int                      `json:"version"`
	ExportedAtUtc    time.Time                `json:"exported_at_utc"`
	Settings         *AdminSettingsSnapshot   `json:"settings,omitempty"`
	Notes            []MemoryNoteItem         `json:"notes"`
	Profiles         []UserProfile            `json:"profiles"`
	Proposals        []LearningProposal       `json:"proposals"`
	Automations      []AutomationDefinition   `json:"automations"`
	ProviderPolicies []ProviderPolicyRule     `json:"provider_policies"`
	ManagedSkills    []ManagedSkillBundleItem `json:"managed_skills"`
}

func NewAgentBundleExportBundle() *AgentBundleExportBundle {
	return &AgentBundleExportBundle{
		Format:           "openclaw-agent-bundle",
		Version:          1,
		ExportedAtUtc:    time.Now().UTC(),
		Notes:            make([]MemoryNoteItem, 0),
		Profiles:         make([]UserProfile, 0),
		Proposals:        make([]LearningProposal, 0),
		Automations:      make([]AutomationDefinition, 0),
		ProviderPolicies: make([]ProviderPolicyRule, 0),
		ManagedSkills:    make([]ManagedSkillBundleItem, 0),
	}
}

type AgentBundleImportResponse struct {
	Success                  bool   `json:"success"`
	Version                  int    `json:"version"`
	SettingsImported         bool   `json:"settings_imported"`
	NotesImported            int    `json:"notes_imported"`
	ProfilesImported         int    `json:"profiles_imported"`
	ProposalsImported        int    `json:"proposals_imported"`
	AutomationsImported      int    `json:"automations_imported"`
	ProviderPoliciesImported int    `json:"provider_policies_imported"`
	ManagedSkillsImported    int    `json:"managed_skills_imported"`
	SkillsReloaded           bool   `json:"skills_reloaded"`
	Message                  string `json:"message"`
}

func NewDefaultAgentBundleImportResponse() *AgentBundleImportResponse {
	return &AgentBundleImportResponse{
		Message: "",
	}
}

// --- LearningProposalProvenance ---

type LearningProposalProvenance struct {
	ActorId             *string                   `json:"actor_id,omitempty"`
	SourceSessionIds    []string                  `json:"source_session_ids"`
	SourceTurnIds       []string                  `json:"source_turn_ids"`
	ToolNames           []string                  `json:"tool_names"`
	ToolSequence        []string                  `json:"tool_sequence"`
	ToolObservations    []LearningToolObservation `json:"tool_observations"`
	RepeatedCount       int                       `json:"repeated_count"`
	ProposalFingerprint *string                   `json:"proposal_fingerprint,omitempty"`
	CreatedReason       *string                   `json:"created_reason,omitempty"`
	Confidence          float32                   `json:"confidence"`
	CreatedAtUtc        time.Time                 `json:"created_at_utc"`
	UpdatedAtUtc        time.Time                 `json:"updated_at_utc"`
	ReviewedAtUtc       *time.Time                `json:"reviewed_at_utc,omitempty"`
}

func NewDefaultLearningProposalProvenance() *LearningProposalProvenance {
	return &LearningProposalProvenance{
		SourceSessionIds: []string{},
		SourceTurnIds:    []string{},
		ToolNames:        []string{},
		ToolSequence:     []string{},
		ToolObservations: []LearningToolObservation{},
	}
}

// --- ProfileDiffEntry ---

type ProfileDiffEntry struct {
	Path       string  `json:"path"`
	ChangeType string  `json:"change_type"`
	Before     *string `json:"before,omitempty"`
	After      *string `json:"after,omitempty"`
}

// --- LearningProposalDetailResponse ---

type LearningProposalDetailResponse struct {
	Proposal        *LearningProposal           `json:"proposal,omitempty"`
	BaselineProfile *UserProfile                `json:"baseline_profile,omitempty"`
	CurrentProfile  *UserProfile                `json:"current_profile,omitempty"`
	ProfileDiff     []ProfileDiffEntry          `json:"profile_diff"`
	Provenance      *LearningProposalProvenance `json:"provenance,omitempty"`
	CanRollback     bool                        `json:"can_rollback"`
}

func NewDefaultLearningProposalDetailResponse() *LearningProposalDetailResponse {
	return &LearningProposalDetailResponse{
		ProfileDiff: []ProfileDiffEntry{},
	}
}

// --- ProfileExportBundle ---

type ProfileExportBundle struct {
	ExportedAtUtc time.Time          `json:"exported_at_utc"`
	Profiles      []UserProfile      `json:"profiles"`
	Proposals     []LearningProposal `json:"proposals"`
}

func NewDefaultProfileExportBundle() *ProfileExportBundle {
	return &ProfileExportBundle{
		ExportedAtUtc: time.Now().UTC(),
		Profiles:      []UserProfile{},
		Proposals:     []LearningProposal{},
	}
}

// --- ProfileImportResponse ---

type ProfileImportResponse struct {
	Success           bool   `json:"success"`
	ProfilesImported  int    `json:"profiles_imported"`
	ProposalsImported int    `json:"proposals_imported"`
	Message           string `json:"message"`
}

// --- SessionMetadataSnapshot ---

type SessionMetadataSnapshot struct {
	SessionId      string            `json:"session_id"`
	Starred        bool              `json:"starred"`
	Tags           []string          `json:"tags"`
	ActivePresetId *string           `json:"active_preset_id,omitempty"`
	TodoItems      []SessionTodoItem `json:"todo_items"`
}

func NewDefaultSessionMetadataSnapshot() *SessionMetadataSnapshot {
	return &SessionMetadataSnapshot{
		Tags:      []string{},
		TodoItems: []SessionTodoItem{},
	}
}

// --- SessionMetadataUpdateRequest ---

type SessionMetadataUpdateRequest struct {
	Starred        *bool             `json:"starred,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	ActivePresetId *string           `json:"active_preset_id,omitempty"`
	TodoItems      []SessionTodoItem `json:"todo_items,omitempty"`
}

// --- SessionPromotionTarget (Constants) ---

const (
	SessionPromotionTargetAutomation     = "automation"
	SessionPromotionTargetProviderPolicy = "provider_policy"
	SessionPromotionTargetSkillDraft     = "skill_draft"
)

// --- SessionPromotionRequest ---

type SessionPromotionRequest struct {
	Target              string   `json:"target"`
	Name                *string  `json:"name,omitempty"`
	Prompt              *string  `json:"prompt,omitempty"`
	Schedule            *string  `json:"schedule,omitempty"`
	DeliveryChannelId   *string  `json:"delivery_channel_id,omitempty"`
	DeliveryRecipientId *string  `json:"delivery_recipient_id,omitempty"`
	DeliverySubject     *string  `json:"delivery_subject,omitempty"`
	Tags                []string `json:"tags"`
	Scope               string   `json:"scope"`
	ProviderId          *string  `json:"provider_id,omitempty"`
	ModelId             *string  `json:"model_id,omitempty"`
	FallbackModels      []string `json:"fallback_models"`
	Priority            int      `json:"priority"`
	Enabled             bool     `json:"enabled"`
	Summary             *string  `json:"summary,omitempty"`
}

func NewDefaultSessionPromotionRequest() *SessionPromotionRequest {
	return &SessionPromotionRequest{
		Target:         SessionPromotionTargetAutomation,
		Tags:           []string{},
		Scope:          "session",
		FallbackModels: []string{},
		Priority:       100,
		Enabled:        true,
	}
}

// --- SessionPromotionResponse ---

type SessionPromotionResponse struct {
	Success        bool                  `json:"success"`
	Target         string                `json:"target"`
	Message        string                `json:"message"`
	CreatedId      *string               `json:"created_id,omitempty"`
	Automation     *AutomationDefinition `json:"automation,omitempty"`
	ProviderPolicy *ProviderPolicyRule   `json:"provider_policy,omitempty"`
	Proposal       *LearningProposal     `json:"proposal,omitempty"`
	Error          *string               `json:"error,omitempty"`
}

// --- SessionTodoItem ---

type SessionTodoItem struct {
	Id           string    `json:"id"`
	Text         string    `json:"text"`
	Completed    bool      `json:"completed"`
	Notes        *string   `json:"notes,omitempty"`
	CreatedAtUtc time.Time `json:"created_at_utc"`
	UpdatedAtUtc time.Time `json:"updated_at_utc"`
}

func NewDefaultSessionTodoItem() *SessionTodoItem {
	now := time.Now().UTC()
	return &SessionTodoItem{
		Text:         "",
		CreatedAtUtc: now,
		UpdatedAtUtc: now,
	}
}

// --- SessionDiffResponse ---

type SessionDiffResponse struct {
	SessionId                string                   `json:"session_id"`
	BranchId                 string                   `json:"branch_id"`
	BranchName               *string                  `json:"branch_name,omitempty"`
	SharedPrefixTurns        int                      `json:"shared_prefix_turns"`
	CurrentTurnCount         int                      `json:"current_turn_count"`
	BranchTurnCount          int                      `json:"branch_turn_count"`
	CurrentOnlyTurnSummaries []string                 `json:"current_only_turn_summaries"`
	BranchOnlyTurnSummaries  []string                 `json:"branch_only_turn_summaries"`
	Metadata                 *SessionMetadataSnapshot `json:"metadata,omitempty"`
}

func NewDefaultSessionDiffResponse() *SessionDiffResponse {
	return &SessionDiffResponse{
		CurrentOnlyTurnSummaries: []string{},
		BranchOnlyTurnSummaries:  []string{},
	}
}

// --- SessionTimelineResponse ---

type SessionTimelineResponse struct {
	SessionId     string                   `json:"session_id"`
	Events        []RuntimeEventEntry      `json:"events"`
	ProviderTurns []ProviderTurnUsageEntry `json:"provider_turns"`
}

func NewDefaultSessionTimelineResponse() *SessionTimelineResponse {
	return &SessionTimelineResponse{
		Events:        []RuntimeEventEntry{},
		ProviderTurns: []ProviderTurnUsageEntry{},
	}
}

// --- SessionExportItem ---

type SessionExportItem struct {
	Session  Session                  `json:"session"`
	Metadata *SessionMetadataSnapshot `json:"metadata,omitempty"`
}

// --- SessionExportResponse ---

type SessionExportResponse struct {
	Filters SessionListQuery    `json:"filters"`
	Items   []SessionExportItem `json:"items"`
}

func NewDefaultSessionExportResponse() *SessionExportResponse {
	return &SessionExportResponse{
		Items: []SessionExportItem{},
	}
}

const (
	SecurityPostureResponseDefaultAutonomyMode          = "full"
	SecurityPostureResponseDefaultPluginBridgeTransport = "stdio"
	SecurityPostureResponseDefaultPluginBridgeSecurity  = "legacy"
	ApprovalSimulationResponseDefaultAutonomyMode       = "full"
)

type WebhookDeadLetterEntry struct {
	Id             string     `json:"id"`
	Source         string     `json:"source"`
	DeliveryKey    string     `json:"delivery_key"`
	EndpointName   *string    `json:"endpoint_name"`
	ChannelId      *string    `json:"channel_id"`
	SenderId       *string    `json:"sender_id"`
	SessionId      *string    `json:"session_id"`
	CreatedAtUtc   time.Time  `json:"created_at_utc"`
	Error          string     `json:"error"`
	PayloadPreview string     `json:"payload_preview"`
	Discarded      bool       `json:"discarded"`
	ReplayedAtUtc  *time.Time `json:"replayed_at_utc"`
}

func DefaultWebhookDeadLetterEntry() WebhookDeadLetterEntry {
	return WebhookDeadLetterEntry{
		CreatedAtUtc: time.Now().UTC(),
	}
}

type WebhookDeadLetterRecord struct {
	Entry         WebhookDeadLetterEntry `json:"entry"`
	ReplayMessage *InboundMessage        `json:"replay_message"`
}

type WebhookDeadLetterResponse struct {
	Items []WebhookDeadLetterEntry `json:"items"`
}

func DefaultWebhookDeadLetterResponse() WebhookDeadLetterResponse {
	return WebhookDeadLetterResponse{
		Items: make([]WebhookDeadLetterEntry, 0),
	}
}

type ActorRateLimitPolicy struct {
	Id                     string    `json:"id"`
	ActorType              string    `json:"actor_type"`
	EndpointScope          string    `json:"endpoint_scope"`
	MatchValue             *string   `json:"match_value"`
	BurstLimit             int       `json:"burst_limit"`
	BurstWindowSeconds     int       `json:"burst_window_seconds"`
	SustainedLimit         int       `json:"sustained_limit"`
	SustainedWindowSeconds int       `json:"sustained_window_seconds"`
	Enabled                bool      `json:"enabled"`
	CreatedAtUtc           time.Time `json:"created_at_utc"`
}

func DefaultActorRateLimitPolicy() ActorRateLimitPolicy {
	return ActorRateLimitPolicy{
		Enabled:      true,
		CreatedAtUtc: time.Now().UTC(),
	}
}

type ActorRateLimitStatus struct {
	ActorType                   string    `json:"actor_type"`
	EndpointScope               string    `json:"endpoint_scope"`
	ActorKey                    string    `json:"actor_key"`
	BurstCount                  int       `json:"burst_count"`
	SustainedCount              int       `json:"sustained_count"`
	BurstWindowStartedAtUtc     time.Time `json:"burst_window_started_at_utc"`
	SustainedWindowStartedAtUtc time.Time `json:"sustained_window_started_at_utc"`
}

type ActorRateLimitResponse struct {
	Policies []ActorRateLimitPolicy `json:"policies"`
	Active   []ActorRateLimitStatus `json:"active"`
}

func DefaultActorRateLimitResponse() ActorRateLimitResponse {
	return ActorRateLimitResponse{
		Policies: make([]ActorRateLimitPolicy, 0),
		Active:   make([]ActorRateLimitStatus, 0),
	}
}

type SecurityPostureResponse struct {
	PublicBind                               bool     `json:"public_bind"`
	AuthTokenConfigured                      bool     `json:"auth_token_configured"`
	BrowserSessionCookieSecureEffective      bool     `json:"browser_session_cookie_secure_effective"`
	BrowserSessionsEnabled                   bool     `json:"browser_sessions_enabled"`
	BrowserToolConfigured                    bool     `json:"browser_tool_configured"`
	BrowserToolRegistered                    bool     `json:"browser_tool_registered"`
	BrowserLocalExecutionSupported           bool     `json:"browser_local_execution_supported"`
	BrowserExecutionBackendConfigured        bool     `json:"browser_execution_backend_configured"`
	TrustForwardedHeaders                    bool     `json:"trust_forwarded_headers"`
	RequireRequesterMatchForHttpToolApproval bool     `json:"require_requester_match_for_http_tool_approval"`
	ToolApprovalRequired                     bool     `json:"tool_approval_required"`
	AutonomyMode                             string   `json:"autonomy_mode"`
	PluginBridgeEnabled                      bool     `json:"plugin_bridge_enabled"`
	PluginBridgeTransportMode                string   `json:"plugin_bridge_transport_mode"`
	PluginBridgeSecurityMode                 string   `json:"plugin_bridge_security_mode"`
	ProcessToolSafeForPublicBind             bool     `json:"process_tool_safe_for_public_bind"`
	StableSessionsScopedByRequester          bool     `json:"stable_sessions_scoped_by_requester"`
	SignedWebhookValidationReady             bool     `json:"signed_webhook_validation_ready"`
	SandboxConfigured                        bool     `json:"sandbox_configured"`
	AllowsRawSecretRefsOnPublicBind          bool     `json:"allows_raw_secret_refs_on_public_bind"`
	RiskFlags                                []string `json:"risk_flags"`
	Recommendations                          []string `json:"recommendations"`
}

func DefaultSecurityPostureResponse() SecurityPostureResponse {
	return SecurityPostureResponse{
		AutonomyMode:              SecurityPostureResponseDefaultAutonomyMode,
		PluginBridgeTransportMode: SecurityPostureResponseDefaultPluginBridgeTransport,
		PluginBridgeSecurityMode:  SecurityPostureResponseDefaultPluginBridgeSecurity,
		RiskFlags:                 make([]string, 0),
		Recommendations:           make([]string, 0),
	}
}

type ApprovalSimulationRequest struct {
	ToolName              *string  `json:"tool_name"`
	ArgumentsJson         *string  `json:"arguments_json"`
	ChannelId             *string  `json:"channel_id"`
	SenderId              *string  `json:"sender_id"`
	SessionId             *string  `json:"session_id"`
	AutonomyMode          *string  `json:"autonomy_mode"`
	RequireToolApproval   *bool    `json:"require_tool_approval"`
	ApprovalRequiredTools []string `json:"approval_required_tools"`
}

type ApprovalSimulationResponse struct {
	Decision                  string   `json:"decision"`
	Reason                    string   `json:"reason"`
	ToolName                  string   `json:"tool_name"`
	AutonomyMode              string   `json:"autonomy_mode"`
	AutonomyAllowed           bool     `json:"autonomy_allowed"`
	RequireToolApproval       bool     `json:"require_tool_approval"`
	ApprovalRequired          bool     `json:"approval_required"`
	BlockingPolicy            *string  `json:"blocking_policy"`
	ExecutionBackend          *string  `json:"execution_backend"`
	ExecutionFallbackBackend  *string  `json:"execution_fallback_backend"`
	ExecutionTemplate         *string  `json:"execution_template"`
	ExecutionSandboxMode      *string  `json:"execution_sandbox_mode"`
	ExecutionRequireWorkspace *bool    `json:"execution_require_workspace"`
	ApprovalRequiredTools     []string `json:"approval_required_tools"`
}

func DefaultApprovalSimulationResponse() ApprovalSimulationResponse {
	return ApprovalSimulationResponse{
		ToolName:              "",
		AutonomyMode:          ApprovalSimulationResponseDefaultAutonomyMode,
		ApprovalRequiredTools: make([]string, 0),
	}
}

type IncidentBundleResponse struct {
	GeneratedAtUtc     time.Time                     `json:"generated_at_utc"`
	Posture            SecurityPostureResponse       `json:"posture"`
	Metrics            MetricsSnapshot               `json:"metrics"`
	Retention          RetentionRunStatus            `json:"retention"`
	ApprovalHistory    []ApprovalHistoryEntry        `json:"approval_history"`
	ProviderPolicies   []ProviderPolicyRule          `json:"provider_policies"`
	ProviderRoutes     []ProviderRouteHealthSnapshot `json:"provider_routes"`
	ProviderUsage      []ProviderUsageSnapshot       `json:"provider_usage"`
	RuntimeEvents      []RuntimeEventEntry           `json:"runtime_events"`
	WebhookDeadLetters []WebhookDeadLetterEntry      `json:"webhook_dead_letters"`
	PluginHealth       []PluginHealthSnapshot        `json:"plugin_health"`
}

func DefaultIncidentBundleResponse() IncidentBundleResponse {
	return IncidentBundleResponse{
		GeneratedAtUtc:     time.Now().UTC(),
		Posture:            DefaultSecurityPostureResponse(),
		ApprovalHistory:    make([]ApprovalHistoryEntry, 0),
		ProviderPolicies:   make([]ProviderPolicyRule, 0),
		ProviderRoutes:     make([]ProviderRouteHealthSnapshot, 0),
		ProviderUsage:      make([]ProviderUsageSnapshot, 0),
		RuntimeEvents:      make([]RuntimeEventEntry, 0),
		WebhookDeadLetters: make([]WebhookDeadLetterEntry, 0),
		PluginHealth:       make([]PluginHealthSnapshot, 0),
	}
}
