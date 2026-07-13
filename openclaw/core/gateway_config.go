package core

// TokenCostRateConfig
type TokenCostRateConfig struct {
	InputUsdPer1K  float64 `json:"input_usd_per_1k"`
	OutputUsdPer1K float64 `json:"output_usd_per_1k"`
}

func DefaultTokenCostRateConfig() TokenCostRateConfig {
	return TokenCostRateConfig{
		InputUsdPer1K:  0.0,
		OutputUsdPer1K: 0.0,
	}
}

var DefaultTokenDetailedRates = map[string]TokenCostRateConfig{
	"openai:gpt-4o":               {InputUsdPer1K: 0.0025, OutputUsdPer1K: 0.010},
	"openai:gpt-4o-mini":          {InputUsdPer1K: 0.00015, OutputUsdPer1K: 0.0006},
	"openai:gpt-4.1":              {InputUsdPer1K: 0.002, OutputUsdPer1K: 0.008},
	"openai:gpt-4.1-mini":         {InputUsdPer1K: 0.0004, OutputUsdPer1K: 0.0016},
	"openai:gpt-4.1-nano":         {InputUsdPer1K: 0.0001, OutputUsdPer1K: 0.0004},
	"anthropic:claude-sonnet-4-5": {InputUsdPer1K: 0.003, OutputUsdPer1K: 0.015},
	"anthropic:claude-haiku-3-5":  {InputUsdPer1K: 0.0008, OutputUsdPer1K: 0.004},
	"ollama":                      {InputUsdPer1K: 0.0, OutputUsdPer1K: 0.0},
}

type LlmProviderConfig struct {
	Provider                      string              `json:"provider"`
	Model                         string              `json:"model"`
	ApiKey                        *string             `json:"api_key"`
	Endpoint                      string              `json:"endpoint"`
	AuthMode                      string              `json:"auth_mode"`
	SendRequestMetadata           bool                `json:"send_request_metadata"`
	FallbackModels                []string            `json:"fallback_models"`
	MaxTokens                     int                 `json:"max_tokens"`
	Temperature                   float32             `json:"temperature"`
	TimeoutSeconds                int                 `json:"timeout_seconds"`
	RetryCount                    int                 `json:"retry_count"`
	CircuitBreakerThreshold       int                 `json:"circuit_breaker_threshold"`
	CircuitBreakerCooldownSeconds int                 `json:"circuit_breaker_cooldown_seconds"`
	PromptCaching                 PromptCachingConfig `json:"prompt_caching"`
}

func DefaultLlmProviderConfig() LlmProviderConfig {
	return LlmProviderConfig{
		Provider:                      "openai",
		Model:                         "gpt-4o",
		AuthMode:                      "bearer",
		SendRequestMetadata:           false,
		FallbackModels:                []string{},
		MaxTokens:                     4096,
		Temperature:                   0.7,
		TimeoutSeconds:                120,
		RetryCount:                    3,
		CircuitBreakerThreshold:       5,
		CircuitBreakerCooldownSeconds: 30,
		PromptCaching:                 DefaultPromptCachingConfig(),
	}
}

type LocalInferenceConfig struct {
	Enabled                  bool   `json:"enabled"`
	AutoStart                bool   `json:"auto_start"`
	Backend                  string `json:"backend"`
	RuntimePath              string `json:"runtime_path"`
	ModelsRoot               string `json:"models_root"`
	LogsPath                 string `json:"logs_path"`
	Host                     string `json:"host"`
	Port                     int    `json:"port"`
	Threads                  string `json:"threads"`
	GpuLayers                string `json:"gpu_layers"`
	ContextSize              int    `json:"context_size"`
	StartupTimeoutSeconds    int    `json:"startup_timeout_seconds"`
	MaxRestartAttempts       int    `json:"max_restart_attempts"`
	EnableJinja              bool   `json:"enable_jinja"`
	ChatTemplate             string `json:"chat_template"`
	ChatTemplateFilePath     string `json:"chat_template_file_path"`
	MultimodalProjectorPath  string `json:"multimodal_projector_path"`
	MediaPath                string `json:"media_path"`
	DraftModelPath           string `json:"draft_model_path"`
	DraftModelGpuLayers      string `json:"draft_model_gpu_layers"`
	ReasoningEffort          string `json:"reasoning_effort"`
	ReasoningMode            string `json:"reasoning_mode"`
	ReasoningBudget          int    `json:"reasoning_budget"`
	LiteRtRuntimePath        string `json:"lite_rt_runtime_path"`
	LiteRtMediaPipeGraphPath string `json:"lite_rt_media_pipe_graph_path"`
}

func DefaultLocalInferenceConfig() LocalInferenceConfig {
	return LocalInferenceConfig{
		Enabled:               false,
		AutoStart:             true,
		Backend:               "llama.cpp",
		Host:                  "127.0.0.1",
		Port:                  0,
		Threads:               "auto",
		GpuLayers:             "auto",
		ContextSize:           0,
		StartupTimeoutSeconds: 30,
		MaxRestartAttempts:    3,
		EnableJinja:           true,
		DraftModelGpuLayers:   "auto",
	}
}

type DiagnosticsConfig struct {
	CacheTrace PromptCacheTraceConfig `json:"cache_trace"`
}

func DefaultDiagnosticsConfig() DiagnosticsConfig {
	return DiagnosticsConfig{
		CacheTrace: DefaultPromptCacheTraceConfig(),
	}
}

type PromptCacheTraceConfig struct {
	Enabled         bool    `json:"enabled"`
	FilePath        *string `json:"file_path"`
	IncludeMessages bool    `json:"include_messages"`
	IncludePrompt   bool    `json:"include_prompt"`
	IncludeSystem   bool    `json:"include_system"`
}

func DefaultPromptCacheTraceConfig() PromptCacheTraceConfig {
	return PromptCacheTraceConfig{
		Enabled:         false,
		IncludeMessages: true,
		IncludePrompt:   true,
		IncludeSystem:   true,
	}
}

type MemoryConfig struct {
	Provider             string                 `json:"provider"`
	StoragePath          string                 `json:"storage_path"`
	MaxHistoryTurns      int                    `json:"max_history_turns"`
	MaxCachedSessions    *int                   `json:"max_cached_sessions"`
	Sqlite               *MemorySqliteConfig    `json:"sqlite"`
	Postgres             *MemoryPostgresConfig  `json:"postgres"`
	Mempalace            *MemoryMempalaceConfig `json:"mempalace"`
	Fractal              *FractalMemoryConfig   `json:"fractal"`
	Recall               *MemoryRecallConfig    `json:"recall"`
	Retention            *MemoryRetentionConfig `json:"retention"`
	EnableCompaction     bool                   `json:"enable_compaction"`
	CompactionThreshold  int                    `json:"compaction_threshold"`
	CompactionKeepRecent int                    `json:"compaction_keep_recent"`
	ProjectId            *string                `json:"project_id"`
}

