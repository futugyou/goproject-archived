package client

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type McpClientResource struct {
	client           IMcpClient
	ProtocolResource core.Resource
	Uri              string
	Name             string
	Description      *string
	MimeTyp          *string
}

func NewMcpClientResource(client IMcpClient, protocolResource core.Resource) *McpClientResource {
	return &McpClientResource{
		client:           client,
		ProtocolResource: protocolResource,
		Uri:              protocolResource.Uri,
		Name:             protocolResource.Name,
		Description:      protocolResource.Description,
		MimeTyp:          protocolResource.MimeType,
	}
}

func (m *McpClientResource) Read(ctx context.Context) (*core.ReadResourceResult, error) {
	return m.client.ReadResource(ctx, m.Uri)
}
