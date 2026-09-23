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
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/payments"
	"github.com/futugyou/openclaw/util"
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

func (c *OpenClawHttpClient) ReadMcpResource(ctx context.Context, uri string) (*McpResourceTemplateListResult, error) {
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

func buildPaymentUri(baseUri *url.URL, provider, environment string, yes bool) *url.URL {
	resultUri := *baseUri

	query := resultUri.Query()

	if provider != "" {
		query.Set("provider", provider)
	}

	if environment != "" {
		query.Set("environment", environment)
	}

	if yes {
		query.Set("yes", "true")
	}

	resultUri.RawQuery = query.Encode()

	return &resultUri
}

func (c *OpenClawHttpClient) GetIntegrationDashboard(ctx context.Context) (*core.IntegrationDashboardResponse, error) {
	return SendHttp[any, core.IntegrationDashboardResponse](ctx, c, "GET", c.integrationDashboardUri, nil, nil)
}

func (c *OpenClawHttpClient) GetIntegrationStatus(ctx context.Context) (*core.IntegrationStatusResponse, error) {
	return SendHttp[any, core.IntegrationStatusResponse](ctx, c, "GET", c.integrationStatusUri, nil, nil)
}

func (c *OpenClawHttpClient) GetPaymentSetupStatus(ctx context.Context, provider string) (*payments.PaymentSetupStatus, error) {
	return SendHttp[any, payments.PaymentSetupStatus](ctx, c, "GET", buildPaymentUri(c.integrationPaymentSetupUri, provider, "", false), nil, nil)
}

func (c *OpenClawHttpClient) ListPaymentFundingSources(ctx context.Context, provider, environment string) ([]payments.FundingSource, error) {
	result, err := SendHttp[any, []payments.FundingSource](ctx, c, "GET", buildPaymentUri(c.integrationPaymentFundingUri, provider, environment, false), nil, nil)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

func (c *OpenClawHttpClient) IssueVirtualCard(ctx context.Context, request payments.VirtualCardRequest, yes bool) (*payments.VirtualCardHandle, error) {
	return SendHttp[payments.VirtualCardRequest, payments.VirtualCardHandle](ctx, c, "POST", buildPaymentUri(c.integrationPaymentVirtualCardUri, request.ProviderId, request.Environment, yes), &request, nil)
}

func (c *OpenClawHttpClient) ExecuteMachinePayment(ctx context.Context, request payments.MachinePaymentRequest, yes bool) (*payments.MachinePaymentResult, error) {
	return SendHttp[payments.MachinePaymentRequest, payments.MachinePaymentResult](ctx, c, "POST", buildPaymentUri(c.integrationPaymentExecuteUri, request.ProviderId, request.Environment, yes), &request, nil)
}

func (c *OpenClawHttpClient) GetPaymentStatus(ctx context.Context, id, provider, environment string) (*payments.PaymentStatus, error) {
	if id == "" {
		return nil, errors.New("Payment id is required")
	}
	return SendHttp[any, payments.PaymentStatus](ctx, c, "GET", buildPaymentUri(c.integrationPaymentStatusUri.JoinPath(url.PathEscape(id)), provider, environment, false), nil, nil)
}

func (c *OpenClawHttpClient) buildApprovalsUri(channelId, senderId string) *url.URL {
	resultUri := *c.integrationApprovalsUri

	query := resultUri.Query()

	if channelId != "" {
		query.Set("channelId", channelId)
	}

	if senderId != "" {
		query.Set("senderId", senderId)
	}

	resultUri.RawQuery = query.Encode()
	return &resultUri
}

func (c *OpenClawHttpClient) GetIntegrationApprovals(ctx context.Context, channelId, senderId string) (*core.IntegrationApprovalsResponse, error) {
	return SendHttp[any, core.IntegrationApprovalsResponse](ctx, c, "GET", c.buildApprovalsUri(channelId, senderId), nil, nil)
}

func (c *OpenClawHttpClient) buildApprovalHistoryUri(hquery core.ApprovalHistoryQuery) *url.URL {
	resultUri := *c.integrationApprovalHistoryUri

	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(hquery.Limit, 1, 500)))

	if hquery.ChannelId != "" {
		query.Set("channelId", hquery.ChannelId)
	}
	if hquery.SenderId != "" {
		query.Set("senderId", hquery.SenderId)
	}
	if hquery.ToolName != "" {
		query.Set("toolName", hquery.ToolName)
	}
	if hquery.FromUtc != nil {
		query.Set("fromUtc", hquery.FromUtc.Format(time.RFC3339Nano))
	}
	if hquery.ToUtc != nil {
		query.Set("toUtc", hquery.ToUtc.Format(time.RFC3339Nano))
	}

	resultUri.RawQuery = query.Encode()
	return &resultUri
}

func (c *OpenClawHttpClient) GetIntegrationApprovalHistory(ctx context.Context, query core.ApprovalHistoryQuery) (*core.IntegrationApprovalHistoryResponse, error) {
	return SendHttp[any, core.IntegrationApprovalHistoryResponse](ctx, c, "GET", c.buildApprovalHistoryUri(query), nil, nil)
}

