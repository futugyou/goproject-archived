package core

import "time"

const (
	BackendLocal       = "local"
	BackendOpenSandbox = "opensandbox"
	BackendDocker      = "docker"
	BackendSsh         = "ssh"
)

type ExecutionConfig struct {
	Enabled        bool                                      `json:"enabled"`
	DefaultBackend string                                    `json:"default_backend"`
	Profiles       map[string]*ExecutionBackendProfileConfig `json:"profiles"`
	Tools          map[string]*ExecutionToolRouteConfig      `json:"tools"`
}

func DefaultExecutionConfig() *ExecutionConfig {
	return &ExecutionConfig{
		Enabled:        true,
		DefaultBackend: BackendLocal,
		Profiles: map[string]*ExecutionBackendProfileConfig{
			BackendLocal: DefaultExecutionBackendProfileConfig(),
		},
		Tools: make(map[string]*ExecutionToolRouteConfig),
	}
}

type ExecutionBackendProfileConfig struct {
	Type             string            `json:"type"`
	Enabled          bool              `json:"enabled"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	Environment      map[string]string `json:"environment"`
	Endpoint         string            `json:"endpoint,omitempty"`
	ApiKey           string            `json:"api_key,omitempty"`
	Image            string            `json:"image,omitempty"`
	Host             string            `json:"host,omitempty"`
	Port             int               `json:"port"`
	Username         string            `json:"username,omitempty"`
	PrivateKeyPath   string            `json:"private_key_path,omitempty"`
	TimeoutSeconds   int               `json:"timeout_seconds"`
	WorkspaceRoot    string            `json:"workspace_root,omitempty"`
}

func DefaultExecutionBackendProfileConfig() *ExecutionBackendProfileConfig {
	return &ExecutionBackendProfileConfig{
		Type:           BackendLocal,
		Enabled:        true,
		Environment:    make(map[string]string),
		Port:           22,
		TimeoutSeconds: 30,
	}
}

type ExecutionToolRouteConfig struct {
	Backend          string `json:"backend"`
	FallbackBackend  string `json:"fallback_backend"`
	RequireWorkspace bool   `json:"require_workspace"`
}

func DefaultExecutionToolRouteConfig() *ExecutionToolRouteConfig {
	return &ExecutionToolRouteConfig{
		Backend:          "",
		RequireWorkspace: true,
	}
}

type ExecutionRequest struct {
	ToolName           string            `json:"tool_name"`
	BackendName        string            `json:"backend_name"`
	Command            string            `json:"command"`
	Arguments          []string          `json:"arguments"`
	LeaseKey           string            `json:"lease_key,omitempty"`
	WorkingDirectory   string            `json:"working_directory,omitempty"`
	Environment        map[string]string `json:"environment"`
	Template           string            `json:"template,omitempty"`
	TimeToLiveSeconds  *int              `json:"time_to_live_seconds,omitempty"`
	RequireWorkspace   bool              `json:"require_workspace"`
	AllowLocalFallback bool              `json:"allow_local_fallback"`
}

func DefaultExecutionRequest() *ExecutionRequest {
	return &ExecutionRequest{
		Arguments:          []string{},
		Environment:        make(map[string]string),
		RequireWorkspace:   true,
		AllowLocalFallback: true,
	}
}

type ExecutionResult struct {
	BackendName  string  `json:"backend_name"`
	ExitCode     int     `json:"exit_code"`
	Stdout       string  `json:"stdout"`
	Stderr       string  `json:"stderr"`
	TimedOut     bool    `json:"timed_out"`
	FallbackUsed bool    `json:"fallback_used"`
	DurationMs   float64 `json:"duration_ms"`
}

type ExecutionBackendCapabilities struct {
	SupportsOneShotCommands  bool `json:"supports_one_shot_commands"`
	SupportsProcesses        bool `json:"supports_processes"`
	SupportsPty              bool `json:"supports_pty"`
	SupportsInteractiveInput bool `json:"supports_interactive_input"`
}

func DefaultExecutionBackendCapabilities() *ExecutionBackendCapabilities {
	return &ExecutionBackendCapabilities{
		SupportsOneShotCommands:  true,
		SupportsProcesses:        false,
		SupportsPty:              false,
		SupportsInteractiveInput: false,
	}
}

const (
	ProcessStateRunning   = "running"
	ProcessStateCompleted = "completed"
	ProcessStateKilled    = "killed"
	ProcessStateFailed    = "failed"
	ProcessStateTimedOut  = "timed_out"
)

type ExecutionProcessStartRequest struct {
	ToolName         string            `json:"tool_name"`
	BackendName      string            `json:"backend_name"`
	OwnerSessionId   string            `json:"owner_session_id"`
	OwnerChannelId   string            `json:"owner_channel_id"`
	OwnerSenderId    string            `json:"owner_sender_id"`
	Command          string            `json:"command"`
	Arguments        []string          `json:"arguments"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	Environment      map[string]string `json:"environment"`
	TimeoutSeconds   *int              `json:"timeout_seconds,omitempty"`
	Pty              bool              `json:"pty"`
	Template         string            `json:"template,omitempty"`
	RequireWorkspace bool              `json:"require_workspace"`
}

