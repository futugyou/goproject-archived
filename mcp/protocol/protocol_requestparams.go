package protocol

import (
	"encoding/json"
	"errors"
)

type InputResponse struct {
	RawValue json.RawMessage `json:"-"`
}

func (r *InputResponse) UnmarshalJSON(data []byte) error {
	r.RawValue = append(r.RawValue[:0], data...)
	return nil
}

func (r InputResponse) MarshalJSON() ([]byte, error) {
	if len(r.RawValue) == 0 {
		return []byte("null"), nil
	}
	return r.RawValue, nil
}

func InputResponseDeserialize[T any](resp InputResponse) (T, error) {
	var target T
	if len(resp.RawValue) == 0 {
		return target, errors.New("rawValue is empty")
	}
	err := json.Unmarshal(resp.RawValue, &target)
	return target, err
}

type RequestParams struct {
	Meta           map[string]any           `json:"_meta,omitempty"`
	InputResponses map[string]InputResponse `json:"inputResponses,omitempty"`
	RequestState   string                   `json:"requestState,omitempty"`
}

func (r *RequestParams) ProgressToken() *ProgressToken {
	if r.Meta == nil {
		return nil
	}

	var token ProgressToken

	switch v := r.Meta["progressToken"].(type) {
	case int64:
		token = NewProgressTokenFromInt(v)
	case float64:
		token = NewProgressTokenFromInt(int64(v))
	case int:
		token = NewProgressTokenFromInt(int64(v))
	case string:
		token = NewProgressTokenFromString(v)
	default:
		return nil
	}

	return &token
}

type RequestParamsMetadata struct {
	ProgressToken *ProgressToken `json:"progressToken"`
}
