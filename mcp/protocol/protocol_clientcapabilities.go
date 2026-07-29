package protocol

// ClientCapabilities represents the capabilities supported by the client.
type ClientCapabilities struct {
	Experimental map[string]any         `json:"experimental,omitempty"`
	Elicitation  *ElicitationCapability `json:"elicitation,omitempty"`
	Extensions   map[string]any         `json:"extensions,omitempty"`
}

type MissingRequiredClientCapabilityErrorData struct {
	RequiredCapabilities ClientCapabilities `json:"requiredCapabilities"`
}

type UnsupportedProtocolVersionErrorData struct {
	Supported []string `json:"supported"`
	Requested string   `json:"requested"`
}

type UrlElicitationRequiredErrorData struct {
	Elicitations []ElicitRequestParams `json:"elicitations"`
}
