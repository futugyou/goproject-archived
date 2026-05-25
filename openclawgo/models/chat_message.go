package models

import "encoding/json"

type ChatMessage struct {
	Role       ChatMessageRole
	Content    string
	Name       string
	ToolCallId string
	ToolCalls  []ToolCall
}

type ChatMessageRole string

const (
	ChatMessageRoleSystem    ChatMessageRole = "System"
	ChatMessageRoleUser      ChatMessageRole = "User"
	ChatMessageRoleAssistant ChatMessageRole = "Assistant"
	ChatMessageRoleTool      ChatMessageRole = "Tool"
)

func (c ChatMessageRole) Name() string {
	return (string)(c)
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  *json.RawMessage
}

type ToolCall struct {
	Id        string
	Name      string
	Arguments string
}

type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Tools       []ToolDefinition
	Temperature float32
	MaxTokens   int
}

type ChatResponse struct {
	Content      string
	Role         ChatMessageRole
	ToolCalls    []ToolCall
	Model        string
	Usage        UsageInfo
	FinishReason string
}

type ChatResponseChunk struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
}

type UsageInfo struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
