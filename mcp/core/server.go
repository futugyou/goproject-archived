package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type IMcpServerPrimitive interface {
	GetId() string
	Metadata() []any
}

type McpServer interface {
	McpSession
	ClientCapabilities() *ClientCapabilities
	ClientInfo() *Implementation
	IsMrtrSupported() bool
	Run(ctx context.Context) error
	OutgoingRequestInterceptor() InterceptorFunc
}

func errorIfElicitationUnsupported(request *ElicitRequestParams, server McpServer) error {
	clientCapabilities := server.ClientCapabilities()
	if clientCapabilities == nil {
		return errors.New("elicitation is not supported in stateless mode")
	}

	var elicitationCapability = clientCapabilities.Elicitation
	if elicitationCapability == nil {
		return errors.New("client does not support elicitation requests")
	}

	switch request.Mode {
	case "form":
		if request.RequestedSchema == nil {
			return errors.New("form mode elicitation requests require a requested schema")
		}

		if elicitationCapability.Form == nil {
			return errors.New("client does not support form mode elicitation requests")
		}
	case "url":
		if request.Url == nil {
			return errors.New("url mode elicitation requests require a url")
		}

		if request.ElicitationId == nil {
			return errors.New("url mode elicitation requests require an elicitation id")
		}

		if elicitationCapability.Url == nil {
			return errors.New("client does not support URL mode elicitation requests")
		}
	}

	return nil
}

type InterceptorFunc = func(ctx context.Context, method string, obj any) ([]byte, error)

func McpServerElicitTyped[T any](ctx context.Context, server McpServer, message string, options *RequestOptions) (*ElicitResultTyped[T], error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, errors.New("message can not be empty")
	}

	if options == nil {
		options = &RequestOptions{}
	}

	requestedSchema, err := ElicitTypeCache[T]()
	if err != nil {
		return nil, err
	}

	var request = &ElicitRequestParams{
		Message: message,
		RequestParams: RequestParams{
			Meta: options.GetMetaForRequest(),
		},
		RequestedSchema: requestedSchema,
	}

	if err := errorIfElicitationUnsupported(request, server); err != nil {
		return nil, err
	}

	raw, err := McpServerElicit(ctx, server, request)
	if err != nil {
		return nil, err
	}

	if !raw.IsAccepted() || raw.Content == nil {
		return &ElicitResultTyped[T]{Action: raw.Action}, nil
	}

	var typed T
	bytes, err := json.Marshal(raw.Content)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bytes, &typed); err != nil {
		return nil, err
	}

	return &ElicitResultTyped[T]{Action: raw.Action, Content: typed}, nil
}

func McpServerElicit(ctx context.Context, server McpServer, request *ElicitRequestParams) (*ElicitResult, error) {
	if request == nil {
		return nil, errors.New("request can not be nil")
	}

	interceptor := server.OutgoingRequestInterceptor()
	if interceptor != nil {
		paramsNode, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}

		resultNode, err := interceptor(ctx, RequestMethods_ElicitationCreate, paramsNode)
		if err != nil {
			return nil, err
		}

		var result ElicitResult
		if err := json.Unmarshal(resultNode, &result); err != nil {
			return &ElicitResult{Action: "cancel"}, nil
		}

		return &result, nil
	}

	if err := errorIfElicitationUnsupported(request, server); err != nil {
		return nil, err
	}

	result, err := SendRequestTyped[ElicitRequestParams, ElicitResult](ctx, server, RequestMethods_ElicitationCreate, *request, nil)
	if err != nil {
		return nil, err
	}

	*result = ElicitResultWithDefaults(request, *result)
	return result, nil
}

var _ McpServer = (*OutgoingRequestInterceptingMcpServer)(nil)

type OutgoingRequestInterceptingMcpServer struct {
	server      McpServer
	interceptor InterceptorFunc
}

func NewOutgoingRequestInterceptingMcpServer(server McpServer, interceptor InterceptorFunc) *OutgoingRequestInterceptingMcpServer {
	return &OutgoingRequestInterceptingMcpServer{
		server:      server,
		interceptor: interceptor,
	}
}

func (o *OutgoingRequestInterceptingMcpServer) ClientCapabilities() *ClientCapabilities {
	return o.server.ClientCapabilities()
}

func (o *OutgoingRequestInterceptingMcpServer) ClientInfo() *Implementation {
	return o.server.ClientInfo()
}

func (o *OutgoingRequestInterceptingMcpServer) Close() error {
	return o.server.Close()
}

func (o *OutgoingRequestInterceptingMcpServer) IsMrtrSupported() bool {
	return o.server.IsMrtrSupported()
}

func (o *OutgoingRequestInterceptingMcpServer) NegotiatedProtocolVersion() *string {
	return o.server.NegotiatedProtocolVersion()
}

func (o *OutgoingRequestInterceptingMcpServer) OutgoingRequestInterceptor() InterceptorFunc {
	return o.interceptor
}

func (o *OutgoingRequestInterceptingMcpServer) RegisterNotificationHandler(method string, handler NotificationHandler) (unsubscribe func(), err error) {
	return o.server.RegisterNotificationHandler(method, handler)
}

func (o *OutgoingRequestInterceptingMcpServer) Run(ctx context.Context) error {
	return o.server.Run(ctx)
}

func (o *OutgoingRequestInterceptingMcpServer) SendMessage(ctx context.Context, msg *JsonRpcMessage) error {
	return o.server.SendMessage(ctx, msg)
}

func (o *OutgoingRequestInterceptingMcpServer) SendRequest(ctx context.Context, req *JsonRpcRequest) (*JsonRpcResponse, error) {
	if req == nil {
		return nil, errors.New("request can not be nil")
	}

	r, err := o.interceptor(ctx, req.Method, req.Params)
	if err != nil {
		return nil, err
	}

	return &JsonRpcResponse{
		ID:     req.ID,
		Result: r,
	}, nil
}

func (o *OutgoingRequestInterceptingMcpServer) SessionId() *string {
	return o.server.SessionId()
}
