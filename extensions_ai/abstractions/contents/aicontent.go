package contents

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

var _ IAIContent = (*AIContent)(nil)

type IAIContent interface {
	GetRawRepresentation() any
	GetAdditionalProperties() map[string]any
}

type AIContent struct {
	RawRepresentation    any            // Raw representation of the content (for debugging or underlying object model).
	AdditionalProperties map[string]any `json:"additionalProperties,omitempty"` // Additional properties for the content.
}

func NewAIContent(rawRepresentation any, additionalProperties map[string]any) *AIContent {
	if additionalProperties == nil {
		additionalProperties = make(map[string]any)
	}
	return &AIContent{
		RawRepresentation:    rawRepresentation,
		AdditionalProperties: additionalProperties,
	}
}

// GetAdditionalProperties implements IAIContent.
func (ac *AIContent) GetAdditionalProperties() map[string]any {
	return ac.AdditionalProperties
}

// GetRawRepresentation implements IAIContent.
func (ac *AIContent) GetRawRepresentation() any {
	return ac.RawRepresentation
}

// AddAdditionalProperty allows adding properties to the content.
func (ac *AIContent) AddAdditionalProperty(key string, value any) {
	if ac.AdditionalProperties == nil {
		ac.AdditionalProperties = make(map[string]any)
	}
	ac.AdditionalProperties[key] = value
}

// PrintContentInfo prints out the details of the content type.
func PrintContentInfo(content AIContent) {
	contentType := reflect.TypeFor[AIContent]()
	fmt.Printf("Content Type: %s\n", contentType.Name())
	if len(content.AdditionalProperties) > 0 {
		fmt.Println("Additional Properties:")
		for key, value := range content.AdditionalProperties {
			fmt.Printf(" - %s: %v\n", key, value)
		}
	}
}

func (ac AIContent) MarshalJSON() ([]byte, error) {
	type Alias AIContent
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "AIContent",
		Alias: (*Alias)(&ac),
	})
}

func (ac *AIContent) UnmarshalJSON(data []byte) error {
	type Alias AIContent
	aux := &struct {
		Type string `json:"type"`
		*Alias
	}{
		Alias: (*Alias)(ac),
	}

	return json.Unmarshal(data, aux)
}

func ConcatTextContents(contents []IAIContent) string {
	var text strings.Builder
	for _, content := range contents {
		if textContent, ok := content.(TextContent); ok {
			text.WriteString(textContent.Text)
		}
	}
	return text.String()
}

var ContentTypeRegistry = map[string]func() IAIContent{
	"AIContent":             func() IAIContent { return &AIContent{} },
	"DataContent":           func() IAIContent { return &DataContent{} },
	"ErrorContent":          func() IAIContent { return &ErrorContent{} },
	"FunctionCallContent":   func() IAIContent { return &FunctionCallContent{} },
	"FunctionResultContent": func() IAIContent { return &FunctionResultContent{} },
	"TextContent":           func() IAIContent { return &TextContent{} },
	"UriContent":            func() IAIContent { return &UriContent{} },
	"TextReasoningContent":  func() IAIContent { return &TextReasoningContent{} },
	"UsageContent":          func() IAIContent { return &UsageContent{} },
}
