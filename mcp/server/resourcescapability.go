package server

import (
	"context"

	"github.com/futugyou/mcp/protocol"
)

// LoggingCapability represents the logging capability configuration.
type LoggingCapability struct {
	SetLoggingLevelHandler func(ctx context.Context, req RequestContext[*protocol.SetLevelRequestParams]) (*protocol.EmptyResult, error) `json:"-"`
}
