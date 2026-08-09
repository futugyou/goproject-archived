package agent

import (
	"context"

	"github.com/futugyou/openclaw/core"
)

type TurnRoutingRequest struct {
	Session     core.Session
	UserMessage string
	Messages    []string
}

type TurnRoutingDecision struct {
	Tier                                string
	ModelProfileId                      string
	DirectModelFallbackProfileId        string
	DisableTools                        bool
	AllowedTools                        []string
	PreferredTags                       []string
	ReasoningLevel                      string
	ResponsePolicy                      string
	ImageCapableModelProfileId          string
	CacheContinuitySafeguardsEnabled    bool
	CacheContinuityMaxConversationTurns int
	CacheContinuityResetOnProfileSwitch bool
	SystemPromptSuffix                  string
	Reason                              string
}

type ITurnRoutingPolicy interface {
	Resolve(ctx context.Context, request TurnRoutingRequest) (*TurnRoutingDecision, error)
}

type NoopTurnRoutingPolicy struct{}

func (n *NoopTurnRoutingPolicy) Resolve(ctx context.Context, request TurnRoutingRequest) (*TurnRoutingDecision, error) {
	return &TurnRoutingDecision{}, nil
}
