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

func CreateMetaStepFailedToolResult(
	toolName,
	arguments,
	failureCode,
	failureMessage string) *ToolExecutionResult {
	return &ToolExecutionResult{
		Invocation: core.ToolInvocation{
			ToolName:       toolName,
			Arguments:      arguments,
			Result:         failureMessage,
			ResultStatus:   "failed",
			FailureCode:    failureCode,
			FailureMessage: failureMessage,
		},
		ResultText:     failureMessage,
		ResultStatus:   "failed",
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
	}
}

func (t *ToolExecutionResult) ToFunctionResultContent(callId string) *contents.FunctionResultContent {
	return contents.NewFunctionResultContent(callId, t.ResultText)
}

type MetaParallelToolStepExecution struct {
	Step       core.MetaSkillStepDefinition
	ToolResult ToolExecutionResult
	DurationMs int64
}

type MetaParallelToolStepCandidate struct {
	Step         core.MetaSkillStepDefinition
	ToolName     string
	ToolArgsJson string
}

type MetaLlmStepExecutionResult struct {
	ExecutionResult *LlmExecutionResult
	FailureCode     string
	FailureMessage  string
}

func (m *MetaLlmStepExecutionResult) Completed() bool {
	return m.ExecutionResult != nil
}

func SucceededMetaLlmStepExecutionResult(executionResult LlmExecutionResult) *MetaLlmStepExecutionResult {
	return &MetaLlmStepExecutionResult{ExecutionResult: &executionResult}
}

func FaileddMetaLlmStepExecutionResult(failureCode, failureMessage string) *MetaLlmStepExecutionResult {
	return &MetaLlmStepExecutionResult{FailureCode: failureCode, FailureMessage: failureMessage}
}

type TurnRoutingSnapshot struct {
	ModelProfileId          string
	PreferredModelTags      []string
	FallbackModelProfileIds []string
	SystemPromptOverride    string
	RouteAllowedTools       []string
	RouteToolsDisabled      bool
	RouteModelTier          string
	RouteReason             string
	ReasoningEffort         string
	ResponseMode            string
}

type TurnRoutingRestoreScope struct {
	Session  core.Session
	Snapshot TurnRoutingSnapshot
}

type StreamCollectResult struct {
	TextDeltas       []string
	FullText         string
	ToolCalls        contents.FunctionCallContent
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ProviderId       string
	ModelId          string
	IsUsageEstimated bool
	Error            string
}
