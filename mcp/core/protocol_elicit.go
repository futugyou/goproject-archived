package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

type ElicitRequestParams struct {
	RequestParams
	Mode            string         `json:"mode"`
	ElicitationId   *string        `json:"elicitationId,omitempty"`
	Url             *string        `json:"url,omitempty"`
	Message         string         `json:"message"`
	RequestedSchema *RequestSchema `json:"requestedSchema,omitempty"`
}

type RequestSchema struct {
	Type       string                               `json:"type"`
	Properties map[string]PrimitiveSchemaDefinition `json:"properties,omitempty"`
	Required   []string                             `json:"required,omitempty"`
}

type ElicitResult struct {
	Meta       map[string]any             `json:"_meta,omitempty"`
	ResultType string                     `json:"resultType"`
	Action     string                     `json:"action"`
	Content    map[string]json.RawMessage `json:"content,omitempty"`
}

func (e *ElicitResult) IsAccepted() bool {
	return strings.EqualFold(e.Action, "accept")
}

type ElicitResultTyped[T any] struct {
	Meta       map[string]any `json:"_meta,omitempty"`
	ResultType string         `json:"resultType"`
	Action     string         `json:"action"`
	Content    T              `json:"content,omitempty"`
}

func (e *ElicitResultTyped[T]) IsAccepted() bool {
	return strings.EqualFold(e.Action, "accept")
}

func FromElicitResult(result ElicitResult) (InputResponse, error) {
	if result.Action == "" {
		result.Action = "cancel"
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return InputResponse{}, err
	}
	return InputResponse{RawValue: bytes}, nil
}

type FormElicitationCapability struct{}
type UrlElicitationCapability struct{}

type ElicitationCapability struct {
	Form *FormElicitationCapability `json:"form,omitempty"`
	Url  *UrlElicitationCapability  `json:"url,omitempty"`
}

func (e *ElicitationCapability) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return errors.New("elicitation capability must be an object")
	}

	if formVal, exists := raw["form"]; exists {
		if string(formVal) != "null" {
			e.Form = &FormElicitationCapability{}
		}
	}

	if urlVal, exists := raw["url"]; exists {
		if string(urlVal) != "null" {
			e.Url = &UrlElicitationCapability{}
		}
	}

	if e.Form == nil && e.Url == nil {
		e.Form = &FormElicitationCapability{}
	}

	return nil
}

func (e *ElicitationCapability) MarshalJSON() ([]byte, error) {
	out := make(map[string]any)

	writeForm := e.Form != nil || e.Url == nil
	if writeForm {
		out["form"] = struct{}{} // `{}`
	}

	if e.Url != nil {
		out["url"] = struct{}{} // `{}`
	}

	return json.Marshal(out)
}

type EnumSchemaOption struct {
	Const string `json:"const"`
	Title string `json:"title"`
}

type UntitledEnumItemsSchema struct {
	Type string   `json:"type"`
	Enum []string `json:"enum"`
}

type TitledEnumItemsSchema struct {
	AnyOf []EnumSchemaOption `json:"anyOf"`
}

type IPrimitiveSchemaDefinition interface {
	GetType() string
	SetType(t string)
	GetTitle() *string
	SetTitle(title *string)
	GetDescription() *string
	SetDescription(desc *string)
}

