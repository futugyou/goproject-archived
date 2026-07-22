package core

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type SetupVerificationRequest struct {
	Config                          *GatewayConfig                `json:"config"`
	Policy                          *OrganizationPolicySnapshot   `json:"policy,omitempty"`
	OperatorAccountCount            int                           `json:"operator_account_count"`
	Offline                         bool                          `json:"offline"`
	RequireProvider                 bool                          `json:"require_provider"`
	WorkspacePath                   string                        `json:"workspace_path,omitempty"`
	ModelDoctor                     *ModelSelectionDoctorResponse `json:"model_doctor,omitempty"`
	ModelProfiles                   IModelProfileRegistry         `json:"model_profiles,omitempty"`
	ProviderSmokeRegistry           *ProviderSmokeRegistry        `json:"provider_smoke_registry,omitempty"`
	ConfigSources                   *ConfigSourceDiagnostics      `json:"config_sources,omitempty"`
	IncludeTailscaleServeCheck      bool                          `json:"include_tailscale_serve_check"`
	TailscaleIdentityHeadersPresent bool                          `json:"tailscale_identity_headers_present"`
}

type DoctorReportRequest struct {
	Config                          *GatewayConfig                `json:"config"`
	Policy                          *OrganizationPolicySnapshot   `json:"policy,omitempty"`
	OperatorAccountCount            int                           `json:"operator_account_count"`
	Offline                         bool                          `json:"offline"`
	RequireProvider                 bool                          `json:"require_provider"`
	CheckPortAvailability           bool                          `json:"check_port_availability"`
	WorkspacePath                   string                        `json:"workspace_path,omitempty"`
	ModelDoctor                     *ModelSelectionDoctorResponse `json:"model_doctor,omitempty"`
	ModelProfiles                   IModelProfileRegistry         `json:"model_profiles,omitempty"`
	ProviderSmokeRegistry           *ProviderSmokeRegistry        `json:"provider_smoke_registry,omitempty"`
	ConfigSources                   *ConfigSourceDiagnostics      `json:"config_sources,omitempty"`
	IncludeTailscaleServeCheck      bool                          `json:"include_tailscale_serve_check"`
	TailscaleIdentityHeadersPresent bool                          `json:"tailscale_identity_headers_present"`
}

var SetupVerificationCheckIds = map[string]struct{}{
	"config":             {},
	"workspace":          {},
	"security_posture":   {},
	"browser_capability": {},
	"operator_readiness": {},
	"model_doctor":       {},
	"provider_smoke":     {},
	"tailscale_serve":    {},
}

var SetupVerificationServiceInstance = &SetupVerificationService{}

type SetupVerificationService struct{}

func (s *SetupVerificationService) resolveConfiguredPath(value string) string {
	if IsBlank(value) {
		return ""
	}

	var resolved = SecretResolverInstance.Resolve(value)
	if IsBlank(resolved) {
		resolved = value
	}

	if filepath.IsAbs(resolved) {
		return resolved
	}

	p, err := filepath.Abs(resolved)
	if err != nil {
		return ""
	}
	return p
}

func (s *SetupVerificationService) isNodeAvailable() bool {
	var path = os.Getenv("PATH")
	if IsBlank(path) {
		return false
	}

	candidates := []string{"node"}
	if runtime.GOOS == "windows" {
		candidates = []string{"node.exe", "node.cmd", "node.bat"}
	}

	for dir := range strings.SplitSeq(filepath.Clean(path), string(filepath.Separator)) {
		for _, candidate := range candidates {

			if FileExists(filepath.Join(dir, candidate)) {
				return true
			}
		}
	}
	return false
}

func (s *SetupVerificationService) pingOpenSandbox(ctx context.Context, httpCLient *http.Client, config *GatewayConfig) bool {
	if config == nil || IsBlank(config.Sandbox.Endpoint) {
		return false
	}
	endpoint, err := url.Parse(config.Sandbox.Endpoint)
	if err != nil || !endpoint.IsAbs() {
		return false
	}

	// 2. Resolve API Key if prefixed
	apiKey := config.Sandbox.ApiKey
	if apiKey != "" {
		lowerKey := strings.ToLower(apiKey)
		if strings.HasPrefix(lowerKey, "env:") || strings.HasPrefix(lowerKey, "raw:") {
			apiKey = SecretResolverInstance.Resolve(apiKey)
		}
	}

	// 3. Construct Ping URL
	basePath := strings.TrimRight(endpoint.Path, "/")
	if !strings.HasSuffix(strings.ToLower(basePath), "/v1") {
		basePath += "/v1"
	}
	endpoint.Path = basePath + "/ping"

	// 4. Set up HTTP Client with 5-second timeout bound to context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return false
	}

	if apiKey != "" {
		req.Header.Set("OPEN-SANDBOX-API-KEY", apiKey)
	}
	resp, err := httpCLient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 5. Check for 2xx status code
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (s *SetupVerificationService) hasValidModelProfileConfiguration(config *GatewayConfig) bool {
	profileIds := make(map[string]bool)

	for _, profile := range config.Models.Profiles {
		trimmedId := strings.TrimSpace(profile.Id)
		if trimmedId == "" {
			return false
		}

		lowerId := strings.ToLower(trimmedId)
		if profileIds[lowerId] {
			return false
		}
		profileIds[lowerId] = true
	}

	if len(profileIds) == 0 {
		profileIds["default"] = true
	}

	trimmedDefaultProfile := strings.TrimSpace(config.Models.DefaultProfile)
	if trimmedDefaultProfile != "" && !profileIds[strings.ToLower(trimmedDefaultProfile)] {
		return false
	}

	for _, profile := range config.Models.Profiles {
		for _, fallbackId := range profile.FallbackProfileIds {
			trimmedFallback := strings.TrimSpace(fallbackId)
			if trimmedFallback != "" && !profileIds[strings.ToLower(trimmedFallback)] {
				return false
			}
		}
	}

	// 5. 校验 Routing 路由中的 ModelProfileId 与 FallbackModelProfileIds 是否合法
	for _, route := range config.Routing.Routes {
		trimmedRouteProfile := strings.TrimSpace(route.ModelProfileId)
		if trimmedRouteProfile != "" && !profileIds[strings.ToLower(trimmedRouteProfile)] {
			return false
		}

		for _, fallbackId := range route.FallbackModelProfileIds {
			trimmedFallback := strings.TrimSpace(fallbackId)
			if trimmedFallback != "" && !profileIds[strings.ToLower(fallbackId)] {
				return false
			}
		}
	}

	return true
}

