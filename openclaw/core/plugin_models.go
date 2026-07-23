package core

import (
	"encoding/json"
	"strings"
	"time"
)

type PluginManifest struct {
	ID           string           `json:"id"`
	Name         string           `json:"name,omitempty"`
	Description  string           `json:"description,omitempty"`
	Version      string           `json:"version,omitempty"`
	Kind         string           `json:"kind,omitempty"`
	Channels     []string         `json:"channels"`
	Providers    []string         `json:"providers"`
	Skills       []string         `json:"skills"`
	ConfigSchema *json.RawMessage `json:"config_schema,omitempty"`
	UIHints      *json.RawMessage `json:"ui_hints,omitempty"`
}

func NewDefaultPluginManifest() *PluginManifest {
	return &PluginManifest{
		Channels:  []string{},
		Providers: []string{},
		Skills:    []string{},
	}
}

// DiscoveredPlugin 代表在磁盘上扫描到的插件
type DiscoveredPlugin struct {
	Manifest  PluginManifest `json:"manifest"`
	RootPath  string         `json:"root_path"`
	EntryPath string         `json:"entry_path"`
}

// PluginEntryConfig 代表网关配置中针对单个插件的设置
type PluginEntryConfig struct {
	Enabled bool             `json:"enabled"`
	Config  *json.RawMessage `json:"config,omitempty"`
}

func NewDefaultPluginEntryConfig() *PluginEntryConfig {
	return &PluginEntryConfig{
		Enabled: true,
	}
}

// PluginsConfig 代表顶层插件系统的核心配置
type PluginsConfig struct {
	Enabled       bool                          `json:"enabled"`
	Prefer        string                        `json:"prefer"`
	Overrides     map[string]string             `json:"overrides"`
	Allow         []string                      `json:"allow"`
	Deny          []string                      `json:"deny"`
	Load          PluginLoadConfig              `json:"load"`
	Entries       map[string]*PluginEntryConfig `json:"entries"`
	Slots         map[string]string             `json:"slots"`
	Transport     BridgeTransportConfig         `json:"transport"`
	RuntimeBudget PluginBridgeBudgetConfig      `json:"runtime_budget"`
	Native        NativePluginsConfig           `json:"native"`
	Mcp           McpPluginsConfig              `json:"mcp"`
	DynamicNative NativeDynamicPluginsConfig    `json:"dynamic_native"`
}

func NewDefaultPluginsConfig() *PluginsConfig {
	return &PluginsConfig{
		Enabled:       true,
		Prefer:        "native",
		Overrides:     make(map[string]string),
		Allow:         []string{},
		Deny:          []string{},
		Entries:       make(map[string]*PluginEntryConfig),
		Slots:         make(map[string]string),
		RuntimeBudget: PluginBridgeBudgetConfig{},
		Native:        NativePluginsConfig{},
		Mcp:           McpPluginsConfig{},
	}
}

// PluginBridgeBudgetConfig 针对 Bridge 运行时的健康隔离预算
type PluginBridgeBudgetConfig struct {
	MaxRestartCount        int   `json:"max_restart_count"`
	MaxWorkingSetBytes     int64 `json:"max_working_set_bytes"`
	MaxCompatibilityErrors int   `json:"max_compatibility_errors"`
}

// McpPluginsConfig 暴露为 Native 道具的 MCP 服务总控
type McpPluginsConfig struct {
	Enabled bool                        `json:"enabled"`
	Servers map[string]*McpServerConfig `json:"servers"`
}

// McpServerConfig 单个 MCP 服务节点的详细配置
type McpServerConfig struct {
	Enabled               bool              `json:"enabled"`
	Name                  string            `json:"name,omitempty"`
	Transport             string            `json:"transport,omitempty"`
	Command               string            `json:"command,omitempty"`
	Arguments             []string          `json:"arguments"`
	WorkingDirectory      string            `json:"working_directory,omitempty"`
	Environment           map[string]string `json:"environment"`
	URL                   string            `json:"url,omitempty"`
	Headers               map[string]string `json:"headers"`
	ToolNamePrefix        string            `json:"tool_name_prefix,omitempty"`
	StartupTimeoutSeconds int               `json:"startup_timeout_seconds"`
	RequestTimeoutSeconds int               `json:"request_timeout_seconds"`
}

func NewDefaultMcpServerConfig() *McpServerConfig {
	return &McpServerConfig{
		Enabled:               true,
		Arguments:             []string{},
		Environment:           make(map[string]string),
		Headers:               make(map[string]string),
		StartupTimeoutSeconds: 15,
		RequestTimeoutSeconds: 60,
	}
}

func (config *McpServerConfig) NormalizeTransport() string {
	if strings.TrimSpace(config.Transport) == "" {
		if strings.TrimSpace(config.URL) == "" {
			return "stdio"
		}
		return "http"
	}

	transport := strings.TrimSpace(config.Transport)
	if strings.EqualFold(transport, "streamable-http") || strings.EqualFold(transport, "streamable_http") {
		return "http"
	}

	return strings.ToLower(transport)
}

