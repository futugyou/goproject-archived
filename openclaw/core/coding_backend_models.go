package core

import (
	"encoding/json"
	"fmt"
	"time"
)

type IBackendEvent interface {
	GetBase() *BackendEventBase
}

type BackendEventBase struct {
	EventType    string    `json:"event_type"`
	SessionID    string    `json:"session_id"`
	Sequence     int64     `json:"sequence"`
	TimestampUtc time.Time `json:"timestamp_utc"`
	RawLine      *string   `json:"raw_line,omitempty"`
}

func (b *BackendEventBase) GetBase() *BackendEventBase {
	return b
}

func NewBackendEventBase() BackendEventBase {
	return BackendEventBase{
		TimestampUtc: time.Now().UTC(),
	}
}

type BackendAssistantMessageEvent struct {
	BackendEventBase
	Text string `json:"text"`
}

type BackendStdoutOutputEvent struct {
	BackendEventBase
	Text string `json:"text"`
}

type BackendStderrOutputEvent struct {
	BackendEventBase
	Text string `json:"text"`
}

type BackendToolCallRequestedEvent struct {
	BackendEventBase
	ToolName      string  `json:"tool_name"`
	ArgumentsJson *string `json:"arguments_json,omitempty"`
}

type BackendShellCommandProposedEvent struct {
	BackendEventBase
	Command string `json:"command"`
}

type BackendShellCommandExecutedEvent struct {
	BackendEventBase
	Command  string  `json:"command"`
	ExitCode *int    `json:"exit_code,omitempty"`
	Stdout   *string `json:"stdout,omitempty"`
	Stderr   *string `json:"stderr,omitempty"`
}

type BackendPatchProposedEvent struct {
	BackendEventBase
	Path  *string `json:"path,omitempty"`
	Patch string  `json:"patch"`
}

type BackendPatchAppliedEvent struct {
	BackendEventBase
	Path    *string `json:"path,omitempty"`
	Summary *string `json:"summary,omitempty"`
}

type BackendFileReadEvent struct {
	BackendEventBase
	Path string `json:"path"`
}

type BackendFileWriteEvent struct {
	BackendEventBase
	Path string `json:"path"`
}

type BackendErrorEvent struct {
	BackendEventBase
	Message string `json:"message"`
}

type BackendSessionCompletedEvent struct {
	BackendEventBase
	ExitCode *int    `json:"exit_code,omitempty"`
	Reason   *string `json:"reason,omitempty"`
}

func UnmarshalBackendEvent(data []byte) (IBackendEvent, error) {
	var discriminator struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return nil, err
	}

	var event IBackendEvent

	switch discriminator.EventType {
	case "assistant_message":
		event = &BackendAssistantMessageEvent{BackendEventBase: NewBackendEventBase()}
	case "stdout_output":
		event = &BackendStdoutOutputEvent{BackendEventBase: NewBackendEventBase()}
	case "stderr_output":
		event = &BackendStderrOutputEvent{BackendEventBase: NewBackendEventBase()}
	case "tool_call_requested":
		event = &BackendToolCallRequestedEvent{BackendEventBase: NewBackendEventBase()}
	case "shell_command_proposed":
		event = &BackendShellCommandProposedEvent{BackendEventBase: NewBackendEventBase()}
	case "shell_command_executed":
		event = &BackendShellCommandExecutedEvent{BackendEventBase: NewBackendEventBase()}
	case "patch_proposed":
		event = &BackendPatchProposedEvent{BackendEventBase: NewBackendEventBase()}
	case "patch_applied":
		event = &BackendPatchAppliedEvent{BackendEventBase: NewBackendEventBase()}
	case "file_read":
		event = &BackendFileReadEvent{BackendEventBase: NewBackendEventBase()}
	case "file_write":
		event = &BackendFileWriteEvent{BackendEventBase: NewBackendEventBase()}
	case "error":
		event = &BackendErrorEvent{BackendEventBase: NewBackendEventBase()}
	case "session_completed":
		event = &BackendSessionCompletedEvent{BackendEventBase: NewBackendEventBase()}
	default:
		return nil, fmt.Errorf("unknown event type: %s", discriminator.EventType)
	}

	if err := json.Unmarshal(data, event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *BackendAssistantMessageEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "assistant_message"
	type Alias BackendAssistantMessageEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendStdoutOutputEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "stdout_output"
	type Alias BackendStdoutOutputEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendStderrOutputEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "stderr_output"
	type Alias BackendStderrOutputEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendToolCallRequestedEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "tool_call_requested"
	type Alias BackendToolCallRequestedEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendShellCommandProposedEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "shell_command_proposed"
	type Alias BackendShellCommandProposedEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendShellCommandExecutedEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "shell_command_executed"
	type Alias BackendShellCommandExecutedEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendPatchProposedEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "patch_proposed"
	type Alias BackendPatchProposedEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendPatchAppliedEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "patch_applied"
	type Alias BackendPatchAppliedEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendFileReadEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "file_read"
	type Alias BackendFileReadEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendFileWriteEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "file_write"
	type Alias BackendFileWriteEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendErrorEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "error"
	type Alias BackendErrorEvent
	return json.Marshal((*Alias)(e))
}