func (s *SetupVerificationService) hasValidRootSet(roots []string) bool {
	wildcardCount := 0
	for _, v := range roots {
		if v == "*" {
			wildcardCount++
		}
	}

	if wildcardCount > 0 && len(roots) > wildcardCount {
		return false
	}

	for _, root := range roots {
		if root == "*" {
			continue
		}

		var resolved = s.resolveConfiguredPath(root)
		if IsBlank(resolved) || !filepath.IsAbs(resolved) {
			return false
		}
	}

	return true
}

func (s *SetupVerificationService) hasValidPromptCacheConfiguration(config *GatewayConfig) bool {
	requiresExplicitDialect := func(provider string) bool {
		return strings.EqualFold(provider, "openai-compatible") ||
			strings.EqualFold(provider, "groq") ||
			strings.EqualFold(provider, "together") ||
			strings.EqualFold(provider, "lmstudio")
	}

	supportsKeepWarm := func(provider, dialect string) bool {
		isAnthropicDialect := strings.EqualFold(dialect, "anthropic")
		isAnthropicProvider := strings.EqualFold(provider, "anthropic") ||
			strings.EqualFold(provider, "claude") ||
			strings.EqualFold(provider, "anthropic-vertex") ||
			strings.EqualFold(provider, "amazon-bedrock")

		isGeminiDialect := strings.EqualFold(dialect, "gemini")
		isGeminiProvider := strings.EqualFold(provider, "gemini") ||
			strings.EqualFold(provider, "google")

		return (isAnthropicDialect && isAnthropicProvider) || (isGeminiDialect && isGeminiProvider)
	}

	isValid := func(provider string, caching *PromptCachingConfig) bool {
		if caching == nil || caching.Enabled == nil || !*caching.Enabled {
			return true
		}

		dialect := "auto"
		if !IsBlank(caching.Dialect) {
			dialect = strings.TrimSpace(caching.Dialect)
		}

		if requiresExplicitDialect(provider) && strings.EqualFold(dialect, "auto") {
			return false
		}

		keepWarm := caching.KeepWarmEnabled != nil && *caching.KeepWarmEnabled
		return !keepWarm || supportsKeepWarm(provider, dialect)
	}

	if !isValid(config.Llm.Provider, config.Llm.PromptCaching) {
		return false
	}

	for _, profile := range config.Models.Profiles {
		if !isValid(profile.Provider, profile.PromptCaching) {
			return false
		}
	}

	return true
}

func (s *SetupVerificationService) normalizePortProbeHost(bindAddress string) string {
	if IsBlank(bindAddress) {
		return "127.0.0.1"
	}

	switch bindAddress {
	case "0.0.0.0":
		return "127.0.0.1"
	case "::", "[::]":
		return "::1"
	}

	return bindAddress
}

