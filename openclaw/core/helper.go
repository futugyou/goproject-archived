package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unsafe"

	_ "time/tzdata"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

func IsBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func IsBlankP(s *string) bool {
	if s == nil {
		return true
	}
	return strings.TrimSpace(*s) == ""
}

func ReadStringArray(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func ContainsIgnoreCase(slice []string, val string) bool {
	target := strings.ToLower(val)
	for _, item := range slice {
		if strings.ToLower(item) == target {
			return true
		}
	}
	return false
}

func IsLoopbackBind(bindAddress string) bool {
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

func GenerateCode(min, max int64) string {
	rangeSize := big.NewInt(max - min)

	randomNum, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		panic("critical system error: failed to generate secure random number: " + err.Error())
	}

	codeInt := randomNum.Int64() + min
	code := strconv.FormatInt(codeInt, 10)
	return code
}

func IndexOf(s string, substr string, startIndex int) int {
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

func IsLetterOrDigit(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func IsKeywordCharacter(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value) || value == '_'
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

func DaysInMonth(year int, month time.Month) string {
	t := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	return strconv.Itoa(t.Day())
}

func NormalizeCronExpression(expression string) string {
	expression = strings.ToLower(strings.TrimSpace(expression))
	switch expression {
	case "@hourly":
		return "0 * * * *"
	case "@daily":
		return "0 0 * * *"
	case "@weekly":
		return "0 0 * * 0"
	case "@monthly":
		return "0 0 1 * *"
	default:
		return expression
	}
}

func IsValidCronExpression(expression string) bool {
	if IsBlank(expression) {
		return false
	}

	expression = NormalizeCronExpression(expression)

	var parts = strings.Split(expression, " ")
	if len(parts) != 5 {
		return false
	}

	return IsValidCronField(parts[0], 0, 59) &&
		IsValidCronField(parts[1], 0, 23) &&
		IsValidCronField(parts[2], 1, 31) &&
		IsValidCronField(parts[3], 1, 12) &&
		IsValidCronField(parts[4], 0, 6)
}

func IsValidCronField(field string, min, max int) bool {
	if IsBlank(field) {
		return false
	}

	if field == "*" {
		return true
	}

	if field == "L" {
		return min == 1
	}

	if exact, err := strconv.Atoi(field); err == nil {
		return exact >= min && exact <= max
	}

	if strings.Contains(field, ",") {
		options := strings.Split(field, ",")
		for _, option := range options {
			if option == "" || !IsValidCronField(option, min, max) {
				return false
			}
		}
		return true
	}

	if strings.Contains(field, "/") {
		var stepParts = strings.Split(field, "/")
		if len(stepParts) != 2 {
			return false
		}

		step, err := strconv.Atoi(stepParts[1])
		if err != nil || step <= 0 {
			return false
		}

		return stepParts[0] == "*" || IsValidCronField(stepParts[0], min, max)
	}

	if strings.Contains(field, "-") {
		var rangeParts = strings.Split(field, "-")
		if len(rangeParts) != 2 {
			return false
		}
		start, err1 := strconv.Atoi(rangeParts[0])
		end, err2 := strconv.Atoi(rangeParts[1])
		if err1 != nil || err2 != nil {
			return false
		}

		return start >= min && start <= max && end >= min && end <= max
	}

	return false
}

func NormalizeExpression(expression string, time time.Time) string {
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
		parts[dayOfMonthIndex] = DaysInMonth(time.Year(), time.Month())
	}

	return strings.Join(parts, " ")
}

func ParseCronExpression(spec string) (cron.Schedule, bool) {
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

func IsTime(expression string, t time.Time) bool {
	sched, ok := ParseCronExpression(expression)
	if !ok {
		return false
	}

	truncatedTime := t.Truncate(time.Second)
	previousSecond := truncatedTime.Add(-1 * time.Second)
	nextOccurrence := sched.Next(previousSecond)
	return nextOccurrence.Equal(truncatedTime)
}

func IsValidIANA(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}

// encodeKey 实现 URL 安全的 Base64 编码 (密匙转码)
func EncodeKey(key string) string {
	if strings.TrimSpace(key) == "" {
		return "item"
	}

	bytes := []byte(key)
	encoded := base64.StdEncoding.EncodeToString(bytes)

	// 转换成 URL 安全格式并移除填充符 '='
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	return strings.TrimRight(encoded, "=")
}

func ExpandAllEnv(input string) string {
	if input == "" {
		return ""
	}

	winRe := regexp.MustCompile(`%([^%]+)%`)
	winExpanded := winRe.ReplaceAllStringFunc(input, func(match string) string {
		varName := match[1 : len(match)-1]
		return os.Getenv(varName)
	})

	return os.ExpandEnv(winExpanded)
}

func LoadAndDelete[T any](db *gorm.DB, id any) (*T, error) {
	var result T

	// 1. 利用 GORM 的 Statement 自动获取该结构体对应的真实表名
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&result); err != nil {
		return nil, err
	}
	tableName := stmt.Schema.Table

	// 2. 动态拼接并执行强类型的 DELETE ... RETURNING 语句
	// PostgreSQL 允许 RETURNING * 返回整行所有字段
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ? RETURNING *", tableName)

	err := db.Raw(query, id).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// ==========================================
// 通用私有泛型辅助工具函数
// ==========================================

func TryResolveLinkTarget(path string) (string, bool) {
	finalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return "", false
		}

		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) {
			return "", false
		}

		return "", false
	}

	// EvalSymlinks 如果传入普通路径，会直接返回原路径。
	// 我们检查原路径是否真的是一个符号链接。
	if IsLstatSame(path, finalPath) {
		return "", false
	}

	return finalPath, true
}

