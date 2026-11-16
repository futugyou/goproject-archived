package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

func setupLifecycleCommandResolveConfigPath(parsed *CliArgs) (string, error) {
	configPath := util.Deref(parsed.GetOption("--config"))
	if configPath == "" {
		configPath = core.DefaultConfigPath
	}
	configPath = core.GatewaySetupPathsIntance.ExpandPath(configPath)
	return filepath.Abs(configPath)
}

func SetupLifecycleCommandRunVerify(args []string, output io.Writer, errwirter io.Writer) (int, error) {
	parsed := CliArgsParse(args)
	configPath, err := setupLifecycleCommandResolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(configPath)

	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 1, err
	}

	response, err := runLocalVerification(
		context.Background(),
		configPath,
		config,
		parsed.HasFlag("--offline"),
		parsed.HasFlag("--require-provider"),
		core.SetupVerificationSourcesCli)

	if err != nil {
		return 1, err
	}
	if parsed.HasFlag("--json") {
		data, _ := json.Marshal(response)
		fmt.Fprintln(output, string(data))
	} else {
		setupLifecycleWriteVerification(output, response)
	}

	if response.HasFailures {
		return 1, errors.New("")
	}
	return 0, nil
}

func setupLifecycleWriteVerification(output io.Writer, response *core.SetupVerificationResponse) {
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Setup verification:")
	for _, check := range response.Checks {
		fmt.Fprintf(output, "- [%s] %s: %s\n", check.Status, check.Label, check.Summary)
		if check.Detail != "" {
			fmt.Fprintf(output, "  \n", check.Detail)
		}
	}

	if len(response.RecommendedNextActions) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Verification next actions:")
		for _, action := range response.RecommendedNextActions {
			fmt.Fprintf(output, "- %s\n", action)
		}
	}

	fmt.Fprintln(output)
	fmt.Fprintf(output, "Verification result: %s\n", response.OverallStatus)
}

func runLocalVerification(ctx context.Context, configPath string, config *core.GatewayConfig, offline, requireProvider bool, source string) (*core.SetupVerificationResponse, error) {
	localState := core.LocalSetupStateLoaderInstance.Load(config.Memory.StoragePath)
	verification, err := core.SetupVerificationServiceInstance.Verify(ctx, &core.SetupVerificationRequest{
		Config:               config,
		Policy:               localState.Policy,
		OperatorAccountCount: localState.OperatorAccountCount,
		Offline:              offline,
		RequireProvider:      requireProvider,
		WorkspacePath:        resolveWorkspacePath(config),
		ModelDoctor:          core.ModelDoctorEvaluatorInstance.Build(config, nil, nil),
	}, nil)
	if err != nil {
		return nil, err
	}

	core.NewSetupVerificationSnapshotStore(config.Memory.StoragePath).Save(&core.SetupVerificationSnapshot{
		Source:          source,
		Offline:         offline,
		RequireProvider: requireProvider,
		Verification:    verification,
	})

	return verification, nil
}

func SetupLifecycleCommandRunStatus(args []string, output io.Writer, errwirter io.Writer) (int, error) {
	parsed := CliArgsParse(args)
	configPath, err := setupLifecycleCommandResolveConfigPath(parsed)
	if err != nil {
		return 1, err
	}

	config, err := core.GatewayConfigFileInstance.Load(configPath)

	if err != nil {
		fmt.Fprintln(errwirter, err.Error())
		return 1, err
	}

	status, err := setupLifecycleCommandBuildStatus(configPath, config, nil)
	if err != nil {
		return 1, err
	}
	setupLifecycleCommandWriteStatus(output, status)
	return 0, nil
}

