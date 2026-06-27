package core

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ChannelAllowlistFile struct {
	AllowedFrom  []string  `json:"allowed_from"`
	AllowedTo    []string  `json:"allowed_to"`
	UpdatedAtUtc time.Time `json:"updated_at_utc"`
}

type AllowlistManager struct {
	rootDir string
	logger  *slog.Logger
	locks   sync.Map
}

func NewAllowlistManager(baseStoragePath string, logger *slog.Logger) (*AllowlistManager, error) {
	rootDir := filepath.Join(baseStoragePath, "allowlists")
	if logger == nil {
		logger = slog.Default()
	}

	// 创建初始目录
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	return &AllowlistManager{
		rootDir: rootDir,
		logger:  logger,
	}, nil
}

// TryGetDynamic 读取并解析动态白名单文件
func (m *AllowlistManager) TryGetDynamic(channelID string) *ChannelAllowlistFile {
	path := m.getPath(channelID)

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	// 读取文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		m.logger.Warn("Failed to read allowlist file", "channelId", channelID, "error", err)
		return nil
	}

	// 反序列化
	var file ChannelAllowlistFile
	if err := json.Unmarshal(data, &file); err != nil {
		m.logger.Warn("Failed to deserialize allowlist file", "channelId", channelID, "error", err)
		return nil
	}

	if file.AllowedFrom == nil {
		file.AllowedFrom = []string{}
	}
	if file.AllowedTo == nil {
		file.AllowedTo = []string{}
	}

	return &file
}

// GetEffective 获取有效配置，如果动态的不存在则返回默认配置
func (m *AllowlistManager) GetEffective(channelID string, configAllowlist ChannelAllowlistFile) ChannelAllowlistFile {
	if dynamic := m.TryGetDynamic(channelID); dynamic != nil {
		return *dynamic
	}
	return configAllowlist
}

// UpsertDynamic 执行原子的“读取-修改-写入”操作
func (m *AllowlistManager) UpsertDynamic(channelID string, updateFn func(*ChannelAllowlistFile) ChannelAllowlistFile) {
	// 获取或创建针对该 channelID 的锁
	actual, _ := m.locks.LoadOrStore(channelID, &sync.Mutex{})
	mutex := actual.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	current := m.TryGetDynamic(channelID)

	// 执行用户传入的更新逻辑，并强制刷新时间戳
	next := updateFn(current)
	next.UpdatedAtUtc = time.Now().UTC()

	path := m.getPath(channelID)
	tmpFile, err := os.CreateTemp(m.rootDir, "allowlist-*.tmp")
	if err != nil {
		m.logger.Warn("Failed to create temp file", "channelId", channelID, "error", err)
		return
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// 尽最大努力清理临时文件
		if _, err := os.Stat(tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	// 序列化并写入临时文件
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ") // 可选：美化 JSON 输出
	if err := encoder.Encode(next); err != nil {
		_ = tmpFile.Close()
		m.logger.Warn("Failed to serialize allowlist file", "channelId", channelID, "error", err)
		return
	}
	_ = tmpFile.Close()

	// 原子替换
	if err := os.Rename(tmpPath, path); err != nil {
		m.logger.Warn("Failed to persist allowlist file", "channelId", channelID, "error", err)
	}
}

// AddAllowedFrom 往 AllowedFrom 追加单个 senderId
func (m *AllowlistManager) AddAllowedFrom(channelID string, senderID string) {
	if strings.TrimSpace(senderID) == "" {
		return
	}

	m.UpsertDynamic(channelID, func(cur *ChannelAllowlistFile) ChannelAllowlistFile {
		if cur == nil {
			cur = &ChannelAllowlistFile{AllowedFrom: []string{}, AllowedTo: []string{}}
		}

		// 检查是否存在
		for _, id := range cur.AllowedFrom {
			if id == senderID {
				return *cur
			}
		}

		// 追加元素
		nextAllowedFrom := append(cur.AllowedFrom, senderID)
		return ChannelAllowlistFile{
			AllowedFrom: nextAllowedFrom,
			AllowedTo:   cur.AllowedTo,
		}
	})
}

// SetAllowedFrom 覆盖设置 AllowedFrom 并进行去重和过滤
func (m *AllowlistManager) SetAllowedFrom(channelID string, senderIDs []string) {
	// 过滤、去重与清洗
	seen := make(map[string]bool)
	var list []string

	for _, id := range senderIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			list = append(list, trimmed)
		}
	}
	if list == nil {
		list = []string{}
	}

	m.UpsertDynamic(channelID, func(cur *ChannelAllowlistFile) ChannelAllowlistFile {
		if cur == nil {
			cur = &ChannelAllowlistFile{AllowedFrom: []string{}, AllowedTo: []string{}}
		}
		return ChannelAllowlistFile{
			AllowedFrom: list,
			AllowedTo:   cur.AllowedTo,
		}
	})
}

// getPath 过滤文件名，确保路径安全
func (m *AllowlistManager) getPath(channelID string) string {
	var sb strings.Builder
	for _, r := range channelID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			sb.WriteRune(r)
		}
	}

	safe := sb.String()
	// 去除开头的 '.' 防止隐藏文件攻击
	safe = strings.TrimLeft(safe, ".")

	if strings.TrimSpace(safe) == "" {
		safe = "unknown"
	}

	return filepath.Join(m.rootDir, safe+".json")
}
