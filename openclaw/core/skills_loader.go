package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type SkillLoader struct{}

func (s *SkillLoader) ParseSkillFile(filePath, skillDir string, source SkillSource) (*SkillDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return s.ParseSkillContent(string(data), skillDir, source)
}

func (s *SkillLoader) ParseSkillContent(content, skillDir string, source SkillSource) (*SkillDefinition, error) {
	if !strings.HasPrefix(content, "---") {
		return nil, errors.New("missing frontmatter delimiter")
	}
	if len(content) < 3 {
		return nil, errors.New("content too short")
	}

	endIndex := strings.Index(content[3:], "\n---")
	if endIndex < 0 {
		return nil, errors.New("missing frontmatter closing delimiter")
	}
	endIndex += 3

	frontmatter := strings.TrimSpace(content[3:endIndex])

	var body string
	if endIndex+4 < len(content) {
		body = strings.TrimSpace(content[endIndex+4:])
	}

	// Parse frontmatter lines
	var (
		name                   string
		description            string
		metadataJson           string
		userInvocable          = true
		disableModelInvocation = false
		kind                   = SkillKind_Standard
		triggers               []string
		metaPriority           int
		finalTextMode          string
		compositionJson        string
		commandDispatch        string
		commandTool            string
		commandArgMode         string
		homepage               string
	)

	frontmatterLines := strings.Split(frontmatter, "\n")
	for lineIndex := 0; lineIndex < len(frontmatterLines); lineIndex++ {
		rawLine := strings.TrimSuffix(frontmatterLines[lineIndex], "\r")

		if strings.TrimSpace(rawLine) != "" && s.GetIndent(rawLine) != 0 {
			continue
		}

		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			name = s.NormalizeFrontmatterScalar(value)
		case "description":
			description = s.NormalizeFrontmatterScalar(value)
		case "metadata":
			metadataJson = value
		case "user-invocable":
			userInvocable = !strings.EqualFold(value, "false")
		case "disable-model-invocation":
			disableModelInvocation = strings.EqualFold(value, "true")
		case "kind":
			var err error
			kind, err = s.TryParseSkillKind(s.NormalizeFrontmatterScalar(value))
			if err != nil {
				return nil, err
			}
		case "triggers":
			if strings.TrimSpace(value) == "" {
				triggerBlock, consumedLines := s.CollectIndentedBlock(frontmatterLines, lineIndex+1)
				lineIndex += consumedLines

				triggerJson, err := s.TryConvertYamlBlockToJson(triggerBlock)
				if err != nil {
					return nil, err
				}
				value = triggerJson
			}

			var success bool
			triggers, success = s.TryParseStringList(value)
			if !success {
				return nil, errors.New("TryParseStringList error")
			}
		case "meta-priority", "meta_priority":
			parsedMetaPriority, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			metaPriority = parsedMetaPriority
		case "final-text-mode", "final_text_mode":
			finalTextMode = s.NormalizeFrontmatterScalar(value)
		case "composition":
			if strings.TrimSpace(value) == "" {
				compositionBlock, consumedLines := s.CollectIndentedBlock(frontmatterLines, lineIndex+1)
				lineIndex += consumedLines

				var err error
				compositionJson, err = s.TryConvertYamlBlockToJson(compositionBlock)
				if err != nil {
					return nil, err
				}
			} else {
				compositionJson = value
			}
		case "command-dispatch":
			commandDispatch = s.NormalizeFrontmatterScalar(value)
		case "command-tool":
			commandTool = s.NormalizeFrontmatterScalar(value)
		case "command-arg-mode":
			commandArgMode = s.NormalizeFrontmatterScalar(value)
		case "homepage":
			homepage = s.NormalizeFrontmatterScalar(value)
		}
	}

	if strings.TrimSpace(name) == "" {
		return nil, errors.New("skill name is required")
	}

	metadata := s.ParseMetadata(metadataJson)
	if homepage != "" && metadata.Homepage == "" {
		metadata.Homepage = homepage
	}

	var composition *MetaSkillComposition
	if kind == SkillKind_Meta {
		if strings.TrimSpace(compositionJson) == "" {
			return nil, errors.New("meta skill requires a composition")
		}

		composition, _ = s.ParseComposition(compositionJson)
		if composition == nil || len(composition.Steps) == 0 {
			return nil, errors.New("invalid or empty composition steps")
		}

		if !s.ValidateFinalTextMode(finalTextMode, composition.Steps) {
			return nil, errors.New("final text mode validation failed")
		}
	}

	// Replace {baseDir} placeholder in instructions
	body = strings.ReplaceAll(body, "{baseDir}", skillDir)

	// Scan resources
	resources := s.ScanSkillResources(skillDir)

	if triggers == nil {
		triggers = []string{}
	}

	return &SkillDefinition{
		Name:                   name,
		Description:            description,
		Instructions:           body,
		Location:               skillDir,
		Source:                 source,
		Metadata:               &metadata,
		Kind:                   kind,
		Triggers:               triggers,
		MetaPriority:           metaPriority,
		FinalTextMode:          finalTextMode,
		Composition:            composition,
		UserInvocable:          userInvocable,
		DisableModelInvocation: disableModelInvocation,
		CommandDispatch:        commandDispatch,
		CommandTool:            commandTool,
		CommandArgMode:         commandArgMode,
		Resources:              resources,
	}, nil
}

func (s *SkillLoader) GetIndent(line string) int {
	var indent = 0
	for {
		if indent >= len(line) || line[indent] != ' ' {
			break
		}
		indent++
	}

	return indent
}

func (s *SkillLoader) NormalizeFrontmatterScalar(rawValue string) string {
	var value = strings.TrimSpace(rawValue)
	l := len(value)
	if l >= 2 && ((value[0] == '"' && value[l-1] == '"') || (value[0] == '\'' && value[l-1] == '\'')) {
		return s.UnquoteYamlScalar(value)
	}

	return value
}

func (s *SkillLoader) UnquoteYamlScalar(value string) string {
	if len(value) < 2 {
		return value
	}

	// 处理单引号情况：单引号内连续的两个单引号 '' 代表一个转义的单引号 '
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		content := value[1 : len(value)-1]
		return strings.ReplaceAll(content, "''", "'")
	}

	// 如果不是双引号包裹，直接返回原字符串
	if value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}

	var builder strings.Builder
	builder.Grow(len(value) - 2)

	length := len(value)
	for i := 1; i < length-1; i++ {
		character := value[i]

		// 如果当前字符不是反斜杠，或者反斜杠是最后一个有效内容（即紧邻末尾双引号）
		if character != '\\' || i+1 >= length-1 {
			builder.WriteByte(character)
			continue
		}

		// 遇到转义字符，跳到下一个字符
		i++
		escaped := value[i]
		switch escaped {
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case '"':
			builder.WriteByte('"')
		case '\\':
			builder.WriteByte('\\')
		default:
			builder.WriteByte(escaped)
		}
	}

	return builder.String()
}

func (s *SkillLoader) TryParseSkillKind(rawValue string) (SkillKind, error) {
	kind := SkillKind_Standard
	if len(rawValue) == 0 {
		return kind, fmt.Errorf("%s is not the correct kind.", rawValue)
	}
	switch strings.ToLower(rawValue) {
	case "standard":
		return kind, nil
	case "meta":
		return SkillKind_Meta, nil
	}
	return kind, fmt.Errorf("%s is not the correct kind.", rawValue)
}

func (s *SkillLoader) CollectIndentedBlock(lines []string, startIndex int) (string, int) {
	consumedLines := 0
	var sb strings.Builder
	for i := startIndex; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if len(line) > 0 && s.GetIndent(line) == 0 {
			break
		}
		sb.WriteString(line)
		sb.WriteString("\n")
		consumedLines++
	}

	return sb.String(), consumedLines
}

func (s *SkillLoader) TryConvertYamlBlockToJson(yaml string) (string, error) {
	if len(yaml) == 0 {
		return "", errors.New("yaml string can not be empty")
	}

	lines := []string{}

	for _, v := range strings.Split(yaml, "\n") {
		lines = append(lines, strings.TrimRight(v, "\r"))
	}

	var index = 0
	index = s.SkipYamlBlankLines(lines, index)
	if index >= len(lines) {
		return "", errors.New("yaml string format error")
	}

	var indent = s.GetIndent(lines[index])
	index, f, node := s.TryParseYamlNode(lines, index, indent)
	if !f || node == nil {
		return "", errors.New("yaml string format error")
	}

	index = s.SkipYamlBlankLines(lines, index)
	if index < len(lines) {
		return "", errors.New("yaml string format error")
	}

	var buf bytes.Buffer

	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	err := node.WriteTo(encoder)
	if err != nil {
		return "", err
	}
	jsonStr := buf.String()
	return jsonStr, nil
}

func (s *SkillLoader) SkipYamlBlankLines(lines []string, index int) int {
	for {
		if index >= len(lines) || !s.IsYamlIgnorableLine(lines[index]) {
			break
		}
		index++
	}
	return index
}

func (s *SkillLoader) IsYamlIgnorableLine(line string) bool {
	return len(line) == 0 || strings.HasPrefix(strings.TrimSpace(line), "#")
}

func (s *SkillLoader) TryParseYamlNode(lines []string, index int, indent int) (int, bool, SkillYamlNode) {
	index = s.SkipYamlBlankLines(lines, index)
	if index >= len(lines) {
		return index, false, nil
	}

	line := lines[index]
	lineIndent := s.GetIndent(line)
	if lineIndent < indent {
		return index, false, nil
	}

	text := strings.TrimRight(line[lineIndent:], " \t\r\n")
	if strings.HasPrefix(text, "- ") {
		return s.TryParseYamlArray(lines, index, lineIndent)
	}
	return s.TryParseYamlMapping(lines, index, lineIndent)
}

