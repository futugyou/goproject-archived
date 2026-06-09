package core

type ToolSandboxMode string

const (
	ToolSandboxMode_None    ToolSandboxMode = "None"
	ToolSandboxMode_Prefer  ToolSandboxMode = "Prefer"
	ToolSandboxMode_Require ToolSandboxMode = "Require"
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
