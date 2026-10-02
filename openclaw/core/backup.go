package core

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// InstanceBackupPlan 备份计划定义
type InstanceBackupPlan struct {
	Roots            map[string]string `json:"Roots"`
	Categories       map[string]string `json:"Categories"`
	SecretReferences []string          `json:"SecretReferences"`
}

// InstanceBackupFile 单个文件的元数据
type InstanceBackupFile struct {
	Path   string `json:"Path"`
	Length int64  `json:"Length"`
	Sha256 string `json:"Sha256"`
}

// InstanceBackupManifest 备份清单结构
type InstanceBackupManifest struct {
	SchemaVersion int                  `json:"SchemaVersion"`
	CreatedAtUtc  time.Time            `json:"CreatedAtUtc"`
	Plan          InstanceBackupPlan   `json:"Plan"`
	Files         []InstanceBackupFile `json:"Files"`
}

const (
	maxBytes = 100 * 1024 * 1024 * 1024 // 100 GB
	maxFiles = 100_000
)

var requiredCategories = []string{"configuration", "sessions", "goals", "schedules", "governance"}

// 创建实例备份
func CreateBackup(plan InstanceBackupPlan, destination string, offline bool) error {
	if !offline {
		return errors.New("invalid operation: stop every writer and confirm offline mode before creating a backup")
	}

	if err := validatePlan(plan, true); err != nil {
		return err
	}

	destAbs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	destination = destAbs

	// 检查备份目标是否包含源根目录，或者源根目录包含备份目标
	for _, rawRoot := range plan.Roots {
		rootAbs, err := filepath.Abs(rawRoot)
		if err != nil {
			return err
		}
		if isWithin(rootAbs, destination) || isWithin(destination, rootAbs) {
			return errors.New("invalid operation: backup destination must be outside every source root")
		}
	}

	if err := ensureNewDestination(destination); err != nil {
		return err
	}

	staging := fmt.Sprintf("%s.staging-%s", destination, strings.ReplaceAll(uuid.New().String(), "-", ""))
	if err := ensureNewDestination(staging); err != nil {
		return err
	}

	defer func() {
		if _, err := os.Stat(staging); err == nil {
			_ = os.RemoveAll(staging)
		}
	}()

	if err := privateDirectory(staging); err != nil {
		return err
	}

	manifest := InstanceBackupManifest{
		SchemaVersion: 1,
		CreatedAtUtc:  time.Now().UTC(),
		Plan:          plan,
		Files:         make([]InstanceBackupFile, 0),
	}

	var totalBytes int64 = 0

	// 按 Key 排序处理 Root 目录
	rootKeys := make([]string, 0, len(plan.Roots))
	for k := range plan.Roots {
		rootKeys = append(rootKeys, k)
	}
	sort.Strings(rootKeys)

	for _, name := range rootKeys {
		rawRoot := plan.Roots[name]
		root, err := filepath.Abs(rawRoot)
		if err != nil {
			return err
		}

		files, err := enumerateSafe(root)
		if err != nil {
			return err
		}

		for _, file := range files {
			relPath, err := filepath.Rel(root, file)
			if err != nil {
				return err
			}
			relative := name + "/" + strings.ReplaceAll(relPath, "\\", "/")

			info, err := os.Stat(file)
			if err != nil {
				return err
			}
			length := info.Size()

			totalBytes += length
			if totalBytes > maxBytes || len(manifest.Files) >= maxFiles {
				return errors.New("invalid data: backup size limit exceeded")
			}

			payloadDir := filepath.Join(staging, "payload")
			target, err := safePath(payloadDir, relative)
			if err != nil {
				return err
			}

			if err := privateDirectory(filepath.Dir(target)); err != nil {
				return err
			}

			if err := copyPrivate(file, target); err != nil {
				return err
			}

			hash, err := digest(target)
			if err != nil {
				return err
			}

			manifest.Files = append(manifest.Files, InstanceBackupFile{
				Path:   relative,
				Length: length,
				Sha256: hash,
			})
		}
	}

	// 再次校验源文件是否有变更
	var currentPaths []string
	for name, rawRoot := range plan.Roots {
		root, _ := filepath.Abs(rawRoot)
		files, err := enumerateSafe(root)
		if err != nil {
			return err
		}
		for _, f := range files {
			rel, _ := filepath.Rel(root, f)
			currentPaths = append(currentPaths, name+"/"+strings.ReplaceAll(rel, "\\", "/"))
		}
	}
	sort.Strings(currentPaths)

	manifestPaths := make([]string, len(manifest.Files))
	for i, f := range manifest.Files {
		manifestPaths[i] = f.Path
	}
	sort.Strings(manifestPaths)

	if !sliceEqual(currentPaths, manifestPaths) {
		return errors.New("io error: source inventory changed during backup. Stop all writers and retry")
	}

	for _, file := range manifest.Files {
		parts := strings.SplitN(file.Path, "/", 2)
		rootPath, _ := filepath.Abs(plan.Roots[parts[0]])
		source, err := safePath(rootPath, parts[1])
		if err != nil {
			return err
		}

		info, err := os.Stat(source)
		if err != nil {
			return err
		}

		hash, err := digest(source)
		if err != nil {
			return err
		}

		if info.Size() != file.Length || hash != file.Sha256 {
			return errors.New("io error: source changed during backup. Stop all writers and retry")
		}
	}

	// 写入 manifest.json
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(staging, "manifest.json")
	if err := writePrivate(manifestPath, manifestData); err != nil {
		return err
	}

	if _, err := ValidateBackup(staging); err != nil {
		return err
	}

	return os.Rename(staging, destination)
}

