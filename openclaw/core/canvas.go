package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

type A2UiCatalogDescriptor struct {
	CatalogId      string            `json:"catalog_id"`
	ComponentTypes map[string]bool   `json:"component_types"`
	FunctionTypes  map[string]bool   `json:"function_types"`
	SharedTypes    map[string]bool   `json:"shared_types"`
	Aliases        map[string]string `json:"aliases"`
	DisplayName    string            `json:"display_name"`
}

func DefaultA2UiCatalogDescriptor() A2UiCatalogDescriptor {
	return A2UiCatalogDescriptor{
		ComponentTypes: make(map[string]bool),
		FunctionTypes:  make(map[string]bool),
		SharedTypes:    make(map[string]bool),
		Aliases:        make(map[string]string),
	}
}

// --- A2UiCatalogRegistry 常量 ---
const (
	A2UiCatalogRegistry_OpenClawV08CatalogId = "urn:a2ui:catalog:openclaw_v0_8"
	A2UiCatalogRegistry_AGenUiCatalogId      = "urn:a2ui:catalog:agenui_catalog"
)

// --- A2UiCatalogRegistry 内部变量 ---
var (
	A2UiCatalogRegistry_AGenUiComponentTypes = []string{
		"Text", "Image", "Icon", "Divider", "Video", "AudioPlayer", "Markdown",
		"Button", "TextField", "CheckBox", "Slider", "ChoicePicker", "DateTimeInput",
		"Row", "Column", "Card", "List", "Tabs", "Modal", "Table", "Carousel",
		"Web", "RichText", "Chart",
	}

	A2UiCatalogRegistry_OpenClawV08ComponentTypes = []string{
		"Text", "Markdown", "Card", "Button", "TextField", "ChoicePicker",
		"CheckBox", "Table", "Image", "Slider", "Chart",
	}

	A2UiCatalogRegistry_V08Aliases = map[string]string{
		"text":      "Text",
		"markdown":  "Markdown",
		"card":      "Card",
		"button":    "Button",
		"input":     "TextField",
		"select":    "ChoicePicker",
		"checklist": "CheckBox",
		"table":     "Table",
		"image":     "Image",
		"progress":  "Slider",
		"chart":     "Chart",
	}

	A2UiCatalogRegistry_Catalogs map[string]A2UiCatalogDescriptor
)

func init() {
	A2UiCatalogRegistry_Catalogs = map[string]A2UiCatalogDescriptor{
		strings.ToLower(A2UiCatalogRegistry_OpenClawV08CatalogId): NewA2UiCatalogDescriptor(
			A2UiCatalogRegistry_OpenClawV08CatalogId,
			A2UiCatalogRegistry_OpenClawV08ComponentTypes,
			A2UiCatalogRegistry_V08Aliases,
			"OpenClaw A2UI v0.8",
		),
		strings.ToLower(A2UiCatalogRegistry_AGenUiCatalogId): NewA2UiCatalogDescriptor(
			A2UiCatalogRegistry_AGenUiCatalogId,
			A2UiCatalogRegistry_AGenUiComponentTypes,
			A2UiCatalogRegistry_V08Aliases,
			"AGenUI Catalog",
		),
	}
}

func NewA2UiCatalogDescriptor(
	catalogId string,
	componentTypes []string,
	aliases map[string]string,
	displayName string,
) A2UiCatalogDescriptor {
	desc := DefaultA2UiCatalogDescriptor()
	desc.CatalogId = catalogId
	desc.DisplayName = displayName

	for _, v := range componentTypes {
		desc.ComponentTypes[strings.ToLower(v)] = true
	}

	for k, v := range aliases {
		desc.Aliases[strings.ToLower(k)] = v
	}

	return desc
}

// A2UiCatalogRegistry_BuiltInCatalogs 获取所有内置 Catalog
func A2UiCatalogRegistry_BuiltInCatalogs() []A2UiCatalogDescriptor {
	values := make([]A2UiCatalogDescriptor, 0, len(A2UiCatalogRegistry_Catalogs))
	for _, v := range A2UiCatalogRegistry_Catalogs {
		values = append(values, v)
	}
	return values
}

