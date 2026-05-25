package models

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
	Parameters  any
}

type ToolCall struct {
	Id        string
	Name      string
	Arguments string
}