// NativePluginsConfig 针对 OpenClaw 常用插件的副本配置
type NativePluginsConfig struct {
	WebSearch     WebSearchConfig     `json:"web_search"`
	WebFetch      WebFetchConfig      `json:"web_fetch"`
	GitTools      GitToolsConfig      `json:"git_tools"`
	CodeExec      CodeExecConfig      `json:"code_exec"`
	ImageGen      ImageGenConfig      `json:"image_gen"`
	PdfRead       PdfReadConfig       `json:"pdf_read"`
	ImageAnalyze  ImageAnalyzeConfig  `json:"image_analyze"`
	MinerUPdf     MinerUPdfConfig     `json:"miner_updf"`
	Calendar      CalendarConfig      `json:"calendar"`
	Email         EmailConfig         `json:"email"`
	Database      DatabaseConfig      `json:"database"`
	InboxZero     InboxZeroConfig     `json:"inbox_zero"`
	HomeAssistant HomeAssistantConfig `json:"home_assistant"`
	Mqtt          MqttConfig          `json:"mqtt"`
	Notion        NotionConfig        `json:"notion"`
}

// HomeAssistantConfig 智能家居插件配置
type HomeAssistantConfig struct {
	Enabled        bool                      `json:"enabled"`
	BaseURL        string                    `json:"base_url"`
	TokenRef       string                    `json:"token_ref"`
	TimeoutSeconds int                       `json:"timeout_seconds"`
	VerifyTLS      bool                      `json:"verify_tls"`
	MaxOutputChars int                       `json:"max_output_chars"`
	MaxEntities    int                       `json:"max_entities"`
	Policy         HomeAssistantPolicyConfig `json:"policy"`
	Events         HomeAssistantEventsConfig `json:"events"`
}

func NewDefaultHomeAssistantConfig() *HomeAssistantConfig {
	return &HomeAssistantConfig{
		BaseURL:        "http://homeassistant.local:8123",
		TokenRef:       "env:HOME_ASSISTANT_TOKEN",
		TimeoutSeconds: 15,
		VerifyTLS:      true,
		MaxOutputChars: 60000,
		MaxEntities:    200,
	}
}

const (
	PluginCompatibilityDiagnosticDefaultSeverity = "error"
	PluginCompatibilityDiagnosticDefaultCode     = ""
	PluginCompatibilityDiagnosticDefaultMessage  = ""
	PluginLoadReportDefaultOrigin                = "bridge"
	PluginLoadReportDefaultEffectiveRuntimeMode  = "jit"
)

type PluginCompatibilityDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Surface  string `json:"surface,omitempty"`
	Path     string `json:"path"`
}

func DefaultPluginCompatibilityDiagnostic() PluginCompatibilityDiagnostic {
	return PluginCompatibilityDiagnostic{
		Severity: PluginCompatibilityDiagnosticDefaultSeverity,
		Code:     PluginCompatibilityDiagnosticDefaultCode,
		Message:  PluginCompatibilityDiagnosticDefaultMessage,
	}
}

type PluginLoadReport struct {
	PluginId               string                          `json:"plugin_id"`
	SourcePath             string                          `json:"source_path"`
	EntryPath              string                          `json:"entry_path"`
	Origin                 string                          `json:"origin"`
	Loaded                 bool                            `json:"loaded"`
	EffectiveRuntimeMode   string                          `json:"effective_runtime_mode"`
	RequestedCapabilities  []string                        `json:"requested_capabilities"`
	BlockedByRuntimeMode   bool                            `json:"blocked_by_runtime_mode"`
	BlockedReason          string                          `json:"blocked_reason,omitempty"`
	ToolCount              int                             `json:"tool_count"`
	ChannelCount           int                             `json:"channel_count"`
	CommandCount           int                             `json:"command_count"`
	EventSubscriptionCount int                             `json:"event_subscription_count"`
	ProviderCount          int                             `json:"provider_count"`
	SkillDirectories       []string                        `json:"skill_directories"`
	Diagnostics            []PluginCompatibilityDiagnostic `json:"diagnostics"`
	Error                  string                          `json:"error,omitempty"`
}

func DefaultPluginLoadReport() PluginLoadReport {
	return PluginLoadReport{
		Origin:               PluginLoadReportDefaultOrigin,
		EffectiveRuntimeMode: PluginLoadReportDefaultEffectiveRuntimeMode,
	}
}

// ==========================================
// HomeAssistant 相关配置
// ==========================================

type HomeAssistantPolicyConfig struct {
	AllowEntityIdGlobs []string `json:"allow_entity_id_globs"`
	DenyEntityIdGlobs  []string `json:"deny_entity_id_globs"`
	AllowServiceGlobs  []string `json:"allow_service_globs"`
	DenyServiceGlobs   []string `json:"deny_service_globs"`
}

func DefaultHomeAssistantPolicyConfig() *HomeAssistantPolicyConfig {
	return &HomeAssistantPolicyConfig{
		AllowEntityIdGlobs: []string{"*"},
		DenyEntityIdGlobs:  []string{},
		AllowServiceGlobs:  []string{"*"},
		DenyServiceGlobs:   []string{},
	}
}

type HomeAssistantEventsConfig struct {
	Enabled               bool                     `json:"enabled"`
	ChannelId             string                   `json:"channel_id"`
	SessionId             string                   `json:"session_id"`
	SubscribeEventTypes   []string                 `json:"subscribe_event_types"`
	EmitAllMatchingEvents bool                     `json:"emit_all_matching_events"`
	GlobalCooldownSeconds int                      `json:"global_cooldown_seconds"`
	AllowEntityIdGlobs    []string                 `json:"allow_entity_id_globs"`
	DenyEntityIdGlobs     []string                 `json:"deny_entity_id_globs"`
	PromptTemplate        string                   `json:"prompt_template"`
	Rules                 []HomeAssistantEventRule `json:"rules"`
}