func (s *SkillLoader) TrySplitYamlKeyValue(text string) (bool, string, string) {
	colonIndex := strings.IndexByte(text, ':')
	if colonIndex <= 0 {
		return false, "", ""
	}

	key := strings.TrimSpace(text[:colonIndex])
	value := strings.TrimSpace(text[colonIndex+1:])

	if len(key) == 0 {
		return false, "", ""
	}
	return true, key, value
}

func (s *SkillLoader) TryParseYamlValue(lines []string, index int, parentIndent int, rawValue string) (int, bool, SkillYamlNode) {
	var success bool
	var node SkillYamlNode
	index, success, node = s.TryParseYamlInlineValueOrLiteral(lines, index, parentIndent, rawValue)
	if success {
		return index, true, node
	}
	return index, false, nil
}

func (s *SkillLoader) IsYamlLiteralIndicator(value string) bool {
	return value == "|" || value == "|-" || value == "|+" || strings.HasPrefix(value, "|")
}

func (s *SkillLoader) ParseYamlLiteralBlock(lines []string, index, parentIndent int) (int, string) {
	literalLines := []string{}
	var contentIndent = -1

	for {
		if index > len(lines) {
			break
		}
		var line = lines[index]
		if IsBlank(line) {
			literalLines = append(literalLines, "")
			index++
			continue
		}

		var lineIndent = s.GetIndent(line)
		if lineIndent <= parentIndent {
			break
		}

		if contentIndent < 0 {
			contentIndent = lineIndent
		}

		var remove = min(contentIndent, len(line))
		literalLines = append(literalLines, strings.TrimRightFunc(line[remove:], unicode.IsSpace))
		index++
	}

	return index, strings.Join(literalLines, "\n")
}

func (s *SkillLoader) TryParseYamlInlineValueOrLiteral(lines []string, index int, parentIndent int, rawValue string) (int, bool, SkillYamlNode) {
	value := strings.TrimSpace(rawValue)

	if s.IsYamlLiteralIndicator(value) {
		index++
		var literalStr string
		index, literalStr = s.ParseYamlLiteralBlock(lines, index, parentIndent)
		return index, true, &SkillYamlScalarNode{Value: literalStr}
	}

	if value == "" {
		index++
		childIndex := s.FindNextYamlContentLine(lines, index)
		if childIndex < 0 || s.GetIndent(lines[childIndex]) <= parentIndent {
			return index, true, &SkillYamlScalarNode{Value: ""}
		}

		childIndent := s.GetIndent(lines[childIndex])
		return s.TryParseYamlNode(lines, index, childIndent)
	}

	node := s.ParseYamlScalar(value)
	index++
	return index, true, node
}

func (s *SkillLoader) SplitYamlInlineArray(content string) []string {
	var result []string
	start := 0
	var quote rune = 0
	escaped := false

	runes := []rune(content)

	for index := 0; index < len(runes); index++ {
		character := runes[index]

		if quote != 0 {
			if quote == '"' && character == '\\' && !escaped {
				escaped = true
				continue
			}

			if character == quote && !escaped {
				quote = 0
			}

			escaped = false
			continue
		}

		if character == '"' || character == '\'' {
			quote = character
			continue
		}

		if character != ',' {
			continue
		}

		item := strings.TrimSpace(string(runes[start:index]))
		result = append(result, item)
		start = index + 1
	}

	item := strings.TrimSpace(string(runes[start:]))
	result = append(result, item)

	return result
}

func (s *SkillLoader) ParseYamlInlineArray(rawValue string) SkillYamlNode {
	var content = strings.TrimRight(strings.TrimLeft(rawValue, "["), "]")
	if len(content) == 0 {
		return &SkillYamlArrayNode{}
	}

	items := []SkillYamlNode{}
	for _, v := range s.SplitYamlInlineArray(content) {
		items = append(items, s.ParseYamlScalar(v))
	}

	return &SkillYamlArrayNode{Items: items}
}

func (s *SkillLoader) ParseYamlScalar(rawValue string) SkillYamlNode {
	var value = strings.TrimSpace(rawValue)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return s.ParseYamlInlineArray(value)
	}

	if value == "true" {
		return &SkillYamlScalarNode{Value: true}
	}

	if value == "false" {
		return &SkillYamlScalarNode{Value: false}
	}

	if value == "null" || value == "~" {
		return &SkillYamlScalarNode{Value: nil}
	}

	if longValue, err := strconv.ParseInt(value, 10, 64); err == nil {
		return &SkillYamlScalarNode{Value: longValue}
	}

	if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
		return &SkillYamlScalarNode{Value: floatValue}
	}

	return &SkillYamlScalarNode{Value: s.UnquoteYamlScalar(value)}
}

func (s *SkillLoader) FindNextYamlContentLine(lines []string, startIndex int) int {
	for index := startIndex; index < len(lines); index++ {
		if !s.IsYamlIgnorableLine(lines[index]) {
			return index
		}
	}

	return -1
}

func (s *SkillLoader) NormalizeYamlPropertyName(key string) string {
	switch key {
	case "skill_exec_entrypoint":
		return "entrypoint"
	case "skill_exec_args":
		return "args"
	case "skill_exec_stdin":
		return "stdin"
	case "skill_exec_cwd":
		return "cwd"
	case "skill_exec_parse_mode":
		return "parse_mode"
	default:
		return key
	}
}

func (s *SkillLoader) TryParseYamlMapping(lines []string, linesIndex int, indent int) (int, bool, SkillYamlNode) {
	var properties []PropertyPair
	index := linesIndex

	for index < len(lines) {
		index = s.SkipYamlBlankLines(lines, index)
		if index >= len(lines) {
			break
		}

		line := lines[index]
		lineIndent := s.GetIndent(line)
		if lineIndent < indent {
			break
		}
		if lineIndent > indent {
			return index, false, nil
		}

		text := strings.TrimRight(line[lineIndent:], " \t\r\n")
		if strings.HasPrefix(text, "- ") {
			break
		}

		success, key, value := s.TrySplitYamlKeyValue(text)
		if !success {
			return index, false, nil
		}

		var ok bool
		var valueNode SkillYamlNode
		index, ok, valueNode = s.TryParseYamlValue(lines, index, indent, value)
		if !ok || valueNode == nil {
			return index, false, nil
		}

		properties = append(properties, PropertyPair{
			Key:   s.NormalizeYamlPropertyName(key),
			Value: valueNode,
		})
	}

	return index, true, &SkillYamlObjectNode{Properties: properties}
}

func (s *SkillLoader) TryParseYamlArray(lines []string, linesIndex int, indent int) (int, bool, SkillYamlNode) {
	var items []SkillYamlNode
	index := linesIndex

	for index < len(lines) {
		index = s.SkipYamlBlankLines(lines, index)
		if index >= len(lines) {
			break
		}

		line := lines[index]
		lineIndent := s.GetIndent(line)
		if lineIndent < indent {
			break
		}
		if lineIndent > indent {
			return index, false, nil
		}

		text := strings.TrimRight(line[lineIndent:], " \t\r\n")
		if !strings.HasPrefix(text, "- ") {
			break
		}

		itemText := strings.TrimSpace(text[2:])
		if itemText == "" {
			index++
			childIndex := s.FindNextYamlContentLine(lines, index)
			if childIndex < 0 || s.GetIndent(lines[childIndex]) <= indent {
				items = append(items, &SkillYamlScalarNode{Value: ""})
				continue
			}

			childIndent := s.GetIndent(lines[childIndex])
			var ok bool
			var childNode SkillYamlNode
			index, ok, childNode = s.TryParseYamlNode(lines, index, childIndent)
			if !ok || childNode == nil {
				return index, false, nil
			}

			items = append(items, childNode)
			continue
		}

		if success, key, value := s.TrySplitYamlKeyValue(itemText); success {
			var properties []PropertyPair
			var ok bool
			var firstValue SkillYamlNode
			index, ok, firstValue = s.TryParseYamlInlineValueOrLiteral(lines, index, indent, value)
			if !ok || firstValue == nil {
				return index, false, nil
			}

			properties = append(properties, PropertyPair{
				Key:   s.NormalizeYamlPropertyName(key),
				Value: firstValue,
			})

			childIndex := s.FindNextYamlContentLine(lines, index)
			if childIndex >= 0 && s.GetIndent(lines[childIndex]) > indent {
				childIndent := s.GetIndent(lines[childIndex])
				var continuationNode SkillYamlNode
				index, ok, continuationNode = s.TryParseYamlMapping(lines, index, childIndent)

				continuationObject, isObject := continuationNode.(*SkillYamlObjectNode)
				if !ok || !isObject {
					return index, false, nil
				}

				properties = append(properties, continuationObject.Properties...)
			}

			items = append(items, &SkillYamlObjectNode{Properties: properties})
			continue
		}

		items = append(items, s.ParseYamlScalar(itemText))
		index++

		nextIndex := s.FindNextYamlContentLine(lines, index)
		if nextIndex >= 0 && s.GetIndent(lines[nextIndex]) > indent {
			return index, false, nil
		}
	}

	return index, true, &SkillYamlArrayNode{Items: items}
}

func (s *SkillLoader) TryParseStringList(rawValue string) ([]string, bool) {
	if IsBlank(rawValue) {
		return nil, false
	}

	var rawArray []any
	if err := json.Unmarshal([]byte(rawValue), &rawArray); err != nil {
		return nil, false
	}

	var values []string
	for _, item := range rawArray {
		strItem, ok := item.(string)
		if !ok || IsBlank(strItem) {
			return nil, false
		}
		values = append(values, strItem)
	}

	return values, true
}