func (s *SetupVerificationService) toBoolWord(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func (s *SetupVerificationService) appendAllowlistLine(lines *[]string, channelId string, values []string) {
	if len(values) == 0 {
		return
	}
	*lines = append(*lines, fmt.Sprintf("- %s: %s", channelId, strings.Join(values, ", ")))
}

func (s *SetupVerificationService) buildStorageFreeSpaceCheck(config *GatewayConfig) *DoctorCheckItem {
	item := &DoctorCheckItem{
		Id:       "storage_free_space",
		Label:    "Storage free space",
		Category: "storage",
	}

	absPath, err := filepath.Abs(config.Memory.StoragePath)
	if err != nil {
		item.Status = "skip"
		item.Summary = "Storage free space could not be determined."
		return item
	}

	availableBytes, err := getDiskFreeSpace(absPath)
	if err != nil {
		item.Status = "skip"
		item.Summary = "Storage free space could not be determined."
		return item
	}

	const threshold = 100 * 1024 * 1024 // 100 MB

	if availableBytes > threshold {
		item.Status = "pass"
		item.Summary = "Storage volume has sufficient free space."
	} else {
		item.Status = "warn"
		item.Summary = "Storage volume has less than 100 MB free space."
		item.NextStep = "Free disk space before running heavier workloads."
	}

	return item
}

func (s *SetupVerificationService) buildPortAvailabilityCheck(config *GatewayConfig) *DoctorCheckItem {
	item := &DoctorCheckItem{
		Id:       "port_availability",
		Label:    "TCP port availability",
		Category: "network",
	}

	host := s.normalizePortProbeHost(config.BindAddress)
	address := net.JoinHostPort(host, strconv.Itoa(config.Port))

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err == nil {
		conn.Close()
		item.Status = "fail"
		item.Summary = "The configured TCP port is already in use."
		item.NextStep = "Free the port or change OpenClaw:Port before launching the gateway."
		return item
	}

	if netErr, ok := err.(*net.OpError); ok {
		_ = netErr
		item.Status = "pass"
		item.Summary = "The configured TCP port is available."
		return item
	}

	// 其他未知异常
	item.Status = "skip"
	item.Summary = "TCP port availability could not be determined."
	item.Detail = err.Error()
	return item
}

func (s *SetupVerificationService) buildRecommendedNextActionsFromDoctorChecks(checks []DoctorCheckItem) []string {
	seen := make(map[string]bool)
	var actions []string

	for _, check := range checks {
		if (check.Status == "fail" || check.Status == "warn") && check.NextStep != "" {
			if !seen[check.NextStep] {
				seen[check.NextStep] = true
				actions = append(actions, check.NextStep)
			}
		}
	}
	return actions
}

func (s *SetupVerificationService) buildRecommendedNextActionsFromSetupChecks(checks []SetupVerificationCheck) []string {
	seen := make(map[string]bool)
	var actions []string

	for _, check := range checks {
		if (check.Status == "fail" || check.Status == "warn") && check.NextStep != "" {
			if !seen[check.NextStep] {
				seen[check.NextStep] = true
				actions = append(actions, check.NextStep)
			}
		}
	}
	return actions
}

func (s *SetupVerificationService) buildBrowserCapabilityDetail(browser *BrowserToolCapabilitySummary) string {
	return fmt.Sprintf(
		"- browser_tool: configured=%s registered=%s local_supported=%s backend_configured=%s",
		s.toBoolWord(browser.ConfiguredEnabled),
		s.toBoolWord(browser.Registered),
		s.toBoolWord(browser.LocalExecutionSupported),
		s.toBoolWord(browser.ExecutionBackendConfigured),
	)
}

func (s *SetupVerificationService) buildChannelChecks(config *GatewayConfig) []DoctorCheckItem {
	var checks []DoctorCheckItem

	// MQTT Check
	if config.Plugins.Native.Mqtt.Enabled {
		if strings.TrimSpace(config.Plugins.Native.Mqtt.Host) != "" && config.Plugins.Native.Mqtt.Port > 0 {
			checks = append(checks, DoctorCheckItem{
				Id:       "mqtt_integration",
				Label:    "MQTT integration",
				Category: "channels",
				Status:   "pass",
				Summary:  "MQTT integration host and port are configured.",
			})
		} else {
			checks = append(checks, DoctorCheckItem{
				Id:       "mqtt_integration",
				Label:    "MQTT integration",
				Category: "channels",
				Status:   "warn",
				Summary:  "MQTT integration host or port is missing.",
				Detail:   "Set Plugins:Native:Mqtt:Host and a valid Port, or disable MQTT.",
				NextStep: "Configure MQTT host/port or disable MQTT.",
			})
		}
	}

	// Email Checks
	if config.Plugins.Native.Email.Enabled {
		// SMTP
		if strings.TrimSpace(config.Plugins.Native.Email.SmtpHost) != "" && config.Plugins.Native.Email.SmtpPort > 0 {
			checks = append(checks, DoctorCheckItem{
				Id:       "email_smtp",
				Label:    "Email SMTP integration",
				Category: "channels",
				Status:   "pass",
				Summary:  "Email SMTP settings are configured.",
			})
		} else {
			checks = append(checks, DoctorCheckItem{
				Id:       "email_smtp",
				Label:    "Email SMTP integration",
				Category: "channels",
				Status:   "warn",
				Summary:  "Email SMTP settings are incomplete.",
				Detail:   "Set Plugins:Native:Email:SmtpHost/SmtpPort for outbound mail.",
				NextStep: "Configure SMTP settings or disable the email integration.",
			})
		}

		// IMAP
		if strings.TrimSpace(config.Plugins.Native.Email.ImapHost) != "" && config.Plugins.Native.Email.ImapPort > 0 {
			checks = append(checks, DoctorCheckItem{
				Id:       "email_imap",
				Label:    "Email IMAP integration",
				Category: "channels",
				Status:   "pass",
				Summary:  "Email IMAP settings are configured.",
			})
		} else {
			checks = append(checks, DoctorCheckItem{
				Id:       "email_imap",
				Label:    "Email IMAP integration",
				Category: "channels",
				Status:   "warn",
				Summary:  "Email IMAP settings are incomplete.",
				Detail:   "Set Plugins:Native:Email:ImapHost/ImapPort for inbox monitoring.",
				NextStep: "Configure IMAP settings or disable the email integration.",
			})
		}
	}

	// Twilio SMS Check
	if config.Channels.Sms.Twilio.Enabled {
		if strings.TrimSpace(config.Channels.Sms.Twilio.AccountSid) != "" &&
			strings.TrimSpace(config.Channels.Sms.Twilio.AuthTokenRef) != "" {
			checks = append(checks, DoctorCheckItem{
				Id:       "twilio_sms",
				Label:    "Twilio SMS channel",
				Category: "channels",
				Status:   "pass",
				Summary:  "Twilio account SID and token reference are configured.",
			})
		} else {
			checks = append(checks, DoctorCheckItem{
				Id:       "twilio_sms",
				Label:    "Twilio SMS channel",
				Category: "channels",
				Status:   "warn",
				Summary:  "Twilio SMS credentials are incomplete.",
				NextStep: "Set Twilio AccountSid/AuthTokenRef or disable the SMS channel.",
			})
		}
	}

	// Telegram Check
	if config.Channels.Telegram.Enabled {
		if strings.TrimSpace(config.Channels.Telegram.BotTokenRef) != "" ||
			strings.TrimSpace(config.Channels.Telegram.BotToken) != "" {
			checks = append(checks, DoctorCheckItem{
				Id:       "telegram_channel",
				Label:    "Telegram channel",
				Category: "channels",
				Status:   "pass",
				Summary:  "Telegram bot token is configured.",
			})
		} else {
			checks = append(checks, DoctorCheckItem{
				Id:       "telegram_channel",
				Label:    "Telegram channel",
				Category: "channels",
				Status:   "warn",
				Summary:  "Telegram bot token is missing.",
				NextStep: "Set BotToken or BotTokenRef, or disable Telegram.",
			})
		}
	}

	// Allowlists Check
	var allowlistLines []string
	s.appendAllowlistLine(&allowlistLines, "teams", config.Channels.Teams.AllowedFromIds)
	s.appendAllowlistLine(&allowlistLines, "slack", config.Channels.Slack.AllowedFromUserIds)
	s.appendAllowlistLine(&allowlistLines, "discord", config.Channels.Discord.AllowedFromUserIds)
	s.appendAllowlistLine(&allowlistLines, "signal", config.Channels.Signal.AllowedFromNumbers)
	s.appendAllowlistLine(&allowlistLines, "telegram", config.Channels.Telegram.AllowedFromUserIds)
	s.appendAllowlistLine(&allowlistLines, "whatsapp", config.Channels.WhatsApp.AllowedFromIds)
	s.appendAllowlistLine(&allowlistLines, "sms", config.Channels.Sms.Twilio.AllowedFromNumbers)

	if len(allowlistLines) > 0 {
		checks = append(checks, DoctorCheckItem{
			Id:       "channel_allowlists",
			Label:    "Configured channel allowlists",
			Category: "channels",
			Status:   "pass",
			Summary:  "Static sender allowlists are configured for one or more channels.",
			Detail:   strings.Join(allowlistLines, "\n"),
		})
	}

	return checks
}

func (s *SetupVerificationService) buildOpenSandboxChecks(ctx context.Context, client *http.Client, config *GatewayConfig, offline bool) ([]DoctorCheckItem, error) {
	if !IsOpenSandboxProviderConfigured(config) {
		return []DoctorCheckItem{
			{
				Id:       "opensandbox",
				Label:    "OpenSandbox connectivity",
				Category: "network",
				Status:   "skip",
				Summary:  "OpenSandbox is not configured.",
			},
		}, nil
	}

	checks := make([]DoctorCheckItem, 0, 2)

	// 验证绝对 URL
	parsedURL, err := url.ParseRequestURI(config.Sandbox.Endpoint)
	isValidURL := err == nil && parsedURL.IsAbs()

	if isValidURL {
		checks = append(checks, DoctorCheckItem{
			Id:       "opensandbox_endpoint",
			Label:    "OpenSandbox endpoint",
			Category: "network",
			Status:   "pass",
			Summary:  "OpenSandbox endpoint is configured.",
		})
	} else {
		checks = append(checks, DoctorCheckItem{
			Id:       "opensandbox_endpoint",
			Label:    "OpenSandbox endpoint",
			Category: "network",
			Status:   "fail",
			Summary:  "OpenSandbox endpoint is missing or invalid.",
			NextStep: "Set OpenClaw:Sandbox:Endpoint to a valid absolute URL.",
		})
		return checks, nil
	}

	// 离线模式处理
	if offline {
		checks = append(checks, DoctorCheckItem{
			Id:       "opensandbox_reachability",
			Label:    "OpenSandbox reachability",
			Category: "network",
			Status:   "skip",
			Summary:  "OpenSandbox reachability was skipped because offline mode is enabled.",
		})
		return checks, nil
	}

	// Ping 连通性测试
	reachable := s.pingOpenSandbox(ctx, client, config)
	if reachable {
		checks = append(checks, DoctorCheckItem{
			Id:       "opensandbox_reachability",
			Label:    "OpenSandbox reachability",
			Category: "network",
			Status:   "pass",
			Summary:  "OpenSandbox endpoint is reachable.",
		})
	} else {
		checks = append(checks, DoctorCheckItem{
			Id:       "opensandbox_reachability",
			Label:    "OpenSandbox reachability",
			Category: "network",
			Status:   "fail",
			Summary:  "OpenSandbox endpoint is not reachable.",
			Detail:   "Verify OpenClaw:Sandbox:Endpoint and the OpenSandbox service/API key.",
			NextStep: "Fix the OpenSandbox endpoint or credentials before relying on sandbox-required tools.",
		})
	}

	return checks, nil
}

func (s *SetupVerificationService) buildStorageWritableCheck(config *GatewayConfig) *DoctorCheckItem {
	storagePath := config.Memory.StoragePath

	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return s.failStorageCheck()
	}

	testFile := filepath.Join(storagePath, ".doctor-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return s.failStorageCheck()
	}

	if err := os.Remove(testFile); err != nil {
		return s.failStorageCheck()
	}

	return &DoctorCheckItem{
		Id:       "storage_writable",
		Label:    "Storage path writability",
		Category: "storage",
		Status:   "pass",
		Summary:  "Storage path exists and is writable.",
	}
}

