package core

type ProviderUsageSnapshot struct {
	ProviderID       string `json:"provider_id"`
	ModelID          string `json:"model_id"`
	Requests         int64  `json:"requests"`
	Retries          int64  `json:"retries"`
	Errors           int64  `json:"errors"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
}

type ToolUsageSnapshot struct {
	ToolName        string  `json:"tool_name"`
	Calls           int64   `json:"calls"`
	Failures        int64   `json:"failures"`
	Timeouts        int64   `json:"timeouts"`
	TotalDurationMs float64 `json:"total_duration_ms"`
}