func (c *OpenClawHttpClient) postApprovalDecision(ctx context.Context, approvalId string, approved bool) (*core.OperationStatusResponse, error) {
	if approvalId == "" {
		return nil, errors.New("approvalId is required.")
	}

	resultUri := *c.toolsApproveUri

	query := resultUri.Query()
	query.Set("approvalId", approvalId)

	if approved {
		query.Set("approved", "true")
	} else {
		query.Set("approved", "false")
	}

	return SendHttp[any, core.OperationStatusResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ApproveToolRequest(ctx context.Context, approvalId string) (*core.OperationStatusResponse, error) {
	return c.postApprovalDecision(ctx, approvalId, true)
}

func (c *OpenClawHttpClient) DenyToolRequest(ctx context.Context, approvalId string) (*core.OperationStatusResponse, error) {
	return c.postApprovalDecision(ctx, approvalId, false)
}

func (c *OpenClawHttpClient) GetIntegrationProviders(ctx context.Context, recentTurnsLimit int) (*core.IntegrationProvidersResponse, error) {
	resultUri := *c.integrationProvidersUri
	query := resultUri.Query()
	query.Set("recentTurnsLimit", fmt.Sprintf("%d", util.Clamp(recentTurnsLimit, 1, 256)))

	return SendHttp[any, core.IntegrationProvidersResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetIntegrationPlugins(ctx context.Context) (*core.IntegrationPluginsResponse, error) {
	return SendHttp[any, core.IntegrationPluginsResponse](ctx, c, "GET", c.integrationPluginsUri, nil, nil)
}

func (c *OpenClawHttpClient) buildCompatibilityCatalogUri(compatibilityStatus, kind, category string) *url.URL {
	resultUri := *c.integrationCompatibilityCatalogUri

	query := resultUri.Query()

	if compatibilityStatus != "" {
		query.Set("compatibilityStatus", compatibilityStatus)
	}
	if kind != "" {
		query.Set("kind", kind)
	}
	if category != "" {
		query.Set("category", category)
	}

	resultUri.RawQuery = query.Encode()
	return &resultUri
}

func (c *OpenClawHttpClient) GetCompatibilityCatalog(ctx context.Context, compatibilityStatus, kind, category string) (*core.IntegrationCompatibilityCatalogResponse, error) {
	return SendHttp[any, core.IntegrationCompatibilityCatalogResponse](ctx, c, "GET", c.buildCompatibilityCatalogUri(compatibilityStatus, kind, category), nil, nil)
}

func (c *OpenClawHttpClient) GetIntegrationAccounts(ctx context.Context) (*core.IntegrationAccountsResponse, error) {
	return SendHttp[any, core.IntegrationAccountsResponse](ctx, c, "GET", c.integrationAccountsUri, nil, nil)
}

func (c *OpenClawHttpClient) GetIntegrationAccount(ctx context.Context, accountId string) (*core.IntegrationConnectedAccountResponse, error) {
	resultUri := *c.integrationAccountsUri
	query := resultUri.Query()

	if accountId != "" {
		query.Set("accountId", accountId)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.IntegrationConnectedAccountResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) CreateIntegrationAccount(ctx context.Context, request core.ConnectedAccountCreateRequest) (*core.IntegrationConnectedAccountResponse, error) {
	return SendHttp[core.ConnectedAccountCreateRequest, core.IntegrationConnectedAccountResponse](ctx, c, "POST", c.integrationAccountsUri, &request, nil)
}

func (c *OpenClawHttpClient) DeleteIntegrationAccount(ctx context.Context, accountId string) (*core.OperationStatusResponse, error) {
	resultUri := *c.integrationAccountsUri
	query := resultUri.Query()

	if accountId != "" {
		query.Set("accountId", accountId)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.OperationStatusResponse](ctx, c, "DELETE", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetIntegrationBackends(ctx context.Context) (*core.IntegrationBackendsResponse, error) {
	return SendHttp[any, core.IntegrationBackendsResponse](ctx, c, "GET", c.integrationBackendsUri, nil, nil)
}

func (c *OpenClawHttpClient) buildIntegrationBackendUri(backendId string) *url.URL {
	return c.integrationBackendsUri.JoinPath(backendId)
}
func (c *OpenClawHttpClient) buildIntegrationBackendProbeUri(backendId string) *url.URL {
	return c.buildIntegrationBackendUri(backendId).JoinPath("probe")
}
func (c *OpenClawHttpClient) buildIntegrationBackendSessionsUri(backendId string) *url.URL {
	return c.buildIntegrationBackendUri(backendId).JoinPath("sessions")
}

func (c *OpenClawHttpClient) GetIntegrationBackend(ctx context.Context, backendId string) (*core.IntegrationBackendResponse, error) {
	return SendHttp[any, core.IntegrationBackendResponse](ctx, c, "GET", c.buildIntegrationBackendUri(backendId), nil, nil)
}

func (c *OpenClawHttpClient) ProbeIntegrationBackend(ctx context.Context, backendId string, request core.BackendProbeRequest) (*core.BackendProbeResult, error) {
	return SendHttp[core.BackendProbeRequest, core.BackendProbeResult](ctx, c, "POST", c.buildIntegrationBackendProbeUri(backendId), &request, nil)
}

func (c *OpenClawHttpClient) StartBackendSession(ctx context.Context, backendId string, request core.StartBackendSessionRequest) (*core.IntegrationBackendSessionResponse, error) {
	return SendHttp[core.StartBackendSessionRequest, core.IntegrationBackendSessionResponse](ctx, c, "POST", c.buildIntegrationBackendSessionsUri(backendId), &request, nil)
}

func (c *OpenClawHttpClient) SendBackendInput(ctx context.Context, backendId, sessionId string, request core.BackendInput) (*core.IntegrationBackendSessionResponse, error) {
	return SendHttp[core.BackendInput, core.IntegrationBackendSessionResponse](ctx, c, "POST", c.buildIntegrationBackendSessionsUri(backendId), &request, nil)
}

func (c *OpenClawHttpClient) buildIntegrationBackendSessionUri(backendId, sessionId string) *url.URL {
	return c.buildIntegrationBackendSessionsUri(backendId).JoinPath(sessionId)
}

func (c *OpenClawHttpClient) buildIntegrationBackendInputUri(backendId, sessionId string) *url.URL {
	return c.buildIntegrationBackendSessionUri(backendId, sessionId).JoinPath("input")
}

func (c *OpenClawHttpClient) buildIntegrationBackendEventsUri(backendId, sessionId string, afterSequence int64, limit int) *url.URL {
	resultUri := c.buildIntegrationBackendSessionUri(backendId, sessionId).JoinPath("events")
	query := resultUri.Query()
	query.Set("afterSequence", fmt.Sprintf("%d", max(afterSequence, 0)))
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 500)))
	resultUri.RawQuery = query.Encode()
	return resultUri
}

func (c *OpenClawHttpClient) buildIntegrationBackendEventStreamUri(backendId, sessionId string, afterSequence int64, limit int) *url.URL {
	resultUri := c.buildIntegrationBackendSessionUri(backendId, sessionId).JoinPath("events").JoinPath("stream")
	query := resultUri.Query()
	query.Set("afterSequence", fmt.Sprintf("%d", max(afterSequence, 0)))
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 500)))
	resultUri.RawQuery = query.Encode()
	return resultUri

}

func (c *OpenClawHttpClient) StopBackendSession(ctx context.Context, backendId, sessionId string) (*core.IntegrationBackendSessionResponse, error) {
	return SendHttp[any, core.IntegrationBackendSessionResponse](ctx, c, "DELETE", c.buildIntegrationBackendInputUri(backendId, sessionId), nil, nil)
}

func (c *OpenClawHttpClient) GetBackendSession(ctx context.Context, backendId, sessionId string) (*core.IntegrationBackendSessionResponse, error) {
	return SendHttp[any, core.IntegrationBackendSessionResponse](ctx, c, "GET", c.buildIntegrationBackendSessionUri(backendId, sessionId), nil, nil)
}

func (c *OpenClawHttpClient) GetBackendEvents(ctx context.Context, backendId, sessionId string, afterSequence int64, limit int) (*core.IntegrationBackendEventsResponse, error) {
	return SendHttp[any, core.IntegrationBackendEventsResponse](ctx, c, "GET", c.buildIntegrationBackendEventsUri(backendId, sessionId, afterSequence, limit), nil, nil)
}

func (c *OpenClawHttpClient) StreamBackendEvents(
	ctx context.Context,
	backendId, sessionId string, afterSequence int64, limit int,
	onEvent func(core.BackendEvent),
) error {
	resulturi := c.buildIntegrationBackendEventStreamUri(backendId, sessionId, afterSequence, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resulturi.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(line[len("data:"):])
		if len(data) == 0 {
			continue
		}

		if data == "[DONE]" {
			break
		}

		var event core.BackendEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("failed to parse SSE chunk: %s: %w", data, err)
		}

		if onEvent != nil {
			onEvent(event)
		}
	}

	return scanner.Err()
}

func (c *OpenClawHttpClient) GetIntegrationOperatorAudit(ctx context.Context, query core.OperatorAuditQuery) (*core.IntegrationOperatorAuditResponse, error) {
	return SendHttp[any, core.IntegrationOperatorAuditResponse](ctx, c, "GET", c.buildOperatorAuditUri(query), nil, nil)
}

func (c *OpenClawHttpClient) buildOperatorAuditUri(hquery core.OperatorAuditQuery) *url.URL {
	resultUri := *c.integrationOperatorAuditUri
	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(hquery.Limit, 1, 500)))
	if hquery.ActorId != "" {
		query.Set("actorId", hquery.ActorId)
	}
	if hquery.ActionType != "" {
		query.Set("actionType", hquery.ActionType)
	}
	if hquery.TargetId != "" {
		query.Set("targetId", hquery.TargetId)
	}
	if hquery.FromUtc != nil {
		query.Set("fromUtc", hquery.FromUtc.Format(time.RFC3339Nano))
	}
	if hquery.ToUtc != nil {
		query.Set("toUtc", hquery.ToUtc.Format(time.RFC3339Nano))
	}
	resultUri.RawQuery = query.Encode()

	return &resultUri
}

