package server

import (
	"context"
	"fmt"
	"reflect"

	"github.com/futugyou/extensions_ai/abstractions/contents"
	"github.com/futugyou/extensions_ai/abstractions/functions"
	"github.com/futugyou/mcp/core"
	"github.com/futugyou/mcp/shared"
)

var _ IMcpServerResource = (*AIFunctionMcpServerResource)(nil)

type AIFunctionMcpServerResource struct {
	AIFunction       functions.AIFunction
	Resource         *core.Resource
	ResourceTemplate core.ResourceTemplate
	uriParser        *shared.UriParser
}

func NewAIFunctionMcpServerResource(function functions.AIFunction, resourceTemplate core.ResourceTemplate) *AIFunctionMcpServerResource {
	r := &AIFunctionMcpServerResource{
		AIFunction:       function,
		ResourceTemplate: resourceTemplate,
		Resource:         resourceTemplate.AsResource(),
	}
	r.uriParser, _ = shared.CreateUriParser(resourceTemplate.UriTemplate)
	return r
}

// GetId implements IMcpServerResource.
func (a *AIFunctionMcpServerResource) GetId() string {
	if a == nil || a.AIFunction == nil {
		return ""
	}

	return a.AIFunction.GetName()
}

// GetProtocolResource implements IMcpServerResource.
func (a *AIFunctionMcpServerResource) GetProtocolResource() *core.Resource {
	return a.Resource
}

// GetProtocolResourceTemplate implements IMcpServerResource.
func (a *AIFunctionMcpServerResource) GetProtocolResourceTemplate() core.ResourceTemplate {
	return a.ResourceTemplate
}

// Read implements IMcpServerResource.
func (m *AIFunctionMcpServerResource) Read(ctx context.Context, request RequestContext[*core.ReadResourceRequestParams]) (*core.ReadResourceResult, error) {
	if m == nil || m.AIFunction == nil {
		return nil, fmt.Errorf("ai function is nil")
	}

	if request.Params == nil {
		return nil, fmt.Errorf("request.Params is nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	arguments := functions.AIFunctionArguments{
		Context: map[any]any{reflect.TypeOf(request): request},
	}
	var matches map[string]string
	if m.uriParser != nil {
		matches = m.uriParser.Match(request.Params.Uri)
		for k, v := range matches {
			arguments.Set(k, v)
		}
	}

	result, err := m.AIFunction.Invoke(ctx, arguments)
	if err != nil {
		return nil, err
	}

	switch r := result.(type) {
	case *core.ReadResourceResult:
		return r, nil

	case *core.ResourceContents:
		return &core.ReadResourceResult{
			Contents: []core.ResourceContents{*r},
		}, nil
	case []core.ResourceContents:
		return &core.ReadResourceResult{
			Contents: r,
		}, nil

	case *contents.TextContent:
		textRes := &core.TextResourceContents{
			BaseResourceContents: core.BaseResourceContents{
				Uri:      request.Params.Uri,
				MimeType: m.ResourceTemplate.MimeType,
			},
			Text: r.Text,
		}

		return &core.ReadResourceResult{
			Contents: []core.ResourceContents{
				{
					IResourceContents: textRes,
				},
			},
		}, nil

	case *contents.DataContent:
		return &core.ReadResourceResult{
			Contents: []core.ResourceContents{
				{
					IResourceContents: &core.BlobResourceContents{
						BaseResourceContents: core.BaseResourceContents{Uri: request.Params.Uri, MimeType: &r.MediaType},
						Blob:                 r.GetBase64Data(),
					},
				},
			},
		}, nil

	case *string:
		return &core.ReadResourceResult{
			Contents: []core.ResourceContents{
				{
					IResourceContents: &core.TextResourceContents{
						BaseResourceContents: core.BaseResourceContents{
							Uri:      request.Params.Uri,
							MimeType: m.ResourceTemplate.MimeType,
						},
						Text: *r,
					},
				},
			},
		}, nil
	case []string:
		contents := []core.ResourceContents{}
		for _, v := range r {
			contents = append(contents, core.ResourceContents{
				IResourceContents: &core.TextResourceContents{
					BaseResourceContents: core.BaseResourceContents{
						Uri:      request.Params.Uri,
						MimeType: m.ResourceTemplate.MimeType,
					},

					Text: v,
				}})
		}
		return &core.ReadResourceResult{
			Contents: contents,
		}, nil
	case []contents.IAIContent:
		conts := []core.ResourceContents{}
		for _, v := range r {
			if a, ok := v.(*contents.TextContent); ok {
				conts = append(conts, core.ResourceContents{
					IResourceContents: &core.TextResourceContents{
						BaseResourceContents: core.BaseResourceContents{
							Uri:      request.Params.Uri,
							MimeType: m.ResourceTemplate.MimeType,
						},
						Text: a.Text,
					}})
			}
			if a, ok := v.(*contents.DataContent); ok {
				conts = append(conts, core.ResourceContents{
					IResourceContents: &core.BlobResourceContents{
						BaseResourceContents: core.BaseResourceContents{Uri: request.Params.Uri, MimeType: &a.MediaType},
						Blob:                 a.GetBase64Data(),
					}})
			}
		}
		return &core.ReadResourceResult{
			Contents: conts,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
}
