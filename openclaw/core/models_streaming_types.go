package core

type AgentStreamEventType uint8

const (
	AgentStreamEventTypeTextDelta AgentStreamEventType = iota
	AgentStreamEventTypeToolStart
	AgentStreamEventTypeToolDelta
	AgentStreamEventTypeToolResult
	AgentStreamEventTypeError
	AgentStreamEventTypeDone
)

var ToolResultStatuses = struct {
	Completed string
}{
	Completed: "Completed",
}

type AgentStreamEvent struct {
	Type           AgentStreamEventType
	Content        string
	ToolName       string
	ToolArguments  string
	ErrorCode      string
	ResultStatus   string
	FailureCode    string
	FailureMessage string
	NextStep       string
}

func NewTextDelta(text string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:    AgentStreamEventTypeTextDelta,
		Content: text,
	}
}

func NewToolStarted(toolName string, arguments string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:          AgentStreamEventTypeToolStart,
		Content:       toolName,
		ToolName:      toolName,
		ToolArguments: arguments,
	}
}

func NewToolDelta(toolName string, chunk string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:     AgentStreamEventTypeToolDelta,
		Content:  chunk,
		ToolName: toolName,
	}
}

func NewToolCompleted(
	toolName string,
	result string,
	resultStatus string,
	failureCode string,
	failureMessage string,
	nextStep string,
) AgentStreamEvent {
	status := ToolResultStatuses.Completed
	if resultStatus != "" {
		status = resultStatus
	}

	return AgentStreamEvent{
		Type:           AgentStreamEventTypeToolResult,
		Content:        result,
		ToolName:       toolName,
		ResultStatus:   status,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
		NextStep:       nextStep,
	}
}

func NewErrorOccurred(err string, errorCode string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:      AgentStreamEventTypeError,
		Content:   err,
		ErrorCode: errorCode,
	}
}

func NewComplete() AgentStreamEvent {
	return AgentStreamEvent{
		Type:    AgentStreamEventTypeDone,
		Content: "",
	}
}

func (e AgentStreamEvent) EnvelopeType() string {
	switch e.Type {
	case AgentStreamEventTypeTextDelta:
		return "assistant_chunk"
	case AgentStreamEventTypeToolStart:
		return "tool_start"
	case AgentStreamEventTypeToolDelta:
		return "tool_chunk"
	case AgentStreamEventTypeToolResult:
		return "tool_result"
	case AgentStreamEventTypeError:
		return "error"
	case AgentStreamEventTypeDone:
		return "assistant_done"
	default:
		return "assistant_chunk"
	}
}