func (s *SetupVerificationService) failStorageCheck() *DoctorCheckItem {
	return &DoctorCheckItem{
		Id:       "storage_writable",
		Label:    "Storage path writability",
		Category: "storage",
		Status:   "fail",
		Summary:  "Storage path is not writable.",
		NextStep: "Fix Memory.StoragePath permissions before starting the gateway.",
	}
}

func (s *SetupVerificationService) buildModelDoctorCheck(response *ModelSelectionDoctorResponse) *DoctorCheckItem {
	status := s.GetModelDoctorStatus(response)

	if status == "fail" {
		// 模拟 C# LINQ 的 Take(8) 和 string.Join
		limit := len(response.Errors)
		if limit > 8 {
			limit = 8
		}

		formattedErrors := make([]string, limit)
		for i := 0; i < limit; i++ {
			formattedErrors[i] = fmt.Sprintf("- %v", response.Errors[i])
		}

		return &DoctorCheckItem{
			Id:       "model_doctor",
			Label:    "Model doctor",
			Category: "model_doctor",
			Status:   "fail",
			Summary:  fmt.Sprintf("Model doctor reported %d error(s).", len(response.Errors)),
			Detail:   strings.Join(formattedErrors, "\n"),
			NextStep: "Fix the model/provider configuration before using chat surfaces.",
		}
	}

	if status == "warn" {
		limit := len(response.Warnings)
		if limit > 8 {
			limit = 8
		}

		formattedWarnings := make([]string, limit)
		for i := 0; i < limit; i++ {
			formattedWarnings[i] = fmt.Sprintf("- %v", response.Warnings[i])
		}

		return &DoctorCheckItem{
			Id:       "model_doctor",
			Label:    "Model doctor",
			Category: "model_doctor",
			Status:   "warn",
			Summary:  fmt.Sprintf("Model doctor reported %d warning(s).", len(response.Warnings)),
			Detail:   strings.Join(formattedWarnings, "\n"),
		}
	}

	return &DoctorCheckItem{
		Id:       "model_doctor",
		Label:    "Model doctor",
		Category: "model_doctor",
		Status:   "pass",
		Summary:  "Model doctor did not find blocking configuration issues.",
	}
}

func (s *SetupVerificationService) buildProviderSmokeCheck(result *ProviderSmokeProbeResult, requireProvider bool) *DoctorCheckItem {
	if result.Status == "skip" && requireProvider {
		detail := result.Summary
		if strings.TrimSpace(result.Detail) != "" {
			detail = fmt.Sprintf("%s\n%s", result.Summary, result.Detail)
		}

		return &DoctorCheckItem{
			Id:       "provider_smoke",
			Label:    "Provider smoke",
			Category: "provider_smoke",
			Status:   "fail",
			Summary:  "Provider smoke was required but could not be completed.",
			Detail:   detail,
			NextStep: "Ensure credentials and network access are available, then rerun setup verify --require-provider.",
		}
	}

	nextStep := ""
	if result.Status == "fail" {
		nextStep = "Fix provider credentials/model/endpoint settings and rerun setup verify."
	}

	return &DoctorCheckItem{
		Id:       "provider_smoke",
		Label:    "Provider smoke",
		Category: "provider_smoke",
		Status:   result.Status,
		Summary:  result.Summary,
		Detail:   result.Detail,
		NextStep: nextStep,
	}
}

func (s *SetupVerificationService) buildPluginRuntimeCheck(config *GatewayConfig) *DoctorCheckItem {
	if !config.Plugins.DynamicNative.Enabled {
		return &DoctorCheckItem{
			Id:       "plugin_runtime",
			Label:    "Plugin runtime mode",
			Category: "plugins",
			Status:   "pass",
			Summary:  "Dynamic native plugins are disabled.",
		}
	}

	return &DoctorCheckItem{
		Id:       "plugin_runtime",
		Label:    "Plugin runtime mode",
		Category: "plugins",
		Status:   "fail",
		Summary:  "Dynamic native plugins require JIT mode.",
		Detail:   "Disable OpenClaw:Plugins:DynamicNative:Enabled or run a JIT-capable artifact / mode.",
		NextStep: "Disable dynamic native plugins or switch to a JIT-capable runtime mode.",
	}
}

func (s *SetupVerificationService) buildPluginDependencyCheck(config *GatewayConfig) *DoctorCheckItem {
	if !config.Plugins.Enabled {
		return &DoctorCheckItem{
			Id:       "plugin_bridge_dependency",
			Label:    "Bridge plugin host dependency",
			Category: "plugins",
			Status:   "pass",
			Summary:  "Bridge plugins are disabled.",
		}
	}

	if s.isNodeAvailable() {
		return &DoctorCheckItem{
			Id:       "plugin_bridge_dependency",
			Label:    "Bridge plugin host dependency",
			Category: "plugins",
			Status:   "pass",
			Summary:  "Node.js is available for bridge plugins.",
		}
	}

	return &DoctorCheckItem{
		Id:       "plugin_bridge_dependency",
		Label:    "Bridge plugin host dependency",
		Category: "plugins",
		Status:   "warn",
		Summary:  "Node.js is not available for bridge plugins.",
		Detail:   "Install Node.js or disable bridge plugins.",
		NextStep: "Install Node.js or disable bridge plugins.",
	}
}

