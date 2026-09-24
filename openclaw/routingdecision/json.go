package routingdecision

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// LayaCanonicalJson 规范 JSON 序列化工具
type LayaCanonicalJson struct{}

// 将 json.RawMessage 或 any 转换为规范 JSON 字符串
func Serialize(v any) (string, error) {
	var val any

	switch data := v.(type) {
	case json.RawMessage:
		if err := json.Unmarshal(data, &val); err != nil {
			return "", err
		}
	case []byte:
		if err := json.Unmarshal(data, &val); err != nil {
			return "", err
		}
	default:
		val = v
	}

	var buf bytes.Buffer
	appendValue(val, &buf)
	return buf.String(), nil
}

func appendValue(v any, buf *bytes.Buffer) {
	switch val := v.(type) {
	case map[string]any:
		buf.WriteByte('{')
		first := true
		// 注意：Go 的 map 在读取时无序
		for k, item := range val {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			appendString(k, buf)
			buf.WriteByte(':')
			appendValue(item, buf)
		}
		buf.WriteByte('}')

	case []any:
		buf.WriteByte('[')
		first := true
		for _, item := range val {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			appendValue(item, buf)
		}
		buf.WriteByte(']')

	case string:
		appendString(val, buf)

	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}

	case nil:
		buf.WriteString("null")

	case float64:
		// 保留原始数字字面量表达，避免额外空格或科学计数法变形
		fmt.Fprintf(buf, "%g", val)

	default:
		// 兜底情况（如 json.Number 等）
		fmt.Fprintf(buf, "%v", val)
	}
}

func appendString(s string, buf *bytes.Buffer) {
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
