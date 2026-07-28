package protocol

import (
	"encoding/json"
	"errors"
	"strings"
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

type ElicitResult struct {
	Meta       map[string]any             `json:"_meta,omitempty"`
	ResultType string                     `json:"resultType"`
	Action     string                     `json:"action"`
	Content    map[string]json.RawMessage `json:"content,omitempty"`
}

func (e *ElicitResult) IsAccepted() bool {
	return strings.EqualFold(e.Action, "accept")
}

type ElicitResultTyped[T any] struct {
	Meta       map[string]any `json:"_meta,omitempty"`
	ResultType string         `json:"resultType"`
	Action     string         `json:"action"`
	Content    T              `json:"content,omitempty"`
}

func (e *ElicitResultTyped[T]) IsAccepted() bool {
	return strings.EqualFold(e.Action, "accept")
}

func FromElicitResult(result ElicitResult) (InputResponse, error) {
	if result.Action == "" {
		result.Action = "cancel"
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return InputResponse{}, err
	}
	return InputResponse{RawValue: bytes}, nil
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
