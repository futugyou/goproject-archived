package protocol

import (
	"encoding/json"
	"time"
)

type ListToolsRequestParams struct {
	PaginatedRequestParams `json:",inline"`
}

type ListToolsResult struct {
	PaginatedResult `json:",inline"`
	Tools           []Tool      `json:"tools"`
	TimeToLive      time.Time   `json:"ttlMs"`
	CacheScope      *CacheScope `json:"cacheScope"`
}

type CallToolRequestParams struct {
	RequestParams `json:",inline"`
	Name          string         `json:"name"`
	Arguments     map[string]any `json:"arguments"`
}

type CallToolResult struct {
	Meta       map[string]any `json:"_meta,omitempty" yaml:"_meta,omitempty" mapstructure:"_meta,omitempty"`
	ResultType string         `json:"resultType,omitempty" yaml:"resultType,omitempty" mapstructure:"resultType,omitempty"`

	Content           []ContentBlock `json:"content" yaml:"content" mapstructure:"content"`
	StructuredContent map[string]any `json:"structuredContent" yaml:"structuredContent" mapstructure:"structuredContent"`
	IsError           bool           `json:"isError,omitempty" yaml:"isError,omitempty" mapstructure:"isError,omitempty"`
}

func NewCallToolResultWithContents(contents []ContentBlock) *CallToolResult {
	return &CallToolResult{
		Meta:    make(map[string]any),
		Content: contents,
	}
}

func NewCallToolResultWithContent(content ContentBlock) *CallToolResult {
	return &CallToolResult{
		Meta:    make(map[string]any),
		Content: []ContentBlock{content},
	}
}

type Tool struct {
	Name         string           `json:"name"`
	Title        string           `json:"title"`
	Description  *string          `json:"description"`
	InputSchema  json.RawMessage  `json:"inputSchema"`
	OutputSchema json.RawMessage  `json:"outputSchema"`
	Annotations  *ToolAnnotations `json:"annotations"`
	Icons        []Icon           `json:"icons"`
	Meta         map[string]any   `json:"_meta,omitempty"`
}

type ToolAnnotations struct {
	Title           string `json:"title"`
	DestructiveHint *bool  `json:"destructiveHint"`
	IdempotentHint  *bool  `json:"idempotentHint"`
	OpenWorldHint   *bool  `json:"openWorldHint"`
	ReadOnlyHint    *bool  `json:"readOnlyHint"`
}

type ToolListChangedNotificationParams struct {
	Meta map[string]any `json:"_meta,omitempty"`
}

type ToolsCapability struct {
	ListChanged *bool `json:"listChanged,omitempty"`
}
