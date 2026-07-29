package shared

import (
	"context"

	"github.com/futugyou/mcp/core"
)

type NullProgress struct {
}

func (p *NullProgress) Report(value core.ProgressNotificationValue) {}

type TokenProgress struct {
	endpoint      IMcpEndpoint
	progressToken core.ProgressToken
}

func NewTokenProgress(endpoint IMcpEndpoint, progressToken core.ProgressToken) *TokenProgress {
	return &TokenProgress{endpoint, progressToken}
}

func (p *TokenProgress) Report(value core.ProgressNotificationValue) {
	p.endpoint.NotifyProgress(context.Background(), p.progressToken, value)
}