func setupLifecycleCommandWriteStatus(output io.Writer, status *core.SetupStatusResponse) {
	fmt.Fprintf(output, "Profile: %s\n", status.Profile)
	fmt.Fprintf(output, "Bind: %s:%d\n", status.BindAddress, status.Port)
	fmt.Fprintf(output, "Reachable base URL: %s\n", status.ReachableBaseUrl)
	fmt.Fprintf(output, "Public bind: %t\n", status.PublicBind)
	fmt.Fprintf(output, "Auth token configured: %t\n", status.AuthTokenConfigured)
	fmt.Fprintf(output, "Bootstrap token enabled: %t\n", status.BootstrapTokenEnabled)
	fmt.Fprintf(output, "Allowed auth modes: %s\n", strings.Join(status.AllowedAuthModes, ", "))
	fmt.Fprintf(output, "Minimum plugin trust: %s\n", status.MinimumPluginTrustLevel)
	fmt.Fprintf(output, "Reverse proxy recommended: %t\n", status.ReverseProxyRecommended)
	fmt.Fprintf(output, "Workspace: %s exists=%t\n", status.WorkspacePath, status.WorkspaceExists)
	fmt.Fprintf(output, "Operator accounts: %d\n", status.OperatorAccountCount)
	fmt.Fprintf(output, "Provider configured: %t smoke=%s\n", status.ProviderConfigured, status.ProviderSmokeStatus)
	fmt.Fprintf(output, "Model doctor: %s\n", status.ModelDoctorStatus)
	fmt.Fprintf(output, "Reliability: %d/100 (%s)\n", status.Reliability.Score, status.Reliability.Status)
	fmt.Fprintf(output, "Browser tool: registered=%t backend_configured=%t reason=%s\n",
		status.BrowserToolRegistered, status.BrowserExecutionBackendConfigured, status.BrowserCapabilityReason)

	if status.LastVerificationAtUtc != nil {
		fmt.Fprintf(output, "Last verification: %s at %s via %s\n",
			status.LastVerificationStatus,
			status.LastVerificationAtUtc.Format(time.RFC3339),
			status.LastVerificationSource)
	} else {
		fmt.Fprintf(output, "Last verification: %s\n", core.SetupCheckStatesNotRun)
	}

	fmt.Fprintf(output, "Bootstrap guidance: %s\n", status.BootstrapGuidanceState)

	if status.TailscaleServe != nil {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Tailscale Serve:")
		fmt.Fprintf(output, "- Mode: %s\n", status.TailscaleServe.Mode)
		fmt.Fprintf(output, "- Local gateway: %s\n", status.TailscaleServe.LocalGatewayUrl)
		fmt.Fprintf(output, "- Suggested command: %s\n", status.TailscaleServe.SuggestedServeCommand)
		fmt.Fprintf(output, "- CLI detected: %t\n", status.TailscaleServe.TailscaleCliDetected)
		fmt.Fprintf(output, "- Tailnet reachability: %s\n", status.TailscaleServe.TailnetReachability)
		fmt.Fprintf(output, "- Serve detected: %s\n", status.TailscaleServe.ServeDetected)
		fmt.Fprintf(output, "- Identity headers present: %t\n", status.TailscaleServe.IdentityHeadersPresent)
		fmt.Fprintf(output, "- Public bind: %t\n", status.TailscaleServe.PublicBind)
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Artifacts:")
	for _, artifact := range status.Artifacts {
		fmt.Fprintf(output, "- %s: %s (%s)\n", artifact.Label, artifact.Status, artifact.Path)
	}

	if len(status.RecommendedNextActions) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Recommended next actions:")
		for _, action := range status.RecommendedNextActions {
			fmt.Fprintf(output, "- %s\n", action)
		}
	}

	if len(status.Reliability.Recommendations) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Reliability recommendations:")
		for _, recommendation := range status.Reliability.Recommendations {
			fmt.Fprintf(output, "- %s: %s\n", recommendation.Summary, recommendation.Command)
		}
	}

	if len(status.Warnings) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Warnings:")
		for _, warning := range status.Warnings {
			fmt.Fprintf(output, "- %s\n", warning)
		}
	}
}