type BasePrimitiveSchemaDefinition struct {
	Type        string  `json:"type"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (b *BasePrimitiveSchemaDefinition) GetType() string             { return b.Type }
func (b *BasePrimitiveSchemaDefinition) SetType(t string)            { b.Type = t }
func (b *BasePrimitiveSchemaDefinition) GetTitle() *string           { return b.Title }
func (b *BasePrimitiveSchemaDefinition) SetTitle(title *string)      { b.Title = title }
func (b *BasePrimitiveSchemaDefinition) GetDescription() *string     { return b.Description }
func (b *BasePrimitiveSchemaDefinition) SetDescription(desc *string) { b.Description = desc }

type PrimitiveSchemaDefinition struct {
	IPrimitiveSchemaDefinition
}

func (p *PrimitiveSchemaDefinition) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		p.IPrimitiveSchemaDefinition = nil
		return nil
	}

	// 1. 中间数据载体，提取所有可能存在的字段
	var raw struct {
		Type        *string            `json:"type"`
		Title       *string            `json:"title"`
		Description *string            `json:"description"`
		MinLength   *int               `json:"minLength"`
		MaxLength   *int               `json:"maxLength"`
		Format      *string            `json:"format"`
		Minimum     *float64           `json:"minimum"`
		Maximum     *float64           `json:"maximum"`
		MinItems    *int               `json:"minItems"`
		MaxItems    *int               `json:"maxItems"`
		Enum        []string           `json:"enum"`
		OneOf       []EnumSchemaOption `json:"oneOf"`
		Items       json.RawMessage    `json:"items"`
		Default     json.RawMessage    `json:"default"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.Type == nil {
		return fmt.Errorf("the 'type' property is required")
	}

	schemaType := *raw.Type
	var target IPrimitiveSchemaDefinition

	// 2. 根据 type 以及特定属性判断具体的 Schema 类型
	switch schemaType {
	case "string":
		if len(raw.OneOf) > 0 {
			// TitledSingleSelectEnumSchema
			var def *string
			if len(raw.Default) > 0 {
				_ = json.Unmarshal(raw.Default, &def)
			}
			target = &TitledSingleSelectEnumSchema{
				OneOf:   raw.OneOf,
				Default: def,
			}
			target.SetType("string")
		} else if len(raw.Enum) > 0 {
			// UntitledSingleSelectEnumSchema
			var def *string
			if len(raw.Default) > 0 {
				_ = json.Unmarshal(raw.Default, &def)
			}
			target = &UntitledSingleSelectEnumSchema{
				Enum:    raw.Enum,
				Default: def,
			}
			target.SetType("string")
		} else {
			// StringSchema
			var def *string
			if len(raw.Default) > 0 {
				_ = json.Unmarshal(raw.Default, &def)
			}
			target = &StringSchema{
				MinLength: raw.MinLength,
				MaxLength: raw.MaxLength,
				Format:    raw.Format,
				Default:   def,
			}
			target.SetType("string")
		}

	case "array":
		var itemsObj any
		if len(raw.Items) > 0 {
			// 尝试解析 Items
			var titledItems TitledEnumItemsSchema
			var untitledItems UntitledEnumItemsSchema

			if err := json.Unmarshal(raw.Items, &titledItems); err == nil && len(titledItems.AnyOf) > 0 {
				itemsObj = titledItems
			} else if err := json.Unmarshal(raw.Items, &untitledItems); err == nil && (len(untitledItems.Enum) > 0 || untitledItems.Type != "") {
				if untitledItems.Type == "" {
					untitledItems.Type = "string"
				}
				itemsObj = untitledItems
			} else {
				return fmt.Errorf("items schema must have either 'enum' or 'anyOf' property")
			}
		}

		var def []string
		if len(raw.Default) > 0 {
			_ = json.Unmarshal(raw.Default, &def)
		}

		switch it := itemsObj.(type) {
		case TitledEnumItemsSchema:
			target = &TitledMultiSelectEnumSchema{
				MinItems: raw.MinItems,
				MaxItems: raw.MaxItems,
				Items:    it,
				Default:  def,
			}
			target.SetType("array")
		case UntitledEnumItemsSchema:
			target = &UntitledMultiSelectEnumSchema{
				MinItems: raw.MinItems,
				MaxItems: raw.MaxItems,
				Items:    it,
				Default:  def,
			}
			target.SetType("array")
		}

	case "number", "integer":
		var def *float64
		if len(raw.Default) > 0 {
			_ = json.Unmarshal(raw.Default, &def)
		}
		target = &NumberSchema{
			Minimum: raw.Minimum,
			Maximum: raw.Maximum,
			Default: def,
		}
		target.SetType("number")
	case "boolean":
		var def *bool
		if len(raw.Default) > 0 {
			_ = json.Unmarshal(raw.Default, &def)
		}
		target = &BooleanSchema{
			Default: def,
		}
		target.SetType("boolean")
	}

	if target == nil {
		return fmt.Errorf("unexpected schema type or configuration for type: %s", schemaType)
	}

	target.SetType(schemaType)
	target.SetTitle(raw.Title)
	target.SetDescription(raw.Description)

	p.IPrimitiveSchemaDefinition = target
	return nil
}

func (p PrimitiveSchemaDefinition) MarshalJSON() ([]byte, error) {
	if p.IPrimitiveSchemaDefinition == nil {
		return []byte("null"), nil
	}

	return json.Marshal(p.IPrimitiveSchemaDefinition)
}