func (s *SetupVerificationService) buildMcpChecks(config *GatewayConfig) []DoctorCheckItem {
	var checks []DoctorCheckItem

	if !config.Plugins.Mcp.Enabled {
		checks = append(checks, DoctorCheckItem{
			Id:       "mcp_servers",
			Label:    "MCP servers",
			Category: "plugins",
			Status:   "pass",
			Summary:  "MCP servers are disabled.",
		})
		return checks
	}

	for serverID, server := range config.Plugins.Mcp.Servers {
		if !server.Enabled {
			continue
		}

		transport := server.NormalizeTransport()
		if transport == "http" {
			// 验证是否为有效的绝对 URL
			parsedURL, err := url.ParseRequestURI(server.URL)
			isValidURL := err == nil && parsedURL.IsAbs()

			if isValidURL {
				checks = append(checks, DoctorCheckItem{
					Id:       fmt.Sprintf("mcp_server_%s", serverID),
					Label:    fmt.Sprintf("MCP server '%s'", serverID),
					Category: "plugins",
					Status:   "pass",
					Summary:  "HTTP MCP server URL is configured.",
				})
			} else {
				checks = append(checks, DoctorCheckItem{
					Id:       fmt.Sprintf("mcp_server_%s", serverID),
					Label:    fmt.Sprintf("MCP server '%s'", serverID),
					Category: "plugins",
					Status:   "warn",
					Summary:  "HTTP MCP server URL is missing or invalid.",
					Detail:   "Set a valid absolute HTTP(S) URL for the MCP server.",
					NextStep: fmt.Sprintf("Fix MCP server '%s' URL or disable the server.", serverID),
				})
			}
		} else {
			if strings.TrimSpace(server.Command) != "" {
				checks = append(checks, DoctorCheckItem{
					Id:       fmt.Sprintf("mcp_server_%s", serverID),
					Label:    fmt.Sprintf("MCP server '%s'", serverID),
					Category: "plugins",
					Status:   "pass",
					Summary:  "MCP server command is configured.",
				})
			} else {
				checks = append(checks, DoctorCheckItem{
					Id:       fmt.Sprintf("mcp_server_%s", serverID),
					Label:    fmt.Sprintf("MCP server '%s'", serverID),
					Category: "plugins",
					Status:   "warn",
					Summary:  "MCP server command is missing.",
					Detail:   "Set Plugins:Mcp:Servers:<id>:Command or disable the server.",
					NextStep: fmt.Sprintf("Fix MCP server '%s' command or disable the server.", serverID),
				})
			}
		}
	}

	return checks
}

func (s *SetupVerificationService) buildWorkspaceCheck(config *GatewayConfig, workspacePath string) *DoctorCheckItem {
	if !config.Tooling.WorkspaceOnly {
		return &DoctorCheckItem{
			Id:       "workspace",
			Label:    "Workspace readiness",
			Category: "workspace",
			Status:   "warn",
			Summary:  "Workspace-only mode is disabled, so filesystem access is broader than the first-run defaults.",
			Detail:   ("Re-enable Tooling.WorkspaceOnly if you want the safer local-first defaults."),
		}
	}

	if strings.TrimSpace(workspacePath) != "" {
		path := workspacePath
		if filepath.IsAbs(path) && DirectoryExists(path) {
			return &DoctorCheckItem{
				Id:       "workspace",
				Label:    "Workspace readiness",
				Category: "workspace",
				Status:   "pass",
				Summary:  fmt.Sprintf("Workspace is ready at '%s'.", path),
			}
		}
	}

	return &DoctorCheckItem{
		Id:       "workspace",
		Label:    "Workspace readiness",
		Category: "workspace",
		Status:   "fail",
		Summary:  "The configured workspace directory does not exist.",
		Detail:   workspacePath,
		NextStep: ("Create the workspace directory or update Tooling.WorkspaceRoot."),
	}
}

func (s *SetupVerificationService) buildFilesystemRootPolicyCheck(config *GatewayConfig) *DoctorCheckItem {
	if s.hasValidRootSet(config.Tooling.AllowedReadRoots) && s.hasValidRootSet(config.Tooling.AllowedWriteRoots) {
		return &DoctorCheckItem{
			Id:       "filesystem_root_policy",
			Label:    "Filesystem root policy",
			Category: "workspace",
			Status:   "pass",
			Summary:  "Filesystem roots are well-formed.",
		}
	}

	return &DoctorCheckItem{
		Id:       "filesystem_root_policy",
		Label:    "Filesystem root policy",
		Category: "workspace",
		Status:   "fail",
		Summary:  "Filesystem root policy is not well-formed.",
		Detail:   ("Do not mix '*' with explicit roots, and use absolute paths for explicit filesystem roots."),
		NextStep: ("Fix AllowedReadRoots/AllowedWriteRoots before exposing filesystem tools."),
	}
}

func (s *SetupVerificationService) buildSecurityPostureCheck(config *GatewayConfig, publicBind bool) *DoctorCheckItem {
	var issues []string
	var warnings []string

	if publicBind && strings.TrimSpace(config.AuthToken) == "" {
		issues = append(issues, "Public bind is enabled without an auth token.")
	}
	if publicBind && !config.Security.RequireRequesterMatchForHttpToolApproval {
		warnings = append(warnings, "Requester-matched HTTP tool approvals are disabled.")
	}
	if publicBind && config.Canvas.Enabled && !config.Canvas.AllowOnPublicBind {
		issues = append(issues, "Canvas command forwarding is enabled on a public bind without Canvas.AllowOnPublicBind.")
	}
	if publicBind && !config.Security.TrustForwardedHeaders {
		warnings = append(warnings, "Forwarded headers are not trusted, so browser session cookies may not be marked secure behind TLS termination.")
	}
	if publicBind && config.Tooling.AllowShell && !IsRequireSandboxed(config, "shell", ToolSandboxMode_Prefer) {
		warnings = append(warnings, "Shell is enabled on a public bind without required sandboxing.")
	}

	if len(issues) > 0 {
		var detail string
		if len(warnings) > 0 {
			formattedWarnings := make([]string, len(warnings))
			for i, w := range warnings {
				formattedWarnings[i] = "- " + w
			}
			detail = (strings.Join(formattedWarnings, "\n"))
		}

		return &DoctorCheckItem{
			Id:       "security_posture",
			Label:    "Security posture",
			Category: "security",
			Status:   "fail",
			Summary:  strings.Join(issues, " "),
			Detail:   detail,
			NextStep: ("Fix the public-bind security issues before treating the deployment as ready."),
		}
	}

	if len(warnings) > 0 {
		formattedWarnings := make([]string, len(warnings))
		for i, w := range warnings {
			formattedWarnings[i] = "- " + w
		}

		return &DoctorCheckItem{
			Id:       "security_posture",
			Label:    "Security posture",
			Category: "security",
			Status:   "warn",
			Summary:  warnings[0],
			Detail:   (strings.Join(formattedWarnings, "\n")),
		}
	}

	summary := "Loopback mode keeps the first-run surface local."
	if publicBind {
		summary = "Public-bind guardrails are in place."
	}

	return &DoctorCheckItem{
		Id:       "security_posture",
		Label:    "Security posture",
		Category: "security",
		Status:   "pass",
		Summary:  summary,
	}
}