func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		Provider:             "file",
		StoragePath:          "./memory",
		MaxHistoryTurns:      50,
		Retention:            DefaultMemoryRetentionConfig(),
		EnableCompaction:     false,
		CompactionThreshold:  80,
		CompactionKeepRecent: 10,
	}
}

type MemoryRetentionConfig struct {
	Enabled              bool   `json:"enabled"`
	RunOnStartup         bool   `json:"run_on_startup"`
	SweepIntervalMinutes int    `json:"sweep_interval_minutes"`
	SessionTtlDays       int    `json:"session_ttl_days"`
	BranchTtlDays        int    `json:"branch_ttl_days"`
	ArchiveEnabled       bool   `json:"archive_enabled"`
	ArchivePath          string `json:"archive_path"`
	ArchiveRetentionDays int    `json:"archive_retention_days"`
	MaxItemsPerSweep     int    `json:"max_items_per_sweep"`
}

func DefaultMemoryRetentionConfig() *MemoryRetentionConfig {
	return &MemoryRetentionConfig{
		Enabled:              false,
		RunOnStartup:         true,
		SweepIntervalMinutes: 30,
		SessionTtlDays:       30,
		BranchTtlDays:        14,
		ArchiveEnabled:       true,
		ArchivePath:          "./memory/archive",
		ArchiveRetentionDays: 30,
		MaxItemsPerSweep:     1000,
	}
}

type GatewayConfig struct {
	BindAddress                          string                         `json:"bind_address"`
	Port                                 int                            `json:"port"`
	AuthToken                            string                         `json:"auth_token"`
	Runtime                              RuntimeConfig                  `json:"runtime"`
	Llm                                  LlmProviderConfig              `json:"llm"`
	Models                               ModelsConfig                   `json:"models"`
	LocalInference                       LocalInferenceConfig           `json:"local_inference"`
	Memory                               MemoryConfig                   `json:"memory"`
	Security                             SecurityConfig                 `json:"security"`
	WebSocket                            WebSocketConfig                `json:"web_socket"`
	Canvas                               CanvasConfig                   `json:"canvas"`
	Tooling                              ToolingConfig                  `json:"tooling"`
	Harness                              HarnessConfig                  `json:"harness"`
	Governance                           ToolGovernanceConfig           `json:"governance"`
	Payments                             PaymentConfig                  `json:"payments"`
	ExternalCli                          ExternalCliOptions             `json:"external_cli"`
	Sandbox                              SandboxConfig                  `json:"sandbox"`
	Execution                            ExecutionConfig                `json:"execution"`
	CodingBackends                       CodingBackendsConfig           `json:"coding_backends"`
	Multimodal                           MultimodalConfig               `json:"multimodal"`
	Channels                             ChannelsConfig                 `json:"channels"`
	Plugins                              PluginsConfig                  `json:"plugins"`
	Skills                               SkillsConfig                   `json:"skills"`
	Delegation                           DelegationConfig               `json:"delegation"`
	Workflows                            WorkflowsConfig                `json:"workflows"`
	Pulse                                PulseConfig                    `json:"pulse"`
	Cron                                 CronConfig                     `json:"cron"`
	Automations                          AutomationsConfig              `json:"automations"`
	Profiles                             ProfilesConfig                 `json:"profiles"`
	Learning                             LearningConfig                 `json:"learning"`
	Webhooks                             WebhooksConfig                 `json:"webhooks"`
	Routing                              RoutingConfig                  `json:"routing"`
	Deployment                           *DeploymentConfig              `json:"deployment"`
	Tailscale                            TailscaleConfig                `json:"tailscale"`
	GmailPubSub                          GmailPubSubConfig              `json:"gmail_pub_sub"`
	Mdns                                 MdnsConfig                     `json:"mdns"`
	Diagnostics                          DiagnosticsConfig              `json:"diagnostics"`
	UsageFooter                          string                         `json:"usage_footer"`
	MaxConcurrentSessions                int                            `json:"max_concurrent_sessions"`
	SessionTimeoutMinutes                int                            `json:"session_timeout_minutes"`
	SessionTokenBudget                   int64                          `json:"session_token_budget"`
	EnableEstimatedTokenAdmissionControl bool                           `json:"enable_estimated_token_admission_control"`
	SessionRateLimitPerMinute            int                            `json:"session_rate_limit_per_minute"`
	GracefulShutdownSeconds              int                            `json:"graceful_shutdown_seconds"`
	TokenCostRates                       map[string]float64             `json:"token_cost_rates"`
	TokenCostRateDetails                 map[string]TokenCostRateConfig `json:"token_cost_rate_details"`
}

func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		BindAddress:             "127.0.0.1",
		Port:                    18789,
		Llm:                     DefaultLlmProviderConfig(),
		LocalInference:          DefaultLocalInferenceConfig(),
		Memory:                  DefaultMemoryConfig(),
		Diagnostics:             DefaultDiagnosticsConfig(),
		UsageFooter:             "off",
		MaxConcurrentSessions:   64,
		SessionTimeoutMinutes:   30,
		GracefulShutdownSeconds: 15,
		TokenCostRates:          make(map[string]float64),
		TokenCostRateDetails:    make(map[string]TokenCostRateConfig),
	}
}

type MemorySqliteConfig struct {
	DbPath              string  `json:"db_path"`
	EnableFts           bool    `json:"enable_fts"`
	EnableVectors       bool    `json:"enable_vectors"`
	EmbeddingModel      *string `json:"embedding_model"` // Nullable string maps to *string
	EmbeddingDimensions int     `json:"embedding_dimensions"`
}

type MemoryPostgresConfig struct {
	PostgresUrl string `json:"postgres_url"`
}

func NewDefaultMemorySqliteConfig() *MemorySqliteConfig {
	return &MemorySqliteConfig{
		DbPath:              "./memory/openclaw.db",
		EnableFts:           true,
		EnableVectors:       false,
		EmbeddingModel:      nil,
		EmbeddingDimensions: 1536,
	}
}

type MemoryMempalaceConfig struct {
	BasePath             string  `json:"base_path"`
	PalaceId             string  `json:"palace_id"`
	Namespace            *string `json:"namespace"` // Nullable string maps to *string
	CollectionName       string  `json:"collection_name"`
	EmbeddingDimensions  int     `json:"embedding_dimensions"`
	EmbedderIdentifier   string  `json:"embedder_identifier"`
	DefaultWing          string  `json:"default_wing"`
	DefaultRoom          string  `json:"default_room"`
	SessionDbPath        string  `json:"session_db_path"`
	KnowledgeGraphDbPath string  `json:"knowledge_graph_db_path"`
	MaxSearchCandidates  int     `json:"max_search_candidates"`
}

