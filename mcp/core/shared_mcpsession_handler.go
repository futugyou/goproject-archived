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

var (
	s_clientSessionDuration   = CreateDurationHistogram("mcp.client.session.duration", "the duration of the MCP session as observed on the MCP client", true)
	s_serverSessionDuration   = CreateDurationHistogram("mcp.server.session.duration", "the duration of the MCP session as observed on the MCP server", true)
	s_clientOperationDuration = CreateDurationHistogram("mcp.client.operation.duration", "the duration of the MCP request or notification as observed on the sender from the time it was sent until the response or ack is received", false)
	s_serverOperationDuration = CreateDurationHistogram("mcp.server.operation.duration", "mcp request or notification duration as observed on the receiver from the time it was received until the result or ack is sent", false)
)

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
	pendingRequests           sync.Map // map[RequestId]chan rpcResult
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

func NewMcpSessionHandler(
	isServer bool,
	transp ITransport,
	endpointName string,
	requestHandlers *RequestHandlers,
	notificationHandlers *NotificationHandlers,
	incomingMessageFilter JsonRpcMessageFilter,
	outgoingMessageFilter JsonRpcMessageFilter,
	logger *slog.Logger,
) (*McpSessionHandler, error) {
	if transp == nil {
		return nil, errors.New("transport cannot be nil")
	}

	// TODO
	// transportKind := transp.GetTransportKind()
	transportKind := ""

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

	if requestHandlers != nil {
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
	}

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

	// 等待核心消息循环与所有 in-flight goroutines 彻底结束
	<-done

	// 记录 Session 生命周期指标
	durationMetric := s_clientSessionDuration
	if p.isServer {
		durationMetric = s_serverSessionDuration
	}

	tags := []attribute.KeyValue{
		attribute.String("session.id", p.sessionId),
		attribute.String("network.transport", p.transportKind),
	}

	incr := time.Now().UnixNano() - p.sessionStartingTimestamp
	durationMetric.Record(context.Background(), float64(incr)/float64(time.Millisecond), metric.WithAttributes(tags...))
}

func (p *McpSessionHandler) processMessagesCore(ctx context.Context) {
	var wg sync.WaitGroup

	defer func() {
		// 1. 等待所有处理中的请求 goroutine 退出
		wg.Wait()
		// 2. 清理所有尚未响应的 pending 请求
		p.failPendingRequests(errors.New("the session shut down unexpectedly"))
	}()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("message processing loop canceled", "EndpointName", p.EndpointName)
			return

		case msg, ok := <-p.transport.MessageReader():
			if !ok {
				p.logger.Info("message reader channel closed", "EndpointName", p.EndpointName)
				return
			}

			p.logger.Debug("endpoint read message", "EndpointName", p.EndpointName, "MsgType", msg.GetType())

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
		if p.pendingRequests.CompareAndDelete(key, value) {
			if done, ok := value.(chan rpcResult); ok {
				select {
				case done <- rpcResult{err: err}:
				default:
				}
			}
		}
		return true
	})
}

func (p *McpSessionHandler) processSingleMessage(parentCtx context.Context, msg *JsonRpcMessage) {
	reqCtx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	messageWithID, isReqWithID := msg.ToJsonRpcMessageWithId()
	if isReqWithID && messageWithID != nil && messageWithID.ID != nil {
		if msg.IsJsonRpcRequest() {
			p.handlingRequests.Store(*messageWithID.ID, cancel)
			defer p.handlingRequests.Delete(*messageWithID.ID)
		}
	}

	err := p.handleMessage(reqCtx, msg)
	if err == nil {
		return
	}

	isUserCancellation := errors.Is(err, context.Canceled) &&
		parentCtx.Err() == nil &&
		reqCtx.Err() != nil

	requestMsg, ok := msg.ToJsonRpcRequest()
	if !isUserCancellation && ok {
		errDetail := p.mapExceptionToRPCError(err)
		msgContext := &JsonRpcMessageContext{}
		if requestMsg.Context != nil {
			msgContext.RelatedTransport = requestMsg.Context.RelatedTransport
		}
		errMsg := NewJsonRpcError(requestMsg.ID, errDetail, msgContext)

		_ = p.SendMessage(parentCtx, errMsg)
	} else if !errors.Is(err, context.Canceled) {
		p.logger.Warn("message handler failed", "EndpointName", p.EndpointName, "MsgType", msg.GetType(), "error", err)
	}
}

func (p *McpSessionHandler) mapExceptionToRPCError(err error) *JsonRpcErrorDetail {
	if errors.Is(err, context.Canceled) {
		return &JsonRpcErrorDetail{
			Code:    -32800, // JSON-RPC Request Canceled
			Message: "Request canceled",
		}
	}
	return &JsonRpcErrorDetail{
		Code:    -32603, // Internal JSON-RPC error
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
		return fmt.Errorf("cannot send '%s' request via SendMessage; use SendRequest instead", request.Method)
	}

	durationMetric := s_clientOperationDuration
	if p.isServer {
		durationMetric = s_serverOperationDuration
	}

	method := getMethodName(message)
	startingTimestamp := time.Now().UnixNano()

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
			if value, loaded := p.pendingRequests.LoadAndDelete(params.RequestId); loaded {
				if done, ok := value.(chan rpcResult); ok {
					select {
					case done <- rpcResult{err: context.Canceled}:
					default:
					}
				}
			}
		}
	}

	return nil
}

