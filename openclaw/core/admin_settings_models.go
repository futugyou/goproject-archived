package core

import "time"

type AdminSettingsSnapshot struct {
	UsageFooter                          string                         `json:"usage_footer"`
	MaxConcurrentSessions                int                            `json:"max_concurrent_sessions"`
	SessionTimeoutMinutes                int                            `json:"session_timeout_minutes"`
	SessionTokenBudget                   int64                          `json:"session_token_budget"`
	SessionRateLimitPerMinute            int                            `json:"session_rate_limit_per_minute"`
	AllowQueryStringToken                bool                           `json:"allow_query_string_token"`
	BrowserSessionIdleMinutes            int                            `json:"browser_session_idle_minutes"`
	BrowserRememberDays                  int                            `json:"browser_remember_days"`
	AutonomyMode                         string                         `json:"autonomy_mode"`
	RequireToolApproval                  bool                           `json:"require_tool_approval"`
	ToolApprovalTimeoutSeconds           int                            `json:"tool_approval_timeout_seconds"`
	ParallelToolExecution                bool                           `json:"parallel_tool_execution"`
	AllowShell                           bool                           `json:"allow_shell"`
	ReadOnlyMode                         string                         `json:"read_only_mode"`
	EnableBrowserTool                    bool                           `json:"enable_browser_tool"`
	AllowBrowserEvaluate                 bool                           `json:"allow_browser_evaluate"`
	MaxHistoryTurns                      int                            `json:"max_history_turns"`
	EnableCompaction                     bool                           `json:"enable_compaction"`
	CompactionThreshold                  int                            `json:"compaction_threshold"`
	CompactionKeepRecent                 int                            `json:"compaction_keep_recent"`
	RetentionEnabled                     bool                           `json:"retention_enabled"`
	RetentionRunOnStartup                bool                           `json:"retention_run_on_startup"`
	RetentionSweepIntervalMinutes        int                            `json:"retention_sweep_interval_minutes"`
	RetentionSessionTtlDays              int                            `json:"retention_session_ttl_days"`
	RetentionBranchTtlDays               int                            `json:"retention_branch_ttl_days"`
	RetentionArchiveEnabled              bool                           `json:"retention_archive_enabled"`
	RetentionArchiveRetentionDays        int                            `json:"retention_archive_retention_days"`
	RetentionMaxItemsPerSweep            int                            `json:"retention_max_items_per_sweep"`
	AllowlistSemantics                   string                         `json:"allowlist_semantics"`
	SmsEnabled                           bool                           `json:"sms_enabled"`
	SmsValidateSignature                 bool                           `json:"sms_validate_signature"`
	SmsDmPolicy                          string                         `json:"sms_dm_policy"`
	TelegramEnabled                      bool                           `json:"telegram_enabled"`
	TelegramValidateSignature            bool                           `json:"telegram_validate_signature"`
	TelegramDmPolicy                     string                         `json:"telegram_dm_policy"`
	TeamsEnabled                         bool                           `json:"teams_enabled"`
	TeamsValidateToken                   bool                           `json:"teams_validate_token"`
	TeamsDmPolicy                        string                         `json:"teams_dm_policy"`
	SlackEnabled                         bool                           `json:"slack_enabled"`
	SlackValidateSignature               bool                           `json:"slack_validate_signature"`
	SlackDmPolicy                        string                         `json:"slack_dm_policy"`
	DiscordEnabled                       bool                           `json:"discord_enabled"`
	DiscordValidateSignature             bool                           `json:"discord_validate_signature"`
	DiscordDmPolicy                      string                         `json:"discord_dm_policy"`
	SignalEnabled                        bool                           `json:"signal_enabled"`
	SignalDmPolicy                       string                         `json:"signal_dm_policy"`
	WhatsAppEnabled                      bool                           `json:"whatsapp_enabled"`
	WhatsAppValidateSignature            bool                           `json:"whatsapp_validate_signature"`
	WhatsAppDmPolicy                     string                         `json:"whatsapp_dm_policy"`
	WhatsAppType                         string                         `json:"whatsapp_type"`
	WhatsAppWebhookPath                  string                         `json:"whatsapp_webhook_path"`
	WhatsAppWebhookPublicBaseUrl         string                         `json:"whatsapp_webhook_public_base_url,omitempty"`
	WhatsAppWebhookVerifyToken           string                         `json:"whatsapp_webhook_verify_token"`
	WhatsAppWebhookVerifyTokenRef        string                         `json:"whatsapp_webhook_verify_token_ref"`
	WhatsAppWebhookAppSecret             string                         `json:"whatsapp_webhook_app_secret,omitempty"`
	WhatsAppWebhookAppSecretRef          string                         `json:"whatsapp_webhook_app_secret_ref"`
	WhatsAppCloudApiToken                string                         `json:"whatsapp_cloud_api_token,omitempty"`
	WhatsAppCloudApiTokenRef             string                         `json:"whatsapp_cloud_api_token_ref"`
	WhatsAppPhoneNumberId                string                         `json:"whatsapp_phone_number_id,omitempty"`
	WhatsAppBusinessAccountId            string                         `json:"whatsapp_business_account_id,omitempty"`
	WhatsAppBridgeUrl                    string                         `json:"whatsapp_bridge_url,omitempty"`
	WhatsAppBridgeToken                  string                         `json:"whatsapp_bridge_token,omitempty"`
	WhatsAppBridgeTokenRef               string                         `json:"whatsapp_bridge_token_ref"`
	WhatsAppBridgeSuppressSendExceptions bool                           `json:"whatsapp_bridge_suppress_send_exceptions"`
	WhatsAppFirstPartyWorker             WhatsAppFirstPartyWorkerConfig `json:"whatsapp_first_party_worker"`
}

func DefaultAdminSettingsSnapshot() *AdminSettingsSnapshot {
	return &AdminSettingsSnapshot{
		UsageFooter:                   "off",
		AutonomyMode:                  "supervised",
		AllowlistSemantics:            "legacy",
		SmsDmPolicy:                   "pairing",
		TelegramDmPolicy:              "pairing",
		TeamsDmPolicy:                 "pairing",
		SlackDmPolicy:                 "pairing",
		DiscordDmPolicy:               "pairing",
		SignalDmPolicy:                "pairing",
		WhatsAppDmPolicy:              "pairing",
		WhatsAppType:                  "official",
		WhatsAppWebhookPath:           "/whatsapp/inbound",
		WhatsAppWebhookVerifyToken:    "openclaw-verify",
		WhatsAppWebhookVerifyTokenRef: "env:WHATSAPP_VERIFY_TOKEN",
		WhatsAppWebhookAppSecretRef:   "env:WHATSAPP_APP_SECRET",
		WhatsAppCloudApiTokenRef:      "env:WHATSAPP_CLOUD_API_TOKEN",
		WhatsAppBridgeTokenRef:        "env:WHATSAPP_BRIDGE_TOKEN",
		WhatsAppFirstPartyWorker:      WhatsAppFirstPartyWorkerConfig{},
	}
}

type AdminSettingsPersistenceInfo struct {
	Path              string     `json:"path"`
	Exists            bool       `json:"exists"`
	LastModifiedAtUtc *time.Time `json:"last_modified_at_utc,omitempty"`
}