func A2UiCatalogRegistry_TryGetCatalog(catalogId string) (*A2UiCatalogDescriptor, bool) {
	if strings.TrimSpace(catalogId) == "" {
		return &A2UiCatalogDescriptor{}, false
	}

	// 转换为小写匹配
	if cat, ok := A2UiCatalogRegistry_Catalogs[strings.ToLower(catalogId)]; ok {
		return &cat, true
	}

	return &A2UiCatalogDescriptor{}, false
}

func A2UiCatalogRegistry_TryChooseCatalog(supportedCatalogIds []string, requestedCatalogId string) (*A2UiCatalogDescriptor, bool) {
	if strings.TrimSpace(requestedCatalogId) != "" {
		if supportedCatalogIds != nil {
			found := false
			for _, id := range supportedCatalogIds {
				if strings.EqualFold(id, requestedCatalogId) {
					found = true
					break
				}
			}
			if !found {
				return &A2UiCatalogDescriptor{}, false
			}
		}
		return A2UiCatalogRegistry_TryGetCatalog(requestedCatalogId)
	}

	if supportedCatalogIds != nil {
		// 优先匹配 AGenUiCatalogId
		for _, id := range supportedCatalogIds {
			if strings.EqualFold(id, A2UiCatalogRegistry_AGenUiCatalogId) {
				return A2UiCatalogRegistry_TryGetCatalog(A2UiCatalogRegistry_AGenUiCatalogId)
			}
		}

		// 遍历支持的列表尝试获取第一个匹配成功的
		for _, id := range supportedCatalogIds {
			if cat, ok := A2UiCatalogRegistry_TryGetCatalog(id); ok {
				return cat, true
			}
		}

		if len(supportedCatalogIds) > 0 {
			return &A2UiCatalogDescriptor{}, false
		}
	}

	return A2UiCatalogRegistry_TryGetCatalog(A2UiCatalogRegistry_AGenUiCatalogId)
}

func A2UiCatalogRegistry_ResolveComponentType(catalog *A2UiCatalogDescriptor, componentType string) (string, bool) {
	if strings.TrimSpace(componentType) == "" {
		return "", false
	}

	lowerCompType := strings.ToLower(componentType)

	if _, ok := catalog.ComponentTypes[lowerCompType]; ok {
		return componentType, true // 匹配成功，返回原值
	}

	if canonical, ok := catalog.Aliases[lowerCompType]; ok {
		if _, exists := catalog.ComponentTypes[strings.ToLower(canonical)]; exists {
			return canonical, true // 匹配别名成功，返回标准名
		}
	}

	return "", false
}

func A2UiCatalogRegistry_IsSupportedComponentType(catalog *A2UiCatalogDescriptor, componentType string) bool {
	_, ok := A2UiCatalogRegistry_ResolveComponentType(catalog, componentType)
	return ok
}

type A2UiValidationResult struct {
	IsValid    bool   `json:"is_valid"`
	Error      string `json:"error,omitempty"`
	FrameCount int    `json:"frame_count"`
}

func NewValidResult(frameCount int) *A2UiValidationResult {
	return &A2UiValidationResult{
		IsValid:    true,
		FrameCount: frameCount,
	}
}

func NewInvalidResult(errMessage string, frameCount ...int) *A2UiValidationResult {
	count := 0
	if len(frameCount) > 0 {
		count = frameCount[0]
	}
	return &A2UiValidationResult{
		IsValid:    false,
		Error:      errMessage,
		FrameCount: count,
	}
}

// DefaultA2UiValidationResult 提供默认值方法
func DefaultA2UiValidationResult() *A2UiValidationResult {
	return &A2UiValidationResult{
		IsValid:    false,
		FrameCount: 0,
	}
}

