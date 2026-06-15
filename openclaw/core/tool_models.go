package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type GovernanceAction int

const (
	GovernanceActionAllow GovernanceAction = iota
	GovernanceActionDeny
	GovernanceActionRequireApproval
	GovernanceActionRedact
	GovernanceActionAuditOnly
)

var (
	governanceActionToString = map[GovernanceAction]string{
		GovernanceActionAllow:           "allow",
		GovernanceActionDeny:            "deny",
		GovernanceActionRequireApproval: "require_approval",
		GovernanceActionRedact:          "redact",
		GovernanceActionAuditOnly:       "audit_only",
	}

	stringToGovernanceAction = map[string]GovernanceAction{
		"allow":            GovernanceActionAllow,
		"deny":             GovernanceActionDeny,
		"require_approval": GovernanceActionRequireApproval,
		"redact":           GovernanceActionRedact,
		"audit_only":       GovernanceActionAuditOnly,
	}
)

func (g GovernanceAction) MarshalJSON() ([]byte, error) {
	if s, ok := governanceActionToString[g]; ok {
		return json.Marshal(s)
	}
	return nil, fmt.Errorf("invalid GovernanceAction value: %d", g)
}

func (g *GovernanceAction) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if v, ok := stringToGovernanceAction[s]; ok {
		*g = v
		return nil
	}
	return fmt.Errorf("invalid GovernanceAction string: %s", s)
}

// ============================================================================
// Enums (ToolGovernanceRiskLevel)
// ============================================================================

type ToolGovernanceRiskLevel int

const (
	ToolGovernanceRiskLevelLow ToolGovernanceRiskLevel = iota
	ToolGovernanceRiskLevelMedium
	ToolGovernanceRiskLevelHigh
	ToolGovernanceRiskLevelCritical
)

var (
	riskLevelToString = map[ToolGovernanceRiskLevel]string{
		ToolGovernanceRiskLevelLow:      "low",
		ToolGovernanceRiskLevelMedium:   "medium",
		ToolGovernanceRiskLevelHigh:     "high",
		ToolGovernanceRiskLevelCritical: "critical",
	}

	stringToRiskLevel = map[string]ToolGovernanceRiskLevel{
		"low":      ToolGovernanceRiskLevelLow,
		"medium":   ToolGovernanceRiskLevelMedium,
		"high":     ToolGovernanceRiskLevelHigh,
		"critical": ToolGovernanceRiskLevelCritical,
	}
)

func (r ToolGovernanceRiskLevel) MarshalJSON() ([]byte, error) {
	if s, ok := riskLevelToString[r]; ok {
		return json.Marshal(s)
	}
	return nil, fmt.Errorf("invalid ToolGovernanceRiskLevel value: %d", r)
}

func (r *ToolGovernanceRiskLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if v, ok := stringToRiskLevel[s]; ok {
		*r = v
		return nil
	}
	return fmt.Errorf("invalid ToolGovernanceRiskLevel string: %s", s)
}

// ============================================================================
// Constants (ToolGovernanceProviders)
// ============================================================================

const (
	ToolGovernanceProvidersNone        = "none"
	ToolGovernanceProvidersHttpSidecar = "http_sidecar"
)

// ============================================================================
// Structs & Records
// ============================================================================

type ToolGovernanceConfig struct {
	Enabled                           bool    `json:"enabled"`
	Provider                          string  `json:"provider"`
	SidecarBaseUrl                    *string `json:"sidecar_base_url,omitempty"`
	DecisionEndpoint                  string  `json:"decision_endpoint"`
	ResultEndpoint                    string  `json:"result_endpoint"`
	TimeoutMs                         int     `json:"timeout_ms"`
	AuditResults                      bool    `json:"audit_results"`
	FailClosed                        bool    `json:"fail_closed"`
	FailOpenReadOnlyLowRisk           bool    `json:"fail_open_read_only_low_risk"`
	RequireGovernanceForHighRiskTools bool    `json:"require_governance_for_high_risk_tools"`
}

func DefaultToolGovernanceConfig() ToolGovernanceConfig {
	return ToolGovernanceConfig{
		Enabled:                           false,
		Provider:                          ToolGovernanceProvidersHttpSidecar,
		DecisionEndpoint:                  "/api/v1/execute",
		TimeoutMs:                         300,
		AuditResults:                      true,
		FailClosed:                        true,
		FailOpenReadOnlyLowRisk:           false,
		RequireGovernanceForHighRiskTools: true,
	}
}

