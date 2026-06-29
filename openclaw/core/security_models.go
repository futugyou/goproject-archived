package core

type SentinelSubstitutionContext struct {
	ToolName           string `json:"tool_name"`
	ArgumentsJson      string `json:"arguments_json"`
	SessionId          string `json:"session_id"`
	ChannelId          string `json:"channel_id"`
	SenderId           string `json:"sender_id"`
	CorrelationId      string `json:"correlation_id"`
	WorkspaceId        string `json:"workspace_id"`
	PaymentProviderId  string `json:"payment_provider_id"`
	PaymentEnvironment string `json:"payment_environment"`
}

type SentinelSubstitutionResult struct {
	ExecutionArgumentsJson string `json:"execution_arguments_json"`
	PersistedArgumentsJson string `json:"persisted_arguments_json"`
	Substituted            bool   `json:"substituted"`
}
