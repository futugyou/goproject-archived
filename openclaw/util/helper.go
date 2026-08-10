package util

import (
	"cmp"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unsafe"

	_ "time/tzdata"

	"github.com/google/uuid"
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

func DaysInMonth(year int, month time.Month) string {
	t := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	return strconv.Itoa(t.Day())
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

func DistinctStrings(input []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func Clamp[T cmp.Ordered](val, min, max T) T {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func Truncate(text string, max int) string {
	runes := []rune(text)

	if len(runes) <= max {
		return text
	}

	return string(runes[:max]) + "..."
}

func ToPascal(value string) string {
	if len(value) == 0 {
		return value
	}
	runes := []rune(value)
	return string(unicode.ToUpper(runes[0])) + strings.ToLower(string(runes[1:]))
}

func Deref[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

func Ptr[T any](val T) *T {
	return &val
}

func CleanUUID() string {
	rawUUID := uuid.New().String()
	return strings.ReplaceAll(rawUUID, "-", "")
}

func SlicesRemoveRange[T any](array []T, index, count int) []T {
	if index < 0 || count < 0 || index+count > len(array) {
		return array
	}
	return append(array[:index], array[index+count:]...)
}

func SlicesInsert[T any](s []T, index int, value T) []T {
	if index < 0 || index > len(s) {
		panic("index out of range")
	}
	s = append(s, value)
	copy(s[index+1:], s[index:])
	s[index] = value
	return s
}

func SlicesInsertRange[T any](s []T, index int, values []T) []T {
	if index < 0 || index > len(s) {
		panic("index out of range")
	}
	if len(values) == 0 {
		return s
	}

	s = append(s, make([]T, len(values))...)
	copy(s[index+len(values):], s[index:])
	copy(s[index:], values)
	return s
}