func NewDefaultMemoryMempalaceConfig() *MemoryMempalaceConfig {
	return &MemoryMempalaceConfig{
		BasePath:             "./memory/mempalace",
		PalaceId:             "openclaw",
		Namespace:            nil,
		CollectionName:       "memories",
		EmbeddingDimensions:  384,
		EmbedderIdentifier:   "openclaw:mempalace:hash-v1",
		DefaultWing:          "openclaw",
		DefaultRoom:          "notes",
		SessionDbPath:        "./memory/mempalace/openclaw-sessions.db",
		KnowledgeGraphDbPath: "./memory/mempalace/kg.db",
		MaxSearchCandidates:  200,
	}
}

type FractalMemoryConfig struct {
	Enabled                  bool   `json:"enabled"`
	Mode                     string `json:"mode"`
	RepositoryRoot           string `json:"repository_root"`
	McpCommand               string `json:"mcp_command"`
	DefaultDepth             int    `json:"default_depth"`
	DefaultView              string `json:"default_view"`
	DefaultExportMode        string `json:"default_export_mode"`
	MaxContextChars          int    `json:"max_context_chars"`
	MaxContextTokens         int    `json:"max_context_tokens"`
	AutoContextMode          string `json:"auto_context_mode"`
	AllowWrites              bool   `json:"allow_writes"`
	RequireApprovalForWrites bool   `json:"require_approval_for_writes"`
	AutoRefreshIndexes       bool   `json:"auto_refresh_indexes"`
	IncludeTimeline          bool   `json:"include_timeline"`
	IncludeDecisions         bool   `json:"include_decisions"`
	IncludeArtifacts         bool   `json:"include_artifacts"`
}

func NewDefaultFractalMemoryConfig() *FractalMemoryConfig {
	return &FractalMemoryConfig{
		Enabled:                  false,
		Mode:                     "mcp",
		RepositoryRoot:           "",
		McpCommand:               "fractalmem-mcp",
		DefaultDepth:             1,
		DefaultView:              "index",
		DefaultExportMode:        "compact",
		MaxContextChars:          24000,
		MaxContextTokens:         6000,
		AutoContextMode:          "off",
		AllowWrites:              false,
		RequireApprovalForWrites: true,
		AutoRefreshIndexes:       false,
		IncludeTimeline:          false,
		IncludeDecisions:         true,
		IncludeArtifacts:         false,
	}
}

var DefaultTokenCostRates = map[string]float64{
	"openai:gpt-4o":               0.005,
	"openai:gpt-4o-mini":          0.0003,
	"openai:gpt-4.1":              0.004,
	"openai:gpt-4.1-mini":         0.0008,
	"openai:gpt-4.1-nano":         0.0002,
	"anthropic:claude-sonnet-4-5": 0.006,
	"anthropic:claude-haiku-3-5":  0.002,
	"ollama":                      0.0,
}

// 转换后的 TokenCostRateResolver 匹配逻辑
func ResolveTokenCostRate(config *GatewayConfig, providerId, modelId string) TokenCostRateConfig {
	key := providerId + ":" + modelId

	// Go 的 map 查找默认区分大小写。若需要完全对齐 C# 的 StringComparer.OrdinalIgnoreCase，
	// 建议在存入 GatewayConfig 时统一转成小写，并在查询前：key = strings.ToLower(key)

	if modelDetailedRate, ok := config.TokenCostRateDetails[key]; ok {
		return modelDetailedRate
	}
	if providerDetailedRate, ok := config.TokenCostRateDetails[providerId]; ok {
		return providerDetailedRate
	}
	if modelRate, ok := config.TokenCostRates[key]; ok {
		return TokenCostRateConfig{InputUsdPer1K: modelRate, OutputUsdPer1K: modelRate}
	}
	if providerRate, ok := config.TokenCostRates[providerId]; ok {
		return TokenCostRateConfig{InputUsdPer1K: providerRate, OutputUsdPer1K: providerRate}
	}
	if defaultModelDetailedRate, ok := DefaultTokenDetailedRates[key]; ok {
		return defaultModelDetailedRate
	}
	if defaultProviderDetailedRate, ok := DefaultTokenDetailedRates[providerId]; ok {
		return defaultProviderDetailedRate
	}
	if defaultModelRate, ok := DefaultTokenCostRates[key]; ok {
		return TokenCostRateConfig{InputUsdPer1K: defaultModelRate, OutputUsdPer1K: defaultModelRate}
	}
	if defaultProviderRate, ok := DefaultTokenCostRates[providerId]; ok {
		return TokenCostRateConfig{InputUsdPer1K: defaultProviderRate, OutputUsdPer1K: defaultProviderRate}
	}

	return TokenCostRateConfig{}
}

type MemoryRecallConfig struct {
	Enabled  bool `json:"enabled"`
	MaxNotes int  `json:"max_notes"`
	MaxChars int  `json:"max_chars"`
}

func NewDefaultMemoryRecallConfig() *MemoryRecallConfig {
	return &MemoryRecallConfig{
		Enabled:  false,
		MaxNotes: 8,
		MaxChars: 8000,
	}
}

type SecurityConfig struct {
	StrictPublicBindProfile                  bool     `json:"strict_public_bind_profile"`
	AllowQueryStringToken                    bool     `json:"allow_query_string_token"`
	AllowedOrigins                           []string `json:"allowed_origins"`
	TrustForwardedHeaders                    bool     `json:"trust_forwarded_headers"`
	KnownProxies                             []string `json:"known_proxies"`
	RequireRequesterMatchForHttpToolApproval bool     `json:"require_requester_match_for_http_tool_approval"`
	AllowUnsafeToolingOnPublicBind           bool     `json:"allow_unsafe_tooling_on_public_bind"`
	AllowPluginBridgeOnPublicBind            bool     `json:"allow_plugin_bridge_on_public_bind"`
	AllowRawSecretRefsOnPublicBind           bool     `json:"allow_raw_secret_refs_on_public_bind"`
	BrowserSessionIdleMinutes                int      `json:"browser_session_idle_minutes"`
	BrowserRememberDays                      int      `json:"browser_remember_days"`
}

func NewDefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		StrictPublicBindProfile:                  false,
		AllowQueryStringToken:                    false,
		AllowedOrigins:                           []string{},
		TrustForwardedHeaders:                    false,
		KnownProxies:                             []string{},
		RequireRequesterMatchForHttpToolApproval: false,
		AllowUnsafeToolingOnPublicBind:           false,
		AllowPluginBridgeOnPublicBind:            false,
		AllowRawSecretRefsOnPublicBind:           false,
		BrowserSessionIdleMinutes:                60,
		BrowserRememberDays:                      30,
	}
}

