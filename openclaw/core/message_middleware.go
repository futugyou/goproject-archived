package core

import "context"

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

type MiddlewarePipeline struct {
	middleware []IMessageMiddleware
}

func NewMiddlewarePipeline(middleware []IMessageMiddleware) *MiddlewarePipeline {
	return &MiddlewarePipeline{middleware: middleware}
}

func (m *MiddlewarePipeline) Execute(ctx context.Context, messageContext *MessageContext) bool {
	if len(m.middleware) == 0 {
		return true
	}

	var index = 0

	var next func(ctx context.Context) error

	next = func(ctx context.Context) error {
		if messageContext.IsShortCircuited {
			return nil
		}
		if index < len(m.middleware) {
			var mw = m.middleware[index]
			index++
			return mw.Invoke(ctx, messageContext, next)
		}

		return nil
	}

	if err := next(ctx); err != nil {
		return false
	}

	return !messageContext.IsShortCircuited
}
