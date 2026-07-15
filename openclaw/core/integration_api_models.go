package core

type IntegrationStatusResponse struct {
	Health               HealthResponse  `json:"health"`
	Metrics              MetricsSnapshot `json:"metrics"`
	ActiveSessions       int             `json:"active_sessions"`
	PendingApprovals     int             `json:"pending_approvals"`
	ActiveApprovalGrants int             `json:"active_approval_grants"`
}

type IntegrationDashboardResponse struct {
	Status          IntegrationStatusResponse          `json:"status"`
	Approvals       IntegrationApprovalsResponse       `json:"approvals"`
	ApprovalHistory IntegrationApprovalHistoryResponse `json:"approval_history"`
	Providers       IntegrationProvidersResponse       `json:"providers"`
	Plugins         IntegrationPluginsResponse         `json:"plugins"`
	Events          IntegrationRuntimeEventsResponse   `json:"events"`
	Operator        OperatorDashboardSnapshot          `json:"operator"`
}

// ==========================================
// Sessions
// ==========================================

type IntegrationSessionsResponse struct {
	Filters   SessionListQuery `json:"filters"`
	Active    []SessionSummary `json:"active"`
	Persisted PagedSessionList `json:"persisted"`
}

func DefaultIntegrationSessionsResponse() IntegrationSessionsResponse {
	return IntegrationSessionsResponse{
		Active: []SessionSummary{},
	}
}

type IntegrationSessionDetailResponse struct {
	Session     *Session                 `json:"session"`
	IsActive    bool                     `json:"is_active"`
	BranchCount int                      `json:"branch_count"`
	Metadata    *SessionMetadataSnapshot `json:"metadata"`
}

type IntegrationSessionTimelineResponse struct {
	SessionId     string                   `json:"session_id"`
	Events        []RuntimeEventEntry      `json:"events"`
	ProviderTurns []ProviderTurnUsageEntry `json:"provider_turns"`
}

func DefaultIntegrationSessionTimelineResponse() IntegrationSessionTimelineResponse {
	return IntegrationSessionTimelineResponse{
		Events:        []RuntimeEventEntry{},
		ProviderTurns: []ProviderTurnUsageEntry{},
	}
}

type IntegrationSessionSearchResponse struct {
	Result SessionSearchResult `json:"result"`
}

// ==========================================
// Messaging
// ==========================================

type IntegrationMessageRequest struct {
	ChannelId        string `json:"channel_id"`
	SenderId         string `json:"sender_id"`
	SessionId        string `json:"session_id"`
	Text             string `json:"text"`
	MessageId        string `json:"message_id"`
	ReplyToMessageId string `json:"reply_to_message_id"`
}

type IntegrationMessageResponse struct {
	Accepted  bool   `json:"accepted"`
	ChannelId string `json:"channel_id"`
	SenderId  string `json:"sender_id"`
	SessionId string `json:"session_id"`
	MessageId string `json:"message_id"`
}

// ==========================================
// Profiles
// ==========================================

type IntegrationProfileUpdateRequest struct {
	Profile UserProfile `json:"profile"`
}

type IntegrationProfilesResponse struct {
	Items []UserProfile `json:"items"`
}

func DefaultIntegrationProfilesResponse() IntegrationProfilesResponse {
	return IntegrationProfilesResponse{
		Items: []UserProfile{},
	}
}

type IntegrationProfileResponse struct {
	Profile *UserProfile `json:"profile"`
}

// ==========================================
// Audio / TextToSpeech
// ==========================================

type IntegrationTextToSpeechRequest struct {
	Text      string `json:"text"`
	Provider  string `json:"provider"`
	VoiceName string `json:"voice_name"`
	VoiceId   string `json:"voice_id"`
	Model     string `json:"model"`
}

type IntegrationTextToSpeechResponse struct {
	Provider  string `json:"provider"`
	AssetId   string `json:"asset_id"`
	MediaType string `json:"media_type"`
	DataUrl   string `json:"data_url"`
	Marker    string `json:"marker"`
}

// ==========================================
// Approvals & History
// ==========================================

type LearningProposalReviewRequest struct {
	Reason string `json:"reason"`
}

type IntegrationApprovalsResponse struct {
	ChannelId string                `json:"channel_id"`
	SenderId  string                `json:"sender_id"`
	Items     []ToolApprovalRequest `json:"items"`
}

func DefaultIntegrationApprovalsResponse() IntegrationApprovalsResponse {
	return IntegrationApprovalsResponse{
		Items: []ToolApprovalRequest{},
	}
}

type IntegrationApprovalHistoryResponse struct {
	Query ApprovalHistoryQuery   `json:"query"`
	Items []ApprovalHistoryEntry `json:"items"`
}

func DefaultIntegrationApprovalHistoryResponse() IntegrationApprovalHistoryResponse {
	return IntegrationApprovalHistoryResponse{
		Items: []ApprovalHistoryEntry{},
	}
}

