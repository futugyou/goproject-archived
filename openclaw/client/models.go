package client

import "encoding/json"

type McpJsonRpcRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r *McpJsonRpcRequest) UnmarshalJSON(data []byte) error {
	type Alias McpJsonRpcRequest
	aux := &Alias{
		Jsonrpc: "2.0",
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	*r = McpJsonRpcRequest(*aux)
	return nil
}

type McpJsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type McpJsonRpcResponse struct {
	Jsonrpc string           `json:"jsonrpc"`
	Id      json.RawMessage  `json:"id,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *McpJsonRpcError `json:"error,omitempty"`
}

func (r *McpJsonRpcResponse) UnmarshalJSON(data []byte) error {
	type Alias McpJsonRpcResponse
	aux := &Alias{
		Jsonrpc: "2.0",
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	*r = McpJsonRpcResponse(*aux)
	return nil
}

type McpInitializeRequest struct {
	ProtocolVersion *string               `json:"protocolVersion,omitempty"`
	Capabilities    McpClientCapabilities `json:"capabilities"`
	ClientInfo      McpClientInfo         `json:"clientInfo"`
}

func (r *McpInitializeRequest) UnmarshalJSON(data []byte) error {
	type Alias McpInitializeRequest
	aux := &Alias{
		ClientInfo: McpClientInfo{
			Name:    "openclaw-client",
			Version: "1.0.0",
		},
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	*r = McpInitializeRequest(*aux)
	return nil
}

type McpClientCapabilities struct{}

type McpClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type McpInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    McpCapabilities `json:"capabilities"`
	ServerInfo      McpServerInfo   `json:"serverInfo"`
}

func (r *McpInitializeResult) UnmarshalJSON(data []byte) error {
	type Alias McpInitializeResult
	aux := &Alias{
		ProtocolVersion: "2025-03-26",
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	*r = McpInitializeResult(*aux)
	return nil
}

type McpDiscoverRequest struct{}

type McpDiscoverResult struct {
	ProtocolVersion   string          `json:"protocolVersion"`
	SupportedVersions []string        `json:"supportedVersions"`
	Capabilities      json.RawMessage `json:"capabilities,omitempty"`
	ServerInfo        *McpServerInfo  `json:"serverInfo,omitempty"`
}

type McpCapabilities struct {
	Tools     McpToolCapabilities     `json:"tools"`
	Resources McpResourceCapabilities `json:"resources"`
	Prompts   McpPromptCapabilities   `json:"prompts"`
}

type McpToolCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

type McpResourceCapabilities struct {
	ListChanged       bool `json:"listChanged"`
	SupportsTemplates bool `json:"supportsTemplates"`
}

type McpPromptCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

type McpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type McpCallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type McpToolDefinition struct {
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type McpToolListResult struct {
	Tools []McpToolDefinition `json:"tools"`
}

type McpTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type McpCallToolResult struct {
	Content           []McpTextContent `json:"content"`
	StructuredContent json.RawMessage  `json:"structuredContent,omitempty"`
	IsError           bool             `json:"isError"`
}

type McpResourceDefinition struct {
	Uri         string  `json:"uri"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	MimeType    string  `json:"mimeType"`
}

func (r *McpResourceDefinition) UnmarshalJSON(data []byte) error {
	type Alias McpResourceDefinition
	aux := &Alias{
		MimeType: "application/json",
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	*r = McpResourceDefinition(*aux)
	return nil
}

type McpResourceListResult struct {
	Resources []McpResourceDefinition `json:"resources"`
}

type McpResourceTemplateDefinition struct {
	UriTemplate string  `json:"uriTemplate"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	MimeType    string  `json:"mimeType"`
}

func (r *McpResourceTemplateDefinition) UnmarshalJSON(data []byte) error {
	type Alias McpResourceTemplateDefinition
	aux := &Alias{
		MimeType: "application/json",
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	*r = McpResourceTemplateDefinition(*aux)
	return nil
}

type McpResourceTemplateListResult struct {
	ResourceTemplates []McpResourceTemplateDefinition `json:"resourceTemplates"`
}

type McpReadResourceRequest struct {
	Uri string `json:"uri"`
}

type McpResourceTextContents struct {
	Uri      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

func (r *McpResourceTextContents) UnmarshalJSON(data []byte) error {
	type Alias McpResourceTextContents
	aux := &Alias{
		MimeType: "application/json",
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	*r = McpResourceTextContents(*aux)
	return nil
}

type McpReadResourceResult struct {
	Contents []McpResourceTextContents `json:"contents"`
}

type McpPromptDefinition struct {
	Name        string                        `json:"name"`
	Description *string                       `json:"description,omitempty"`
	Arguments   []McpPromptArgumentDefinition `json:"arguments"`
}

type McpPromptArgumentDefinition struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Required    bool    `json:"required"`
}

type McpPromptListResult struct {
	Prompts []McpPromptDefinition `json:"prompts"`
}

type McpGetPromptRequest struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

type McpPromptMessage struct {
	Role    string         `json:"role"`
	Content McpTextContent `json:"content"`
}

type McpGetPromptResult struct {
	Description *string            `json:"description,omitempty"`
	Messages    []McpPromptMessage `json:"messages"`
}
