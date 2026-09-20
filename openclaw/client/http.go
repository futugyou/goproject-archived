package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type authTransport struct {
	authToken string
	base      http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", "openclaw-client/1.0")
	if t.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.authToken)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

const (
	LatestMcpProtocolVersion       = "2026-07-28"
	LatestLegacyMcpProtocolVersion = "2025-11-25"
	McpProtocolVersionMetaKey      = "io.modelcontextprotocol/protocolVersion"
	McpClientInfoMetaKey           = "io.modelcontextprotocol/clientInfo"
	McpClientCapabilitiesMetaKey   = "io.modelcontextprotocol/clientCapabilities"
)

type OpenClawHttpClient struct {
	baseUri    *url.URL
	httpClient *http.Client

	mcpClient  *mcp.Client
	mcpSession *mcp.ClientSession

	authToken                    string
	negotiatedMcpProtocolVersion string
	mcpRequestId                 atomic.Int64

	// Endpoint URIs
	chatCompletionsUri                 *url.URL
	mcpUri                             *url.URL
	authSessionUri                     *url.URL
	integrationDashboardUri            *url.URL
	integrationStatusUri               *url.URL
	integrationApprovalsUri            *url.URL
	integrationApprovalHistoryUri      *url.URL
	integrationProvidersUri            *url.URL
	integrationPluginsUri              *url.URL
	integrationCompatibilityCatalogUri *url.URL
	integrationOperatorAuditUri        *url.URL
	integrationAccountsUri             *url.URL
	integrationBackendsUri             *url.URL
	integrationSessionsUri             *url.URL
	integrationSessionSearchUri        *url.URL
	integrationProfilesUri             *url.URL
	integrationToolPresetsUri          *url.URL
	integrationWorkflowsUri            *url.URL
	integrationAutomationsUri          *url.URL
	integrationRuntimeEventsUri        *url.URL
	integrationMessagesUri             *url.URL
	integrationPaymentSetupUri         *url.URL
	integrationPaymentFundingUri       *url.URL
	integrationPaymentVirtualCardUri   *url.URL
	integrationPaymentExecuteUri       *url.URL
	integrationPaymentStatusUri        *url.URL
	adminAutomationsUri                *url.URL
	adminLearningProposalsUri          *url.URL
	adminMemoryNotesUri                *url.URL
	adminMemorySearchUri               *url.URL
	adminMemoryExportUri               *url.URL
	adminMemoryImportUri               *url.URL
	adminMemoryFractalStatusUri        *url.URL
	adminMemoryFractalSearchUri        *url.URL
	adminMemoryFractalOpenUri          *url.URL
	adminMemoryFractalExportUri        *url.URL
	adminMemoryFractalRecentUri        *url.URL
	adminMemoryFractalValidateUri      *url.URL
	adminMemoryFractalIndexRefreshUri  *url.URL
	adminMemoryFractalHandoffUri       *url.URL
	adminHarnessSharedStateUri         *url.URL
	adminAgentBundleExportUri          *url.URL
	adminAgentBundleImportUri          *url.URL
	adminHeartbeatUri                  *url.URL
	adminHeartbeatPreviewUri           *url.URL
	adminHeartbeatStatusUri            *url.URL
	adminPulseStatusUri                *url.URL
	adminPulseRunUri                   *url.URL
	adminPulseEventsUri                *url.URL
	adminPulseEnableUri                *url.URL
	adminPulseDisableUri               *url.URL
	adminPostureUri                    *url.URL
	adminModelsUri                     *url.URL
	adminModelsDoctorUri               *url.URL
	adminModelEvaluationsUri           *url.URL
	adminExternalCliConnectorsUri      *url.URL
	adminExternalCliPreviewUri         *url.URL
	adminExternalCliExecuteUri         *url.URL
	adminApprovalSimulationUri         *url.URL
	toolsApproveUri                    *url.URL
	adminAccountResolutionUri          *url.URL
	adminBackendsUri                   *url.URL
	adminIncidentExportUri             *url.URL
	authOperatorTokenUri               *url.URL
	adminOperatorAccountsUri           *url.URL
	adminOrganizationPolicyUri         *url.URL
	adminSetupStatusUri                *url.URL
	adminInsightsUri                   *url.URL
	adminObservabilitySummaryUri       *url.URL
	adminObservabilitySeriesUri        *url.URL
	adminAuditExportUri                *url.URL
	adminTrajectoryExportUri           *url.URL
	adminWhatsAppSetupUri              *url.URL
	adminWhatsAppRestartUri            *url.URL
}

