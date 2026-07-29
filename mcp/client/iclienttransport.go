package client

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type IClientTransport interface {
	GetName() string
	Connect(context.Context) (core.ITransport, error)
}