func addExceptionTags(tags *[]attribute.KeyValue, err error) {
	*tags = append(*tags, attribute.String("error.type", fmt.Sprintf("%T", err)))
	*tags = append(*tags, attribute.String("error.message", err.Error()))
}

func (p *McpSessionHandler) sendToRelatedTransport(ctx context.Context, message *JsonRpcMessage) error {
	return p.outgoingMessageFilter(func(ct context.Context, msg *JsonRpcMessage) error {
		msgType := "message"
		if req, ok := msg.ToJsonRpcRequest(); ok {
			msgType = fmt.Sprintf("request [%s]", req.Method)
		} else if resp, ok := msg.ToJsonRpcResponse(); ok {
			msgType = fmt.Sprintf("response [ID: %v]", resp.ID)
		} else if notif, ok := msg.ToJsonRpcNotification(); ok {
			msgType = fmt.Sprintf("notification [%s]", notif.Method)
		}

		p.logger.Debug("sending outgoing message", "EndpointName", p.EndpointName, "Type", msgType)

		var relatedTransport ITransport
		con := msg.GetContext()
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
	if len(rawMessage) == 0 {
		return nil
	}
	var result CancelledNotificationParams
	if err := json.Unmarshal(rawMessage, &result); err != nil {
		return nil
	}
	return &result
}

func finalizeDiagnostics(ctx context.Context, startingTimestamp *int64, durationMetric metric.Float64Histogram, tags []attribute.KeyValue) {
	if startingTimestamp != nil && *startingTimestamp > 0 {
		// 修正耗时计算逻辑 (当前纳秒 - 开始纳秒)
		elapsedNs := time.Now().UnixNano() - *startingTimestamp
		elapsedMs := float64(elapsedNs) / float64(time.Millisecond)
		durationMetric.Record(ctx, elapsedMs, metric.WithAttributes(tags...))
	}
}

func extractTargetFromMessage(message *JsonRpcMessage, method string) string {
	params := message.GetParams()
	if len(params) == 0 {
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
	if message == nil || message.IJsonRpcMessage == nil {
		return "unknownMethod"
	}
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

	if withid, ok := message.ToJsonRpcMessageWithId(); ok && withid != nil && withid.ID != nil {
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
		if len(params) > 0 {
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

	startingTimestamp := time.Now().UnixNano()
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
	if len(result) == 0 {
		return
	}

	var data struct {
		IsError bool            `json:"isError"`
		Content json.RawMessage `json:"content"`
	}

	if err := json.Unmarshal(result, &data); err != nil {
		return
	}

	if data.IsError {
		span.SetStatus(500, string(data.Content))
	}

	if method == RequestMethods_ToolsCall {
		*tags = append(*tags, attribute.String("error.type", "tool_error"))
	} else {
		*tags = append(*tags, attribute.String("error.type", "_OTHER"))
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
			if value, ok := m.handlingRequests.Load(cn.RequestId); ok {
				if cancel, ok := value.(context.CancelFunc); ok {
					cancel()
				}
			}
		}
	}

	if m.notificationHandlers == nil {
		return nil
	}

	return m.notificationHandlers.InvokeHandlers(ctx, notification.Method, *notification)
}

func (m *McpSessionHandler) handleMessageWithId(message *JsonRpcMessage, messageWithId JsonRpcMessageWithId) error {
	if messageWithId.ID == nil {
		return nil
	}

	requestid := *messageWithId.ID
	value, ok := m.pendingRequests.LoadAndDelete(requestid)
	if !ok {
		return nil
	}

	done, ok := value.(chan rpcResult)
	if !ok {
		return errors.New("invalid pending request result channel")
	}

	select {
	case done <- rpcResult{msg: *message}:
	default:
		m.logger.Warn("pending request result channel full or unreachable", "request_id", requestid.String())
	}

	return nil
}

func (m *McpSessionHandler) handleRequest(ctx context.Context, request JsonRpcRequest) (json.RawMessage, error) {
	if m.requestHandlers == nil {
		return nil, fmt.Errorf("no request handlers configured for method %s", request.Method)
	}

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
	// 实现业务自定义 Context 解包逻辑
}

func (m *McpSessionHandler) RegisterNotificationHandler(method string, handler HandlerFunc) *Registration {
	if m.notificationHandlers == nil {
		return nil
	}
	return m.notificationHandlers.Register(method, handler, true)
}

func (m *McpSessionHandler) SendRequest(ctx context.Context, request *JsonRpcRequest) (*JsonRpcResponse, error) {
	if m == nil || request == nil {
		return nil, errors.New("session or request is nil")
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

	startingTimestamp := time.Now().UnixNano()
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
				_ = m.sendCancelNotification(context.Background(), request)
			}()
		}
		return nil, ctx.Err()
	}
}

func (m *McpSessionHandler) sendCancelNotification(ctx context.Context, request *JsonRpcRequest) error {
	if request == nil || request.ID == nil {
		return nil
	}

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
