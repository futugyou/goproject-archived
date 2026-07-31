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
	requestID RequestId,
) (*TResult, error) {
	if strings.TrimSpace(method) == "" {
		return nil, errors.New("method cannot be empty")
	}

	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	req := &JsonRpcRequest{
		ID:     &requestID,
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