func (c *OpenClawHttpClient) ListSessions(ctx context.Context, page, pageSize int, hquery core.SessionListQuery) (*core.IntegrationSessionsResponse, error) {
	resultUri := *c.integrationOperatorAuditUri
	query := resultUri.Query()
	query.Set("page", fmt.Sprintf("%d", max(page, 1)))
	query.Set("pageSize", fmt.Sprintf("%d", util.Clamp(pageSize, 1, 200)))
	if hquery.Search != "" {
		query.Set("search", hquery.Search)
	}
	if hquery.ChannelId != "" {
		query.Set("channelId", hquery.ChannelId)
	}
	if hquery.SenderId != "" {
		query.Set("senderId", hquery.SenderId)
	}
	if hquery.FromUtc != nil {
		query.Set("fromUtc", hquery.FromUtc.Format(time.RFC3339Nano))
	}
	if hquery.ToUtc != nil {
		query.Set("toUtc", hquery.ToUtc.Format(time.RFC3339Nano))
	}
	if hquery.State != nil {
		query.Set("state", hquery.State.String())
	}
	if hquery.Starred != nil {
		query.Set("starred", strconv.FormatBool(*hquery.Starred))
	}
	if hquery.Tag != "" {
		query.Set("tag", hquery.Tag)
	}
	resultUri.RawQuery = query.Encode()

	return SendHttp[any, core.IntegrationSessionsResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetSession(ctx context.Context, sessionId string) (*core.IntegrationSessionDetailResponse, error) {
	if sessionId == "" {
		return nil, errors.New("Session id is required")
	}

	return SendHttp[any, core.IntegrationSessionDetailResponse](ctx, c, "GET", c.integrationSessionsUri.JoinPath(sessionId), nil, nil)
}

func (c *OpenClawHttpClient) GetSessionTimeline(ctx context.Context, sessionId string, limit int) (*core.IntegrationSessionTimelineResponse, error) {
	if sessionId == "" {
		return nil, errors.New("Session id is required")
	}

	resultUri := c.integrationSessionsUri.JoinPath(sessionId)
	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 500)))
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.IntegrationSessionTimelineResponse](ctx, c, "GET", resultUri, nil, nil)
}