func DefaultHomeAssistantEventsConfig() *HomeAssistantEventsConfig {
	return &HomeAssistantEventsConfig{
		Enabled:               false,
		ChannelId:             "homeassistant",
		SessionId:             "homeassistant:events",
		SubscribeEventTypes:   []string{"state_changed"},
		EmitAllMatchingEvents: true,
		GlobalCooldownSeconds: 2,
		AllowEntityIdGlobs:    []string{"*"},
		DenyEntityIdGlobs:     []string{},
		PromptTemplate:        "Home Assistant event: {event_type} entity={entity_id} from={from_state} to={to_state} (name={friendly_name})",
		Rules:                 []HomeAssistantEventRule{},
	}
}

type HomeAssistantEventRule struct {
	Name              string   `json:"name"`
	EntityIdGlobs     []string `json:"entity_id_globs"`
	FromState         string   `json:"from_state"`
	ToState           string   `json:"to_state"`
	BetweenLocalStart string   `json:"between_local_start"`
	BetweenLocalEnd   string   `json:"between_local_end"`
	DaysOfWeek        []string `json:"days_of_week"`
	PromptTemplate    string   `json:"prompt_template"`
	CooldownSeconds   int      `json:"cooldown_seconds"`
}

func DefaultHomeAssistantEventRule() *HomeAssistantEventRule {
	return &HomeAssistantEventRule{
		Name:            "",
		EntityIdGlobs:   []string{"*"},
		DaysOfWeek:      []string{},
		PromptTemplate:  "",
		CooldownSeconds: 2,
	}
}

// ==========================================
// MQTT 相关配置
// ==========================================

type MqttConfig struct {
	Enabled         bool             `json:"enabled"`
	Host            string           `json:"host"`
	Port            int              `json:"port"`
	UseTls          bool             `json:"use_tls"`
	UsernameRef     string           `json:"username_ref"`
	PasswordRef     string           `json:"password_ref"`
	ClientId        string           `json:"client_id"`
	TimeoutSeconds  int              `json:"timeout_seconds"`
	MaxPayloadBytes int              `json:"max_payload_bytes"`
	Policy          MqttPolicyConfig `json:"policy"`
	Events          MqttEventsConfig `json:"events"`
}

func DefaultMqttConfig() *MqttConfig {
	return &MqttConfig{
		Enabled:         false,
		Host:            "127.0.0.1",
		Port:            1883,
		UseTls:          false,
		ClientId:        "openclaw",
		TimeoutSeconds:  10,
		MaxPayloadBytes: 262144,
		Policy:          *DefaultMqttPolicyConfig(),
		Events:          *DefaultMqttEventsConfig(),
	}
}

type MqttPolicyConfig struct {
	AllowPublishTopicGlobs   []string `json:"allow_publish_topic_globs"`
	DenyPublishTopicGlobs    []string `json:"deny_publish_topic_globs"`
	AllowSubscribeTopicGlobs []string `json:"allow_subscribe_topic_globs"`
	DenySubscribeTopicGlobs  []string `json:"deny_subscribe_topic_globs"`
}

func DefaultMqttPolicyConfig() *MqttPolicyConfig {
	return &MqttPolicyConfig{
		AllowPublishTopicGlobs:   []string{"*"},
		DenyPublishTopicGlobs:    []string{},
		AllowSubscribeTopicGlobs: []string{"*"},
		DenySubscribeTopicGlobs:  []string{},
	}
}

type MqttEventsConfig struct {
	Enabled       bool                     `json:"enabled"`
	ChannelId     string                   `json:"channel_id"`
	SessionId     string                   `json:"session_id"`
	Subscriptions []MqttSubscriptionConfig `json:"subscriptions"`
}

func DefaultMqttEventsConfig() *MqttEventsConfig {
	return &MqttEventsConfig{
		Enabled:       false,
		ChannelId:     "mqtt",
		SessionId:     "mqtt:events",
		Subscriptions: []MqttSubscriptionConfig{},
	}
}

type MqttSubscriptionConfig struct {
	Topic           string `json:"topic"`
	Qos             int    `json:"qos"`
	PromptTemplate  string `json:"prompt_template"`
	CooldownSeconds int    `json:"cooldown_seconds"`
}

func DefaultMqttSubscriptionConfig() *MqttSubscriptionConfig {
	return &MqttSubscriptionConfig{
		Topic:           "",
		Qos:             0,
		PromptTemplate:  "MQTT message on {topic}: {payload}",
		CooldownSeconds: 1,
	}
}

// ==========================================
// Notion 相关配置
// ==========================================

type NotionConfig struct {
	Enabled                  bool     `json:"enabled"`
	ApiKeyRef                string   `json:"api_key_ref"`
	BaseUrl                  string   `json:"base_url"`
	ApiVersion               string   `json:"api_version"`
	DefaultPageId            string   `json:"default_page_id"`
	DefaultDatabaseId        string   `json:"default_database_id"`
	AllowedPageIds           []string `json:"allowed_page_ids"`
	AllowedDatabaseIds       []string `json:"allowed_database_ids"`
	MaxSearchResults         int      `json:"max_search_results"`
	ReadOnly                 bool     `json:"read_only"`
	RequireApprovalForWrites bool     `json:"require_approval_for_writes"`
}

