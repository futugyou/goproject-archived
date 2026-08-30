package mcpnative

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type McpNativeTool struct {
	mcpSession                *mcp.ClientSession
	localName                 string
	remoteName                string
	description               string
	parameterSchema           string
	suppressStructuredContent bool
}

func NewMcpNativeTool(
	mcpSession *mcp.ClientSession,
	localName,
	remoteName,
	description,
	parameterSchema string,
	suppressStructuredContent bool) *McpNativeTool {
	return &McpNativeTool{mcpSession: mcpSession, localName: localName, remoteName: remoteName, description: description, parameterSchema: parameterSchema, suppressStructuredContent: suppressStructuredContent}
}

func (e *McpNativeTool) Name() string {
	return e.localName
}

func (e *McpNativeTool) Description() string {
	return e.description
}

func (e *McpNativeTool) ParameterSchema() string {
	return e.parameterSchema
}

func (a *McpNativeTool) Execute(ctx context.Context, argumentsJson string) string {
	return a.executeCore(ctx, argumentsJson, nil)
}

func (a *McpNativeTool) ExecuteContext(ctx context.Context, argumentsJson string, toolContext core.ToolExecutionContext) string {
	return a.executeCore(ctx, argumentsJson, &toolContext)
}

func (e *McpNativeTool) executeCore(ctx context.Context, argumentsJson string, toolContext *core.ToolExecutionContext) string {
	var argsDoc map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &argsDoc); err != nil {
		return err.Error()
	}

	params := &mcp.CallToolParams{
		Name:      e.remoteName,
		Arguments: argsDoc,
		Meta:      map[string]any{},
	}

	if toolContext.Session != nil {
		params.Meta["userId"] = toolContext.Session.AuthenticatedUserId
		params.Meta["sessionId"] = toolContext.Session.Id

	}
	response, err := e.mcpSession.CallTool(ctx, params)
	if err != nil {
		return err.Error()
	}
	var text = FormatResponseContent(response, e.suppressStructuredContent)
	if response.IsError {
		return "Error: " + text
	}
	return text
}

func FormatResponseContent(response *mcp.CallToolResult, suppressStructuredContent bool) string {
	parts := []string{}

	for _, item := range response.Content {
		switch a := item.(type) {
		case *mcp.TextContent:
			if a.Text != "" {
				parts = append(parts, a.Text)
			}

		case *mcp.EmbeddedResource:
			if a.Resource != nil && a.Resource.Text != "" {
				parts = append(parts, a.Resource.Text)
			}
		default:
			data, _ := json.Marshal(a)
			if len(data) > 0 {
				parts = append(parts, string(data))
			}
		}
	}

	if !suppressStructuredContent && response.StructuredContent != nil {
		data, _ := json.Marshal(response.StructuredContent)
		if len(data) > 0 {
			parts = append(parts, string(data))
		}
	}

	return strings.Join(parts, "\n\n")
}
