package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ProviderSmokeProbeResult struct {
	Status  string
	Summary string
	Detail  string
}

type ProbeFunc func(ctx context.Context, config LlmProviderConfig) (*ProviderSmokeProbeResult, error)

type ProviderSmokeRegistration struct {
	ProviderID        string
	Probe             ProbeFunc
	TreatAsConfigured bool
	SkipReason        string
}

type ProviderSmokeRegistry struct {
	mu            sync.RWMutex
	registrations map[string]ProviderSmokeRegistration
}

func NewProviderSmokeRegistry() *ProviderSmokeRegistry {
	return &ProviderSmokeRegistry{
		registrations: make(map[string]ProviderSmokeRegistration),
	}
}

func (r *ProviderSmokeRegistry) RegisterHandler(providerID string, probe ProbeFunc, treatAsConfigured bool) {
	normalized := r.normalize(providerID)
	if normalized == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.registrations[normalized] = ProviderSmokeRegistration{
		ProviderID:        normalized,
		Probe:             probe,
		TreatAsConfigured: treatAsConfigured,
	}
}

func (r *ProviderSmokeRegistry) RegisterMetadata(providerID string, treatAsConfigured bool, skipReason string) {
	normalized := r.normalize(providerID)
	if normalized == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.registrations[normalized] = ProviderSmokeRegistration{
		ProviderID:        normalized,
		TreatAsConfigured: treatAsConfigured,
		SkipReason:        skipReason,
	}
}

func (r *ProviderSmokeRegistry) TryGet(providerID string) (*ProviderSmokeRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, exists := r.registrations[r.normalize(providerID)]
	return &reg, exists
}

// 返回按 ProviderID 字母顺序排序后的切片副本
func (r *ProviderSmokeRegistry) Snapshot() []ProviderSmokeRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ProviderSmokeRegistration, 0, len(r.registrations))
	for _, reg := range r.registrations {
		result = append(result, reg)
	}

	// 按照 ProviderID 不区分大小写排序（由于存储时已经过存储标准化，直接比较即可）
	sort.Slice(result, func(i, j int) bool {
		return result[i].ProviderID < result[j].ProviderID
	})

	return result
}

// 内部标准化方法
func (r *ProviderSmokeRegistry) normalize(providerID string) string {
	return strings.ToLower(strings.TrimSpace(providerID))
}

const DefaultOllamaBaseUrl = "http://127.0.0.1:11434"

type OllamaResult struct {
	BaseUrl                   string
	UsesCompatibilityEndpoint bool
}

func OllamaNormalizeBaseUrl(endpoint string) string {
	return OllamaNormalize(endpoint).BaseUrl
}

func OllamaUsesCompatibilityEndpoint(endpoint string) bool {
	return OllamaNormalize(endpoint).UsesCompatibilityEndpoint
}

func OllamaNormalize(endpoint string) OllamaResult {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return OllamaResult{BaseUrl: DefaultOllamaBaseUrl, UsesCompatibilityEndpoint: false}
	}

	trimmed = strings.TrimRight(trimmed, "/")

	parsedUrl, err := url.Parse(trimmed)
	if err != nil || parsedUrl.Scheme == "" || parsedUrl.Host == "" {
		return OllamaResult{BaseUrl: trimmed, UsesCompatibilityEndpoint: false}
	}

	path := strings.TrimRight(parsedUrl.Path, "/")

	// 检查是否以 /v1 结尾（不区分大小写）
	if strings.EqualFold(path, "/v1") {
		parsedUrl.Path = ""
		parsedUrl.RawQuery = ""

		baseUrl := strings.TrimRight(parsedUrl.String(), "/")
		return OllamaResult{
			BaseUrl:                   baseUrl,
			UsesCompatibilityEndpoint: true,
		}
	}

	return OllamaResult{BaseUrl: trimmed, UsesCompatibilityEndpoint: false}
}

