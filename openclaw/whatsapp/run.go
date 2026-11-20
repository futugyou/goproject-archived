package whatsapp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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

func (w *WhatsAppRun) notificationHandler(notification core.BridgeNotification) error {
	payload, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	writer := bufio.NewWriter(os.Stdout)
	fmt.Fprintln(writer, string(payload))
	return writer.Flush()
}

func (w *WhatsAppRun) Run() error {
	if w.service == nil {
		w.service = &WhatsAppWorkerService{}
	}

	w.service.Notification = w.notificationHandler

	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		var request core.BridgeRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			return err
		}

		response := w.HandleRequest(request)

		data, _ := json.Marshal(response)

		fmt.Fprintln(writer, string(data))
		writer.Flush()

		if line == "__shutdown__" {
			break
		}
	}

	return scanner.Err()
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