func (e *BackendSessionCompletedEvent) MarshalJSON() ([]byte, error) {
	e.EventType = "session_completed"
	type Alias BackendSessionCompletedEvent
	return json.Marshal((*Alias)(e))
}

type CodingCliBackendConfig struct {
	Enabled                bool                          `json:"enabled"`
	BackendId              string                        `json:"backend_id"`
	Provider               string                        `json:"provider"`
	DisplayName            *string                       `json:"display_name,omitempty"`
	ExecutablePath         *string                       `json:"executable_path,omitempty"`
	Args                   []string                      `json:"args"`
	ProbeArgs              []string                      `json:"probe_args"`
	TimeoutSeconds         int                           `json:"timeout_seconds"`
	Environment            map[string]string             `json:"environment"`
	DefaultModel           *string                       `json:"default_model,omitempty"`
	RequireWorkspace       bool                          `json:"require_workspace"`
	DefaultWorkspacePath   *string                       `json:"default_workspace_path,omitempty"`
	ReadOnlyByDefault      bool                          `json:"read_only_by_default"`
	WriteEnabled           bool                          `json:"write_enabled"`
	PreferStructuredOutput bool                          `json:"prefer_structured_output"`
	Credentials            BackendCredentialSourceConfig `json:"credentials"`
}

func DefaultCodingCliBackendConfig() CodingCliBackendConfig {
	return CodingCliBackendConfig{
		Enabled:                false,
		BackendId:              "",
		Provider:               "",
		Args:                   []string{},
		ProbeArgs:              []string{"--help"},
		TimeoutSeconds:         600,
		Environment:            make(map[string]string),
		RequireWorkspace:       true,
		ReadOnlyByDefault:      false,
		WriteEnabled:           true,
		PreferStructuredOutput: true,
		Credentials:            DefaultBackendCredentialSourceConfig(),
	}
}

type BackendCredentialSourceConfig struct {
	SecretRef          *string `json:"secret_ref,omitempty"`
	TokenFilePath      *string `json:"token_file_path,omitempty"`
	ConnectedAccountId *string `json:"connected_account_id,omitempty"`
}

func DefaultBackendCredentialSourceConfig() BackendCredentialSourceConfig {
	return BackendCredentialSourceConfig{}
}

type ConnectedAccount struct {
	Id                  string            `json:"id"`
	Provider            string            `json:"provider"`
	DisplayName         *string           `json:"display_name,omitempty"`
	SecretKind          string            `json:"secret_kind"`
	SecretRef           *string           `json:"secret_ref,omitempty"`
	EncryptedSecretJson *string           `json:"encrypted_secret_json,omitempty"`
	TokenFilePath       *string           `json:"token_file_path,omitempty"`
	Scopes              []string          `json:"scopes"`
	ExpiresAt           *time.Time        `json:"expires_at,omitempty"`
	IsActive            bool              `json:"is_active"`
	Metadata            map[string]string `json:"metadata"`
	CreatedAtUtc        time.Time         `json:"created_at_utc"`
	UpdatedAtUtc        time.Time         `json:"updated_at_utc"`
}

func DefaultConnectedAccount() ConnectedAccount {
	now := time.Now().UTC()
	return ConnectedAccount{
		SecretKind:   "protected_blob",
		Scopes:       []string{},
		IsActive:     true,
		Metadata:     make(map[string]string),
		CreatedAtUtc: now,
		UpdatedAtUtc: now,
	}
}

type ConnectedAccountSecretRef struct {
	SecretRef          *string `json:"secret_ref,omitempty"`
	TokenFilePath      *string `json:"token_file_path,omitempty"`
	ConnectedAccountId *string `json:"connected_account_id,omitempty"`
}

type ConnectedAccountSecretPayload struct {
	Secret string `json:"secret"`
}

