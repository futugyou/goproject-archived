package protocol

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
