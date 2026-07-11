package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type MaintenanceScanInputs struct {
	ConfigPath          string                        `json:"config_path"`
	SetupStatus         *SetupStatusResponse          `json:"setup_status"`
	ModelDoctor         *ModelSelectionDoctorResponse `json:"model_doctor"`
	RecentTurns         []TurnTokenUsageRecord        `json:"recent_turns"`
	ProviderRoutes      []ProviderRouteHealthSnapshot `json:"provider_routes"`
	AutomationRunStates []AutomationRunState          `json:"automation_run_states"`
	RuntimeMetrics      *MetricsSnapshot              `json:"runtime_metrics"`
	LoadedSkills        []SkillDefinition             `json:"loaded_skills"`
	ChannelDriftCount   int                           `json:"channel_drift_count"`
	PluginWarningCount  int                           `json:"plugin_warning_count"`
	PluginErrorCount    int                           `json:"plugin_error_count"`
}

type UpgradeRollbackSnapshotStore struct {
	rootPath     string
	manifestPath string
	payloadPath  string
}

func buildSnapshotKey(configPath string) string {
	var stem = getFileNameWithoutExtension(configPath)
	if isBlank(stem) {
		stem = "config"
	}

	var sb strings.Builder
	for _, ch := range stem {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			sb.WriteRune(unicode.ToLower(ch))
		} else {
			sb.WriteRune('-')
		}
	}

	safeStem := strings.Trim(sb.String(), "-")
	if isBlank(safeStem) {
		safeStem = "config"
	}

	hashBytes := sha256.Sum256([]byte(configPath))
	hashHex := hex.EncodeToString(hashBytes[:])
	shortHash := hashHex[:12]

	return fmt.Sprintf("%s-%s", safeStem, shortHash)
}

func NewUpgradeRollbackSnapshotStore(configPath string) *UpgradeRollbackSnapshotStore {
	normalizedConfigPath, _ := filepath.Abs(configPath)
	var key = buildSnapshotKey(normalizedConfigPath)
	store := &UpgradeRollbackSnapshotStore{}
	store.rootPath = filepath.Join(GatewaySetupPathsIntance.ResolveDefaultUpgradeSnapshotRootPath(), key)
	store.manifestPath = filepath.Join(store.rootPath, "snapshot.json")
	store.payloadPath = filepath.Join(store.rootPath, "payload")

	return store
}

func (u *UpgradeRollbackSnapshotStore) SnapshotDirectory() string {
	return u.rootPath
}

// ResolvePayloadPath 解析并安全校验 Payload 路径，防止路径穿越攻击（Directory Traversal）
func (s *UpgradeRollbackSnapshotStore) ResolvePayloadPath(relativePath string) (string, error) {
	normalized, err := normalizeRelativePath(relativePath, "payload")
	if err != nil {
		return "", err
	}

	fullPayloadPath, err := filepath.Abs(s.payloadPath)
	if err != nil {
		return "", err
	}

	combined := filepath.Clean(filepath.Join(fullPayloadPath, normalized))
	combinedAbs, err := filepath.Abs(combined)
	if err != nil {
		return "", err
	}

	if !isPathUnderRoot(combinedAbs, fullPayloadPath) {
		return "", fmt.Errorf("rollback snapshot payload path '%s' escapes the snapshot payload directory", relativePath)
	}

	return combinedAbs, nil
}

func (s *UpgradeRollbackSnapshotStore) Load() (*UpgradeRollbackSnapshot, error) {
	// 判断文件是否存在
	if _, err := os.Stat(s.manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("manifest file does not exist: %w", err)
	}

	data, err := os.ReadFile(s.manifestPath)
	if err != nil {
		return nil, fmt.Errorf("rollback snapshot manifest '%s' could not be read: %w", s.manifestPath, err)
	}

	var snapshot UpgradeRollbackSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("rollback snapshot manifest '%s' is corrupt or invalid JSON: %w", s.manifestPath, err)
	}

	return &snapshot, nil
}

