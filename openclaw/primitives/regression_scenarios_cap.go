package primitives

import (
	"context"
	"encoding/json"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type CapabilityBindingTrajectorySerializationScenario struct {
	BaseScenario
}

func NewCapabilityBindingTrajectorySerializationScenario() *CapabilityBindingTrajectorySerializationScenario {
	s := &CapabilityBindingTrajectorySerializationScenario{
		BaseScenario: NewBaseScenario(
			"harness.capability_binding_trajectory_serialization",
			"Capability Binding Trajectory serialization",
			HarnessRegressionCategoryHarness,
			true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *CapabilityBindingTrajectorySerializationScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	run := core.SessionMetaRunRecord{
		RunId:     "meta_binding_regression",
		SkillName: "meta-capability",
		Status:    "completed",
		StepResults: []core.SessionMetaStepResult{
			{
				Id:     "query",
				Kind:   "tool_call",
				Status: "completed",
				ExecutionEvidence: &core.SessionMetaStepExecutionEvidence{
					CapabilityBinding: &core.CapabilityBindingTrajectory{
						Binding:         "dynamic",
						IntentKey:       new("a1b2c3"),
						TaskDescription: new("weather city"),
						KeyWords:        new("weather,city"),
						SelectionPolicy: "first",
						Server:          new("weather-mcp"),
						Tool:            new("get_weather"),
						ElapsedMs:       12.5,
						Candidates: []core.CapabilityBindingCandidate{
							{Name: "weather-mcp", Rank: 1},
							{Name: "amap-mcp-server", Rank: 2},
						},
						Attempted: []core.CapabilityBindingCandidate{
							{Name: "weather-mcp", Rank: 1},
						},
					},
				},
			},
		},
	}

	jsondata, err := json.Marshal(run)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	var restored core.SessionMetaRunRecord
	if err := json.Unmarshal(jsondata, &restored); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	var binding *core.CapabilityBindingTrajectory
	if len(restored.StepResults) > 0 && restored.StepResults[0].ExecutionEvidence != nil {
		binding = restored.StepResults[0].ExecutionEvidence.CapabilityBinding
	}

	if binding != nil &&
		binding.Binding == "dynamic" &&
		util.Deref(binding.IntentKey) == "a1b2c3" &&
		!binding.CacheHit &&
		util.Deref(binding.Server) == "weather-mcp" &&
		util.Deref(binding.Tool) == "get_weather" &&
		len(binding.Candidates) == 2 &&
		binding.Candidates[0].Name == "weather-mcp" && binding.Candidates[0].Rank == 1 &&
		binding.Candidates[1].Name == "amap-mcp-server" && binding.Candidates[1].Rank == 2 &&
		len(binding.Attempted) == 1 {
		return s.Passed("Capability binding trajectory round-tripped through source-generated JSON.", ""), nil
	}
	return s.Failed("Capability binding trajectory did not round-trip correctly.", "", ""), nil
}
