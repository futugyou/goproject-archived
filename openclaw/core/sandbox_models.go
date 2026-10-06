package core

type ToolSandboxMode string

const (
	ToolSandboxModeNone    ToolSandboxMode = "None"
	ToolSandboxModePrefer  ToolSandboxMode = "Prefer"
	ToolSandboxModeRequire ToolSandboxMode = "Require"
)

type SandboxExecutionRequest struct {
	Command           string
	WorkingDirectory  string
	Environment       map[string]string
	Arguments         []string
	LeaseKey          string
	Template          string
	TimeToLiveSeconds int
}

type SandboxResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