func NewOpenClawHttpClient(baseUrl string, authToken string, customHTTPClient *http.Client) (*OpenClawHttpClient, error) {
	if strings.TrimSpace(baseUrl) == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	normalized := strings.TrimRight(baseUrl, "/")
	baseUri, err := url.Parse(normalized)
	if err != nil || !baseUri.IsAbs() {
		return nil, fmt.Errorf("invalid base URL: %s", baseUrl)
	}

	baseClient := customHTTPClient
	if baseClient == nil {
		baseClient = &http.Client{
			Timeout: 0,
		}
	}

	httpClient := &http.Client{
		Transport: &authTransport{
			authToken: authToken,
			base:      baseClient.Transport,
		},
		Jar:     baseClient.Jar,
		Timeout: baseClient.Timeout,
	}

	c := &OpenClawHttpClient{
		baseUri:    baseUri,
		httpClient: httpClient,
		authToken:  authToken,

		chatCompletionsUri:                 baseUri.JoinPath("/v1/chat/completions"),
		mcpUri:                             baseUri.JoinPath("/mcp"),
		authSessionUri:                     baseUri.JoinPath("/auth/session"),
		integrationDashboardUri:            baseUri.JoinPath("/api/integration/dashboard"),
		integrationStatusUri:               baseUri.JoinPath("/api/integration/status"),
		integrationApprovalsUri:            baseUri.JoinPath("/api/integration/approvals"),
		integrationApprovalHistoryUri:      baseUri.JoinPath("/api/integration/approval-history"),
		integrationProvidersUri:            baseUri.JoinPath("/api/integration/providers"),
		integrationPluginsUri:              baseUri.JoinPath("/api/integration/plugins"),
		integrationCompatibilityCatalogUri: baseUri.JoinPath("/api/integration/compatibility/catalog"),
		integrationOperatorAuditUri:        baseUri.JoinPath("/api/integration/operator-audit"),
		integrationAccountsUri:             baseUri.JoinPath("/api/integration/accounts"),
		integrationBackendsUri:             baseUri.JoinPath("/api/integration/backends"),
		integrationSessionsUri:             baseUri.JoinPath("/api/integration/sessions"),
		integrationSessionSearchUri:        baseUri.JoinPath("/api/integration/session-search"),
		integrationProfilesUri:             baseUri.JoinPath("/api/integration/profiles"),
		integrationToolPresetsUri:          baseUri.JoinPath("/api/integration/tool-presets"),
		integrationWorkflowsUri:            baseUri.JoinPath("/api/integration/workflows"),
		integrationAutomationsUri:          baseUri.JoinPath("/api/integration/automations"),
		integrationRuntimeEventsUri:        baseUri.JoinPath("/api/integration/runtime-events"),
		integrationMessagesUri:             baseUri.JoinPath("/api/integration/messages"),
		integrationPaymentSetupUri:         baseUri.JoinPath("/api/integration/payment/setup"),
		integrationPaymentFundingUri:       baseUri.JoinPath("/api/integration/payment/funding"),
		integrationPaymentVirtualCardUri:   baseUri.JoinPath("/api/integration/payment/virtual-card"),
		integrationPaymentExecuteUri:       baseUri.JoinPath("/api/integration/payment/execute"),
		integrationPaymentStatusUri:        baseUri.JoinPath("/api/integration/payment/status/"),
		adminAutomationsUri:                baseUri.JoinPath("/admin/automations"),
		adminLearningProposalsUri:          baseUri.JoinPath("/admin/learning/proposals"),
		adminMemoryNotesUri:                baseUri.JoinPath("/admin/memory/notes"),
		adminMemorySearchUri:               baseUri.JoinPath("/admin/memory/search"),
		adminMemoryExportUri:               baseUri.JoinPath("/admin/memory/export"),
		adminMemoryImportUri:               baseUri.JoinPath("/admin/memory/import"),
		adminMemoryFractalStatusUri:        baseUri.JoinPath("/admin/memory/fractal/status"),
		adminMemoryFractalSearchUri:        baseUri.JoinPath("/admin/memory/fractal/search"),
		adminMemoryFractalOpenUri:          baseUri.JoinPath("/admin/memory/fractal/open"),
		adminMemoryFractalExportUri:        baseUri.JoinPath("/admin/memory/fractal/export"),
		adminMemoryFractalRecentUri:        baseUri.JoinPath("/admin/memory/fractal/recent"),
		adminMemoryFractalValidateUri:      baseUri.JoinPath("/admin/memory/fractal/validate"),
		adminMemoryFractalIndexRefreshUri:  baseUri.JoinPath("/admin/memory/fractal/index/refresh"),
		adminMemoryFractalHandoffUri:       baseUri.JoinPath("/admin/memory/fractal/handoff"),
		adminHarnessSharedStateUri:         baseUri.JoinPath("/admin/harness/shared-state"),
		adminAgentBundleExportUri:          baseUri.JoinPath("/admin/agent-bundle/export"),
		adminAgentBundleImportUri:          baseUri.JoinPath("/admin/agent-bundle/import"),
		adminHeartbeatUri:                  baseUri.JoinPath("/admin/heartbeat"),
		adminHeartbeatPreviewUri:           baseUri.JoinPath("/admin/heartbeat/preview"),
		adminHeartbeatStatusUri:            baseUri.JoinPath("/admin/heartbeat/status"),
		adminPulseStatusUri:                baseUri.JoinPath("/admin/pulse/status"),
		adminPulseRunUri:                   baseUri.JoinPath("/admin/pulse/run"),
		adminPulseEventsUri:                baseUri.JoinPath("/admin/pulse/events"),
		adminPulseEnableUri:                baseUri.JoinPath("/admin/pulse/enable"),
		adminPulseDisableUri:               baseUri.JoinPath("/admin/pulse/disable"),
		adminPostureUri:                    baseUri.JoinPath("/admin/posture"),
		adminModelsUri:                     baseUri.JoinPath("/admin/models"),
		adminModelsDoctorUri:               baseUri.JoinPath("/admin/models/doctor"),
		adminModelEvaluationsUri:           baseUri.JoinPath("/admin/models/evaluations"),
		adminExternalCliConnectorsUri:      baseUri.JoinPath("/admin/external-cli/connectors"),
		adminExternalCliPreviewUri:         baseUri.JoinPath("/admin/external-cli/preview"),
		adminExternalCliExecuteUri:         baseUri.JoinPath("/admin/external-cli/execute"),
		adminApprovalSimulationUri:         baseUri.JoinPath("/admin/approvals/simulate"),
		toolsApproveUri:                    baseUri.JoinPath("/tools/approve"),
		adminAccountResolutionUri:          baseUri.JoinPath("/admin/accounts/test-resolution"),
		adminBackendsUri:                   baseUri.JoinPath("/admin/backends"),
		adminIncidentExportUri:             baseUri.JoinPath("/admin/incident/export"),
		authOperatorTokenUri:               baseUri.JoinPath("/auth/operator-token"),
		adminOperatorAccountsUri:           baseUri.JoinPath("/admin/operator-accounts"),
		adminOrganizationPolicyUri:         baseUri.JoinPath("/admin/organization-policy"),
		adminSetupStatusUri:                baseUri.JoinPath("/admin/setup/status"),
		adminInsightsUri:                   baseUri.JoinPath("/admin/insights"),
		adminObservabilitySummaryUri:       baseUri.JoinPath("/admin/observability/summary"),
		adminObservabilitySeriesUri:        baseUri.JoinPath("/admin/observability/series"),
		adminAuditExportUri:                baseUri.JoinPath("/admin/audit/export"),
		adminTrajectoryExportUri:           baseUri.JoinPath("/admin/trajectory/export"),
		adminWhatsAppSetupUri:              baseUri.JoinPath("/admin/channels/whatsapp/setup"),
		adminWhatsAppRestartUri:            baseUri.JoinPath("/admin/channels/whatsapp/restart"),
	}

	mcpTransport := &mcp.SSEClientTransport{
		Endpoint:   c.mcpUri.String(),
		HTTPClient: httpClient,
	}

	mcpImpl := &mcp.Implementation{
		Name:    "openclaw-client",
		Version: "1.0.0",
	}

	c.mcpClient = mcp.NewClient(mcpImpl, &mcp.ClientOptions{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := c.mcpClient.Connect(ctx, mcpTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to establish MCP connection: %w", err)
	}
	c.mcpSession = session

	return c, nil
}

func (c *OpenClawHttpClient) newRequest(method string, targetUrl *url.URL) (*http.Request, error) {
	req, err := http.NewRequest(method, targetUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "openclaw-client/1.0")
	if strings.TrimSpace(c.authToken) != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	return req, nil
}

func (c *OpenClawHttpClient) nextRequestId() int64 {
	return c.mcpRequestId.Add(1)
}

func (c *OpenClawHttpClient) resolveMcpProtocolVersion(method string, parameters any) string {
	switch p := parameters.(type) {
	case McpInitializeRequest:
		if p.ProtocolVersion != nil && len(*p.ProtocolVersion) > 0 {
			return *p.ProtocolVersion
		}
	}

	if len(c.negotiatedMcpProtocolVersion) > 0 {
		return c.negotiatedMcpProtocolVersion
	}

	if strings.HasPrefix(method, "server/discover") {
		return LatestMcpProtocolVersion
	}

	return LatestLegacyMcpProtocolVersion
}

func SendHttp[TParams any, TResult any](
	ctx context.Context,
	c *OpenClawHttpClient,
	method string,
	url *url.URL,
	parameters *TParams,
	header map[string]string,
) (*TResult, error) {
	var data []byte
	var err error

	if parameters != nil {
		data, err = json.Marshal(parameters)
	}

	if err != nil {
		return nil, err
	}

	var request *http.Request
	if len(data) > 0 {
		request, err = http.NewRequestWithContext(ctx, method, url.String(), bytes.NewBuffer(data))
	} else {
		request, err = http.NewRequestWithContext(ctx, method, url.String(), nil)
	}

	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		request.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Error: Failed to search (HTTP %d)", resp.StatusCode)
	}

	var root TResult
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, err
	}

	return &root, nil
}

