package whatsapp

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/futugyou/openclaw/core"
)

func Ok(id string, value any) core.BridgeResponse {
	var data []byte
	if value != nil {
		data, _ = json.Marshal(value)
	}

	return core.BridgeResponse{Id: id, Result: data}
}

func Error(id string, code int, message string) core.BridgeResponse {
	return core.BridgeResponse{
		Id: id,
		Error: &core.BridgeError{
			Code:    code,
			Message: message,
		},
	}
}

type WhatsAppRun struct {
	service *WhatsAppWorkerService
	mu      sync.Mutex
}

func bindAndCall[TReq any, TRes any](
	id string,
	rawParams json.RawMessage,
	fn func(TReq) (TRes, error),
) core.BridgeResponse {
	var req TReq
	if err := json.Unmarshal(rawParams, &req); err != nil {
		return Error(id, -32603, err.Error())
	}
	res, err := fn(req)
	if err != nil {
		return Error(id, -32603, err.Error())
	}
	return Ok(id, res)
}

func (w *WhatsAppRun) HandleRequest(request core.BridgeRequest) core.BridgeResponse {
	switch request.Method {
	case "init":
		return bindAndCall(request.Id, request.Params, w.service.Initialize)
	case "channel_start":
		return bindAndCall(request.Id, request.Params, w.service.Start)
	case "channel_stop":
		return bindAndCall(request.Id, request.Params, w.service.Stop)
	case "channel_send":
		return bindAndCall(request.Id, request.Params, w.service.Send)
	case "channel_typing":
		return bindAndCall(request.Id, request.Params, w.service.SendTyping)
	case "channel_read_receipt":
		return bindAndCall(request.Id, request.Params, w.service.SendReadReceipt)
	case "channel_react":
		return bindAndCall(request.Id, request.Params, w.service.SendReaction)
	case "debug_simulate_inbound":
		result := w.service.DebugSimulateInbound(request.Params)
		return Ok(request.Id, result)
	case "debug_emit_auth_event":
		result := w.service.DebugEmitAuthEvent(request.Params)
		return Ok(request.Id, result)
	case "debug_get_state":
		result := w.service.DebugGetState()
		return Ok(request.Id, result)
	case "shutdown":
		result := w.service.Shutdown()
		return Ok(request.Id, result)
	default:
		return Error(request.Id, -32601, fmt.Sprintf("Unsupported method '%s'", request.Method))
	}
}
