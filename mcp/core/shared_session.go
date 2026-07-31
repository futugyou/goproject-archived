package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type NotificationHandler func(ctx context.Context, notification *JsonRpcNotification) error

type McpSession interface {
	io.Closer
	SessionId() *string
	NegotiatedProtocolVersion() *string

	SendRequest(ctx context.Context, req *JsonRpcRequest) (*JsonRpcResponse, error)
	SendMessage(ctx context.Context, msg *JsonRpcMessage) error

	RegisterNotificationHandler(method string, handler NotificationHandler) (unsubscribe func(), err error)
}

func IsJuly2026OrLaterProtocol(s McpSession) bool {
	version := s.NegotiatedProtocolVersion()
	if version == nil {
		return false
	}
	return *version >= "2026-07-28"
}

func SendRequestTyped[TParameters any, TResult any](
	ctx context.Context,
	s McpSession,
	method string,
	params TParameters,
	requestID *RequestId,
) (*TResult, error) {
	if strings.TrimSpace(method) == "" {
		return nil, errors.New("method cannot be empty")
	}

	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	req := &JsonRpcRequest{
		ID:     requestID,
		Method: method,
		Params: rawParams,
	}

	resp, err := s.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var result TResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response result: %w", err)
	}

	return &result, nil
}

func SendNotification(ctx context.Context, s McpSession, method string) error {
	if strings.TrimSpace(method) == "" {
		return errors.New("method cannot be empty")
	}

	msg := &JsonRpcNotification{
		Method: method,
	}

	return s.SendMessage(ctx, &JsonRpcMessage{
		IJsonRpcMessage: msg,
	})
}

func SendNotificationWithParams[TParameters any](
	ctx context.Context,
	s McpSession,
	method string,
	params TParameters,
) error {
	if strings.TrimSpace(method) == "" {
		return errors.New("method cannot be empty")
	}

	rawParams, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	msg := &JsonRpcNotification{
		Method: method,
		Params: rawParams,
	}

	return s.SendMessage(ctx, &JsonRpcMessage{
		IJsonRpcMessage: msg,
	})
}

func NotifyProgress(
	ctx context.Context,
	s McpSession,
	token ProgressToken,
	progress ProgressNotificationValue,
	meta map[string]any,
) error {
	params := ProgressNotificationParams{
		ProgressToken: token,
		Progress:      progress,
		Meta:          meta,
	}
	return SendNotificationWithParams(ctx, s, NotificationMethods_ProgressNotification, params)
}

type RequestOptions struct {
	Meta          map[string]any
	ProgressToken *ProgressToken
}

func (o *RequestOptions) Clone() *RequestOptions {
	if o == nil {
		return nil
	}

	var metaCopy map[string]any
	if o.Meta != nil {
		metaCopy = make(map[string]any, len(o.Meta))
		for k, v := range o.Meta {
			metaCopy[k] = v
		}
	}

	return &RequestOptions{
		Meta:          metaCopy,
		ProgressToken: o.ProgressToken,
	}
}

func (o *RequestOptions) GetMetaForRequest() map[string]any {
	if o == nil {
		return nil
	}

	if o.ProgressToken == nil {
		return o.Meta
	}
	result := make(map[string]any, len(o.Meta)+1)
	for k, v := range o.Meta {
		result[k] = v
	}
	result["progressToken"] = o.ProgressToken

	return result
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
