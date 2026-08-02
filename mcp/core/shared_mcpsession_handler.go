package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type JsonRpcMessageHandler func(ctx context.Context, msg *JsonRpcMessage) error
type JsonRpcMessageFilter func(next JsonRpcMessageHandler) JsonRpcMessageHandler

var s_clientSessionDuration = CreateDurationHistogram("mcp.client.session.duration", "the duration of the MCP session as observed on the MCP client", true)
var s_serverSessionDuration = CreateDurationHistogram("mcp.server.session.duration", "the duration of the MCP session as observed on the MCP server", true)
var s_clientOperationDuration = CreateDurationHistogram("mcp.client.operation.duration", "the duration of the MCP request or notification as observed on the sender from the time it was sent until the response or ack is received", false)
var s_serverOperationDuration = CreateDurationHistogram("mcp.server.operation.duration", "mcp request or notification duration as observed on the receiver from the time it was received until the result or ack is sent", false)

type rpcResult struct {
	msg JsonRpcMessage
	err error
}

type McpSessionHandler struct {
	isServer                  bool
	transportKind             string
	transport                 ITransport
	requestHandlers           *RequestHandlers
	notificationHandlers      *NotificationHandlers
	incomingMessageFilter     JsonRpcMessageFilter
	outgoingMessageFilter     JsonRpcMessageFilter
	sessionStartingTimestamp  int64
	pendingRequests           sync.Map // map[RequestId]rpcResult
	handlingRequests          sync.Map // map[RequestId]context.CancelFunc
	sessionId                 string
	lastRequestId             atomic.Int64
	EndpointName              string
	NegotiatedProtocolVersion string
	logger                    *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewMcpSessionHandler(isServer bool, transp ITransport, endpointName string, requestHandlers *RequestHandlers, notificationHandlers *NotificationHandlers, incomingMessageFilter JsonRpcMessageFilter, outgoingMessageFilter JsonRpcMessageFilter, logger *slog.Logger) (*McpSessionHandler, error) {
	transportKind := ""
	// TODO
	// transportKind := transp.GetTransportKind()
	if outgoingMessageFilter == nil {
		outgoingMessageFilter = func(next JsonRpcMessageHandler) JsonRpcMessageHandler { return next }
	}
	if incomingMessageFilter == nil {
		incomingMessageFilter = func(next JsonRpcMessageHandler) JsonRpcMessageHandler { return next }
	}
	if logger == nil {
		logger = slog.Default()
	}

	mcpSessionHandler := &McpSessionHandler{
		isServer:                 isServer,
		transportKind:            transportKind,
		transport:                transp,
		requestHandlers:          requestHandlers,
		notificationHandlers:     notificationHandlers,
		outgoingMessageFilter:    outgoingMessageFilter,
		incomingMessageFilter:    incomingMessageFilter,
		logger:                   logger,
		sessionStartingTimestamp: time.Now().UnixNano(),
		sessionId:                uuid.New().String(),
		EndpointName:             endpointName,
	}

	SetRequestHandler(requestHandlers, RequestMethods_Ping, func(ctx context.Context, request *struct{}, jsonRpcRequest *JsonRpcRequest) (*PingResult, error) {
		perRequestVersion := mcpSessionHandler.NegotiatedProtocolVersion
		if jsonRpcRequest != nil && jsonRpcRequest.Context != nil && jsonRpcRequest.Context.ProtocolVersion != "" {
			perRequestVersion = jsonRpcRequest.Context.ProtocolVersion
		}

		if RequiresPerRequestMetadata(perRequestVersion) {
			return nil, fmt.Errorf("method '%s' is not available on protocol version '%s'", RequestMethods_Ping, perRequestVersion)
		}

		return &PingResult{}, nil
	})

	logger.Info("session created",
		"endpoint", mcpSessionHandler.EndpointName,
		"session_id", mcpSessionHandler.sessionId,
		"transport", mcpSessionHandler.transportKind,
	)

	return mcpSessionHandler, nil
}

func (p *McpSessionHandler) NextID() int64 {
	return p.lastRequestId.Add(1)
}

func (p *McpSessionHandler) ProcessMessagesStart(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.done != nil {
		return errors.New("the message processing loop has already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)

		p.processMessagesCore(ctx)
	}()

	return nil
}

func (p *McpSessionHandler) Close() {
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.mu.Unlock()

	if done == nil {
		return
	}

	if cancel != nil {
		cancel()
	}

	durationMetric := s_clientSessionDuration
	if p.isServer {
		durationMetric = s_serverSessionDuration
	}

	tags := []attribute.KeyValue{}
	tags = append(tags, attribute.String("session.id", p.sessionId))
	tags = append(tags, attribute.String("network.transport", p.transportKind))

	incr := time.Now().UnixNano() - p.sessionStartingTimestamp
	durationMetric.Record(context.Background(), (float64)(incr), metric.WithAttributes(tags...))

	p.pendingRequests.Range(func(key, value any) bool {
		if p.pendingRequests.CompareAndDelete(key, value) {
			done := value.(chan rpcResult)
			select {
			case done <- rpcResult{err: context.Canceled}:
			default:
			}
		}
		return true
	})
	<-done
}

func (p *McpSessionHandler) processMessagesCore(ctx context.Context) {
	// 使用 sync.WaitGroup 来追踪所有正在运行的 handler (in-flight)
	var wg sync.WaitGroup

	defer func() {
		wg.Wait()

		p.failPendingRequests(errors.New("the server shut down unexpectedly"))
	}()

	// 中从 channel 迭代读取消息，同时监听 Context 取消
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("message processing canceled", "EndpointName", p.EndpointName)
			return

		case msg, ok := <-p.transport.MessageReader():
			if !ok {
				return
			}

			p.logger.Info("endpoint read message from channel.", "EndpointName", p.EndpointName, "MsgType", msg.GetType())

			wg.Add(1)

			go func(m JsonRpcMessage) {
				defer wg.Done()

				p.processSingleMessage(ctx, &m)
			}(msg)
		}
	}
}

