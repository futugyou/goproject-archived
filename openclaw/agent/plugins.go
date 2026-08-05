package agent

import (
	"context"
	"os"

	"github.com/futugyou/openclaw/core"
)

type BridgeProcessLaunchSpec struct {
	FileName             string
	Arguments            []string
	WorkingDirectory     string
	EnvironmentVariables map[string]string
}

type BridgeNotificationHandler func(notify core.BridgeNotification) error

type IBridgeTransport interface {
	Prepare(ctx context.Context) error
	Start(ctx context.Context, process *os.Process) error
	SendRequest(ctx context.Context, method string, parameters any) error
	SendAndWait(ctx context.Context, method string, parameters any) (*core.BridgeResponse, error)
	SetNotificationHandler(handler BridgeNotificationHandler) error
}

type PluginBridgeMemorySnapshot struct {
	ProcessId          int
	WorkingSetBytes    int64
	PrivateMemoryBytes int64
}

type IPluginRuntimeTelemetrySource interface {
	TryGetRestartCount(pluginId string) (int, bool)
	TryGetMemorySnapshot(pluginId string) (*PluginBridgeMemorySnapshot, bool)
}

type SocketTransportOptions struct {
	SocketPath          string
	SocketDirectory     string
	OwnsSocketDirectory bool
	AuthToken           string
}
