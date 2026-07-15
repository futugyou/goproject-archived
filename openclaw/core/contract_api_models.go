package core

type ContractCreateRequest struct {
	SessionID           string              `json:"session_id,omitempty"`
	Name                string              `json:"name,omitempty"`
	RequiredRuntimeMode string              `json:"required_runtime_mode,omitempty"`
	RequestedTools      []string            `json:"requested_tools,omitempty"`
	ScopedCapabilities  []ScopedCapability  `json:"scoped_capabilities,omitempty"`
	MaxCostUSD          float64             `json:"max_cost_usd"`
	SoftCostWarningUSD  float64             `json:"soft_cost_warning_usd"`
	MaxTokens           int64               `json:"max_tokens"`
	MaxToolCalls        int                 `json:"max_tool_calls"`
	MaxRuntimeSeconds   int                 `json:"max_runtime_seconds"`
	CreatedBy           string              `json:"created_by,omitempty"`
	Verification        *VerificationPolicy `json:"verification,omitempty"`
}

type ContractCreateResponse struct {
	Policy     ContractPolicy           `json:"policy"`
	Validation ContractValidationResult `json:"validation"`
}

type ContractValidateRequest struct {
	RequiredRuntimeMode string              `json:"required_runtime_mode,omitempty"`
	RequestedTools      []string            `json:"requested_tools,omitempty"`
	ScopedCapabilities  []ScopedCapability  `json:"scoped_capabilities,omitempty"`
	MaxCostUSD          float64             `json:"max_cost_usd"`
	SoftCostWarningUSD  float64             `json:"soft_cost_warning_usd"`
	MaxTokens           int64               `json:"max_tokens"`
	MaxToolCalls        int                 `json:"max_tool_calls"`
	MaxRuntimeSeconds   int                 `json:"max_runtime_seconds"`
	Verification        *VerificationPolicy `json:"verification,omitempty"`
}

type ContractStatusResponse struct {
	Policy   ContractPolicy             `json:"policy"`
	Snapshot *ContractExecutionSnapshot `json:"snapshot,omitempty"`
}

type ContractListResponse struct {
	Items []ContractStatusResponse `json:"items"`
}
