package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

type ExecutionHostKind uint8

const (
	ExecutionHostKind_Bridge ExecutionHostKind = iota
	ExecutionHostKind_NativeDynamic
)

const (
	Plugin_Const_Tools         = "tools"
	Plugin_Const_Services      = "services"
	Plugin_Const_Skills        = "skills"
	Plugin_Const_Channels      = "channels"
	Plugin_Const_Commands      = "commands"
	Plugin_Const_Providers     = "providers"
	Plugin_Const_Hooks         = "hooks"
	Plugin_Const_Memory        = "memory"
	Plugin_Const_NativeDynamic = "native_dynamic"
)

type PluginCapabilityPolicy struct{}

var PluginCapabilityPolicyInstance = &PluginCapabilityPolicy{}

func (p *PluginCapabilityPolicy) Normalize(capabilities []string) []string {
	tmps := map[string]struct{}{}
	resuls := []string{}
	for _, cap := range capabilities {
		if isBlank(cap) {
			continue
		}

		cap = strings.ToLower(strings.TrimSpace(cap))
		if _, ok := tmps[cap]; !ok {
			tmps[cap] = struct{}{}
			resuls = append(resuls, cap)
		}
	}

	slices.Sort(resuls)

	return resuls
}

func (p *PluginCapabilityPolicy) GetBlockedCapabilities(capabilities []string, hostKind ExecutionHostKind) []string {
	var normalized = p.Normalize(capabilities)
	switch hostKind {
	case ExecutionHostKind_Bridge:
		return []string{}
	case ExecutionHostKind_NativeDynamic:
		return normalized
	default:
		return normalized
	}
}

type PluginConfigValidator struct{}

var PluginConfigValidatorinstance = &PluginConfigValidator{}

var allowedKeywords = map[string]bool{
	"type": true, "properties": true, "required": true, "additionalProperties": true,
	"items": true, "enum": true, "const": true, "description": true, "title": true,
	"default": true, "minLength": true, "maxLength": true, "minimum": true, "maximum": true,
	"minItems": true, "maxItems": true, "pattern": true, "oneOf": true, "anyOf": true,
}

func (p *PluginConfigValidator) Validate(manifest PluginManifest, config *json.RawMessage) []PluginCompatibilityDiagnostic {
	if manifest.ConfigSchema == nil {
		return nil
	}

	var diagnostics []PluginCompatibilityDiagnostic

	// 解析 Schema 为 Go 对象 (map[string]any)
	var schemaMap map[string]any
	if err := json.Unmarshal(*manifest.ConfigSchema, &schemaMap); err != nil {
		return append(diagnostics, makeDiagnostic("invalid_schema", "Schema must be an object.", "$schema"))
	}

	// 1. 验证 Schema 本身的合法性
	p.validateSchemaObject(schemaMap, "$schema", &diagnostics)
	if len(diagnostics) > 0 {
		return diagnostics
	}

	// 2. 处理 Config 默认值
	var cfg any
	if config == nil || len(*config) == 0 {
		cfg = make(map[string]any)
	} else {
		_ = json.Unmarshal(*config, &cfg)
	}

	// 3. 校验 Config 值
	p.validateValue(cfg, schemaMap, "$", &diagnostics)
	return diagnostics
}

func (p *PluginConfigValidator) validateSchemaObject(schema map[string]any, path string, diagnostics *[]PluginCompatibilityDiagnostic) {
	// 检查未定义关键字
	for k := range schema {
		if !allowedKeywords[k] {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"unsupported_schema_keyword",
				fmt.Sprintf("Schema keyword '%s' is not supported at '%s'.", k, path),
				path,
			))
		}
	}

	// 递归检查 properties
	if props, ok := schema["properties"].(map[string]any); ok {
		for propName, propSchema := range props {
			if subMap, ok := propSchema.(map[string]any); ok {
				p.validateSchemaObject(subMap, fmt.Sprintf("%s.properties.%s", path, propName), diagnostics)
			}
		}
	}

	// 递归检查 items
	if items, ok := schema["items"].(map[string]any); ok {
		p.validateSchemaObject(items, path+".items", diagnostics)
	}

	// 检查 oneOf 和 anyOf
	p.validateSchemaArray(schema, "oneOf", path, diagnostics)
	p.validateSchemaArray(schema, "anyOf", path, diagnostics)
}

