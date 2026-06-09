package core

type AgentStreamEventType uint8

const (
	TextDelta AgentStreamEventType = iota
	ToolStart
	ToolDelta
	ToolResult
	Error
	Done
)

var ToolResultStatuses = struct {
	Completed string
}{
	Completed: "Completed",
}

type AgentStreamEvent struct {
	Type           AgentStreamEventType
	Content        string
	ToolName       *string
	ToolArguments  *string
	ErrorCode      *string
	ResultStatus   *string
	FailureCode    *string
	FailureMessage *string
	NextStep       *string
}

func NewTextDelta(text string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:    TextDelta,
		Content: text,
	}
}

func NewToolStarted(toolName string, arguments *string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:          ToolStart,
		Content:       toolName,
		ToolName:      &toolName,
		ToolArguments: arguments,
	}
}

func NewToolDelta(toolName string, chunk string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:     ToolDelta,
		Content:  chunk,
		ToolName: &toolName,
	}
}

func NewToolCompleted(
	toolName string,
	result string,
	resultStatus *string,
	failureCode *string,
	failureMessage *string,
	nextStep *string,
) AgentStreamEvent {
	status := ToolResultStatuses.Completed
	if resultStatus != nil {
		status = *resultStatus
	}

	return AgentStreamEvent{
		Type:           ToolResult,
		Content:        result,
		ToolName:       &toolName,
		ResultStatus:   &status,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
		NextStep:       nextStep,
	}
}

func NewErrorOccurred(err string, errorCode *string) AgentStreamEvent {
	return AgentStreamEvent{
		Type:      Error,
		Content:   err,
		ErrorCode: errorCode,
	}
}

func NewComplete() AgentStreamEvent {
	return AgentStreamEvent{
		Type:    Done,
		Content: "",
	}
}

func (e AgentStreamEvent) EnvelopeType() string {
	switch e.Type {
	case TextDelta:
		return "assistant_chunk"
	case ToolStart:
		return "tool_start"
	case ToolDelta:
		return "tool_chunk"
	case ToolResult:
		return "tool_result"
	case Error:
		return "error"
	case Done:
		return "assistant_done"
	default:
		return "assistant_chunk"
	}
}