// Save 保存快照和 Payload，采用“先写临时目录再原子替换”的策略
func (s *UpgradeRollbackSnapshotStore) Save(snapshot *UpgradeRollbackSnapshot, populatePayload func(string) error) error {
	parentDirectory := filepath.Dir(s.rootPath)
	if parentDirectory == "." || parentDirectory == "/" {
		return errors.New("snapshot root must contain a parent directory")
	}

	// 生成唯一临时目录名
	tempRoot, err := os.MkdirTemp(parentDirectory, "snapshot.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary root directory: %w", err)
	}
	// 确保发生异常或函数退出时清理临时目录
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	// 0700 代表仅当前用户可读写执行
	if err := os.MkdirAll(parentDirectory, 0700); err != nil {
		return err
	}
	if err := os.Chmod(tempRoot, 0700); err != nil {
		return err
	}

	tempPayload := filepath.Join(tempRoot, "payload")
	if err := os.MkdirAll(tempPayload, 0700); err != nil {
		return err
	}

	// 填充 Payload 业务数据
	if err := populatePayload(tempPayload); err != nil {
		return fmt.Errorf("failed to populate payload: %w", err)
	}

	// 序列化 Manifest
	manifestData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to serialize snapshot: %w", err)
	}

	manifestPath := filepath.Join(tempRoot, "snapshot.json")
	// 写入文件并限制权限为 0600（仅当前用户可读写）
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		return err
	}

	// 替换目录
	if err := replaceDirectory(tempRoot, s.rootPath); err != nil {
		return err
	}

	// 递归加固整个树的权限
	hardenSnapshotTree(s.rootPath)

	return nil
}

// 原子（或安全备份）替换目录
func replaceDirectory(source, destination string) error {
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		return os.Rename(source, destination)
	}

	// 目标已存在，创建备份目录
	backup, err := os.MkdirTemp(filepath.Dir(destination), "snapshot.*.bak")
	if err != nil {
		return err
	}
	_ = os.Remove(backup) // 移除创建的空文件夹，只需要这个唯一路径名进行重命名

	if err := os.Rename(destination, backup); err != nil {
		return err
	}

	if err := os.Rename(source, destination); err != nil {
		// 失败则回滚备份
		_ = os.Rename(backup, destination)
		return err
	}

	// 成功则删除备份
	_ = os.RemoveAll(backup)
	return nil
}

// 规范化相对路径校验
func normalizeRelativePath(path string, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("rollback snapshot %s path is missing", label)
	}

	if filepath.IsAbs(path) {
		return "", fmt.Errorf("rollback snapshot %s path '%s' must be relative", label, path)
	}

	// 统一替换分隔符并按分隔符拆分
	normalized := filepath.Clean(path)
	segments := strings.SplitSeq(normalized, string(filepath.Separator))

	for segment := range segments {
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("rollback snapshot %s path '%s' is invalid", label, path)
		}
	}

	return normalized, nil
}

// 判断 candidatePath 是否在 rootPath 之下
func isPathUnderRoot(candidatePath, rootPath string) bool {
	sep := string(filepath.Separator)
	r := filepath.Clean(rootPath)
	if !strings.HasSuffix(r, sep) {
		r += sep
	}
	c := filepath.Clean(candidatePath)
	return strings.HasPrefix(c, r)
}

func hardenSnapshotTree(rootPath string) {
	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略单项错误
		}
		if info.IsDir() {
			_ = os.Chmod(path, 0700)
		} else {
			mode := info.Mode()
			var restrictedMode os.FileMode = 0600 // 默认读写
			if mode&0111 != 0 {                   // 任意位有执行权限
				restrictedMode = 0700 // 赋予所有者读写执行权限
			}
			_ = os.Chmod(path, restrictedMode)
		}
		return nil
	})
}

const (
	MaintenanceHistoryRetention    = 20
	MaxModelEvaluationGroupsToKeep = 20
	PromptSizeWarningBytes         = 12_000
)

var ProtectedRetentionTags = map[string]struct{}{
	"keep":             {},
	"pinned":           {},
	"retain":           {},
	"retention-exempt": {},
}

type ReliabilityScorer struct{}