func (s *SkillLoader) ParseMetadata(jsonStr string) SkillMetadata {
	if IsBlank(jsonStr) {
		return SkillMetadata{}
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		return SkillMetadata{}
	}

	var oc json.RawMessage
	var exists bool

	if oc, exists = doc["openclaw"]; !exists {
		if oc, exists = doc["opensquilla"]; !exists {
			return SkillMetadata{}
		}
	}

	var ocMap map[string]json.RawMessage
	if err := json.Unmarshal(oc, &ocMap); err != nil {
		return SkillMetadata{}
	}

	meta := SkillMetadata{}

	if val, ok := ocMap["always"]; ok {
		_ = json.Unmarshal(val, &meta.Always)
	}
	if val, ok := ocMap["emoji"]; ok {
		_ = json.Unmarshal(val, &meta.Emoji)
	}
	if val, ok := ocMap["homepage"]; ok {
		_ = json.Unmarshal(val, &meta.Homepage)
	}
	if val, ok := ocMap["primaryEnv"]; ok {
		_ = json.Unmarshal(val, &meta.PrimaryEnv)
	}
	if val, ok := ocMap["skillKey"]; ok {
		_ = json.Unmarshal(val, &meta.SkillKey)
	}
	if val, ok := ocMap["risk"]; ok {
		var riskStr string
		if err := json.Unmarshal(val, &riskStr); err == nil {
			meta.Risk = riskStr
		}
	}
	if val, ok := ocMap["capabilities"]; ok {
		meta.Capabilities = ReadStringArray(val)
	}
	if val, ok := ocMap["os"]; ok {
		meta.Os = ReadStringArray(val)
	}

	if val, ok := ocMap["requires"]; ok {
		var reqMap map[string]json.RawMessage
		if err := json.Unmarshal(val, &reqMap); err == nil {
			if bins, ok := reqMap["bins"]; ok {
				meta.RequireBins = ReadStringArray(bins)
			}
			if anyBins, ok := reqMap["anyBins"]; ok {
				meta.RequireAnyBins = ReadStringArray(anyBins)
			}
			if env, ok := reqMap["env"]; ok {
				meta.RequireEnv = ReadStringArray(env)
			}
			if cfg, ok := reqMap["config"]; ok {
				meta.RequireConfig = ReadStringArray(cfg)
			}
		}
	}

	return meta
}

func (s *SkillLoader) ParseComposition(jsonStr string) (*MetaSkillComposition, string) {
	if IsBlank(jsonStr) {
		return nil, "invalid_meta_composition"
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		return nil, "invalid_meta_composition"
	}

	// 校验 steps 是否存在且为数组
	stepsRaw, hasSteps := doc["steps"]
	if !hasSteps {
		return nil, "invalid_meta_composition"
	}
	var stepsArray []map[string]json.RawMessage
	if err := json.Unmarshal(stepsRaw, &stepsArray); err != nil {
		return nil, "invalid_meta_composition"
	}

	var toolArgsJson string
	if compositionToolArgsElement, ok := doc["tool_args"]; ok {
		var objCheck map[string]any
		if err := json.Unmarshal(compositionToolArgsElement, &objCheck); err != nil {
			return nil, "invalid_tool_args"
		}
		rawText := string(compositionToolArgsElement)
		toolArgsJson = rawText
	}

	var steps []MetaSkillStepDefinition
	for _, stepElement := range stepsArray {
		idRaw, hasId := stepElement["id"]
		var id string
		if !hasId || json.Unmarshal(idRaw, &id) != nil || IsBlank(id) {
			return nil, "invalid_meta_composition"
		}

		var kind string
		if kindRaw, ok := stepElement["kind"]; ok {
			_ = json.Unmarshal(kindRaw, &kind)
		} else if typeRaw, ok := stepElement["type"]; ok {
			_ = json.Unmarshal(typeRaw, &kind)
		}

		if IsBlank(kind) {
			return nil, "invalid_meta_composition"
		}

		// 获取 depends_on
		var dependsOn []string
		if dependsOnRaw, ok := stepElement["depends_on"]; ok {
			var depArray []string
			if err := json.Unmarshal(dependsOnRaw, &depArray); err != nil {
				return nil, "invalid_meta_composition"
			}
			for _, dep := range depArray {
				if IsBlank(dep) {
					return nil, "invalid_meta_composition"
				}
				dependsOn = append(dependsOn, dep)
			}
		}

		var skill string
		if skillRaw, ok := stepElement["skill"]; ok {
			var s string
			if err := json.Unmarshal(skillRaw, &s); err == nil {
				skill = s
			}
		}

		var tool string
		if toolRaw, ok := stepElement["tool"]; ok {
			var t string
			if err := json.Unmarshal(toolRaw, &t); err == nil {
				tool = t
			}
		}

		var withJson string
		if withElement, ok := stepElement["with"]; ok {
			var objCheck map[string]any
			if err := json.Unmarshal(withElement, &objCheck); err != nil {
				return nil, "invalid_with_payload"
			}
			withJson = string(withElement)
		}

		var when string
		if whenElement, ok := stepElement["when"]; ok {
			var w string
			if err := json.Unmarshal(whenElement, &w); err != nil || IsBlank(w) {
				return nil, "invalid_when_expression"
			}
			when = w
		}

		var stepToolArgsJson, errCode string
		var success bool

		success, stepToolArgsJson, errCode = s.TryParseStepToolArgs(stepElement, kind)
		if !success {
			return nil, errCode
		}

		var toolAllowlist []string
		success, toolAllowlist, errCode = s.TryParseToolAllowlist(stepElement, kind)
		if !success {
			return nil, errCode
		}

		var outputChoices []string
		success, outputChoices, errCode = s.TryParseOutputChoices(stepElement, kind)
		if !success {
			return nil, errCode
		}

		withJson, errCode, success = s.TryNormalizeClassifyOptions(kind, withJson, outputChoices)
		if !success {
			return nil, errCode
		}

		var clarify *MetaClarifySchema
		clarify, errCode, success = s.TryParseClarify(stepElement, withJson, kind)
		if !success {
			return nil, errCode
		}

		var routes []MetaRouteDefinition
		var hasRouteArray bool
		routes, hasRouteArray, errCode, success = s.TryParseRouteArray(stepElement)
		if !success {
			return nil, errCode
		}

		if hasRouteArray && s.HasLegacyRouteObject(withJson) {
			return nil, "invalid_route"
		}

		var onFailure string
		success, onFailure, errCode = s.TryParseOnFailure(stepElement)
		if !success {
			return nil, errCode
		}

		var timeoutSeconds int
		success, timeoutSeconds, errCode = s.TryParseTimeoutSeconds(stepElement)
		if !success {
			return nil, errCode
		}

		var retry *MetaStepRetryPolicy
		success, retry, errCode = s.TryParseRetryPolicy(stepElement)
		if !success {
			return nil, errCode
		}

		var outputContract *MetaStepOutputContract
		success, outputContract, errCode = s.TryParseOutputContract(stepElement)
		if !success {
			return nil, errCode
		}

		var skillExecEntrypoint, skillExecStdin, skillExecCwd, skillExecParseMode string
		var skillExecArgs []string
		success, skillExecEntrypoint, skillExecArgs, skillExecStdin, skillExecCwd, skillExecParseMode, errCode = s.TryParseSkillExecOptions(stepElement, kind)
		if !success {
			return nil, errCode
		}

		steps = append(steps, MetaSkillStepDefinition{
			ID:                  id,
			Kind:                kind,
			Skill:               skill,
			Tool:                tool,
			SkillExecEntrypoint: skillExecEntrypoint,
			SkillExecArgs:       skillExecArgs,
			SkillExecStdin:      skillExecStdin,
			SkillExecCwd:        skillExecCwd,
			SkillExecParseMode:  skillExecParseMode,
			WithJSON:            withJson,
			When:                when,
			ToolArgsJSON:        stepToolArgsJson,
			ToolAllowlist:       toolAllowlist,
			OutputChoices:       outputChoices,
			Clarify:             clarify,
			Routes:              routes,
			DependsOn:           dependsOn,
			OnFailure:           onFailure,
			TimeoutSeconds:      &timeoutSeconds,
			Retry:               retry,
			OutputContract:      outputContract,
		})
	}

	if success, errCode := s.ValidateComposition(steps); !success {
		return nil, errCode
	}

	return &MetaSkillComposition{
		ToolArgsJson: toolArgsJson,
		Steps:        steps,
	}, ""
}

