package client

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type McpClientPrompt struct {
	prompt core.Prompt
	client IMcpClient
}

func NewMcpClientPrompt(prompt core.Prompt, client IMcpClient) *McpClientPrompt {
	return &McpClientPrompt{
		prompt: prompt,
		client: client,
	}
}

func (p *McpClientPrompt) GetPrompt() *core.Prompt {
	return &p.prompt
}

func (p *McpClientPrompt) GetName() string {
	return p.prompt.Name
}

func (p *McpClientPrompt) GetDescription() *string {
	return p.prompt.Description
}

func (p *McpClientPrompt) Get(ctx context.Context, arguments map[string]any) (*core.GetPromptResult, error) {
	return p.client.GetPrompt(ctx, p.prompt.Name, arguments)
}
