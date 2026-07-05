package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "time/tzdata"

	"github.com/robfig/cron/v3"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func findDirectoriesCantainsFileName(candidatePath string, filename string) ([]string, error) {
	uniquePaths := make(map[string]bool)

	err := filepath.WalkDir(candidatePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if !d.IsDir() && d.Name() == filename {
			dir := filepath.Dir(path)

			if strings.TrimSpace(dir) != "" {
				uniquePaths[dir] = true
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	matches := make([]string, 0, len(uniquePaths))
	for path := range uniquePaths {
		matches = append(matches, path)
	}

	return matches, nil
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func isBlankP(s *string) bool {
	if s == nil {
		return true
	}
	return strings.TrimSpace(*s) == ""
}

func readStringArray(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func containsIgnoreCase(slice []string, val string) bool {
	target := strings.ToLower(val)
	for _, item := range slice {
		if strings.ToLower(item) == target {
			return true
		}
	}
	return false
}

func isLoopbackBind(bindAddress string) bool {
	// 1. 排除常见的通配符（绑定到所有接口，非 loopback）
	if bindAddress == "*" || bindAddress == "+" || bindAddress == "[::]" || bindAddress == ":" || bindAddress == "0.0.0.0" {
		return false
	}

	// 2. 尝试解析为 IP 地址并判断是否为环回地址
	if ip := net.ParseIP(bindAddress); ip != nil {
		return ip.IsLoopback()
	}

	// 3. 不区分大小写判断是否为 "localhost"
	return strings.EqualFold(bindAddress, "localhost")
}

func generateCode(min, max int64) string {
	rangeSize := big.NewInt(max - min)

	randomNum, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		panic("critical system error: failed to generate secure random number: " + err.Error())
	}

	codeInt := randomNum.Int64() + min
	code := strconv.FormatInt(codeInt, 10)
	return code
}

func indexOf(s string, substr string, startIndex int) int {
	if startIndex < 0 || startIndex > len(s) {
		return -1
	}

	result := strings.Index(s[startIndex:], substr)

	if result != -1 {
		return result + startIndex
	}

	return -1
}

func IntervalToCron(interval string) (string, error) {
	if len(interval) < 2 {
		return "", fmt.Errorf("invalid interval: %s", interval)
	}

	unit := interval[len(interval)-1]
	valStr := interval[:len(interval)-1]
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		return "", fmt.Errorf("invalid interval value: %s", interval)
	}

	switch unit {
	case 's':
		if val >= 60 {
			return fmt.Sprintf("*/%d * * * *", val/60), nil
		}
		return fmt.Sprintf("*/%d * * * * *", val), nil
	case 'm':
		return fmt.Sprintf("*/%d * * * *", val), nil
	case 'h':
		return fmt.Sprintf("0 */%d * * *", val), nil
	default:
		return "", fmt.Errorf("unknown interval unit: %c", unit)
	}
}

func Truncate(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}

	return value[:maxLength] + "..."
}

func NormalizeForComparison(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(trimmed))

	wasSpace := false

	for _, ch := range trimmed {
		if unicode.IsSpace(ch) {
			if !wasSpace {
				sb.WriteByte(' ')
				wasSpace = true
			}
		} else {
			sb.WriteRune(ch)
			wasSpace = false
		}
	}

	return sb.String()
}

func ComputeTurnHash(normalizedText string) string {
	if normalizedText == "" {
		return ""
	}

	hash := sha256.Sum256([]byte(normalizedText))

	// hex.EncodeToString 会自动生成纯小写的十六进制字符串
	return hex.EncodeToString(hash[:])
}

func isLetterOrDigit(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

type NamedLockManager struct {
	mu    sync.Mutex
	gates map[string]*countedLock
}

type countedLock struct {
	ch       chan struct{}
	refCount int // 计数器：记录当前有多少个协程在引用这把锁
}

func NewNamedLockManager() *NamedLockManager {
	return &NamedLockManager{
		gates: make(map[string]*countedLock),
	}
}

// Lock 获取并锁定指定的 key，返回一个释放锁的函数
func (m *NamedLockManager) Lock(ctx context.Context, key string) (unlock func(), err error) {
	m.mu.Lock()

	// 1. 获取或创建锁，并将引用计数 +1
	lock, exists := m.gates[key]
	if !exists {
		lock = &countedLock{
			ch:       make(chan struct{}, 1),
			refCount: 0,
		}
		m.gates[key] = lock
	}
	lock.refCount++

	m.mu.Unlock()

	// 2. 尝试抢锁
	select {
	case lock.ch <- struct{}{}:
		// 抢锁成功
	case <-ctx.Done():
		// 抢锁超时/取消，需要把刚刚加的引用计数扣掉
		m.mu.Lock()
		lock.refCount--
		if lock.refCount == 0 {
			delete(m.gates, key)
		}
		m.mu.Unlock()
		return nil, ctx.Err()
	}

	// 3. 返回一个闭包用于释放锁
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		// 释放通道锁
		<-lock.ch

		// 引用计数 -1
		lock.refCount--
		// 如果没有任何协程在引用它了，安全地从 map 中剔除
		if lock.refCount == 0 {
			delete(m.gates, key)
		}
	}, nil
}

func daysInMonth(year int, month time.Month) string {
	t := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	return strconv.Itoa(t.Day())
}

func normalizeExpression(expression string, time time.Time) string {
	expression = strings.ToLower(strings.TrimSpace(expression))
	normalized := expression
	switch expression {
	case "@hourly":
		normalized = "0 * * * *"
	case "@daily":
		normalized = "0 0 * * *"
	case "@weekly":
		normalized = "0 0 * * 0"
	case "@monthly":
		normalized = "0 0 1 * *"
	}

	parts := strings.Split(normalized, " ")
	dayOfMonthIndex := -1
	if len(parts) == 5 {
		dayOfMonthIndex = 2
	}
	if len(parts) == 6 {
		dayOfMonthIndex = 3
	}

	if dayOfMonthIndex >= 0 && parts[dayOfMonthIndex] == "1" {
		parts[dayOfMonthIndex] = daysInMonth(time.Year(), time.Month())
	}

	return strings.Join(parts, " ")
}

func parseCronExpression(spec string) (cron.Schedule, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, false
	}

	// 1. 处理预定义描述符（如 @yearly, @monthly, @every 1h 等）
	if strings.HasPrefix(spec, "@") {
		parser := cron.NewParser(cron.Descriptor)
		if sched, err := parser.Parse(spec); err == nil {
			return sched, true
		}
		return nil, false
	}

	// 2. 根据空格计算段数
	fields := len(strings.Fields(spec))

	var parser cron.Parser
	switch fields {
	case 5:
		// 标准 5 段式：分 时 日 月 周
		parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	case 6:
		// 常见 6 段式：秒 分 时 日 月 周
		parser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	default:
		// 其他段数（如 7 段带年份的）当前库默认不支持，直接返回失败
		return nil, false
	}

	sched, err := parser.Parse(spec)
	if err != nil {
		return nil, false
	}

	return sched, true
}

func isTime(expression string, t time.Time) bool {
	sched, ok := parseCronExpression(expression)
	if !ok {
		return false
	}

	truncatedTime := t.Truncate(time.Second)
	previousSecond := truncatedTime.Add(-1 * time.Second)
	nextOccurrence := sched.Next(previousSecond)
	return nextOccurrence.Equal(truncatedTime)
}

func isValidIANA(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}