func (s *ReliabilityScorer) Build(
	config GatewayConfig,
	inputs *MaintenanceScanInputs,
	maintenanceReport *MaintenanceReportResponse,
	modelDoctor *ModelSelectionDoctorResponse,
	automationStates []AutomationRunState,
) ReliabilitySnapshot {

	if inputs == nil {
		inputs = &MaintenanceScanInputs{}
	}
	if modelDoctor == nil {
		if inputs.ModelDoctor != nil {
			modelDoctor = inputs.ModelDoctor
		} else {
			// 模拟 ModelDoctorEvaluator.Build(config)
			modelDoctor = &ModelSelectionDoctorResponse{}
		}
	}
	if automationStates == nil {
		automationStates = inputs.AutomationRunStates
	}

	var factors []ReliabilityFactor
	var recommendations []ReliabilityRecommendation

	factors = append(factors, s.buildReadinessFactor(config, inputs.SetupStatus, &recommendations, inputs.ConfigPath))
	factors = append(factors, s.buildModelFactor(*modelDoctor, inputs.ProviderRoutes, &recommendations))
	factors = append(factors, s.buildAutomationFactor(automationStates, &recommendations))
	factors = append(factors, s.buildMaintenanceFactor(maintenanceReport, &recommendations, inputs.ConfigPath))
	factors = append(factors, s.buildOperatorFactor(*inputs, &recommendations))

	var score int64 = 0
	for _, f := range factors {
		score += f.Score
	}

	var status string
	if score >= 90 {
		status = "healthy"
	} else if score >= 70 {
		status = "watch"
	} else {
		status = "action_needed"
	}

	return ReliabilitySnapshot{
		Score:           score,
		Status:          status,
		Factors:         factors,
		Recommendations: s.processRecommendations(recommendations),
	}
}

func (s *ReliabilityScorer) buildReadinessFactor(
	config GatewayConfig,
	setupStatus *SetupStatusResponse,
	recommendations *[]ReliabilityRecommendation,
	configPath string,
) ReliabilityFactor {
	var weight int64 = 25
	score := weight
	var findings []string

	publicBind := s.isNonLoopbackBind(config.BindAddress)
	if setupStatus != nil {
		publicBind = setupStatus.PublicBind
	}

	workspacePath := config.Tooling.WorkspaceRoot
	if setupStatus != nil && setupStatus.WorkspacePath != nil {
		workspacePath = *setupStatus.WorkspacePath
	}

	workspaceExists := false
	if setupStatus != nil {
		workspaceExists = setupStatus.WorkspaceExists
	} else {
		workspaceExists = !isBlank(workspacePath) && directoryExists(workspacePath)
	}

	providerConfigured := false
	if setupStatus != nil {
		providerConfigured = setupStatus.ProviderConfigured
	} else {
		providerConfigured = ProviderSmokeProbeInstance.IsProviderConfigured(config.Llm, nil)
	}

	if publicBind && strings.TrimSpace(config.AuthToken) == "" {
		score -= 10
		findings = append(findings, "Public bind is missing an auth token.")
		*recommendations = append(*recommendations, ReliabilityRecommendation{
			Id:       "verify-provider",
			Summary:  "Re-run setup verification with provider requirements.",
			Command:  s.buildConfigAwareCommand("openclaw setup verify --require-provider", configPath),
			Priority: 100,
		})
	}

	if !workspaceExists {
		score -= 8
		findings = append(findings, "Configured workspace path does not exist.")
		*recommendations = append(*recommendations, ReliabilityRecommendation{
			Id:       "verify-workspace",
			Summary:  "Confirm the configured workspace and setup posture.",
			Command:  s.buildConfigAwareCommand("openclaw setup status", configPath),
			Priority: 85,
		})
	}

	if !providerConfigured {
		score -= 7
		findings = append(findings, "No provider is configured.")
		*recommendations = append(*recommendations, ReliabilityRecommendation{
			Id:       "require-provider",
			Summary:  "Finish provider configuration before relying on the gateway.",
			Command:  s.buildConfigAwareCommand("openclaw setup verify --require-provider", configPath),
			Priority: 95,
		})
	}

	return s.buildFactor("readiness", "Readiness & posture", weight, score, findings)
}