func setupLifecycleCommandBuildStatus(configPath string, config *core.GatewayConfig, policy *core.OrganizationPolicySnapshot) (*core.SetupStatusResponse, error) {
	localState := core.LocalSetupStateLoaderInstance.Load(config.Memory.StoragePath)
	effectivePolicy := policy
	if effectivePolicy == nil {
		effectivePolicy = localState.Policy
	}
	if effectivePolicy == nil {
		effectivePolicy = new(core.DefaultOrganizationPolicySnapshot())
	}

	publicBind := !util.IsLoopbackBind(config.BindAddress)
	workspacePath := resolveWorkspacePath(config)
	workspaceExists := workspacePath != "" && util.DirectoryExists(workspacePath)
	browser := core.BrowserToolCapabilityEvaluatorInstance.Evaluate(config)
	modelDoctor := core.ModelDoctorEvaluatorInstance.Build(config, nil, nil)
	snapshot := localState.VerificationSnapshot
	warnings := []string{}
	if publicBind {
		warnings = append(warnings, "Reverse proxy and TLS are recommended for public bind deployments.")
	}

	if config.AuthToken == "" {
		warnings = append(warnings, "No auth token is configured.")
	}

	ctx := context.Background()
	tailscaleServe, err := core.TailscaleServeAdvisorInstance.BuildStatus(ctx, config, nil)
	if err != nil {
		return nil, err
	}

	profile := "tailscale-serve"
	if !core.TailscaleServeAdvisorInstance.IsTailscaleServeConfigured(config) {
		if publicBind {
			profile = "public"
		} else {
			profile = "local"
		}
	}

	deployDirectory, err := getDeployDirectory(configPath)
	if err != nil {
		return nil, err
	}

	status := &core.SetupStatusResponse{
		Profile:                           profile,
		BindAddress:                       config.BindAddress,
		Port:                              config.Port,
		PublicBind:                        publicBind,
		AuthTokenConfigured:               config.AuthToken != "",
		BootstrapTokenEnabled:             effectivePolicy.BootstrapTokenEnabled,
		AllowedAuthModes:                  effectivePolicy.AllowedAuthModes,
		MinimumPluginTrustLevel:           effectivePolicy.MinimumPluginTrustLevel,
		ReverseProxyRecommended:           publicBind,
		ReachableBaseUrl:                  buildReachableBaseUrl(config.BindAddress, config.Port),
		WorkspacePath:                     workspacePath,
		WorkspaceExists:                   workspaceExists,
		HasOperatorAccounts:               localState.OperatorAccountCount > 0,
		OperatorAccountCount:              localState.OperatorAccountCount,
		ProviderConfigured:                core.ProviderSmokeProbeInstance.IsProviderConfigured(config.Llm, nil),
		ProviderSmokeStatus:               core.SetupVerificationServiceInstance.GetCheckStatus(snapshot, "provider_smoke"),
		ModelDoctorStatus:                 core.SetupVerificationServiceInstance.GetModelDoctorStatus(modelDoctor),
		BrowserToolRegistered:             browser.Registered,
		BrowserExecutionBackendConfigured: browser.ExecutionBackendConfigured,
		BrowserCapabilityReason:           browser.Reason,
		BootstrapGuidanceState:            core.SetupVerificationServiceInstance.GetBootstrapGuidanceState(publicBind, effectivePolicy.BootstrapTokenEnabled, localState.OperatorAccountCount),
		RecommendedNextActions:            core.SetupVerificationServiceInstance.BuildRecommendedNextActions(config, effectivePolicy, localState.OperatorAccountCount, browser, workspaceExists, publicBind, nil),
		ChannelReadiness:                  []core.ChannelReadinessDto{},
		Artifacts:                         buildArtifactItems(deployDirectory),
		Warnings:                          warnings,
		TailscaleServe:                    tailscaleServe,
	}

	if snapshot != nil {
		status.LastVerificationAtUtc = new(snapshot.RecordedAtUtc)
		status.LastVerificationSource = snapshot.Source
		status.LastVerificationStatus = snapshot.Verification.OverallStatus
		status.LastVerificationHasFailures = snapshot.Verification.HasFailures
		status.LastVerificationHasWarnings = snapshot.Verification.HasWarnings
	}

	maintenance, err := core.MaintenanceCoordinatorInstance.Scan(ctx, config, &core.MaintenanceScanInputs{
		ConfigPath:  configPath,
		SetupStatus: status,
		ModelDoctor: modelDoctor,
	})

	if err != nil {
		return nil, err
	}
	return &core.SetupStatusResponse{
		Profile:                           status.Profile,
		BindAddress:                       status.BindAddress,
		Port:                              status.Port,
		PublicBind:                        status.PublicBind,
		AuthTokenConfigured:               status.AuthTokenConfigured,
		BootstrapTokenEnabled:             status.BootstrapTokenEnabled,
		AllowedAuthModes:                  status.AllowedAuthModes,
		MinimumPluginTrustLevel:           status.MinimumPluginTrustLevel,
		ReverseProxyRecommended:           status.ReverseProxyRecommended,
		ReachableBaseUrl:                  status.ReachableBaseUrl,
		WorkspacePath:                     status.WorkspacePath,
		WorkspaceExists:                   status.WorkspaceExists,
		HasOperatorAccounts:               status.HasOperatorAccounts,
		OperatorAccountCount:              status.OperatorAccountCount,
		ProviderConfigured:                status.ProviderConfigured,
		ProviderSmokeStatus:               status.ProviderSmokeStatus,
		ModelDoctorStatus:                 status.ModelDoctorStatus,
		BrowserToolRegistered:             status.BrowserToolRegistered,
		BrowserExecutionBackendConfigured: status.BrowserExecutionBackendConfigured,
		BrowserCapabilityReason:           status.BrowserCapabilityReason,
		LastVerificationAtUtc:             status.LastVerificationAtUtc,
		LastVerificationSource:            status.LastVerificationSource,
		LastVerificationStatus:            status.LastVerificationStatus,
		LastVerificationHasFailures:       status.LastVerificationHasFailures,
		LastVerificationHasWarnings:       status.LastVerificationHasWarnings,
		BootstrapGuidanceState:            status.BootstrapGuidanceState,
		RecommendedNextActions:            status.RecommendedNextActions,
		ChannelReadiness:                  status.ChannelReadiness,
		Artifacts:                         status.Artifacts,
		Warnings:                          status.Warnings,
		TailscaleServe:                    status.TailscaleServe,
		Reliability:                       maintenance.Reliability,
	}, nil
}