// 带有类名前缀的常量
const A2UiFrameValidatorContentTypeV08 = "application/x-a2ui+jsonl;version=0.8"

var a2uiFrameValidatorSupportedTypes = map[string]bool{
	"text":      true,
	"markdown":  true,
	"card":      true,
	"button":    true,
	"input":     true,
	"select":    true,
	"checklist": true,
	"table":     true,
	"image":     true,
	"progress":  true,
	"chart":     true,
}

func ValidateJsonl(frames string, maxFrames int, maxBytes int) *A2UiValidationResult {
	if strings.TrimSpace(frames) == "" {
		return NewInvalidResult("A2UI frames are required.")
	}

	// 计算 UTF-8 字节数
	if len([]byte(frames)) > max(1, maxBytes) {
		return NewInvalidResult(fmt.Sprintf("A2UI frame payload exceeds %d bytes.", maxBytes))
	}

	// 统一换行符并分割
	normalized := strings.ReplaceAll(frames, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	count := 0

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if len(line) == 0 {
			continue
		}

		count++
		if count > max(1, maxFrames) {
			return NewInvalidResult(fmt.Sprintf("A2UI push exceeds %d frames.", maxFrames), count)
		}

		var root map[string]interface{}
		if err := json.Unmarshal([]byte(line), &root); err != nil {
			return NewInvalidResult(fmt.Sprintf("Frame %d is not valid JSON: %s", count, err.Error()), count)
		}

		// 拦截 v0.9 指令
		if hasString(root, "command", "createSurface") || hasString(root, "type", "createSurface") {
			return NewInvalidResult("A2UI v0.9 createSurface is not supported; use v0.8 JSONL frames.", count)
		}

		// 校验 type
		typeVal, hasType := root["type"].(string)
		if !hasType {
			return NewInvalidResult(fmt.Sprintf("Frame %d is missing string property 'type'.", count), count)
		}

		typeLower := strings.ToLower(typeVal)
		if strings.TrimSpace(typeVal) == "" || !a2uiFrameValidatorSupportedTypes[typeLower] {
			return NewInvalidResult(fmt.Sprintf("Frame %d has unsupported A2UI type '%s'.", count, typeVal), count)
		}

		// 校验 id
		if !hasNonEmptyString(root, "id") {
			return NewInvalidResult(fmt.Sprintf("Frame %d is missing string property 'id'.", count), count)
		}

		// 按组件类型进行子校验
		if typeError := validateByType(root, typeLower); typeError != "" {
			return NewInvalidResult(fmt.Sprintf("Frame %d %s", count, typeError), count)
		}
	}

	if count == 0 {
		return NewInvalidResult("A2UI frames are required.")
	}

	return NewValidResult(count)
}

func validateByType(root map[string]interface{}, typeLower string) string {
	switch typeLower {
	case "text", "markdown":
		if hasNonEmptyString(root, "text") {
			return ""
		}
		return "is missing string property 'text'."
	case "card":
		if hasNonEmptyString(root, "title") || hasNonEmptyString(root, "body") {
			return ""
		}
		return "requires 'title' or 'body'."
	case "button":
		if hasNonEmptyString(root, "label") {
			return ""
		}
		return "is missing string property 'label'."
	case "input":
		return ""
	case "select", "checklist":
		if hasArray(root, "options") {
			return ""
		}
		return "is missing array property 'options'."
	case "table":
		if hasArray(root, "columns") && hasArray(root, "rows") {
			return ""
		}
		return "requires array properties 'columns' and 'rows'."
	case "image":
		if hasNonEmptyString(root, "url") || hasNonEmptyString(root, "src") {
			return ""
		}
		return "requires 'url' or 'src'."
	case "progress":
		if hasNumber(root, "value") {
			return ""
		}
		return "is missing numeric property 'value'."
	case "chart":
		if _, exists := root["data"]; exists {
			return ""
		}
		return "is missing property 'data'."
	default:
		return "has unsupported type."
	}
}

