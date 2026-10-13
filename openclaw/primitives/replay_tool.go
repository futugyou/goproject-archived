package primitives

import (
	"context"

	"github.com/futugyou/openclaw/core"
)

var _ core.ITool = (*replayTool)(nil)

type replayTool struct {
	owner *TrajectoryReplay
	name  string
}

func NewReplayTool(owner *TrajectoryReplay, name string) *replayTool {
	return &replayTool{
		owner: owner,
		name:  name,
	}
}

func (r *replayTool) Description() string {
	return "Offline replay stub; returns a recorded result without performing external actions."
}

func (r *replayTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	return r.owner.execute(ctx, r.name, argumentsJson)
}

func (r *replayTool) Name() string {
	return r.name
}

func (r *replayTool) ParameterSchema() string {
	return `{"type":"object","additionalProperties":true}`
}

type PendingCall struct {
	Id       string
	Call     ReplayToolCall
	Consumed bool
}

func NewPendingCall(id string, call ReplayToolCall) *PendingCall {
	return &PendingCall{Id: id, Call: call}
}
