package client

import (
	"context"
	"net/url"
	"sync"

	"github.com/futugyou/mcp/core"
	"github.com/futugyou/mcp/shared"
)

var McpClientDefaultImplementation core.Implementation = core.Implementation{
	Name:    "McpClient",
	Version: "1.0.0",
}

var _ IMcpClient = (*McpClient)(nil)

type McpClient struct {
	*shared.BaseMcpEndpoint
	clientTransport  IClientTransport
	options          McpClientOptions
	sessionTransport core.ITransport
	reqHandlers      *shared.RequestHandlers
	notifHandlers    *shared.NotificationHandlers

	ctx        context.Context
	connectCts context.CancelFunc
	mu         sync.Mutex
	disposed   bool

	ServerCapabilities core.ServerCapabilities
	ServerInfo         core.Implementation
	ServerInstructions *string
	EndpointName       string
}

func CreateMcpClient(ctx context.Context, clientTransport IClientTransport, options McpClientOptions) (*McpClient, error) {
	client := NewMcpClient(clientTransport, options)
	err := client.Connect(ctx)
	if err != nil {
		client.Dispose(ctx)
		return nil, err
	}
	return client, nil
}

func NewMcpClient(clientTransport IClientTransport, options McpClientOptions) *McpClient {
	client := &McpClient{
		BaseMcpEndpoint: shared.NewBaseMcpEndpoint(),
		clientTransport: clientTransport,
		options:         options,
		EndpointName:    clientTransport.GetName(),
		reqHandlers:     shared.NewRequestHandlers(),
		notifHandlers:   shared.NewNotificationHandlers(),
	}

	capabilities := options.Capabilities
	if capabilities != nil {
		// notificationHandlers := capabilities.NotificationHandlers
		// if notificationHandlers != nil {
		// 	client.notifHandlers.RegisterRange(notificationHandlers)
		// }
		// samplingCapability := capabilities.Sampling
		// if samplingCapability != nil && samplingCapability.SamplingHandler != nil {
		// 	shared.GenericRequestHandlerAdd(
		// 		client.reqHandlers,
		// 		core.RequestMethods_SamplingCreateMessage,
		// 		func(ctx context.Context, request *core.CreateMessageRequestParams, tran core.ITransport) (*core.CreateMessageResult, error) {
		// 			var progres core.IProgressReporter = &shared.NullProgress{}
		// 			if request.Meta != nil && request.ProgressToken() != nil {
		// 				progres = shared.NewTokenProgress(client, *request.ProgressToken())
		// 			}
		// 			return samplingCapability.SamplingHandler(ctx, request, progres)
		// 		},
		// 		nil,
		// 		nil,
		// 	)
		// }

		// if capabilities.Roots != nil && capabilities.Roots.RootsHandler != nil {
		// 	shared.GenericRequestHandlerAdd(
		// 		client.reqHandlers,
		// 		core.RequestMethods_RootsList,
		// 		func(ctx context.Context, request *core.ListRootsRequestParams, tran core.ITransport) (*core.ListRootsResult, error) {
		// 			return capabilities.Roots.RootsHandler(ctx, request)
		// 		},
		// 		nil,
		// 		nil,
		// 	)
		// }

		// if capabilities.Elicitation != nil && capabilities.Elicitation.ElicitationHandler != nil {
		// 	shared.GenericRequestHandlerAdd(
		// 		client.reqHandlers,
		// 		core.RequestMethods_ElicitationCreate,
		// 		func(ctx context.Context, request *core.ElicitRequestParams, tran core.ITransport) (*core.ElicitResult, error) {
		// 			return capabilities.Elicitation.ElicitationHandler(ctx, request)
		// 		},
		// 		nil,
		// 		nil,
		// 	)
		// }
	}
	return client
}

func (e *McpClient) Dispose(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.disposed {
		return nil
	}
	e.disposed = true

	if e.connectCts != nil {
		e.connectCts()
	}

	defer e.sessionTransport.Close()

	return e.BaseMcpEndpoint.Dispose(ctx)
}

