package shared

import (
	"context"
	"errors"
	"sync"

	"github.com/futugyou/mcp/core"
)

var _ IMcpEndpoint = (*BaseMcpEndpoint)(nil)

type BaseMcpEndpoint struct {
	mu            sync.Mutex
	disposed      bool
	session       *core.McpSessionHandler
	sessionCts    context.CancelFunc
	messageTask   <-chan struct{}
	reqHandlers   *core.RequestHandlers
	notifHandlers *core.NotificationHandlers
	endpointName  string
}

func (e *BaseMcpEndpoint) GetMcpSession() *core.McpSessionHandler {
	return e.session
}

// NotifyProgress implements IMcpEndpoint.
func (e *BaseMcpEndpoint) NotifyProgress(ctx context.Context, progressToken core.ProgressToken, progress core.ProgressNotificationValue) error {
	// p := core.ProgressNotification{ProgressToken: &progressToken, Progress: &progress}
	// data, err := json.Marshal(p)
	// if err != nil {
	// 	return err
	// }
	// notification := core.NewJsonRpcNotification(core.NotificationMethods_ProgressNotification, data)
	// return e.SendNotification(ctx, *notification)
	return nil
}

// SendNotification implements IMcpEndpoint.
func (e *BaseMcpEndpoint) SendNotification(ctx context.Context, notification core.JsonRpcNotification) error {
	return e.SendMessage(ctx, &notification)
}

func NewBaseMcpEndpoint() *BaseMcpEndpoint {
	return &BaseMcpEndpoint{
		reqHandlers:   core.NewRequestHandlers(),
		notifHandlers: core.NewNotificationHandlers(),
		endpointName:  "",
	}
}

func (e *BaseMcpEndpoint) GetEndpointName() string {
	return e.endpointName
}

func (e *BaseMcpEndpoint) GetMessageProcessingTask() <-chan struct{} {
	return e.messageTask
}

func (e *BaseMcpEndpoint) GetRequestHandlers() *core.RequestHandlers {
	return e.reqHandlers
}

func (e *BaseMcpEndpoint) GetNotificationHandlers() *core.NotificationHandlers {
	return e.notifHandlers
}

func (e *BaseMcpEndpoint) InitializeSession(transport core.ITransport, isServer bool) {
	// e.session = core.NewMcpSession(isServer, transport, e.endpointName, e.reqHandlers, e.notifHandlers)
}

func (e *BaseMcpEndpoint) StartSession(ctx context.Context, transport core.ITransport) {
	_, cancel := context.WithCancel(ctx)
	e.sessionCts = cancel

	done := make(chan struct{})
	e.messageTask = done

	go func() {
		defer close(done)
		// e.session.ProcessMessages(childCtx)
	}()
}

func (e *BaseMcpEndpoint) CancelSession() {
	if e != nil && e.sessionCts != nil {
		e.sessionCts()
	}
}

func (e *BaseMcpEndpoint) SendRequest(ctx context.Context, req *core.JsonRpcRequest) (*core.JsonRpcResponse, error) {
	if e == nil || e.session == nil {
		return nil, errors.New("session not initialized")
	}
	// return e.session.SendRequest(ctx, req)
	return nil, nil
}

func (e *BaseMcpEndpoint) SendMessage(ctx context.Context, msg core.IJsonRpcMessage) error {
	// if e == nil || e.session == nil {
	// 	return errors.New("session not initialized")
	// }
	// return e.session.SendMessage(ctx, msg)
	return nil
}

// func (e *BaseMcpEndpoint) RegisterNotificationHandler(method string, handler core.NotificationHandler) *RegistrationHandle {
// 	if e.session == nil {
// 		return nil
// 	}
// 	return e.session.RegisterNotificationHandler(method, handler)
// }

func (e *BaseMcpEndpoint) Dispose(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.disposed {
		return nil
	}
	e.disposed = true
	return e.disposeUnsynchronized(ctx)
}

func (e *BaseMcpEndpoint) disposeUnsynchronized(ctx context.Context) error {
	if e.sessionCts != nil {
		e.sessionCts()
	}

	if e.messageTask != nil {
		select {
		case <-e.messageTask:
		case <-ctx.Done():
		}
	}

	// e.session.Dispose()
	return nil
}
