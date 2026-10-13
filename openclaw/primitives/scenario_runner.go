package primitives

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/openclaw/agent"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type RuntimeScenarioFactory func(AgentScenario) agent.IAgentRuntime
type RuntimeScenarioRunner struct {
	runtimeFactory RuntimeScenarioFactory
	approval       agent.ToolApprovalCallback
	oracles        *ScenarioOracleRegistry

	mu sync.Mutex
}

func NewRuntimeScenarioRunner(runtimeFactory RuntimeScenarioFactory,
	approval agent.ToolApprovalCallback,
	oracles *ScenarioOracleRegistry) *RuntimeScenarioRunner {
	return &RuntimeScenarioRunner{
		runtimeFactory: runtimeFactory,
		approval:       approval,
		oracles:        oracles,
	}
}

func (r *RuntimeScenarioRunner) Run(ctx context.Context, scenario AgentScenario) (*ScenarioRunResult, error) {
	started := time.Now()
	runId := "runtime_" + util.CleanUUID()
	session := &core.Session{Id: runId, ChannelId: "test", SenderId: "scenario-runner"}
	for _, v := range scenario.Input.Context {
		session.History = append(session.History, core.ChatTurn{Role: "user", Content: v})
	}

	steps := []TraceStep{}
	answer := &strings.Builder{}
	failed := false
	completed := false
	approve := func(ctx context.Context, tool, args string) bool {
		r.mu.Lock()
		defer r.mu.Unlock()

		steps = append(steps, TraceStep{Kind: TraceStepKindsApprovalRequest, TimestampUtc: time.Now(), ToolName: new(tool), ArgumentsJson: new(args)})

		if r.approval == nil {
			return false
		}
		return r.approval(ctx, tool, args)
	}

	runtime := r.runtimeFactory(scenario)
	events, err := runtime.RunStreaming(ctx, session, scenario.Input.UserMessage, approve, "")
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	for item := range events {
		if item.Type == core.AgentStreamEventTypeTextDelta {
			answer.WriteString(item.Content)
		}
		if item.Type == core.AgentStreamEventTypeDone {
			completed = true
		}
		if item.Type == core.AgentStreamEventTypeError {
			failed = true
		}

		kind := ""
		switch item.Type {
		case core.AgentStreamEventTypeToolStart:
			kind = TraceStepKindsToolCall
		case core.AgentStreamEventTypeToolResult:
			kind = TraceStepKindsToolResult
		case core.AgentStreamEventTypeError:
			kind = TraceStepKindsError
		}

		step := TraceStep{Kind: kind, TimestampUtc: time.Now(), ToolName: new(item.ToolName), ArgumentsJson: new(item.ToolArguments)}

		if item.Type == core.AgentStreamEventTypeToolResult {
			step.Result = new(item.Content)
		}
		if item.Type == core.AgentStreamEventTypeError {
			step.Result = new(item.Content)
		}

		steps = append(steps, step)
	}
	if err != nil {
		step := TraceStep{Kind: TraceStepKindsError, TimestampUtc: time.Now(), Error: new(err.Error())}
		steps = append(steps, step)
	}

	var capturedSteps []TraceStep
	r.mu.Lock()
	capturedSteps = steps
	r.mu.Unlock()

	ended := time.Now()
	trace := AgentRunTrace{RunId: runId, ScenarioId: scenario.Id, StartedAtUtc: started, CompletedAtUtc: ended, Steps: capturedSteps, FinalAnswer: new(answer.String()), Status: ScenarioRunStatusesFailed}
	completedmsg := "Runtime failed or did not emit completion."
	if !failed && completed {
		trace.Status = ScenarioRunStatusesCompleted
		completedmsg = "Runtime completed."
	}
	results := []OracleResult{
		{Name: "runtime-completed", Passed: !failed && completed, Message: completedmsg},
		{Name: "oracles-present", Passed: len(scenario.Oracles) > 0, Message: "Runtime scenarios must declare outcome assertions."},
	}
	registry := r.oracles
	if registry == nil {
		registry = NewScenarioOracleRegistry(nil)
	}
	for _, definition := range scenario.Oracles {
		result, err := registry.Create(definition).Evaluate(ctx, scenario, trace)
		if err != nil {
			results = append(results, *result)
		}
	}

	runResult := &ScenarioRunResult{Scenario: scenario, Trace: trace, OracleResults: results, StartedAtUtc: started, CompletedAtUtc: ended, Passed: true}
	summary := []string{}
	for _, v := range results {
		if !v.Passed {
			runResult.Passed = false
			summary = append(summary, v.Message)
		}
	}
	runResult.FailureSummary = new(strings.Join(summary, "; "))
	return runResult, nil
}

func RuntimeScenarioRunReplay(ctx context.Context, fixture TrajectoryReplayFixture, assertions []ScenarioOracleDefinition,
	createRuntime func(chatcompletion.IChatClient, []core.ITool) agent.IAgentRuntime,
	approval agent.ToolApprovalCallback) (*ScenarioRunResult, error) {
	replay, err := NewTrajectoryReplay(fixture)
	if err != nil {
		return nil, err
	}
	scenario := AgentScenario{
		Id: "trajectory-replay", Input: ScenarioInput{UserMessage: fixture.Prompt}, Oracles: assertions,
	}
	runner := NewRuntimeScenarioRunner(func(AgentScenario) agent.IAgentRuntime {
		return createRuntime(replay, replay.Tools)
	}, approval, nil)
	result, err := runner.Run(ctx, scenario)
	if err != nil {
		return nil, err
	}
	divergence := ""
	if err := replay.VerifyComplete(); err != nil {
		divergence = err.Error()
	}

	message := "All recorded responses and tools were consumed."
	if divergence != "" {
		message = divergence
	}
	oracleResult := OracleResult{
		Name:    "replay-consumed",
		Passed:  divergence == "",
		Message: message,
	}

	outcomes := append(result.OracleResults, oracleResult)
	passed := true
	summary := []string{}
	for _, v := range outcomes {
		if !v.Passed {
			passed = false
			summary = append(summary, v.Message)
		}
	}

	return &ScenarioRunResult{
		Scenario: result.Scenario, Trace: result.Trace, OracleResults: outcomes,
		Passed: passed, StartedAtUtc: result.StartedAtUtc, CompletedAtUtc: result.CompletedAtUtc,
		FailureSummary: new(strings.Join(summary, "; ")),
	}, nil
}