func (m *McpClient) Connect(ctx context.Context) error {
	// ctx, cancel := context.WithCancel(ctx)
	// m.ctx = ctx
	// m.connectCts = cancel
	// sessionTransport, err := m.clientTransport.Connect(ctx)
	// if err != nil {
	// 	return err
	// }
	// m.sessionTransport = sessionTransport

	// m.InitializeSession(sessionTransport, false)
	// // We don't want the ConnectAsync token to cancel the session after we've successfully connected.
	// // The base class handles cleaning up the session in DisposeAsync without our help.
	// m.StartSession(context.Background(), sessionTransport)
	// ctx, cancel = context.WithTimeout(ctx, m.options.InitializationTimeout)
	// defer cancel()

	// params := core.InitializeRequestParams{
	// 	ProtocolVersion: m.options.ProtocolVersion,
	// 	Capabilities:    m.options.Capabilities,
	// }

	// if m.options.ClientInfo != nil {
	// 	params.ClientInfo = *m.options.ClientInfo
	// }

	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_Initialize, params, nil)
	// initializeResponse, err := m.SendRequest(ctx, jsonRpcRequest)
	// if err != nil {
	// 	return err
	// }
	// var initializeResult server.InitializeResult
	// if err := json.Unmarshal(initializeResponse.Result, &initializeResult); err != nil {
	// 	return err
	// }

	// m.ServerCapabilities = initializeResult.Capabilities
	// m.ServerInfo = initializeResult.ServerInfo
	// m.ServerInstructions = &initializeResult.Instructions
	// return m.SendMessage(ctx, core.NewJsonRpcNotification(core.NotificationMethods_InitializedNotification, nil))
	return nil
}

// CallTool implements IMcpClient.
func (m *McpClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}, reporter any) (*core.CallToolResult, error) {
	// params := core.CallToolRequestParams{
	// 	RequestParams: core.RequestParams{},
	// 	Name:          toolName,
	// 	Arguments:     arguments,
	// }

	// if reporter != nil {
	// progressToken := core.NewProgressTokenFromString(uuid.New().String())
	// var handler core.NotificationHandler = func(ctx context.Context, notification *core.JsonRpcNotification) error {
	// 	var pn core.ProgressNotification
	// 	if err := json.Unmarshal(notification.Params, &pn); err != nil {
	// 		return err
	// 	}
	// 	if pn.ProgressToken != nil && *pn.ProgressToken == progressToken {
	// 		reporter.Report(*pn.Progress)
	// 	}
	// 	return nil
	// }
	// m.RegisterNotificationHandler(core.NotificationMethods_ProgressNotification, handler)
	// params.Meta = &core.RequestParamsMetadata{ProgressToken: &progressToken}
	// }

	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ToolsCall, params, nil)
	// resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// if err != nil {
	// 	return nil, err
	// }
	// var rsult core.CallToolResult
	// if err := json.Unmarshal(resp.Result, &rsult); err != nil {
	// 	return nil, err
	// }
	// return &rsult, nil
	return nil, nil
}

// Complete implements IMcpClient.
func (m *McpClient) Complete(ctx context.Context, reference core.Reference, argumentName string, argumentValue string) (*core.CompleteResult, error) {
	// params := core.CompleteRequestParams{
	// 	Ref: reference,
	// 	Argument: core.Argument{
	// 		Name:  argumentName,
	// 		Value: argumentValue,
	// 	},
	// }
	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_CompletionComplete, params, nil)
	// resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// if err != nil {
	// 	return nil, err
	// }
	// var rsult core.CompleteResult
	// if err := json.Unmarshal(resp.Result, &rsult); err != nil {
	// 	return nil, err
	// }
	// return &rsult, nil
	return nil, nil
}

// EnumeratePrompts implements IMcpClient.
func (m *McpClient) EnumeratePrompts(ctx context.Context, client IMcpClient) (<-chan McpClientPrompt, <-chan error) {
	promptsCh := make(chan McpClientPrompt)
	errCh := make(chan error, 1)

	// go func() {
	// 	defer close(promptsCh)
	// 	defer close(errCh)

	// 	var cursor *string
	// 	for {
	// 		params := core.ListPromptsRequestParams{
	// 			PaginatedRequestParams: core.PaginatedRequestParams{
	// 				RequestParams: core.RequestParams{},
	// 				Cursor:        cursor,
	// 			},
	// 		}
	// 		jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_PromptsList, params, nil)
	// 		resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 		if err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		var promptResults core.ListPromptsResult
	// 		if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		for _, prompt := range promptResults.Prompts {
	// 			select {
	// 			case <-ctx.Done():
	// 				errCh <- ctx.Err()
	// 				return
	// 			case promptsCh <- *NewMcpClientPrompt(prompt, m):
	// 			}
	// 		}

	// 		if promptResults.NextCursor == nil {
	// 			break
	// 		}
	// 		cursor = promptResults.NextCursor
	// 	}
	// }()

	return promptsCh, errCh
}