func (p *PluginConfigValidator) validateValue(value any, schema map[string]any, path string, diagnostics *[]PluginCompatibilityDiagnostic) {
	// 1. oneOf 逻辑
	if oneOf, ok := schema["oneOf"].([]any); ok {
		matches := p.countSchemaMatches(value, oneOf, path)
		if matches != 1 {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"config_one_of_mismatch",
				fmt.Sprintf("Config value at '%s' must match exactly one schema in 'oneOf'.", path),
				path,
			))
		}
		return
	}

	// 2. anyOf 逻辑
	if anyOf, ok := schema["anyOf"].([]any); ok {
		matches := p.countSchemaMatches(value, anyOf, path)
		if matches == 0 {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"config_any_of_mismatch",
				fmt.Sprintf("Config value at '%s' must match at least one schema in 'anyOf'.", path),
				path,
			))
		}
		return
	}

	// 3. 类型检测
	if typeStr, ok := schema["type"].(string); ok {
		if !matchesType(value, typeStr) {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"config_type_mismatch",
				fmt.Sprintf("Config value at '%s' must be of type '%s'.", path, typeStr),
				path,
			))
			return
		}
	}

	// 4. Enum 检测
	if enumArr, ok := schema["enum"].([]any); ok {
		matched := false
		for _, candidate := range enumArr {
			if jsonElementsEqual(candidate, value) {
				matched = true
				break
			}
		}
		if !matched {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"config_enum_mismatch",
				fmt.Sprintf("Config value at '%s' must match one of the allowed enum values.", path),
				path,
			))
			return
		}
	}

	// 5. Const 检测
	if constVal, exists := schema["const"]; exists {
		if !jsonElementsEqual(constVal, value) {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"config_const_mismatch",
				fmt.Sprintf("Config value at '%s' must match the schema const value.", path),
				path,
			))
			return
		}
	}

	// 6. 根据具体 Go 类型分发路由校验
	switch v := value.(type) {
	case map[string]any:
		p.validateObject(v, schema, path, diagnostics)
	case []any:
		p.validateArray(v, schema, path, diagnostics)
	case string:
		p.validateString(v, schema, path, diagnostics)
	case float64:
		p.validateNumber(v, schema, path, diagnostics)
	}
}

func (p *PluginConfigValidator) validateObject(value map[string]any, schema map[string]any, path string, diagnostics *[]PluginCompatibilityDiagnostic) {
	// 获取 properties 规则
	properties, _ := schema["properties"].(map[string]any)

	// 获取 required 规则
	var required []string
	if reqObj, ok := schema["required"].([]any); ok {
		for _, r := range reqObj {
			if s, ok := r.(string); ok && strings.TrimSpace(s) != "" {
				required = append(required, s)
			}
		}
	}

	// 获取 additionalProperties 规则 (默认是 true)
	allowAdditional := true
	if addProp, exists := schema["additionalProperties"]; exists {
		if b, ok := addProp.(bool); ok && !b {
			allowAdditional = false
		}
	}

	// 检查必填项
	for _, reqName := range required {
		if _, exists := value[reqName]; !exists {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"config_required_missing",
				fmt.Sprintf("Config value at '%s' is missing required property '%s'.", path, reqName),
				path,
			))
		}
	}

	// 检查每一项属性
	for propName, propVal := range value {
		if propSchema, exists := properties[propName]; exists {
			if subSchemaMap, ok := propSchema.(map[string]any); ok {
				p.validateValue(propVal, subSchemaMap, fmt.Sprintf("%s.%s", path, propName), diagnostics)
			}
		} else if !allowAdditional {
			*diagnostics = append(*diagnostics, makeDiagnostic(
				"config_additional_property",
				fmt.Sprintf("Config value at '%s' contains unsupported property '%s'.", path, propName),
				fmt.Sprintf("%s.%s", path, propName),
			))
		}
	}
}

func (p *PluginConfigValidator) validateArray(value []any, schema map[string]any, path string, diagnostics *[]PluginCompatibilityDiagnostic) {
	if minItems, ok := schema["minItems"].(float64); ok && float64(len(value)) < minItems {
		*diagnostics = append(*diagnostics, makeDiagnostic(
			"config_min_items",
			fmt.Sprintf("Config array at '%s' must contain at least %d items.", path, int(minItems)),
			path,
		))
	}

	if maxItems, ok := schema["maxItems"].(float64); ok && float64(len(value)) > maxItems {
		*diagnostics = append(*diagnostics, makeDiagnostic(
			"config_max_items",
			fmt.Sprintf("Config array at '%s' must contain at most %d items.", path, int(maxItems)),
			path,
		))
	}

	if itemsSchema, ok := schema["items"].(map[string]any); ok {
		for i, item := range value {
			p.validateValue(item, itemsSchema, fmt.Sprintf("%s[%d]", path, i), diagnostics)
		}
	}
}

