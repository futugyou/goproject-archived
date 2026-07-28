package protocol

type SamplingMessage struct {
	Content ContentBlock `json:"content"`
	Role    Role         `json:"role"`
}