func DefaultNotionConfig() *NotionConfig {
	return &NotionConfig{
		Enabled:                  false,
		ApiKeyRef:                "env:NOTION_API_KEY",
		BaseUrl:                  "https://api.notion.com/v1",
		ApiVersion:               "2022-06-28",
		AllowedPageIds:           []string{},
		AllowedDatabaseIds:       []string{},
		MaxSearchResults:         10,
		ReadOnly:                 false,
		RequireApprovalForWrites: true,
	}
}

// ==========================================
// Web 搜索与抓取配置
// ==========================================

const (
	WebSearchConfigProviderTavily  = "tavily"
	WebSearchConfigProviderBrave   = "brave"
	WebSearchConfigProviderSearxng = "searxng"
)

type WebSearchConfig struct {
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider"`
	ApiKey     string `json:"api_key"`
	Endpoint   string `json:"endpoint"`
	MaxResults int    `json:"max_results"`
}

func DefaultWebSearchConfig() *WebSearchConfig {
	return &WebSearchConfig{
		Enabled:    false,
		Provider:   WebSearchConfigProviderTavily,
		MaxResults: 5,
	}
}

type WebFetchConfig struct {
	Enabled        bool             `json:"enabled"`
	MaxSizeKb      int              `json:"max_size_kb"`
	TimeoutSeconds int              `json:"timeout_seconds"`
	UserAgent      string           `json:"user_agent"`
	UrlSafety      *UrlSafetyConfig `json:"url_safety"`
}

func DefaultWebFetchConfig() *WebFetchConfig {
	return &WebFetchConfig{
		Enabled:        false,
		MaxSizeKb:      512,
		TimeoutSeconds: 15,
		UserAgent:      "OpenClaw/1.0",
	}
}

// ==========================================
// Git 工具配置
// ==========================================

type GitToolsConfig struct {
	Enabled      bool `json:"enabled"`
	AllowPush    bool `json:"allow_push"`
	MaxDiffBytes int  `json:"max_diff_bytes"`
}

func DefaultGitToolsConfig() *GitToolsConfig {
	return &GitToolsConfig{
		Enabled:      false,
		AllowPush:    false,
		MaxDiffBytes: 64 * 1024,
	}
}

// ==========================================
// 代码执行配置
// ==========================================

const (
	CodeExecConfigBackendDocker  = "docker"
	CodeExecConfigBackendProcess = "process"
)

type CodeExecConfig struct {
	Enabled          bool     `json:"enabled"`
	Backend          string   `json:"backend"`
	DockerImage      string   `json:"docker_image"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
	MaxOutputBytes   int      `json:"max_output_bytes"`
	AllowedLanguages []string `json:"allowed_languages"`
}

func DefaultCodeExecConfig() *CodeExecConfig {
	return &CodeExecConfig{
		Enabled:          false,
		Backend:          CodeExecConfigBackendProcess,
		DockerImage:      "python:3.12-slim",
		TimeoutSeconds:   30,
		MaxOutputBytes:   64 * 1024,
		AllowedLanguages: []string{"python", "javascript", "bash"},
	}
}

type ImageGenConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`           // Provider: "openai" (DALL-E)
	ApiKey   string `json:"api_key,omitempty"`  // API key (or env: / raw: secret ref)
	Endpoint string `json:"endpoint,omitempty"` // API endpoint (optional, for compatible APIs)
	Model    string `json:"model"`              // Model name (e.g. "dall-e-3")
	Size     string `json:"size"`               // Default image size
	Quality  string `json:"quality"`            // Default quality ("standard" or "hd" for DALL-E 3)
}

func DefaultImageGenConfig() ImageGenConfig {
	return ImageGenConfig{
		Enabled:  false,
		Provider: "openai",
		Model:    "dall-e-3",
		Size:     "1024x1024",
		Quality:  "standard",
	}
}

// --- PdfReadConfig ---

type PdfReadConfig struct {
	Enabled        bool `json:"enabled"`
	MaxPages       int  `json:"max_pages"`        // Maximum pages to extract (0 = all)
	MaxOutputChars int  `json:"max_output_chars"` // Maximum output characters
}

func DefaultPdfReadConfig() PdfReadConfig {
	return PdfReadConfig{
		Enabled:        false,
		MaxPages:       50,
		MaxOutputChars: 100000,
	}
}

// --- CalendarConfig ---

type CalendarConfig struct {
	Enabled         bool   `json:"enabled"`
	Provider        string `json:"provider"`                   // Provider: "google"
	CredentialsPath string `json:"credentials_path,omitempty"` // Path to service account JSON key or OAuth credentials file
	CalendarId      string `json:"calendar_id"`                // Calendar ID to operate on (default: primary)
	MaxEvents       int    `json:"max_events"`                 // Maximum events to return in list operations
}

func DefaultCalendarConfig() CalendarConfig {
	return CalendarConfig{
		Enabled:    false,
		Provider:   "google",
		CalendarId: "primary",
		MaxEvents:  25,
	}
}

// --- EmailConfig ---

