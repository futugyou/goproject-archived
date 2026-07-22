package core

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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