// EnumerateResourceTemplates implements IMcpClient.
func (m *McpClient) EnumerateResourceTemplates(ctx context.Context, client IMcpClient) (<-chan McpClientResourceTemplate, <-chan error) {
	promptsCh := make(chan McpClientResourceTemplate)
	errCh := make(chan error, 1)

	// go func() {
	// 	defer close(promptsCh)
	// 	defer close(errCh)

	// 	var cursor *string
	// 	for {
	// 		params := core.ListResourceTemplatesRequestParams{
	// 			PaginatedRequestParams: core.PaginatedRequestParams{
	// 				RequestParams: core.RequestParams{},
	// 				Cursor:        cursor,
	// 			},
	// 		}
	// 		jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ResourcesTemplatesList, params, nil)
	// 		resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 		if err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		var promptResults core.ListResourceTemplatesResult
	// 		if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		for _, prompt := range promptResults.ResourceTemplates {
	// 			select {
	// 			case <-ctx.Done():
	// 				errCh <- ctx.Err()
	// 				return
	// 			default:
	// 				t := NewMcpClientResourceTemplate(m, prompt)
	// 				promptsCh <- *t
	// 			}
	// 		}

	// 		if promptResults.NextCursor == nil {
	// 			break
	// 		}
	// 		cursor = promptResults.NextCursor
	// 	}
	// }()

	return promptsCh, errCh
}

// EnumerateResources implements IMcpClient.
func (m *McpClient) EnumerateResources(ctx context.Context, client IMcpClient) (<-chan McpClientResource, <-chan error) {
	promptsCh := make(chan McpClientResource)
	errCh := make(chan error, 1)

	// go func() {
	// 	defer close(promptsCh)
	// 	defer close(errCh)

	// 	var cursor *string
	// 	for {
	// 		params := core.ListResourcesRequestParams{
	// 			PaginatedRequestParams: core.PaginatedRequestParams{
	// 				RequestParams: core.RequestParams{},
	// 				Cursor:        cursor,
	// 			},
	// 		}
	// 		jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ResourcesList, params, nil)
	// 		resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 		if err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		var promptResults core.ListResourcesResult
	// 		if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		for _, prompt := range promptResults.Resources {
	// 			select {
	// 			case <-ctx.Done():
	// 				errCh <- ctx.Err()
	// 				return
	// 			default:
	// 				t := NewMcpClientResource(m, prompt)
	// 				promptsCh <- *t
	// 			}
	// 		}

	// 		if promptResults.NextCursor == nil {
	// 			break
	// 		}
	// 		cursor = promptResults.NextCursor
	// 	}
	// }()

	return promptsCh, errCh
}

// EnumerateTools implements IMcpClient.
func (m *McpClient) EnumerateTools(ctx context.Context) (<-chan McpClientTool, <-chan error) {
	promptsCh := make(chan McpClientTool)
	errCh := make(chan error, 1)

	// go func() {
	// 	defer close(promptsCh)
	// 	defer close(errCh)

	// 	var cursor *string
	// 	for {
	// 		params := core.ListToolsRequestParams{
	// 			PaginatedRequestParams: core.PaginatedRequestParams{
	// 				RequestParams: core.RequestParams{},
	// 				Cursor:        cursor,
	// 			},
	// 		}
	// 		jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ToolsList, params, nil)
	// 		resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 		if err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		var promptResults core.ListToolsResult
	// 		if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 			errCh <- err
	// 			return
	// 		}

	// 		for _, prompt := range promptResults.Tools {
	// 			select {
	// 			case <-ctx.Done():
	// 				errCh <- ctx.Err()
	// 				return
	// 			case promptsCh <- *NewMcpClientTool(m, prompt.Name, prompt.Description, prompt):
	// 			}
	// 		}

	// 		if promptResults.NextCursor == nil {
	// 			break
	// 		}
	// 		cursor = promptResults.NextCursor
	// 	}
	// }()

	return promptsCh, errCh
}

