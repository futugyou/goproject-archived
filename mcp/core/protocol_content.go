package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

type IContentBlock interface {
	GetType() string
	GetAnnotations() *Annotations
	GetMeta() map[string]any
}

type BaseContentBlock struct {
	Annotations *Annotations   `json:"annotations,omitempty"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

func (b *BaseContentBlock) GetAnnotations() *Annotations {
	return b.Annotations
}

func (b *BaseContentBlock) GetMeta() map[string]any {
	return b.Meta
}

// --- TextContentBlock ---
type TextContentBlock struct {
	BaseContentBlock
	Text string `json:"text"`
}

func (t *TextContentBlock) GetType() string { return "text" }

// --- ImageContentBlock ---
type ImageContentBlock struct {
	BaseContentBlock
	Data     []byte `json:"-"`
	MimeType string `json:"mimeType"`
}

func (i *ImageContentBlock) GetType() string { return "image" }

// --- AudioContentBlock ---
type AudioContentBlock struct {
	BaseContentBlock
	Data     []byte `json:"-"`
	MimeType string `json:"mimeType"`
}

func (a *AudioContentBlock) GetType() string { return "audio" }

// --- EmbeddedResourceBlock ---
type EmbeddedResourceBlock struct {
	BaseContentBlock
	Resource ResourceContents `json:"resource"`
}

func (e *EmbeddedResourceBlock) GetType() string { return "resource" }

// --- ResourceLinkBlock ---
type ResourceLinkBlock struct {
	BaseContentBlock
	URI         string  `json:"uri"`
	Name        string  `json:"name"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	MimeType    *string `json:"mimeType,omitempty"`
	Size        *int64  `json:"size,omitempty"`
	Icons       []Icon  `json:"icons,omitempty"`
}

type Icon struct {
	Source   string   `json:"src"`
	MimeType string   `json:"mimeType"`
	Sizes    []string `json:"sizes"`
	Theme    string   `json:"theme"`
}

func (r *ResourceLinkBlock) GetType() string { return "resource_link" }

type ToolUseContentBlock struct {
	BaseContentBlock
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

func (t *ToolUseContentBlock) GetType() string { return "tool_use" }

type ToolResultContentBlock struct {
	BaseContentBlock
	ToolUseId         string         `json:"toolUseId"`
	Content           []ContentBlock `json:"content"`
	StructuredContent map[string]any `json:"structuredContent,omitempty"`
	IsError           *bool          `json:"isError,omitempty"`
}

func (t *ToolResultContentBlock) GetType() string { return "tool_result" }

type ContentBlock struct {
	IContentBlock
}

func (cb *ContentBlock) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		cb.IContentBlock = nil
		return nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	typeMsg, ok := rawMap["type"]
	if !ok {
		return errors.New("missing 'type' field in ContentBlock JSON")
	}

	var blockType string
	if err := json.Unmarshal(typeMsg, &blockType); err != nil {
		return err
	}

	switch blockType {
	case "text":
		var b TextContentBlock
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		if b.Text == "" {
			return errors.New("Text contents must be provided for 'text' type.")
		}
		cb.IContentBlock = &b

	case "image":
		var b ImageContentBlock
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		if b.MimeType == "" {
			return errors.New("MIME type must be provided for 'image' type.")
		}
		if len(b.Data) == 0 {
			return errors.New("Image data must be provided for 'image' type.")
		}
		cb.IContentBlock = &b

	case "audio":
		var b AudioContentBlock
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		if b.MimeType == "" {
			return errors.New("MIME type must be provided for 'audio' type.")
		}
		if len(b.Data) == 0 {
			return errors.New("Audio data must be provided for 'audio' type.")
		}
		cb.IContentBlock = &b

	case "resource":
		var b EmbeddedResourceBlock
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		cb.IContentBlock = &b

	case "resource_link":
		var b ResourceLinkBlock
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		if b.URI == "" || b.Name == "" {
			return errors.New("URI and Name must be provided for 'resource_link' type.")
		}
		cb.IContentBlock = &b

	case "tool_use":
		var b ToolUseContentBlock
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		if b.ID == "" || b.Name == "" {
			return errors.New("ID and Name must be provided for 'tool_use' type.")
		}
		cb.IContentBlock = &b

	case "tool_result":
		var raw struct {
			BaseContentBlock
			ToolUseId         string          `json:"toolUseId"`
			Content           json.RawMessage `json:"content"`
			StructuredContent map[string]any  `json:"structuredContent,omitempty"`
			IsError           *bool           `json:"isError,omitempty"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		if raw.ToolUseId == "" {
			return errors.New("ToolUseId must be provided for 'tool_result' type.")
		}

		var contents []ContentBlock
		if len(raw.Content) > 0 {
			if raw.Content[0] == '[' {
				if err := json.Unmarshal(raw.Content, &contents); err != nil {
					return err
				}
			} else {
				var single ContentBlock
				if err := json.Unmarshal(raw.Content, &single); err != nil {
					return err
				}
				contents = []ContentBlock{single}
			}
		}

		cb.IContentBlock = &ToolResultContentBlock{
			BaseContentBlock:  raw.BaseContentBlock,
			ToolUseId:         raw.ToolUseId,
			Content:           contents,
			StructuredContent: raw.StructuredContent,
			IsError:           raw.IsError,
		}

	default:
		return fmt.Errorf("unknown content type: '%s'", blockType)
	}

	return nil
}

func (cb ContentBlock) MarshalJSON() ([]byte, error) {
	if cb.IContentBlock == nil {
		return []byte("null"), nil
	}

	rawBytes, err := json.Marshal(cb.IContentBlock)
	if err != nil {
		return nil, err
	}

	var blockMap map[string]any
	if err := json.Unmarshal(rawBytes, &blockMap); err != nil {
		return nil, err
	}

	blockMap["type"] = cb.IContentBlock.GetType()

	return json.Marshal(blockMap)
}
