package primitives

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	Id          string                     `json:"id"`
	Title       string                     `json:"title"`
	Description *string                    `json:"description,omitempty"`
	Risk        ScenarioRisk               `json:"risk"`
	Type        string                     `json:"type"`
	Tags        []string                   `json:"tags"`
	Input       ScenarioInput              `json:"input"`
	Expected    ScenarioExpected           `json:"expected"`
	Oracles     []ScenarioOracleDefinition `json:"oracles"`
	Scripted    *ScriptedTrace             `json:"scriptedTrace,omitempty"`
	Metadata    map[string]string          `json:"metadata"`
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

type ScenarioOracleDefinition struct {
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
	Markdown       string              `json:"markdown,omitempty"`
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

const (
	ScenarioOracleTypesToolCalled             = "tool-called"
	ScenarioOracleTypesToolNotCalled          = "tool-not-called"
	ScenarioOracleTypesMaxToolCalls           = "max-tool-calls"
	ScenarioOracleTypesFinalAnswerContains    = "final-answer-contains"
	ScenarioOracleTypesFinalAnswerNotContains = "final-answer-not-contains"
	ScenarioOracleTypesApprovalRequired       = "approval-required"
	ScenarioOracleTypesApprovalNotRequired    = "approval-not-required"
	ScenarioOracleTypesNoUnsafeTool           = "no-unsafe-tool"
)

var DefaultUnsafeTools = []string{
	"shell",
	"write_file",
	"code_exec",
	"git",
	"process",
	"home_assistant_write",
	"mqtt_publish",
	"notion_write",
}

type IScenarioRunner interface {
	Run(ctx context.Context, scenario AgentScenario) (*ScenarioRunResult, error)
}

type IScenarioLoader interface {
	Load(ctx context.Context, directoryPath string) ([]AgentScenario, error)
}

type ITraceWriter interface {
	Write(ctx context.Context,
		report ScenarioRunReport,
		outputRoot string) (*TraceWriteResult, error)
}

type IScenarioOracle interface {
	Name() string

	Evaluate(ctx context.Context,
		scenario AgentScenario,
		trace AgentRunTrace) (*OracleResult, error)
}

func ScenarioOraclePass(name, message string, evidence ...string) *OracleResult {
	ev := make([]string, len(evidence))
	copy(ev, evidence)
	return &OracleResult{
		Passed:   true,
		Name:     name,
		Message:  message,
		Evidence: ev,
	}
}

func ScenarioOracleFail(name, message string, evidence ...string) *OracleResult {
	ev := make([]string, len(evidence))
	copy(ev, evidence)
	return &OracleResult{
		Passed:   false,
		Name:     name,
		Message:  message,
		Evidence: ev,
	}
}

func ScenarioOracleIsKind(step TraceStep, kind string) bool {
	return strings.EqualFold(ScenarioOracleNormalizeKind(step.Kind), ScenarioOracleNormalizeKind(kind))
}

func ScenarioOracleToolCalls(trace AgentRunTrace) []TraceStep {
	var result []TraceStep
	for _, step := range trace.Steps {
		if ScenarioOracleIsKind(step, TraceStepKindsToolCall) {
			result = append(result, step)
		}
	}
	return result
}

func ScenarioOracleApprovalRequests(trace AgentRunTrace) []TraceStep {
	var result []TraceStep
	for _, step := range trace.Steps {
		if ScenarioOracleIsKind(step, TraceStepKindsApprovalRequest) {
			result = append(result, step)
		}
	}
	return result
}

func ScenarioOracleToolFromDefinition(definition ScenarioOracleDefinition) *string {
	valStr := ScenarioOracleValueAsString(definition.Value)
	metaVal := ScenarioOracleMetadataValue(definition, "tool")
	return ScenarioOracleFirstNonEmpty(definition.Tool, valStr, metaVal)
}

func ScenarioOracleValuesFromDefinitionOrExpected(definition ScenarioOracleDefinition, fallback []string) []string {
	valStr := ScenarioOracleValueAsString(definition.Value)
	metaVal := ScenarioOracleMetadataValue(definition, "value")
	value := ScenarioOracleFirstNonEmpty(valStr, metaVal)

	if value == nil || strings.TrimSpace(*value) == "" {
		var result []string
		for _, item := range fallback {
			if strings.TrimSpace(item) != "" {
				result = append(result, item)
			}
		}
		return result
	}
	return []string{*value}
}

func ScenarioOracleIntFromDefinitionOrExpected(definition ScenarioOracleDefinition, fallback *int) *int {
	if definition.Limit != nil {
		return definition.Limit
	}
	if v := ScenarioOracleValueAsInt(definition.Value); v != nil {
		return v
	}
	if v := ScenarioOracleIntFromMetadata(definition, "limit"); v != nil {
		return v
	}
	if v := ScenarioOracleIntFromMetadata(definition, "value"); v != nil {
		return v
	}
	return fallback
}

func ScenarioOracleMetadataValue(definition ScenarioOracleDefinition, key string) *string {
	if definition.Metadata == nil {
		return nil
	}
	if value, ok := definition.Metadata[key]; ok {
		return &value
	}
	return nil
}

func ScenarioOracleFirstNonEmpty(values ...*string) *string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			trimmed := strings.TrimSpace(*value)
			return &trimmed
		}
	}
	return nil
}

func ScenarioOracleJoinStable(values []string) string {
	seen := make(map[string]bool)
	var list []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			trimmed := strings.TrimSpace(value)
			lower := strings.ToLower(trimmed)
			if !seen[lower] {
				seen[lower] = true
				list = append(list, trimmed)
			}
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		return strings.ToLower(list[i]) < strings.ToLower(list[j])
	})
	return strings.Join(list, ",")
}

func ScenarioOracleNormalizeKind(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	r := strings.ReplaceAll(value, "-", "")
	r = strings.ReplaceAll(r, "_", "")
	return strings.TrimSpace(r)
}

func ScenarioOracleValueAsString(value any) *string {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		return &v
	case float64:
		s := fmt.Sprintf("%v", v)
		return &s
	case int:
		s := fmt.Sprintf("%d", v)
		return &s
	case bool:
		s := fmt.Sprintf("%t", v)
		return &s
	default:
		return nil
	}
}

func ScenarioOracleValueAsInt(value any) *int {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case int:
		return &v
	case float64:
		i := int(v)
		return &i
	case string:
		var parsed int
		_, err := fmt.Sscanf(v, "%d", &parsed)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func ScenarioOracleIntFromMetadata(definition ScenarioOracleDefinition, key string) *int {
	if definition.Metadata == nil {
		return nil
	}
	if val, ok := definition.Metadata[key]; ok {
		var parsed int
		_, err := fmt.Sscanf(val, "%d", &parsed)
		if err == nil {
			return &parsed
		}
	}
	return nil
}