type ResolvedBackendCredential struct {
	Provider      string            `json:"provider"`
	SourceKind    string            `json:"source_kind"`
	AccountId     *string           `json:"account_id,omitempty"`
	DisplayName   *string           `json:"display_name,omitempty"`
	Secret        *string           `json:"secret,omitempty"`
	TokenFilePath *string           `json:"token_file_path,omitempty"`
	Scopes        []string          `json:"scopes"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	Metadata      map[string]string `json:"metadata"`
}

func DefaultResolvedBackendCredential() ResolvedBackendCredential {
	return ResolvedBackendCredential{
		Scopes:   []string{},
		Metadata: make(map[string]string),
	}
}

type BackendDefinition struct {
	BackendId      string              `json:"backend_id"`
	Provider       string              `json:"provider"`
	DisplayName    string              `json:"display_name"`
	Enabled        bool                `json:"enabled"`
	ExecutablePath *string             `json:"executable_path,omitempty"`
	DefaultModel   *string             `json:"default_model,omitempty"`
	Capabilities   BackendCapabilities `json:"capabilities"`
	AccessPolicy   BackendAccessPolicy `json:"access_policy"`
}

func DefaultBackendDefinition() BackendDefinition {
	return BackendDefinition{
		Capabilities: DefaultBackendCapabilities(),
		AccessPolicy: DefaultBackendAccessPolicy(),
	}
}

type BackendCapabilities struct {
	SupportsSessions            bool `json:"supports_sessions"`
	SupportsInteractiveInput    bool `json:"supports_interactive_input"`
	SupportsJsonEvents          bool `json:"supports_json_events"`
	SupportsStructuredStreaming bool `json:"supports_structured_streaming"`
	SupportsWorkspace           bool `json:"supports_workspace"`
	SupportsReadOnlyMode        bool `json:"supports_read_only_mode"`
	SupportsWriteMode           bool `json:"supports_write_mode"`
	SupportsModelOverride       bool `json:"supports_model_override"`
}

func DefaultBackendCapabilities() BackendCapabilities {
	return BackendCapabilities{
		SupportsSessions:         true,
		SupportsInteractiveInput: true,
		SupportsWorkspace:        true,
		SupportsReadOnlyMode:     true,
		SupportsWriteMode:        true,
		SupportsModelOverride:    true,
	}
}

type BackendAccessPolicy struct {
	ReadOnlyByDefault bool `json:"read_only_by_default"`
	WriteEnabled      bool `json:"write_enabled"`
	RequireWorkspace  bool `json:"require_workspace"`
}

func DefaultBackendAccessPolicy() BackendAccessPolicy {
	return BackendAccessPolicy{
		WriteEnabled:     true,
		RequireWorkspace: true,
	}
}

type BackendSessionHandle struct {
	BackendId    string    `json:"backend_id"`
	SessionId    string    `json:"session_id"`
	CreatedAtUtc time.Time `json:"created_at_utc"`
}

func DefaultBackendSessionHandle() BackendSessionHandle {
	return BackendSessionHandle{
		CreatedAtUtc: time.Now().UTC(),
	}
}

type BackendSessionRecord struct {
	SessionId               string     `json:"session_id"`
	BackendId               string     `json:"backend_id"`
	Provider                string     `json:"provider"`
	State                   string     `json:"state"`
	OwnerSessionId          *string    `json:"owner_session_id,omitempty"`
	WorkspacePath           *string    `json:"workspace_path,omitempty"`
	Model                   *string    `json:"model,omitempty"`
	ReadOnly                bool       `json:"read_only"`
	StructuredOutputEnabled bool       `json:"structured_output_enabled"`
	DisplayName             *string    `json:"display_name,omitempty"`
	CreatedAtUtc            time.Time  `json:"created_at_utc"`
	StartedAtUtc            *time.Time `json:"started_at_utc,omitempty"`
	CompletedAtUtc          *time.Time `json:"completed_at_utc,omitempty"`
	LastEventSequence       int64      `json:"last_event_sequence"`
	ExitCode                *int       `json:"exit_code,omitempty"`
	LastError               *string    `json:"last_error,omitempty"`
}

func DefaultBackendSessionRecord() BackendSessionRecord {
	return BackendSessionRecord{
		State:        "pending",
		CreatedAtUtc: time.Now().UTC(),
	}
}

type StartBackendSessionRequest struct {
	BackendId        string                     `json:"backend_id"`
	OwnerSessionId   *string                    `json:"owner_session_id,omitempty"`
	WorkspacePath    *string                    `json:"workspace_path,omitempty"`
	Prompt           *string                    `json:"prompt,omitempty"`
	Model            *string                    `json:"model,omitempty"`
	ReadOnly         *bool                      `json:"read_only,omitempty"`
	Environment      map[string]string          `json:"environment"`
	CredentialSource *ConnectedAccountSecretRef `json:"credential_source,omitempty"`
}

func DefaultStartBackendSessionRequest() StartBackendSessionRequest {
	return StartBackendSessionRequest{
		Environment: make(map[string]string),
	}
}

type BackendInput struct {
	Text          *string `json:"text,omitempty"`
	AppendNewline bool    `json:"append_newline"`
	CloseInput    bool    `json:"close_input"`
}

func DefaultBackendInput() BackendInput {
	return BackendInput{
		AppendNewline: true,
	}
}

type BackendProbeRequest struct {
	WorkspacePath    *string                    `json:"workspace_path,omitempty"`
	Model            *string                    `json:"model,omitempty"`
	Environment      map[string]string          `json:"environment"`
	CredentialSource *ConnectedAccountSecretRef `json:"credential_source,omitempty"`
}

func DefaultBackendProbeRequest() BackendProbeRequest {
	return BackendProbeRequest{
		Environment: make(map[string]string),
	}
}

type BackendProbeResult struct {
	BackendId                 string  `json:"backend_id"`
	Success                   bool    `json:"success"`
	Message                   *string `json:"message,omitempty"`
	ExecutablePath            *string `json:"executable_path,omitempty"`
	ExitCode                  *int    `json:"exit_code,omitempty"`
	Stdout                    *string `json:"stdout,omitempty"`
	Stderr                    *string `json:"stderr,omitempty"`
	DurationMs                float64 `json:"duration_ms"`
	StructuredOutputSupported bool    `json:"structured_output_supported"`
}

type ConnectedAccountCreateRequest struct {
	Provider      string            `json:"provider"`
	DisplayName   *string           `json:"display_name,omitempty"`
	SecretRef     *string           `json:"secret_ref,omitempty"`
	Secret        *string           `json:"secret,omitempty"`
	TokenFilePath *string           `json:"token_file_path,omitempty"`
	Scopes        []string          `json:"scopes"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	IsActive      *bool             `json:"is_active,omitempty"`
	Metadata      map[string]string `json:"metadata"`
}

