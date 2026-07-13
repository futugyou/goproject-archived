package core

import (
	"context"
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
	"time"
	"unicode"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	var stem = GetFileNameWithoutExtension(configPath)
	if IsBlank(stem) {
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
	if IsBlank(safeStem) {
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
		workspaceExists = !IsBlank(workspacePath) && DirectoryExists(workspacePath)
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

type MaintenanceCoordinator struct{}

func (m *MaintenanceCoordinator) createAutomationStore(config *GatewayConfig) (IAutomationStore, error) {
	switch config.Memory.Provider {
	case "sqlite":
		dppath := config.Memory.Sqlite.DbPath
		if !filepath.IsAbs(dppath) {
			storagePath, err := filepath.Abs(config.Memory.StoragePath)
			if err != nil {
				return nil, err
			}
			dppath = filepath.Join(storagePath, dppath)
		}

		fullpath, err := filepath.Abs(dppath)
		if err != nil {
			return nil, err
		}
		return NewSqliteFeatureStore(fullpath)
	case "postgres":
		db, err := gorm.Open(postgres.Open(config.Memory.Postgres.PostgresUrl), &gorm.Config{})
		if err != nil {
			return nil, err
		}

		return NewPostgresFeatureStore(db)
	}

	fullpath, err := filepath.Abs(config.Memory.StoragePath)
	if err != nil {
		return nil, err
	}
	return NewFileFeatureStore(fullpath)
}

func (m *MaintenanceCoordinator) createMemoryStore(config *GatewayConfig) (IMemoryStore, error) {
	switch config.Memory.Provider {
	case "sqlite":
		dppath := config.Memory.Sqlite.DbPath
		if !filepath.IsAbs(dppath) {
			storagePath, err := filepath.Abs(config.Memory.StoragePath)
			if err != nil {
				return nil, err
			}
			dppath = filepath.Join(storagePath, dppath)
		}

		fullpath, err := filepath.Abs(dppath)
		if err != nil {
			return nil, err
		}
		return NewSqliteMemoryStore(fullpath, config.Memory.Sqlite.EnableFts, nil, false, nil, nil)
	case "postgres":
		db, err := gorm.Open(postgres.Open(config.Memory.Postgres.PostgresUrl), &gorm.Config{})
		if err != nil {
			return nil, err
		}

		return NewPostgresMemoryStore(db, false, false, nil)
	}

	fullpath, err := filepath.Abs(config.Memory.StoragePath)
	if err != nil {
		return nil, err
	}
	return NewFileMemoryStore(fullpath, 0, nil, nil, nil)
}

func (m *MaintenanceCoordinator) resolveWorkspacePromptPath(workspaceRoot, fileName string) string {
	var dir string
	var err error
	if IsBlank(workspaceRoot) {
		dir, err = os.Getwd()
	} else {
		dir, err = filepath.Abs(workspaceRoot)
	}

	if err != nil {
		return ""
	}
	return filepath.Join(dir, fileName)
}

func (m *MaintenanceCoordinator) resolvePromptCacheTracePath(config *GatewayConfig, memoryRoot string) string {
	var raw = config.Llm.PromptCaching.TraceFilePath
	if IsBlankP(raw) {
		raw = config.Diagnostics.CacheTrace.FilePath
	}
	if IsBlankP(raw) {
		*raw = filepath.Join(memoryRoot, "logs", "cache-trace.jsonl")
	}

	return m.pathFor(*raw, memoryRoot)
}

func (m *MaintenanceCoordinator) pathFor(configuredPath, basePath string) string {
	path := ""
	if IsBlank(configuredPath) {
		path, _ = filepath.Abs(basePath)
		return path
	}

	if filepath.IsAbs(configuredPath) {
		path, _ = filepath.Abs(basePath)
	} else {
		path, _ = filepath.Abs(filepath.Join(basePath, configuredPath))
	}
	return path
}

func (m *MaintenanceCoordinator) isUnderRoot(path, root string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	// 确保 root 结尾有路径分隔符（例如 / 或 \）
	// filepath.FromSlash("/") 会根据系统自动转为正确的系统分隔符
	sep := string(filepath.Separator)
	rootWithSep := strings.TrimSuffix(absRoot, sep) + sep

	// 获取 path 的绝对路径并进行标准化清理
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// 情况 1: path 是 root 的子目录或子文件
	// 情况 2: path 和 root 是同一个目录
	return strings.HasPrefix(absPath, rootWithSep) || absPath == absRoot
}

func (m *MaintenanceCoordinator) prunePromptCacheTraceArtifacts(config *GatewayConfig, dryRun bool) *MaintenanceFixAction {
	memoryRoot, err := filepath.Abs(config.Memory.StoragePath)
	if err != nil {
		return nil
	}
	var path = m.resolvePromptCacheTracePath(config, memoryRoot)
	if !m.isUnderRoot(path, memoryRoot) {
		return nil
	}

	removable := []string{}
	if FileExists(path) {
		removable = []string{path}
	} else if DirectoryExists(path) {
		removable = EnumerateAllFiles(path)
	}

	if !dryRun {
		for _, file := range removable {
			os.Remove(file)
		}
	}

	result := &MaintenanceFixAction{
		Id:           "prompt-cache-traces",
		Applied:      !dryRun && len(removable) > 0,
		Summary:      "No managed prompt-cache trace artifacts were found",
		NumericValue: int64(len(removable)),
	}
	if len(removable) != 0 {
		if dryRun {
			result.Summary = fmt.Sprintf("would remove %d managed prompt-cache trace artifact(s)", len(removable))
		} else {

			result.Summary = fmt.Sprintf("removed %d managed prompt-cache trace artifact(s)", len(removable))
		}
	}
	return result
}

func (m *MaintenanceCoordinator) pruneModelEvaluationArtifacts(config *GatewayConfig, dryRun bool) *MaintenanceFixAction {
	path, err := filepath.Abs(config.Memory.StoragePath)
	if err != nil {
		return nil
	}
	path = filepath.Join(path, "admin", "model-evaluations")
	if !DirectoryExists(path) {
		return &MaintenanceFixAction{
			Id:      "model-evaluations",
			Summary: "No model evaluation artifacts were found.",
		}
	}

	groups, err := GetGroupByFilename(path)
	if err != nil {
		return nil
	}
	removable := []string{}
	if len(groups) > MaxModelEvaluationGroupsToKeep {
		for i := MaxModelEvaluationGroupsToKeep; i < len(groups); i++ {
			removable = append(removable, groups[i].Files...)
		}
	}

	if !dryRun {
		for _, file := range removable {
			os.Remove(file)
		}
	}

	result := &MaintenanceFixAction{
		Id:           "model-evaluations",
		Applied:      !dryRun && len(removable) > 0,
		Summary:      "Model evaluation artifacts are already within the retention window",
		NumericValue: int64(len(removable)),
	}
	if len(removable) != 0 {
		if dryRun {
			result.Summary = fmt.Sprintf("would remove %d old model evaluation artifact(s)", len(removable))
		} else {

			result.Summary = fmt.Sprintf("removed %d old model evaluation artifact(s)", len(removable))
		}
	}
	return result
}

func (m *MaintenanceCoordinator) loadPersistedSessionIds(ctx context.Context, config *GatewayConfig) (map[string]struct{}, error) {
	store, err := m.createMemoryStore(config)
	if err != nil {
		return nil, err
	}

	adminStore, ok := store.(ISessionAdminStore)
	if !ok {
		return map[string]struct{}{}, nil
	}

	sessionIds := map[string]struct{}{}

	for i := 0; i <= 100; i++ {

		result, err := adminStore.ListSessions(ctx, i, 500, &SessionListQuery{})
		if err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			sessionIds[item.Id] = struct{}{}
		}

		if !result.HasMore {
			break
		}
	}

	return sessionIds, nil
}

func (m *MaintenanceCoordinator) loadSessionMetadata(memoryRoot string) []SessionMetadataSnapshot {
	var path = filepath.Join(memoryRoot, "admin", "session-metadata.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return []SessionMetadataSnapshot{}
	}

	var result []SessionMetadataSnapshot

	json.Unmarshal(data, &result)
	return result
}

func (m *MaintenanceCoordinator) saveSessionMetadata(memoryRoot string, metadata []SessionMetadataSnapshot) error {
	var path = filepath.Join(memoryRoot, "admin", "session-metadata.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (m *MaintenanceCoordinator) pruneOrphanedMetadata(ctx context.Context, config *GatewayConfig, dryRun bool) (*MaintenanceFixAction, error) {
	memoryRoot, err := filepath.Abs(config.Memory.StoragePath)
	if err != nil {
		return nil, err
	}
	var metadata = m.loadSessionMetadata(memoryRoot)
	sessionIds, err := m.loadPersistedSessionIds(ctx, config)
	if err != nil {
		return nil, err
	}

	orphaned := []SessionMetadataSnapshot{}
	retained := []SessionMetadataSnapshot{}
	for _, item := range metadata {
		_, ok := sessionIds[item.SessionId]
		if !ok {
			orphaned = append(orphaned, item)
		} else {
			retained = append(retained, item)
		}
	}

	if !dryRun && len(orphaned) > 0 {
		m.saveSessionMetadata(memoryRoot, retained)
	}

	result := &MaintenanceFixAction{
		Id:           "metadata",
		Applied:      !dryRun && len(orphaned) > 0,
		Summary:      "No orphaned session metadata entries were found",
		NumericValue: int64(len(orphaned)),
	}

	msg := "ies"
	if len(orphaned) == 1 {
		msg = "y"
	}
	if len(orphaned) != 0 {
		if dryRun {
			result.Summary = fmt.Sprintf("would remove %d orphaned session metadata entr%s", len(orphaned), msg)
		} else {
			result.Summary = fmt.Sprintf("removed %d orphaned session metadata entr%s", len(orphaned), msg)
		}
	}
	return result, nil
}

func (m *MaintenanceCoordinator) runRetentionFix(ctx context.Context, config *GatewayConfig, dryRun bool) (*MaintenanceFixAction, error) {
	store, err := m.createMemoryStore(config)
	if err != nil {
		return nil, err
	}

	retentionStore, ok := store.(IMemoryRetentionStore)
	if !ok {
		return &MaintenanceFixAction{
			Id:      "retention",
			Summary: "Current memory store does not support retention sweeps.",
		}, nil
	}

	path, err := filepath.Abs(config.Memory.StoragePath)
	if err != nil {
		return nil, err
	}
	var metadata = m.loadSessionMetadata(path)
	protectedSessions := map[string]struct{}{}
	for _, item := range metadata {
		tagflag := false
		for _, tag := range item.Tags {
			if _, ok := ProtectedRetentionTags[tag]; ok {
				tagflag = true
				break
			}
		}
		if item.Starred || tagflag {
			protectedSessions[item.SessionId] = struct{}{}
		}
	}
	now := time.Now().UTC()
	request := &RetentionSweepRequest{
		DryRun:                  dryRun,
		NowUtc:                  now,
		SessionExpiresBeforeUtc: now.Add(-time.Hour * 24 * time.Duration(max(1, config.Memory.Retention.SessionTtlDays))),
		BranchExpiresBeforeUtc:  now.Add(-time.Hour * 24 * time.Duration(max(1, config.Memory.Retention.BranchTtlDays))),
		ArchivePath:             m.pathFor(config.Memory.Retention.ArchivePath, config.Memory.StoragePath),
		ArchiveEnabled:          config.Memory.Retention.ArchiveEnabled,
		ArchiveRetentionDays:    max(1, config.Memory.Retention.ArchiveRetentionDays),
		MaxItems:                max(10, config.Memory.Retention.MaxItemsPerSweep),
	}

	result, err := retentionStore.Sweep(ctx, request, protectedSessions)
	if err != nil {
		return nil, err
	}

	action := &MaintenanceFixAction{
		Id:           "retention",
		Applied:      !dryRun,
		NumericValue: int64(result.TotalEligible()),
	}

	if dryRun {
		action.Summary = fmt.Sprintf("retention dry-run found %d eligible item(s)", result.TotalEligible())
	} else {
		action.Summary = fmt.Sprintf("retention archived %d and deleted %d item(s)", result.TotalArchived(), result.TotalDeleted())
	}
	return action, nil
}

func (m *MaintenanceCoordinator) countPromptCacheTraceArtifacts(config *GatewayConfig, memoryRoot string) int {
	var path = m.resolvePromptCacheTracePath(config, memoryRoot)
	if !m.isUnderRoot(path, memoryRoot) {
		return 0
	}

	if FileExists(path) {
		return 1
	}

	if DirectoryExists(path) {
		return len(EnumerateAllFiles(path))
	}

	return 0
}

func (m *MaintenanceCoordinator) countModelEvaluationArtifacts(adminRoot string) int {
	var path = filepath.Join(adminRoot, "model-evaluations")
	if DirectoryExists(path) {
		return len(EnumerateTopFiles(path))
	}

	return 0
}
