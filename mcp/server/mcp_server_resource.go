package server

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type IMcpServerResource interface {
	IMcpServerPrimitive
	GetProtocolResourceTemplate() core.ResourceTemplate
	GetProtocolResource() *core.Resource
	Read(ctx context.Context, request RequestContext[*core.ReadResourceRequestParams]) (*core.ReadResourceResult, error)
}
