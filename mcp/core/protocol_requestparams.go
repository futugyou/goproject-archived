package core

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
