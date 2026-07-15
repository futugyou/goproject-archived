package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		if IsBlank(cap) {
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

type PluginDiscovery struct{}

var PluginDiscoveryInstance = &PluginDiscovery{}

const (
	MaxSymlinkResolutionDepth = 64
	ManifestFileName          = "openclaw.plugin.json"
	PackageJsonFileName       = "package.json"
)

func (p *PluginDiscovery) resolveRealPath(path string, visited map[string]struct{}, depth int) string {
	if depth >= MaxSymlinkResolutionDepth {
		return ""
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	_, ok := visited[full]
	visited[full] = struct{}{}

	if ok {
		return ""
	}
	root := filepath.VolumeName(full)

	if IsBlank(root) {
		return full
	}

	current := root
	remaining := full[len(root):]

	segments := strings.FieldsFunc(remaining, func(r rune) bool {
		return r == '/' || r == '\\'
	})

	for _, segment := range segments {
		if filepath.IsAbs(segment) {
			p, _ := filepath.Abs(segment)
			return p
		}
		current = filepath.Join(current, segment)
		resolved, _ := TryResolveLinkTarget(current)
		if !IsBlank(resolved) {
			current = p.resolveRealPath(resolved, visited, depth+1)
		}
	}

	current, _ = filepath.Abs(current)
	return current
}

func (p *PluginDiscovery) TryResolveContainedPath(rootPath, relativePath string) (string, bool) {
	if filepath.IsAbs(relativePath) {
		return "", false
	}

	candidatePath, err := filepath.Abs(filepath.Join(rootPath, relativePath))
	if err != nil {
		return "", false
	}
	if IsUnresolvedLink(candidatePath) {
		return "", false
	}

	resolvedPath := candidatePath
	rootful, err := filepath.Abs(rootPath)
	if err != nil {
		return "", false
	}
	var fullRoot = filepath.Clean(rootful)

	// Resolve symlinks for both paths to prevent symlink-based escape from the root.
	resolvedPath = p.resolveRealPath(resolvedPath, map[string]struct{}{}, 0)
	fullRoot = p.resolveRealPath(fullRoot, map[string]struct{}{}, 0)

	if PathEqual(resolvedPath, fullRoot) {
		return resolvedPath, true
	}

	prefix := fullRoot + string(os.PathSeparator)
	return resolvedPath, PathHasPrefix(resolvedPath, prefix)
}

func (p *PluginDiscovery) findEntryFile(pluginRoot string) (string, *PluginCompatibilityDiagnostic) {
	// Check common entry points
	candidates := []string{
		"index.ts", "index.js", "index.mjs",
		"src/index.ts", "src/index.js", "src/index.mjs",
	}

	for _, candidate := range candidates {
		var path = filepath.Join(pluginRoot, candidate)
		if FileExists(path) {
			return path, nil
		}

	}

	fileback := func(pluginRoot string) string {
		extensions := []string{"*.ts", "*.js", "*.mjs"}
		for _, ext := range extensions {
			pattern := filepath.Join(pluginRoot, ext)
			files, err := filepath.Glob(pattern)
			if err != nil {
				return ""
			}
			if len(files) == 1 {
				return files[0]
			}
		}
		return ""
	}

	// Check package.json for openclaw.extensions
	var packageJson = filepath.Join(pluginRoot, PackageJsonFileName)
	if FileExists(packageJson) {
		data, err := os.ReadFile(packageJson)
		if err != nil {
			return fileback(pluginRoot), nil
		}

		var config struct {
			Openclaw struct {
				Extensions []string `json:"extensions"`
			} `json:"openclaw"`
		}

		if err := json.Unmarshal(data, &config); err == nil {
			for _, relPath := range config.Openclaw.Extensions {
				if relPath == "" {
					continue
				}

				entryPath, ok := p.TryResolveContainedPath(pluginRoot, relPath)
				if !ok {
					path, err := filepath.Abs(pluginRoot)
					if err != nil {
						return "", nil
					}

					return pluginRoot, &PluginCompatibilityDiagnostic{
						Code:    "entry_outside_root",
						Message: fmt.Sprintf("package entry '%s' resolves outside the plugin root", relPath),
						Path:    path,
					}
				}

				if FileExists(entryPath) {
					return entryPath, nil
				}
			}
		}
	}

	return fileback(pluginRoot), nil
}

func (p *PluginDiscovery) tryAddPluginPack(dir, packageJsonPath string, seen map[string]struct{}, result *PluginDiscoveryResult) {
	data, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return
	}

	var config struct {
		Openclaw struct {
			Extensions []string `json:"extensions"`
		} `json:"openclaw"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return
	}

	packName := config.Name
	if IsBlank(packName) {
		packName = filepath.Base(dir)
	}

	for _, relPath := range config.Openclaw.Extensions {
		if IsBlank(relPath) {
			continue
		}
		var fileBase = GetFileNameWithoutExtension(relPath)
		var pluginId = packName
		if len(config.Openclaw.Extensions) > 1 {
			pluginId = fmt.Sprintf("%s/%s", packName, fileBase)
		}

		entryPath, ok := p.TryResolveContainedPath(dir, relPath)
		if !ok {
			result.Reports = append(result.Reports, PluginLoadReport{
				PluginId:   pluginId,
				SourcePath: PathGetFullPath(dir),
				EntryPath:  PathGetFullPath(filepath.Join(dir, relPath)),
				Loaded:     false,
				Diagnostics: []PluginCompatibilityDiagnostic{
					{
						Code:    "entry_outside_root",
						Message: fmt.Sprintf("package entry '%s' for plugin '%s' resolves outside the plugin root", relPath, pluginId),
						Path:    PathGetFullPath(dir),
					},
				},
			})
			continue
		}

		if !FileExists(entryPath) {
			result.Reports = append(result.Reports, PluginLoadReport{
				PluginId:   pluginId,
				SourcePath: PathGetFullPath(dir),
				EntryPath:  entryPath,
				Loaded:     false,
				Diagnostics: []PluginCompatibilityDiagnostic{
					{
						Code:    "entry_not_found",
						Message: fmt.Sprintf("package entry '%s' for plugin '%s' does not exist", relPath, pluginId),
						Path:    entryPath,
					},
				},
			})
			continue
		}

		_, found := seen[pluginId]
		seen[pluginId] = struct{}{}
		if found {
			result.Reports = append(result.Reports, PluginLoadReport{
				PluginId:   pluginId,
				SourcePath: PathGetFullPath(dir),
				EntryPath:  entryPath,
				Loaded:     false,
				Diagnostics: []PluginCompatibilityDiagnostic{
					{
						Code:    "duplicate_plugin_id",
						Message: fmt.Sprintf("plugin id '%s' was discovered more than once. Later entries are skipped", pluginId),
						Path:    entryPath,
					},
				},
			})
			continue
		}

		entryDir := filepath.Dir(entryPath)
		if IsBlank(entryDir) {
			entryDir = dir
		}

		var entryManifestPath = filepath.Join(entryDir, ManifestFileName)
		var manifest PluginManifest = PluginManifest{
			ID: pluginId,
		}

		data, err := os.ReadFile(entryManifestPath)
		if err == nil {
			json.Unmarshal(data, &manifest)
		}

		if IsBlank(manifest.ID) {
			manifest = PluginManifest{
				ID: pluginId,
			}
		}

		result.Plugins = append(result.Plugins, DiscoveredPlugin{
			Manifest:  manifest,
			RootPath:  PathGetFullPath(dir),
			EntryPath: entryPath,
		})
	}
}

func (p *PluginDiscovery) tryAddPluginFromManifest(pluginRoot, manifestPath string, seen map[string]struct{}, result *PluginDiscoveryResult) {
	var manifest PluginManifest
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		err = json.Unmarshal(data, &manifest)
	}

	if err != nil {
		result.Reports = append(result.Reports, PluginLoadReport{
			PluginId:   filepath.Base(pluginRoot),
			SourcePath: PathGetFullPath(pluginRoot),
			Diagnostics: []PluginCompatibilityDiagnostic{
				{
					Code:    "invalid_manifest",
					Message: fmt.Sprintf("failed to parse manifest '%s'", manifestPath),
					Path:    PathGetFullPath(manifestPath),
				},
			},
		})
		return
	}

	if IsBlank(manifest.ID) {
		return
	}

	_, found := seen[manifest.ID]
	seen[manifest.ID] = struct{}{}

	if found {
		result.Reports = append(result.Reports, PluginLoadReport{
			PluginId:   manifest.ID,
			SourcePath: PathGetFullPath(pluginRoot),
			Diagnostics: []PluginCompatibilityDiagnostic{
				{
					Code:    "duplicate_plugin_id",
					Message: fmt.Sprintf("plugin id '%s' was discovered more than once. Later entries are skipped", manifest.ID),
					Path:    PathGetFullPath(manifestPath),
				},
			},
		})
		return
	}

	// Find entry file
	var entryPath, entryDiagnostic = p.findEntryFile(pluginRoot)
	if IsBlank(entryPath) {
		code := "entry_not_found"
		message := fmt.Sprintf("no plugin entry file was found for '%s'. Expected index.ts, index.js, index.mjs, src/index.*, or a package.json openclaw.extensions entry", manifest.ID)
		path := PathGetFullPath(pluginRoot)

		if entryDiagnostic != nil {
			if !IsBlank(entryDiagnostic.Code) {
				code = entryDiagnostic.Code
			}
			if !IsBlank(entryDiagnostic.Message) {
				message = entryDiagnostic.Message
			}
			if !IsBlank(entryDiagnostic.Path) {
				path = entryDiagnostic.Path
			}
		}

		result.Reports = append(result.Reports, PluginLoadReport{
			PluginId:   manifest.ID,
			SourcePath: PathGetFullPath(pluginRoot),
			Diagnostics: []PluginCompatibilityDiagnostic{
				{
					Code:    code,
					Message: message,
					Path:    path,
				},
			},
		})
		return
	}

	rel, err := filepath.Rel(pluginRoot, entryPath)
	var containedEntryPath string
	var ok bool
	if err == nil {
		containedEntryPath, ok = p.TryResolveContainedPath(pluginRoot, rel)
		if !ok {
			result.Reports = append(result.Reports, PluginLoadReport{
				PluginId:   manifest.ID,
				SourcePath: PathGetFullPath(pluginRoot),
				EntryPath:  PathGetFullPath(entryPath),
				Diagnostics: []PluginCompatibilityDiagnostic{
					{
						Code:    "entry_outside_root",
						Message: fmt.Sprintf("plugin entry file for '%s' resolves outside the plugin root", manifest.ID),
						Path:    PathGetFullPath(entryPath),
					},
				},
			})
			return
		}
	}

	result.Plugins = append(result.Plugins, DiscoveredPlugin{
		Manifest:  manifest,
		RootPath:  PathGetFullPath(pluginRoot),
		EntryPath: containedEntryPath,
	})
}

func (p *PluginDiscovery) tryAddPluginFromFile(path string, seen map[string]struct{}, result *PluginDiscoveryResult) {
	var dir = filepath.Dir(path)
	if IsBlank(dir) {
		return
	}

	var manifestPath = filepath.Join(dir, ManifestFileName)
	if FileExists(manifestPath) {
		p.tryAddPluginFromManifest(dir, manifestPath, seen, result)
	} else {
		// Standalone file — use file base name as id
		var id = GetFileNameWithoutExtension(path)
		_, found := seen[id]
		seen[id] = struct{}{}

		if found {
			return
		}

		result.Plugins = append(result.Plugins, DiscoveredPlugin{
			Manifest:  PluginManifest{ID: id},
			RootPath:  dir,
			EntryPath: PathGetFullPath(path),
		})
	}
}

func (p *PluginDiscovery) scanDirectory(dir string, seen map[string]struct{}, result *PluginDiscoveryResult) {
	// Check if this directory is itself a plugin (has manifest)
	var manifestPath = filepath.Join(dir, ManifestFileName)
	if FileExists(manifestPath) {
		p.tryAddPluginFromManifest(dir, manifestPath, seen, result)
		return
	}

	// Check for package pack (package.json with openclaw.extensions)
	var packageJsonPath = filepath.Join(dir, PackageJsonFileName)
	if FileExists(packageJsonPath) {
		p.tryAddPluginPack(dir, packageJsonPath, seen, result)
		return
	}

	// Scan subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			p.scanDirectory(subDir, seen, result)
		}
	}
}

func (p *PluginDiscovery) scanExtensionsDirectory(extensionsDir string, seen map[string]struct{}, result *PluginDiscoveryResult) {
	entries, err := os.ReadDir(extensionsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".ts" || ext == ".js" || ext == ".mjs" {
				filePath := filepath.Join(extensionsDir, entry.Name())
				p.tryAddPluginFromFile(filePath, seen, result)
			}
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(extensionsDir, entry.Name())

			indexTs := filepath.Join(subDir, "index.ts")
			indexJs := filepath.Join(subDir, "index.js")
			indexMjs := filepath.Join(subDir, "index.mjs")

			if _, err := os.Stat(indexTs); err == nil {
				p.tryAddPluginFromFile(indexTs, seen, result)
			} else if _, err := os.Stat(indexJs); err == nil {
				p.tryAddPluginFromFile(indexJs, seen, result)
			} else if _, err := os.Stat(indexMjs); err == nil {
				p.tryAddPluginFromFile(indexMjs, seen, result)
			}
		}
	}
}

func (p *PluginDiscovery) Filter(discovered []DiscoveredPlugin, pluginsConfig *PluginsConfig) []DiscoveredPlugin {
	var result = []DiscoveredPlugin{}

	for _, plugin := range discovered {
		var id = plugin.Manifest.ID

		if pluginsConfig != nil {
			if slices.Contains(pluginsConfig.Deny, id) {
				continue
			}

			if len(pluginsConfig.Allow) > 0 && !slices.Contains(pluginsConfig.Allow, id) {
				continue
			}

			if entry, ok := pluginsConfig.Entries[id]; ok && entry != nil && !entry.Enabled {
				continue
			}
		}

		// Slot exclusivity check
		if plugin.Manifest.Kind != "" {
			if slotWinner, ok := pluginsConfig.Slots[plugin.Manifest.Kind]; ok {
				if slotWinner == "none" || slotWinner != id {
					continue
				}
			}

		}

		result = append(result, plugin)
	}

	return result
}

func (p *PluginDiscovery) DiscoverWithDiagnostics(pluginsConfig *PluginsConfig, workspacePath string) *PluginDiscoveryResult {
	seen := map[string]struct{}{}
	var result = &PluginDiscoveryResult{}

	// 1. Config paths
	for _, configPath := range pluginsConfig.Load.Paths {
		var expanded = ExpandAllEnv(configPath)
		if strings.HasPrefix(expanded, "~") {

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return result
			}
			subPath := strings.TrimLeft(expanded[1:], "/\\")
			expanded = filepath.Join(homeDir, subPath)
		}

		if FileExists(expanded) {
			p.tryAddPluginFromFile(expanded, seen, result)
		} else {
			p.scanDirectory(expanded, seen, result)
		}
	}

	// 2. Workspace extensions
	if !IsBlank(workspacePath) {
		var wsExtDir = filepath.Join(workspacePath, ".openclaw", "extensions")
		if DirectoryExists(wsExtDir) {
			p.scanExtensionsDirectory(wsExtDir, seen, result)
		}
	}

	// 3. Global extensions
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	var globalExtDir = filepath.Join(homeDir, ".openclaw", "extensions")
	if DirectoryExists(globalExtDir) {
		p.scanExtensionsDirectory(globalExtDir, seen, result)
	}

	return result
}

func (p *PluginDiscovery) Discover(pluginsConfig *PluginsConfig, workspacePath string) []DiscoveredPlugin {
	result := p.DiscoverWithDiagnostics(pluginsConfig, workspacePath)
	if result != nil {
		return result.Plugins
	}
	return []DiscoveredPlugin{}
}