func (s *ReliabilityScorer) buildModelFactor(
	modelDoctor ModelSelectionDoctorResponse,
	routes []ProviderRouteHealthSnapshot,
	recommendations *[]ReliabilityRecommendation,
) ReliabilityFactor {
	var weight int64 = 25
	score := weight
	var findings []string

	if len(modelDoctor.Errors) > 0 {
		score -= int64(min(15, len(modelDoctor.Errors)*5))
		findings = append(findings, fmt.Sprintf("%d model-doctor error(s) are unresolved.", len(modelDoctor.Errors)))
	}

	if len(modelDoctor.Warnings) > 0 {
		score -= int64(min(8, len(modelDoctor.Warnings)*2))
		findings = append(findings, fmt.Sprintf("%d model-doctor warning(s) are active.", len(modelDoctor.Warnings)))
	}

	compatibilityCount := 0
	for _, p := range modelDoctor.Profiles {
		if p.UsesCompatibilityTransport {
			compatibilityCount++
		}
	}
	if compatibilityCount > 0 {
		score -= int64(min(6, compatibilityCount*2))
		findings = append(findings, fmt.Sprintf("%d profile(s) still rely on compatibility transport.", compatibilityCount))
	}

	var routeErrors int64 = 0
	for _, r := range routes {
		routeErrors += r.Errors
	}
	if routeErrors > 0 {
		score -= min(6, routeErrors)
		findings = append(findings, fmt.Sprintf("%d recent provider route error(s) were recorded.", routeErrors))
	}

	if len(findings) > 0 {
		*recommendations = append(*recommendations, ReliabilityRecommendation{
			Id:       "model-doctor",
			Summary:  "Resolve model and provider routing issues.",
			Command:  "openclaw models doctor",
			Priority: 90,
		})
	}

	return s.buildFactor("model_health", "Model & provider health", weight, score, findings)
}

func (s *ReliabilityScorer) buildAutomationFactor(
	automationStates []AutomationRunState,
	recommendations *[]ReliabilityRecommendation,
) ReliabilityFactor {
	var weight int64 = 20
	score := weight
	var findings []string

	degraded := 0
	quarantined := 0
	for _, state := range automationStates {
		if strings.EqualFold(state.HealthState, "degraded") {
			degraded++
		} else if strings.EqualFold(state.HealthState, "quarantined") {
			quarantined++
		}
	}

	if degraded > 0 {
		score -= int64(min(8, degraded*2))
		findings = append(findings, fmt.Sprintf("%d automation(s) are degraded.", degraded))
	}

	if quarantined > 0 {
		score -= int64(min(12, quarantined*4))
		findings = append(findings, fmt.Sprintf("%d automation(s) are quarantined.", quarantined))
	}

	if len(findings) > 0 {
		priority := 70
		if quarantined > 0 {
			priority = 88
		}
		*recommendations = append(*recommendations, ReliabilityRecommendation{
			Id:       "maintenance-scan",
			Summary:  "Review automation health before trusting scheduled runs.",
			Command:  "openclaw maintenance scan",
			Priority: priority,
		})
	}

	return s.buildFactor("automation_health", "Automation health", weight, score, findings)
}

func (s *ReliabilityScorer) buildMaintenanceFactor(
	maintenanceReport *MaintenanceReportResponse,
	recommendations *[]ReliabilityRecommendation,
	configPath string,
) ReliabilityFactor {
	var weight int64 = 20
	score := weight
	var findings []string

	if maintenanceReport != nil {
		failCount := 0
		warnCount := 0
		for _, f := range maintenanceReport.Findings {
			if strings.EqualFold(f.Severity, "fail") {
				failCount++
			} else if strings.EqualFold(f.Severity, "warn") {
				warnCount++
			}
		}
		score -= int64(min(12, failCount*4))
		score -= int64(min(8, warnCount*2))

		if maintenanceReport.Storage.OrphanedSessionMetadataEntries > 0 {
			findings = append(findings, fmt.Sprintf("%d orphaned metadata entries remain.", maintenanceReport.Storage.OrphanedSessionMetadataEntries))
		}
		if maintenanceReport.Drift.RetentionFailures > 0 {
			findings = append(findings, fmt.Sprintf("%d retention failure(s) were observed.", maintenanceReport.Drift.RetentionFailures))
		}
		if maintenanceReport.Drift.PromptP95Delta > 0 {
			findings = append(findings, fmt.Sprintf("Prompt p95 drift is +%d tokens.", maintenanceReport.Drift.PromptP95Delta))
		}
	}

	if len(findings) > 0 {
		*recommendations = append(*recommendations, ReliabilityRecommendation{
			Id:       "maintenance-fix",
			Summary:  "Run a dry-run maintenance fix and apply only the safe cleanup you want.",
			Command:  s.buildConfigAwareCommand("openclaw maintenance fix --dry-run", configPath),
			Priority: 80,
		})
	}

	return s.buildFactor("maintenance_drift", "Maintenance & drift", weight, score, findings)
}

