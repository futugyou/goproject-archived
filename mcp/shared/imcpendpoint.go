package shared

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type IMcpEndpoint interface {
	SendRequest(ctx context.Context, req *core.JsonRpcRequest) (*core.JsonRpcResponse, error)
	SendMessage(ctx context.Context, msg core.IJsonRpcMessage) error
	// RegisterNotificationHandler(method string, handler core.NotificationHandler) *RegistrationHandle
	GetEndpointName() string
	GetMessageProcessingTask() <-chan struct{}
	Dispose(ctx context.Context) error
	SendNotification(ctx context.Context, notification core.JsonRpcNotification) error
	NotifyProgress(ctx context.Context, progressToken core.ProgressToken, progress core.ProgressNotificationValue) error
}

type Disposable interface {
	Dispose() error
}