func (s *SkillLoader) TryParseSkillExecOptions(stepElement map[string]json.RawMessage, kind string) (bool, string, []string, string, string, string, string) {
	var entrypoint string
	args := []string{}
	var stdin string
	var cwd string
	var parseMode string
	var errorCode string

	isSkillExec := strings.EqualFold(kind, "skill_exec")

	if raw, exists := stepElement["entrypoint"]; exists {
		var val string
		if !isSkillExec || json.Unmarshal(raw, &val) != nil || strings.TrimSpace(val) == "" {
			errorCode = "invalid_skill_exec"
			return false, entrypoint, args, stdin, cwd, parseMode, errorCode
		}
		entrypoint = strings.TrimSpace(val)
	}

	if _, exists := stepElement["args"]; exists {
		parsedArgs, ok, _ := s.TryParseStringArrayProperty(stepElement, "args")
		if !isSkillExec || !ok {
			errorCode = "invalid_skill_exec"
			return false, entrypoint, args, stdin, cwd, parseMode, errorCode
		}
		args = parsedArgs
	}

	if raw, exists := stepElement["stdin"]; exists {
		var val string
		if !isSkillExec || json.Unmarshal(raw, &val) != nil || strings.TrimSpace(val) == "" {
			errorCode = "invalid_skill_exec"
			return false, entrypoint, args, stdin, cwd, parseMode, errorCode
		}
		stdin = val
	}

	if raw, exists := stepElement["cwd"]; exists {
		var val string
		if !isSkillExec || json.Unmarshal(raw, &val) != nil || strings.TrimSpace(val) == "" {
			errorCode = "invalid_skill_exec"
			return false, entrypoint, args, stdin, cwd, parseMode, errorCode
		}
		cwd = strings.TrimSpace(val)

		if filepath.IsAbs(cwd) || strings.Contains(cwd, "..") {
			errorCode = "invalid_skill_exec"
			return false, entrypoint, args, stdin, cwd, parseMode, errorCode
		}
	}

	if raw, exists := stepElement["parse_mode"]; exists {
		var val string
		if !isSkillExec || json.Unmarshal(raw, &val) != nil || strings.TrimSpace(val) == "" {
			errorCode = "invalid_skill_exec"
			return false, entrypoint, args, stdin, cwd, parseMode, errorCode
		}
		parseMode = strings.ToLower(strings.TrimSpace(val))
		if parseMode != "text" && parseMode != "json" {
			errorCode = "invalid_skill_exec"
			return false, entrypoint, args, stdin, cwd, parseMode, errorCode
		}
	}

	return true, entrypoint, args, stdin, cwd, parseMode, errorCode
}

func (s *SkillLoader) TryParseOutputContract(stepElement map[string]json.RawMessage) (bool, *MetaStepOutputContract, string) {
	isSupportedOutputContractProperty := func(propertyName string) bool {
		return propertyName == "format" || propertyName == "required_properties"
	}
	outputContract := &MetaStepOutputContract{}
	contractRaw, hasContract := stepElement["output_contract"]
	if !hasContract {
		contractRaw, hasContract = stepElement["output_schema"]
	}

	if !hasContract || string(contractRaw) == "null" {
		return true, outputContract, ""
	}

	var contractMap map[string]json.RawMessage
	if err := json.Unmarshal(contractRaw, &contractMap); err != nil {
		return false, outputContract, "invalid_output_contract"
	}

	for key := range contractMap {
		if !isSupportedOutputContractProperty(key) {
			return false, outputContract, "invalid_output_contract"
		}
	}

	format := "text"
	if formatRaw, hasFormat := contractMap["format"]; hasFormat {
		var formatStr string
		if err := json.Unmarshal(formatRaw, &formatStr); err != nil || strings.TrimSpace(formatStr) == "" {
			return false, outputContract, "invalid_output_contract"
		}

		format = strings.ToLower(strings.TrimSpace(formatStr))
	}

	if format != "text" && format != "json" {
		return false, outputContract, "invalid_output_contract"
	}

	var requiredProperties []string
	if requiredRaw, hasRequired := contractMap["required_properties"]; hasRequired {
		var rawList []json.RawMessage
		if err := json.Unmarshal(requiredRaw, &rawList); err != nil {
			return false, outputContract, "invalid_output_contract"
		}

		seen := make(map[string]bool)
		for _, itemRaw := range rawList {
			var itemStr string
			if err := json.Unmarshal(itemRaw, &itemStr); err != nil {
				return false, outputContract, "invalid_output_contract"
			}

			trimmedItem := strings.TrimSpace(itemStr)
			if trimmedItem == "" || seen[strings.ToLower(trimmedItem)] {
				return false, outputContract, "invalid_output_contract"
			}
			seen[strings.ToLower(trimmedItem)] = true

			requiredProperties = append(requiredProperties, trimmedItem)
		}
	}

	if len(requiredProperties) > 0 && format != "json" {
		return false, outputContract, "invalid_output_contract"
	}

	outputContract.Format = format
	outputContract.RequiredProperties = requiredProperties

	return true, outputContract, ""
}

// 临时结构体，用于区分“未传字段”和“传了默认值(0)”
type retryPolicyInput struct {
	MaxAttempts *int `json:"max_attempts"`
	BackoffMs   *int `json:"backoff_ms"`
}

func (s *SkillLoader) TryParseRetryPolicy(stepElement map[string]json.RawMessage) (bool, *MetaStepRetryPolicy, string) {
	retry := &MetaStepRetryPolicy{
		MaxAttempts: 1,
		BackoffMs:   0,
	}

	retryRaw, exists := stepElement["retry"]
	if !exists || retryRaw == nil || string(retryRaw) == "null" {
		return true, retry, ""
	}

	trimmed := bytes.TrimSpace(retryRaw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false, nil, "invalid_step_retry"
	}

	decoder := json.NewDecoder(bytes.NewReader(retryRaw))
	decoder.DisallowUnknownFields()

	var input retryPolicyInput
	if err := decoder.Decode(&input); err != nil {
		// 任何解析错误（类型不符、包含未知字段等）统一返回错误码
		return false, nil, "invalid_step_retry"
	}

	// 5. 校验 max_attempts 范围
	if input.MaxAttempts != nil {
		val := *input.MaxAttempts
		if val < 1 || val > 10 {
			return false, nil, "invalid_step_retry"
		}
		retry.MaxAttempts = val
	}

	// 6. 校验 backoff_ms 范围
	if input.BackoffMs != nil {
		val := *input.BackoffMs
		if val < 0 || val > 600000 {
			return false, nil, "invalid_step_retry"
		}
		retry.BackoffMs = val
	}

	return true, retry, ""
}

func (s *SkillLoader) TryParseTimeoutSeconds(stepElement map[string]json.RawMessage) (bool, int, string) {
	timeoutSecondsVal, exists := stepElement["timeout_seconds"]
	if !exists || timeoutSecondsVal == nil {
		return false, 0, "invalid_timeout_seconds"
	}

	var timeoutSeconds int
	if err := json.Unmarshal(timeoutSecondsVal, &timeoutSeconds); err != nil {
		return false, 0, "invalid_timeout_seconds"
	}

	if timeoutSeconds <= 0 {
		return false, 0, "invalid_timeout_seconds"
	}

	return true, timeoutSeconds, ""
}

func (s *SkillLoader) TryParseOnFailure(stepElement map[string]json.RawMessage) (bool, string, string) {
	onFailureVal, exists := stepElement["on_failure"]
	if !exists || onFailureVal == nil {
		return false, "", "invalid_on_failure"
	}

	var onFailure string
	if err := json.Unmarshal(onFailureVal, &onFailure); err != nil {
		return false, "", "invalid_on_failure"
	}

	if len(onFailure) == 0 {
		return false, "", "invalid_on_failure"
	}

	return true, onFailure, ""
}