type UrlSafetyConfig struct {
	Enabled                    bool     `json:"enabled"`
	BlockPrivateNetworkTargets bool     `json:"block_private_network_targets"`
	BlockedHostGlobs           []string `json:"blocked_host_globs"`
	BlockedCidrs               []string `json:"blocked_cidrs"`
}

func NewDefaultUrlSafetyConfig() *UrlSafetyConfig {
	return &UrlSafetyConfig{
		Enabled:                    true,
		BlockPrivateNetworkTargets: true,
		BlockedHostGlobs:           []string{},
		BlockedCidrs:               []string{},
	}
}

type WebSocketConfig struct {
	MaxMessageBytes                int `json:"max_message_bytes"`
	MaxConnections                 int `json:"max_connections"`
	MaxConnectionsPerIp            int `json:"max_connections_per_ip"`
	MessagesPerMinutePerConnection int `json:"messages_per_minute_per_connection"`
	ReceiveTimeoutSeconds          int `json:"receive_timeout_seconds"`
}

func NewDefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		MaxMessageBytes:                256 * 1024,
		MaxConnections:                 1000,
		MaxConnectionsPerIp:            50,
		MessagesPerMinutePerConnection: 120,
		ReceiveTimeoutSeconds:          120,
	}
}

type CanvasConfig struct {
	Enabled                bool `json:"enabled"`
	AllowOnPublicBind      bool `json:"allow_on_public_bind"`
	MaxCommandBytes        int  `json:"max_command_bytes"`
	MaxSnapshotBytes       int  `json:"max_snapshot_bytes"`
	CommandTimeoutSeconds  int  `json:"command_timeout_seconds"`
	MaxFramesPerPush       int  `json:"max_frames_per_push"`
	EnableLocalHtml        bool `json:"enable_local_html"`
	EnableRemoteNavigation bool `json:"enable_remote_navigation"`
	EnableEval             bool `json:"enable_eval"`
}

func NewDefaultCanvasConfig() *CanvasConfig {
	return &CanvasConfig{
		Enabled:                true,
		AllowOnPublicBind:      false,
		MaxCommandBytes:        256 * 1024,
		MaxSnapshotBytes:       256 * 1024,
		CommandTimeoutSeconds:  10,
		MaxFramesPerPush:       100,
		EnableLocalHtml:        true,
		EnableRemoteNavigation: false,
		EnableEval:             true,
	}
}

type ToolingConfig struct {
	AutonomyMode               string                      `json:"autonomy_mode"`
	WorkspaceRoot              string                      `json:"workspace_root"`
	WorkspaceOnly              bool                        `json:"workspace_only"`
	AllowedShellCommandGlobs   []string                    `json:"allowed_shell_command_globs"`
	ForbiddenPathGlobs         []string                    `json:"forbidden_path_globs"`
	AllowShell                 bool                        `json:"allow_shell"`
	ReadOnlyMode               bool                        `json:"read_only_mode"`
	AllowedReadRoots           []string                    `json:"allowed_read_roots"`
	AllowedWriteRoots          []string                    `json:"allowed_write_roots"`
	ToolTimeoutSeconds         int                         `json:"tool_timeout_seconds"`
	ParallelToolExecution      bool                        `json:"parallel_tool_execution"`
	RequireToolApproval        bool                        `json:"require_tool_approval"`
	ApprovalRequiredTools      []string                    `json:"approval_required_tools"`
	ToolApprovalTimeoutSeconds int                         `json:"tool_approval_timeout_seconds"`
	EnableBrowserTool          bool                        `json:"enable_browser_tool"`
	EnableLocalTool            bool                        `json:"enable_local_tool"`
	AllowBrowserEvaluate       bool                        `json:"allow_browser_evaluate"`
	BrowserHeadless            bool                        `json:"browser_headless"`
	BrowserTimeoutSeconds      int                         `json:"browser_timeout_seconds"`
	UrlSafety                  UrlSafetyConfig             `json:"url_safety"`
	Toolsets                   map[string]ToolsetConfig    `json:"toolsets"`
	Presets                    map[string]ToolPresetConfig `json:"presets"`
	SurfaceBindings            map[string]string           `json:"surface_bindings"`
}

func NewDefaultToolingConfig() *ToolingConfig {
	workspaceRootDefault := "env:OPENCLAW_WORKSPACE"

	return &ToolingConfig{
		AutonomyMode:               "supervised",
		WorkspaceRoot:              workspaceRootDefault,
		WorkspaceOnly:              false,
		AllowedShellCommandGlobs:   []string{"*"},
		ForbiddenPathGlobs:         []string{},
		AllowShell:                 true,
		ReadOnlyMode:               false,
		AllowedReadRoots:           []string{"*"},
		AllowedWriteRoots:          []string{"*"},
		ToolTimeoutSeconds:         30,
		ParallelToolExecution:      true,
		RequireToolApproval:        false,
		ApprovalRequiredTools:      []string{"shell", "write_file"},
		ToolApprovalTimeoutSeconds: 300,
		EnableBrowserTool:          true,
		AllowBrowserEvaluate:       true,
		BrowserHeadless:            true,
		BrowserTimeoutSeconds:      30,
		UrlSafety:                  *NewDefaultUrlSafetyConfig(),
		Toolsets:                   make(map[string]ToolsetConfig),
		Presets:                    make(map[string]ToolPresetConfig),
		SurfaceBindings:            make(map[string]string),
	}
}

type HarnessConfig struct {
	ExecutionMode     string                   `json:"execution_mode"`
	PlanExecuteVerify PlanExecuteVerifyOptions `json:"plan_execute_verify"`
}

func DefaultHarnessConfig() *HarnessConfig {
	return &HarnessConfig{
		ExecutionMode:     "normal", // 对应 HarnessExecutionModes.Normal
		PlanExecuteVerify: *DefaultPlanExecuteVerifyOptions(),
	}
}

// --- PlanExecuteVerifyOptions ---

type PlanExecuteVerifyOptions struct {
	Enabled                          bool     `json:"enabled"`
	ContractRequiredFor              []string `json:"contract_required_for"`
	RequireApprovalForRisk           []string `json:"require_approval_for_risk"`
	CreateEvidenceBundles            bool     `json:"create_evidence_bundles"`
	RunVerification                  bool     `json:"run_verification"`
	AutoRollbackOnFailedVerification bool     `json:"auto_rollback_on_failed_verification"`
	MaxPlanActions                   int      `json:"max_plan_actions"`
	MaxVerificationSteps             int      `json:"max_verification_steps"`
	RegressionCategories             []string `json:"regression_categories"`
}

