package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	p := &GatewaySetupPaths{}

	store := &UpgradeRollbackSnapshotStore{}
	store.rootPath = filepath.Join(p.ResolveDefaultUpgradeSnapshotRootPath(), key)
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

func resolveDefaultUpgradeSnapshotRootPath() string {
	return filepath.Join(os.TempDir(), "upgrade_snapshots")
}