func (p *PluginConfigValidator) validateString(value string, schema map[string]any, path string, diagnostics *[]PluginCompatibilityDiagnostic) {
	if minLength, ok := schema["minLength"].(float64); ok && float64(len(value)) < minLength {
		*diagnostics = append(*diagnostics, makeDiagnostic(
			"config_min_length",
			fmt.Sprintf("Config string at '%s' must be at least %d characters.", path, int(minLength)),
			path,
		))
	}

	if maxLength, ok := schema["maxLength"].(float64); ok && float64(len(value)) > maxLength {
		*diagnostics = append(*diagnostics, makeDiagnostic(
			"config_max_length",
			fmt.Sprintf("Config string at '%s' must be at most %d characters.", path, int(maxLength)),
			path,
		))
	}

	// 正则匹配
	if patternStr, ok := schema["pattern"].(string); ok {
		// Go 没有原生的正则超时控制，通常借助 context 或 channel 来实现超时。
		// 这里采用简单的 channel 包装来还原 C# 的 1 秒超时逻辑。
		ch := make(chan bool, 1)
		go func() {
			matched, err := regexp.MatchString(patternStr, value)
			if err != nil {
				ch <- false // 相当于捕获 ArgumentException
				return
			}
			ch <- matched
		}()

		select {
		case matched := <-ch:
			if !matched {
				*diagnostics = append(*diagnostics, makeDiagnostic("config_pattern_mismatch", fmt.Sprintf("Config string at '%s' does not match the required pattern.", path), path))
			}
		case <-time.After(1 * time.Second):
			*diagnostics = append(*diagnostics, makeDiagnostic("schema_pattern_timeout", fmt.Sprintf("Schema pattern at '%s' timed out during validation.", path), path))
		}
	}
}

func (p *PluginConfigValidator) validateNumber(value float64, schema map[string]any, path string, diagnostics *[]PluginCompatibilityDiagnostic) {
	if min, ok := schema["minimum"].(float64); ok && value < min {
		*diagnostics = append(*diagnostics, makeDiagnostic("config_minimum", fmt.Sprintf("Config number at '%s' must be >= %v.", path, min), path))
	}
	if max, ok := schema["maximum"].(float64); ok && value > max {
		*diagnostics = append(*diagnostics, makeDiagnostic("config_maximum", fmt.Sprintf("Config number at '%s' must be <= %v.", path, max), path))
	}
}

// 辅助工具：判断类型匹配
func matchesType(value any, typeStr string) bool {
	switch typeStr {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		v, ok := value.(float64)
		return ok && v == float64(int64(v))
	case "null":
		return value == nil
	default:
		return false
	}
}

func jsonElementsEqual(left, right any) bool {
	lBytes, _ := json.Marshal(left)
	rBytes, _ := json.Marshal(right)
	return string(lBytes) == string(rBytes)
}

func (p *PluginConfigValidator) countSchemaMatches(value any, schemas []any, path string) int {
	matches := 0
	for _, s := range schemas {
		if schemaMap, ok := s.(map[string]any); ok {
			var probeDiag []PluginCompatibilityDiagnostic
			p.validateValue(value, schemaMap, path, &probeDiag)
			if len(probeDiag) == 0 {
				matches++
			}
		}
	}
	return matches
}

func (p *PluginConfigValidator) validateSchemaArray(schema map[string]any, keyword string, path string, diagnostics *[]PluginCompatibilityDiagnostic) {
	if sub, exists := schema[keyword]; exists {
		if subSchemas, ok := sub.([]any); ok {
			for i, s := range subSchemas {
				if m, ok := s.(map[string]any); ok {
					p.validateSchemaObject(m, fmt.Sprintf("%s.%s[%d]", path, keyword, i), diagnostics)
				}
			}
		} else {
			*diagnostics = append(*diagnostics, makeDiagnostic("invalid_schema", fmt.Sprintf("Schema keyword '%s' at '%s' must be an array.", keyword, path), path))
		}
	}
}

func makeDiagnostic(code, message, path string) PluginCompatibilityDiagnostic {
	return PluginCompatibilityDiagnostic{Code: code, Message: message, Path: path}
}