func DefaultPlanExecuteVerifyOptions() *PlanExecuteVerifyOptions {
	return &PlanExecuteVerifyOptions{
		Enabled: false,
		ContractRequiredFor: []string{
			"high_risk_tools", // 对应 PlanExecuteVerifyContractTriggers.*
			"write_tools",
			"shell",
			"browser",
			"external_api",
			"multi_tool_workflows",
		},
		RequireApprovalForRisk: []string{
			"high",     // 对应 HarnessContractRiskLevels.High
			"critical", // 对应 HarnessContractRiskLevels.Critical
		},
		CreateEvidenceBundles:            true,
		RunVerification:                  true,
		AutoRollbackOnFailedVerification: false,
		MaxPlanActions:                   20,
		MaxVerificationSteps:             20,
		RegressionCategories:             []string{},
	}
}

// --- PaymentConfig ---

type PaymentConfig struct {
	Enabled          bool                      `json:"enabled"`
	ToolEnabled      bool                      `json:"tool_enabled"`
	Provider         string                    `json:"provider"`
	Environment      string                    `json:"environment"`
	SecretTtlMinutes int                       `json:"secret_ttl_minutes"`
	Policy           PaymentPolicyConfig       `json:"policy"`
	Mock             PaymentMockProviderConfig `json:"mock"`
	StripeLink       PaymentStripeLinkConfig   `json:"stripe_link"`
	MachinePayments  PaymentMachineConfig      `json:"machine_payments"`
}

func DefaultPaymentConfig() *PaymentConfig {
	return &PaymentConfig{
		Enabled:          false,
		ToolEnabled:      true,
		Provider:         "mock",
		Environment:      "test",
		SecretTtlMinutes: 30,
		Policy:           *DefaultPaymentPolicyConfig(),
		Mock:             *DefaultPaymentMockProviderConfig(),
		StripeLink:       *DefaultPaymentStripeLinkConfig(),
		MachinePayments:  *DefaultPaymentMachineConfig(),
	}
}

type PaymentPolicyConfig struct {
	AllowTestModeWithoutApproval   bool   `json:"allow_test_mode_without_approval"`
	DenyLiveWithoutApprovalService bool   `json:"deny_live_without_approval_service"`
	MaxLiveAmountMinor             *int64 `json:"max_live_amount_minor"` // C# 的 long? 对应 *int64
}

func DefaultPaymentPolicyConfig() *PaymentPolicyConfig {
	return &PaymentPolicyConfig{
		AllowTestModeWithoutApproval:   true,
		DenyLiveWithoutApprovalService: true,
		MaxLiveAmountMinor:             nil,
	}
}

type PaymentMockProviderConfig struct {
	ProviderId               string `json:"provider_id"`
	FundingSourceDisplayName string `json:"funding_source_display_name"`
}

func DefaultPaymentMockProviderConfig() *PaymentMockProviderConfig {
	return &PaymentMockProviderConfig{
		ProviderId:               "mock",
		FundingSourceDisplayName: "Mock Visa ending 4242",
	}
}

type PaymentStripeLinkConfig struct {
	ProviderId           string            `json:"provider_id"`
	CliPath              string            `json:"cli_path"`
	TimeoutSeconds       int               `json:"timeout_seconds"`
	WorkingDirectory     *string           `json:"working_directory"`
	EnvironmentVariables map[string]string `json:"environment_variables"`
}

func DefaultPaymentStripeLinkConfig() *PaymentStripeLinkConfig {
	return &PaymentStripeLinkConfig{
		ProviderId:           "stripe-link",
		CliPath:              "link-cli",
		TimeoutSeconds:       30,
		WorkingDirectory:     nil,
		EnvironmentVariables: make(map[string]string),
	}
}

type PaymentMachineConfig struct {
	EnableHttp402Handler bool `json:"enable_http_402_handler"`
}

func DefaultPaymentMachineConfig() *PaymentMachineConfig {
	return &PaymentMachineConfig{
		EnableHttp402Handler: false,
	}
}

// --- ChannelsConfig ---

type ChannelsConfig struct {
	AllowlistSemantics string                `json:"allowlist_semantics"`
	Sms                SmsChannelConfig      `json:"sms"`
	Telegram           TelegramChannelConfig `json:"telegram"`
	WhatsApp           WhatsAppChannelConfig `json:"whatsapp"`
	Teams              TeamsChannelConfig    `json:"teams"`
	Slack              SlackChannelConfig    `json:"slack"`
	Discord            DiscordChannelConfig  `json:"discord"`
	Signal             SignalChannelConfig   `json:"signal"`
}

func DefaultChannelsConfig() *ChannelsConfig {
	return &ChannelsConfig{
		AllowlistSemantics: "legacy",
		WhatsApp:           *DefaultWhatsAppChannelConfig(),
	}
}

// --- WhatsAppChannelConfig ---

type WhatsAppChannelConfig struct {
	Enabled                      bool                           `json:"enabled"`
	Type                         string                         `json:"type"`      // "official", "bridge", or "first_party_worker"
	DmPolicy                     string                         `json:"dm_policy"` // open, pairing, closed
	WebhookPath                  string                         `json:"webhook_path"`
	WebhookPublicBaseUrl         *string                        `json:"webhook_public_base_url"`
	WebhookVerifyToken           string                         `json:"webhook_verify_token"`
	WebhookVerifyTokenRef        string                         `json:"webhook_verify_token_ref"`
	ValidateSignature            bool                           `json:"validate_signature"`
	WebhookAppSecret             *string                        `json:"webhook_app_secret"`
	WebhookAppSecretRef          string                         `json:"webhook_app_secret_ref"`
	CloudApiToken                *string                        `json:"cloud_api_token"`
	CloudApiTokenRef             string                         `json:"cloud_api_token_ref"`
	PhoneNumberId                *string                        `json:"phone_number_id"`
	BusinessAccountId            *string                        `json:"business_account_id"`
	BridgeUrl                    *string                        `json:"bridge_url"`
	BridgeToken                  *string                        `json:"bridge_token"`
	BridgeTokenRef               string                         `json:"bridge_token_ref"`
	BridgeSuppressSendExceptions bool                           `json:"bridge_suppress_send_exceptions"`
	FirstPartyWorker             WhatsAppFirstPartyWorkerConfig `json:"first_party_worker"`
	MaxInboundChars              int                            `json:"max_inbound_chars"`
	MaxRequestBytes              int                            `json:"max_request_bytes"`
	AllowedFromIds               []string                       `json:"allowed_from_ids"`
}

