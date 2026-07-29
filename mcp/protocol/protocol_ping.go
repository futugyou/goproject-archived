package protocol

type PingResult struct {
	Meta       map[string]any `json:"_meta,omitempty"`
	ResultType string         `json:"resultType"`
}

type PingRequestParams struct {
	RequestParams
}
