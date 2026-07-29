package core

import (
	"encoding/json"
	"errors"
)

type IResourceContents interface {
	GetUri() string
	GetMimeType() *string
	GetMeta() map[string]any
}

type BaseResourceContents struct {
	Uri      string         `json:"uri"`
	MimeType *string        `json:"mimeType,omitempty"`
	Meta     map[string]any `json:"_meta,omitempty"`
}

func (r *BaseResourceContents) GetUri() string {
	return r.Uri
}

func (r *BaseResourceContents) GetMimeType() *string {
	return r.MimeType
}

func (r *BaseResourceContents) GetMeta() map[string]any {
	return r.Meta
}

type BlobResourceContents struct {
	BaseResourceContents
	Blob []byte `json:"blob"`
}

type TextResourceContents struct {
	BaseResourceContents
	Text string `json:"text"`
}

type ResourceContents struct {
	IResourceContents
}

func (r *ResourceContents) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		r.IResourceContents = nil
		return nil
	}

	// 1. 探测 JSON 属性结构
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// 2. 根据特征属性多态解析
	if _, hasBlob := rawMap["blob"]; hasBlob {
		var blobRes BlobResourceContents
		if err := json.Unmarshal(data, &blobRes); err != nil {
			return err
		}
		r.IResourceContents = &blobRes
		return nil
	}

	if _, hasText := rawMap["text"]; hasText {
		var textRes TextResourceContents
		if err := json.Unmarshal(data, &textRes); err != nil {
			return err
		}
		r.IResourceContents = &textRes
		return nil
	}

	return errors.New("invalid ResourceContents JSON: missing 'blob' or 'text' property")
}

func (r ResourceContents) MarshalJSON() ([]byte, error) {
	if r.IResourceContents == nil {
		return []byte("null"), nil
	}
	return json.Marshal(r.IResourceContents)
}