func DefaultExecutionProcessStartRequest() *ExecutionProcessStartRequest {
	return &ExecutionProcessStartRequest{
		Arguments:        []string{},
		Environment:      make(map[string]string),
		RequireWorkspace: true,
	}
}

type ExecutionProcessHandle struct {
	ProcessId      string    `json:"process_id"`
	BackendName    string    `json:"backend_name"`
	OwnerSessionId string    `json:"owner_session_id"`
	OwnerChannelId string    `json:"owner_channel_id"`
	OwnerSenderId  string    `json:"owner_sender_id"`
	CommandPreview string    `json:"command_preview"`
	CreatedAtUtc   time.Time `json:"created_at_utc"`
	ExpiresAtUtc   time.Time `json:"expires_at_utc"`
	Pty            bool      `json:"pty"`
}

func DefaultExecutionProcessHandle() *ExecutionProcessHandle {
	now := time.Now().UTC()
	return &ExecutionProcessHandle{
		CommandPreview: "",
		CreatedAtUtc:   now,
		ExpiresAtUtc:   now.Add(1 * time.Hour),
	}
}

type ExecutionProcessStatus struct {
	ProcessId       string     `json:"process_id"`
	BackendName     string     `json:"backend_name"`
	OwnerSessionId  string     `json:"owner_session_id"`
	State           string     `json:"state"`
	ExitCode        *int       `json:"exit_code,omitempty"`
	TimedOut        bool       `json:"timed_out"`
	Pty             bool       `json:"pty"`
	NativeProcessId *int       `json:"native_process_id,omitempty"`
	CreatedAtUtc    time.Time  `json:"created_at_utc"`
	StartedAtUtc    *time.Time `json:"started_at_utc,omitempty"`
	CompletedAtUtc  *time.Time `json:"completed_at_utc,omitempty"`
	DurationMs      float64    `json:"duration_ms"`
	StdoutBytes     int64      `json:"stdout_bytes"`
	StderrBytes     int64      `json:"stderr_bytes"`
	CommandPreview  string     `json:"command_preview"`
}

func DefaultExecutionProcessStatus() *ExecutionProcessStatus {
	return &ExecutionProcessStatus{
		State:          ProcessStateRunning,
		CreatedAtUtc:   time.Now().UTC(),
		CommandPreview: "",
	}
}

type ExecutionProcessLogRequest struct {
	ProcessId      string `json:"process_id"`
	OwnerSessionId string `json:"owner_session_id,omitempty"`
	StdoutOffset   int    `json:"stdout_offset"`
	StderrOffset   int    `json:"stderr_offset"`
	MaxChars       int    `json:"max_chars"`
}

func DefaultExecutionProcessLogRequest() *ExecutionProcessLogRequest {
	return &ExecutionProcessLogRequest{
		MaxChars: 8192,
	}
}

type ExecutionProcessLogResult struct {
	ProcessId        string `json:"process_id"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	NextStdoutOffset int    `json:"next_stdout_offset"`
	NextStderrOffset int    `json:"next_stderr_offset"`
	Completed        bool   `json:"completed"`
}

type ExecutionProcessInputRequest struct {
	ProcessId      string `json:"process_id"`
	OwnerSessionId string `json:"owner_session_id,omitempty"`
	Data           string `json:"data"`
}
