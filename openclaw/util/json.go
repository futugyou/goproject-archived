package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

func ReadStringArray(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func TryGetPropertyString(raw json.RawMessage, propertyName string) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", false
	}

	if value, ok := data[propertyName].(string); ok {
		return value, true
	}

	return "", false
}

func ParseStringArray(root map[string]any, propertyName string) []string {
	val, ok := TryGetProperty(root, propertyName)
	if !ok {
		return []string{}
	}

	arr, ok := val.([]any)
	if !ok {
		return []string{}
	}

	result := make([]string, 0, len(arr))
	for _, item := range arr {
		var strVal string
		switch v := item.(type) {
		case string:
			strVal = v
		case nil:
			continue
		default:
			strVal = fmt.Sprintf("%v", v)
		}

		if strings.TrimSpace(strVal) != "" {
			result = append(result, strVal)
		}
	}
	return result
}

func TryGetStringArray(args map[string]any, key string) []string {
	values := []string{}
	value, ok := args[key]
	if !ok {
		return values
	}

	arr, ok := value.([]any)
	if !ok {
		return []string{}
	}

	result := make([]string, 0, len(arr))
	for _, item := range arr {
		var strVal string
		switch v := item.(type) {
		case string:
			strVal = strings.TrimSpace(v)
		}

		if strVal != "" {
			result = append(result, strVal)
		}
	}
	return result
}

func TryGetObject(element any) (map[string]any, bool) {
	if element == nil {
		return nil, false
	}

	switch v := element.(type) {
	case map[string]any:
		return v, true
	case []any:
		if len(v) > 0 {
			if firstObj, ok := v[0].(map[string]any); ok {
				return firstObj, true
			}
		}
	}

	return nil, false
}

func TryGetArrayOrObjectArray(element any, propertyName string) ([]any, bool) {
	if element == nil {
		return []any{}, false
	}

	if arr, ok := element.([]any); ok {
		return arr, true
	}

	if obj, ok := element.(map[string]any); ok {
		if val, found := TryGetProperty(obj, propertyName); found {
			if arr, ok := val.([]any); ok {
				return arr, true
			}
		}
	}

	return []any{}, false
}

func GetString(root map[string]any, name string) *string {
	val, ok := TryGetProperty(root, name)
	if !ok || val == nil {
		return nil
	}

	var res string
	if str, ok := val.(string); ok {
		res = str
	} else {
		res = fmt.Sprintf("%v", val)
	}
	return &res
}

func GetInt(root map[string]any, name string) *int {
	val, ok := TryGetProperty(root, name)
	if !ok || val == nil {
		return nil
	}

	switch v := val.(type) {
	case float64:
		i := int(v)
		return &i
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return &i
		}
	}
	return nil
}

func GetFloat64(root map[string]any, name string) *float64 {
	val, ok := TryGetProperty(root, name)
	if !ok || val == nil {
		return nil
	}

	switch v := val.(type) {
	case float64:
		return &v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return &f
		}
	}
	return nil
}

func GetBool(root map[string]any, name string) *bool {
	val, ok := TryGetProperty(root, name)
	if !ok || val == nil {
		return nil
	}

	switch v := val.(type) {
	case bool:
		return &v
	case string:
		if b, err := strconv.ParseBool(v); err == nil {
			return &b
		}
	}
	return nil
}

// 获取 ISO 8601 / RFC 3339 格式的微秒/时区时间
func GetDateTimeOffset(root map[string]any, name string) *time.Time {
	strPtr := GetString(root, name)
	if strPtr == nil {
		return nil
	}

	if t, err := time.Parse(time.RFC3339, *strPtr); err == nil {
		return &t
	}
	return nil
}

func TryGetProperty(root map[string]any, name string) (any, bool) {
	if root == nil {
		return nil, false
	}

	if val, ok := root[name]; ok {
		return val, true
	}

	if len(name) > 0 {
		pascal := strings.ToUpper(name[:1]) + name[1:]
		if val, ok := root[pascal]; ok {
			return val, true
		}
	}

	return nil, false
}