type StringSchema struct {
	BasePrimitiveSchemaDefinition
	Format    *string `json:"format,omitempty"`
	Default   *string `json:"default,omitempty"`
	MinLength *int    `json:"minLength,omitempty"`
	MaxLength *int    `json:"maxLength,omitempty"`
}

type TitledSingleSelectEnumSchema struct {
	BasePrimitiveSchemaDefinition
	OneOf   []EnumSchemaOption `json:"oneOf"`
	Default *string            `json:"default,omitempty"`
}

type UntitledSingleSelectEnumSchema struct {
	BasePrimitiveSchemaDefinition
	Default *string  `json:"default,omitempty"`
	Enum    []string `json:"enum"`
}

type TitledMultiSelectEnumSchema struct {
	BasePrimitiveSchemaDefinition
	Default  []string              `json:"default,omitempty"`
	MinItems *int                  `json:"minItems"`
	MaxItems *int                  `json:"maxItems"`
	Items    TitledEnumItemsSchema `json:"items"`
}

type UntitledMultiSelectEnumSchema struct {
	BasePrimitiveSchemaDefinition
	Default  []string                `json:"default,omitempty"`
	MinItems *int                    `json:"minItems"`
	MaxItems *int                    `json:"maxItems"`
	Items    UntitledEnumItemsSchema `json:"items"`
}

type NumberSchema struct {
	BasePrimitiveSchemaDefinition
	Default *float64 `json:"default,omitempty"`
	Minimum *float64 `json:"minimum"`
	Maximum *float64 `json:"maximum"`
}

type BooleanSchema struct {
	BasePrimitiveSchemaDefinition
	Default *bool `json:"default,omitempty"`
}

type ElicitationCompleteNotificationParams struct {
	Meta          map[string]any `json:"_meta,omitempty"`
	ElicitationId string         `json:"elicitationId"`
}

var PrimitiveSchemasMap map[reflect.Type]*jsonschema.Schema
var primitiveSchemasSlice []*jsonschema.Schema

func init() {
	str, _ := jsonschema.For[StringSchema](nil)
	num, _ := jsonschema.For[NumberSchema](nil)
	boo, _ := jsonschema.For[BooleanSchema](nil)
	title, _ := jsonschema.For[TitledSingleSelectEnumSchema](nil)
	unitsingle, _ := jsonschema.For[UntitledSingleSelectEnumSchema](nil)
	multtile, _ := jsonschema.For[TitledMultiSelectEnumSchema](nil)
	multunit, _ := jsonschema.For[UntitledMultiSelectEnumSchema](nil)

	PrimitiveSchemasMap = map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[StringSchema]():                   str,
		reflect.TypeFor[NumberSchema]():                   num,
		reflect.TypeFor[BooleanSchema]():                  boo,
		reflect.TypeFor[TitledSingleSelectEnumSchema]():   title,
		reflect.TypeFor[UntitledSingleSelectEnumSchema](): unitsingle,
		reflect.TypeFor[TitledMultiSelectEnumSchema]():    multtile,
		reflect.TypeFor[UntitledMultiSelectEnumSchema]():  multunit,
	}

	primitiveSchemasSlice = make([]*jsonschema.Schema, 0, len(PrimitiveSchemasMap))
	for _, schema := range PrimitiveSchemasMap {
		primitiveSchemasSlice = append(primitiveSchemasSlice, schema)
	}
}

func (PrimitiveSchemaDefinition) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: "Primitive schema definition accommodating string, number, boolean, or enum types.",
		OneOf:       primitiveSchemasSlice,
	}
}

func GetPrimitiveSchemaByType[T any]() (*jsonschema.Schema, bool) {
	schemaType := reflect.TypeOf((*T)(nil)).Elem()
	if schemaType.Kind() == reflect.Pointer {
		schemaType = schemaType.Elem()
	}
	s, exists := PrimitiveSchemasMap[schemaType]
	return s, exists
}

func ValidatePrimitiveSchemaDefinition[T any](jsonBytes []byte) error {
	schema, exists := GetPrimitiveSchemaByType[T]()
	if !exists {
		typeName := reflect.TypeFor[T]().Name()
		return fmt.Errorf("%s type exist", typeName)
	}

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return err
	}

	var value any
	if err := json.Unmarshal(jsonBytes, &value); err != nil {
		return err
	}

	if err := resolved.Validate(value); err != nil {
		return err
	}

	return nil
}