func (s *SetupVerificationService) buildBrowserCheck(browser *BrowserToolCapabilitySummary) *DoctorCheckItem {
	if !browser.ConfiguredEnabled {
		return &DoctorCheckItem{
			Id:       "browser_capability",
			Label:    "Browser capability",
			Category: "browser",
			Status:   "pass",
			Summary:  "Browser tool is disabled.",
			Detail:   (s.buildBrowserCapabilityDetail(browser)),
		}
	}

	if browser.Registered {
		summary := "Browser tool is available in this runtime."
		if browser.ExecutionBackendConfigured && !browser.LocalExecutionSupported {
			summary = "Browser tool is available through a configured execution backend."
		}

		return &DoctorCheckItem{
			Id:       "browser_capability",
			Label:    "Browser capability",
			Category: "browser",
			Status:   "pass",
			Summary:  summary,
			Detail:   (s.buildBrowserCapabilityDetail(browser)),
		}
	}

	detailMsg := fmt.Sprintf("%s\nConfigure a non-local execution backend or sandbox for the browser tool, or disable Tooling.EnableBrowserTool.", s.buildBrowserCapabilityDetail(browser))

	return &DoctorCheckItem{
		Id:       "browser_capability",
		Label:    "Browser capability",
		Category: "browser",
		Status:   "fail",
		Summary:  "Browser tool is enabled but unavailable in this runtime.",
		Detail:   (detailMsg),
		NextStep: ("Configure a non-local execution backend or sandbox for the browser tool, or disable Tooling.EnableBrowserTool."),
	}
}

func (s *SetupVerificationService) buildOperatorReadinessCheck(publicBind bool, policy *OrganizationPolicySnapshot, operatorAccountCount int) *DoctorCheckItem {
	if !publicBind {
		return &DoctorCheckItem{
			Id:       "operator_readiness",
			Label:    "Operator readiness",
			Category: "operator",
			Status:   "pass",
			Summary:  "Loopback mode does not require named operator accounts for first run.",
		}
	}

	if operatorAccountCount <= 0 {
		return &DoctorCheckItem{
			Id:       "operator_readiness",
			Label:    "Operator readiness",
			Category: "operator",
			Status:   "warn",
			Summary:  "No named operator accounts exist yet.",
			Detail:   ("Create an admin account, sign in with a browser session, then retire the bootstrap token."),
			NextStep: ("Use the admin UI wizard to create the first admin operator account."),
		}
	}

	if policy.BootstrapTokenEnabled {
		return &DoctorCheckItem{
			Id:       "operator_readiness",
			Label:    "Operator readiness",
			Category: "operator",
			Status:   "warn",
			Summary:  "Bootstrap token is still enabled after operator accounts were created.",
			Detail:   ("Keep it only until account-token and browser-session authentication are confirmed."),
			NextStep: ("Disable the bootstrap token from the admin policy once browser sign-in works."),
		}
	}

	return &DoctorCheckItem{
		Id:       "operator_readiness",
		Label:    "Operator readiness",
		Category: "operator",
		Status:   "pass",
		Summary:  fmt.Sprintf("Operator accounts are configured (%d).", operatorAccountCount),
	}
}

func (s *SetupVerificationService) BuildRecommendedNextActions(
	config *GatewayConfig,
	policy *OrganizationPolicySnapshot,
	operatorAccountCount int,
	browser *BrowserToolCapabilitySummary,
	workspaceExists bool,
	publicBind bool,
	providerSmokeRegistry *ProviderSmokeRegistry,
) []string {
	var actions []string

	if !workspaceExists {
		actions = append(actions, "Create the configured workspace directory before using workspace-only file tools.")
	}

	if !ProviderSmokeProbeInstance.IsProviderConfigured(config.Llm, providerSmokeRegistry) {
		actions = append(actions, "Resolve the configured provider credentials before running live chat turns.")
	}

	if browser.ConfiguredEnabled && !browser.Registered {
		actions = append(actions, "Configure a non-local browser execution backend or disable the browser tool for this runtime.")
	}

	if publicBind && operatorAccountCount <= 0 {
		actions = append(actions, "Create a named admin operator account before exposing the admin UI publicly.")
	}

	if publicBind && policy.BootstrapTokenEnabled && operatorAccountCount > 0 {
		actions = append(actions, "Disable the bootstrap token after confirming account-token and browser-session login work.")
	}

	if publicBind {
		actions = append(actions, "Put the gateway behind TLS termination and a reverse proxy before Internet exposure.")
	}

	return DistinctStrings(actions)
}

func (s *SetupVerificationService) GetCheckStatus(snapshot *SetupVerificationSnapshot, checkID string) string {
	if snapshot == nil {
		return SetupCheckStatesNotRun
	}

	for _, item := range snapshot.Verification.Checks {
		if item.Id == checkID {
			return item.Status
		}
	}

	return SetupCheckStatesNotRun
}

func (s *SetupVerificationService) writeDoctorSection(sb *strings.Builder, title string, checks []DoctorCheckItem) {
	sb.WriteString(title)
	sb.WriteString("\n")
	if len(checks) == 0 {
		sb.WriteString("- none\n\n")
		return
	}

	for _, check := range checks {
		fmt.Fprintf(sb, "- [%s] %s/%s: %s -> %s\n", check.Status, check.Category, check.Id, check.Label, check.Summary)
		if strings.TrimSpace(check.Detail) != "" {
			fmt.Fprintf(sb, "  detail: %s\n", check.Detail)
		}
		if strings.TrimSpace(check.NextStep) != "" {
			fmt.Fprintf(sb, "  next_step: %s\n", check.NextStep)
		}
	}

	sb.WriteString("\n")
}