func (s *SkillLoader) BuildClassifyOptionsWithJson(outputChoices []string) (string, error) {
	data := map[string][]string{
		"options": outputChoices,
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (s *SkillLoader) TryNormalizeClassifyOptions(stepKind, withJson string, outputChoices []string) (normalizedWithJson string, errorCode string, ok bool) {
	normalizedWithJson = withJson
	errorCode = ""

	if !strings.EqualFold(stepKind, "llm_classify") {
		return normalizedWithJson, "", true
	}

	if strings.TrimSpace(withJson) == "" {
		if len(outputChoices) == 0 {
			return normalizedWithJson, "", true
		}

		generated, err := s.BuildClassifyOptionsWithJson(outputChoices)
		if err != nil {
			return "", "invalid_meta_composition", false
		}
		return generated, "", true
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(withJson), &doc); err != nil {
		return "", "invalid_meta_composition", false
	}

	if _, exists := doc["options"]; exists {
		return normalizedWithJson, "", true
	}

	if len(outputChoices) == 0 {
		return normalizedWithJson, "", true
	}

	outputChoicesBytes, err := json.Marshal(outputChoices)
	if err != nil {
		return "", "invalid_meta_composition", false
	}
	doc["options"] = json.RawMessage(outputChoicesBytes)

	mergedBytes, err := json.Marshal(doc)
	if err != nil {
		return "", "invalid_meta_composition", false
	}

	return string(mergedBytes), "", true
}

func (s *SkillLoader) IsOutputChoicesStepKind(stepKind string) bool {
	return stepKind == "agent" || stepKind == "skill_exec" || stepKind == "tool_call" || stepKind == "llm_chat" || stepKind == "llm_classify"
}

func (s *SkillLoader) TryParseOutputChoices(stepElement map[string]json.RawMessage, kind string) (bool, []string, string) {
	outputChoices := []string{}

	_, exists := stepElement["output_choices"]
	if !exists {
		return true, outputChoices, ""
	}

	if !s.IsOutputChoicesStepKind(kind) {
		return false, outputChoices, "invalid_output_choices"
	}

	values, ok, _ := s.TryParseStringArrayProperty(stepElement, "output_choices")
	if !ok {
		return false, outputChoices, "invalid_output_choices"
	}

	if len(values) == 0 {
		return false, outputChoices, "invalid_output_choices"
	}

	outputChoices = values
	return true, outputChoices, ""
}

func (s *SkillLoader) ValidateComposition(steps []MetaSkillStepDefinition) (bool, string) {
	if len(steps) == 0 {
		return false, "invalid_meta_composition"
	}

	supportedKinds := map[string]bool{
		"agent":        true,
		"skill_exec":   true,
		"tool_call":    true,
		"llm_chat":     true,
		"llm_classify": true,
		"user_input":   true,
	}

	ids := make(map[string]bool)
	for _, step := range steps {
		stepIdLower := strings.ToLower(step.ID)
		if ids[stepIdLower] {
			return false, "duplicate_step_id"
		}
		ids[stepIdLower] = true

		kindLower := strings.ToLower(step.Kind)
		if !supportedKinds[kindLower] {
			return false, "unsupported_step_kind"
		}

		if strings.EqualFold(step.Kind, "tool_call") {
			if IsBlank(step.Tool) || !IsBlank(step.Skill) {
				return false, "invalid_step_kind_fields"
			}
		}

		if strings.EqualFold(step.Kind, "skill_exec") {
			if IsBlank(step.Skill) || IsBlank(step.SkillExecEntrypoint) || !IsBlank(step.Tool) {
				return false, "invalid_step_kind_fields"
			}
		}

		if strings.EqualFold(step.Kind, "agent") {
			if !IsBlank(step.Tool) {
				return false, "invalid_step_kind_fields"
			}
		}

		if strings.EqualFold(step.Kind, "llm_chat") || strings.EqualFold(step.Kind, "llm_classify") || strings.EqualFold(step.Kind, "user_input") {
			if !IsBlank(step.Skill) || !IsBlank(step.Tool) {
				return false, "invalid_step_kind_fields"
			}
		}
	}

	for _, step := range steps {
		if strings.EqualFold(step.Kind, "llm_classify") && !s.ValidateClassifyStep(step.WithJSON, ids) {
			return false, "invalid_classify_step"
		}

		for _, dependency := range step.DependsOn {
			depLower := strings.ToLower(dependency)
			if !ids[depLower] {
				return false, "invalid_dependency"
			}

			if strings.EqualFold(step.ID, dependency) {
				return false, "self_dependency"
			}
		}
	}

	if s.HasDependencyCycle(steps) {
		return false, "dependency_cycle"
	}

	if isValid, errCode := s.ValidateRouteArrays(steps); !isValid {
		return false, errCode
	}

	if isValid, errCode := s.ValidateFailureBranches(steps, ids); !isValid {
		return false, errCode
	}

	return true, ""
}

func (s *SkillLoader) ValidateFailureBranches(steps []MetaSkillStepDefinition, ids map[string]bool) (bool, string) {
	stepById := make(map[string]MetaSkillStepDefinition, len(steps))
	for _, step := range steps {
		stepById[strings.ToLower(step.ID)] = step
	}

	knownStepIds := make(map[string]bool, len(ids))
	for k, v := range ids {
		knownStepIds[strings.ToLower(k)] = v
	}

	designatedBy := make(map[string]string)
	fallbackTargets := make(map[string]bool)

	// 第一轮循环：验证 OnFailure 的合法性并收集 fallback 节点
	for _, step := range steps {
		if strings.TrimSpace(step.OnFailure) == "" {
			continue
		}
		onFailureTrimmed := strings.TrimSpace(step.OnFailure)
		if onFailureTrimmed == "" {
			continue
		}
		stepIdLower := strings.ToLower(step.ID)
		onFailureLower := strings.ToLower(onFailureTrimmed)

		// 1. 不能指向自己，且必须在已知的节点 ID 中
		if stepIdLower == onFailureLower || !knownStepIds[onFailureLower] {
			return false, "invalid_on_failure"
		}

		// 2. 替代节点（OnFailure 节点）自身不能有 OnFailure 且不能有依赖
		substitute, exists := stepById[onFailureLower]
		if !exists || (strings.TrimSpace(substitute.OnFailure) != "") || len(substitute.DependsOn) > 0 {
			return false, "invalid_on_failure"
		}

		// 3. 每一个失败跳转目标只能被一个步骤指定（独占性）
		if _, exists := designatedBy[onFailureLower]; exists {
			return false, "invalid_on_failure"
		}

		designatedBy[onFailureLower] = step.ID
		fallbackTargets[onFailureLower] = true
	}

	// 第二轮循环：确保常规路由/依赖不会误入 fallback 节点
	for _, step := range steps {
		// 检查常规依赖
		for _, dependency := range step.DependsOn {
			if fallbackTargets[strings.ToLower(dependency)] {
				return false, "invalid_on_failure"
			}
		}

		// 检查显式路由
		for _, route := range step.Routes {
			if fallbackTargets[strings.ToLower(route.To)] {
				return false, "invalid_on_failure"
			}
		}

		// 检查 Legacy Classify 路由（JSON 解析）
		if s.containsFallbackTargetInLegacyClassifyRoutes(step.WithJSON, fallbackTargets) {
			return false, "invalid_on_failure"
		}
	}

	return true, ""
}

func (s *SkillLoader) containsFallbackTargetInLegacyClassifyRoutes(withJson string, fallbackTargets map[string]bool) bool {
	if IsBlank(withJson) {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(withJson), &doc); err != nil {
		return false
	}
	routeRaw, exists := doc["route"]
	if !exists {
		return false
	}

	routeMap, ok := routeRaw.(map[string]any)
	if !ok {
		return false
	}

	// 遍历 route 对象下的所有属性
	for _, val := range routeMap {
		switch v := val.(type) {
		case string:
			// 情况A：属性值是字符串
			targetStep := strings.TrimSpace(v)
			if targetStep != "" && fallbackTargets[strings.ToLower(targetStep)] {
				return true
			}

		case []any:
			// 情况B：属性值是数组
			for _, item := range v {
				if targetStr, ok := item.(string); ok {
					targetStep := strings.TrimSpace(targetStr)
					if targetStep != "" && fallbackTargets[strings.ToLower(targetStep)] {
						return true
					}
				}
			}
		}
	}

	return false
}

func (s *SkillLoader) ValidateRouteArrays(steps []MetaSkillStepDefinition) (bool, string) {
	ids := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		ids[strings.ToLower(step.ID)] = struct{}{}
	}

	for _, step := range steps {
		fallbackCount := 0
		stepIdLower := strings.ToLower(step.ID)
		routesCount := len(step.Routes)

		for i, route := range step.Routes {
			routeToLower := strings.ToLower(route.To)

			if _, exists := ids[routeToLower]; !exists {
				return false, "invalid_route_target"
			}

			if routeToLower == stepIdLower {
				return false, "invalid_route_scope"
			}

			if IsBlank(route.When) {
				fallbackCount++
				if fallbackCount > 1 {
					return false, "invalid_route_fallback"
				}

				if i != routesCount-1 {
					return false, "invalid_route_fallback"
				}
			}
		}
	}

	return true, ""
}

func (s *SkillLoader) HasDependencyCycle(steps []MetaSkillStepDefinition) bool {
	state := make(map[string]int)

	stepById := make(map[string]MetaSkillStepDefinition)
	for _, step := range steps {
		stepById[strings.ToLower(step.ID)] = step
	}

	var dfs func(string) bool
	dfs = func(stepId string) bool {
		lowerId := strings.ToLower(stepId)

		if currentState, exists := state[lowerId]; exists {
			// 如果状态为 1，说明在当前路径中再次遇到了该节点，存在环
			return currentState == 1
		}

		// 标记为正在访问
		state[lowerId] = 1

		// 注意：如果输入的依赖项 ID 在 steps 中不存在，直接从 map 取可能会导致不安全
		// 这里加上了安全检查，防止程序因找不到依赖项而崩溃
		if step, exists := stepById[lowerId]; exists {
			for _, dependency := range step.DependsOn {
				if dfs(dependency) {
					return true
				}
			}
		}

		// 标记为已完成访问
		state[lowerId] = 2
		return false
	}

	// 遍历所有步骤进行检测
	for _, step := range steps {
		lowerId := strings.ToLower(step.ID)
		if currentState, exists := state[lowerId]; exists && currentState == 2 {
			continue
		}

		if dfs(step.ID) {
			return true
		}
	}

	return false
}

func (s *SkillLoader) ValidateClassifyStep(withJSON string, knownStepIds map[string]bool) bool {
	if strings.TrimSpace(withJSON) == "" {
		return false
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(withJSON), &root); err != nil {
		return false
	}

	optionsRaw, exists := root["options"]
	if !exists {
		return false
	}
	optionsArray, ok := optionsRaw.([]any)
	if !ok || len(optionsArray) == 0 {
		return false
	}

	optionsSet := make(map[string]struct{})
	for _, optRaw := range optionsArray {
		optStr, ok := optRaw.(string)
		if !ok || strings.TrimSpace(optStr) == "" {
			return false
		}
		optionsSet[strings.ToLower(optStr)] = struct{}{}
	}

	routeRaw, exists := root["route"]
	if !exists {
		return true
	}

	routeMap, ok := routeRaw.(map[string]any)
	if !ok {
		return false
	}

	for routeKey, routeValue := range routeMap {
		if strings.TrimSpace(routeKey) == "" {
			return false
		}

		if _, found := optionsSet[strings.ToLower(routeKey)]; !found {
			return false
		}

		switch v := routeValue.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return false
			}
			if _, found := knownStepIds[v]; !found {
				return false
			}

		case []any:
			for _, targetRaw := range v {
				targetStr, ok := targetRaw.(string)
				if !ok || strings.TrimSpace(targetStr) == "" {
					return false
				}
				if _, found := knownStepIds[targetStr]; !found {
					return false
				}
			}

		default:
			return false
		}
	}

	return true
}

func (s *SkillLoader) IsToolStepKind(stepKind string) bool {
	return "tool_call" == stepKind
}

func (s *SkillLoader) TryParseStepToolArgs(stepElement map[string]json.RawMessage, stepKind string) (bool, string, string) {
	stepToolArgsElement, exists := stepElement["tool_args"]
	if !exists {
		return true, "", ""
	}

	isObject := len(stepToolArgsElement) > 0 && stepToolArgsElement[0] == '{'

	if !s.IsToolStepKind(stepKind) || !isObject {
		errCode := "invalid_tool_args"
		return false, "", errCode
	}

	toolArgsStr := string(stepToolArgsElement)
	return true, toolArgsStr, ""
}

