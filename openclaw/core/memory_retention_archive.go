package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ArchiveResult struct {
	DeletedFiles  int
	Errors        int
	ErrorMessages []string
}

// MemoryRetentionArchive 提供内存留存归档与清理的核心功能
type MemoryRetentionArchive struct{}

// ArchivePayloadMetadata JSON 包装结构
type ArchivePayloadMetadata struct {
	Kind          string          `json:"kind"`
	ID            string          `json:"id"`
	SweptAtUtc    string          `json:"sweptAtUtc"`
	ExpiresAtUtc  string          `json:"expiresAtUtc"`
	SourceBackend string          `json:"sourceBackend"`
	Payload       json.RawMessage `json:"payload"`
}

// ArchivePayload 将 Payload 异步/安全地归档到磁盘
func (m *MemoryRetentionArchive) ArchivePayload(
	ctx context.Context,
	archiveRoot string,
	nowUtc time.Time,
	kind string,
	id string,
	expiresAtUtc time.Time,
	sourceBackend string,
	payloadJson string,
) error {
	// 1. 参数校验
	if strings.TrimSpace(archiveRoot) == "" {
		return errors.New("archiveRoot must be set")
	}
	if strings.TrimSpace(kind) == "" {
		return errors.New("kind must be set")
	}
	if strings.TrimSpace(id) == "" {
		return errors.New("id must be set")
	}

	// 验证 payloadJson 是否为合法的 JSON
	var rawPayload json.RawMessage = []byte(payloadJson)
	if !json.Valid(rawPayload) {
		return errors.New("payloadJson is not valid JSON")
	}

	// 检查 context 是否已取消
	if err := ctx.Err(); err != nil {
		return err
	}

	// 2. 路径规划与目录创建
	now := nowUtc.UTC()
	archiveBase, err := filepath.Abs(archiveRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	destinationDir := filepath.Join(
		archiveBase,
		now.Format("2006"), // yyyy
		now.Format("01"),   // MM
		now.Format("02"),   // dd
		kind,
	)

	if err := os.MkdirAll(destinationDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 3. 构造文件名 (这里转成纯文本格式避免点号破坏后缀)
	timestamp := now.Format("20060102T150405_0000000Z")
	fileName := fmt.Sprintf("%s-%s.json", m.encodeID(id), timestamp)
	destinationPath := filepath.Join(destinationDir, fileName)

	// 4. 使用临时文件安全写入（防断电/崩溃导致文件损坏）
	tempFile, err := os.CreateTemp(destinationDir, fileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// 确保发生异常或提前退出时清理临时文件
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	// 5. 构造并写入 JSON 数据
	meta := ArchivePayloadMetadata{
		Kind:          kind,
		ID:            id,
		SweptAtUtc:    now.Format(time.RFC3339Nano),
		ExpiresAtUtc:  expiresAtUtc.UTC().Format(time.RFC3339Nano),
		SourceBackend: sourceBackend,
		Payload:       rawPayload,
	}

	encoder := json.NewEncoder(tempFile)
	if err := encoder.Encode(meta); err != nil {
		return fmt.Errorf("failed to encode json: %w", err)
	}

	// 显式 Flush/Sync 确保数据落盘
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	tempFile.Close()
	tempFile = nil // 释放 defer 中的 Close 占用

	// 检查 Context 状态后再做原子替换
	if err := ctx.Err(); err != nil {
		return err
	}

	// 原子重命名/覆盖
	if err := os.Rename(tempPath, destinationPath); err != nil {
		return fmt.Errorf("failed to move temp file to destination: %w", err)
	}

	return nil
}

// PurgeExpiredArchives 扫描并清理过期的归档文件以及空目录
func (m *MemoryRetentionArchive) PurgeExpiredArchives(
	ctx context.Context,
	archiveRoot string,
	nowUtc time.Time,
	retentionDays int,
) ArchiveResult {
	result := ArchiveResult{
		ErrorMessages: make([]string, 0, 4),
	}

	if strings.TrimSpace(archiveRoot) == "" {
		return result
	}
	if _, err := os.Stat(archiveRoot); os.IsNotExist(err) {
		return result
	}

	if retentionDays < 1 {
		retentionDays = 1
	}
	cutoff := nowUtc.UTC().AddDate(0, 0, -retentionDays)

	// 1. 递归枚举所有文件
	var files []string
	err := filepath.WalkDir(archiveRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		result.Errors++
		result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Failed to enumerate archive files: %v", err))
		return result
	}

	// 2. 遍历并检查过期情况
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return result // 响应 Context 取消
		}

		shouldDelete := false

		if archiveDayUtc, ok := m.tryGetArchiveSweepDayUtc(archiveRoot, file); ok {
			cutoffDate := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)
			if archiveDayUtc.After(cutoffDate) {
				continue
			}
			if archiveDayUtc.Before(cutoffDate) {
				shouldDelete = true
			}
		}

		if !shouldDelete {
			// 如果无法从路径推断，则解析 JSON 内容或检查修改时间
			if sweptAtUtc, err := m.readSweptAtFromJSON(file); err == nil {
				if sweptAtUtc.Before(cutoff) {
					shouldDelete = true
				} else {
					continue
				}
			} else {
				// 降级：读取文件系统最后修改时间
				if info, err := os.Stat(file); err == nil {
					if info.ModTime().UTC().Before(cutoff) {
						shouldDelete = true
					} else {
						continue
					}
				}
			}
		}

		// 执行删除
		if shouldDelete {
			if err := os.Remove(file); err != nil {
				result.Errors++
				if len(result.ErrorMessages) < 16 {
					result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Failed to delete archive file '%s': %v", file, err))
				}
			} else {
				result.DeletedFiles++
			}
		}
	}

	// 3. 清理空目录
	m.cleanupEmptyDirectories(archiveRoot)

	return result
}

