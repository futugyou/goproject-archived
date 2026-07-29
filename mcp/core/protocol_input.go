package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

type InputRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

func (r *InputRequest) ElicitationParams() (*ElicitRequestParams, error) {
	if r.Method != RequestMethods_ElicitationCreate || len(r.Params) == 0 {
		return nil, nil
	}

	var params ElicitRequestParams
	if err := json.Unmarshal(r.Params, &params); err != nil {
		return nil, fmt.Errorf("failed to deserialize elicit params: %w", err)
	}

	return &params, nil
}

func ForElicitation(requestParams *ElicitRequestParams) (InputRequest, error) {
	if requestParams == nil {
		return InputRequest{}, errors.New("requestParams cannot be nil")
	}

	rawParams, err := json.Marshal(requestParams)
	if err != nil {
		return InputRequest{}, fmt.Errorf("failed to serialize requestParams: %w", err)
	}

	return InputRequest{
		Method: RequestMethods_ElicitationCreate,
		Params: rawParams,
	}, nil
}

func (r *InputRequest) UnmarshalJSON(data []byte) error {
	type Alias InputRequest
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if r.Method == "" {
		return errors.New("InputRequest must have a 'method' property")
	}

	return nil
}

type InputRequiredResult struct {
	Meta          map[string]any          `json:"_meta,omitempty"`
	ResultType    string                  `json:"resultType"`
	InputRequests map[string]InputRequest `json:"inputRequests,omitempty"`
	RequestState  string                  `json:"requestState"`
}

func NewInputRequiredResult() *InputRequiredResult {
	return &InputRequiredResult{ResultType: "input_required"}
}

type InputRequiredException struct {
	Result InputRequiredResult
}

func ExceptionFromInputRequiredResult(result InputRequiredResult) *InputRequiredException {
	return &InputRequiredException{Result: result}
}

func ExceptionFromInputRequiredResultParameter(inputRequests map[string]InputRequest, requestState string) *InputRequiredException {
	input := NewInputRequiredResult()
	input.InputRequests = inputRequests
	input.RequestState = requestState
	return &InputRequiredException{Result: *input}
}

func (i *InputRequiredException) Error() string {
	return "the server returned an input-required result requiring additional client input"
}
