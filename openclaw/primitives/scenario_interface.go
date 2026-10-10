package primitives

import (
	"context"
	"encoding/json"
	"time"
)

type ScenarioRisk string

const (
	ScenarioRiskLow      ScenarioRisk = "Low"
	ScenarioRiskMedium   ScenarioRisk = "Medium"
	ScenarioRiskHigh     ScenarioRisk = "High"
	ScenarioRiskCritical ScenarioRisk = "Critical"
)

type AgentScenario struct {
	Id          string            `json:"id"`
	Title       string            `json:"title"`
	Description *string           `json:"description,omitempty"`
	Risk        ScenarioRisk      `json:"risk"`
	Type        string            `json:"type"`
	Tags        []string          `json:"tags"`
	Input       ScenarioInput     `json:"input"`
	Expected    ScenarioExpected  `json:"expected"`
	Oracles     []ScenarioOracle  `json:"oracles"`
	Scripted    *ScriptedTrace    `json:"scriptedTrace,omitempty"`
	Metadata    map[string]string `json:"metadata"`
}

type ScenarioInput struct {
	UserMessage string            `json:"userMessage"`
	Context     []string          `json:"context"`
	Variables   map[string]string `json:"variables"`
}

type ScenarioExpected struct {
	MustCallTools             []string          `json:"mustCallTools"`
	MustNotCallTools          []string          `json:"mustNotCallTools"`
	FinalAnswerContains       []string          `json:"finalAnswerContains"`
	FinalAnswerMustNotContain []string          `json:"finalAnswerMustNotContain"`
	MaxToolCalls              *int              `json:"maxToolCalls,omitempty"`
	RequiresApproval          *bool             `json:"requiresApproval,omitempty"`
	ExpectedState             map[string]string `json:"expectedState"`
	ExpectedEvents            []string          `json:"expectedEvents"`
	ForbiddenEvents           []string          `json:"forbiddenEvents"`
}

type ScenarioOracle struct {
	Type     string            `json:"type"`
	Tool     *string           `json:"tool,omitempty"`
	Value    *json.RawMessage  `json:"value,omitempty"`
	Limit    *int              `json:"limit,omitempty"`
	Tools    []string          `json:"tools"`
	Metadata map[string]string `json:"metadata"`
}

type ScriptedTrace struct {
	FinalAnswer *string           `json:"finalAnswer,omitempty"`
	Status      string            `json:"status"`
	Steps       []TraceStep       `json:"steps"`
	Metadata    map[string]string `json:"metadata"`
}

type AgentRunTrace struct {
	RunId          string            `json:"runId"`
	ScenarioId     string            `json:"scenarioId"`
	StartedAtUtc   time.Time         `json:"startedAtUtc"`
	CompletedAtUtc time.Time         `json:"completedAtUtc"`
	Steps          []TraceStep       `json:"steps"`
	FinalAnswer    *string           `json:"finalAnswer,omitempty"`
	Status         string            `json:"status"`
	Metadata       map[string]string `json:"metadata"`
}

type TraceStep struct {
	Kind          string            `json:"kind"`
	TimestampUtc  time.Time         `json:"timestampUtc"`
	Message       *string           `json:"message,omitempty"`
	ToolName      *string           `json:"toolName,omitempty"`
	ArgumentsJson *string           `json:"argumentsJson,omitempty"`
	Result        *string           `json:"result,omitempty"`
	Error         *string           `json:"error,omitempty"`
	StatePath     *string           `json:"statePath,omitempty"`
	From          *string           `json:"from,omitempty"`
	To            *string           `json:"to,omitempty"`
	ApprovalId    *string           `json:"approvalId,omitempty"`
	Metadata      map[string]string `json:"metadata"`
}

type OracleResult struct {
	Passed   bool     `json:"passed"`
	Name     string   `json:"name"`
	Message  string   `json:"message"`
	Evidence []string `json:"evidence"`
}

type ScenarioRunResult struct {
	Scenario       AgentScenario  `json:"scenario"`
	Trace          AgentRunTrace  `json:"trace"`
	OracleResults  []OracleResult `json:"oracleResults"`
	Passed         bool           `json:"passed"`
	StartedAtUtc   time.Time      `json:"startedAtUtc"`
	CompletedAtUtc time.Time      `json:"completedAtUtc"`
	FailureSummary *string        `json:"failureSummary,omitempty"`
}

type ScenarioRunReport struct {
	RunId          string              `json:"runId"`
	StartedAtUtc   time.Time           `json:"startedAtUtc"`
	CompletedAtUtc time.Time           `json:"completedAtUtc"`
	Summary        ScenarioRunSummary  `json:"summary"`
	Results        []ScenarioRunResult `json:"results"`
	Markdown       *string             `json:"markdown,omitempty"`
}

type ScenarioRunSummary struct {
	Total                  int `json:"total"`
	Passed                 int `json:"passed"`
	Failed                 int `json:"failed"`
	HighOrCriticalFailures int `json:"highOrCriticalFailures"`
}

type TraceWriteResult struct {
	RunId         string   `json:"runId"`
	DirectoryPath string   `json:"directoryPath"`
	ResultsPath   string   `json:"resultsPath"`
	ReportPath    string   `json:"reportPath"`
	TracePaths    []string `json:"tracePaths"`
}

const (
	TraceStepKindsReasoning       = "reasoning"
	TraceStepKindsToolCall        = "toolCall"
	TraceStepKindsToolResult      = "toolResult"
	TraceStepKindsStateChange     = "stateChange"
	TraceStepKindsEvent           = "event"
	TraceStepKindsApprovalRequest = "approvalRequest"
	TraceStepKindsError           = "error"

	ScenarioRunStatusesUnknown   = "unknown"
	ScenarioRunStatusesCompleted = "completed"
	ScenarioRunStatusesFailed    = "failed"
)

type IScenarioRunner interface {
	Run(ctx context.Context, scenario AgentScenario) (*ScenarioRunResult, error)
}

type IScenarioOracle interface {
	GetName() string

	Evaluate(ctx context.Context,
		scenario AgentScenario,
		trace AgentRunTrace) (*OracleResult, error)
}

type IScenarioLoader interface {
	Load(ctx context.Context, directoryPath string) ([]AgentScenario, error)
}

type ITraceWriter interface {
	Write(ctx context.Context,
		report ScenarioRunReport,
		outputRoot string) (*TraceWriteResult, error)
}