func (c *OpenClawHttpClient) SearchSessions(ctx context.Context, search core.SessionSearchQuery) (*core.IntegrationSessionSearchResponse, error) {
	resultUri := *c.integrationSessionsUri
	query := resultUri.Query()
	query.Set("text", search.Text)
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(search.Limit, 1, 200)))
	query.Set("snippetLength", fmt.Sprintf("%d", util.Clamp(search.SnippetLength, 40, 1000)))
	if search.ChannelId != "" {
		query.Set("channelId", search.ChannelId)
	}
	if search.SenderId != "" {
		query.Set("senderId", search.SenderId)
	}
	if search.FromUtc != nil {
		query.Set("fromUtc", search.FromUtc.Format(time.RFC3339Nano))
	}
	if search.ToUtc != nil {
		query.Set("toUtc", search.ToUtc.Format(time.RFC3339Nano))
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.IntegrationSessionSearchResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ListProfiles(ctx context.Context) (*core.IntegrationProfilesResponse, error) {
	return SendHttp[any, core.IntegrationProfilesResponse](ctx, c, "GET", c.integrationProfilesUri, nil, nil)
}

func (c *OpenClawHttpClient) ListToolPresets(ctx context.Context) (*core.IntegrationToolPresetsResponse, error) {
	return SendHttp[any, core.IntegrationToolPresetsResponse](ctx, c, "GET", c.integrationToolPresetsUri, nil, nil)
}

func (c *OpenClawHttpClient) GetProfile(ctx context.Context, actorId string) (*core.IntegrationProfileResponse, error) {
	if actorId == "" {
		return nil, errors.New("Actor id is required")
	}
	return SendHttp[any, core.IntegrationProfileResponse](ctx, c, "GET", c.integrationProfilesUri.JoinPath(actorId), nil, nil)
}

func (c *OpenClawHttpClient) SaveProfile(ctx context.Context, actorId string, profile core.UserProfile) (*core.IntegrationProfileResponse, error) {
	if actorId == "" {
		return nil, errors.New("Actor id is required")
	}
	return SendHttp[core.IntegrationProfileUpdateRequest, core.IntegrationProfileResponse](ctx, c, "PUT", c.integrationProfilesUri.JoinPath(actorId), &core.IntegrationProfileUpdateRequest{
		Profile: profile,
	}, nil)
}

func (c *OpenClawHttpClient) ListMemoryNotes(ctx context.Context, prefix, memoryClass, projectId string, limit int) (*core.MemoryNoteListResponse, error) {
	resultUri := *c.adminMemoryNotesUri
	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 200)))
	if prefix != "" {
		query.Set("prefix", prefix)
	}
	if memoryClass != "" {
		query.Set("memoryClass", memoryClass)
	}
	if projectId != "" {
		query.Set("projectId", projectId)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.MemoryNoteListResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) SearchMemoryNotes(ctx context.Context, querystr, memoryClass, projectId string, limit int) (*core.MemoryNoteListResponse, error) {
	resultUri := *c.adminMemorySearchUri
	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 200)))
	query.Set("query", querystr)
	if memoryClass != "" {
		query.Set("memoryClass", memoryClass)
	}
	if projectId != "" {
		query.Set("projectId", projectId)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.MemoryNoteListResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetMemoryNote(ctx context.Context, key string) (*core.MemoryNoteDetailResponse, error) {
	if key == "" {
		return nil, errors.New("Memory note key is required.")
	}
	return SendHttp[any, core.MemoryNoteDetailResponse](ctx, c, "GET", c.adminMemoryNotesUri.JoinPath(key), nil, nil)
}

func (c *OpenClawHttpClient) SaveMemoryNote(ctx context.Context, request core.MemoryNoteUpsertRequest) (*core.MemoryNoteDetailResponse, error) {
	return SendHttp[core.MemoryNoteUpsertRequest, core.MemoryNoteDetailResponse](ctx, c, "POST", c.adminMemoryNotesUri, &request, nil)
}

func (c *OpenClawHttpClient) DeleteMemoryNote(ctx context.Context, key string) (*core.MutationResponse, error) {
	if key == "" {
		return nil, errors.New("Memory note key is required.")
	}
	return SendHttp[any, core.MutationResponse](ctx, c, "DELETE", c.adminMemoryNotesUri.JoinPath(key), nil, nil)
}

func (c *OpenClawHttpClient) ExportMemoryConsole(ctx context.Context, actorId, projectId string, includeProfiles, includeProposals, includeAutomations, includeNotes bool) (*core.MemoryConsoleExportBundle, error) {
	resultUri := *c.adminMemoryExportUri
	query := resultUri.Query()
	query.Set("includeProfiles", strconv.FormatBool(includeProfiles))
	query.Set("includeProposals", strconv.FormatBool(includeProposals))
	query.Set("includeAutomations", strconv.FormatBool(includeAutomations))
	query.Set("includeNotes", strconv.FormatBool(includeNotes))
	if actorId != "" {
		query.Set("actorId", actorId)
	}
	if projectId != "" {
		query.Set("projectId", projectId)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.MemoryConsoleExportBundle](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ImportMemoryConsole(ctx context.Context, bundle core.MemoryConsoleExportBundle) (*core.MemoryConsoleImportResponse, error) {
	return SendHttp[core.MemoryConsoleExportBundle, core.MemoryConsoleImportResponse](ctx, c, "POST", c.adminMemoryImportUri, &bundle, nil)
}

func (c *OpenClawHttpClient) GetFractalMemoryStatus(ctx context.Context) (*core.StructuredMemoryStatusResponse, error) {
	return SendHttp[any, core.StructuredMemoryStatusResponse](ctx, c, "GET", c.adminMemoryFractalStatusUri, nil, nil)
}

func (c *OpenClawHttpClient) SearchFractalMemory(ctx context.Context, querystr string, limit int, scope string) (*core.StructuredMemorySearchResult, error) {
	if querystr == "" {
		return nil, errors.New("Query is required.")
	}

	resultUri := *c.adminMemoryFractalSearchUri
	query := resultUri.Query()
	query.Set("query", querystr)
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 50)))
	if scope != "" {
		query.Set("scope", scope)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.StructuredMemorySearchResult](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) OpenFractalMemory(ctx context.Context, path string, depth int, view string) (*core.StructuredMemoryOpenResult, error) {
	if path == "" {
		return nil, errors.New("path is required.")
	}

	resultUri := *c.adminMemoryFractalOpenUri
	query := resultUri.Query()
	query.Set("path", path)
	if depth >= 0 {
		query.Set("depth", fmt.Sprintf("%d", util.Clamp(depth, 0, 3)))
	}

	if view != "" {
		query.Set("view", view)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.StructuredMemoryOpenResult](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ExportFractalMemory(ctx context.Context, path string, mode string) (*core.StructuredMemoryExportResult, error) {
	if path == "" {
		return nil, errors.New("path is required.")
	}

	resultUri := *c.adminMemoryFractalExportUri
	query := resultUri.Query()
	query.Set("path", path)
	if mode != "" {
		query.Set("mode", mode)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.StructuredMemoryExportResult](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetRecentFractalMemory(ctx context.Context, days, limit int, scope string) (*core.StructuredMemoryRecentResult, error) {
	resultUri := *c.adminMemoryFractalRecentUri
	query := resultUri.Query()
	query.Set("days", fmt.Sprintf("%d", util.Clamp(days, 1, 3650)))
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 100)))
	if scope != "" {
		query.Set("scope", scope)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.StructuredMemoryRecentResult](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ValidateFractalMemory(ctx context.Context) (*core.StructuredMemoryValidationResult, error) {
	return SendHttp[any, core.StructuredMemoryValidationResult](ctx, c, "POST", c.adminMemoryFractalValidateUri, nil, nil)
}

func (c *OpenClawHttpClient) RefreshFractalMemoryIndex(ctx context.Context) (*core.StructuredMemoryValidationResult, error) {
	return SendHttp[any, core.StructuredMemoryValidationResult](ctx, c, "POST", c.adminMemoryFractalIndexRefreshUri, nil, nil)
}

func (c *OpenClawHttpClient) CreateFractalMemoryHandoff(ctx context.Context, path string) (*core.StructuredMemoryHandoffResult, error) {
	if path == "" {
		return nil, errors.New("path is required.")
	}
	return SendHttp[core.StructuredMemoryPathRequest, core.StructuredMemoryHandoffResult](ctx, c, "POST", c.adminMemoryFractalHandoffUri, &core.StructuredMemoryPathRequest{Path: path}, nil)
}

func (c *OpenClawHttpClient) ListSharedHarnessState(ctx context.Context, request core.SharedHarnessStateListQuery) (*core.SharedHarnessStateListResponse, error) {
	resultUri := *c.adminHarnessSharedStateUri
	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(request.Limit, 1, 500)))
	if request.SessionId != "" {
		query.Set("sessionId", request.SessionId)
	}
	if request.ParentSessionId != "" {
		query.Set("parentSessionId", request.ParentSessionId)
	}
	if request.HarnessContractId != "" {
		query.Set("harnessContractId", request.HarnessContractId)
	}
	if request.Status != "" {
		query.Set("status", request.Status)
	}
	if request.Tag != "" {
		query.Set("tag", request.Tag)
	}
	if request.CreatedFromUtc != nil {
		query.Set("createdFromUtc", request.CreatedFromUtc.Format(time.RFC3339Nano))
	}
	if request.CreatedToUtc != nil {
		query.Set("createdToUtc", request.CreatedToUtc.Format(time.RFC3339Nano))
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.SharedHarnessStateListResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetSharedHarnessState(ctx context.Context, id string) (*core.SharedHarnessStateDetailResponse, error) {
	if id == "" {
		return nil, errors.New("Shared harness state id is required.")
	}
	return SendHttp[any, core.SharedHarnessStateDetailResponse](ctx, c, "GET", c.adminHarnessSharedStateUri.JoinPath(id), nil, nil)
}

func (c *OpenClawHttpClient) GetSharedHarnessStateForSession(ctx context.Context, sessionId string) (*core.SharedHarnessStateDetailResponse, error) {
	if sessionId == "" {
		return nil, errors.New("Session id is required.")
	}
	return SendHttp[any, core.SharedHarnessStateDetailResponse](ctx, c, "GET", c.baseUri.JoinPath("/admin/sessions/").JoinPath(sessionId).JoinPath("arness-state"), nil, nil)
}

func (c *OpenClawHttpClient) DetectSharedHarnessStateConflicts(ctx context.Context, id string) (*core.SharedHarnessStateMutationResponse, error) {
	if id == "" {
		return nil, errors.New("Shared harness state id is required.")
	}
	return SendHttp[any, core.SharedHarnessStateMutationResponse](ctx, c, "POST", c.adminHarnessSharedStateUri.JoinPath(id).JoinPath("detect-conflicts"), nil, nil)
}

func (c *OpenClawHttpClient) ExportAgentBundle(
	ctx context.Context,
	actorId, projectId string,
	includeSettings, includeNotes, includeProfiles, includeProposals, includeAutomations, includePolicies, includeManagedSkills bool) (*core.AgentBundleExportBundle, error) {
	resultUri := *c.adminAgentBundleExportUri
	query := resultUri.Query()
	query.Set("includeSettings", strconv.FormatBool(includeSettings))
	query.Set("includeNotes", strconv.FormatBool(includeNotes))
	query.Set("includeProfiles", strconv.FormatBool(includeProfiles))
	query.Set("includeProposals", strconv.FormatBool(includeProposals))
	query.Set("includeAutomations", strconv.FormatBool(includeAutomations))
	query.Set("includePolicies", strconv.FormatBool(includePolicies))
	query.Set("includeManagedSkills", strconv.FormatBool(includeManagedSkills))
	if actorId != "" {
		query.Set("actorId", actorId)
	}
	if projectId != "" {
		query.Set("projectId", projectId)
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.AgentBundleExportBundle](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ImportAgentBundle(ctx context.Context, bundle core.AgentBundleExportBundle) (*core.AgentBundleImportResponse, error) {
	return SendHttp[core.AgentBundleExportBundle, core.AgentBundleImportResponse](ctx, c, "POST", c.adminAgentBundleImportUri, &bundle, nil)
}

func (c *OpenClawHttpClient) UpdateSessionMetadata(ctx context.Context, sessionId string, bundle core.SessionMetadataUpdateRequest) (*core.AgentBundleImportResponse, error) {
	if sessionId == "" {
		return nil, errors.New("Session id is required.")
	}

	requesturi := c.baseUri.JoinPath("/admin/sessions/").JoinPath(sessionId).JoinPath("/metadata")
	return SendHttp[core.SessionMetadataUpdateRequest, core.AgentBundleImportResponse](ctx, c, "POST", requesturi, &bundle, nil)
}

func (c *OpenClawHttpClient) PromoteSession(ctx context.Context, sessionId string, bundle core.SessionPromotionRequest) (*core.SessionPromotionResponse, error) {
	if sessionId == "" {
		return nil, errors.New("Session id is required.")
	}

	requesturi := c.baseUri.JoinPath("/admin/sessions/").JoinPath(sessionId).JoinPath("/promote")
	return SendHttp[core.SessionPromotionRequest, core.SessionPromotionResponse](ctx, c, "POST", requesturi, &bundle, nil)
}

func (c *OpenClawHttpClient) ListAutomations(ctx context.Context) (*core.IntegrationAutomationsResponse, error) {
	return SendHttp[any, core.IntegrationAutomationsResponse](ctx, c, "GET", c.integrationAutomationsUri, nil, nil)
}

func (c *OpenClawHttpClient) ListAutomationTemplates(ctx context.Context) (*core.AutomationTemplateListResponse, error) {
	return SendHttp[any, core.AutomationTemplateListResponse](ctx, c, "GET", c.integrationAutomationsUri.JoinPath("templates"), nil, nil)
}

func (c *OpenClawHttpClient) GetAutomation(ctx context.Context, automationId string) (*core.IntegrationAutomationDetailResponse, error) {
	if automationId == "" {
		return nil, errors.New("Automation id is required.")
	}

	return SendHttp[any, core.IntegrationAutomationDetailResponse](ctx, c, "GET", c.integrationAutomationsUri.JoinPath(automationId), nil, nil)
}

func (c *OpenClawHttpClient) RunAutomation(ctx context.Context, automationId string, dryRun bool) (*core.MutationResponse, error) {
	if automationId == "" {
		return nil, errors.New("Automation id is required.")
	}

	return SendHttp[core.AutomationRunRequest, core.MutationResponse](ctx, c, "POST",
		c.integrationAutomationsUri.JoinPath(automationId).JoinPath("run"),
		&core.AutomationRunRequest{
			DryRun: dryRun,
		}, nil)
}

func (c *OpenClawHttpClient) DeleteAutomation(ctx context.Context, automationId string) (*core.MutationResponse, error) {
	if automationId == "" {
		return nil, errors.New("Automation id is required.")
	}

	return SendHttp[any, core.MutationResponse](ctx, c, "DELETE",
		c.integrationAutomationsUri.JoinPath(automationId),
		nil, nil)
}

func (c *OpenClawHttpClient) GetAutomationRuns(ctx context.Context, automationId string) (*core.IntegrationAutomationRunsResponse, error) {
	if automationId == "" {
		return nil, errors.New("Automation id is required.")
	}

	return SendHttp[any, core.IntegrationAutomationRunsResponse](ctx, c, "GET", c.integrationAutomationsUri.JoinPath(automationId).JoinPath("/runs"), nil, nil)
}

func (c *OpenClawHttpClient) GetAutomationRun(ctx context.Context, automationId, runId string) (*core.IntegrationAutomationRunDetailResponse, error) {
	if automationId == "" {
		return nil, errors.New("Automation id is required.")
	}
	if runId == "" {
		return nil, errors.New("Automation run id is required.")
	}

	return SendHttp[any, core.IntegrationAutomationRunDetailResponse](ctx, c, "GET",
		c.integrationAutomationsUri.JoinPath(automationId).JoinPath(runId),
		nil, nil)
}

func (c *OpenClawHttpClient) ReplayAutomationRun(ctx context.Context, automationId, runId string) (*core.MutationResponse, error) {
	if automationId == "" {
		return nil, errors.New("Automation id is required.")
	}
	if runId == "" {
		return nil, errors.New("Automation run id is required.")
	}

	return SendHttp[any, core.MutationResponse](ctx, c, "POST",
		c.integrationAutomationsUri.JoinPath(automationId).JoinPath(runId).JoinPath("/replay"),
		nil, nil)
}

func (c *OpenClawHttpClient) ClearAutomationQuarantine(ctx context.Context, automationId string) (*core.MutationResponse, error) {
	if automationId == "" {
		return nil, errors.New("Automation id is required.")
	}
	return SendHttp[any, core.MutationResponse](ctx, c, "POST",
		c.integrationAutomationsUri.JoinPath(automationId).JoinPath("/quarantine/clear"),
		nil, nil)
}

func (c *OpenClawHttpClient) ListWorkflows(ctx context.Context) (*core.IntegrationWorkflowsResponse, error) {
	return SendHttp[any, core.IntegrationWorkflowsResponse](ctx, c, "GET", c.integrationWorkflowsUri, nil, nil)
}

func (c *OpenClawHttpClient) RunWorkflow(ctx context.Context, workflowId string, request core.AgentWorkflowRequest) (*core.IntegrationWorkflowsResponse, error) {
	if workflowId == "" {
		return nil, errors.New("workflow id is required.")
	}
	return SendHttp[core.AgentWorkflowRequest, core.IntegrationWorkflowsResponse](ctx, c, "POST", c.integrationWorkflowsUri.JoinPath(workflowId).JoinPath("/runs"), &request, nil)
}

func (c *OpenClawHttpClient) GetWorkflowRun(ctx context.Context, workflowId, runId string) (*core.AgentWorkflowRunSnapshot, error) {
	if workflowId == "" {
		return nil, errors.New("workflow id is required.")
	}
	if runId == "" {
		return nil, errors.New("workflow run id is required.")
	}
	requesturi := c.integrationWorkflowsUri.JoinPath(workflowId).JoinPath("/runs").JoinPath(runId)
	return SendHttp[any, core.AgentWorkflowRunSnapshot](ctx, c, "GET", requesturi, nil, nil)
}

func (c *OpenClawHttpClient) RespondWorkflowRun(ctx context.Context, workflowId, runId string, response core.AgentWorkflowResponse) (*core.AgentWorkflowRunSnapshot, error) {
	if workflowId == "" {
		return nil, errors.New("workflow id is required.")
	}
	if runId == "" {
		return nil, errors.New("workflow run id is required.")
	}
	requesturi := c.integrationWorkflowsUri.JoinPath(workflowId).JoinPath("/runs").JoinPath(runId).JoinPath("/responses")
	return SendHttp[core.AgentWorkflowResponse, core.AgentWorkflowRunSnapshot](ctx, c, "POST", requesturi, &response, nil)
}

func (c *OpenClawHttpClient) QueryRuntimeEvents(ctx context.Context, request core.RuntimeEventQuery) (*core.IntegrationRuntimeEventsResponse, error) {
	resultUri := *c.integrationRuntimeEventsUri
	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(request.Limit, 1, 500)))
	if request.SessionId != "" {
		query.Set("sessionId", request.SessionId)
	}
	if request.ChannelId != "" {
		query.Set("channelId", request.ChannelId)
	}
	if request.SenderId != "" {
		query.Set("senderId", request.SenderId)
	}
	if request.Component != "" {
		query.Set("component", request.Component)
	}
	if request.Action != "" {
		query.Set("action", request.Action)
	}
	if request.FromUtc != nil {
		query.Set("fromUtc", request.FromUtc.Format(time.RFC3339Nano))
	}
	if request.ToUtc != nil {
		query.Set("toUtc", request.ToUtc.Format(time.RFC3339Nano))
	}
	resultUri.RawQuery = query.Encode()
	return SendHttp[any, core.IntegrationRuntimeEventsResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) EnqueueMessage(ctx context.Context, request core.IntegrationMessageRequest) (*core.IntegrationMessageResponse, error) {
	return SendHttp[core.IntegrationMessageRequest, core.IntegrationMessageResponse](ctx, c, "POST", c.integrationMessagesUri, &request, nil)
}

func (c *OpenClawHttpClient) GetHeartbeat(ctx context.Context) (*core.HeartbeatPreviewResponse, error) {
	return SendHttp[any, core.HeartbeatPreviewResponse](ctx, c, "GET", c.adminHeartbeatUri, nil, nil)
}

func (c *OpenClawHttpClient) GetAdminAutomations(ctx context.Context) (*core.IntegrationAutomationsResponse, error) {
	return SendHttp[any, core.IntegrationAutomationsResponse](ctx, c, "GET", c.adminAutomationsUri, nil, nil)
}

func (c *OpenClawHttpClient) GetAdminAutomationTemplates(ctx context.Context) (*core.AutomationTemplateListResponse, error) {
	return SendHttp[any, core.AutomationTemplateListResponse](ctx, c, "GET", c.adminAutomationsUri.JoinPath("/templates"), nil, nil)
}

func (c *OpenClawHttpClient) GetAdminAutomation(ctx context.Context, automationId string) (*core.IntegrationAutomationDetailResponse, error) {
	if automationId == "" {
		return nil, errors.New("automation id is required")
	}
	return SendHttp[any, core.IntegrationAutomationDetailResponse](ctx, c, "GET", c.adminAutomationsUri.JoinPath(automationId), nil, nil)
}

func (c *OpenClawHttpClient) PreviewAutomation(ctx context.Context, automation core.AutomationDefinition) (*core.AutomationPreview, error) {
	return SendHttp[core.AutomationDefinition, core.AutomationPreview](ctx, c, "POST", c.adminAutomationsUri.JoinPath("/preview"), &automation, nil)
}

func (c *OpenClawHttpClient) SaveAutomation(ctx context.Context, automationId string, automation core.AutomationDefinition) (*core.IntegrationAutomationDetailResponse, error) {
	if automationId == "" {
		return nil, errors.New("automation id is required")
	}
	return SendHttp[core.AutomationDefinition, core.IntegrationAutomationDetailResponse](ctx, c, "POST", c.adminAutomationsUri.JoinPath(automationId), &automation, nil)
}

func (c *OpenClawHttpClient) RunAdminAutomation(ctx context.Context, automationId string, dryRun bool) (*core.MutationResponse, error) {
	if automationId == "" {
		return nil, errors.New("automation id is required")
	}
	return SendHttp[core.AutomationRunRequest, core.MutationResponse](ctx, c, "POST",
		c.adminAutomationsUri.JoinPath(automationId).JoinPath("/run"),
		&core.AutomationRunRequest{DryRun: dryRun}, nil)
}

func (c *OpenClawHttpClient) DeleteAdminAutomation(ctx context.Context, automationId string) (*core.MutationResponse, error) {
	if automationId == "" {
		return nil, errors.New("automation id is required")
	}
	return SendHttp[any, core.MutationResponse](ctx, c, "DELETE", c.adminAutomationsUri.JoinPath(automationId), nil, nil)
}

func (c *OpenClawHttpClient) MigrateAutomations(ctx context.Context, apply bool) (*core.IntegrationAutomationsResponse, error) {
	resultUri := *c.adminAutomationsUri.JoinPath("/migrate")
	query := resultUri.Query()
	query.Set("apply", strconv.FormatBool(apply))
	resultUri.RawQuery = query.Encode()

	return SendHttp[any, core.IntegrationAutomationsResponse](ctx, c, "POST", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ListLearningProposals(ctx context.Context, status, kind string) (*core.LearningProposalListResponse, error) {
	resultUri := *c.adminLearningProposalsUri
	query := resultUri.Query()
	if status != "" {
		query.Set("status", status)
	}
	if kind != "" {
		query.Set("kind", kind)
	}

	resultUri.RawQuery = query.Encode()

	return SendHttp[any, core.LearningProposalListResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetLearningProposalDetail(ctx context.Context, proposalId string) (*core.LearningProposalDetailResponse, error) {
	if proposalId == "" {
		return nil, fmt.Errorf("Proposal id is required")
	}

	return SendHttp[any, core.LearningProposalDetailResponse](ctx, c, "GET", c.adminLearningProposalsUri.JoinPath(proposalId), nil, nil)
}

func (c *OpenClawHttpClient) ApproveLearningProposal(ctx context.Context, proposalId string) (*core.LearningProposal, error) {
	if proposalId == "" {
		return nil, fmt.Errorf("Proposal id is required")
	}

	return SendHttp[any, core.LearningProposal](ctx, c, "POST", c.adminLearningProposalsUri.JoinPath(proposalId).JoinPath("approve"), nil, nil)
}

func (c *OpenClawHttpClient) RejectLearningProposal(ctx context.Context, proposalId string) (*core.LearningProposal, error) {
	if proposalId == "" {
		return nil, fmt.Errorf("Proposal id is required")
	}

	return SendHttp[any, core.LearningProposal](ctx, c, "POST", c.adminLearningProposalsUri.JoinPath(proposalId).JoinPath("reject"), nil, nil)
}

func (c *OpenClawHttpClient) RollbackLearningProposal(ctx context.Context, proposalId, reason string) (*core.LearningProposal, error) {
	if proposalId == "" {
		return nil, fmt.Errorf("Proposal id is required")
	}

	return SendHttp[core.LearningProposalReviewRequest, core.LearningProposal](ctx, c, "POST",
		c.adminLearningProposalsUri.JoinPath(proposalId).JoinPath("rollback"), &core.LearningProposalReviewRequest{Reason: reason}, nil)
}

func (c *OpenClawHttpClient) PreviewHeartbeat(ctx context.Context, request core.HeartbeatConfigDto) (*core.HeartbeatPreviewResponse, error) {
	return SendHttp[core.HeartbeatConfigDto, core.HeartbeatPreviewResponse](ctx, c, "POST", c.adminHeartbeatPreviewUri, &request, nil)
}

func (c *OpenClawHttpClient) SaveHeartbeat(ctx context.Context, request core.HeartbeatConfigDto) (*core.HeartbeatPreviewResponse, error) {
	return SendHttp[core.HeartbeatConfigDto, core.HeartbeatPreviewResponse](ctx, c, "PUT", c.adminHeartbeatUri, &request, nil)
}

func (c *OpenClawHttpClient) GetHeartbeatStatus(ctx context.Context) (*core.HeartbeatStatusResponse, error) {
	return SendHttp[any, core.HeartbeatStatusResponse](ctx, c, "GET", c.adminHeartbeatStatusUri, nil, nil)
}

func (c *OpenClawHttpClient) GetPulseStatus(ctx context.Context) (*core.PulseStatusResponse, error) {
	return SendHttp[any, core.PulseStatusResponse](ctx, c, "GET", c.adminPulseStatusUri, nil, nil)
}

func (c *OpenClawHttpClient) RunPulse(ctx context.Context, request core.PulseRunRequest) (*core.PulseRunResponse, error) {
	return SendHttp[core.PulseRunRequest, core.PulseRunResponse](ctx, c, "POST", c.adminPulseRunUri, &request, nil)
}

func (c *OpenClawHttpClient) GetPulseEvents(ctx context.Context, limit int) (*core.RuntimeEventListResponse, error) {
	resultUri := *c.adminPulseEventsUri
	query := resultUri.Query()
	query.Set("limit", fmt.Sprintf("%d", util.Clamp(limit, 1, 500)))

	resultUri.RawQuery = query.Encode()

	return SendHttp[any, core.RuntimeEventListResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) EnablePulse(ctx context.Context) (*core.PulseStatusResponse, error) {
	return SendHttp[any, core.PulseStatusResponse](ctx, c, "POST", c.adminPulseEnableUri, nil, nil)
}

func (c *OpenClawHttpClient) DisablePulse(ctx context.Context) (*core.PulseStatusResponse, error) {
	return SendHttp[any, core.PulseStatusResponse](ctx, c, "POST", c.adminPulseDisableUri, nil, nil)
}

func (c *OpenClawHttpClient) GetSecurityPosture(ctx context.Context) (*core.SecurityPostureResponse, error) {
	return SendHttp[any, core.SecurityPostureResponse](ctx, c, "GET", c.adminPostureUri, nil, nil)
}

func (c *OpenClawHttpClient) GetModelProfiles(ctx context.Context) (*core.ModelProfilesStatusResponse, error) {
	return SendHttp[any, core.ModelProfilesStatusResponse](ctx, c, "GET", c.adminModelsUri, nil, nil)
}

func (c *OpenClawHttpClient) GetModelSelectionDoctor(ctx context.Context) (*core.ModelSelectionDoctorResponse, error) {
	return SendHttp[any, core.ModelSelectionDoctorResponse](ctx, c, "GET", c.adminModelsDoctorUri, nil, nil)
}

func (c *OpenClawHttpClient) RunModelEvaluation(ctx context.Context, request core.ModelEvaluationRequest) (*core.ModelEvaluationReport, error) {
	return SendHttp[core.ModelEvaluationRequest, core.ModelEvaluationReport](ctx, c, "POST", c.adminModelEvaluationsUri, &request, nil)
}

func (c *OpenClawHttpClient) ListExternalCliConnectors(ctx context.Context) (*core.ExternalCliConnectorListResponse, error) {
	return SendHttp[any, core.ExternalCliConnectorListResponse](ctx, c, "GET", c.adminExternalCliConnectorsUri, nil, nil)
}

func (c *OpenClawHttpClient) GetExternalCliConnectorStatus(ctx context.Context, connector string) (*core.ExternalCliConnectorStatus, error) {
	if connector == "" {
		return nil, errors.New("Connector is required.")
	}
	requesturi := c.baseUri.JoinPath("/admin/external-cli/connectors").JoinPath(connector)
	return SendHttp[any, core.ExternalCliConnectorStatus](ctx, c, "GET", requesturi, nil, nil)
}

func (c *OpenClawHttpClient) ListExternalCliCommands(ctx context.Context, connector string) (*core.ExternalCliCommandListResponse, error) {
	if connector == "" {
		return nil, errors.New("Connector is required.")
	}
	requesturi := c.baseUri.JoinPath("/admin/external-cli/connectors").JoinPath(connector).JoinPath("/commands")
	return SendHttp[any, core.ExternalCliCommandListResponse](ctx, c, "GET", requesturi, nil, nil)
}

func (c *OpenClawHttpClient) PreviewExternalCli(ctx context.Context, request core.ExternalCliPreviewRequest) (*core.ExternalCliPreviewResponse, error) {
	return SendHttp[core.ExternalCliPreviewRequest, core.ExternalCliPreviewResponse](ctx, c, "POST", c.adminExternalCliPreviewUri, &request, nil)
}

func (c *OpenClawHttpClient) ExecuteExternalCli(ctx context.Context, request core.ExternalCliExecuteRequest) (*core.ExternalCliExecutionResult, error) {
	return SendHttp[core.ExternalCliExecuteRequest, core.ExternalCliExecutionResult](ctx, c, "POST", c.adminExternalCliExecuteUri, &request, nil)
}

func (c *OpenClawHttpClient) SimulateApproval(ctx context.Context, request core.ApprovalSimulationRequest) (*core.ApprovalSimulationResponse, error) {
	return SendHttp[core.ApprovalSimulationRequest, core.ApprovalSimulationResponse](ctx, c, "POST", c.adminApprovalSimulationUri, &request, nil)
}

func (c *OpenClawHttpClient) TestAccountResolution(ctx context.Context, request core.BackendCredentialResolutionRequest) (*core.BackendCredentialResolutionResponse, error) {
	return SendHttp[core.BackendCredentialResolutionRequest, core.BackendCredentialResolutionResponse](ctx, c, "POST", c.adminAccountResolutionUri, &request, nil)
}

func (c *OpenClawHttpClient) GetOperatorAccounts(ctx context.Context) (*core.OperatorAccountListResponse, error) {
	return SendHttp[any, core.OperatorAccountListResponse](ctx, c, "GET", c.adminOperatorAccountsUri, nil, nil)
}

func (c *OpenClawHttpClient) ExchangeOperatorToken(ctx context.Context, request core.OperatorTokenExchangeRequest) (*core.OperatorTokenExchangeResponse, error) {
	return SendHttp[core.OperatorTokenExchangeRequest, core.OperatorTokenExchangeResponse](ctx, c, "POST", c.authOperatorTokenUri, &request, nil)
}

func (c *OpenClawHttpClient) GetOperatorAccount(ctx context.Context, accountId string) (*core.OperatorAccountDetailResponse, error) {
	if accountId == "" {
		return nil, errors.New("Account id is required.")
	}
	return SendHttp[any, core.OperatorAccountDetailResponse](ctx, c, "GET", c.adminOperatorAccountsUri.JoinPath(accountId), nil, nil)
}

func (c *OpenClawHttpClient) CreateOperatorAccount(ctx context.Context, request core.OperatorAccountCreateRequest) (*core.OperatorAccountDetailResponse, error) {
	return SendHttp[core.OperatorAccountCreateRequest, core.OperatorAccountDetailResponse](ctx, c, "POST", c.adminOperatorAccountsUri, &request, nil)
}

func (c *OpenClawHttpClient) UpdateOperatorAccount(ctx context.Context, accountId string, request core.OperatorAccountUpdateRequest) (*core.OperatorAccountDetailResponse, error) {
	if accountId == "" {
		return nil, errors.New("Account id is required.")
	}
	return SendHttp[core.OperatorAccountUpdateRequest, core.OperatorAccountDetailResponse](ctx, c, "PUT", c.adminOperatorAccountsUri.JoinPath(accountId), &request, nil)
}

func (c *OpenClawHttpClient) DeleteOperatorAccount(ctx context.Context, accountId string) (*core.MutationResponse, error) {
	if accountId == "" {
		return nil, errors.New("Account id is required.")
	}
	return SendHttp[any, core.MutationResponse](ctx, c, "DELETE", c.adminOperatorAccountsUri.JoinPath(accountId), nil, nil)
}

func (c *OpenClawHttpClient) CreateOperatorAccountToken(ctx context.Context, accountId string, request core.OperatorAccountTokenCreateRequest) (*core.OperatorAccountTokenCreateResponse, error) {
	if accountId == "" {
		return nil, errors.New("Account id is required.")
	}
	return SendHttp[core.OperatorAccountTokenCreateRequest, core.OperatorAccountTokenCreateResponse](ctx, c, "POST", c.adminOperatorAccountsUri.JoinPath(accountId).JoinPath("/tokens"), &request, nil)
}

func (c *OpenClawHttpClient) RevokeOperatorAccountToken(ctx context.Context, accountId, tokenId string) (*core.MutationResponse, error) {
	if accountId == "" {
		return nil, errors.New("Account id is required.")
	}
	if tokenId == "" {
		return nil, errors.New("token id is required.")
	}
	return SendHttp[any, core.MutationResponse](ctx, c, "DELETE", c.adminOperatorAccountsUri.JoinPath(accountId).JoinPath("/tokens").JoinPath(tokenId), nil, nil)
}

func (c *OpenClawHttpClient) GetOrganizationPolicy(ctx context.Context) (*core.OrganizationPolicyResponse, error) {
	return SendHttp[any, core.OrganizationPolicyResponse](ctx, c, "GET", c.adminOrganizationPolicyUri, nil, nil)
}

func (c *OpenClawHttpClient) SaveOrganizationPolicy(ctx context.Context, request core.OrganizationPolicySnapshot) (*core.OrganizationPolicyResponse, error) {
	return SendHttp[core.OrganizationPolicySnapshot, core.OrganizationPolicyResponse](ctx, c, "POST", c.adminOrganizationPolicyUri, &request, nil)
}

func buildDateRangeUri(baseUri *url.URL, fromUtc, toUtc *time.Time) *url.URL {
	resultUri := *baseUri
	query := resultUri.Query()
	if fromUtc != nil {
		query.Set("fromUtc", fromUtc.Format(time.RFC3339Nano))
	}

	if toUtc != nil {
		query.Set("toUtc", toUtc.Format(time.RFC3339Nano))
	}

	resultUri.RawQuery = query.Encode()

	return &resultUri
}

func (c *OpenClawHttpClient) GetSetupStatus(ctx context.Context) (*core.SetupStatusResponse, error) {
	return SendHttp[any, core.SetupStatusResponse](ctx, c, "GET", c.adminSetupStatusUri, nil, nil)
}

func (c *OpenClawHttpClient) GetOperatorInsights(ctx context.Context, fromUtc, toUtc *time.Time) (*core.OperatorInsightsResponse, error) {
	return SendHttp[any, core.OperatorInsightsResponse](ctx, c, "GET", buildDateRangeUri(c.adminInsightsUri, fromUtc, toUtc), nil, nil)
}

func (c *OpenClawHttpClient) GetObservabilitySummary(ctx context.Context, fromUtc, toUtc *time.Time) (*core.ObservabilitySummaryResponse, error) {
	return SendHttp[any, core.ObservabilitySummaryResponse](ctx, c, "GET", buildDateRangeUri(c.adminObservabilitySummaryUri, fromUtc, toUtc), nil, nil)
}

func (c *OpenClawHttpClient) GetObservabilitySeries(ctx context.Context, fromUtc, toUtc *time.Time, bucketMinutes int) (*core.ObservabilitySeriesResponse, error) {
	resultUri := *c.adminObservabilitySeriesUri
	if bucketMinutes <= 0 {
		bucketMinutes = 60
	}
	query := resultUri.Query()
	query.Set("bucketMinutes", fmt.Sprintf("%d", util.Clamp(bucketMinutes, 5, 24*60)))
	if fromUtc != nil {
		query.Set("fromUtc", fromUtc.Format(time.RFC3339Nano))
	}

	if toUtc != nil {
		query.Set("toUtc", toUtc.Format(time.RFC3339Nano))
	}

	resultUri.RawQuery = query.Encode()

	return SendHttp[any, core.ObservabilitySeriesResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) ExportAuditBundle(ctx context.Context, fromUtc, toUtc *time.Time) ([]byte, error) {
	requesturi := buildDateRangeUri(c.adminAuditExportUri, fromUtc, toUtc)
	request, err := http.NewRequestWithContext(ctx, "GET", requesturi.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Error: Failed to search (HTTP %d)", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (c *OpenClawHttpClient) ExportTrajectoryJsonl(ctx context.Context, fromUtc, toUtc *time.Time, sessionId string, anonymize bool) (string, error) {
	resultUri := *c.adminTrajectoryExportUri
	query := resultUri.Query()
	if fromUtc != nil {
		query.Set("fromUtc", fromUtc.Format(time.RFC3339Nano))
	}
	if toUtc != nil {
		query.Set("toUtc", toUtc.Format(time.RFC3339Nano))
	}
	if sessionId != "" {
		query.Set("sessionId", sessionId)
	}
	if anonymize {
		query.Set("anonymize", strconv.FormatBool(anonymize))
	}

	resultUri.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, "GET", resultUri.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Error: Failed to search (HTTP %d)", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (c *OpenClawHttpClient) ExportIncidentBundle(ctx context.Context, approvalLimit, eventLimit int) (*core.IncidentBundleResponse, error) {
	resultUri := *c.adminIncidentExportUri
	query := resultUri.Query()
	query.Set("approvalLimit", fmt.Sprintf("%d", util.Clamp(approvalLimit, 1, 500)))
	query.Set("eventLimit", fmt.Sprintf("%d", util.Clamp(eventLimit, 1, 500)))
	resultUri.RawQuery = query.Encode()

	return SendHttp[any, core.IncidentBundleResponse](ctx, c, "GET", &resultUri, nil, nil)
}

func (c *OpenClawHttpClient) GetWhatsAppSetup(ctx context.Context) (*core.WhatsAppSetupResponse, error) {
	return SendHttp[any, core.WhatsAppSetupResponse](ctx, c, "GET", c.adminWhatsAppSetupUri, nil, nil)
}

func (c *OpenClawHttpClient) SaveWhatsAppSetup(ctx context.Context, request core.WhatsAppSetupRequest) (*core.WhatsAppSetupResponse, error) {
	return SendHttp[core.WhatsAppSetupRequest, core.WhatsAppSetupResponse](ctx, c, "PUT", c.adminWhatsAppSetupUri, &request, nil)
}

func (c *OpenClawHttpClient) RestartWhatsApp(ctx context.Context) (*core.WhatsAppSetupResponse, error) {
	return SendHttp[any, core.WhatsAppSetupResponse](ctx, c, "POST", c.adminWhatsAppRestartUri, nil, nil)
}

func (c *OpenClawHttpClient) GetChannelAuth(ctx context.Context, channelId, accountId string) (*core.ChannelAuthStatusResponse, error) {
	if channelId == "" {
		return nil, errors.New("Channel id is required")
	}
	resultUri := c.baseUri.JoinPath("/admin/channels").JoinPath(channelId)
	query := resultUri.Query()
	if accountId != "" {
		query.Set("accountId", accountId)
	}
	resultUri.RawQuery = query.Encode()

	return SendHttp[any, core.ChannelAuthStatusResponse](ctx, c, "GET", resultUri, nil, nil)
}

func (c *OpenClawHttpClient) StreamChannelAuth(
	ctx context.Context,
	channelId, accountId string,
	onEvent func(core.ChannelAuthStatusItem),
) error {
	resulturi := c.baseUri.JoinPath("/admin/channels").JoinPath(channelId).JoinPath("/auth/stream")
	query := resulturi.Query()
	if accountId != "" {
		query.Set("accountId", accountId)
	}
	resulturi.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resulturi.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(line[len("data:"):])
		if len(data) == 0 {
			continue
		}

		if data == "[DONE]" {
			break
		}

		var event core.ChannelAuthStatusItem
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("failed to parse SSE chunk: %s: %w", data, err)
		}

		if onEvent != nil {
			onEvent(event)
		}
	}

	return scanner.Err()
}