// discoverRes, err := SendMcp[any, DiscoverResult](ctx, client, "server/discover", nil)
//
// toolRes, err := SendMcp[mcp.CallToolRequest, mcp.CallToolResult](
//
//	ctx,
//	client,
//	"tools/call",
//	&mcp.CallToolRequest{
//	    Name: "calculator",
//	    Arguments: map[string]any{"a": 1, "b": 2},
//	},
//
// )
func SendMcp[TParams any, TResult any](
	ctx context.Context,
	c *OpenClawHttpClient,
	method string,
	parameters *TParams,
) (*TResult, error) {
	protocolVersion := c.resolveMcpProtocolVersion(method, parameters)

	paramsObj, err := c.buildMcpParams(parameters, protocolVersion)
	if err != nil {
		return nil, &JsonError{Op: "build_params", Err: err}
	}

	requestPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("%d", c.nextRequestId()),
		"method":  method,
		"params":  paramsObj,
	}

	bodyBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, &JsonError{Op: "marshal_payload", Err: err}
	}

	// 构造 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.mcpUri.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Accept", "text/event-stream")
	req.Header.Set("mcp-protocol-version", protocolVersion)
	req.Header.Set("Mcp-Method", method)

	if mcpName, ok := tryResolveMcpName(method, parameters); ok {
		req.Header.Set("Mcp-Name", mcpName)
	}

	// 发送 HTTP 请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err // 原生网络错误 (如 context cancelled, DNS 失败等)
	}
	defer resp.Body.Close()

	// HTTP 状态码非 2xx 检查 (抛出 HttpError)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &HttpError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(bodyBytes),
		}
	}

	// 提取响应内容 (SSE 或 普通 JSON)
	jsonBody, err := extractMcpResponseJson(resp)
	if err != nil {
		return nil, err
	}

	// 解析 JSON-RPC 外壳
	var envelope McpJsonRpcResponse
	if err := json.Unmarshal([]byte(jsonBody), &envelope); err != nil {
		return nil, &JsonError{Op: "unmarshal_envelope", Err: err}
	}

	// MCP 协议层面的 JSON-RPC 错误处理 (抛出 McpProtocolError)
	if envelope.Error != nil {
		rpcCode := envelope.Error.Code
		return nil, &McpProtocolError{
			StatusCode: resp.StatusCode,
			RpcCode:    &rpcCode,
			Message:    envelope.Error.Message,
		}
	}

	if len(envelope.Result) == 0 {
		return nil, &McpProtocolError{
			StatusCode: resp.StatusCode,
			RpcCode:    nil,
			Message:    "MCP response did not include a result payload",
		}
	}

	// 解析最终结果 payload
	var result TResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return nil, &JsonError{Op: "unmarshal_result", Err: err}
	}

	return &result, nil
}

