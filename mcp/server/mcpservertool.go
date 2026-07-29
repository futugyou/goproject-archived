package server

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type IMcpServerTool interface {
	IMcpServerPrimitive
	GetProtocolTool() *core.Tool
	Invoke(ctx context.Context, request RequestContext[*core.CallToolRequestParams]) (*core.CallToolResult, error)
}
