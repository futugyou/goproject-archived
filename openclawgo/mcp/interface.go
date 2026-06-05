package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/futugyou/extensions_ai/abstractions"
	"github.com/google/uuid"
)

type IMcpAITool interface {
	GetStorageName() string
	GetServerId() string
	GetServerName() string
}

type McpProcessLaunchPlan struct {
	ServerName       string
	Executable       string
	Arguments        []string
	Environment      map[string]string
	WorkingDirectory string
}

type IMcpProcessIsolationPolicy interface {
	Apply(plan *McpProcessLaunchPlan) error
}

var _ IMcpProcessIsolationPolicy = (*NoIsolationPolicy)(nil)

type NoIsolationPolicy struct{}

// Apply implements [IMcpProcessIsolationPolicy].
func (n *NoIsolationPolicy) Apply(plan *McpProcessLaunchPlan) error {
	return nil
}

var _ IMcpProcessIsolationPolicy = (*WorkingDirIsolationPolicy)(nil)

type WorkingDirIsolationPolicy struct {
}

// Apply implements [IMcpProcessIsolationPolicy].
func (w *WorkingDirIsolationPolicy) Apply(plan *McpProcessLaunchPlan) error {
	if plan == nil {
		return errors.New("McpProcessLaunchPlan can not be nil")
	}
	safeName := w.makeSafeDirName(plan.ServerName)
	dir := filepath.Join(os.TempDir(), "openclawnet-mcp", safeName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	plan.WorkingDirectory = dir

	path, ok := plan.Environment["PATH"]
	plan.Environment = make(map[string]string)
	if ok {
		plan.Environment["PATH"] = path
	}
	return nil
}

func (w *WorkingDirIsolationPolicy) makeSafeDirName(serverName string) string {
	var chars = GetInvalidFileNameChars()
	invalidChars := make(map[rune]struct{})
	for _, c := range chars {
		invalidChars[c] = struct{}{}
	}
	return strings.Map(func(r rune) rune {
		if _, exists := invalidChars[r]; exists {
			return '_'
		}
		return r
	}, serverName)
}

func GetInvalidFileNameChars() []rune {
	if runtime.GOOS == "windows" {
		return []rune{'<', '>', ':', '"', '/', '\\', '|', '?', '*'}
	}
	return []rune{'/', 0}
}

type McpTransport string

const (
	TransportInProcess McpTransport = "InProcess"
	TransportStdio     McpTransport = "Stdio"
	TransportHttp      McpTransport = "Http"
)

type McpServerDefinition struct {
	ID string `json:"id"`

	// Human-readable display name.
	Name string `json:"name"`

	// How OpenClawNet talks to this server.
	Transport McpTransport `json:"transport"`

	// ---- Stdio transport fields ------------------------------------------------

	// Executable to launch (stdio transport only).
	Command *string `json:"command,omitempty"`

	// Command-line arguments (stdio transport only).
	Args []string `json:"args"`

	// JSON-encoded environment variables for the spawned process.
	// Stored as ciphertext via ISecretStore — never read raw from the DB.
	EnvJSON *string `json:"env_json,omitempty"`

	// ---- HTTP transport fields -------------------------------------------------

	// Base URL (HTTP transport only).
	URL *string `json:"url,omitempty"`

	// JSON-encoded HTTP headers (e.g. auth tokens) for HTTP transport.
	// Stored as ciphertext via ISecretStore.
	HeadersJSON *string `json:"headers_json,omitempty"`

	// ---- State -----------------------------------------------------------------

	// Whether the lifecycle service should start this server.
	Enabled bool `json:"enabled"`

	// Built-in servers ship with OpenClawNet. They can be disabled but never deleted —
	// the destructive seed migration in PR-E reasserts them.
	IsBuiltIn bool `json:"is_built_in"`

	// Last error captured while starting or talking to this server.
	LastError *string `json:"last_error,omitempty"`

	// Last time this server reported alive.
	LastSeenUtc *time.Time `json:"last_seen_utc,omitempty"`
}

func DefaultMcpServerDefinition() McpServerDefinition {
	return McpServerDefinition{
		ID:        uuid.NewString(),
		Name:      "",
		Transport: TransportInProcess,
		Args:      []string{},
		Enabled:   true,
		IsBuiltIn: false,
	}
}

type McpToolOverride struct {
	ServerId        string
	ToolName        string
	RequireApproval bool
	Disabled        bool
}

type IMcpServerCatalog interface {
	GetServers(ctx context.Context) ([]McpServerDefinition, error)
	GetOverrides(ctx context.Context) ([]McpToolOverride, error)
}

type IMcpServerHost interface {
	GetTransport() McpTransport
	IsRunning(serverId string) bool
	Start(ctx context.Context, definition *McpServerDefinition) error
	Stop(ctx context.Context, serverId string) error
}

type IMcpToolProvider interface {
	GetAllTools(ctx context.Context) ([]abstractions.AITool, error)
	GetToolsForServer(ctx context.Context, serverId string) ([]abstractions.AITool, error)
	Refresh(ctx context.Context) error
}

type ISecretStore interface {
	Protect(plaintext string) string
	Unprotect(ciphertext string) string
}