func (c *OpenClawHttpClient) buildMcpParams(parameters any, protocolVersion string) (map[string]any, error) {
	if protocolVersion < LatestMcpProtocolVersion {
		if parameters == nil {
			return map[string]any{}, nil
		}
		data, err := json.Marshal(parameters)
		if err != nil {
			return nil, err
		}
		var m map[string]any
		json.Unmarshal(data, &m)
		return m, nil
	}

	paramsMap := make(map[string]any)
	if parameters != nil {
		data, err := json.Marshal(parameters)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(data, &paramsMap)
	}

	// 提取或初始化 _meta 字典
	metaMap := make(map[string]any)
	if existingMeta, ok := paramsMap["_meta"].(map[string]any); ok {
		for k, v := range existingMeta {
			if k != McpProtocolVersionMetaKey && k != McpClientInfoMetaKey && k != McpClientCapabilitiesMetaKey {
				metaMap[k] = v
			}
		}
	}

	// 写入元数据
	metaMap[McpProtocolVersionMetaKey] = protocolVersion
	metaMap[McpClientInfoMetaKey] = map[string]string{
		"name":    "openclaw-client",
		"version": "1.0.0",
	}
	metaMap[McpClientCapabilitiesMetaKey] = map[string]any{}

	paramsMap["_meta"] = metaMap
	return paramsMap, nil
}

