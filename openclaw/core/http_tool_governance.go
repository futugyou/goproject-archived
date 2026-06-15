package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var _ IToolGovernanceService = (*HttpSidecarToolGovernanceService)(nil)

type HttpSidecarToolGovernanceService struct {
	httpClient *http.Client
	config     *ToolGovernanceConfig
	logger     *slog.Logger
}

func NewHttpSidecarToolGovernanceService(
	httpClient *http.Client,
	config *ToolGovernanceConfig,
	logger *slog.Logger,
) *HttpSidecarToolGovernanceService {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &HttpSidecarToolGovernanceService{
		httpClient: httpClient,
		config:     config,
		logger:     logger,
	}
}

// Authorize implements [IToolGovernanceService].
func (s *HttpSidecarToolGovernanceService) Authorize(ctx context.Context, governanceCtx ToolGovernanceContext) (*GovernanceDecision, error) {
	if !s.config.Enabled {
		return NewGovernanceDecisionAllow("Governance disabled"), nil
	}

	// 处理超时
	var cancel context.CancelFunc
	if s.config.TimeoutMs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.config.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	requestBody := ToolGovernanceSidecarRequest{
		AgentId:          governanceCtx.AgentId,
		ConversationId:   governanceCtx.SessionId,
		SessionId:        governanceCtx.SessionId,
		ChannelId:        governanceCtx.ChannelId,
		UserId:           governanceCtx.SenderId,
		TraceId:          governanceCtx.CorrelationId,
		CallId:           governanceCtx.CallId,
		ToolName:         governanceCtx.ToolName,
		ToolCategory:     governanceCtx.Descriptor.Category,
		RiskLevel:        fmt.Sprintf("%v", governanceCtx.Descriptor.RiskLevel),
		ArgumentsJson:    governanceCtx.ArgumentsJson,
		ActionDescriptor: governanceCtx.ActionDescriptor,
		Descriptor:       governanceCtx.Descriptor,
	}

	endpoint := s.normalizeEndpoint(s.config.DecisionEndpoint, "/api/v1/execute")

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return s.buildUnavailableDecision(governanceCtx.Descriptor, governanceCtx.ToolName, fmt.Sprintf("Failed to marshal request: %v", err)), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return s.buildUnavailableDecision(governanceCtx.Descriptor, governanceCtx.ToolName, fmt.Sprintf("Failed to create request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Go 中通过判断 ctx.Err() 来识别是否因超时或手动取消导致
		if ctx.Err() == context.DeadlineExceeded {
			return s.buildUnavailableDecision(governanceCtx.Descriptor, governanceCtx.ToolName, "Governance sidecar timed out"), nil
		}
		return s.buildUnavailableDecision(governanceCtx.Descriptor, governanceCtx.ToolName, fmt.Sprintf("Governance sidecar unavailable: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.buildUnavailableDecision(
			governanceCtx.Descriptor,
			governanceCtx.ToolName,
			fmt.Sprintf("Governance sidecar returned %d %s", resp.StatusCode, resp.Status),
		), nil
	}

	var sidecarResponse ToolGovernanceSidecarResponse
	if err := json.NewDecoder(resp.Body).Decode(&sidecarResponse); err != nil {
		return s.buildUnavailableDecision(governanceCtx.Descriptor, governanceCtx.ToolName, "Governance sidecar returned an empty or invalid response"), nil
	}

	action := s.mapAction(sidecarResponse.Action)
	var allowed bool

	switch action {
	case GovernanceActionDeny:
		allowed = false
	case GovernanceActionRequireApproval:
		allowed = true
	case GovernanceActionAllow, GovernanceActionAuditOnly, GovernanceActionRedact:
		if sidecarResponse.Allowed != nil {
			allowed = *sidecarResponse.Allowed
		} else {
			allowed = true
		}
	default:
		allowed = false
	}

	return &GovernanceDecision{
		Allowed:                  allowed,
		Action:                   action,
		Reason:                   sidecarResponse.Reason,
		TrustScore:               sidecarResponse.TrustScore,
		PolicyId:                 sidecarResponse.PolicyId,
		RuleId:                   sidecarResponse.RuleId,
		EvaluationMs:             sidecarResponse.EvaluationMs,
		RedactedArgumentsJson:    sidecarResponse.RedactedArgumentsJson,
		ReplacementArgumentsJson: sidecarResponse.ReplacementArgumentsJson,
	}, nil
}

// RecordResult implements [IToolGovernanceService].
func (s *HttpSidecarToolGovernanceService) RecordResult(ctx context.Context, governanceCtx ToolGovernanceContext, decision GovernanceDecision, result ToolGovernanceExecutionResult) error {
	if !s.config.Enabled || !s.config.AuditResults || strings.TrimSpace(s.config.ResultEndpoint) == "" {
		return nil
	}

	var cancel context.CancelFunc
	if s.config.TimeoutMs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.config.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	requestBody := ToolGovernanceSidecarResultRequest{
		AgentId:        governanceCtx.AgentId,
		ConversationId: governanceCtx.SessionId,
		SessionId:      governanceCtx.SessionId,
		ChannelId:      governanceCtx.ChannelId,
		UserId:         governanceCtx.SenderId,
		TraceId:        governanceCtx.CorrelationId,
		CallId:         governanceCtx.CallId,
		ToolName:       governanceCtx.ToolName,
		Descriptor:     governanceCtx.Descriptor,
		Decision:       &decision,
		Result:         &result,
	}

	endpoint := s.normalizeEndpoint(s.config.ResultEndpoint, "/api/v1/result")

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		s.logger.Warn("Governance sidecar result audit failed", "tool", governanceCtx.ToolName, "error", err)
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		s.logger.Warn("Governance sidecar result audit failed", "tool", governanceCtx.ToolName, "error", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Warn("Governance sidecar result audit failed", "tool", governanceCtx.ToolName, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Warn(
			"Governance sidecar result audit failed",
			"tool", governanceCtx.ToolName,
			"statusCode", resp.StatusCode,
		)
	}

	return nil
}
func (s *HttpSidecarToolGovernanceService) buildUnavailableDecision(
	descriptor ToolGovernanceDescriptor,
	toolName string,
	reason string,
) *GovernanceDecision {

	if s.shouldFailClosed(descriptor) {
		return &GovernanceDecision{
			Allowed:       false,
			Action:        GovernanceActionDeny,
			Reason:        reason,
			IsUnavailable: true,
		}
	}

	s.logger.Warn(
		"Governance sidecar unavailable for tool. Continuing because low-risk fail-open is enabled.",
		"tool", toolName,
		"reason", reason,
	)

	return &GovernanceDecision{
		Allowed:       true,
		Action:        GovernanceActionAuditOnly,
		Reason:        reason,
		IsUnavailable: true,
	}
}

func (s *HttpSidecarToolGovernanceService) shouldFailClosed(descriptor ToolGovernanceDescriptor) bool {
	if s.config.RequireGovernanceForHighRiskTools && s.isHighRiskOrSideEffecting(descriptor) {
		return true
	}

	if s.isLowRiskReadOnly(descriptor) && s.config.FailOpenReadOnlyLowRisk {
		return false
	}

	return s.config.FailClosed
}

func (s *HttpSidecarToolGovernanceService) isHighRiskOrSideEffecting(descriptor ToolGovernanceDescriptor) bool {
	if descriptor.RiskLevel == ToolGovernanceRiskLevelHigh || descriptor.RiskLevel == ToolGovernanceRiskLevelCritical {
		return true
	}
	if !descriptor.ReadOnly || descriptor.CanExecuteCode || descriptor.CanSendDataExternally {
		return true
	}

	highRiskCapabilities := map[string]bool{
		"process.execute":  true,
		"filesystem.write": true,
		"external.http":    true,
		"data.export":      true,
		"message.send":     true,
	}

	for _, capability := range descriptor.Capabilities {
		if highRiskCapabilities[capability] {
			return true
		}
	}

	return false
}

func (s *HttpSidecarToolGovernanceService) isLowRiskReadOnly(descriptor ToolGovernanceDescriptor) bool {
	return descriptor.ReadOnly &&
		descriptor.RiskLevel == ToolGovernanceRiskLevelLow &&
		!descriptor.CanExecuteCode &&
		!descriptor.CanSendDataExternally
}

func (s *HttpSidecarToolGovernanceService) normalizeEndpoint(endpoint string, fallback string) string {
	if strings.TrimSpace(endpoint) == "" {
		return fallback
	}
	return strings.TrimSpace(endpoint)
}

func (s *HttpSidecarToolGovernanceService) mapAction(action string) GovernanceAction {
	normalized := strings.TrimSpace(action)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ToLower(normalized)

	switch normalized {
	case "allow":
		return GovernanceActionAllow
	case "deny":
		return GovernanceActionDeny
	case "require_approval", "requireapproval":
		return GovernanceActionRequireApproval
	case "redact":
		return GovernanceActionRedact
	case "audit_only", "auditonly", "log", "warn":
		return GovernanceActionAuditOnly
	default:
		return GovernanceActionDeny
	}
}