// 将 ID 计算为 SHA256 小写十六进制字符串
func (m *MemoryRetentionArchive) encodeID(id string) string {
	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:])
}

// 尝试从路径的 yyyy/MM/dd 结构中提取时间
func (m *MemoryRetentionArchive) tryGetArchiveSweepDayUtc(archiveRoot, filePath string) (time.Time, bool) {
	rel, err := filepath.Rel(archiveRoot, filePath)
	if err != nil {
		return time.Time{}, false
	}

	// 标准化路径分隔符
	rel = filepath.ToSlash(rel)
	segments := strings.Split(rel, "/")
	if len(segments) < 4 {
		return time.Time{}, false
	}

	year, err1 := strconv.Atoi(segments[0])
	month, err2 := strconv.Atoi(segments[1])
	day, err3 := strconv.Atoi(segments[2])

	if err1 != nil || err2 != nil || err3 != nil {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
}

// 读取 JSON 中的 sweptAtUtc 字段
func (m *MemoryRetentionArchive) readSweptAtFromJSON(filePath string) (time.Time, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	// 使用延迟解析避免读取整个大文件到内存
	var meta struct {
		SweptAtUtc string `json:"sweptAtUtc"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&meta); err != nil {
		return time.Time{}, err
	}

	// 支持多种标准时间格式解析
	return time.Parse(time.RFC3339, meta.SweptAtUtc)
}

// 递归清理空目录
func (m *MemoryRetentionArchive) cleanupEmptyDirectories(archiveRoot string) {
	var dirs []string

	_ = filepath.WalkDir(archiveRoot, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && path != archiveRoot {
			dirs = append(dirs, path)
		}
		return nil
	})

	// 按照路径长度从长到短排序
	// 这确保了子目录优先于父目录被处理和删除
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})

	for _, dir := range dirs {
		f, err := os.Open(dir)
		if err != nil {
			continue
		}
		_, err = f.Readdirnames(1) // 尝试读取一个子项
		f.Close()

		// 如果返回 io.EOF，说明目录确实为空
		if errors.Is(err, io.EOF) {
			_ = os.Remove(dir)
		}
	}
}