func (s *SkillLoader) TryParseToolAllowlist(stepElement map[string]json.RawMessage, stepKind string) (bool, []string, string) {
	toolAllowlist := []string{}
	_, exists := stepElement["tool_allowlist"]
	if !exists {
		return true, toolAllowlist, ""
	}

	if !s.IsToolStepKind(stepKind) {
		return false, toolAllowlist, "invalid_tool_allowlist"
	}

	values, ok, _ := s.TryParseStringArrayProperty(stepElement, "tool_allowlist")
	if !ok {
		return false, toolAllowlist, "invalid_tool_allowlist"
	}

	if len(values) == 0 {
		return false, toolAllowlist, "invalid_tool_allowlist"
	}

	toolAllowlist = values
	return true, toolAllowlist, ""
}

func (s *SkillLoader) TryParseStringArrayProperty(stepElement map[string]json.RawMessage, propertyName string) ([]string, bool, string) {
	values := []string{}

	propertyElement, exists := stepElement[propertyName]
	if !exists {
		return values, true, ""
	}

	trimmed := strings.TrimSpace(string(propertyElement))
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return values, false, "invalid_meta_composition"
	}

	var rawItems []json.RawMessage
	if err := json.Unmarshal(propertyElement, &rawItems); err != nil {
		return values, false, "invalid_meta_composition"
	}

	items := make([]string, 0, len(rawItems))

	for _, itemElement := range rawItems {
		var itemStr string

		if err := json.Unmarshal(itemElement, &itemStr); err != nil {
			return values, false, "invalid_meta_composition"
		}

		if strings.TrimSpace(itemStr) == "" {
			return values, false, "invalid_meta_composition"
		}

		items = append(items, itemStr)
	}

	values = items
	return values, true, ""
}

func (s *SkillLoader) ValidateFinalTextMode(finalTextMode string, steps []MetaSkillStepDefinition) bool {
	if IsBlank(finalTextMode) {
		return true
	}

	var mode = strings.TrimSpace(finalTextMode)
	if mode == "auto" || mode == "raw" || mode == "structured" {
		return true
	}

	if !strings.HasPrefix(mode, "step:") {
		return false
	}
	var stepId = strings.TrimSpace(mode[5:])
	if IsBlank(stepId) {
		return false
	}
	for _, step := range steps {
		if step.ID == stepId {
			return true
		}
	}

	return false
}

func (s *SkillLoader) ScanSkillResources(skillDir string) []SkillResource {
	list := []SkillResource{}
	if IsBlank(skillDir) || !DirectoryExists(skillDir) {
		return list
	}

	list = append(list, s.AppendResourcesFromSubdir(skillDir, "references", SkillResourceKind_Reference)...)
	list = append(list, s.AppendResourcesFromSubdir(skillDir, "scripts", SkillResourceKind_Script)...)
	return list
}

func (s *SkillLoader) AppendResourcesFromSubdir(skillDir, subDir string, kind SkillResourceKind) []SkillResource {
	dir := filepath.Join(skillDir, subDir)
	if !DirectoryExists(dir) {
		return []SkillResource{}
	}

	list := []SkillResource{}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(skillDir, path)
		if err != nil {
			return nil
		}
		relPathUniform := filepath.ToSlash(relPath)

		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil
		}

		list = append(list, SkillResource{
			Name:         d.Name(),
			RelativePath: relPathUniform,
			AbsolutePath: absPath,
			Kind:         kind,
		})

		return nil
	})

	return list
}

func (s *SkillLoader) IsSupportedClarifyProperty(propertyName string) bool {
	switch propertyName {
	case "mode", "extract_natural_language", "cancel_words", "skip_if", "timeout_seconds", "fields":
		return true
	default:
		return false
	}
}

func (s *SkillLoader) IsSupportedClarifyFieldProperty(propertyName string) bool {
	switch propertyName {
	case "name", "type", "required", "default", "options", "min_length", "max_length", "min", "max":
		return true
	default:
		return false
	}
}

func (s *SkillLoader) TryGetClarifyElement(stepElement map[string]json.RawMessage, withJson string) (clarifyElement json.RawMessage, errorCode string, success bool) {
	directClarifyElement, hasDirectClarify := stepElement["clarify"]
	var withClarifyElement json.RawMessage

	if strings.TrimSpace(withJson) != "" {
		var doc map[string]json.RawMessage
		decoder := json.NewDecoder(strings.NewReader(withJson))
		if err := decoder.Decode(&doc); err != nil {
			return nil, "invalid_clarify_schema", false
		}

		if parsedWithClarify, exists := doc["clarify"]; exists {
			withClarifyElement = parsedWithClarify
		}
	}

	if hasDirectClarify {
		return directClarifyElement, "", true
	}

	return withClarifyElement, "", true
}

func (s *SkillLoader) TryParseClarify(stepElement map[string]json.RawMessage, withJson string, stepKind string) (clarify *MetaClarifySchema, errorCode string, success bool) {
	clarifyElement, errorCode, ok := s.TryGetClarifyElement(stepElement, withJson)
	if !ok {
		return nil, errorCode, false
	}

	if clarifyElement == nil {
		return nil, "", true
	}

	if !strings.EqualFold(stepKind, "user_input") {
		return nil, "invalid_clarify_schema", false
	}

	var clarifyObj map[string]json.RawMessage
	if err := json.Unmarshal(clarifyElement, &clarifyObj); err != nil {
		return nil, "invalid_clarify_schema", false
	}

	for propName := range clarifyObj {
		if !s.IsSupportedClarifyProperty(propName) {
			return nil, "invalid_clarify_schema", false
		}
	}

	mode := "chat"
	if modeRaw, exists := clarifyObj["mode"]; exists {
		var modeStr string
		if err := json.Unmarshal(modeRaw, &modeStr); err != nil || strings.TrimSpace(modeStr) == "" {
			return nil, "invalid_clarify_schema", false
		}
		if !strings.EqualFold(modeStr, "chat") && !strings.EqualFold(modeStr, "form") {
			return nil, "invalid_clarify_schema", false
		}
		mode = modeStr
	}

	extractNaturalLanguage := false
	if extractRaw, exists := clarifyObj["extract_natural_language"]; exists {
		if err := json.Unmarshal(extractRaw, &extractNaturalLanguage); err != nil {
			return nil, "invalid_clarify_schema", false
		}
	}

	cancelWords, ok, _ := s.TryParseStringArrayProperty(clarifyObj, "cancel_words")
	if !ok {
		return nil, "invalid_clarify_schema", false
	}

	var timeoutSeconds *int
	if timeoutRaw, exists := clarifyObj["timeout_seconds"]; exists {
		var parsedTimeout int
		if err := json.Unmarshal(timeoutRaw, &parsedTimeout); err != nil || parsedTimeout <= 0 {
			return nil, "invalid_clarify_schema", false
		}
		timeoutSeconds = &parsedTimeout
	}

	var skipIf string
	if skipIfRaw, exists := clarifyObj["skip_if"]; exists {
		var skipIfStr string
		if err := json.Unmarshal(skipIfRaw, &skipIfStr); err != nil || strings.TrimSpace(skipIfStr) == "" {
			return nil, "invalid_clarify_schema", false
		}
		skipIf = skipIfStr
	}

	fields := []MetaClarifyField{}
	fieldNames := make(map[string]bool)

	if fieldsRaw, exists := clarifyObj["fields"]; exists {
		var fieldsSlice []json.RawMessage
		if err := json.Unmarshal(fieldsRaw, &fieldsSlice); err != nil {
			return nil, "invalid_clarify_schema", false
		}

		for _, fieldRaw := range fieldsSlice {
			var fieldObj map[string]json.RawMessage
			if err := json.Unmarshal(fieldRaw, &fieldObj); err != nil {
				return nil, "invalid_clarify_schema", false
			}

			for propName := range fieldObj {
				if !s.IsSupportedClarifyFieldProperty(propName) {
					return nil, "invalid_clarify_schema", false
				}
			}

			var nameRaw, typeRaw json.RawMessage
			var nameExists, typeExists bool
			nameRaw, nameExists = fieldObj["name"]
			typeRaw, typeExists = fieldObj["type"]

			if !nameExists || !typeExists {
				return nil, "invalid_clarify_schema", false
			}

			var fieldName, fieldType string
			if err := json.Unmarshal(nameRaw, &fieldName); err != nil || strings.TrimSpace(fieldName) == "" {
				return nil, "invalid_clarify_schema", false
			}
			if err := json.Unmarshal(typeRaw, &fieldType); err != nil || strings.TrimSpace(fieldType) == "" {
				return nil, "invalid_clarify_schema", false
			}

			if fieldNames[strings.ToLower(fieldName)] {
				return nil, "invalid_clarify_schema", false
			}
			fieldNames[strings.ToLower(fieldName)] = true

			options, ok, _ := s.TryParseStringArrayProperty(fieldObj, "options")
			if !ok {
				return nil, "invalid_clarify_schema", false
			}

			var minLength *int
			if minLengthRaw, exists := fieldObj["min_length"]; exists {
				var parsedMinLength int
				if err := json.Unmarshal(minLengthRaw, &parsedMinLength); err != nil || parsedMinLength < 0 {
					return nil, "invalid_clarify_schema", false
				}
				minLength = &parsedMinLength
			}

			var maxLength *int
			if maxLengthRaw, exists := fieldObj["max_length"]; exists {
				var parsedMaxLength int
				if err := json.Unmarshal(maxLengthRaw, &parsedMaxLength); err != nil || parsedMaxLength < 0 {
					return nil, "invalid_clarify_schema", false
				}
				maxLength = &parsedMaxLength
			}

			var min *float64
			if minRaw, exists := fieldObj["min"]; exists {
				var parsedMin float64
				if err := json.Unmarshal(minRaw, &parsedMin); err != nil {
					return nil, "invalid_clarify_schema", false
				}
				min = &parsedMin
			}

			var max *float64
			if maxRaw, exists := fieldObj["max"]; exists {
				var parsedMax float64
				if err := json.Unmarshal(maxRaw, &parsedMax); err != nil {
					return nil, "invalid_clarify_schema", false
				}
				max = &parsedMax
			}

			var required bool
			if requiredRaw, exists := fieldObj["required"]; exists {
				if err := json.Unmarshal(requiredRaw, &required); err != nil {
					return nil, "invalid_clarify_schema", false
				}
			}

			if !strings.EqualFold(fieldType, "string") &&
				!strings.EqualFold(fieldType, "integer") &&
				!strings.EqualFold(fieldType, "number") &&
				!strings.EqualFold(fieldType, "boolean") &&
				!strings.EqualFold(fieldType, "enum") {
				return nil, "invalid_clarify_schema", false
			}

			if strings.EqualFold(fieldType, "enum") && len(options) == 0 {
				return nil, "invalid_clarify_schema", false
			}

			if errorCode, ok := s.ValidateClarifyFieldConstraints(fieldType, options, minLength, maxLength, min, max); !ok {
				return nil, errorCode, false
			}

			var defaultValue json.RawMessage
			if defaultRaw, exists := fieldObj["default"]; exists {
				if !s.IsValidClarifyDefaultValue(fieldType, options, minLength, maxLength, min, max, defaultRaw) {
					return nil, "invalid_clarify_schema", false
				}
				defaultValue = defaultRaw
			}

			fields = append(fields, MetaClarifyField{
				Name:         fieldName,
				Type:         fieldType,
				Required:     required,
				DefaultValue: defaultValue,
				Options:      options,
				MinLength:    minLength,
				MaxLength:    maxLength,
				Min:          min,
				Max:          max,
			})
		}
	}

	if strings.EqualFold(mode, "form") && len(fields) == 0 {
		return nil, "invalid_clarify_schema", false
	}

	clarify = &MetaClarifySchema{
		Mode:                   mode,
		ExtractNaturalLanguage: extractNaturalLanguage,
		Fields:                 fields,
		CancelWords:            cancelWords,
		SkipIf:                 skipIf,
		TimeoutSeconds:         timeoutSeconds,
	}

	return clarify, "", true
}

