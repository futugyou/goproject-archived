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