func buildArtifactItems(deployDirectory string) []core.SetupArtifactStatusItem {
	result := []core.SetupArtifactStatusItem{
		{
			Id:     "gateway-systemd",
			Label:  "Gateway systemd unit",
			Path:   filepath.Join(deployDirectory, "openclaw-gateway.service"),
			Status: "missing",
		},
		{
			Id:     "companion-systemd",
			Label:  "Companion systemd unit",
			Path:   filepath.Join(deployDirectory, "openclaw-companion.service"),
			Status: "missing",
		},
		{
			Id:     "gateway-launchd",
			Label:  "Gateway launchd plist",
			Path:   filepath.Join(deployDirectory, "ai.openclaw.gateway.plist"),
			Status: "missing",
		},
		{
			Id:     "companion-launchd",
			Label:  "Companion launchd plist",
			Path:   filepath.Join(deployDirectory, "ai.openclaw.companion.plist"),
			Status: "missing",
		},
		{
			Id:     "caddy",
			Label:  "Caddy reverse proxy recipe",
			Path:   filepath.Join(deployDirectory, "Caddyfile"),
			Status: "missing",
		},
	}

	for i := 0; i < len(result); i++ {
		exists := util.FileExists(result[i].Path)
		result[i].Exists = exists
		if exists {
			result[i].Status = "present"
		}
	}
	return result
}

func getDeployDirectory(configPath string) (string, error) {
	dir := filepath.Dir(configPath)
	if dir == "" {
		return "", errors.New("Config path must have a directory.")
	}

	return filepath.Join(dir, "deploy"), nil
}

func resolveWorkspacePath(config *core.GatewayConfig) string {
	workspace := config.Tooling.WorkspaceRoot
	if workspace == "" {
		return ""
	}

	resolved := core.SecretResolverInstance.Resolve(workspace)
	if resolved == "" {
		resolved = workspace
	}

	if filepath.IsAbs(resolved) {
		return resolved
	}

	resolved, _ = filepath.Abs(resolved)
	return resolved
}