type GovernanceDecision struct {
	Allowed                  bool             `json:"allowed"`
	Reason                   string           `json:"reason,omitempty"`
	TrustScore               *float64         `json:"trust_score,omitempty"`
	PolicyId                 *string          `json:"policy_id,omitempty"`
	RuleId                   *string          `json:"rule_id,omitempty"`
	Action                   GovernanceAction `json:"action"`
	EvaluationMs             *float64         `json:"evaluation_ms,omitempty"`
	IsUnavailable            bool             `json:"is_unavailable"`
	RedactedArgumentsJson    *string          `json:"redacted_arguments_json,omitempty"`
	ReplacementArgumentsJson *string          `json:"replacement_arguments_json,omitempty"`
}

func DefaultGovernanceDecision() *GovernanceDecision {
	return &GovernanceDecision{
		Action: GovernanceActionAllow,
	}
}

func NewGovernanceDecisionAllow(reason string) *GovernanceDecision {
	return &GovernanceDecision{
		Allowed: true,
		Action:  GovernanceActionAllow,
		Reason:  reason,
	}
}

type ToolGovernanceDescriptor struct {
	Name                  string                  `json:"name"`
	Description           string                  `json:"description"`
	Category              string                  `json:"category"`
	RiskLevel             ToolGovernanceRiskLevel `json:"risk_level"`
	RequiresApproval      bool                    `json:"requires_approval"`
	ReadOnly              bool                    `json:"read_only"`
	CanAccessNetwork      bool                    `json:"can_access_network"`
	CanAccessFileSystem   bool                    `json:"can_access_file_system"`
	CanExecuteCode        bool                    `json:"can_execute_code"`
	CanSendDataExternally bool                    `json:"can_send_data_externally"`
	Capabilities          []string                `json:"capabilities"`
}

func DefaultToolGovernanceDescriptor() ToolGovernanceDescriptor {
	return ToolGovernanceDescriptor{
		Description:  "",
		Category:     "plugin",
		RiskLevel:    ToolGovernanceRiskLevelMedium,
		ReadOnly:     true,
		Capabilities: []string{},
	}
}

type ToolGovernanceContext struct {
	AgentId          string                   `json:"agent_id"`
	SessionId        string                   `json:"session_id"`
	ChannelId        string                   `json:"channel_id"`
	SenderId         string                   `json:"sender_id"`
	CorrelationId    string                   `json:"correlation_id"`
	CallId           *string                  `json:"call_id,omitempty"`
	ToolName         string                   `json:"tool_name"`
	ArgumentsJson    string                   `json:"arguments_json"`
	ActionDescriptor ToolActionDescriptor     `json:"action_descriptor"`
	Descriptor       ToolGovernanceDescriptor `json:"descriptor"`
	IsStreaming      bool                     `json:"is_streaming"`
}

type ToolGovernanceExecutionResult struct {
	ResultStatus   string  `json:"result_status"`
	FailureCode    *string `json:"failure_code,omitempty"`
	FailureMessage *string `json:"failure_message,omitempty"`
	Failed         bool    `json:"failed"`
	TimedOut       bool    `json:"timed_out"`
	DurationMs     float64 `json:"duration_ms"`
	ResultBytes    int     `json:"result_bytes"`
}

type ToolGovernanceSidecarRequest struct {
	AgentId          string                   `json:"agent_id"`
	ConversationId   string                   `json:"conversation_id"`
	SessionId        string                   `json:"session_id"`
	ChannelId        string                   `json:"channel_id"`
	UserId           string                   `json:"user_id"`
	TraceId          string                   `json:"trace_id"`
	CallId           *string                  `json:"call_id,omitempty"`
	ToolName         string                   `json:"tool_name"`
	ToolCategory     string                   `json:"tool_category"`
	RiskLevel        string                   `json:"risk_level"`
	ArgumentsJson    string                   `json:"arguments_json"`
	ActionDescriptor ToolActionDescriptor     `json:"action_descriptor"`
	Descriptor       ToolGovernanceDescriptor `json:"descriptor"`
}

type ToolGovernanceSidecarResponse struct {
	Allowed                  *bool    `json:"allowed,omitempty"`
	Reason                   string   `json:"reason"`
	TrustScore               *float64 `json:"trust_score,omitempty"`
	PolicyId                 *string  `json:"policy_id,omitempty"`
	RuleId                   *string  `json:"rule_id,omitempty"`
	Action                   string   `json:"action"`
	EvaluationMs             *float64 `json:"evaluation_ms,omitempty"`
	RedactedArgumentsJson    *string  `json:"redacted_arguments_json,omitempty"`
	ReplacementArgumentsJson *string  `json:"replacement_arguments_json,omitempty"`
}