func (s *ReliabilityScorer) buildOperatorFactor(
	inputs MaintenanceScanInputs,
	recommendations *[]ReliabilityRecommendation,
) ReliabilityFactor {
	var weight int64 = 10
	score := weight
	var findings []string

	if inputs.PluginErrorCount > 0 {
		score -= int64(min(6, inputs.PluginErrorCount*2))
		findings = append(findings, fmt.Sprintf("%d plugin error(s) need operator review.", inputs.PluginErrorCount))
	}
	if inputs.PluginWarningCount > 0 {
		score -= int64(min(4, inputs.PluginWarningCount))
		findings = append(findings, fmt.Sprintf("%d plugin warning(s) are active.", inputs.PluginWarningCount))
	}
	if inputs.ChannelDriftCount > 0 {
		findings = append(findings, fmt.Sprintf("%d channel(s) are not fully ready.", inputs.ChannelDriftCount))
	}

	if len(findings) > 0 {
		*recommendations = append(*recommendations, ReliabilityRecommendation{
			Id:       "setup-status",
			Summary:  "Review setup posture and runtime hygiene before widening scope.",
			Command:  s.buildConfigAwareCommand("openclaw setup status", inputs.ConfigPath),
			Priority: 60,
		})
	}

	return s.buildFactor("operator_hygiene", "Operator & runtime hygiene", weight, score, findings)
}

func (s *ReliabilityScorer) buildFactor(id, label string, weight, score int64, findings []string) ReliabilityFactor {
	boundedScore := score
	if boundedScore < 0 {
		boundedScore = 0
	} else if boundedScore > weight {
		boundedScore = weight
	}

	var status string
	if boundedScore >= int64(math.Ceil(float64(weight)*0.9)) {
		status = "Healthy"
	} else if boundedScore >= int64(math.Ceil(float64(weight)*0.6)) {
		status = "Watch"
	} else {
		status = "ActionNeeded"
	}

	return ReliabilityFactor{
		Id:       id,
		Label:    label,
		Weight:   weight,
		Score:    boundedScore,
		Status:   status,
		Findings: findings,
	}
}

func (s *ReliabilityScorer) buildConfigAwareCommand(command, configPath string) string {
	if strings.TrimSpace(configPath) == "" {
		return command
	}

	quoted := GatewaySetupPathsIntance.QuoteIfNeeded(configPath)
	if strings.Contains(configPath, " ") {
		quoted = `"` + configPath + `"`
	}
	return fmt.Sprintf("%s --config %s", command, quoted)
}

func (s *ReliabilityScorer) isNonLoopbackBind(bindAddress string) bool {
	normalized := strings.ToLower(strings.TrimSpace(bindAddress))
	return len(normalized) > 0 &&
		normalized != "127.0.0.1" &&
		normalized != "localhost" &&
		normalized != "::1" &&
		normalized != "[::1]"
}

func (s *ReliabilityScorer) processRecommendations(recs []ReliabilityRecommendation) []ReliabilityRecommendation {
	groups := make(map[string][]ReliabilityRecommendation)
	for _, item := range recs {
		key := strings.ToLower(item.Id)
		groups[key] = append(groups[key], item)
	}

	var uniqueRecs []ReliabilityRecommendation
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Priority > group[j].Priority
		})
		uniqueRecs = append(uniqueRecs, group[0])
	}

	sort.Slice(uniqueRecs, func(i, j int) bool {
		return uniqueRecs[i].Priority > uniqueRecs[j].Priority
	})

	if len(uniqueRecs) > 6 {
		uniqueRecs = uniqueRecs[:6]
	}
	return uniqueRecs
}