func DefaultWhatsAppChannelConfig() *WhatsAppChannelConfig {
	return &WhatsAppChannelConfig{
		Enabled:                      false,
		Type:                         "official",
		DmPolicy:                     "pairing",
		WebhookPath:                  "/whatsapp/inbound",
		WebhookPublicBaseUrl:         nil,
		WebhookVerifyToken:           "openclaw-verify",
		WebhookVerifyTokenRef:        "env:WHATSAPP_VERIFY_TOKEN",
		ValidateSignature:            false,
		WebhookAppSecret:             nil,
		WebhookAppSecretRef:          "env:WHATSAPP_APP_SECRET",
		CloudApiTokenRef:             "env:WHATSAPP_CLOUD_API_TOKEN",
		PhoneNumberId:                nil,
		BusinessAccountId:            nil,
		BridgeUrl:                    nil,
		BridgeToken:                  nil,
		BridgeTokenRef:               "env:WHATSAPP_BRIDGE_TOKEN",
		BridgeSuppressSendExceptions: false,
		FirstPartyWorker:             *DefaultWhatsAppFirstPartyWorkerConfig(),
		MaxInboundChars:              4096,
		MaxRequestBytes:              64 * 1024,
		AllowedFromIds:               []string{},
	}
}

// --- WhatsAppFirstPartyWorkerConfig ---

type WhatsAppFirstPartyWorkerConfig struct {
	Driver           string                        `json:"driver"` // "baileys", "whatsmeow", "simulated"
	ExecutablePath   *string                       `json:"executable_path"`
	WorkingDirectory *string                       `json:"working_directory"`
	StoragePath      string                        `json:"storage_path"`
	MediaCachePath   *string                       `json:"media_cache_path"`
	HistorySync      bool                          `json:"history_sync"`
	Proxy            *string                       `json:"proxy"`
	Accounts         []WhatsAppWorkerAccountConfig `json:"accounts"`
}

func DefaultWhatsAppFirstPartyWorkerConfig() *WhatsAppFirstPartyWorkerConfig {
	return &WhatsAppFirstPartyWorkerConfig{
		Driver:           "baileys",
		ExecutablePath:   nil,
		WorkingDirectory: nil,
		StoragePath:      "./memory/whatsapp-worker",
		MediaCachePath:   nil,
		HistorySync:      true,
		Proxy:            nil,
		Accounts:         []WhatsAppWorkerAccountConfig{},
	}
}

// --- WhatsAppWorkerAccountConfig ---

type WhatsAppWorkerAccountConfig struct {
	AccountId        string  `json:"account_id"`
	SessionPath      string  `json:"session_path"`
	DeviceName       string  `json:"device_name"`
	PairingMode      string  `json:"pairing_mode"` // "qr" or "pairing_code"
	PhoneNumber      *string `json:"phone_number"`
	SendReadReceipts bool    `json:"send_read_receipts"`
	AckReaction      bool    `json:"ack_reaction"`
	MediaCachePath   *string `json:"media_cache_path"`
	HistorySync      bool    `json:"history_sync"`
	Proxy            *string `json:"proxy"`
}

func DefaultWhatsAppWorkerAccountConfig() *WhatsAppWorkerAccountConfig {
	return &WhatsAppWorkerAccountConfig{
		AccountId:        "default",
		SessionPath:      "./session/default",
		DeviceName:       "OpenClaw",
		PairingMode:      "qr",
		PhoneNumber:      nil,
		SendReadReceipts: true,
		AckReaction:      false,
		MediaCachePath:   nil,
		HistorySync:      true,
		Proxy:            nil,
	}
}

// TeamsChannelConfig represents the MS Teams channel configuration.
type TeamsChannelConfig struct {
	Enabled                bool     `json:"enabled"`
	DmPolicy               string   `json:"dm_policy"`
	GroupPolicy            string   `json:"group_policy"`
	AppId                  *string  `json:"app_id"`
	AppIdRef               string   `json:"app_id_ref"`
	AppPassword            *string  `json:"app_password"`
	AppPasswordRef         string   `json:"app_password_ref"`
	TenantId               *string  `json:"tenant_id"`
	TenantIdRef            string   `json:"tenant_id_ref"`
	WebhookPath            string   `json:"webhook_path"`
	ValidateToken          bool     `json:"validate_token"`
	RequireMention         bool     `json:"require_mention"`
	ReplyStyle             string   `json:"reply_style"`
	TextChunkLimit         int      `json:"text_chunk_limit"`
	ChunkMode              string   `json:"chunk_mode"`
	MaxInboundChars        int      `json:"max_inbound_chars"`
	MaxRequestBytes        int      `json:"max_request_bytes"`
	AllowedTenantIds       []string `json:"allowed_tenant_ids"`
	AllowedFromIds         []string `json:"allowed_from_ids"`
	AllowedTeamIds         []string `json:"allowed_team_ids"`
	AllowedConversationIds []string `json:"allowed_conversation_ids"`
}

func DefaultTeamsChannelConfig() TeamsChannelConfig {
	return TeamsChannelConfig{
		Enabled:                false,
		DmPolicy:               "pairing",
		GroupPolicy:            "allowlist",
		AppIdRef:               "env:TEAMS_APP_ID",
		AppPasswordRef:         "env:TEAMS_APP_PASSWORD",
		TenantIdRef:            "env:TEAMS_TENANT_ID",
		WebhookPath:            "/api/messages",
		ValidateToken:          true,
		RequireMention:         true,
		ReplyStyle:             "thread",
		TextChunkLimit:         4000,
		ChunkMode:              "length",
		MaxInboundChars:        4096,
		MaxRequestBytes:        256 * 1024,
		AllowedTenantIds:       []string{},
		AllowedFromIds:         []string{},
		AllowedTeamIds:         []string{},
		AllowedConversationIds: []string{},
	}
}

// SmsChannelConfig represents the SMS channel configuration.
type SmsChannelConfig struct {
	DmPolicy string          `json:"dm_policy"`
	Twilio   TwilioSmsConfig `json:"twilio"`
}

func DefaultSmsChannelConfig() SmsChannelConfig {
	return SmsChannelConfig{
		DmPolicy: "pairing",
		Twilio:   DefaultTwilioSmsConfig(),
	}
}

// TwilioSmsConfig represents Twilio specific configuration.
type TwilioSmsConfig struct {
	Enabled                bool     `json:"enabled"`
	AccountSid             *string  `json:"account_sid"`
	AuthTokenRef           *string  `json:"auth_token_ref"`
	MessagingServiceSid    *string  `json:"messaging_service_sid"`
	FromNumber             *string  `json:"from_number"`
	WebhookPath            string   `json:"webhook_path"`
	WebhookPublicBaseUrl   *string  `json:"webhook_public_base_url"`
	ValidateSignature      bool     `json:"validate_signature"`
	AllowedFromNumbers     []string `json:"allowed_from_numbers"`
	AllowedToNumbers       []string `json:"allowed_to_numbers"`
	MaxInboundChars        int      `json:"max_inbound_chars"`
	MaxRequestBytes        int      `json:"max_request_bytes"`
	RateLimitPerFromPerMin int      `json:"rate_limit_per_from_per_minute"`
	AutoReplyForBlocked    bool     `json:"auto_reply_for_blocked"`
	HelpText               string   `json:"help_text"`
}