func TryGetValueFromRawMessage[T any](root map[string]json.RawMessage, name string) (T, bool) {
	var data T

	d, ok := root[name]
	if !ok {
		return data, false
	}

	if err := json.Unmarshal(d, &data); err != nil {
		return data, false
	}

	return data, true
}

func IsValidJson(value string) bool {
	var doc map[string]any
	if err := json.Unmarshal([]byte(value), &doc); err != nil {
		return false
	}

	return true
}

func DeserializeMap(withJson string) map[string]any {
	withJson = strings.TrimSpace(withJson)
	if withJson == "" {
		return map[string]any{}
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(withJson), &parsed); err != nil {
		return map[string]any{}
	}

	return parsed
}

func JCSJsonTransform(input []byte) ([]byte, error) {
	var v any
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber() // 使用 json.Number 以准确保留数字类型信息
	if err := decoder.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	var buf bytes.Buffer
	if err := canonicalize(v, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func canonicalize(v any, buf *bytes.Buffer) error {
	switch val := v.(type) {
	case map[string]any:
		return serializeObject(val, buf)

	case []any:
		return serializeArray(val, buf)

	case string:
		serializeString(val, buf)
		return nil

	case json.Number:
		return serializeNumber(val, buf)

	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil

	case nil:
		buf.WriteString("null")
		return nil

	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

// 规则 1：对象 Key 排序 (按 UTF-16 码点升序)
func serializeObject(obj map[string]any, buf *bytes.Buffer) error {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}

	// 按照 RFC 8785 要求，以 UTF-16 Code Unit 顺序排序
	sort.Slice(keys, func(i, j int) bool {
		return compareUTF16(keys[i], keys[j]) < 0
	})

	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		serializeString(k, buf)
		buf.WriteByte(':')
		if err := canonicalize(obj[k], buf); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func serializeArray(arr []any, buf *bytes.Buffer) error {
	buf.WriteByte('[')
	for i, item := range arr {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := canonicalize(item, buf); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

// 规则 3：字符串转义控制 (保持原字面量，仅转义控制字符、" 和 \)
func serializeString(s string, buf *bytes.Buffer) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

// 规则 4：数字规范化 (ECMAScript NumberToString 算法)
func serializeNumber(num json.Number, buf *bytes.Buffer) error {
	f, err := num.Float64()
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("invalid number: %s", num.String())
	}

	// 整数/浮点数标准化转换为 ECMAScript 兼容表达格式
	if f == 0 {
		buf.WriteByte('0')
		return nil
	}

	// 使用 Go strconv 模拟 IEEE 754 ES6 标准表示
	formatted := strconv.FormatFloat(f, 'g', -1, 64)

	// 确保指数部分的 '+' 被移除，如 1e+20 -> 1e20
	formatted = removeExponentPlus(formatted)

	buf.WriteString(formatted)
	return nil
}

func removeExponentPlus(s string) string {
	var result []rune
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if (runes[i] == 'e' || runes[i] == 'E') && i+1 < len(runes) && runes[i+1] == '+' {
			result = append(result, 'e')
			i++ // 跳过 '+' 号
			continue
		}
		if runes[i] == 'E' {
			result = append(result, 'e')
			continue
		}
		result = append(result, runes[i])
	}
	return string(result)
}

// 辅助函数：依据 UTF-16 码点顺序比较两个 UTF-8 字符串
func compareUTF16(a, b string) int {
	u16a := utf16.Encode([]rune(a))
	u16b := utf16.Encode([]rune(b))

	minLen := min(len(u16b), len(u16a))

	for i := range minLen {
		if u16a[i] != u16b[i] {
			if u16a[i] < u16b[i] {
				return -1
			}
			return 1
		}
	}

	if len(u16a) < len(u16b) {
		return -1
	} else if len(u16a) > len(u16b) {
		return 1
	}
	return 0
}