// 校验备份合法性与完整性
func ValidateBackup(backup string) (*InstanceBackupManifest, error) {
	backup, err := filepath.Abs(backup)
	if err != nil {
		return nil, err
	}
	if err := rejectLinks(backup); err != nil {
		return nil, err
	}

	manifestPath, err := safePath(backup, "manifest.json")
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(manifestPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > 32*1024*1024 {
		return nil, errors.New("invalid data: manifest too large")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest InstanceBackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.New("invalid data: missing manifest")
	}

	if manifest.SchemaVersion != 1 || len(manifest.Files) > maxFiles {
		return nil, errors.New("invalid data: unsupported backup manifest")
	}

	if err := validatePlan(manifest.Plan, false); err != nil {
		return nil, err
	}

	payload := filepath.Join(backup, "payload")
	names := make(map[string]struct{})
	var totalBytes int64 = 0

	for _, file := range manifest.Files {
		if _, exists := names[file.Path]; exists {
			return nil, errors.New("invalid data: duplicate or invalid backup path")
		}
		names[file.Path] = struct{}{}

		rootKey := strings.Split(file.Path, "/")[0]
		if _, exists := manifest.Plan.Roots[rootKey]; !exists || file.Length < 0 {
			return nil, errors.New("invalid data: duplicate or invalid backup path")
		}

		totalBytes += file.Length
		if totalBytes > maxBytes {
			return nil, errors.New("invalid data: backup size limit exceeded")
		}

		path, err := safePath(payload, file.Path)
		if err != nil {
			return nil, err
		}

		fInfo, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		hash, err := digest(path)
		if err != nil {
			return nil, err
		}

		if fInfo.Size() != file.Length || hash != file.Sha256 {
			return nil, fmt.Errorf("invalid data: backup checksum mismatch: %s", file.Path)
		}
	}

	var actual []string
	if _, err := os.Stat(payload); err == nil {
		files, err := enumerateSafe(payload)
		if err != nil {
			return nil, err
		}
		for _, p := range files {
			rel, _ := filepath.Rel(payload, p)
			actual = append(actual, strings.ReplaceAll(rel, "\\", "/"))
		}
	}

	if len(actual) != len(names) {
		return nil, errors.New("invalid data: unexpected backup payload files")
	}
	for _, p := range actual {
		if _, ok := names[p]; !ok {
			return nil, errors.New("invalid data: unexpected backup payload files")
		}
	}

	return &manifest, nil
}

// 辅助方法与底层工具函数实现

func validatePlan(plan InstanceBackupPlan, requireSources bool) error {
	if len(plan.Roots) == 0 {
		return errors.New("invalid data: plan must map required categories")
	}

	for _, cat := range requiredCategories {
		root, exists := plan.Categories[cat]
		if !exists {
			return errors.New("invalid data: missing required category")
		}
		if _, rootExists := plan.Roots[root]; !rootExists {
			return errors.New("invalid data: category root non-existent in roots")
		}
	}

	for name, root := range plan.Roots {
		if len(name) == 0 || !isAlphaNumHyphenUnderscore(name) {
			return errors.New("invalid data: invalid root name")
		}
		if strings.TrimSpace(root) == "" {
			return errors.New("invalid data: missing root path")
		}
		if requireSources {
			if _, err := os.Stat(root); os.IsNotExist(err) {
				return fmt.Errorf("directory not found: %s", root)
			}
			absRoot, _ := filepath.Abs(root)
			if err := rejectLinks(absRoot); err != nil {
				return err
			}
		}
	}

	for _, r := range plan.SecretReferences {
		if strings.TrimSpace(r) == "" || !strings.Contains(r, ":") || strings.Contains(r, "\n") {
			return errors.New("invalid data: secret references must be provider-qualified references")
		}
	}

	return nil
}

func isAlphaNumHyphenUnderscore(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func enumerateSafe(root string) ([]string, error) {
	if err := rejectLinks(root); err != nil {
		return nil, err
	}

	var results []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	// 保证排序顺序
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		fullPath := filepath.Join(root, entry.Name())
		if err := rejectLinks(fullPath); err != nil {
			return nil, err
		}

		if entry.IsDir() {
			subFiles, err := enumerateSafe(fullPath)
			if err != nil {
				return nil, err
			}
			results = append(results, subFiles...)
		} else {
			results = append(results, fullPath)
		}
	}

	return results, nil
}

func isWithin(root, path string) bool {
	root = strings.TrimRight(filepath.Clean(root), string(filepath.Separator))
	path = filepath.Clean(path)

	if runtime.GOOS != "linux" {
		root = strings.ToLower(root)
		path = strings.ToLower(path)
	}

	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

func safePath(root, relative string) (string, error) {
	if strings.Contains(relative, "\\") || strings.Contains(relative, ":") || filepath.IsAbs(relative) {
		return "", errors.New("invalid data: unsafe backup path")
	}

	parts := strings.Split(relative, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", errors.New("invalid data: unsafe backup path")
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Clean(filepath.Join(absRoot, relative))
	if !isWithin(absRoot, fullPath) {
		return "", errors.New("invalid data: backup path escapes payload")
	}

	if err := rejectLinks(fullPath); err != nil {
		return "", err
	}

	return fullPath, nil
}

func rejectLinks(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	current := absPath
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
				return errors.New("invalid data: paths must not traverse symbolic links or reparse points")
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return nil
}

func ensureNewDestination(path string) error {
	if err := rejectLinks(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return errors.New("io error: destination already exists; restore never overwrites data")
	}
	return nil
}

func privateDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return os.MkdirAll(path, 0755)
	}
	return os.MkdirAll(path, 0700)
}

func copyPrivate(source, target string) error {
	if err := rejectLinks(source); err != nil {
		return err
	}

	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	mode := os.FileMode(0600)
	dstFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

func writePrivate(path string, data []byte) error {
	mode := os.FileMode(0600)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}

	return f.Sync()
}

func digest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))), nil
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func RestoreBackup(backup, destination string) error {
	manifest, err := ValidateBackup(backup)
	if err != nil {
		return fmt.Errorf("backup validation failed prior to restore: %w", err)
	}

	destAbs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	destination = destAbs

	if err := ensureNewDestination(destination); err != nil {
		return err
	}

	backupAbs, err := filepath.Abs(backup)
	if err != nil {
		return err
	}

	// 检查还原目标是否在备份文件内，或者备份文件在还原目标内
	if isWithin(backupAbs, destination) || isWithin(destination, backupAbs) {
		return errors.New("invalid operation: restore destination must be outside the backup")
	}

	staging := fmt.Sprintf("%s.staging-%s", destination, strings.ReplaceAll(uuid.New().String(), "-", ""))
	if err := ensureNewDestination(staging); err != nil {
		return err
	}

	defer func() {
		if _, err := os.Stat(staging); err == nil {
			_ = os.RemoveAll(staging)
		}
	}()

	if err := privateDirectory(staging); err != nil {
		return err
	}

	// 1. 文件系统大小写敏感度探针测试
	probe := filepath.Join(staging, "case-probe")
	if err := writePrivate(probe, []byte{}); err != nil {
		return err
	}

	probeUpper := filepath.Join(staging, "CASE-PROBE")
	_, errUpper := os.Stat(probeUpper)
	ignoresCase := errUpper == nil

	_ = os.Remove(probe)

	// 如果目标文件系统忽略大小写，检查路径中是否存在大小写碰撞
	if ignoresCase {
		pathSet := make(map[string]struct{})
		for _, f := range manifest.Files {
			parts := strings.Split(f.Path, "/")
			for count := 1; count <= len(parts); count++ {
				pathSet[strings.Join(parts[:count], "/")] = struct{}{}
			}
		}
		for k := range manifest.Plan.Roots {
			pathSet[k] = struct{}{}
		}

		lowerSet := make(map[string]struct{})
		for p := range pathSet {
			lower := strings.ToLower(p)
			if _, exists := lowerSet[lower]; exists {
				return errors.New("invalid data: backup contains case-distinct paths that collide on the restore filesystem")
			}
			lowerSet[lower] = struct{}{}
		}
	}

	// 2. 创建各根目录分类
	for name := range manifest.Plan.Roots {
		if err := privateDirectory(filepath.Join(staging, name)); err != nil {
			return err
		}
	}

	// 3. 复制有效载荷文件并复核校验和
	payloadDir := filepath.Join(backupAbs, "payload")
	for _, file := range manifest.Files {
		target, err := safePath(staging, file.Path)
		if err != nil {
			return err
		}

		if err := privateDirectory(filepath.Dir(target)); err != nil {
			return err
		}

		sourcePath, err := safePath(payloadDir, file.Path)
		if err != nil {
			return err
		}

		if err := copyPrivate(sourcePath, target); err != nil {
			return err
		}

		hash, err := digest(target)
		if err != nil {
			return err
		}
		if hash != file.Sha256 {
			return errors.New("invalid data: backup changed during restore")
		}
	}

	// 4. SQLite 数据库结构完整性校验 (PRAGMA quick_check)
	for _, file := range manifest.Files {
		lowerPath := strings.ToLower(file.Path)
		if strings.HasSuffix(lowerPath, ".db") || strings.HasSuffix(lowerPath, ".sqlite") || strings.HasSuffix(lowerPath, ".sqlite3") {
			if err := validateSqliteDatabase(staging, file.Path); err != nil {
				return err
			}
		}
	}

	// 5. 写入还原标记声明与 Manifest
	reviewNotice := []byte("Offline restore validated. Do not start until paths and secret references have been reviewed. Original absolute paths are not rewritten. No schedules, tools, or providers were started.\n")
	if err := writePrivate(filepath.Join(staging, "RESTORE-REQUIRES-REVIEW.txt"), reviewNotice); err != nil {
		return err
	}

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := writePrivate(filepath.Join(staging, "restore-manifest.json"), manifestBytes); err != nil {
		return err
	}

	// 6. 最终原子提交移动
	return os.Rename(staging, destination)
}