// GetPrompt implements IMcpClient.
func (m *McpClient) GetPrompt(ctx context.Context, name string, arguments map[string]interface{}) (*core.GetPromptResult, error) {
	// params := core.GetPromptRequestParams{
	// 	RequestParams: core.RequestParams{},
	// 	Name:          name,
	// 	Arguments:     arguments,
	// }
	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_PromptsGet, params, nil)
	// resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// if err != nil {
	// 	return nil, err
	// }
	// var rsult core.GetPromptResult
	// if err := json.Unmarshal(resp.Result, &rsult); err != nil {
	// 	return nil, err
	// }
	// return &rsult, nil

	return nil, nil
}

// GetServerCapabilities implements IMcpClient.
func (m *McpClient) GetServerCapabilities() *core.ServerCapabilities {
	return &m.ServerCapabilities
}

// GetServerInfo implements IMcpClient.
func (m *McpClient) GetServerInfo() *core.Implementation {
	return &m.ServerInfo
}

// GetServerInstructions implements IMcpClient.
func (m *McpClient) GetServerInstructions() *string {
	return m.ServerInstructions
}

// ListPrompts implements IMcpClient.
func (m *McpClient) ListPrompts(ctx context.Context, client IMcpClient) ([]McpClientPrompt, error) {
	prompts := []McpClientPrompt{}
	// var cursor *string
	// for {
	// 	params := core.ListPromptsRequestParams{
	// 		PaginatedRequestParams: core.PaginatedRequestParams{
	// 			RequestParams: core.RequestParams{},
	// 			Cursor:        cursor,
	// 		},
	// 	}
	// 	jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_PromptsList, params, nil)
	// 	resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	var promptResults core.ListPromptsResult
	// 	if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 		return nil, err
	// 	}

	// 	for _, prompt := range promptResults.Prompts {
	// 		prompts = append(prompts, *NewMcpClientPrompt(prompt, m))
	// 	}

	// 	if promptResults.NextCursor == nil {
	// 		break
	// 	}
	// 	cursor = promptResults.NextCursor
	// }
	return prompts, nil
}

// ListResourceTemplates implements IMcpClient.
func (m *McpClient) ListResourceTemplates(ctx context.Context, client IMcpClient) ([]McpClientResourceTemplate, error) {
	prompts := []McpClientResourceTemplate{}
	// var cursor *string
	// for {
	// 	params := core.ListResourceTemplatesRequestParams{
	// 		PaginatedRequestParams: core.PaginatedRequestParams{
	// 			RequestParams: core.RequestParams{},
	// 			Cursor:        cursor,
	// 		},
	// 	}
	// 	jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ResourcesTemplatesList, params, nil)
	// 	resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	var promptResults core.ListResourceTemplatesResult
	// 	if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 		return nil, err
	// 	}

	// 	for _, v := range promptResults.ResourceTemplates {
	// 		t := NewMcpClientResourceTemplate(m, v)
	// 		prompts = append(prompts, *t)
	// 	}

	// 	if promptResults.NextCursor == nil {
	// 		break
	// 	}
	// 	cursor = promptResults.NextCursor
	// }
	return prompts, nil
}

// ListResources implements IMcpClient.
func (m *McpClient) ListResources(ctx context.Context, client IMcpClient) ([]McpClientResource, error) {
	prompts := []McpClientResource{}
	// var cursor *string
	// for {
	// 	params := core.ListResourcesRequestParams{
	// 		PaginatedRequestParams: core.PaginatedRequestParams{
	// 			RequestParams: core.RequestParams{},
	// 			Cursor:        cursor,
	// 		},
	// 	}
	// 	jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ResourcesList, params, nil)
	// 	resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	var promptResults core.ListResourcesResult
	// 	if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 		return nil, err
	// 	}

	// 	for _, v := range promptResults.Resources {
	// 		t := NewMcpClientResource(m, v)
	// 		prompts = append(prompts, *t)
	// 	}

	// 	if promptResults.NextCursor == nil {
	// 		break
	// 	}
	// 	cursor = promptResults.NextCursor
	// }
	return prompts, nil
}