// 辅助函数：判断原路径是否本身就是最终路径（排除非链接的情况）
func IsLstatSame(original, final string) bool {
	// 获取原路径的 Lstat（不追踪链接本身）
	origFi, err1 := os.Lstat(original)
	// 获取最终路径的 Stat
	finalFi, err2 := os.Stat(final)

	if err1 != nil || err2 != nil {
		return true
	}

	// 如果原路径的模式不是 Symlink，说明它本来就不是链接
	if origFi.Mode()&os.ModeSymlink == 0 {
		return true
	}

	// 比较它们是否指向同一个文件系统实体
	return os.SameFile(origFi, finalFi)
}

// isUnresolvedLink 判断路径是否是一个无法解析的死链接
func IsUnresolvedLink(path string) bool {
	// 1. 获取路径自身的元数据（Lstat 不会追踪符号链接目标）
	fi, err := os.Lstat(path)
	if err != nil {
		// 如果路径本身就不存在或无法读取，它就谈不上是一个“未解析的链接”，返回 false
		return false
	}

	// 2. 检查它是否是符号链接
	if fi.Mode()&os.ModeSymlink == 0 {
		return false
	}

	// 3. 如果 tryResolveLinkTarget 返回 false，说明链接断开或目标不可达
	_, ok := TryResolveLinkTarget(path)
	return !ok
}

// 判断两个路径在当前操作系统下是否相等
func PathEqual(path1, path2 string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(path1, path2)
	}
	return path1 == path2
}

// 判断 path 是否以 prefix 为前缀（考虑操作系统大小写）
func PathHasPrefix(path, prefix string) bool {
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix))
	}
	return strings.HasPrefix(path, prefix)
}

func GetFileNameWithoutExtension(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func PathGetFullPath(path string) string {
	p, _ := filepath.Abs(path)
	return p
}

// LoadAllFile 遍历目录下所有的 .json 文件并反序列化为对象切片
func LoadAllFile[T any](ctx context.Context, directory string) ([]T, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return []T{}, nil // C# 中 catch 块返回空数组
	}

	var results []T
	for _, file := range files {
		// 检查 Context 是否已取消
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		path := filepath.Join(directory, file.Name())
		item, err := LoadOneFile[T](ctx, path)
		if err == nil && item != nil {
			results = append(results, *item)
		}
	}

	return results, nil
}

func AppendAllText(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(text + "\n")
	return err
}

// LoadOneFile 反序列化单个文件
func LoadOneFile[T any](ctx context.Context, path string) (*T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil // 文件不存在，明确返回 nil 指针
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	var item T
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("unmarshal json failed: %w", err)
	}

	return &item, nil
}

