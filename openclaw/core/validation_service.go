package core

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