type ToolGovernanceSidecarResultRequest struct {
	AgentId        string                         `json:"agent_id"`
	ConversationId string                         `json:"conversation_id"`
	SessionId      string                         `json:"session_id"`
	ChannelId      string                         `json:"channel_id"`
	UserId         string                         `json:"user_id"`
	TraceId        string                         `json:"trace_id"`
	CallId         *string                        `json:"call_id,omitempty"`
	ToolName       string                         `json:"tool_name"`
	Descriptor     ToolGovernanceDescriptor       `json:"descriptor"`
	Decision       *GovernanceDecision            `json:"decision"`
	Result         *ToolGovernanceExecutionResult `json:"result"`
}

type CaseInsensitiveSet map[string]struct{}

func NewCaseInsensitiveSet() CaseInsensitiveSet {
	return make(CaseInsensitiveSet)
}

func (s CaseInsensitiveSet) Add(val string) {
	s[strings.ToLower(val)] = struct{}{}
}

func (s CaseInsensitiveSet) Contains(val string) bool {
	_, exists := s[strings.ToLower(val)]
	return exists
}

func (s *CaseInsensitiveSet) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*s = NewCaseInsensitiveSet()
		return nil
	}

	var slice []string
	if err := json.Unmarshal(data, &slice); err != nil {
		return fmt.Errorf("expected a JSON array for CaseInsensitiveSet: %w", err)
	}

	set := NewCaseInsensitiveSet()
	for _, item := range slice {
		set.Add(item)
	}
	*s = set
	return nil
}

func (s CaseInsensitiveSet) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}

	slice := make([]string, 0, len(s))
	for item := range s {
		slice = append(slice, item)
	}
	return json.Marshal(slice)
}

type ToolsetConfig struct {
	AllowTools    []string `json:"allow_tools"`
	AllowPrefixes []string `json:"allow_prefixes"`
	DenyTools     []string `json:"deny_tools"`
	DenyPrefixes  []string `json:"deny_prefixes"`
}

type ToolPresetConfig struct {
	Toolsets              []string `json:"toolsets"`
	AllowTools            []string `json:"allow_tools"`
	AllowPrefixes         []string `json:"allow_prefixes"`
	DenyTools             []string `json:"deny_tools"`
	DenyPrefixes          []string `json:"deny_prefixes"`
	ApprovalRequiredTools []string `json:"approval_required_tools"`
	AutonomyMode          *string  `json:"autonomy_mode,omitempty"`
	RequireToolApproval   *bool    `json:"require_tool_approval,omitempty"`
	Description           string   `json:"description"`
}

type ResolvedToolPreset struct {
	PresetId              string             `json:"preset_id"`
	Description           string             `json:"description"`
	Surface               string             `json:"surface"`
	EffectiveAutonomyMode string             `json:"effective_autonomy_mode"`
	RequireToolApproval   bool               `json:"require_tool_approval"`
	AllowedTools          CaseInsensitiveSet `json:"allowed_tools"`
	ApprovalRequiredTools CaseInsensitiveSet `json:"approval_required_tools"`
}

type ToolActionDescriptor struct {
	Action              string  `json:"action"`
	IsMutation          bool    `json:"is_mutation"`
	RequiresApproval    bool    `json:"requires_approval"`
	Summary             string  `json:"summary"`
	ApprovalFingerprint *string `json:"approval_fingerprint,omitempty"`
	RiskLevel           *string `json:"risk_level,omitempty"`
	ReadOnly            *bool   `json:"read_only,omitempty"`
}

type ToolApprovalRequest struct {
	ApprovalId string    `json:"approval_id"`
	SessionId  string    `json:"session_id"`
	ChannelId  string    `json:"channel_id"`
	SenderId   string    `json:"sender_id"`
	ToolName   string    `json:"tool_name"`
	Arguments  string    `json:"arguments"`
	Action     string    `json:"action"`
	IsMutation bool      `json:"is_mutation"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}

type ToolExecutionContext struct {
	Session     *Session     `json:"session"`
	TurnContext *TurnContext `json:"turn_context"`
}

type ToolHookContext struct {
	SessionId     string `json:"session_id"`
	ChannelId     string `json:"channel_id"`
	SenderId      string `json:"sender_id"`
	CorrelationId string `json:"correlation_id"`
	ToolName      string `json:"tool_name"`
	ArgumentsJson string `json:"arguments_json"`
	IsStreaming   bool   `json:"is_streaming"`
}