// saveOneFile 安全写入文件（先写临时文件再重命名，以保证原子性）
func SaveOneFile(ctx context.Context, path string, item any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// 重命名（在 Go 中跨平台覆盖行为略有区别，os.Rename 在 Linux/Unix 下支持覆盖，Windows 下建议先移除）
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Rename(tempPath, path)
}

// 定义一个结构体来存放分组数据
type FileGroup struct {
	BaseName     string
	Files        []string
	MaxWriteTime time.Time
}

func GetGroupByFilename(dirPath string) ([]FileGroup, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	groupMap := make(map[string]*FileGroup)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}
		writeTime := info.ModTime().UTC()

		ext := filepath.Ext(entry.Name())
		baseName := strings.TrimSuffix(entry.Name(), ext)

		key := strings.ToLower(baseName)

		if group, exists := groupMap[key]; exists {
			group.Files = append(group.Files, fullPath)
			if writeTime.After(group.MaxWriteTime) {
				group.MaxWriteTime = writeTime
			}
		} else {
			groupMap[key] = &FileGroup{
				BaseName:     baseName,
				Files:        []string{fullPath},
				MaxWriteTime: writeTime,
			}
		}
	}

	var groups []FileGroup
	for _, group := range groupMap {
		groups = append(groups, *group)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].MaxWriteTime.After(groups[j].MaxWriteTime)
	})

	return groups, nil
}

// 计算指定目录的总大小（字节）
func GetDirectorySize(path string) int64 {
	if path == "" {
		return 0
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return 0
	}

	var totalSize int64

	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		totalSize += fileInfo.Size()
		return nil
	})

	return totalSize
}

func DeleteOneFile(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func DeleteDirectory(path string) {
	_ = os.RemoveAll(path)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func FindDirectoriesCantainsFileName(candidatePath string, filename string) ([]string, error) {
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

func EnumerateTopFiles(root string) []string {
	var files []string

	// os.ReadDir 只读取 root 目录下的第一层内容（非递归）
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		// 过滤掉子目录，只保留文件
		if !entry.IsDir() {
			fullPath := filepath.Join(root, entry.Name())
			files = append(files, fullPath)
		}
	}

	return files
}

// 递归获取目录下所有文件的绝对路径
func EnumerateAllFiles(root string) []string {
	var files []string

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return err
		}

		if !d.IsDir() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			files = append(files, absPath)
		}

		return nil
	})

	return files
}

func SerializeEmbedding(v []float64, needCopy bool) []byte {
	if len(v) == 0 {
		return nil
	}

	// 一个 float64 占用 8 个字节
	const sizeOfFloat64 = 8

	// 通过 unsafe 获取底层字节切片（无内存拷贝）
	// 注意：如果这个 []byte 之后会被修改，或者其生命周期超出了 v 的范围
	srcBytes := unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*sizeOfFloat64)

	if needCopy {
		dstBytes := make([]byte, len(srcBytes))
		copy(dstBytes, srcBytes)
		return dstBytes
	}

	return srcBytes
}

// Percentile 计算已排序切片的百分位数
// sortedValues 必须是升序排好序的
// percentile 应当在 0.0 到 1.0 之间 (例如 0.95 表示 P95)
func Percentile(sortedValues []int64, percentile float64) int64 {
	length := len(sortedValues)
	if length == 0 {
		return 0
	}

	// 计算索引：(N - 1) * percentile，然后向上取整
	// math.Ceil 返回的是 float64，我们需要转换为 int
	index := int(math.Ceil(float64(length-1) * percentile))

	// 限制索引边界，防止越界 (相当于 C# 的 Math.Clamp)
	if index < 0 {
		index = 0
	} else if index > length-1 {
		index = length - 1
	}

	return sortedValues[index]
}

// PercentileUnsorted 接收未排序的切片，计算百分位数（不会修改原切片）
func PercentileUnsorted(values []int64, percentile float64) int64 {
	length := len(values)
	if length == 0 {
		return 0
	}

	sortedValues := slices.Clone(values)

	// 2. 升序排序
	slices.Sort(sortedValues)

	// 3. 与Percentile一致
	index := int(math.Ceil(float64(length-1) * percentile))

	if index < 0 {
		index = 0
	} else if index > length-1 {
		index = length - 1
	}

	return sortedValues[index]
}
