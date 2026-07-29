package client

import (
	"time"

	"github.com/futugyou/mcp/core"
)

type McpClientOptions struct {
	ClientInfo            *core.Implementation
	Capabilities          *core.ClientCapabilities
	ProtocolVersion       string
	InitializationTimeout time.Duration
}

func NewMcpClientOptions() *McpClientOptions {
	return &McpClientOptions{
		ClientInfo:            &core.Implementation{},
		Capabilities:          &core.ClientCapabilities{},
		ProtocolVersion:       "2024-11-05",
		InitializationTimeout: time.Duration(60) * time.Second,
	}
}