func extractMcpResponseJson(resp *http.Response) (string, error) {
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "data:")), nil
			}
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("SSE response did not contain a data line")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func tryResolveMcpName(method string, parameters any) (string, bool) {
	switch method {
	case "tools/call":
		if req, ok := parameters.(*mcp.CallToolRequest); ok && req.Params != nil && req.Params.Name != "" {
			return req.Params.Name, true
		}
	case "resources/read":
		if req, ok := parameters.(*mcp.ReadResourceRequest); ok && req.Params != nil && req.Params.URI != "" {
			return req.Params.URI, true
		}
	case "prompts/get":
		if req, ok := parameters.(*mcp.GetPromptRequest); ok && req.Params != nil && req.Params.Name != "" {
			return req.Params.Name, true
		}
	}
	return "", false
}

func (c *OpenClawHttpClient) DiscoverMcp(ctx context.Context) (*McpDiscoverResult, error) {
	result, err := SendMcp[McpDiscoverRequest, McpDiscoverResult](ctx, c, "server/discover", &McpDiscoverRequest{})
	if err != nil {
		if mcpErr, ok := errors.AsType[*McpProtocolError](err); ok {
			isNotFound := mcpErr.StatusCode == http.StatusNotFound
			isMethodNotFound := mcpErr.RpcCode != nil && *mcpErr.RpcCode == -32601

			if isNotFound || isMethodNotFound {
				return c.discoverLegacyMcp(ctx)
			}
		}

		if httpErr, ok := errors.AsType[*HttpError](err); ok && httpErr.StatusCode == http.StatusNotFound {
			return c.discoverLegacyMcp(ctx)
		}

		return nil, err
	}

	if len(result.ProtocolVersion) > 0 {
		c.negotiatedMcpProtocolVersion = result.ProtocolVersion
	}

	return result, nil
}

func (c *OpenClawHttpClient) discoverLegacyMcp(ctx context.Context) (*McpDiscoverResult, error) {
	result, err := SendMcp[McpInitializeRequest, McpInitializeResult](ctx, c, "initialize", &McpInitializeRequest{})
	if err != nil {
		return nil, err
	}

	if len(result.ProtocolVersion) > 0 {
		c.negotiatedMcpProtocolVersion = result.ProtocolVersion
	}

	cap, _ := json.Marshal(result.Capabilities)

	return &McpDiscoverResult{
		ProtocolVersion:   result.ProtocolVersion,
		SupportedVersions: []string{result.ProtocolVersion},
		Capabilities:      cap,
		ServerInfo:        &result.ServerInfo,
	}, nil
}