func DefaultConnectedAccountCreateRequest() ConnectedAccountCreateRequest {
	return ConnectedAccountCreateRequest{
		Scopes:   []string{},
		Metadata: make(map[string]string),
	}
}

type BackendCredentialResolutionRequest struct {
	Provider         *string                    `json:"provider,omitempty"`
	BackendId        *string                    `json:"backend_id,omitempty"`
	CredentialSource *ConnectedAccountSecretRef `json:"credential_source,omitempty"`
}

type BackendCredentialResolutionResponse struct {
	Success    bool                       `json:"success"`
	Error      *string                    `json:"error,omitempty"`
	HasSecret  bool                       `json:"has_secret"`
	Credential *ResolvedBackendCredential `json:"credential,omitempty"`
}

type IntegrationAccountsResponse struct {
	Items []ConnectedAccount `json:"items"`
}

type IntegrationConnectedAccountResponse struct {
	Account *ConnectedAccount `json:"account,omitempty"`
}

type IntegrationBackendsResponse struct {
	Items []BackendDefinition `json:"items"`
}

type IntegrationBackendResponse struct {
	Backend *BackendDefinition `json:"backend,omitempty"`
}

type IntegrationBackendSessionResponse struct {
	Session *BackendSessionRecord `json:"session,omitempty"`
}

type BackendEvent struct {
	Sequence int64  `json:"sequence"`
	Type     string `json:"type"`
	Payload  string `json:"payload"`
}

type IntegrationBackendEventsResponse struct {
	SessionId    string         `json:"session_id"`
	NextSequence int64          `json:"next_sequence"`
	Items        []BackendEvent `json:"items"`
}

type CodingBackendsConfig struct {
	Enabled                     bool
	Codex                       CodingCliBackendConfig
	GeminiCli                   CodingCliBackendConfig
	GitHubCopilotCli            CodingCliBackendConfig
	EnumerateConfiguredBackends []CodingCliBackendConfig
}

func DefaultCodingBackendsConfig() *CodingBackendsConfig {
	code := CodingCliBackendConfig{
		BackendId: "codex-cli",
		Provider:  "codex",
	}
	geminiCli := CodingCliBackendConfig{
		BackendId: "gemini-cli",
		Provider:  "gemini-cli",
	}
	gitHubCopilotCli := CodingCliBackendConfig{
		BackendId: "github-copilot-cli",
		Provider:  "github-copilot-cli",
	}
	return &CodingBackendsConfig{
		Enabled:          true,
		Codex:            code,
		GeminiCli:        geminiCli,
		GitHubCopilotCli: gitHubCopilotCli,
		EnumerateConfiguredBackends: []CodingCliBackendConfig{
			code, geminiCli, gitHubCopilotCli,
		},
	}
}