func (s *SetupVerificationService) buildConfigCheck(config *GatewayConfig) *DoctorCheckItem {
	errors := ConfigValidatorInstance.Validate(config)
	if len(errors) == 0 {
		return &DoctorCheckItem{
			Id:       "config",
			Label:    "Config and static validation",
			Category: DoctorCheckCategoriesConfig,
			Status:   SetupCheckStatesPass,
			Summary:  "Static configuration validation passed.",
		}
	}

	// Limit details to the first 8 errors
	limit := 8
	if len(errors) < limit {
		limit = len(errors)
	}

	var detailLines []string
	for _, err := range errors[:limit] {
		detailLines = append(detailLines, fmt.Sprintf("- %v", err))
	}

	return &DoctorCheckItem{
		Id:       "config",
		Label:    "Config and static validation",
		Category: DoctorCheckCategoriesConfig,
		Status:   SetupCheckStatesFail,
		Summary:  fmt.Sprintf("Static configuration validation found %d issue(s).", len(errors)),
		Detail:   strings.Join(detailLines, "\n"),
		NextStep: "Fix the configuration validation errors and rerun doctor or setup verify.",
	}
}

func (s *SetupVerificationService) buildConfigSourceDiagnosticsCheck(diagnostics *ConfigSourceDiagnostics) *DoctorCheckItem {
	if diagnostics == nil || len(diagnostics.Items) == 0 {
		return &DoctorCheckItem{
			Id:       "config_sources",
			Label:    "Effective configuration sources",
			Category: DoctorCheckCategoriesConfig,
			Status:   SetupCheckStatesSkip,
			Summary:  "Configuration source diagnostics were not supplied.",
		}
	}

	var detailLines []string
	for _, item := range diagnostics.Items {
		detailLines = append(detailLines, fmt.Sprintf("- %s: %s (source: %s)", item.Label, s.formatConfigDiagnosticValue(&item), item.Source))
	}

	return &DoctorCheckItem{
		Id:       "config_sources",
		Label:    "Effective configuration sources",
		Category: DoctorCheckCategoriesConfig,
		Status:   SetupCheckStatesPass,
		Summary:  "Effective bind, storage, provider, and secret-source winners are listed.",
		Detail:   strings.Join(detailLines, "\n"),
	}
}

func (s *SetupVerificationService) formatConfigDiagnosticValue(item *ConfigSourceDiagnosticItem) string {
	if item.Redacted {
		return "configured (redacted)"
	}
	return item.EffectiveValue
}

func (s *SetupVerificationService) buildPromptCacheCheck(config *GatewayConfig) *DoctorCheckItem {
	if s.hasValidPromptCacheConfiguration(config) {
		return &DoctorCheckItem{
			Id:       "prompt_cache",
			Label:    "Prompt cache compatibility",
			Category: DoctorCheckCategoriesConfig,
			Status:   SetupCheckStatesPass,
			Summary:  "Prompt cache settings are compatible with the configured providers.",
		}
	}

	return &DoctorCheckItem{
		Id:       "prompt_cache",
		Label:    "Prompt cache compatibility",
		Category: DoctorCheckCategoriesConfig,
		Status:   SetupCheckStatesWarn,
		Summary:  "Prompt cache settings are not compatible with the configured providers.",
		Detail:   "OpenAI-compatible and dynamic providers require an explicit cache dialect. Keep-warm is only supported for Anthropic-family and Gemini profiles.",
		NextStep: "Set an explicit prompt-cache dialect or disable unsupported keep-warm settings.",
	}
}

func (s *SetupVerificationService) buildModelProfileConsistencyCheck(config *GatewayConfig) *DoctorCheckItem {
	if s.hasValidModelProfileConfiguration(config) {
		return &DoctorCheckItem{
			Id:       "model_profile_configuration",
			Label:    "Model profile configuration",
			Category: DoctorCheckCategoriesModelDoctor,
			Status:   SetupCheckStatesPass,
			Summary:  "Model profile references are internally consistent.",
		}
	}

	return &DoctorCheckItem{
		Id:       "model_profile_configuration",
		Label:    "Model profile configuration",
		Category: DoctorCheckCategoriesModelDoctor,
		Status:   SetupCheckStatesFail,
		Summary:  "Model profile configuration is internally inconsistent.",
		Detail:   "Check Models.DefaultProfile, duplicate profile ids, route profile references, and fallback profile references.",
		NextStep: "Fix profile ids and fallback references before starting chat traffic.",
	}
}

func (s *SetupVerificationService) Verify(ctx context.Context, request *SetupVerificationRequest, client *http.Client) (*SetupVerificationResponse, error) {
	reportReq := &DoctorReportRequest{
		Config:                          request.Config,
		Policy:                          request.Policy,
		OperatorAccountCount:            request.OperatorAccountCount,
		Offline:                         request.Offline,
		RequireProvider:                 request.RequireProvider,
		CheckPortAvailability:           false,
		WorkspacePath:                   request.WorkspacePath,
		ModelDoctor:                     request.ModelDoctor,
		ModelProfiles:                   request.ModelProfiles,
		ProviderSmokeRegistry:           request.ProviderSmokeRegistry,
		ConfigSources:                   request.ConfigSources,
		IncludeTailscaleServeCheck:      request.IncludeTailscaleServeCheck,
		TailscaleIdentityHeadersPresent: request.TailscaleIdentityHeadersPresent,
	}

	report, err := s.BuildDoctorReport(ctx, reportReq, client)
	if err != nil {
		return nil, err
	}

	return s.BuildSetupVerificationResponse(report), nil
}

