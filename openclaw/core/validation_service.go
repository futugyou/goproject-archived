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

func (s *SetupVerificationService) hasValidPromptCacheConfiguration(config GatewayConfig) bool {
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