func (s *SkillLoader) ValidateClarifyFieldConstraints(fieldType string, options []string, minLength *int, maxLength *int, min *float64, max *float64) (errorCode string, success bool) {
	isString := strings.EqualFold(fieldType, "string")
	isInteger := strings.EqualFold(fieldType, "integer")
	isNumber := strings.EqualFold(fieldType, "number")
	isEnum := strings.EqualFold(fieldType, "enum")

	if (!isString && (minLength != nil || maxLength != nil)) ||
		(!(isInteger || isNumber) && (min != nil || max != nil)) ||
		(!isEnum && len(options) > 0) {
		return "invalid_clarify_schema", false
	}

	if minLength != nil && maxLength != nil && *minLength > *maxLength {
		return "invalid_clarify_schema", false
	}

	if min != nil && max != nil && *min > *max {
		return "invalid_clarify_schema", false
	}

	return "", true
}

func (s *SkillLoader) IsValidClarifyDefaultValue(fieldType string, options []string, minLength *int, maxLength *int, min *float64, max *float64, defaultElement json.RawMessage) bool {
	if strings.EqualFold(fieldType, "string") {
		var value string
		if err := json.Unmarshal(defaultElement, &value); err != nil {
			return false
		}
		if minLength != nil && len(value) < *minLength {
			return false
		}
		if maxLength != nil && len(value) > *maxLength {
			return false
		}
		return true
	}

	if strings.EqualFold(fieldType, "integer") {
		var intValue int64
		if err := json.Unmarshal(defaultElement, &intValue); err != nil {
			return false
		}
		if min != nil && float64(intValue) < *min {
			return false
		}
		if max != nil && float64(intValue) > *max {
			return false
		}
		return true
	}

	if strings.EqualFold(fieldType, "number") {
		var doubleValue float64
		if err := json.Unmarshal(defaultElement, &doubleValue); err != nil {
			return false
		}
		if min != nil && doubleValue < *min {
			return false
		}
		if max != nil && doubleValue > *max {
			return false
		}
		return true
	}

	if strings.EqualFold(fieldType, "boolean") {
		var boolValue bool
		if err := json.Unmarshal(defaultElement, &boolValue); err != nil {
			return false
		}
		return true
	}

	if strings.EqualFold(fieldType, "enum") {
		var defaultText string
		if err := json.Unmarshal(defaultElement, &defaultText); err != nil || strings.TrimSpace(defaultText) == "" {
			return false
		}
		for _, opt := range options {
			if opt == defaultText {
				return true
			}
		}
		return false
	}

	return false
}

func (s *SkillLoader) TryParseRouteArray(stepMap map[string]json.RawMessage) (routes []MetaRouteDefinition, hasRouteArray bool, errorCode string, success bool) {
	routes = []MetaRouteDefinition{}
	hasRouteArray = false
	routeRaw, exists := stepMap["route"]
	if !exists {
		return routes, false, "", true
	}

	hasRouteArray = true

	var routeArray []json.RawMessage
	if err := json.Unmarshal(routeRaw, &routeArray); err != nil {
		return routes, hasRouteArray, "invalid_route", false
	}

	if len(routeArray) == 0 {
		return routes, hasRouteArray, "invalid_route", false
	}

	var parsedRoutes []MetaRouteDefinition

	for _, routeElementRaw := range routeArray {
		var routeMap map[string]json.RawMessage
		if err := json.Unmarshal(routeElementRaw, &routeMap); err != nil {
			return routes, hasRouteArray, "invalid_route", false
		}

		for propName := range routeMap {
			if !s.IsSupportedRouteProperty(propName) {
				return routes, hasRouteArray, "invalid_route", false
			}
		}

		var whenStr string
		if whenRaw, whenExists := routeMap["when"]; whenExists {
			var when string
			if err := json.Unmarshal(whenRaw, &when); err != nil || strings.TrimSpace(when) == "" {
				return routes, hasRouteArray, "invalid_when_expression", false
			}
			whenStr = when
		}

		toRaw, toExists := routeMap["to"]
		if !toExists {
			return routes, hasRouteArray, "invalid_route", false
		}

		var toStr string
		if err := json.Unmarshal(toRaw, &toStr); err != nil || strings.TrimSpace(toStr) == "" {
			return routes, hasRouteArray, "invalid_route", false
		}

		parsedRoutes = append(parsedRoutes, MetaRouteDefinition{
			When: whenStr,
			To:   toStr,
		})
	}

	return parsedRoutes, hasRouteArray, "", true
}

func (s *SkillLoader) IsSupportedRouteProperty(name string) bool {
	return name == "when" || name == "to"
}

func (s *SkillLoader) HasLegacyRouteObject(withJson string) bool {
	if IsBlank(withJson) {
		return false
	}

	var doc map[string]json.RawMessage
	decoder := json.NewDecoder(strings.NewReader(withJson))
	if err := decoder.Decode(&doc); err != nil {
		return false
	}
	routeRaw, exists := doc["route"]
	if !exists {
		return false
	}
	var routeObj map[string]any
	if err := json.Unmarshal(routeRaw, &routeObj); err != nil {
		return false
	}

	return true
}

func (s *SkillLoader) ScanDirectory(rootDir string, source SkillSource, results map[string]*SkillDefinition, scanSubdirectories bool) error {
	rootSkillFile := filepath.Join(rootDir, "SKILL.md")
	if FileExists(rootSkillFile) {
		func() {
			skill, errorCode, err := s.TryParseSkillFile(rootSkillFile, rootDir, source)
			if err == nil && skill != nil {
				results[skill.Name] = skill
			} else {
				errCode := errorCode
				if errCode == "" {
					errCode = "parse_failed"
				}
			}
		}()
	}
	return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path == rootDir {
			return nil
		}

		if !scanSubdirectories {
			rel, _ := filepath.Rel(rootDir, path)
			if strings.Contains(rel, string(filepath.Separator)) {
				return filepath.SkipDir
			}
		}

		skillFile := filepath.Join(path, "SKILL.md")
		if !FileExists(skillFile) {
			return nil
		}

		func() {
			skill, errorCode, err := s.TryParseSkillFile(skillFile, path, source)
			if err == nil && skill != nil {
				results[skill.Name] = skill
			} else {
				errCode := errorCode
				if errCode == "" {
					errCode = "parse_failed"
				}
			}
		}()

		return nil
	})

}

func (s *SkillLoader) TryParseSkillFile(filePath string, skillDir string, source SkillSource) (*SkillDefinition, string, error) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", err
	}
	return s.TryParseSkillContent(string(contentBytes), skillDir, source)
}

func (s *SkillLoader) TryParseSkillContent(content string, skillDir string, source SkillSource) (*SkillDefinition, string, error) {
	skill, err := s.ParseSkillContent(content, skillDir, source)
	if err != nil {
		return nil, "", err
	}
	if skill != nil {
		return skill, "", nil
	}

	errorCode := s.DiagnoseSkillParseFailure(content)
	return nil, errorCode, errors.New("parse failed")
}

