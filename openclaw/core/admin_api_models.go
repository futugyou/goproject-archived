package core

import "time"

// --- Auth ---

type AuthSessionRequest struct {
	Remember     bool    `json:"remember"`
	Username     *string `json:"username"`
	Password     *string `json:"password"`
	AccountToken *string `json:"accountToken"`
}

type OperatorTokenExchangeRequest struct {
	Username     *string    `json:"username"`
	Password     *string    `json:"password"`
	Label        *string    `json:"label"`
	ExpiresAtUtc *time.Time `json:"expiresAtUtc"`
}

type OperatorTokenExchangeResponse struct {
	AuthMode  string                       `json:"auth_mode"`
	Account   *OperatorAccountSummary      `json:"account"`
	TokenInfo *OperatorAccountTokenSummary `json:"tokenInfo"`
	Token     string                       `json:"token"`
}

func DefaultOperatorTokenExchangeResponse() OperatorTokenExchangeResponse {
	return OperatorTokenExchangeResponse{
		AuthMode: "account_token",
	}
}

type AuthSessionResponse struct {
	AuthMode                          string     `json:"auth_mode"`
	CsrfToken                         *string    `json:"csrf_token"`
	ExpiresAtUtc                      *time.Time `json:"expires_at_utc"`
	Persistent                        bool       `json:"persistent"`
	Role                              string     `json:"role"`
	AccountId                         *string    `json:"account_id"`
	Username                          *string    `json:"username"`
	DisplayName                       *string    `json:"display_name"`
	IsBootstrapAdmin                  bool       `json:"is_bootstrap_admin"`
	PublicBind                        bool       `json:"public_bind"`
	AllowedAuthModes                  []string   `json:"allowed_auth_modes"`
	EffectiveToolSurface              string     `json:"effective_tool_surface"`
	EffectiveToolPresetId             string     `json:"effective_tool_preset_id"`
	EffectiveToolPresetDescription    *string    `json:"effective_tool_preset_description"`
	BrowserToolRegistered             bool       `json:"browser_tool_registered"`
	BrowserExecutionBackendConfigured bool       `json:"browser_execution_backend_configured"`
	BrowserCapabilityReason           string     `json:"browser_capability_reason"`
	CapabilitySummary                 []string   `json:"capability_summary"`
}

func DefaultAuthSessionResponse() AuthSessionResponse {
	return AuthSessionResponse{
		Role:                  "viewer",
		EffectiveToolSurface:  "web",
		EffectiveToolPresetId: "web",
	}
}

// --- Approval ---

type ApprovalListResponse struct {
	Items []ToolApprovalRequest `json:"items"`
}

type ApprovalHistoryQuery struct {
	Limit     int        `json:"limit"`
	ChannelId *string    `json:"channel_id"`
	SenderId  *string    `json:"sender_id"`
	ToolName  *string    `json:"tool_name"`
	FromUtc   *time.Time `json:"from_utc"`
	ToUtc     *time.Time `json:"to_utc"`
}

func DefaultApprovalHistoryQuery() ApprovalHistoryQuery {
	return ApprovalHistoryQuery{
		Limit: 50,
	}
}

type ApprovalHistoryEntry struct {
	EventType        string     `json:"event_type"`
	ApprovalId       string     `json:"approval_id"`
	SessionId        string     `json:"session_id"`
	ChannelId        string     `json:"channel_id"`
	SenderId         string     `json:"sender_id"`
	ToolName         string     `json:"tool_name"`
	ArgumentsPreview string     `json:"arguments_preview"`
	Action           *string    `json:"action"`
	IsMutation       bool       `json:"is_mutation"`
	Summary          string     `json:"summary"`
	TimestampUtc     time.Time  `json:"timestamp_utc"`
	DecisionAtUtc    *time.Time `json:"decision_at_utc"`
	ActorChannelId   *string    `json:"actorChannel_id"`
	ActorSenderId    *string    `json:"actorSender_id"`
	ActorRole        *string    `json:"actor_role"`
	ActorDisplayName *string    `json:"actor_display_name"`
	DecisionSource   *string    `json:"decision_source"`
	Approved         *bool      `json:"approved"`
}

type ApprovalHistoryResponse struct {
	Items []ApprovalHistoryEntry `json:"items"`
}

type PairingApproveResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type PairingRevokeResponse struct {
	Success bool `json:"success"`
}

type AllowlistSnapshotResponse struct {
	ChannelId string                `json:"channel_id"`
	Semantics string                `json:"semantics"`
	Config    ChannelAllowlistFile  `json:"config"`
	Dynamic   *ChannelAllowlistFile `json:"dynamic"`
	Effective ChannelAllowlistFile  `json:"effective"`
}

type SenderMutationResponse struct {
	Success  bool    `json:"success"`
	Error    *string `json:"error"`
	SenderId *string `json:"sender_id"`
}

type CountMutationResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
	Count   int     `json:"count"`
}

type SkillsReloadResponse struct {
	Reloaded int      `json:"reloaded"`
	Skills   []string `json:"skills"`
}

