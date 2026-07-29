package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	base64Prefix = "=?base64?"
	base64Suffix = "?="
)

type McpHeaderEncoder struct{}

var McpHeaderEncoderInstance = &McpHeaderEncoder{}

func (e *McpHeaderEncoder) EncodeString(value *string) *string {
	if value == nil {
		return nil
	}

	if e.requiresBase64Encoding(*value) {
		encoded := e.encodeAsBase64(*value)
		return &encoded
	}

	return value
}

func (e *McpHeaderEncoder) EncodeBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (e *McpHeaderEncoder) EncodeInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func (e *McpHeaderEncoder) EncodeValue(value any) *string {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		return e.EncodeString(&v)
	case bool:
		res := e.EncodeBool(v)
		return &res
	case *string:
		return e.EncodeString(v)
	case *bool:
		if v == nil {
			return nil
		}
		res := e.EncodeBool(*v)
		return &res
	}

	stringValue := e.convertToString(value)
	if stringValue == nil {
		return nil
	}

	return stringValue
}

func (e *McpHeaderEncoder) DecodeValue(headerValue *string) *string {
	if headerValue == nil || len(*headerValue) == 0 {
		return headerValue
	}

	val := *headerValue

	if strings.HasPrefix(val, base64Prefix) && strings.HasSuffix(val, base64Suffix) {
		base64Content := val[len(base64Prefix) : len(val)-len(base64Suffix)]

		bytes, err := base64.StdEncoding.DecodeString(base64Content)
		if err != nil {
			return nil
		}

		if !utf8.Valid(bytes) {
			return nil
		}

		decoded := string(bytes)
		return &decoded
	}

	return headerValue
}

func (e *McpHeaderEncoder) ConvertJSONRawMessageToHeaderValue(data json.RawMessage) *string {
	var interfaceVal any
	if err := json.Unmarshal(data, &interfaceVal); err != nil {
		return nil
	}

	return e.convertParsedJSONToHeaderValue(interfaceVal, string(data))
}

func (e *McpHeaderEncoder) convertParsedJSONToHeaderValue(val any, rawText string) *string {
	switch v := val.(type) {
	case string:
		return e.EncodeString(&v)
	case bool:
		res := e.EncodeBool(v)
		return &res
	case float64:
		if rawText != "" {
			return &rawText
		}
		s := strconv.FormatFloat(v, 'f', -1, 64)
		return &s
	default:
		return nil
	}
}

func (e *McpHeaderEncoder) convertToString(value any) *string {
	var res string
	switch v := value.(type) {
	case string:
		res = v
	case bool:
		res = e.EncodeBool(v)
	case int:
		res = strconv.FormatInt(int64(v), 10)
	case int8:
		res = strconv.FormatInt(int64(v), 10)
	case int16:
		res = strconv.FormatInt(int64(v), 10)
	case int32:
		res = strconv.FormatInt(int64(v), 10)
	case int64:
		res = strconv.FormatInt(v, 10)
	case uint:
		res = strconv.FormatUint(uint64(v), 10)
	case uint8:
		res = strconv.FormatUint(uint64(v), 10)
	case uint16:
		res = strconv.FormatUint(uint64(v), 10)
	case uint32:
		res = strconv.FormatUint(uint64(v), 10)
	case uint64:
		res = strconv.FormatUint(v, 10)
	default:
		return nil
	}
	return &res
}

func (e *McpHeaderEncoder) requiresBase64Encoding(value string) bool {
	if len(value) == 0 {
		return false
	}

	if value[0] == ' ' || value[0] == '\t' || value[len(value)-1] == ' ' || value[len(value)-1] == '\t' {
		return true
	}
	if strings.HasPrefix(value, base64Prefix) && strings.HasSuffix(value, base64Suffix) {
		return true
	}
	for _, ch := range value {
		if ch < 0x20 || ch > 0x7E {
			return true
		}
	}

	return false
}

func (e *McpHeaderEncoder) encodeAsBase64(value string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(value))
	return fmt.Sprintf("%s%s%s", base64Prefix, encoded, base64Suffix)
}