// 辅助校验函数组
func hasArray(root map[string]interface{}, prop string) bool {
	val, exists := root[prop]
	if !exists {
		return false
	}
	_, isArray := val.([]interface{})
	return isArray
}

func hasNumber(root map[string]interface{}, prop string) bool {
	val, exists := root[prop]
	if !exists {
		return false
	}
	switch val.(type) {
	case float64, int, int64: // Go JSON unmarshal 默认数形式是 float64
		return true
	}
	return false
}

func hasNonEmptyString(root map[string]interface{}, prop string) bool {
	val, exists := root[prop]
	if !exists {
		return false
	}
	str, isStr := val.(string)
	return isStr && strings.TrimSpace(str) != ""
}

func hasString(root map[string]interface{}, prop string, expected string) bool {
	val, exists := root[prop]
	if !exists {
		return false
	}
	str, isStr := val.(string)
	return isStr && strings.EqualFold(str, expected)
}

var a2uiV09MessageValidatorSupportedOperations = map[string]bool{
	"createsurface":    true,
	"updatecomponents": true,
	"updatedatamodel":  true,
	"deletesurface":    true,
	"syncuitodata":     true,
	"action":           true,
	"error":            true,
}

func ValidateV09(envelope *WsServerEnvelope) *A2UiValidationResult {
	if envelope == nil {
		panic("envelope cannot be nil")
	}

	if strings.TrimSpace(envelope.Operation) == "" {
		return NewInvalidResult("A2UI v0.9 operation is required.")
	}

	opLower := strings.ToLower(envelope.Operation)
	if !a2uiV09MessageValidatorSupportedOperations[opLower] {
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 operation '%s' is not supported.", opLower))
	}

	switch opLower {
	case "createsurface":
		return validateCreateSurface(envelope)
	case "updatecomponents":
		return validateUpdateComponents(envelope)
	case "updatedatamodel":
		return validateUpdateDataModel(envelope)
	case "deletesurface":
		return validateSurfaceOperation(envelope, "deleteSurface")
	case "syncuitodata":
		return validateSyncUIToData(envelope)
	case "action":
		return validateAction(envelope)
	case "error":
		return validateError(envelope)
	default:
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 operation '%s' is not supported.", opLower))
	}
}

func validateCreateSurface(envelope *WsServerEnvelope) *A2UiValidationResult {
	if errStr := validateSurfaceId(envelope); errStr != "" {
		return NewInvalidResult(errStr)
	}

	catalogId := envelope.CatalogId

	catalog, success := A2UiCatalogRegistry_TryChooseCatalog(envelope.SupportedCatalogIds, catalogId)
	if !success {
		return NewInvalidResult("A2UI v0.9 createSurface uses an unsupported catalog ID.")
	}

	if envelope.Components != nil {
		componentValidation := validateComponents(envelope.Components, catalog, "createSurface")
		if !componentValidation.IsValid {
			return componentValidation
		}
	}

	jsonStr := envelope.DataModelJson
	if res := validateOptionalJsonObject(jsonStr, "dataModelJson"); res != nil {
		return res
	}
	return NewValidResult(1)
}

func validateUpdateComponents(envelope *WsServerEnvelope) *A2UiValidationResult {
	if errStr := validateSurfaceId(envelope); errStr != "" {
		return NewInvalidResult(errStr)
	}

	catalogId := envelope.CatalogId
	catalog, success := A2UiCatalogRegistry_TryChooseCatalog(envelope.SupportedCatalogIds, catalogId)
	if !success {
		return NewInvalidResult("A2UI v0.9 updateComponents uses an unsupported catalog ID.")
	}

	if len(envelope.Components) == 0 {
		return NewInvalidResult("A2UI v0.9 updateComponents requires components as a non-empty JSON string array.")
	}

	return validateComponents(envelope.Components, catalog, "updateComponents")
}

