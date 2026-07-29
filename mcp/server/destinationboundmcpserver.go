package server

import (
	"context"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/mcp/core"
)

var _ IMcpServer = (*DestinationBoundMcpServer)(nil)

type DestinationBoundMcpServer struct {
	server    *McpServer
	transport core.ITransport
}

func NewDestinationBoundMcpServer(server *McpServer, transport core.ITransport) *DestinationBoundMcpServer {
	return &DestinationBoundMcpServer{
		server:    server,
		transport: transport,
	}
}

// AsSamplingChatClient implements IMcpServer.
func (d *DestinationBoundMcpServer) AsSamplingChatClient() (chatcompletion.IChatClient, error) {
	// if d.GetClientCapabilities() == nil || d.GetClientCapabilities().Sampling == nil {
	// 	return nil, fmt.Errorf("client capabilities sampling not set")
	// }
	return NewSamplingChatClient(d), nil
}

// Dispose implements IMcpServer.
func (d *DestinationBoundMcpServer) Dispose(ctx context.Context) error {
	return d.server.Dispose(ctx)
}

// GetClientCapabilities implements IMcpServer.
func (d *DestinationBoundMcpServer) GetClientCapabilities() *core.ClientCapabilities {
	return d.server.GetClientCapabilities()
}

// GetClientInfo implements IMcpServer.
func (d *DestinationBoundMcpServer) GetClientInfo() *core.Implementation {
	return d.server.GetClientInfo()
}

// GetEndpointName implements IMcpServer.
func (d *DestinationBoundMcpServer) GetEndpointName() string {
	return d.server.EndpointName
}

// GetMcpServerOptions implements IMcpServer.
func (d *DestinationBoundMcpServer) GetMcpServerOptions() *McpServerOptions {
	return d.server.GetMcpServerOptions()
}

// GetMessageProcessingTask implements IMcpServer.
func (d *DestinationBoundMcpServer) GetMessageProcessingTask() <-chan struct{} {
	return d.server.GetMessageProcessingTask()
}

// NotifyProgress implements IMcpServer.
func (d *DestinationBoundMcpServer) NotifyProgress(ctx context.Context, progressToken core.ProgressToken, progress core.ProgressNotificationValue) error {
	return d.server.NotifyProgress(ctx, progressToken, progress)
}

// RegisterNotificationHandler implements IMcpServer.
// func (d *DestinationBoundMcpServer) RegisterNotificationHandler(method string, handler core.NotificationHandler) *shared.RegistrationHandle {
// 	return d.server.RegisterNotificationHandler(method, handler)
// }

// RequestRoots implements IMcpServer.
// func (d *DestinationBoundMcpServer) RequestRoots(ctx context.Context, request core.ListRootsRequestParams) (*core.ListRootsResult, error) {
// 	return d.server.RequestRoots(ctx, request)
// }

// Sample implements IMcpServer.
// func (d *DestinationBoundMcpServer) Sample(ctx context.Context, request core.CreateMessageRequestParams) (*core.CreateMessageResult, error) {
// 	return d.server.Sample(ctx, request)
// }

// SampleWithChatMessage implements IMcpServer.
func (d *DestinationBoundMcpServer) SampleWithChatMessage(ctx context.Context, messages []chatcompletion.ChatMessage, options *chatcompletion.ChatOptions) (*chatcompletion.ChatResponse, error) {
	return d.server.SampleWithChatMessage(ctx, messages, options)
}

// Run implements IMcpServer.
func (d *DestinationBoundMcpServer) Run(ctx context.Context) error {
	return d.server.Run(ctx)
}

// SendMessage implements IMcpServer.
func (d *DestinationBoundMcpServer) SendMessage(ctx context.Context, msg core.IJsonRpcMessage) error {
	// msg.SetRelatedTransport(d.transport)
	return d.server.SendMessage(ctx, msg)
}

// SendNotification implements IMcpServer.
func (d *DestinationBoundMcpServer) SendNotification(ctx context.Context, notification core.JsonRpcNotification) error {
	return d.server.SendNotification(ctx, notification)
}

// SendRequest implements IMcpServer.
func (d *DestinationBoundMcpServer) SendRequest(ctx context.Context, req *core.JsonRpcRequest) (*core.JsonRpcResponse, error) {
	// req.SetRelatedTransport(d.transport)
	return d.server.SendRequest(ctx, req)
}

func (e *DestinationBoundMcpServer) Elicit(ctx context.Context, request core.ElicitRequestParams) (*core.ElicitResult, error) {
	return e.server.Elicit(ctx, request)
}