type SetupVerificationSnapshotStore struct {
	path string
}

func NewSetupVerificationSnapshotStore(storagePath string) *SetupVerificationSnapshotStore {
	rootedStorage := storagePath
	if !filepath.IsAbs(storagePath) {
		rootedStorage, _ = filepath.Abs(storagePath)
	}

	result := &SetupVerificationSnapshotStore{}
	result.path = filepath.Join(rootedStorage, "admin", "setup-verification.json")
	return result
}

func (s *SetupVerificationSnapshotStore) Load() *SetupVerificationSnapshot {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}

	var result SetupVerificationSnapshot

	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return &result
}

func (s *SetupVerificationSnapshotStore) Save(snapshot *SetupVerificationSnapshot) bool {
	directory := filepath.Dir(s.path)
	if !IsBlank(directory) {
		if err := os.MkdirAll(directory, 0755); err != nil {
			return false
		}
	}
	id := uuid.New()
	tempPath := s.path + "." + strings.ReplaceAll(id.String(), "-", "") + ".tmp"
	if err := SaveOneFile(context.Background(), tempPath, snapshot); err != nil {
		return false
	}

	return true
}

type LocalSetupStateSnapshot struct {
	OperatorAccountCount int
	Policy               *OrganizationPolicySnapshot
	VerificationSnapshot *SetupVerificationSnapshot
}

type LocalSetupStateLoader struct{}

func (l *LocalSetupStateLoader) readOrganizationPolicy(path string) *OrganizationPolicySnapshot {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var result OrganizationPolicySnapshot

	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return &result
}

