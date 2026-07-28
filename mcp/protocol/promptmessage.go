package protocol

type PromptMessage struct {
	Content ContentBlock `json:"content"`
	Role    Role         `json:"role"`
}