// ==========================================
// Events & Runtime
// ==========================================

type IntegrationRuntimeEventsResponse struct {
	Query RuntimeEventQuery   `json:"query"`
	Items []RuntimeEventEntry `json:"items"`
}

func DefaultIntegrationRuntimeEventsResponse() IntegrationRuntimeEventsResponse {
	return IntegrationRuntimeEventsResponse{
		Items: []RuntimeEventEntry{},
	}
}

type IntegrationOperatorAuditResponse struct {
	Query OperatorAuditQuery   `json:"query"`
	Items []OperatorAuditEntry `json:"items"`
}

func DefaultIntegrationOperatorAuditResponse() IntegrationOperatorAuditResponse {
	return IntegrationOperatorAuditResponse{
		Items: []OperatorAuditEntry{},
	}
}

// ==========================================
// Providers & Plugins
// ==========================================

type IntegrationProvidersResponse struct {
	ModelProfiles *ModelProfilesStatusResponse  `json:"model_profiles"`
	Routes        []ProviderRouteHealthSnapshot `json:"routes"`
	Usage         []ProviderUsageSnapshot       `json:"usage"`
	Policies      []ProviderPolicyRule          `json:"policies"`
	RecentTurns   []ProviderTurnUsageEntry      `json:"recent_turns"`
}

func DefaultIntegrationProvidersResponse() IntegrationProvidersResponse {
	return IntegrationProvidersResponse{
		Routes:      []ProviderRouteHealthSnapshot{},
		Usage:       []ProviderUsageSnapshot{},
		Policies:    []ProviderPolicyRule{},
		RecentTurns: []ProviderTurnUsageEntry{},
	}
}

type IntegrationPluginsResponse struct {
	Items []PluginHealthSnapshot `json:"items"`
}

func DefaultIntegrationPluginsResponse() IntegrationPluginsResponse {
	return IntegrationPluginsResponse{
		Items: []PluginHealthSnapshot{},
	}
}

// ==========================================
// Compatibility & Catalog
// ==========================================

type IntegrationCompatibilityCatalogResponse struct {
	Catalog CompatibilityCatalogResponse `json:"catalog"`
}

type IntegrationCompatibilityExportResponse struct {
	RequestedRuntimeMode string                       `json:"requested_runtime_mode"`
	EffectiveRuntimeMode string                       `json:"effective_runtime_mode"`
	DynamicCodeSupported bool                         `json:"dynamic_code_supported"`
	Posture              SecurityPostureResponse      `json:"posture"`
	Channels             []ChannelReadinessDto        `json:"channels"`
	Plugins              []PluginHealthSnapshot       `json:"plugins"`
	Catalog              CompatibilityCatalogResponse `json:"catalog"`
}

func DefaultIntegrationCompatibilityExportResponse() IntegrationCompatibilityExportResponse {
	return IntegrationCompatibilityExportResponse{
		Channels: []ChannelReadinessDto{},
		Plugins:  []PluginHealthSnapshot{},
	}
}

// ==========================================
// Automations
// ==========================================

type AutomationRunRequest struct {
	DryRun bool `json:"dry_run"`
}

type IntegrationAutomationsResponse struct {
	Items []AutomationDefinition `json:"items"`
}

func DefaultIntegrationAutomationsResponse() IntegrationAutomationsResponse {
	return IntegrationAutomationsResponse{
		Items: []AutomationDefinition{},
	}
}

type IntegrationAutomationDetailResponse struct {
	Automation *AutomationDefinition `json:"automation"`
	RunState   *AutomationRunState   `json:"run_state"`
}

type IntegrationAutomationRunsResponse struct {
	AutomationId string                `json:"automation_id"`
	RunState     *AutomationRunState   `json:"run_state"`
	Items        []AutomationRunRecord `json:"items"`
}

func DefaultIntegrationAutomationRunsResponse() IntegrationAutomationRunsResponse {
	return IntegrationAutomationRunsResponse{
		Items: []AutomationRunRecord{},
	}
}

type IntegrationAutomationRunDetailResponse struct {
	AutomationId string                `json:"automation_id"`
	Automation   *AutomationDefinition `json:"automation"`
	RunState     *AutomationRunState   `json:"run_state"`
	Run          *AutomationRunRecord  `json:"run"`
}

// ==========================================
// Learning & Tools
// ==========================================

type LearningProposalListResponse struct {
	Items []LearningProposal `json:"items"`
}

func DefaultLearningProposalListResponse() LearningProposalListResponse {
	return LearningProposalListResponse{
		Items: []LearningProposal{},
	}
}

type IntegrationToolPresetsResponse struct {
	Items []ResolvedToolPreset `json:"items"`
}

func DefaultIntegrationToolPresetsResponse() IntegrationToolPresetsResponse {
	return IntegrationToolPresetsResponse{
		Items: []ResolvedToolPreset{},
	}
}