func (s *SkillLoader) DiagnoseSkillParseFailure(content string) string {
	if !strings.HasPrefix(content, "---") {
		return "invalid_frontmatter"
	}

	if len(content) < 3 {
		return "invalid_frontmatter"
	}
	endIndex := strings.Index(content[3:], "\n---")
	if endIndex < 0 {
		return "invalid_frontmatter"
	}
	endIndex += 3 // 加上偏移量

	frontmatter := strings.TrimSpace(content[3:endIndex])
	var name string
	kind := SkillKind_Standard
	var compositionJson string
	var finalTextMode string

	// 统一处理换行符并分割
	frontmatterLines := strings.Split(strings.ReplaceAll(frontmatter, "\r\n", "\n"), "\n")

	for lineIndex := 0; lineIndex < len(frontmatterLines); lineIndex++ {
		rawLine := frontmatterLines[lineIndex]
		if strings.TrimSpace(rawLine) != "" && s.GetIndent(rawLine) != 0 {
			continue
		}

		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			name = s.NormalizeFrontmatterScalar(value)
		case "kind":
			var err error
			kind, err = s.TryParseSkillKind(s.NormalizeFrontmatterScalar(value))
			if err != nil {
				return "invalid_kind"
			}
		case "composition":
			if strings.TrimSpace(value) == "" {
				compositionBlock, _ := s.CollectIndentedBlock(frontmatterLines, lineIndex+1)
				consumedLines := len(strings.Split(compositionBlock, "\n"))
				if compositionBlock == "" {
					consumedLines = 0
				}
				lineIndex += consumedLines

				var err error
				compositionJson, err = s.TryConvertYamlBlockToJson(compositionBlock)
				if err != nil {
					return "invalid_meta_composition"
				}
			} else {
				compositionJson = value
			}
		case "final-text-mode", "final_text_mode":
			finalTextMode = s.NormalizeFrontmatterScalar(value)
		}
	}

	if strings.TrimSpace(name) == "" {
		return "missing_name"
	}

	if kind != SkillKind_Meta {
		return "parse_failed"
	}

	if strings.TrimSpace(compositionJson) == "" {
		return "missing_meta_composition"
	}

	composition, compositionErrorCode := s.ParseComposition(compositionJson)
	if composition == nil {
		if compositionErrorCode != "" {
			return compositionErrorCode
		}
		return "invalid_meta_composition"
	}

	if !s.ValidateFinalTextMode(finalTextMode, composition.Steps) {
		return "invalid_final_text_mode"
	}

	return "parse_failed"
}

func (s *SkillLoader) LoadAll(config *SkillsConfig, workspacePath string, pluginSkillDirs []string) []*SkillDefinition {
	if !config.Enabled {
		return []*SkillDefinition{}
	}

	allSkills := make(map[string]*SkillDefinition)

	scanSubdirectories := config.Load.ScanSubdirectories

	// 1. Extra dirs (最低优先级)
	for _, dir := range config.Load.ExtraDirs {
		if DirectoryExists(dir) {
			s.ScanDirectory(dir, SkillSource_Extra, allSkills, scanSubdirectories)
		}
	}

	// 2. Bundled skills
	if config.Load.IncludeBundled {
		if exePath, err := os.Executable(); err == nil {
			baseDir := filepath.Dir(exePath)
			bundledDir := filepath.Join(baseDir, "skills")
			if DirectoryExists(bundledDir) {
				s.ScanDirectory(bundledDir, SkillSource_Bundled, allSkills, scanSubdirectories)
			}
		}
	}

	// 3. Managed/local skills
	if config.Load.IncludeManaged {
		var managedDir string
		if strings.TrimSpace(config.Load.ManagedRoot) == "" {
			if homeDir, err := os.UserHomeDir(); err == nil {
				// 对应 C# 的 Environment.SpecialFolder.UserProfile
				managedDir = filepath.Join(homeDir, ".openclaw", "skills")
			}
		} else {
			managedDir = s.NormalizeManagedRootPath(config.Load.ManagedRoot)
		}

		if managedDir != "" && DirectoryExists(managedDir) {
			s.ScanDirectory(managedDir, SkillSource_Managed, allSkills, scanSubdirectories)
		}
	}

	// 4. Plugin-packaged skills
	for _, pluginDir := range pluginSkillDirs {
		if DirectoryExists(pluginDir) {
			s.ScanDirectory(pluginDir, SkillSource_Plugin, allSkills, scanSubdirectories)
		}
	}

	// 5. Workspace skills (最高优先级)
	if config.Load.IncludeWorkspace && strings.TrimSpace(workspacePath) != "" {
		wsSkillsDir := filepath.Join(workspacePath, "skills")
		if DirectoryExists(wsSkillsDir) {
			s.ScanDirectory(wsSkillsDir, SkillSource_Workspace, allSkills, scanSubdirectories)
		}
	}

	// 过滤符合条件的技能
	eligible := make([]*SkillDefinition, 0)

	for _, skill := range allSkills {
		// 因为 key 被转成了小写，拿到原始的 skill.Name
		name := skill.Name

		// AllowBundled 过滤器
		if skill.Source == SkillSource_Bundled && len(config.AllowBundled) > 0 {
			if !ContainsIgnoreCase(config.AllowBundled, name) {
				continue
			}
		}

		// 单个技能的配置项过滤
		configKey := name
		if skill.Metadata.SkillKey != "" {
			configKey = skill.Metadata.SkillKey
		}

		if entry, exists := config.Entries[configKey]; exists && !entry.Enabled {
			continue
		}

		// 依赖检查组件限制 (除非 Always=true)
		if !skill.Metadata.Always && !CheckRequirements(skill, config) {
			continue
		}

		eligible = append(eligible, skill)
	}

	return eligible
}

var binaryOnPathCache sync.Map

func IsBinaryOnPath(binaryName string) bool {
	// 1. 尝试从缓存中获取 (Load)
	if val, ok := binaryOnPathCache.Load(binaryName); ok {
		return val.(bool)
	}
	_, err := exec.LookPath(binaryName)
	exists := (err == nil)
	actual, _ := binaryOnPathCache.LoadOrStore(binaryName, exists)
	return actual.(bool)
}

func CheckRequirements(skill *SkillDefinition, config *SkillsConfig) bool {
	meta := skill.Metadata

	// 1. 操作系统网关 (OS Gate)
	if len(meta.Os) > 0 {
		currentOs := runtime.GOOS
		if !ContainsIgnoreCase(meta.Os, currentOs) {
			return false
		}
	}

	// 2. 强依赖的二进制文件 (Required binaries)
	for _, bin := range meta.RequireBins {
		if !IsBinaryOnPath(bin) {
			return false
		}
	}

	// 3. 多选一的二进制文件 (Any-of binaries)
	if len(meta.RequireAnyBins) > 0 {
		hasAny := false
		for _, bin := range meta.RequireAnyBins {
			if IsBinaryOnPath(bin) {
				hasAny = true
				break
			}
		}
		if !hasAny {
			return false
		}
	}

	// 4. 环境变量检查 (Required env vars)
	configKey := skill.Name
	if meta.SkillKey != "" {
		configKey = meta.SkillKey
	}

	entry, hasEntry := config.Entries[configKey]

	for _, envVar := range meta.RequireEnv {
		// 检查系统环境变量
		hasEnv := strings.TrimSpace(os.Getenv(envVar)) != ""

		// 检查配置注入的环境变量
		var hasInConfig bool
		if hasEntry && entry.Env != nil {
			_, hasInConfig = entry.Env[envVar]
		}

		// 检查是否为内置 API Key
		hasApiKey := meta.PrimaryEnv == envVar && hasEntry && strings.TrimSpace(entry.ApiKey) != ""

		if !hasEnv && !hasInConfig && !hasApiKey {
			return false
		}
	}

	// 5. 元技能特殊校验 (Meta Skill gating)
	if skill.Kind == SkillKind_Meta && config.MetaSkill.Enabled {
		// 风险等级过滤
		if len(config.MetaSkill.AllowedRiskLevels) > 0 {
			risk := ""
			if meta.Risk != "" {
				risk = meta.Risk
			}
			if !ContainsIgnoreCase(config.MetaSkill.AllowedRiskLevels, risk) {
				displayRisk := risk
				if displayRisk == "" {
					displayRisk = "(unset)"
				}
				return false
			}
		}

		// 能力项（Capabilities）过滤
		if len(config.MetaSkill.RequiredCapabilities) > 0 {
			// 在 Go 中用 map[string]struct{} 替代 HashSet<string>
			declaredCapabilities := make(map[string]struct{})
			for _, capItem := range meta.Capabilities {
				declaredCapabilities[strings.ToLower(capItem)] = struct{}{}
			}

			for _, requiredCapability := range config.MetaSkill.RequiredCapabilities {
				if _, exists := declaredCapabilities[strings.ToLower(requiredCapability)]; !exists {
					return false
				}
			}
		}
	}

	return true
}

func (s *SkillLoader) NormalizeManagedRootPath(managedRoot string) string {
	normalized := strings.TrimSpace(managedRoot)

	// 1. 处理以波浪号 `~` 开头的主目录相对路径
	if strings.HasPrefix(normalized, "~") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			// 截取 `~` 之后的部分，并去除开头的斜杠或反斜杠
			suffix := normalized[1:]
			suffix = strings.TrimLeft(suffix, "/\\")

			if len(suffix) == 0 {
				normalized = home
			} else {
				normalized = filepath.Join(home, suffix)
			}
		}
	}

	if !filepath.IsAbs(normalized) {
		absPath, err := filepath.Abs(normalized)
		if err != nil {
			return ""
		}
		normalized = absPath
	} else {
		normalized = filepath.Clean(normalized)
	}

	return normalized
}