func DefaultTwilioSmsConfig() TwilioSmsConfig {
	return TwilioSmsConfig{
		Enabled:                false,
		WebhookPath:            "/twilio/sms/inbound",
		ValidateSignature:      true,
		AllowedFromNumbers:     []string{},
		AllowedToNumbers:       []string{},
		MaxInboundChars:        2000,
		MaxRequestBytes:        64 * 1024,
		RateLimitPerFromPerMin: 30,
		AutoReplyForBlocked:    false,
		HelpText:               "OpenClaw: reply STOP to opt out.",
	}
}

// TelegramChannelConfig represents the Telegram channel configuration.
type TelegramChannelConfig struct {
	Enabled               bool     `json:"enabled"`
	DmPolicy              string   `json:"dm_policy"`
	BotToken              *string  `json:"bot_token"`
	BotTokenRef           string   `json:"bot_token_ref"`
	WebhookPath           string   `json:"webhook_path"`
	WebhookPublicBaseUrl  *string  `json:"webhook_public_base_url"`
	AllowedFromUserIds    []string `json:"allowed_from_user_ids"`
	MaxInboundChars       int      `json:"max_inbound_chars"`
	MaxRequestBytes       int      `json:"max_request_bytes"`
	ValidateSignature     bool     `json:"validate_signature"`
	WebhookSecretToken    *string  `json:"webhook_secret_token"`
	WebhookSecretTokenRef string   `json:"webhook_secret_token_ref"`
}

func DefaultTelegramChannelConfig() TelegramChannelConfig {
	return TelegramChannelConfig{
		Enabled:               false,
		DmPolicy:              "pairing",
		BotTokenRef:           "env:TELEGRAM_BOT_TOKEN",
		WebhookPath:           "/telegram/inbound",
		AllowedFromUserIds:    []string{},
		MaxInboundChars:       4096,
		MaxRequestBytes:       64 * 1024,
		ValidateSignature:     false,
		WebhookSecretTokenRef: "env:TELEGRAM_WEBHOOK_SECRET",
	}
}

// SlackChannelConfig represents the Slack channel configuration.
type SlackChannelConfig struct {
	Enabled             bool     `json:"enabled"`
	DmPolicy            string   `json:"dm_policy"`
	BotToken            *string  `json:"bot_token"`
	BotTokenRef         string   `json:"bot_token_ref"`
	SigningSecret       *string  `json:"signing_secret"`
	SigningSecretRef    string   `json:"signing_secret_ref"`
	WebhookPath         string   `json:"webhook_path"`
	SlashCommandPath    string   `json:"slash_command_path"`
	AllowedWorkspaceIds []string `json:"allowed_workspace_ids"`
	AllowedFromUserIds  []string `json:"allowed_from_user_ids"`
	AllowedChannelIds   []string `json:"allowed_channel_ids"`
	MaxInboundChars     int      `json:"max_inbound_chars"`
	MaxRequestBytes     int      `json:"max_request_bytes"`
	ValidateSignature   bool     `json:"validate_signature"`
}

func DefaultSlackChannelConfig() SlackChannelConfig {
	return SlackChannelConfig{
		Enabled:             false,
		DmPolicy:            "pairing",
		BotTokenRef:         "env:SLACK_BOT_TOKEN",
		SigningSecretRef:    "env:SLACK_SIGNING_SECRET",
		WebhookPath:         "/slack/events",
		SlashCommandPath:    "/slack/commands",
		AllowedWorkspaceIds: []string{},
		AllowedFromUserIds:  []string{},
		AllowedChannelIds:   []string{},
		MaxInboundChars:     4096,
		MaxRequestBytes:     64 * 1024,
		ValidateSignature:   true,
	}
}

// DiscordChannelConfig represents the Discord channel configuration.
type DiscordChannelConfig struct {
	Enabled               bool     `json:"enabled"`
	DmPolicy              string   `json:"dm_policy"`
	BotToken              *string  `json:"bot_token"`
	BotTokenRef           string   `json:"bot_token_ref"`
	ApplicationId         *string  `json:"application_id"`
	ApplicationIdRef      string   `json:"application_id_ref"`
	PublicKey             *string  `json:"public_key"`
	PublicKeyRef          string   `json:"public_key_ref"`
	WebhookPath           string   `json:"webhook_path"`
	AllowedGuildIds       []string `json:"allowed_guild_ids"`
	AllowedFromUserIds    []string `json:"allowed_from_user_ids"`
	AllowedChannelIds     []string `json:"allowed_channel_ids"`
	MaxInboundChars       int      `json:"max_inbound_chars"`
	MaxRequestBytes       int      `json:"max_request_bytes"`
	ValidateSignature     bool     `json:"validate_signature"`
	RegisterSlashCommands bool     `json:"register_slash_commands"`
	SlashCommandPrefix    string   `json:"slash_command_prefix"`
}

func DefaultDiscordChannelConfig() DiscordChannelConfig {
	return DiscordChannelConfig{
		Enabled:               false,
		DmPolicy:              "pairing",
		BotTokenRef:           "env:DISCORD_BOT_TOKEN",
		ApplicationIdRef:      "env:DISCORD_APPLICATION_ID",
		PublicKeyRef:          "env:DISCORD_PUBLIC_KEY",
		WebhookPath:           "/discord/interactions",
		AllowedGuildIds:       []string{},
		AllowedFromUserIds:    []string{},
		AllowedChannelIds:     []string{},
		MaxInboundChars:       4096,
		MaxRequestBytes:       64 * 1024,
		ValidateSignature:     true,
		RegisterSlashCommands: true,
		SlashCommandPrefix:    "claw",
	}
}

// SignalChannelConfig represents the Signal channel configuration.
type SignalChannelConfig struct {
	Enabled               bool     `json:"enabled"`
	DmPolicy              string   `json:"dm_policy"`
	Driver                string   `json:"driver"`
	SocketPath            string   `json:"socket_path"`
	SignalCliPath         *string  `json:"signal_cli_path"`
	AccountPhoneNumber    *string  `json:"account_phone_number"`
	AccountPhoneNumberRef string   `json:"account_phone_number_ref"`
	AllowedFromNumbers    []string `json:"allowed_from_numbers"`
	MaxInboundChars       int      `json:"max_inbound_chars"`
	NoContentLogging      bool     `json:"no_content_logging"`
	TrustAllKeys          bool     `json:"trust_all_keys"`
}