type EmailConfig struct {
	Enabled                   bool   `json:"enabled"`
	InboundEnabled            bool   `json:"inbound_enabled"`               // Whether the email channel should poll IMAP and emit inbound messages
	SmtpHost                  string `json:"smtp_host,omitempty"`           // SMTP server host for sending
	SmtpPort                  int    `json:"smtp_port"`                     // SMTP server port
	SmtpUseTls                bool   `json:"smtp_use_tls"`                  // Whether to use TLS for SMTP
	ImapHost                  string `json:"imap_host,omitempty"`           // IMAP server host for reading
	ImapPort                  int    `json:"imap_port"`                     // IMAP server port
	ImapUseTls                bool   `json:"imap_use_tls"`                  // Whether to use TLS for IMAP
	InboundFolder             string `json:"inbound_folder"`                // IMAP folder to poll for inbound messages
	InboundPollSeconds        int    `json:"inbound_poll_seconds"`          // Polling interval in seconds for inbound IMAP checks
	InboundMaxMessagesPerPoll int    `json:"inbound_max_messages_per_poll"` // Maximum number of unseen messages to process per poll
	MarkInboundAsRead         bool   `json:"mark_inbound_as_read"`          // Whether inbound messages should be marked as read after successful handoff
	Username                  string `json:"username,omitempty"`            // Email account username
	PasswordRef               string `json:"password_ref,omitempty"`        // Email account password (or env: / raw: secret ref)
	FromAddress               string `json:"from_address,omitempty"`        // From address for outgoing mail
	MaxResults                int    `json:"max_results"`                   // Maximum emails to return in list/search operations
}

func DefaultEmailConfig() EmailConfig {
	return EmailConfig{
		Enabled:                   false,
		InboundEnabled:            false,
		SmtpPort:                  587,
		SmtpUseTls:                true,
		ImapPort:                  993,
		ImapUseTls:                true,
		InboundFolder:             "INBOX",
		InboundPollSeconds:        30,
		InboundMaxMessagesPerPoll: 10,
		MarkInboundAsRead:         true,
		MaxResults:                20,
	}
}

// --- DatabaseConfig ---

type DatabaseConfig struct {
	Enabled             bool     `json:"enabled"`
	Provider            string   `json:"provider"`                    // Database provider: "sqlite", "postgres", "mysql"
	ConnectionString    string   `json:"connection_string,omitempty"` // Connection string (or env: / raw: secret ref)
	AllowWrite          bool     `json:"allow_write"`                 // Whether to allow write operations (INSERT, UPDATE, DELETE, CREATE, DROP)
	TimeoutSeconds      int      `json:"timeout_seconds"`             // Query timeout in seconds
	MaxRows             int      `json:"max_rows"`                    // Maximum rows to return
	AllowedTables       []string `json:"allowed_tables"`              // Allowed table names (schema-qualified optional). Empty = allow all tables
	DeniedTables        []string `json:"denied_tables"`               // Denied table names (schema-qualified optional). Deny wins over allow
	AllowMultiStatement bool     `json:"allow_multi_statement"`       // Whether SQL containing multiple statements is allowed
}

func DefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Enabled:             false,
		Provider:            "sqlite",
		AllowWrite:          false,
		TimeoutSeconds:      30,
		MaxRows:             1000,
		AllowedTables:       []string{},
		DeniedTables:        []string{},
		AllowMultiStatement: false,
	}
}

// --- InboxZeroConfig ---

type InboxZeroConfig struct {
	VipSenders                  []string `json:"vip_senders"`                    // VIP sender addresses — emails from these are never auto-archived
	ProtectedSenders            []string `json:"protected_senders"`              // Protected sender addresses or domains — e.g. doctor@hospital.org, bank.com
	ProtectedKeywords           []string `json:"protected_keywords"`             // Protected keywords in subject — emails matching these are never auto-archived
	MaxBatchSize                int      `json:"max_batch_size"`                 // Maximum emails to process per batch
	DryRun                      bool     `json:"dry_run"`                        // When true, report what would happen without actually moving/deleting anything
	ImapOperationTimeoutSeconds int      `json:"imap_operation_timeout_seconds"` // Optional IMAP operation timeout in seconds. 0 disables this additional timeout
	MaxResponseLinesPerCommand  int      `json:"max_response_lines_per_command"` // Maximum number of IMAP response lines to read for a tagged command
	Enabled                     bool     `json:"enabled"`
}

func DefaultInboxZeroConfig() InboxZeroConfig {
	return InboxZeroConfig{
		Enabled:                     false,
		VipSenders:                  []string{},
		ProtectedSenders:            []string{},
		ProtectedKeywords:           []string{"appointment", "flight", "boarding", "medical", "prescription", "invoice", "payment", "receipt"},
		MaxBatchSize:                100,
		DryRun:                      true,
		ImapOperationTimeoutSeconds: 0,
		MaxResponseLinesPerCommand:  10000,
	}
}

// --- PluginLoadConfig ---

type PluginLoadConfig struct {
	Paths []string `json:"paths"` // Extra plugin paths to scan (file or directory)
}

func DefaultPluginLoadConfig() PluginLoadConfig {
	return PluginLoadConfig{
		Paths: []string{},
	}
}

// --- PluginDiscoveryResult ---
type PluginDiscoveryResult struct {
	Plugins []DiscoveredPlugin `json:"plugins"`
	Reports []PluginLoadReport `json:"reports"`
}

// PluginToolRegistration 来自插件桥的工具注册 — 描述插件导出的工具。
type PluginToolRegistration struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // 对应 JsonElement
	Optional    bool            `json:"optional"`
}

// NativeDynamicPluginsConfig 动态原生插件整体配置
type NativeDynamicPluginsConfig struct {
	Enabled bool                         `json:"enabled"`
	Allow   []string                     `json:"allow"`
	Deny    []string                     `json:"deny"`
	Load    PluginLoadConfig             `json:"load"`
	Entries map[string]PluginEntryConfig `json:"entries"`
}

