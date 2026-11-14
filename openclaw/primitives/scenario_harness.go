package primitives

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/futugyou/openclaw/util"
)

var _ IScenarioRunner = (*ScriptedScenarioRunner)(nil)

type ScriptedScenarioRunner struct {
	oracleRegistry *ScenarioOracleRegistry
}

func NewScriptedScenarioRunner(oracleRegistry *ScenarioOracleRegistry) *ScriptedScenarioRunner {
	if oracleRegistry == nil {
		oracleRegistry = NewScenarioOracleRegistry(nil)
	}
	return &ScriptedScenarioRunner{oracleRegistry: oracleRegistry}
}

func (s *ScriptedScenarioRunner) Run(ctx context.Context, scenario AgentScenario) (*ScenarioRunResult, error) {
	startedAt := time.Now()
	trace := scriptedScenarioRunnerBuildTrace(scenario, startedAt)
	oracleResults := []OracleResult{}

	if scenario.ScriptedTrace == nil {
		oracleResults = append(oracleResults, OracleResult{
			Name:    "scripted-trace-present",
			Message: "ScriptedScenarioRunner requires scriptedTrace evidence for deterministic scenario execution.",
		})
	}

	if len(scenario.Oracles) == 0 {
		oracleResults = append(oracleResults, OracleResult{
			Name:    "oracles-present",
			Message: "Scenario declared no oracles. A scenario that only executes is not a meaningful agent test.",
		})
	} else {
		for _, definition := range scenario.Oracles {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			var oracle = s.oracleRegistry.Create(definition)
			result, err := oracle.Evaluate(ctx, scenario, trace)
			if err != nil {
				return nil, err
			}

			oracleResults = append(oracleResults, *result)
		}
	}

	completedAt := time.Now()
	failed := []string{}
	for _, result := range oracleResults {
		if !result.Passed {
			failed = append(failed, result.Message)
		}
	}

	result := &ScenarioRunResult{
		Scenario:       scenario,
		Trace:          trace,
		OracleResults:  oracleResults,
		Passed:         len(failed) == 0,
		StartedAtUtc:   startedAt,
		CompletedAtUtc: completedAt,
	}
	if len(failed) > 0 {
		result.FailureSummary = new(strings.Join(failed, " "))
	}
	return result, nil
}

func scriptedScenarioRunnerBuildTrace(scenario AgentScenario, startedAt time.Time) AgentRunTrace {
	runId := fmt.Sprintf("scenario_%s", util.CleanUUID())

	script := scenario.ScriptedTrace
	steps := []TraceStep{}
	offset := 0

	if script != nil {
		for _, step := range script.Steps {
			timestamp := step.TimestampUtc
			if timestamp.IsZero() {
				offset++
				timestamp = startedAt.Add(time.Duration(offset*240) * time.Hour)
			}

			metadata := map[string]string{}
			maps.Copy(metadata, step.Metadata)
			steps = append(steps, TraceStep{
				Kind:          step.Kind,
				TimestampUtc:  timestamp,
				Message:       step.Message,
				ToolName:      step.ToolName,
				ArgumentsJson: step.ArgumentsJson,
				Result:        step.Result,
				Error:         step.Error,
				StatePath:     step.StatePath,
				From:          step.From,
				To:            step.To,
				ApprovalId:    step.ApprovalId,
				Metadata:      metadata,
			})

		}
	} else {
		steps = append(steps, TraceStep{
			Kind:         TraceStepKindsEvent,
			TimestampUtc: startedAt,
			Message:      new("No scripted trace was provided."),
		})

	}

	trace := AgentRunTrace{
		RunId:          runId,
		ScenarioId:     scenario.Id,
		StartedAtUtc:   startedAt,
		CompletedAtUtc: time.Now(),
		Steps:          steps,
		Metadata:       map[string]string{},
	}
	if script != nil {
		metadata := map[string]string{}
		maps.Copy(metadata, script.Metadata)
		trace.FinalAnswer = script.FinalAnswer
		trace.Metadata = metadata
	}
	return trace
}
