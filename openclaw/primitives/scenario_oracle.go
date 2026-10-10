package primitives

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type UnknownScenarioOracle struct {
	t string
}

func NewUnknownScenarioOracle(t string) *UnknownScenarioOracle {
	return &UnknownScenarioOracle{
		t: t,
	}
}

func (u *UnknownScenarioOracle) Name() string {
	return fmt.Sprintf("unknown:%s", u.t)
}

func (u *UnknownScenarioOracle) Evaluate(ctx context.Context, scenario AgentScenario, trace AgentRunTrace) (*OracleResult, error) {
	return &OracleResult{
		Name:    u.Name(),
		Message: fmt.Sprintf("Unknown oracle type '%s'", u.t),
	}, nil
}

type ToolCalledOracle struct {
	definition ScenarioOracleDefinition
}

func NewToolCalledOracle(definition ScenarioOracleDefinition) *ToolCalledOracle {
	return &ToolCalledOracle{definition: definition}
}

func (u *ToolCalledOracle) Name() string {
	return ScenarioOracleTypesToolCalled
}

func (u *ToolCalledOracle) Evaluate(ctx context.Context, scenario AgentScenario, trace AgentRunTrace) (*OracleResult, error) {
	requiredTools := scenario.Expected.MustCallTools
	tool := ScenarioOracleToolFromDefinition(u.definition)
	if tool != nil && *tool != "" {
		requiredTools = []string{*tool}
	}

	if len(requiredTools) == 0 {
		return ScenarioOracleFail(u.Name(), "No required tool was configured for the tool-called oracle."), nil
	}

	called := map[string]struct{}{}
	for _, v := range ScenarioOracleToolCalls(trace) {
		if v.ToolName != nil {
			if _, ok := called[*v.ToolName]; !ok {
				called[*v.ToolName] = struct{}{}
			}
		}
	}
	missing := []string{}
	for _, v := range requiredTools {
		if _, ok := called[v]; !ok {
			missing = append(missing, v)
		}
	}

	var calledEvidence = ScenarioOracleJoinStable(slices.Collect(maps.Keys(called)))
	if len(missing) == 0 {
		return ScenarioOraclePass(u.Name(), "All required tools were called.", fmt.Sprintf("called=%s", calledEvidence)), nil
	}

	return ScenarioOracleFail(u.Name(), fmt.Sprintf("Missing required tool calls: %s.", strings.Join(missing, ", ")), fmt.Sprintf("called=%s", calledEvidence)), nil
}
