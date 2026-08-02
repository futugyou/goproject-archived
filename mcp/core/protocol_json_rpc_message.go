package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

type IJsonRpcMessage interface {
	GetJsonRpc() string
	SetJsonRpc(version string)
	GetContext() *JsonRpcMessageContext
	SetContext(msgContext *JsonRpcMessageContext)
}

// BaseJsonRpcMessage 包含通用的 JSON-RPC 2.0 基础属性
type BaseJsonRpcMessage struct {
	JsonRpc string                 `json:"jsonrpc"`
	Context *JsonRpcMessageContext `json:"-"`
}

func (b BaseJsonRpcMessage) GetJsonRpc() string {
	if b.JsonRpc == "" {
		return "2.0"
	}
	return b.JsonRpc
}

func (b *BaseJsonRpcMessage) SetJsonRpc(version string) {
	b.JsonRpc = version
}

func (b *BaseJsonRpcMessage) GetContext() *JsonRpcMessageContext {
	if b == nil {
		return nil
	}
	return b.Context
}

func (b *BaseJsonRpcMessage) SetContext(msgContext *JsonRpcMessageContext) {
	if b == nil {
		return
	}
	b.Context = msgContext
}

type JsonRpcRequest struct {
	BaseJsonRpcMessage
	ID     *RequestId      `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type JsonRpcNotification struct {
	BaseJsonRpcMessage
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type JsonRpcErrorDetail struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type JsonRpcError struct {
	BaseJsonRpcMessage
	ID    *RequestId         `json:"id"`
	Error JsonRpcErrorDetail `json:"error"`
}

type JsonRpcResponse struct {
	BaseJsonRpcMessage
	ID     *RequestId      `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
}

type JsonRpcMessageWithId struct {
	BaseJsonRpcMessage
	ID *RequestId `json:"id"`
}

type JsonRpcMessage struct {
	IJsonRpcMessage
}

func (j *JsonRpcMessage) GetType() string {
	if j == nil || j.IJsonRpcMessage == nil {
		return "unknown"
	}

	switch j.IJsonRpcMessage.(type) {
	case *JsonRpcRequest:
		return "JsonRpcRequest"
	case *JsonRpcNotification:
		return "JsonRpcNotification"
	case *JsonRpcError:
		return "JsonRpcError"
	case *JsonRpcResponse:
		return "JsonRpcResponse"
	case *JsonRpcMessageWithId:
		return "JsonRpcMessageWithId"
	default:
		return "unknown"
	}
}

func (j *JsonRpcMessage) IsJsonRpcRequest() bool {
	_, ok := j.ToJsonRpcRequest()
	return ok
}

func (j *JsonRpcMessage) ToJsonRpcRequest() (*JsonRpcRequest, bool) {
	if j == nil || j.IJsonRpcMessage == nil {
		return nil, false
	}

	switch msg := j.IJsonRpcMessage.(type) {
	case *JsonRpcRequest:
		return msg, true
	default:
		return nil, false
	}
}

func (j *JsonRpcMessage) IsJsonRpcNotification() bool {
	_, ok := j.ToJsonRpcNotification()
	return ok
}

func (j *JsonRpcMessage) ToJsonRpcNotification() (*JsonRpcNotification, bool) {
	if j == nil || j.IJsonRpcMessage == nil {
		return nil, false
	}

	switch msg := j.IJsonRpcMessage.(type) {
	case *JsonRpcNotification:
		return msg, true
	default:
		return nil, false
	}
}

func (j *JsonRpcMessage) IsJsonRpcError() bool {
	_, ok := j.ToJsonRpcError()
	return ok
}

func (j *JsonRpcMessage) ToJsonRpcError() (*JsonRpcError, bool) {
	if j == nil || j.IJsonRpcMessage == nil {
		return nil, false
	}

	switch msg := j.IJsonRpcMessage.(type) {
	case *JsonRpcError:
		return msg, true
	default:
		return nil, false
	}
}

func (j *JsonRpcMessage) IsJsonRpcResponse() bool {
	_, ok := j.ToJsonRpcResponse()
	return ok
}

func (j *JsonRpcMessage) ToJsonRpcResponse() (*JsonRpcResponse, bool) {
	if j == nil || j.IJsonRpcMessage == nil {
		return nil, false
	}

	switch msg := j.IJsonRpcMessage.(type) {
	case *JsonRpcResponse:
		return msg, true
	default:
		return nil, false
	}
}

func (j *JsonRpcMessage) IsJsonRpcMessageWithId() bool {
	_, ok := j.ToJsonRpcMessageWithId()
	return ok
}

