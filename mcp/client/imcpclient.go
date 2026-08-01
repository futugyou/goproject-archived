package client

import (
	"context"
	"net/url"

	"github.com/futugyou/mcp/core"
	"github.com/futugyou/mcp/shared"
)

type IMcpClient interface {
	shared.IMcpEndpoint
	GetServerCapabilities() *core.ServerCapabilities
	GetServerInfo() *core.Implementation
	GetServerInstructions() *string
	Ping(ctx context.Context) error
	ListTools(ctx context.Context) ([]McpClientTool, error)
	CallTool(ctx context.Context, toolName string, arguments map[string]any, reporter any) (*core.CallToolResult, error)
	GetPrompt(ctx context.Context, name string, arguments map[string]any) (*core.GetPromptResult, error)
	EnumerateTools(ctx context.Context) (<-chan McpClientTool, <-chan error)
	ListPrompts(ctx context.Context, client IMcpClient) ([]McpClientPrompt, error)
	EnumeratePrompts(ctx context.Context, client IMcpClient) (<-chan McpClientPrompt, <-chan error)
	ListResourceTemplates(ctx context.Context, client IMcpClient) ([]McpClientResourceTemplate, error)
	EnumerateResourceTemplates(ctx context.Context, client IMcpClient) (<-chan McpClientResourceTemplate, <-chan error)
	ListResources(ctx context.Context, client IMcpClient) ([]McpClientResource, error)
	EnumerateResources(ctx context.Context, client IMcpClient) (<-chan McpClientResource, <-chan error)
	ReadResource(ctx context.Context, uri string) (*core.ReadResourceResult, error)
	ReadResourceWithUri(ctx context.Context, uri url.URL) (*core.ReadResourceResult, error)
	ReadResourceWithUriAndArguments(ctx context.Context, uriTemplate string, arguments map[string]any) (*core.ReadResourceResult, error)
	Complete(ctx context.Context, reference core.Reference, argumentName string, argumentValue string) (*core.CompleteResult, error)
	SubscribeToResource(ctx context.Context, uri string) error
	SubscribeToResourceWithUri(ctx context.Context, uri url.URL) error
	UnsubscribeFromResource(ctx context.Context, uri string) error
	UnsubscribeFromResourceWithUri(ctx context.Context, uri url.URL) error
	// SetLoggingLevel(ctx context.Context, level core.LoggingLevel) error
	// SetLoggingLevelWithLogLevel(ctx context.Context, level logger.LogLevel) error
}
