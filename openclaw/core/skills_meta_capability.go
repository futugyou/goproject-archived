package core

import (
	"context"
	"io"
)

type ResolveCapabilitySelectionPolicy string

const (
	ResolveCapabilitySelectionPolicyFirst     ResolveCapabilitySelectionPolicy = "First"
	ResolveCapabilitySelectionPolicyExactName ResolveCapabilitySelectionPolicy = "ExactName"
)

type ResolveCapabilityRequest struct {
	TaskDescription string                           `json:"task_description"`
	KeyWords        *string                          `json:"key_words,omitempty"`
	SelectionPolicy ResolveCapabilitySelectionPolicy `json:"selection_policy"`
	Provider        string                           `json:"provider"`
	CapabilityType  *string                          `json:"capability_type,omitempty"`
}

type CapabilityCandidate struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rank        int      `json:"rank"`
	Version     *string  `json:"version,omitempty"`
	Score       *float64 `json:"score,omitempty"`
}

type ResolveCapabilityBinding struct {
	Server            string                `json:"server"`
	Tool              string                `json:"tool"`
	Schema            string                `json:"schema"`
	TriedCandidates   []CapabilityCandidate `json:"tried"`
	Provider          string                `json:"provider"`
	SchemaFingerprint string                `json:"schema_fingerprint"`
	Revision          int64                 `json:"revision"`
}

type ResolveCapabilityFailure struct {
	FailureCode     string                `json:"failure_code"`
	TriedCandidates []CapabilityCandidate `json:"tried"`
}

const (
	ResolveCapabilityFailureCodesNoCandidates           = "no_candidates"
	ResolveCapabilityFailureCodesAllAddsFailed          = "all_bindings_failed"
	ResolveCapabilityFailureCodesSelectionPolicyNoMatch = "selection_policy_no_match"
	ResolveCapabilityFailureCodesProviderUnavailable    = "provider_unavailable"
	ResolveCapabilityFailureCodesToolPolicyDenied       = "tool_policy_denied"
)

const (
	CapabilitySlotFailureCodesNotConfigured       = "capability_not_configured"
	CapabilitySlotFailureCodesProviderUnavailable = "capability_provider_unavailable"
	CapabilitySlotFailureCodesBindingFailed       = "capability_binding_failed"
	CapabilitySlotFailureCodesExecutionFailed     = "capability_execution_failed"
	CapabilitySlotFailureCodesResolveFailed       = "capability_resolve_failed"
)

type CapabilityTarget struct {
	Server    string `json:"server"`
	Tool      ITool  `json:"tool"`
	RetrySafe bool   `json:"retry_safe"`
	ToolId    string `json:"tool_id"`
}

func NewCapabilityTarget(server string, tool ITool, retrySafe bool) *CapabilityTarget {
	toolId := ""
	if tool != nil {
		toolId = tool.Name()
	}
	return &CapabilityTarget{
		Server:    server,
		Tool:      tool,
		RetrySafe: retrySafe,
		ToolId:    toolId,
	}
}

type ICapabilityProvider interface {
	Id() string
	Discover(ctx context.Context, request ResolveCapabilityRequest) ([]CapabilityCandidate, error)
	Bind(ctx context.Context, target string, tool *string) (*CapabilityTarget, error)
}

type CapabilityChange struct {
	Provider string `json:"provider"`
	Scope    string `json:"scope"`
	Revision string `json:"revision"`
}

type ICapabilityInvalidationSink interface {
	Invalidate(change CapabilityChange)
}

type ICapabilityChangeSource interface {
	io.Closer
	ProviderId() string
	Status() string
	Start(ctx context.Context) error
}

type CapabilityProviderStatus struct {
	Id     string `json:"id"`
	Events string `json:"events"`
}

type CapabilityRuntimeStatus struct {
	DefaultProvider string                     `json:"default_provider"`
	Providers       []CapabilityProviderStatus `json:"providers"`
	CacheGeneration int64                      `json:"cache_generation"`
	CachedBindings  int                        `json:"cached_bindings"`
}
