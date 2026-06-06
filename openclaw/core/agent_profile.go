package core

type AgentProfile struct {
	Name            string   `json:"name"`
	SystemPrompt    string   `json:"system_prompt"`
	AllowedTools    []string `json:"allowed_tools"`
	MaxIterations   int      `json:"max_iterations"`
	MaxHistoryTurns int      `json:"max_history_turns"`
}

func DefaultAgentProfile() *AgentProfile {
	return &AgentProfile{
		AllowedTools:    []string{},
		MaxIterations:   5,
		MaxHistoryTurns: 20,
	}
}

type DelegationConfig struct {
	Enabled  bool                    `json:"enabled"`
	MaxDepth int                     `json:"max_depth"`
	Profiles map[string]AgentProfile `json:"profiles"`
}

func DefaultDelegationConfig() *DelegationConfig {
	return &DelegationConfig{
		MaxDepth: 3,
		Profiles: map[string]AgentProfile{},
	}
}