func DefaultSignalChannelConfig() SignalChannelConfig {
	return SignalChannelConfig{
		Enabled:               false,
		DmPolicy:              "pairing",
		Driver:                "signald",
		SocketPath:            "/var/run/signald/signald.sock",
		AccountPhoneNumberRef: "env:SIGNAL_PHONE_NUMBER",
		AllowedFromNumbers:    []string{},
		MaxInboundChars:       4096,
		NoContentLogging:      false,
		TrustAllKeys:          true,
	}
}

// CronConfig represents the global Cron configuration.
type CronConfig struct {
	Enabled bool            `json:"enabled"`
	Jobs    []CronJobConfig `json:"jobs"`
}

func DefaultCronConfig() CronConfig {
	return CronConfig{
		Enabled: false,
		Jobs:    []CronJobConfig{},
	}
}

// CronJobConfig represents a specific cron job's configuration.
type CronJobConfig struct {
	Name                    string `json:"name"`
	CronExpression          string `json:"cron_expression"`
	Prompt                  string `json:"prompt"`
	RunOnStartup            bool   `json:"run_on_startup"`
	SessionId               string `json:"session_id"`
	ChannelId               string `json:"channel_id"`
	RecipientId             string `json:"recipient_id"`
	Subject                 string `json:"subject"`
	AutomationId            string `json:"automation_id"`
	AutomationTriggerSource string `json:"automation_trigger_source"`
	Timezone                string `json:"timezone"`
}

// WebhooksConfig represents the global webhooks configuration.
type WebhooksConfig struct {
	Enabled   bool                             `json:"enabled"`
	Endpoints map[string]WebhookEndpointConfig `json:"endpoints"`
}

func DefaultWebhooksConfig() WebhooksConfig {
	return WebhooksConfig{
		Enabled:   false,
		Endpoints: make(map[string]WebhookEndpointConfig),
	}
}

// WebhookEndpointConfig represents a specific webhook endpoint configuration.
type WebhookEndpointConfig struct {
	Secret          *string `json:"secret"`
	ValidateHmac    bool    `json:"validate_hmac"`
	HmacHeader      string  `json:"hmac_header"`
	SessionId       *string `json:"session_id"`
	PromptTemplate  string  `json:"prompt_template"`
	MaxRequestBytes int     `json:"max_request_bytes"`
	MaxBodyLength   int     `json:"max_body_length"`
}

func DefaultWebhookEndpointConfig() WebhookEndpointConfig {
	return WebhookEndpointConfig{
		ValidateHmac:    false,
		HmacHeader:      "X-Hub-Signature-256",
		PromptTemplate:  "Webhook received:\n\n{body}",
		MaxRequestBytes: 128 * 1024,
		MaxBodyLength:   10240,
	}
}

// ── Multi-Agent Routing ─────────────────────────────────────────

// ── RoutingConfig ───────────────────────────────────────────────

type RoutingConfig struct {
	Enabled bool                        `json:"enabled"`
	Routes  map[string]AgentRouteConfig `json:"routes"`
}

func DefaultRoutingConfig() RoutingConfig {
	return RoutingConfig{
		Enabled: false,
		Routes:  make(map[string]AgentRouteConfig),
	}
}

type AgentRouteConfig struct {
	ChannelId               *string                    `json:"channel_id"`
	SenderId                *string                    `json:"sender_id"`
	SystemPrompt            *string                    `json:"system_prompt"`
	ModelOverride           *string                    `json:"model_override"`
	ModelProfileId          *string                    `json:"model_profile_id"`
	PreferredModelTags      []string                   `json:"preferred_model_tags"`
	FallbackModelProfileIds []string                   `json:"fallback_model_profile_ids"`
	ModelRequirements       ModelSelectionRequirements `json:"model_requirements"`
	PresetId                *string                    `json:"preset_id"`
	AllowedTools            []string                   `json:"allowed_tools"`
}

func DefaultAgentRouteConfig() AgentRouteConfig {
	return AgentRouteConfig{
		PreferredModelTags:      []string{},
		FallbackModelProfileIds: []string{},
		AllowedTools:            []string{},
	}
}

// ── Deployment ──────────────────────────────────────────────────

type DeploymentConfig struct {
	Mode             string `json:"mode"`
	PublicExposure   bool   `json:"public_exposure"`
	ReverseProxy     string `json:"reverse_proxy"`
	ExpectedLocalUrl string `json:"expected_local_url"`
}

func DefaultDeploymentConfig() DeploymentConfig {
	return DeploymentConfig{
		Mode:           "local",
		PublicExposure: false,
	}
}

type TailscaleConfig struct {
	Enabled  bool    `json:"enabled"`
	Mode     string  `json:"mode"` // "off", "serve", "funnel"
	Port     int     `json:"port"`
	Hostname *string `json:"hostname"`
}

func DefaultTailscaleConfig() TailscaleConfig {
	return TailscaleConfig{
		Enabled: false,
		Mode:    "off",
		Port:    443,
	}
}

// ── Gmail Pub/Sub ───────────────────────────────────────────────

type GmailPubSubConfig struct {
	Enabled            bool    `json:"enabled"`
	CredentialsPath    *string `json:"credentials_path"`
	CredentialsPathRef string  `json:"credentials_path_ref"`
	TopicName          *string `json:"topic_name"`
	SubscriptionName   *string `json:"subscription_name"`
	WebhookPath        string  `json:"webhook_path"`
	SessionId          *string `json:"session_id"`
	Prompt             string  `json:"prompt"`
	WebhookSecret      *string `json:"webhook_secret"`
	WebhookSecretRef   string  `json:"webhook_secret_ref"`
}

func DefaultGmailPubSubConfig() GmailPubSubConfig {
	return GmailPubSubConfig{
		Enabled:            false,
		CredentialsPathRef: "env:GOOGLE_APPLICATION_CREDENTIALS",
		WebhookPath:        "/gmail/push",
		Prompt:             "A new email notification was received. Check inbox and triage.",
		WebhookSecretRef:   "env:GMAIL_PUBSUB_SECRET",
	}
}

// ── mDNS/Bonjour Discovery ─────────────────────────────────────

type MdnsConfig struct {
	Enabled      bool    `json:"enabled"`
	ServiceType  string  `json:"service_type"`
	InstanceName *string `json:"instance_name"`
	Port         int     `json:"port"` // 0 = use gateway port
}

func DefaultMdnsConfig() MdnsConfig {
	return MdnsConfig{
		Enabled:     false,
		ServiceType: "_openclaw._tcp",
		Port:        0,
	}
}