func (l *LocalSetupStateLoader) readOperatorAccountCount(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	var result struct {
		Accounts []json.RawMessage `json:"accounts"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return 0
	}

	return len(result.Accounts)
}

func (l *LocalSetupStateLoader) Load(storagePath string) *LocalSetupStateSnapshot {
	rootedStorage := storagePath
	var err error
	if !filepath.IsAbs(storagePath) {
		if rootedStorage, err = filepath.Abs(storagePath); err != nil {
			return nil
		}
	}

	var adminDirectory = filepath.Join(rootedStorage, "admin")
	var operatorAccountsPath = filepath.Join(adminDirectory, "operator-accounts.json")
	var organizationPolicyPath = filepath.Join(adminDirectory, "organization-policy.json")

	store := NewSetupVerificationSnapshotStore(rootedStorage)

	return &LocalSetupStateSnapshot{
		OperatorAccountCount: l.readOperatorAccountCount(operatorAccountsPath),
		Policy:               l.readOrganizationPolicy(organizationPolicyPath),
		VerificationSnapshot: store.Load(),
	}
}

type TailscaleServeProbeOptions struct {
	ForceInclude           bool
	IdentityHeadersPresent bool
	CheckCli               bool
	CommandRunner          func(ctx context.Context, args string) TailscaleCommandResult
}

type TailscaleCommandResult struct {
	ExitCode int
	Output   string
	Error    string
}

var TailscaleServeAdvisorInstance = &TailscaleServeAdvisor{}

type TailscaleServeAdvisor struct{}

func (t *TailscaleServeAdvisor) IsTailscaleServeConfigured(config GatewayConfig) bool {
	return strings.EqualFold(config.Deployment.Mode, "tailscale-serve") ||
		strings.EqualFold(config.Deployment.ReverseProxy, "tailscale-serve") ||
		(config.Tailscale.Enabled && strings.EqualFold(config.Tailscale.Mode, "serve"))
}

func (t *TailscaleServeAdvisor) BuildLocalGatewayUrl(config GatewayConfig) string {
	if u, err := url.Parse(config.Deployment.ExpectedLocalUrl); err == nil && u.IsAbs() {
		if strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https") {
			return strings.TrimRight(u.String(), "/")
		}
	}

	if IsLoopbackBind(config.BindAddress) {
		return GatewaySetupArtifactsInstance.BuildReachableBaseUrl(config.BindAddress, config.Port)
	}

	return fmt.Sprintf("http://127.0.0.1:%d", config.Port)
}

func (t *TailscaleServeAdvisor) BuildSuggestedServeCommand(localGatewayUrl string) string {
	return fmt.Sprintf("tailscale serve --bg %s", strings.TrimRight(localGatewayUrl, "/"))
}

func (t *TailscaleServeAdvisor) BuildStatus(ctx context.Context, config GatewayConfig, options *TailscaleServeProbeOptions) (*TailscaleServeStatusResponse, error) {
	if options == nil {
		options = &TailscaleServeProbeOptions{CheckCli: true}
	}

	if !options.ForceInclude && !options.IdentityHeadersPresent && !t.IsTailscaleServeConfigured(config) {
		return nil, nil
	}

	localGatewayUrl := t.BuildLocalGatewayUrl(config)
	publicBind := !IsLoopbackBind(config.BindAddress)
	var warnings []string
	cliDetected := false
	tailnetReachability := "unknown"
	serveDetected := "unknown"

	if publicBind {
		warnings = append(warnings, "Gateway appears to be bound publicly. Tailscale Serve usually works best with loopback binding.")
	}

	if options.IdentityHeadersPresent {
		warnings = append(warnings, "Tailscale identity headers are not currently used for operator auth unless Tailscale auth is explicitly enabled.")
	}

	if options.CheckCli {
		runner := options.CommandRunner
		if runner == nil {
			runner = t.runTailscale
		}

		// 1. 探测 tailscale status
		statusResult := runner(ctx, "status")
		cliDetected = statusResult.ExitCode != -127

		if !cliDetected {
			warnings = append(warnings, "Tailscale CLI was not found. Install Tailscale or configure Serve manually.")
		} else {
			if statusResult.ExitCode == 0 {
				tailnetReachability = "ok"
			} else {
				tailnetReachability = "error"
				warnings = append(warnings, "Tailscale daemon status could not be confirmed. Run 'tailscale status' and verify this device is connected to the expected tailnet.")
			}

			// 2. 探测 tailscale serve status
			serveStatusResult := runner(ctx, "serve status")
			serveDetected = t.classifyServeStatus(serveStatusResult, localGatewayUrl)
			if serveDetected != "true" {
				warnings = append(warnings, "Tailscale Serve status could not be confirmed. Run 'tailscale serve status' after enabling Serve.")
			}
		}
	}

	mode := "detected-by-request"
	if t.IsTailscaleServeConfigured(config) {
		mode = "tailscale-serve"
	}

	return &TailscaleServeStatusResponse{
		Mode:                   mode,
		LocalGatewayUrl:        localGatewayUrl,
		SuggestedServeCommand:  t.BuildSuggestedServeCommand(localGatewayUrl),
		ServeDetected:          serveDetected,
		TailscaleCliDetected:   cliDetected,
		TailnetReachability:    tailnetReachability,
		IdentityHeadersPresent: options.IdentityHeadersPresent,
		PublicBind:             publicBind,
		Warnings:               warnings,
	}, nil
}

func (t *TailscaleServeAdvisor) BuildDoctorCheck(status *TailscaleServeStatusResponse, offline bool) DoctorCheckItem {
	item := DoctorCheckItem{
		Id:       "tailscale_serve",
		Label:    "Tailscale Serve advisory",
		Category: "Network",
		Detail:   t.buildStatusDetail(status),
	}

	if offline {
		item.Status = "Skip"
		item.Summary = "Tailscale Serve checks were skipped because offline mode is enabled."
		item.NextStep = "Re-run without --offline to inspect local Tailscale CLI and Serve status."
		return item
	}

	if len(status.Warnings) > 0 {
		item.Status = "Warn"
		item.Summary = "Tailscale Serve advisory checks found non-blocking warning(s)."
		item.NextStep = "Review the Tailscale Serve deployment guide and confirm the gateway stays loopback-bound."
		return item
	}

	item.Status = "Pass"
	item.Summary = "Tailscale Serve advisory checks did not find blocking issues."
	return item
}

func (t *TailscaleServeAdvisor) buildStatusDetail(status *TailscaleServeStatusResponse) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- mode: %s\n", status.Mode))
	sb.WriteString(fmt.Sprintf("- local_gateway: %s\n", status.LocalGatewayUrl))
	sb.WriteString(fmt.Sprintf("- serve_detected: %s\n", status.ServeDetected))
	sb.WriteString(fmt.Sprintf("- tailscale_cli_detected: %t\n", status.TailscaleCliDetected))
	sb.WriteString(fmt.Sprintf("- tailnet_reachability: %s\n", status.TailnetReachability))
	sb.WriteString(fmt.Sprintf("- identity_headers_present: %t\n", status.IdentityHeadersPresent))
	sb.WriteString(fmt.Sprintf("- public_bind: %t\n", status.PublicBind))
	sb.WriteString(fmt.Sprintf("- suggested_command: %s", status.SuggestedServeCommand))

	for _, warning := range status.Warnings {
		sb.WriteString(fmt.Sprintf("\n- warning: %s", warning))
	}
	return sb.String()
}

func (t *TailscaleServeAdvisor) classifyServeStatus(result TailscaleCommandResult, localGatewayUrl string) string {
	if result.ExitCode != 0 {
		return "unknown"
	}

	text := strings.TrimSpace(result.Output + "\n" + result.Error)
	if text == "" {
		return "unknown"
	}

	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "no serve") ||
		strings.Contains(lowerText, "not running") ||
		strings.Contains(lowerText, "not enabled") {
		return "false"
	}

	if t.serveStatusTargetsLocalGateway(text, localGatewayUrl) {
		return "true"
	}

	return "unknown"
}

func (t *TailscaleServeAdvisor) serveStatusTargetsLocalGateway(text string, localGatewayUrl string) bool {
	if strings.Contains(strings.ToLower(text), strings.ToLower(localGatewayUrl)) {
		return true
	}

	u, err := url.Parse(localGatewayUrl)
	if err != nil {
		return false
	}

	port := u.Port()
	if port == "" {
		return false
	}

	targets := []string{
		"127.0.0.1:" + port,
		"localhost:" + port,
		"[::1]:" + port,
	}

	lowerText := strings.ToLower(text)
	for _, target := range targets {
		if strings.Contains(lowerText, strings.ToLower(target)) {
			return true
		}
	}

	return false
}

func (t *TailscaleServeAdvisor) runTailscale(ctx context.Context, arguments string) TailscaleCommandResult {
	// 创建 3 秒硬超时的 Context
	subCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := strings.Fields(arguments)
	cmd := exec.CommandContext(subCtx, "tailscale", args...)

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// 1. 判断是否因找不到 CLI 工具报错 (对应 Win32Exception)
	if err != nil && errors.Is(err, exec.ErrNotFound) {
		return TailscaleCommandResult{
			ExitCode: -127,
			Error:    "tailscale command not found",
		}
	}

	// 2. 判断是否超时被杀 (对应 OperationCanceledException)
	if subCtx.Err() == context.DeadlineExceeded {
		return TailscaleCommandResult{
			ExitCode: 124,
			Error:    "tailscale command timed out",
		}
	}

	// 3. 正常执行完毕，提取 ExitCode
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// 其他非进程退出引发的错误（例如无权限等）
			return TailscaleCommandResult{
				ExitCode: 1,
				Error:    err.Error(),
			}
		}
	}

	return TailscaleCommandResult{
		ExitCode: exitCode,
		Output:   stdoutBuf.String(),
		Error:    stderrBuf.String(),
	}
}
