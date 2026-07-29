package core

type InitializeRequestParams struct {
	RequestParams   `json:",inline"`
	ProtocolVersion string              `json:"protocolVersion"`
	Capabilities    *ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation      `json:"clientInfo"`
}

type InitializeResult struct {
	Meta            map[string]any     `json:"_meta,omitempty"`
	ResultType      string             `json:"resultType"`
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions"`
}

type InitializedNotificationParams struct {
	Meta map[string]any `json:"_meta,omitempty"`
}
