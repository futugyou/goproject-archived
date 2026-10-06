package primitives

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type ProviderConfigShapeScenario struct {
	BaseScenario
}

func NewProviderConfigShapeScenario() *ProviderConfigShapeScenario {
	s := &ProviderConfigShapeScenario{
		BaseScenario: NewBaseScenario("providers.config_shape", "Provider config shape", HarnessRegressionCategoryProviders, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *ProviderConfigShapeScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		return HarnessRegressionScenarioResult{}, err
	}

	if regCtx.Config == nil {
		return s.Skipped("Provider shape was skipped because no config is loaded.", ""), nil
	}

	config := regCtx.Config
	var failures []string
	var notes []string
	provider := normalize(config.Llm.Provider)
	model := normalize(config.Llm.Model)

	if strings.TrimSpace(provider) == "" {
		failures = append(failures, "OpenClaw:Llm:Provider must be set.")
	}
	if strings.TrimSpace(model) == "" {
		failures = append(failures, "OpenClaw:Llm:Model must be set.")
	}
	if !isValidAuthMode(config.Llm.AuthMode) {
		failures = append(failures, "OpenClaw:Llm:AuthMode must be bearer or tailnet-identity.")
	}
	if requiresEndpoint(provider) && !isAbsoluteHTTPURL(config.Llm.Endpoint) {
		failures = append(failures, fmt.Sprintf("Provider '%s' requires an absolute http(s) endpoint.", provider))
	}

	for _, profile := range config.Models.Profiles {
		if strings.TrimSpace(profile.Id) == "" {
			failures = append(failures, "Models.Profiles[].Id must be set.")
		}
		if strings.TrimSpace(profile.Provider) == "" {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s.Provider must be set.", profile.Id))
		}
		if strings.TrimSpace(profile.Model) == "" {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s.Model must be set.", profile.Id))
		}
		if strings.TrimSpace(profile.AuthMode) != "" && !isValidAuthMode(profile.AuthMode) {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s.AuthMode must be bearer or tailnet-identity.", profile.Id))
		}

		baseURL := profile.BaseUrl
		if baseURL == "" {
			baseURL = config.Llm.Endpoint
		}
		if requiresEndpoint(normalize(profile.Provider)) && !isAbsoluteHTTPURL(derefString(&baseURL)) {
			failures = append(failures, fmt.Sprintf("Models.Profiles.%s requires an absolute http(s) base URL.", profile.Id))
		}
	}

	if regCtx.Offline {
		notes = append(notes, "Credential and network checks were skipped because offline mode is enabled.")
	}

	if len(failures) > 0 {
		return s.Failed("Provider/model shape has blocking issue(s).", HarnessRegressionScenarioTextJoin(failures), ""), nil
	}

	details := HarnessRegressionScenarioTextJoin(notes)

	return s.Passed("Provider/model shape is valid without external network calls.", details), nil
}

func isValidAuthMode(value string) bool {
	v := strings.TrimSpace(value)
	return v == "" ||
		strings.EqualFold(v, "bearer") ||
		strings.EqualFold(v, "tailnet-identity")
}

func requiresEndpoint(provider string) bool {
	switch provider {
	case "openai-compatible", "aperture", "groq", "together", "lmstudio", "anthropic-vertex", "amazon-bedrock", "azure-openai":
		return true
	default:
		return false
	}
}

func isAbsoluteHTTPURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || !u.IsAbs() {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
