package server

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type IMcpServerPrompt interface {
	IMcpServerPrimitive
	GetProtocolPrompt() *core.Prompt
	Get(ctx context.Context, request RequestContext[*core.GetPromptRequestParams]) (*core.GetPromptResult, error)
}
