package protocol

import "time"

type GetPromptRequestParams struct {
	RequestParams `json:",inline"`
	Name          string         `json:"name"`
	Arguments     map[string]any `json:"arguments"`
}

type GetPromptResult struct {
	Meta        map[string]any  `json:"_meta,omitempty"`
	ResultType  string          `json:"resultType"`
	Description *string         `json:"description"`
	Messages    []PromptMessage `json:"messages"`
}

type PromptMessage struct {
	Content ContentBlock `json:"content"`
	Role    Role         `json:"role"`
}

type Prompt struct {
	Name        string           `json:"name"`
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	Arguments   []PromptArgument `json:"arguments"`
	Icons       []Icon           `json:"icons"`
	Meta        map[string]any   `json:"_meta,omitempty"`
}

type PromptArgument struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Required    *bool   `json:"required"`
}

type PromptListChangedNotificationParams struct {
	NotificationParams
}

type ListPromptsRequestParams struct {
	PaginatedRequestParams `json:",inline"`
}

type ListPromptsResult struct {
	PaginatedResult `json:",inline"`
	Prompts         []Prompt    `json:"prompts"`
	TimeToLive      time.Time   `json:"ttlMs"`
	CacheScope      *CacheScope `json:"cacheScope"`
}