func (p *McpSessionHandler) failPendingRequests(err error) {
	p.pendingRequests.Range(func(key, value any) bool {
		done := value.(chan rpcResult)
		select {
		case done <- rpcResult{err: err}:
		default:
		}
		return true
	})
}

func (p *McpSessionHandler) processSingleMessage(parentCtx context.Context, msg *JsonRpcMessage) {
	// 1. 处理请求级别的 Context
	reqCtx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	messageWithID, isReqWithID := msg.ToJsonRpcMessageWithId()
	if isReqWithID {
		if msg.IsJsonRpcRequest() {
			p.handlingRequests.Store(messageWithID.ID, cancel)
			defer p.handlingRequests.Delete(messageWithID.ID)
		}
	}

	// 执行业务逻辑：
	err := p.handleMessage(reqCtx, msg)
	if err == nil {
		return
	}

	// 检查是否是用户主动取消
	isUserCancellation := errors.Is(err, context.Canceled) &&
		parentCtx.Err() == nil &&
		reqCtx.Err() != nil

	requestMsg, ok := msg.ToJsonRpcRequest()
	if !isUserCancellation && ok {
		// 构造 RPC Error 并发送回客户端
		errDetail := p.mapExceptionToRPCError(err)
		msgContext := &JsonRpcMessageContext{}
		if requestMsg.Context != nil {
			msgContext.RelatedTransport = requestMsg.Context.RelatedTransport
		}
		errMsg := NewJsonRpcError(requestMsg.ID, errDetail, msgContext)

		// 发送错误消息
		_ = p.SendMessage(parentCtx, errMsg)
	} else if !errors.Is(err, context.Canceled) {
		p.logger.Warn("message handler failed", "EndpointName", p.EndpointName, "MsgType", msg.GetType(), "Message", err.Error())
	}
}

func (p *McpSessionHandler) mapExceptionToRPCError(err error) *JsonRpcErrorDetail {
	// TODO
	return &JsonRpcErrorDetail{
		Code:    500,
		Message: err.Error(),
	}
}

