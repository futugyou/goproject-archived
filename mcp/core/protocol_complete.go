package core

type CompleteResult struct {
	Meta       map[string]any `json:"_meta,omitempty"`
	ResultType string         `json:"resultType"`

	Completion Completion `json:"completion"`
}

type Completion struct {
	Values  []string `json:"values,omitempty"`
	Total   *int     `json:"total,omitempty"`
	HasMore *bool    `json:"hasMore,omitempty"`
}

type CompleteContext struct {
	Argument map[string]string `json:"argument"`
}

type CompleteRequestParams struct {
	RequestParams
	Ref      Reference       `json:"ref"`
	Argument Argument        `json:"argument"`
	Context  CompleteContext `json:"context"`
}

type CompletionsCapability struct {
}
