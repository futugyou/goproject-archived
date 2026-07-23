package pluginkit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/openclaw/core"
)

type INativeDynamicPlugin interface {
	Register(pluginContext INativeDynamicPluginContext) error
}

type INativeDynamicPluginContext interface {
	GetPluginId() string
	GetConfig() json.RawMessage

	RegisterTool(tool core.ITool)
	RegisterChannel(adapter core.IChannelAdapter)
	RegisterCommand(name, description string, handler func(context.Context, string) string)
	RegisterProvider(providerId string, models []string, client chatcompletion.IChatClient)
	RegisterMemoryProvider(providerId string, factory func(NativeDynamicMemoryProviderContext) core.IMemoryStore)
	RegisterHook(hook core.IToolHook)
	RegisterService(service INativeDynamicPluginService)
	RegisterResultInterceptor(interceptor core.IToolResultInterceptor)
}

type NativeDynamicMemoryProviderContext struct {
	PluginId      string
	ProviderId    string
	Config        json.RawMessage
	GatewayConfig *core.GatewayConfig
	Metrics       *core.RuntimeMetrics
	Logger        *slog.Logger
}

type INativeDynamicPluginService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