func (p *McpSessionHandler) SendMessage(ctx context.Context, message *JsonRpcMessage) error {
	if message == nil {
		return errors.New("mcp session or message is nil")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if request, ok := message.ToJsonRpcRequest(); ok {
		return fmt.Errorf("cannot send '%s' request via sendMessage. use sendRequest  instead to get a correlated response", request.Method)
	}

	durationMetric := s_clientOperationDuration
	if p.isServer {
		durationMetric = s_serverOperationDuration
	}

	method := getMethodName(message)

	var startingTimestamp int64 = time.Now().UnixNano()
	ctx, span := Tracer.Start(ctx, p.createActivityName(method, ""))
	defer span.End()

	target := extractTargetFromMessage(message, method)
	PropagatorInject(ctx, message)

	tags := []attribute.KeyValue{}
	p.addStandardTags(&tags, method, message, target)
	defer finalizeDiagnostics(ctx, &startingTimestamp, durationMetric, tags)

	err := p.sendToRelatedTransport(ctx, message)
	if err != nil {
		addExceptionTags(&tags, err)
		return err
	}

	if notification, ok := message.ToJsonRpcNotification(); ok {
		if params := getCancelledNotificationParams(notification.Params); params != nil {
			if value, ok := p.pendingRequests.LoadAndDelete(params.RequestId); ok {
				done := value.(chan rpcResult)
				select {
				case done <- rpcResult{err: context.Canceled}:
				default:
				}
			}
		}
	}

	return nil
}

func addExceptionTags(tags *[]attribute.KeyValue, err error) {
	*tags = append(*tags, attribute.String("error", err.Error()))
	*tags = append(*tags, attribute.String("rpc.jsonrpc.error_code", "500"))
}

func (p *McpSessionHandler) sendToRelatedTransport(ctx context.Context, message *JsonRpcMessage) error {
	return p.outgoingMessageFilter(func(ct context.Context, msg *JsonRpcMessage) error {
		request, ok := msg.ToJsonRpcRequest()
		if ok {
			p.logger.Info("sending request.", "EndpointName", p.EndpointName, "Method", request.Method)
		} else {
			p.logger.Info("sending request.", "EndpointName", p.EndpointName)
		}
		var relatedTransport ITransport
		con := request.GetContext()
		if con != nil {
			relatedTransport = con.RelatedTransport
		}
		if relatedTransport == nil {
			relatedTransport = p.transport
		}
		return relatedTransport.SendMessage(ct, *msg)
	})(ctx, message)
}

func getCancelledNotificationParams(rawMessage json.RawMessage) *CancelledNotificationParams {
	var result CancelledNotificationParams
	if err := json.Unmarshal(rawMessage, &result); err != nil {
		return nil
	}

	return &result
}

func finalizeDiagnostics(ctx context.Context, startingTimestamp *int64, durationMetric metric.Float64Histogram, tags []attribute.KeyValue) {
	if startingTimestamp != nil {
		incr := *startingTimestamp - time.Now().UnixNano()
		durationMetric.Record(ctx, (float64)(incr), metric.WithAttributes(tags...))
	}
}

func extractTargetFromMessage(message *JsonRpcMessage, method string) string {
	params := message.GetParams()
	if params == nil {
		return ""
	}

	switch method {
	case RequestMethods_ToolsCall, RequestMethods_PromptsGet:
		var d map[string]any
		if err := json.Unmarshal(params, &d); err != nil {
			return ""
		}
		if name, ok := d["name"].(string); ok {
			return name
		}
	}
	return ""
}

func getMethodName(message *JsonRpcMessage) string {
	switch request := message.IJsonRpcMessage.(type) {
	case *JsonRpcRequest:
		return request.Method
	case *JsonRpcNotification:
		return request.Method
	default:
		return "unknownMethod"
	}
}

func (m *McpSessionHandler) addStandardTags(tags *[]attribute.KeyValue, method string, message *JsonRpcMessage, target string) {
	*tags = append(*tags, attribute.String("mcp.method.name", method))
	*tags = append(*tags, attribute.String("network.transport", m.transportKind))
	if m.transportKind == "tcp" {
		*tags = append(*tags, attribute.String("network.protocol.name", "http"))
	}

	if m.NegotiatedProtocolVersion != "" {
		*tags = append(*tags, attribute.String("mcp.protocol.version", m.NegotiatedProtocolVersion))
	}
	*tags = append(*tags, attribute.String("session.id", m.sessionId))
	if withid, ok := message.ToJsonRpcMessageWithId(); ok {
		*tags = append(*tags, attribute.String("jsonrpc.request.id", withid.ID.String()))
	}

	switch method {
	case RequestMethods_ToolsCall:
		if target != "" {
			*tags = append(*tags, attribute.String("gen_ai.tool.name", target))
			*tags = append(*tags, attribute.String("gen_ai.operation.name", "execute_tool"))
		}
	case RequestMethods_PromptsGet:
		if target != "" {
			*tags = append(*tags, attribute.String("gen_ai.prompt.name", target))
		}
	case RequestMethods_ResourcesRead, RequestMethods_ResourcesSubscribe, RequestMethods_ResourcesUnsubscribe, NotificationMethods_ResourceUpdatedNotification:
		params := message.GetParams()
		if params != nil {
			var d map[string]any
			if err := json.Unmarshal(params, &d); err == nil {
				if uri, ok := d["uri"].(string); ok && uri != "" {
					*tags = append(*tags, attribute.String("mcp.resource.uri", uri))
				}
			}
		}
	}
}

func (m *McpSessionHandler) createActivityName(method, target string) string {
	if target == "" {
		return method
	}

	return fmt.Sprintf("%s %s", method, target)
}

func (p *McpSessionHandler) handleMessage(ctx context.Context, message *JsonRpcMessage) error {
	incomingRequest, ok := message.ToJsonRpcRequest()
	if p.isServer && ok {
		populateContextFromMeta(incomingRequest)
	}

	durationMetric := s_clientOperationDuration
	if p.isServer {
		durationMetric = s_serverOperationDuration
	}
	method := getMethodName(message)

	var startingTimestamp int64 = time.Now().UnixNano()
	target := extractTargetFromMessage(message, method)
	ctx, span := StartSpanWithJsonRpcData(ctx, p.createActivityName(method, target), message)
	defer span.End()

	tags := []attribute.KeyValue{}
	p.addStandardTags(&tags, method, message, target)
	defer finalizeDiagnostics(ctx, &startingTimestamp, durationMetric, tags)

	err := p.incomingMessageFilter(func(ct context.Context, msg *JsonRpcMessage) error {
		result, err := p.handleMessageCore(ct, msg)
		if err != nil {
			return err
		}

		addResponseTags(&tags, span, result, method)

		return nil
	})(ctx, message)
	if err != nil {
		addExceptionTags(&tags, err)
	}

	return err
}

func addResponseTags(tags *[]attribute.KeyValue, span trace.Span, result json.RawMessage, method string) {
	var data struct {
		IsError bool            `json:"isError"`
		Content json.RawMessage `json:"content"`
	}

	err := json.Unmarshal(result, &data)
	if err != nil {
		return
	}

	if data.IsError {
		span.SetStatus(500, string(data.Content))
	}

	if method == RequestMethods_ToolsCall {
		*tags = append(*tags, attribute.String("session.id", "tool_error"))
	} else {
		*tags = append(*tags, attribute.String("session.id", "_OTHER"))
	}
}

func (m *McpSessionHandler) handleMessageCore(ctx context.Context, msg *JsonRpcMessage) (json.RawMessage, error) {
	var result json.RawMessage
	var err error
	switch request := msg.IJsonRpcMessage.(type) {
	case *JsonRpcRequest:
		result, err = m.handleRequest(ctx, *request)
	case *JsonRpcNotification:
		err = m.handleNotification(ctx, request)
	case *JsonRpcMessageWithId:
		err = m.handleMessageWithId(msg, *request)
	default:
	}

	return result, err
}

func (m *McpSessionHandler) handleNotification(ctx context.Context, notification *JsonRpcNotification) error {
	if notification.Method == NotificationMethods_CancelledNotification {
		if cn := getCancelledNotificationParams(notification.Params); cn != nil {
			value, ok := m.handlingRequests.Load(cn.RequestId)
			if ok {
				if cancel, ok := value.(context.CancelFunc); ok {
					cancel()
				}
			}
		}
	}

	return m.notificationHandlers.InvokeHandlers(ctx, notification.Method, *notification)
}

func (m *McpSessionHandler) handleMessageWithId(message *JsonRpcMessage, messageWithId JsonRpcMessageWithId) error {
	if messageWithId.ID == nil || len(messageWithId.ID.String()) == 0 {
		return nil
	}
	requestid := messageWithId.ID
	value, ok := m.pendingRequests.LoadAndDelete(*requestid)
	if !ok {
		return nil
	}

	done := value.(chan rpcResult)
	select {
	case done <- rpcResult{msg: *message}:
	default:
	}

	return nil
}

func (m *McpSessionHandler) handleRequest(ctx context.Context, request JsonRpcRequest) (json.RawMessage, error) {
	handler, ok := m.requestHandlers.Get(request.Method)
	if !ok {
		return nil, fmt.Errorf("no handler found for method %s", request.Method)
	}

	result, err := handler(ctx, &request)
	if err != nil {
		return nil, err
	}

	msg := &JsonRpcMessage{
		IJsonRpcMessage: &JsonRpcResponse{
			ID:     request.ID,
			Result: result,
		},
	}
	msg.SetContext(request.Context)

	if err := m.SendMessage(ctx, msg); err != nil {
		return nil, err
	}

	return result, nil
}

func populateContextFromMeta(incomingRequest *JsonRpcRequest) {
	// TODO
}

func (m *McpSessionHandler) RegisterNotificationHandler(method string, handler HandlerFunc) *Registration {
	return m.notificationHandlers.Register(method, handler, true)
}

func (m *McpSessionHandler) SendRequest(ctx context.Context, request *JsonRpcRequest) (*JsonRpcResponse, error) {
	if m == nil || request == nil {
		return nil, fmt.Errorf("session or request is nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if request.ID == nil {
		request.ID = NewRequestIdFromInt(m.NextID())
	}

	durationMetric := s_clientOperationDuration
	if m.isServer {
		durationMetric = s_serverOperationDuration
	}

	method := request.Method
	message := &JsonRpcMessage{IJsonRpcMessage: request}
	target := extractTargetFromMessage(message, method)

	var startingTimestamp int64 = time.Now().UnixNano()
	ctx, span := Tracer.Start(ctx, m.createActivityName(method, target))
	defer span.End()

	tags := []attribute.KeyValue{}
	m.addStandardTags(&tags, method, message, target)
	defer finalizeDiagnostics(ctx, &startingTimestamp, durationMetric, tags)

	PropagatorInject(ctx, message)

	done := make(chan rpcResult, 1)
	m.pendingRequests.Store(*request.ID, done)
	defer m.pendingRequests.Delete(*request.ID)

	err := m.sendToRelatedTransport(ctx, message)
	if err != nil {
		addExceptionTags(&tags, err)
		return nil, err
	}

	select {
	case res := <-done:
		if res.err != nil {
			return nil, res.err
		}

		jsonerr, ok := res.msg.ToJsonRpcError()
		if ok {
			return nil, errors.New(jsonerr.ErrorMessage())
		}

		response, ok := res.msg.ToJsonRpcResponse()
		if ok {
			addResponseTags(&tags, span, response.Result, method)
			return response, nil
		}

		return nil, errors.New("unknown response type")
	case <-ctx.Done():
		if request.Method != RequestMethods_Initialize {
			go func() {
				m.sendCancelNotification(context.Background(), request)
			}()
		}

		return nil, ctx.Err()
	}
}

func (m *McpSessionHandler) sendCancelNotification(ctx context.Context, request *JsonRpcRequest) error {
	cancelParam := CancelledNotificationParams{
		RequestId: *request.ID,
	}
	param, _ := json.Marshal(cancelParam)
	msgContext := &JsonRpcMessageContext{}
	if request.Context != nil {
		msgContext.RelatedTransport = request.Context.RelatedTransport
	}
	cancelNotification := &JsonRpcNotification{
		Method: NotificationMethods_CancelledNotification,
		Params: param,
	}
	cancelNotification.SetContext(msgContext)
	return m.SendMessage(ctx, &JsonRpcMessage{
		IJsonRpcMessage: cancelNotification,
	})
}