func DefaultNativeDynamicPluginsConfig() NativeDynamicPluginsConfig {
	return NativeDynamicPluginsConfig{
		Enabled: false,
		Allow:   []string{},
		Deny:    []string{},
		Load:    DefaultPluginLoadConfig(), // 假设该类型也有默认值
		Entries: make(map[string]PluginEntryConfig),
	}
}

// NativeDynamicPluginManifest 动态原生插件清单
type NativeDynamicPluginManifest struct {
	Id               string   `json:"id"`
	Name             string   `json:"name,omitempty"`
	Version          string   `json:"version,omitempty"`
	MinHostVersion   string   `json:"min_host_version,omitempty"`
	PluginApiVersion string   `json:"plugin_api_version,omitempty"`
	AssemblyPath     string   `json:"assembly_path"`
	TypeName         string   `json:"type_name"`
	Capabilities     []string `json:"capabilities"`
	Skills           []string `json:"skills"`
	JitOnly          bool     `json:"jit_only"`
}

func DefaultNativeDynamicPluginManifest() NativeDynamicPluginManifest {
	return NativeDynamicPluginManifest{
		Capabilities: []string{},
		Skills:       []string{},
		JitOnly:      true,
	}
}

// DiscoveredNativeDynamicPlugin 已检测到的动态原生插件
type DiscoveredNativeDynamicPlugin struct {
	Manifest     NativeDynamicPluginManifest `json:"manifest"`
	RootPath     string                      `json:"root_path"`
	ManifestPath string                      `json:"manifest_path"`
	AssemblyPath string                      `json:"assembly_path"`
}

// BridgeRequest 用于插件桥接通信的 JSON-RPC 请求外壳。
type BridgeRequest struct {
	Method string           `json:"method"`
	Id     string           `json:"id"`
	Params *json.RawMessage `json:"params,omitempty"`
}

// BridgeResponse 来自插件桥的 JSON-RPC 响应外壳。
type BridgeResponse struct {
	Id     string           `json:"id"`
	Result *json.RawMessage `json:"result,omitempty"`
	Error  *BridgeError     `json:"error,omitempty"`
}

// BridgeError 来自插件桥的错误载荷。
type BridgeError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// BridgeNotification 来自插件桥进程的通知（插件 → 网关）。
type BridgeNotification struct {
	Notification string           `json:"notification"`
	Params       *json.RawMessage `json:"params,omitempty"`
}

// BridgeInitResult 初始化插件桥进程的结果。
type BridgeInitResult struct {
	Tools              []PluginToolRegistration        `json:"tools"`
	Channels           []BridgeChannelRegistration     `json:"channels"`
	Commands           []BridgeCommandRegistration     `json:"commands"`
	EventSubscriptions []string                        `json:"event_subscriptions"`
	Providers          []BridgeProviderRegistration    `json:"providers"`
	Capabilities       []string                        `json:"capabilities"`
	Diagnostics        []PluginCompatibilityDiagnostic `json:"diagnostics"`
	Compatible         bool                            `json:"compatible"`
}

func DefaultBridgeInitResult() BridgeInitResult {
	return BridgeInitResult{
		Tools:              []PluginToolRegistration{},
		Channels:           []BridgeChannelRegistration{},
		Commands:           []BridgeCommandRegistration{},
		EventSubscriptions: []string{},
		Providers:          []BridgeProviderRegistration{},
		Capabilities:       []string{},
		Diagnostics:        []PluginCompatibilityDiagnostic{},
		Compatible:         true,
	}
}

// BridgeTransportConfig 插件桥的传输配置。
type BridgeTransportConfig struct {
	Mode       string `json:"mode"` // "stdio" (default), "socket", or "hybrid"
	SocketPath string `json:"socket_path,omitempty"`
}

func DefaultBridgeTransportConfig() BridgeTransportConfig {
	return BridgeTransportConfig{
		Mode: "stdio",
	}
}

// BridgeTransportRuntimeConfig 在初始化期间发送给桥进程的运行时传输详情。
type BridgeTransportRuntimeConfig struct {
	Mode            string `json:"mode"`
	SocketPath      string `json:"socket_path,omitempty"`
	SocketDirectory string `json:"socket_directory,omitempty"`
	SocketAuthToken string `json:"socket_auth_token,omitempty"`
	SecurityMode    string `json:"security_mode"`
}

func DefaultBridgeTransportRuntimeConfig() BridgeTransportRuntimeConfig {
	return BridgeTransportRuntimeConfig{
		Mode:         "stdio",
		SecurityMode: "legacy",
	}
}

// BridgeChannelRegistration 频道注册信息
type BridgeChannelRegistration struct {
	Id string `json:"id"`
}

// BridgeCommandRegistration 命令注册信息
type BridgeCommandRegistration struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// BridgeProviderRegistration 服务商注册信息
type BridgeProviderRegistration struct {
	Id     string   `json:"id"`
	Models []string `json:"models"`
}

func DefaultBridgeProviderRegistration() BridgeProviderRegistration {
	return BridgeProviderRegistration{
		Models: []string{},
	}
}

// BridgeProviderRequest 网关发往插件桥的提供商补全请求。
type BridgeProviderRequest struct {
	ProviderId string                 `json:"provider_id"`
	Messages   json.RawMessage        `json:"messages"`
	Options    *BridgeProviderOptions `json:"options,omitempty"`
}

