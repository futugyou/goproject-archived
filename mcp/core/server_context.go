package core

import (
	"context"
	"errors"
	"time"
)

type MessageContext struct {
	Server         McpSession
	JsonRpcMessage *JsonRpcMessage
	Items          map[string]any
}

func NewMessageContext(server McpSession, jsonRpcMessage *JsonRpcMessage) *MessageContext {
	return &MessageContext{
		Server:         server,
		JsonRpcMessage: jsonRpcMessage,
	}
}

type RequestContext[TParams any] struct {
	Server         McpSession
	JsonRpcMessage *JsonRpcMessage
	Items          map[string]any
	Params         TParams
	JsonRpcRequest *JsonRpcRequest
}

func (r *RequestContext[TParams]) EnablePollingAsync(ctx context.Context, retryInterval time.Duration) error {
	jsonContext := r.JsonRpcRequest.GetContext()
	if jsonContext == nil {
		return errors.New("JsonRpcRequest has null context")
	}
	return nil
	// TODO
	// transport, ok := jsonContext.RelatedTransport.(*StreamableHttpPostTransport)
	// if !ok {
	// 	return errors.New("polling is only supported for Streamable HTTP transports")
	// }

	// return transport.EnablePolling(ctx, retryInterval)
}
