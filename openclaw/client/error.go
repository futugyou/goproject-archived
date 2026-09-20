package client

import "fmt"

type McpProtocolError struct {
	StatusCode int    // HTTP 状态码，如 404, 500
	RpcCode    *int   // JSON-RPC 错误码，如 -32601 (Method not found)
	Message    string // 错误信息
}

func (e *McpProtocolError) Error() string {
	if e.RpcCode != nil {
		return fmt.Sprintf("HTTP %d (MCP %d): %s", e.StatusCode, *e.RpcCode, e.Message)
	}
	return fmt.Sprintf("HTTP %d (MCP): %s", e.StatusCode, e.Message)
}

type HttpError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("HTTP error [%d %s]: %s", e.StatusCode, e.Status, e.Body)
}

type JsonError struct {
	Op  string // "marshal" 或 "unmarshal"
	Err error
}

func (e *JsonError) Error() string {
	return fmt.Sprintf("json %s failed: %v", e.Op, e.Err)
}
func (e *JsonError) Unwrap() error {
	return e.Err
}