// BridgeProviderOptions 转发给插件提供商的 ChatOptions 序列化子集。
type BridgeProviderOptions struct {
	ConversationId           string                     `json:"conversation_id,omitempty"`
	Instructions             string                     `json:"instructions,omitempty"`
	Temperature              *float32                   `json:"temperature,omitempty"`
	MaxOutputTokens          *int                       `json:"max_output_tokens,omitempty"`
	TopP                     *float32                   `json:"top_p,omitempty"`
	TopK                     *int                       `json:"top_k,omitempty"`
	FrequencyPenalty         *float32                   `json:"frequency_penalty,omitempty"`
	PresencePenalty          *float32                   `json:"presence_penalty,omitempty"`
	Seed                     *int64                     `json:"seed,omitempty"`
	Reasoning                *BridgeReasoningOptions    `json:"reasoning,omitempty"`
	ResponseFormat           *BridgeResponseFormat      `json:"response_format,omitempty"`
	ModelId                  string                     `json:"model_id,omitempty"`
	StopSequences            []string                   `json:"stop_sequences"`
	AllowMultipleToolCalls   *bool                      `json:"allow_multiple_tool_calls,omitempty"`
	ToolMode                 *BridgeToolMode            `json:"tool_mode,omitempty"`
	Tools                    []BridgeToolDescriptor     `json:"tools"`
	AllowBackgroundResponses *bool                      `json:"allow_background_responses,omitempty"`
	ContinuationToken        string                     `json:"continuation_token,omitempty"`
	AdditionalProperties     map[string]json.RawMessage `json:"additional_properties"`
}

func DefaultBridgeProviderOptions() BridgeProviderOptions {
	return BridgeProviderOptions{
		StopSequences:        []string{},
		Tools:                []BridgeToolDescriptor{},
		AdditionalProperties: make(map[string]json.RawMessage),
	}
}

// BridgeReasoningOptions 代表推理选项
type BridgeReasoningOptions struct {
	Effort string `json:"effort,omitempty"`
	Output string `json:"output,omitempty"`
}

// BridgeResponseFormat 代表响应格式
type BridgeResponseFormat struct {
	Kind              string           `json:"kind"`
	Schema            *json.RawMessage `json:"schema,omitempty"`
	SchemaName        string           `json:"schema_name,omitempty"`
	SchemaDescription string           `json:"schema_description,omitempty"`
}

// BridgeToolMode 代表工具模式
type BridgeToolMode struct {
	Kind         string `json:"kind"`
	FunctionName string `json:"function_name,omitempty"`
}

// BridgeToolDescriptor 代表工具描述符
type BridgeToolDescriptor struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	InputSchema  *json.RawMessage `json:"input_schema,omitempty"`
	ReturnSchema *json.RawMessage `json:"return_schema,omitempty"`
}

// DefaultBridgeToolDescriptor 设置默认值
func DefaultBridgeToolDescriptor() BridgeToolDescriptor {
	return BridgeToolDescriptor{
		Description: "",
	}
}

// BridgeInitRequest 代表初始化请求
type BridgeInitRequest struct {
	EntryPath string                       `json:"entry_path"`
	PluginId  string                       `json:"plugin_id"`
	Config    *json.RawMessage             `json:"config,omitempty"`
	Transport BridgeTransportRuntimeConfig `json:"transport"`
}

// DefaultBridgeInitRequest 设置默认值
func DefaultBridgeInitRequest() BridgeInitRequest {
	return BridgeInitRequest{
		Transport: DefaultBridgeTransportRuntimeConfig(), // 假设该依赖结构体也有默认值函数
	}
}

// BridgeExecuteRequest 代表执行请求
type BridgeExecuteRequest struct {
	Name   string           `json:"name"`
	Params *json.RawMessage `json:"params,omitempty"`
}

// BridgeChannelControlRequest 代表通道控制请求
type BridgeChannelControlRequest struct {
	ChannelId string `json:"channel_id"`
}

// BridgeChannelSendRequest 代表通道发送消息请求
type BridgeChannelSendRequest struct {
	ChannelId        string                  `json:"channel_id"`
	RecipientId      string                  `json:"recipient_id"`
	Text             string                  `json:"text"`
	AccountId        string                  `json:"account_id,omitempty"`
	SessionId        string                  `json:"session_id,omitempty"`
	ReplyToMessageId string                  `json:"reply_to_message_id,omitempty"`
	Subject          string                  `json:"subject,omitempty"`
	Attachments      []BridgeMediaAttachment `json:"attachments,omitempty"`
}

// BridgeMediaAttachment 桥接通道消息的的媒体附件
type BridgeMediaAttachment struct {
	// Type 媒体类型: "image", "video", "audio", "document", "sticker"
	Type string `json:"type"`
	// Url HTTP URL 或文件路径
	Url string `json:"url,omitempty"`
	// Caption 可选的说明文字
	Caption string `json:"caption,omitempty"`
	// MimeType MIME 类型提示 (例如 "audio/ogg; codecs=opus")
	MimeType string `json:"mime_type,omitempty"`
	// FileName 原始文件名
	FileName string `json:"file_name,omitempty"`
	// GifPlayback 为 true 时，视频应作为动态 GIF 发送
	GifPlayback bool `json:"gif_playback"`
}

