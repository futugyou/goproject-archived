package util

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ReadStringArray(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func ParseStringArray(root map[string]any, propertyName string) []string {
	val, ok := tryGetProperty(root, propertyName)
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
		if val, found := tryGetProperty(obj, propertyName); found {
			if arr, ok := val.([]any); ok {
				return arr, true
			}
		}
	}

	return []any{}, false
}

func GetString(root map[string]any, name string) *string {
	val, ok := tryGetProperty(root, name)
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
	val, ok := tryGetProperty(root, name)
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
	val, ok := tryGetProperty(root, name)
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
	val, ok := tryGetProperty(root, name)
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

func tryGetProperty(root map[string]any, name string) (any, bool) {
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
