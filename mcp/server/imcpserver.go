package server

import (
	"context"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/mcp/core"
	"github.com/futugyou/mcp/shared"
)

type IMcpServer interface {
	shared.IMcpEndpoint
	GetClientCapabilities() *core.ClientCapabilities
	GetClientInfo() *core.Implementation
	GetMcpServerOptions() *McpServerOptions
	Run(ctx context.Context) error
	// Sample(ctx context.Context, request core.CreateMessageRequestParams) (*core.CreateMessageResult, error)
	Elicit(ctx context.Context, request core.ElicitRequestParams) (*core.ElicitResult, error)
	SampleWithChatMessage(ctx context.Context, messages []chatcompletion.ChatMessage, options *chatcompletion.ChatOptions) (*chatcompletion.ChatResponse, error)
	AsSamplingChatClient() (chatcompletion.IChatClient, error)
	// RequestRoots(ctx context.Context, request core.ListRootsRequestParams) (*core.ListRootsResult, error)
}
