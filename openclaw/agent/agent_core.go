package agent

import (
	"github.com/futugyou/extensions_ai/abstractions/contents"
	"github.com/futugyou/openclaw/core"
)

type ToolExecutionResult struct {
	Invocation     core.ToolInvocation
	ResultText     string
	ResultStatus   string
	FailureCode    string
	FailureMessage string
	NextStep       string
}

func (t *ToolExecutionResult) ToFunctionResultContent(callId string) *contents.FunctionResultContent {
	return contents.NewFunctionResultContent(callId, t.ResultText)
}