func (s *SetupVerificationService) BuildDoctorReport(ctx context.Context, request *DoctorReportRequest, client *http.Client) (*DoctorReportResponse, error) {
	config := request.Config
	publicBind := !IsLoopbackBind(config.BindAddress)

	policy := request.Policy
	if policy == nil {
		policy = &OrganizationPolicySnapshot{}
	}

	workspacePath := request.WorkspacePath
	if workspacePath == "" {
		workspacePath = config.Tooling.WorkspaceRoot
	}
	workspacePath = s.resolveConfiguredPath(workspacePath)

	browser := BrowserToolCapabilityEvaluatorInstance.Evaluate(config)

	modelDoctor := request.ModelDoctor
	if modelDoctor == nil {
		modelDoctor = ModelDoctorEvaluatorInstance.Build(config, request.ModelProfiles, nil)
	}

	checks := []DoctorCheckItem{
		*s.buildConfigCheck(config),
		*s.buildConfigSourceDiagnosticsCheck(request.ConfigSources),
		*s.buildPromptCacheCheck(config),
		*s.buildModelProfileConsistencyCheck(config),
		*s.buildWorkspaceCheck(config, workspacePath),
		*s.buildFilesystemRootPolicyCheck(config),
		*s.buildSecurityPostureCheck(config, publicBind),
		*s.buildBrowserCheck(browser),
		*s.buildOperatorReadinessCheck(publicBind, policy, request.OperatorAccountCount),
		*s.buildModelDoctorCheck(modelDoctor),
		*s.buildPluginRuntimeCheck(config),
		*s.buildPluginDependencyCheck(config),
	}

	checks = append(checks, s.buildMcpChecks(config)...)
	checks = append(checks, s.buildChannelChecks(config)...)

	sandboxChecks, err := s.buildOpenSandboxChecks(ctx, client, config, request.Offline)
	if err != nil {
		return nil, err
	}
	checks = append(checks, sandboxChecks...)

	tailscaleServeConfigured := TailscaleServeAdvisorInstance.IsTailscaleServeConfigured(config)
	tailscaleServe, err := TailscaleServeAdvisorInstance.BuildStatus(ctx, config, &TailscaleServeProbeOptions{
		ForceInclude:           request.IncludeTailscaleServeCheck,
		IdentityHeadersPresent: request.TailscaleIdentityHeadersPresent,
		CheckCli:               !request.Offline && (tailscaleServeConfigured || request.IncludeTailscaleServeCheck),
	})
	if err != nil {
		return nil, err
	}

	if tailscaleServe != nil {
		checks = append(checks, *TailscaleServeAdvisorInstance.BuildDoctorCheck(tailscaleServe, request.Offline))
	}

	checks = append(checks, *s.buildStorageWritableCheck(config))
	checks = append(checks, *s.buildStorageFreeSpaceCheck(config))

	if request.CheckPortAvailability {
		portCheck := s.buildPortAvailabilityCheck(config)
		checks = append(checks, *portCheck)
	}

	if request.Offline {
		checks = append(checks, DoctorCheckItem{
			Id:       "provider_smoke",
			Label:    "Provider smoke",
			Category: DoctorCheckCategoriesProviderSmoke,
			Status:   SetupCheckStatesSkip,
			Summary:  "Provider smoke skipped because offline mode is enabled.",
			NextStep: "Re-run without --offline when network and credentials are available.",
		})
	} else {
		providerProbe, err := ProviderSmokeProbeInstance.Probe(ctx, client, config.Llm, request.ProviderSmokeRegistry)
		if err != nil {
			return nil, err
		}
		checks = append(checks, *s.buildProviderSmokeCheck(providerProbe, request.RequireProvider))
	}

	var hasFailures, hasWarnings, hasSkips bool
	for _, item := range checks {
		switch item.Status {
		case SetupCheckStatesFail:
			hasFailures = true
		case SetupCheckStatesWarn:
			hasWarnings = true
		case SetupCheckStatesSkip:
			hasSkips = true
		}
	}

	overallStatus := SetupCheckStatesPass
	if hasFailures {
		overallStatus = SetupCheckStatesFail
	} else if hasWarnings {
		overallStatus = SetupCheckStatesWarn
	}

	return &DoctorReportResponse{
		OverallStatus:          overallStatus,
		HasFailures:            hasFailures,
		HasWarnings:            hasWarnings,
		HasSkips:               hasSkips,
		Checks:                 checks,
		RecommendedNextActions: s.buildRecommendedNextActionsFromDoctorChecks(checks),
		GeneratedAtUtc:         time.Now().UTC(),
	}, nil
}

func (s *SetupVerificationService) BuildSetupVerificationResponse(report *DoctorReportResponse) *SetupVerificationResponse {
	var filteredChecks []SetupVerificationCheck

	for _, item := range report.Checks {
		if _, ok := SetupVerificationCheckIds[item.Id]; ok {
			filteredChecks = append(filteredChecks, SetupVerificationCheck{
				Id:       item.Id,
				Label:    item.Label,
				Category: item.Category,
				Status:   item.Status,
				Summary:  item.Summary,
				Detail:   item.Detail,
				NextStep: item.NextStep,
			})
		}
	}

	var hasFailures, hasWarnings, hasSkips bool
	for _, item := range filteredChecks {
		switch item.Status {
		case SetupCheckStatesFail:
			hasFailures = true
		case SetupCheckStatesWarn:
			hasWarnings = true
		case SetupCheckStatesSkip:
			hasSkips = true
		}
	}

	overallStatus := SetupCheckStatesPass
	if hasFailures {
		overallStatus = SetupCheckStatesFail
	} else if hasWarnings {
		overallStatus = SetupCheckStatesWarn
	}

	return &SetupVerificationResponse{
		OverallStatus:          overallStatus,
		HasFailures:            hasFailures,
		HasWarnings:            hasWarnings,
		HasSkips:               hasSkips,
		Checks:                 filteredChecks,
		RecommendedNextActions: s.buildRecommendedNextActionsFromSetupChecks(filteredChecks),
	}
}

func (s *SetupVerificationService) RenderDoctorText(report *DoctorReportResponse) string {
	var sb strings.Builder

	sb.WriteString("OpenClaw Doctor\n")
	sb.WriteString(fmt.Sprintf("- generated_at_utc: %s\n", report.GeneratedAtUtc.Format(time.RFC3339Nano)))
	sb.WriteString(fmt.Sprintf("- overall_status: %s\n\n", report.OverallStatus))

	s.writeDoctorSection(&sb, "Failed checks", filterChecks(report.Checks, SetupCheckStatesFail))
	s.writeDoctorSection(&sb, "Warnings", filterChecks(report.Checks, SetupCheckStatesWarn))
	s.writeDoctorSection(&sb, "Skipped checks", filterChecks(report.Checks, SetupCheckStatesSkip))
	s.writeDoctorSection(&sb, "Passing checks", filterChecks(report.Checks, SetupCheckStatesPass))

	if len(report.RecommendedNextActions) > 0 {
		sb.WriteString("Recommended next actions\n")
		for _, action := range report.RecommendedNextActions {
			sb.WriteString(fmt.Sprintf("- %s\n", action))
		}
	}

	return strings.TrimRight(sb.String(), "\r\n\t ")
}

func (s *SetupVerificationService) GetModelDoctorStatus(response *ModelSelectionDoctorResponse) string {
	if response == nil {
		return SetupCheckStatesSkip
	}
	if len(response.Errors) > 0 {
		return SetupCheckStatesFail
	}
	if len(response.Warnings) > 0 {
		return SetupCheckStatesWarn
	}
	return SetupCheckStatesPass
}

func (s *SetupVerificationService) EvaluateBasicModelDoctor(config *GatewayConfig) *ModelSelectionDoctorResponse {
	return ModelDoctorEvaluatorInstance.Build(config, nil, nil)
}

func (s *SetupVerificationService) GetBootstrapGuidanceState(publicBind bool, bootstrapTokenEnabled bool, operatorAccountCount int) string {
	if !publicBind {
		return "not_applicable"
	}
	if operatorAccountCount <= 0 {
		return "create_first_operator"
	}
	if bootstrapTokenEnabled {
		return "disable_recommended"
	}
	return "complete"
}

func filterChecks(checks []DoctorCheckItem, status string) []DoctorCheckItem {
	var result []DoctorCheckItem
	for _, item := range checks {
		if item.Status == status {
			result = append(result, item)
		}
	}
	return result
}
