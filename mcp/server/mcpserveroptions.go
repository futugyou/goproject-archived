package server

import (
	"time"

	"github.com/futugyou/mcp/core"
)

type McpServerOptions struct {
	ServerInfo            core.Implementation
	Capabilities          *core.ServerCapabilities
	ProtocolVersion       string        // "2024-11-05"
	InitializationTimeout time.Duration //  60 sec.
	ServerInstructions    string
	ScopeRequests         bool
	KnownClientInfo       *core.Implementation
}

func NewMcpServerOptions() *McpServerOptions {
	return &McpServerOptions{
		ServerInfo:            core.Implementation{},
		Capabilities:          &core.ServerCapabilities{},
		ProtocolVersion:       "2024-11-05",
		InitializationTimeout: time.Duration(60) * time.Second,
		ServerInstructions:    "",
		ScopeRequests:         true,
	}
}
