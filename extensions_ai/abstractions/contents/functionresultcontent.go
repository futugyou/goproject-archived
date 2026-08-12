package contents

import "encoding/json"

// FunctionResultContent represents the result of a function call.
type FunctionResultContent struct {
	*AIContent `json:",inline"`
	CallId     string `json:"callId,omitempty"`
	Result     any    `json:"result,omitempty"`
	Error      error  `json:"-"`
}

func NewFunctionResultContent(callId, result string) *FunctionResultContent {
	return &FunctionResultContent{
		CallId: callId,
		Result: result,
	}
}

func (ac FunctionResultContent) MarshalJSON() ([]byte, error) {
	type Alias FunctionResultContent
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "FunctionResultContent",
		Alias: (*Alias)(&ac),
	})
}

func (ac *FunctionResultContent) UnmarshalJSON(data []byte) error {
	type Alias FunctionResultContent
	aux := &struct {
		Type string `json:"type"`
		*Alias
	}{
		Alias: (*Alias)(ac),
	}

	return json.Unmarshal(data, aux)
}
