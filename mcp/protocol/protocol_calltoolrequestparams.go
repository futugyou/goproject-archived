package protocol

type CallToolRequestParams struct {
	RequestParams `json:",inline"`
	Name          string         `json:"name"`
	Arguments     map[string]any `json:"arguments"`
}
