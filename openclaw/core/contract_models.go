package core

import "time"

// Verification Kinds
const (
	VerificationKindFileExists       = "file_exists"
	VerificationKindFileContains     = "file_contains"
	VerificationKindHttpStatus       = "http_status"
	VerificationKindHttpBodyContains = "http_body_contains"
	VerificationKindOperatorConfirm  = "operator_confirm"
)

const (
	AutomationVerificationStatusNotRun = "not_run"
	AutomationLifecycleStateRunning    = "running"
)

type VerificationPolicy struct {
	Checks []*VerificationCheckDefinition `json:"checks"`
}

func DefaultVerificationPolicy() *VerificationPolicy {
	return &VerificationPolicy{
		Checks: make([]*VerificationCheckDefinition, 0),
	}
}

type VerificationCheckDefinition struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"`
	Path               string `json:"path,omitempty"`
	URL                string `json:"url,omitempty"`
	Contains           string `json:"contains,omitempty"`
	ExpectedStatusCode *int   `json:"expected_status_code,omitempty"`
	Prompt             string `json:"prompt,omitempty"`
}

type VerificationCheckResult struct {
	CheckID        string    `json:"check_id"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	Summary        string    `json:"summary"`
	EvaluatedAtUTC time.Time `json:"evaluated_at_utc"`
}

func DefaultVerificationCheckResult() *VerificationCheckResult {
	return &VerificationCheckResult{
		Status:         AutomationVerificationStatusNotRun,
		EvaluatedAtUTC: time.Now().UTC(),
	}
}

// ContractPolicy Optional governance policy that can be attached to a session to enforce
// pre-flight capability validation, USD cost budgets, and scoped tool access.
type ContractPolicy struct {
	ID                  string              `json:"id"`                              // Unique contract identifier (e.g. "ctr_" + guid prefix).
	Name                string              `json:"name,omitempty"`                  // Human-readable label for this contract.
	RequiredRuntimeMode string              `json:"required_runtime_mode,omitempty"` // Required runtime mode (null = any, "aot", "jit").
	RequestedTools      []string            `json:"requested_tools"`                 // Tools this contract expects to use.
	ScopedCapabilities  []*ScopedCapability `json:"scoped_capabilities"`             // Path-scoped tool restrictions.
	MaxCostUSD          float64             `json:"max_cost_usd"`                    // Maximum USD spend for the session. 0 = unlimited.
	SoftCostWarningUSD  float64             `json:"soft_cost_warning_usd"`           // Soft USD warning threshold. 0 = no warning.
	MaxTokens           int64               `json:"max_tokens"`                      // Maximum total tokens (input + output). 0 = unlimited.
	MaxToolCalls        int                 `json:"max_tool_calls"`                  // Maximum number of tool calls. 0 = unlimited.
	MaxRuntimeSeconds   int                 `json:"max_runtime_seconds"`             // Maximum runtime in seconds. 0 = unlimited.
	CreatedBy           string              `json:"created_by,omitempty"`            // Who created this contract (operator, API caller, etc.).
	Verification        *VerificationPolicy `json:"verification,omitempty"`          // Optional post-run verification checks.
	CreatedAtUTC        time.Time           `json:"created_at_utc"`
}

func DefaultContractPolicy() *ContractPolicy {
	return &ContractPolicy{
		RequestedTools:     make([]string, 0),
		ScopedCapabilities: make([]*ScopedCapability, 0),
		CreatedAtUTC:       time.Now().UTC(),
	}
}

// ScopedCapability Path-scoped restriction for a specific tool.
type ScopedCapability struct {
	ToolName     string   `json:"tool_name"`     // Tool name this scope applies to (e.g. "file_read", "file_write").
	AllowedPaths []string `json:"allowed_paths"` // Allowed filesystem roots.
}

func DefaultScopedCapability() *ScopedCapability {
	return &ScopedCapability{
		AllowedPaths: make([]string, 0),
	}
}

// ContractValidationResult Result of pre-flight contract validation.
type ContractValidationResult struct {
	IsValid              bool     `json:"is_valid"`
	GrantedTools         []string `json:"granted_tools"`
	DeniedTools          []string `json:"denied_tools"`
	Errors               []string `json:"errors"`
	Warnings             []string `json:"warnings"`
	EffectiveRuntimeMode string   `json:"effective_runtime_mode,omitempty"`
}

func DefaultContractValidationResult() *ContractValidationResult {
	return &ContractValidationResult{
		GrantedTools: make([]string, 0),
		DeniedTools:  make([]string, 0),
		Errors:       make([]string, 0),
		Warnings:     make([]string, 0),
	}
}

// ContractExecutionSnapshot Point-in-time snapshot of a contract-governed session's execution metrics.
type ContractExecutionSnapshot struct {
	ContractID                 string                     `json:"contract_id"`
	SessionID                  string                     `json:"session_id"`
	Status                     string                     `json:"status"` // active | completed | budget_exceeded | cancelled
	AccumulatedCostUSD         float64                    `json:"accumulated_cost_usd"`
	AccumulatedTokens          int64                      `json:"accumulated_tokens"`
	ToolCallCount              int                        `json:"tool_call_count"`
	ElapsedSeconds             float64                    `json:"elapsed_seconds"`
	StartedAtUTC               time.Time                  `json:"started_at_utc"`
	EndedAtUTC                 *time.Time                 `json:"ended_at_utc,omitempty"`
	LifecycleState             string                     `json:"lifecycle_state"`
	VerificationStatus         string                     `json:"verification_status"`
	VerificationSummary        string                     `json:"verification_summary,omitempty"`
	VerificationCompletedAtUTC *time.Time                 `json:"verification_completed_at_utc,omitempty"`
	VerificationChecks         []*VerificationCheckResult `json:"verification_checks"`
}

func DefaultContractExecutionSnapshot() *ContractExecutionSnapshot {
	return &ContractExecutionSnapshot{
		Status:             "active",
		LifecycleState:     AutomationLifecycleStateRunning,
		VerificationStatus: AutomationVerificationStatusNotRun,
		VerificationChecks: make([]*VerificationCheckResult, 0),
	}
}