func (c *OpenClawHttpClient) GetAuthSession(ctx context.Context) (*core.AuthSessionResponse, error) {
	return SendHttp[any, core.AuthSessionResponse](ctx, c, "GET", c.authSessionUri, nil, nil)
}

func (c *OpenClawHttpClient) ChatCompletion(
	ctx context.Context,
	request core.OpenAiChatCompletionRequest,
	presetId *string) (*core.OpenAiChatCompletionResponse, error) {
	var header map[string]string
	if presetId != nil && *presetId != "" {
		header = map[string]string{
			"X-OpenClaw-Preset": *presetId,
		}
	}

	return SendHttp[core.OpenAiChatCompletionRequest, core.OpenAiChatCompletionResponse](ctx, c, "POST", c.chatCompletionsUri, &request, header)
}

func (c *OpenClawHttpClient) StreamChatCompletion(
	ctx context.Context,
	request core.OpenAiChatCompletionRequest,
	onText func(string),
	presetId *string,
) (string, error) {
	request.Stream = true

	reqBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatCompletionsUri.String(), bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	if presetId != nil && *presetId != "" {
		req.Header.Set("X-OpenClaw-Preset", *presetId)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	var fullText strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// 检查数据是否以 "data:" 开头
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		// 提取 "data:" 后面的内容并去除两边空白
		data := strings.TrimSpace(line[len("data:"):])
		if len(data) == 0 {
			continue
		}

		if data == "[DONE]" {
			break
		}

		var chunk core.OpenAiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", fmt.Errorf("failed to parse SSE chunk: %s: %w", data, err)
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			if delta != "" {
				fullText.WriteString(delta)
				if onText != nil {
					onText(delta)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading response stream: %w", err)
	}

	return fullText.String(), nil
}

func (c *OpenClawHttpClient) InitializeMcp(ctx context.Context, request McpInitializeRequest) (*McpInitializeResult, error) {
	result, err := SendMcp[McpInitializeRequest, McpInitializeResult](ctx, c, "initialize", &request)
	if err != nil {
		return nil, err
	}

	if len(result.ProtocolVersion) > 0 {
		c.negotiatedMcpProtocolVersion = result.ProtocolVersion
	}

	return result, nil
}

func (c *OpenClawHttpClient) ListMcpTools(ctx context.Context) (*McpToolListResult, error) {
	return SendMcp[any, McpToolListResult](ctx, c, "tools/list", nil)
}

func (c *OpenClawHttpClient) ListMcpResources(ctx context.Context) (*McpResourceListResult, error) {
	return SendMcp[any, McpResourceListResult](ctx, c, "resources/list", nil)
}

func (c *OpenClawHttpClient) ListMcpResourceTemplates(ctx context.Context) (*McpResourceTemplateListResult, error) {
	return SendMcp[any, McpResourceTemplateListResult](ctx, c, "resources/templates/list", nil)
}

func (c *OpenClawHttpClient) ReadMcpResourceAsync(ctx context.Context, uri string) (*McpResourceTemplateListResult, error) {
	if uri == "" {
		return nil, errors.New("Resource uri is required.")
	}

	return SendMcp[McpReadResourceRequest, McpResourceTemplateListResult](ctx, c, "resources/read", &McpReadResourceRequest{Uri: uri})
}

func (c *OpenClawHttpClient) ListMcpPrompts(ctx context.Context) (*McpPromptListResult, error) {
	return SendMcp[any, McpPromptListResult](ctx, c, "prompts/list", nil)
}

func (c *OpenClawHttpClient) GetMcpPrompt(ctx context.Context, name string, args map[string]string) (*McpGetPromptResult, error) {
	if name == "" {
		return nil, errors.New("Prompt name is required.")
	}

	return SendMcp[McpGetPromptRequest, McpGetPromptResult](ctx, c, "prompts/get", &McpGetPromptRequest{Name: name, Arguments: args})
}

func (c *OpenClawHttpClient) CallMcpTool(ctx context.Context, name string, args json.RawMessage) (*McpCallToolResult, error) {
	if name == "" {
		return nil, errors.New("Tool name is required.")
	}

	return SendMcp[McpCallToolRequest, McpCallToolResult](ctx, c, "tools/call", &McpCallToolRequest{Name: name, Arguments: args})
}
