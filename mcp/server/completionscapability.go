package server

import (
	"context"

	"github.com/futugyou/mcp/protocol"
)

type CompletionsCapability struct {
	CompleteHandler func(context.Context, RequestContext[*protocol.CompleteRequestParams]) (*protocol.CompleteResult, error)
}
