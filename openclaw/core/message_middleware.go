package core

type MessageContext struct {
	ChannelId string
	SenderId  string
	Text      string
	MessageId string
	SessionId string

	/// <summary>Session-level token counters (input + output accumulated across turns).</summary>
	SessionInputTokens  int64
	SessionOutputTokens int64

	Properties map[string]any

	/// <summary>When set to true by a middleware, the message is dropped and the response text is returned directly.</summary>
	IsShortCircuited     bool
	ShortCircuitResponse string
}

func (m *MessageContext) ShortCircuit(responseText string) {
	m.IsShortCircuited = true
	m.ShortCircuitResponse = responseText
}