type ChannelFixGuidanceDto struct {
	Label     string `json:"label"`
	Href      string `json:"href"`
	Reference string `json:"reference"`
}

type ChannelReadinessDto struct {
	ChannelId           string                  `json:"channel_id"`
	DisplayName         string                  `json:"display_name"`
	Mode                string                  `json:"mode"`
	Status              string                  `json:"status"`
	Enabled             bool                    `json:"enabled"`
	Ready               bool                    `json:"ready"`
	MissingRequirements []string                `json:"missing_requirements"`
	Warnings            []string                `json:"warnings"`
	FixGuidance         []ChannelFixGuidanceDto `json:"fix_guidance"`
}

type AdminSettingsResponse struct {
	Settings              AdminSettingsSnapshot        `json:"settings"`
	Persistence           AdminSettingsPersistenceInfo `json:"persistence"`
	Message               string                       `json:"message"`
	RestartRequired       bool                         `json:"restart_required"`
	RestartRequiredFields []string                     `json:"restart_required_fields"`
	ImmediateFieldKeys    []string                     `json:"immediate_field_keys"`
	RestartFieldKeys      []string                     `json:"restart_field_keys"`
	Warnings              []string                     `json:"warnings"`
	ChannelReadiness      []ChannelReadinessDto        `json:"channel_readiness"`
}

// --- Session ---

type SessionBranchListResponse struct {
	Items []SessionBranch `json:"items"`
}

type AdminSessionDetailResponse struct {
	Session     *Session                 `json:"session"`
	IsActive    bool                     `json:"is_active"`
	BranchCount int                      `json:"branch_count"`
	Metadata    *SessionMetadataSnapshot `json:"metadata"`
}

type AdminSessionsResponse struct {
	Filters   SessionListQuery `json:"filters"`
	Active    []SessionSummary `json:"active"`
	Persisted PagedSessionList `json:"persisted"`
}

// --- AdminSummary ---

type AdminSummaryResponse struct {
	Auth        AdminSummaryAuth          `json:"auth"`
	Runtime     AdminSummaryRuntime       `json:"runtime"`
	Settings    AdminSummarySettings      `json:"settings"`
	Channels    AdminSummaryChannels      `json:"channels"`
	Retention   AdminSummaryRetention     `json:"retention"`
	Plugins     AdminSummaryPlugins       `json:"plugins"`
	Usage       AdminSummaryUsage         `json:"usage"`
	Dashboard   OperatorDashboardSnapshot `json:"dashboard"`
	Reliability ReliabilitySnapshot       `json:"reliability"`
}

type AdminSummaryAuth struct {
	Mode                 string `json:"mode"`
	BrowserSessionActive bool   `json:"browser_session_active"`
}

type AdminSummaryRuntime struct {
	RequestedMode        string   `json:"requested_mode"`
	EffectiveMode        string   `json:"effective_mode"`
	Orchestrator         string   `json:"orchestrator"`
	DynamicCodeSupported bool     `json:"dynamic_code_supported"`
	ActiveSessions       int      `json:"active_sessions"`
	PendingApprovals     int      `json:"pending_approvals"`
	ActiveApprovalGrants int      `json:"active_approval_grants"`
	LiveSkillCount       int      `json:"live_skill_count"`
	LiveSkillNames       []string `json:"live_skill_names"`
}

type AdminSummarySettings struct {
	Persistence     AdminSettingsPersistenceInfo `json:"persistence"`
	OverridesActive bool                         `json:"overrides_active"`
	Warnings        []string                     `json:"warnings"`
}

type AdminSummaryChannels struct {
	AllowlistSemantics string                `json:"allowlist_semantics"`
	Readiness          []ChannelReadinessDto `json:"readiness"`
}

type AdminSummaryRetention struct {
	Enabled bool               `json:"enabled"`
	Status  RetentionRunStatus `json:"status"`
}

type AdminSummaryPlugins struct {
	Loaded        int                    `json:"loaded"`
	BlockedByMode int                    `json:"blocked_by_mode"`
	Reports       []PluginLoadReport     `json:"reports"`
	Health        []PluginHealthSnapshot `json:"health"`
}

type AdminSummaryUsage struct {
	Providers   []ProviderUsageSnapshot       `json:"providers"`
	Routes      []ProviderRouteHealthSnapshot `json:"routes"`
	RecentTurns []ProviderTurnUsageEntry      `json:"recent_turns"`
	Tools       []ToolUsageSnapshot           `json:"tools"`
}

// --- Retention ---

type RetentionStatusResponse struct {
	Retention MemoryRetentionConfig `json:"retention"`
	Status    RetentionRunStatus    `json:"status"`
}

type RetentionSweepResponse struct {
	Success bool                  `json:"success"`
	DryRun  bool                  `json:"dry_run"`
	Result  *RetentionSweepResult `json:"result"`
}

type RetentionSweepErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type BranchRestoreResponse struct {
	Success   bool   `json:"success"`
	SessionId string `json:"session_id"`
	BranchId  string `json:"branch_id"`
	TurnCount int    `json:"turn_count"`
}