func (j *JsonRpcMessage) ToJsonRpcMessageWithId() (*JsonRpcMessageWithId, bool) {
	if j == nil || j.IJsonRpcMessage == nil {
		return nil, false
	}

	switch msg := j.IJsonRpcMessage.(type) {
	case *JsonRpcMessageWithId:
		return msg, true
	default:
		return nil, false
	}
}

func (r *JsonRpcMessage) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		r.IJsonRpcMessage = nil
		return nil
	}

	// 1. 解析为 map[string]json.RawMessage 以探测字段特征
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// 2. 校验 jsonrpc 协议版本
	jsonrpcRaw, hasJsonRpc := rawMap["jsonrpc"]
	if !hasJsonRpc {
		return errors.New("missing jsonrpc version")
	}
	var version string
	if err := json.Unmarshal(jsonrpcRaw, &version); err != nil || version != "2.0" {
		return errors.New("invalid jsonrpc version, expected \"2.0\"")
	}

	// 探测关键字段
	rawMethod, hasMethod := rawMap["method"]
	rawID, hasID := rawMap["id"]
	_, hasError := rawMap["error"]
	_, hasResult := rawMap["result"]

	if hasMethod {
		var methodStr string
		if err := json.Unmarshal(rawMethod, &methodStr); err != nil {
			return fmt.Errorf("invalid method property: %w", err)
		}

		if hasID {
			if string(rawID) == "null" {
				return errors.New("request id must not be null")
			}
			var req JsonRpcRequest
			if err := json.Unmarshal(data, &req); err != nil {
				return err
			}
			r.IJsonRpcMessage = &req
			return nil
		}

		// 有 method 无 id -> Notification
		var notif JsonRpcNotification
		if err := json.Unmarshal(data, &notif); err != nil {
			return err
		}
		r.IJsonRpcMessage = &notif
		return nil
	}

	// 没有 method 的情况下，必须包含 id 字段
	if hasID {
		if hasError {
			var errResp JsonRpcError
			if err := json.Unmarshal(data, &errResp); err != nil {
				return err
			}
			r.IJsonRpcMessage = &errResp
			return nil
		}

		if hasResult {
			var resp JsonRpcResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return err
			}
			r.IJsonRpcMessage = &resp
			return nil
		}

		return errors.New("response must have either result or error")
	}

	// 无 id 且无 method，但包含 error
	if hasError {
		var errResp JsonRpcError
		if err := json.Unmarshal(data, &errResp); err != nil {
			return err
		}
		r.IJsonRpcMessage = &errResp
		return nil
	}

	return errors.New("invalid JSON-RPC message format")
}

func (r JsonRpcMessage) MarshalJSON() ([]byte, error) {
	if r.IJsonRpcMessage == nil {
		return []byte("null"), nil
	}

	if r.IJsonRpcMessage.GetJsonRpc() == "" {
		r.IJsonRpcMessage.SetJsonRpc("2.0")
	}

	return json.Marshal(r.IJsonRpcMessage)
}

type RequestId struct {
	id any
}

func NewRequestIdFromString(value string) *RequestId {
	return &RequestId{id: value}
}

func NewRequestIdFromInt(value int64) *RequestId {
	return &RequestId{id: value}
}

func (r RequestId) IsDefault() bool {
	return r.id == nil
}

func (r *RequestId) String() string {
	if r == nil {
		return ""
	}
	switch v := r.id.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

func (r *RequestId) MarshalJSON() ([]byte, error) {
	if r == nil || r.id == nil {
		return []byte("null"), nil
	}
	switch v := r.id.(type) {
	case string:
		return json.Marshal(v)
	case int64:
		return json.Marshal(v)
	case nil:
		return json.Marshal("")
	default:
		return nil, errors.New("invalid RequestId type")
	}
}

func (r *RequestId) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		r.id = nil
		return nil
	}

	var strValue string
	if err := json.Unmarshal(data, &strValue); err == nil {
		r.id = strValue
		return nil
	}

	var intValue int64
	if err := json.Unmarshal(data, &intValue); err == nil {
		r.id = intValue
		return nil
	}

	var numValue float64
	if err := json.Unmarshal(data, &numValue); err == nil {
		if numValue == float64(int64(numValue)) {
			r.id = int64(numValue)
			return nil
		}
	}

	return errors.New("requestId must be a string or an integer")
}

type JsonRpcMessageContext struct {
	RelatedTransport   ITransport
	Items              map[string]any
	RoutingName        string
	ProtocolVersion    string
	ClientInfo         *Implementation
	ClientCapabilities *ClientCapabilities
}
