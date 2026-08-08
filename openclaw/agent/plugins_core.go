package agent

import (
	"context"

	"github.com/futugyou/openclaw/core"
)

type BridgedPluginTool struct {
	bridge   *PluginBridgeProcess
	pluginId string

	Name            string
	Description     string
	ParameterSchema string
	OutputSchema    string
	Optional        bool
}

func NewBridgedPluginTool(bridge *PluginBridgeProcess, pluginId string, registration core.PluginToolRegistration) *BridgedPluginTool {
	tool := &BridgedPluginTool{
		bridge:          bridge,
		pluginId:        pluginId,
		Name:            registration.Name,
		Description:     registration.Description,
		Optional:        registration.Optional,
		ParameterSchema: registration.GetParameterSchema(),
		OutputSchema:    registration.GetOutputSchema(),
	}
	return tool
}

func (b *BridgedPluginTool) Execute(ctx context.Context, argumentsJson string) (string, error) {
	return b.bridge.ExecuteTool(ctx, b.Name, argumentsJson)
}
