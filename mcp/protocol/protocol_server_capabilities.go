package protocol

type ServerCapabilities struct {
	Experimental map[string]any         `json:"experimental,omitempty"`
	Prompts      *PromptsCapability     `json:"prompts,omitempty"`
	Resources    *ResourcesCapability   `json:"resources,omitempty"`
	Tools        *ToolsCapability       `json:"tools,omitempty"`
	Completions  *CompletionsCapability `json:"completions,omitempty"`
	Extensions   map[string]any         `json:"extensions,omitempty"`
}