// BridgeChannelTypingRequest 发送正在输入状态的请求
type BridgeChannelTypingRequest struct {
	ChannelId   string `json:"channel_id"`
	RecipientId string `json:"recipient_id"`
	AccountId   string `json:"account_id,omitempty"`
	IsTyping    bool   `json:"is_typing"`
}

// DefaultBridgeChannelTypingRequest 设置默认值
func DefaultBridgeChannelTypingRequest() BridgeChannelTypingRequest {
	return BridgeChannelTypingRequest{
		IsTyping: true,
	}
}

// BridgeChannelReceiptRequest 发送已读回执的请求
type BridgeChannelReceiptRequest struct {
	ChannelId   string `json:"channel_id"`
	MessageId   string `json:"message_id"`
	AccountId   string `json:"account_id,omitempty"`
	RemoteJid   string `json:"remote_jid,omitempty"`
	Participant string `json:"participant,omitempty"`
}

// BridgeChannelReactionRequest 发送消息回应(Emoji)的请求
type BridgeChannelReactionRequest struct {
	ChannelId   string `json:"channel_id"`
	MessageId   string `json:"message_id"`
	Emoji       string `json:"emoji"`
	AccountId   string `json:"account_id,omitempty"`
	RemoteJid   string `json:"remote_jid,omitempty"`
	Participant string `json:"participant,omitempty"`
}

// 常量定义：带有类名前缀以防冲突
const (
	BridgeChannelAuthEventStateQrCode       = "qr_code"
	BridgeChannelAuthEventStateConnected    = "connected"
	BridgeChannelAuthEventStateDisconnected = "disconnected"
	BridgeChannelAuthEventStateError        = "error"
)

// BridgeChannelAuthEvent 桥接通道的认证事件通知（例如 WhatsApp 连结的二维码）
type BridgeChannelAuthEvent struct {
	ChannelId string `json:"channel_id"`
	// State 认证状态: "qr_code", "connected", "disconnected", "error"
	State string `json:"state"`
	// Data 状态特定数据 (二维码字符串、错误信息等)
	Data string `json:"data,omitempty"`
	// AccountId 多账号通道的的账号标识符
	AccountId string `json:"account_id,omitempty"`
	// UpdatedAtUtc 网关收到认证事件的时间戳
	UpdatedAtUtc time.Time `json:"updated_at_utc"`
}

// DefaultBridgeChannelAuthEvent 设置默认值
func DefaultBridgeChannelAuthEvent() BridgeChannelAuthEvent {
	return BridgeChannelAuthEvent{
		UpdatedAtUtc: time.Now().UTC(),
	}
}

// BridgeCommandExecuteRequest 代表命令执行请求
type BridgeCommandExecuteRequest struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

// DefaultBridgeCommandExecuteRequest 设置默认值
func DefaultBridgeCommandExecuteRequest() BridgeCommandExecuteRequest {
	return BridgeCommandExecuteRequest{
		Args: "",
	}
}

// BridgeHookBeforeRequest 前置 Hook 请求
type BridgeHookBeforeRequest struct {
	EventName string `json:"event_name"`
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
}

// BridgeHookAfterRequest 后置 Hook 请求
type BridgeHookAfterRequest struct {
	EventName  string  `json:"event_name"`
	ToolName   string  `json:"tool_name"`
	Arguments  string  `json:"arguments"`
	Result     string  `json:"result"`
	DurationMs float64 `json:"duration_ms"`
	Failed     bool    `json:"failed"`
}

// BridgeToolResult 来自插件桥接的工具执行结果
type BridgeToolResult struct {
	Content []ToolContentItem `json:"content"`
}

// DefaultBridgeToolResult 设置默认值
func DefaultBridgeToolResult() BridgeToolResult {
	return BridgeToolResult{
		Content: []ToolContentItem{},
	}
}

// ToolContentItem 与 MCP 兼容的插件工具返回的内容项
type ToolContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ImageAnalyzeConfig struct {
	Enabled          bool   `json:"enabled"`
	Provider         string `json:"provider"`
	ApiKey           string `json:"api_key"`
	Endpoint         string `json:"endpoint"`
	Model            string `json:"model"`
	MaxImagesPerCall int    `json:"max_images_per_call"`
	MaxOutputChars   int    `json:"max_output_chars"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
}

type MinerUPdfConfig struct {
	Enabled         bool   `json:"enabled"`
	Url             string `json:"url"`
	Backend         string `json:"backend"`
	ParseMethod     string `json:"parse_method"`
	Lang            string `json:"lang"`
	FormulaEnable   bool   `json:"formula_enable"`
	TableEnable     bool   `json:"table_enable"`
	SglangServerUrl string `json:"sglang_server_url"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	MaxOutputChars  int    `json:"max_output_chars"`
	ExtractImages   bool   `json:"extract_images"`
}

func DefaultImageAnalyzeConfig() *ImageAnalyzeConfig {
	return &ImageAnalyzeConfig{
		Provider:         "openai",
		Model:            "gpt-4o",
		MaxImagesPerCall: 5,
		MaxOutputChars:   8000,
		TimeoutSeconds:   60,
	}
}

func DefaultMinerUPdfConfig() *MinerUPdfConfig {
	return &MinerUPdfConfig{
		Url:            "http://localhost:8888",
		Backend:        "pipeline",
		ParseMethod:    "auto",
		Lang:           "ch",
		FormulaEnable:  true,
		TableEnable:    true,
		TimeoutSeconds: 300,
		MaxOutputChars: 200000,
	}
}