func validateComponents(components []string, catalog *A2UiCatalogDescriptor, operation string) *A2UiValidationResult {
	if len(components) == 0 {
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 %s requires components as a non-empty JSON string array.", operation))
	}

	for i, componentJson := range components {
		index := i + 1
		if strings.TrimSpace(componentJson) == "" {
			return NewInvalidResult(fmt.Sprintf("A2UI v0.9 component %d must be a non-empty JSON string.", index))
		}

		var component map[string]interface{}
		if err := json.Unmarshal([]byte(componentJson), &component); err != nil {
			return NewInvalidResult(fmt.Sprintf("A2UI v0.9 component %d is not valid JSON: %s", index, err.Error()))
		}

		if _, hasId := component["id"]; !hasId || !hasNonEmptyString(component, "id") {
			return NewInvalidResult(fmt.Sprintf("A2UI v0.9 component %d is missing string property 'id'.", index))
		}

		typeVal, hasType := component["type"].(string)
		if !hasType {
			return NewInvalidResult(fmt.Sprintf("A2UI v0.9 component %d is missing string property 'type'.", index))
		}

		if !A2UiCatalogRegistry_IsSupportedComponentType(catalog, typeVal) {
			return NewInvalidResult(fmt.Sprintf("A2UI v0.9 component %d has unsupported component type '%s'.", index, typeVal))
		}
	}

	return NewValidResult(1)
}

func validateUpdateDataModel(envelope *WsServerEnvelope) *A2UiValidationResult {
	if errStr := validateSurfaceId(envelope); errStr != "" {
		return NewInvalidResult(errStr)
	}

	if strings.TrimSpace(envelope.DataModelJson) == "" {
		return NewInvalidResult("A2UI v0.9 updateDataModel requires dataModelJson.")
	}

	var root map[string]interface{}
	if err := json.Unmarshal([]byte(envelope.DataModelJson), &root); err != nil {
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 dataModelJson is not valid JSON: %s", err.Error()))
	}

	return NewValidResult(1)
}

func validateSurfaceOperation(envelope *WsServerEnvelope, operation string) *A2UiValidationResult {
	if errStr := validateSurfaceId(envelope); errStr != "" {
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 %s %s", operation, errStr))
	}
	return NewValidResult(1)
}

func validateSyncUIToData(envelope *WsServerEnvelope) *A2UiValidationResult {
	if errStr := validateSurfaceId(envelope); errStr != "" {
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 syncUIToData %s", errStr))
	}

	jsonStr := envelope.DataModelJson
	if res := validateOptionalJsonObject(jsonStr, "dataModelJson"); res != nil {
		return res
	}
	return NewValidResult(1)
}

func validateAction(envelope *WsServerEnvelope) *A2UiValidationResult {
	if errStr := validateSurfaceId(envelope); errStr != "" {
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 action %s", errStr))
	}

	if strings.TrimSpace(envelope.Action) == "" {
		return NewInvalidResult("A2UI v0.9 action requires action.")
	}

	jsonStr := envelope.ParametersJson
	if res := validateOptionalJsonObject(jsonStr, "parametersJson"); res != nil {
		return res
	}
	return NewValidResult(1)
}

func validateError(envelope *WsServerEnvelope) *A2UiValidationResult {
	err := envelope.Error
	code := envelope.DiagnosticCode
	if strings.TrimSpace(err) == "" && strings.TrimSpace(code) == "" {
		return NewInvalidResult("A2UI v0.9 error requires error or diagnosticCode.")
	}
	return NewValidResult(1)
}

func validateOptionalJsonObject(jsonStr string, propertyName string) *A2UiValidationResult {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}

	var root map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &root); err != nil {
		return NewInvalidResult(fmt.Sprintf("A2UI v0.9 %s is not valid JSON: %s", propertyName, err.Error()))
	}

	return nil
}

func validateSurfaceId(envelope *WsServerEnvelope) string {
	if strings.TrimSpace(envelope.SurfaceId) == "" {
		return "requires non-empty surfaceId."
	}
	return ""
}
