package primitives

import (
	"context"
	"fmt"
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