// validateSqliteDatabase 为 SQLite 库文件建立独立的临时代理环境，验证其物理及逻辑完整性
func validateSqliteDatabase(stagingDir, relativePath string) error {
	scratch := fmt.Sprintf("%s.sqlite-check-%s", stagingDir, strings.ReplaceAll(uuid.New().String(), "-", ""))
	defer func() {
		if _, err := os.Stat(scratch); err == nil {
			_ = os.RemoveAll(scratch)
		}
	}()

	if err := privateDirectory(scratch); err != nil {
		return err
	}

	databasePath := filepath.Join(scratch, "database.db")

	// 拷贝 SQLite 主库文件及其对应的 WAL/SHM 日志附属文件
	for _, suffix := range []string{"", "-wal", "-shm"} {
		sourcePath, err := safePath(stagingDir, relativePath+suffix)
		if err == nil {
			if _, err := os.Stat(sourcePath); err == nil {
				if err := copyPrivate(sourcePath, databasePath+suffix); err != nil {
					return err
				}
			}
		}
	}

	// 使用只读模式（mode=ro）打开临时副本
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(databasePath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("sqlite open failed for %s: %w", relativePath, err)
	}
	defer db.Close()

	var result string
	row := db.QueryRow("PRAGMA quick_check;")
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("sqlite query failed for %s: %w", relativePath, err)
	}

	if !strings.EqualFold(result, "ok") {
		return fmt.Errorf("invalid data: SQLite validation failed for %s (result: %s)", relativePath, result)
	}

	return nil
}