// ListTools implements IMcpClient.
func (m *McpClient) ListTools(ctx context.Context) ([]McpClientTool, error) {
	prompts := []McpClientTool{}
	// var cursor *string
	// for {
	// 	params := core.ListToolsRequestParams{
	// 		PaginatedRequestParams: core.PaginatedRequestParams{
	// 			RequestParams: core.RequestParams{},
	// 			Cursor:        cursor,
	// 		},
	// 	}
	// 	jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ToolsList, params, nil)
	// 	resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	var promptResults core.ListToolsResult
	// 	if err := json.Unmarshal(resp.Result, &promptResults); err != nil {
	// 		return nil, err
	// 	}

	// 	for _, v := range promptResults.Tools {
	// 		prompts = append(prompts, *NewMcpClientTool(m, v.Name, v.Description, v))
	// 	}

	// 	if promptResults.NextCursor == nil {
	// 		break
	// 	}
	// 	cursor = promptResults.NextCursor
	// }
	return prompts, nil
}

// Ping implements IMcpClient.
func (m *McpClient) Ping(ctx context.Context) error {
	// params := core.PingRequest{}
	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_Ping, params, nil)
	// _, err := m.SendRequest(ctx, jsonRpcRequest)
	// return err
	return nil
}

// ReadResource implements IMcpClient.
func (m *McpClient) ReadResource(ctx context.Context, uri string) (*core.ReadResourceResult, error) {
	// params := core.ReadResourceRequestParams{
	// 	RequestParams: core.RequestParams{},
	// 	Uri:           uri,
	// }
	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ResourcesRead, params, nil)
	// resp, err := m.SendRequest(ctx, jsonRpcRequest)
	// if err != nil {
	// 	return nil, err
	// }
	// var rsult core.ReadResourceResult
	// if err := json.Unmarshal(resp.Result, &rsult); err != nil {
	// 	return nil, err
	// }
	// return &rsult, nil

	return nil, nil
}

// ReadResourceWithUri implements IMcpClient.
func (m *McpClient) ReadResourceWithUri(ctx context.Context, uri url.URL) (*core.ReadResourceResult, error) {
	return m.ReadResource(ctx, uri.String())
}

// ReadResourceWithUriAndArguments implements IMcpClient.
func (m *McpClient) ReadResourceWithUriAndArguments(ctx context.Context, uriTemplate string, arguments map[string]interface{}) (*core.ReadResourceResult, error) {
	url, err := shared.FormatUri(uriTemplate, arguments)
	if err != nil {
		return nil, err
	}

	return m.ReadResource(ctx, url)
}

// SetLoggingLevel implements IMcpClient.
// func (m *McpClient) SetLoggingLevel(ctx context.Context, level core.LoggingLevel) error {
// params := core.SetLevelRequestParams{
// 	RequestParams: core.RequestParams{},
// 	Level:         level,
// }
// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_LoggingSetLevel, params, nil)
// _, err := m.SendRequest(ctx, jsonRpcRequest)
// return err
// 	return nil
// }

// SetLoggingLevelWithLogLevel implements IMcpClient.
// func (m *McpClient) SetLoggingLevelWithLogLevel(ctx context.Context, level logger.LogLevel) error {
// 	return m.SetLoggingLevel(ctx, core.LoggingLevel(level))
// }

// SubscribeToResource implements IMcpClient.
func (m *McpClient) SubscribeToResource(ctx context.Context, uri string) error {
	// params := core.SubscribeRequestParams{
	// 	RequestParams: core.RequestParams{},
	// 	Uri:           &uri,
	// }
	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ResourcesSubscribe, params, nil)
	// _, err := m.SendRequest(ctx, jsonRpcRequest)
	// return err
	return nil
}

// SubscribeToResourceWithUri implements IMcpClient.
func (m *McpClient) SubscribeToResourceWithUri(ctx context.Context, uri url.URL) error {
	return m.SubscribeToResource(ctx, uri.String())
}

// UnsubscribeFromResource implements IMcpClient.
func (m *McpClient) UnsubscribeFromResource(ctx context.Context, uri string) error {
	// params := core.UnsubscribeRequestParams{
	// 	RequestParams: core.RequestParams{},
	// 	Uri:           &uri,
	// }
	// jsonRpcRequest := core.NewJsonRpcRequest(core.RequestMethods_ResourcesUnsubscribe, params, nil)
	// _, err := m.SendRequest(ctx, jsonRpcRequest)
	// return err
	return nil
}

// UnsubscribeFromResourceWithUri implements IMcpClient.
func (m *McpClient) UnsubscribeFromResourceWithUri(ctx context.Context, uri url.URL) error {
	return m.UnsubscribeFromResource(ctx, uri.String())
}
