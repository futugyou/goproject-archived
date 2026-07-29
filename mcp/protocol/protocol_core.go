package protocol

import (
	"encoding/json"
	"errors"
	"time"
)

type NotificationParams struct {
	Meta map[string]any `json:"_meta,omitempty"`
}

type BaseMetadata struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type CacheableResult struct {
	TimeToLive time.Time
	CacheScope *CacheScope
}

type Result struct {
	Meta       map[string]any `json:"_meta,omitempty"`
	ResultType string         `json:"resultType"`
}

func (r *Result) GetResultType() string {
	if r == nil || r.ResultType == "" {
		return "complete"
	}
	return r.ResultType
}

type ResultOrAlternate[T any] struct {
	result    *T
	alternate *Result
}

func NewResult[T any](res T) (ResultOrAlternate[T], error) {
	if any(res) == nil {
		return ResultOrAlternate[T]{}, errors.New("result cannot be nil")
	}

	return ResultOrAlternate[T]{
		result: &res,
	}, nil
}

func FromAlternate[T any](alternate *Result) (ResultOrAlternate[T], error) {
	if alternate == nil {
		return ResultOrAlternate[T]{}, errors.New("alternate cannot be nil")
	}

	return ResultOrAlternate[T]{
		alternate: alternate,
	}, nil
}

func (r ResultOrAlternate[T]) IsAlternate() bool {
	return r.alternate != nil
}

func (r ResultOrAlternate[T]) Result() *T {
	return r.result
}

func (r ResultOrAlternate[T]) Alternate() *Result {
	return r.alternate
}

func (r ResultOrAlternate[T]) MarshalJSON() ([]byte, error) {
	if r.IsAlternate() {
		return json.Marshal(r.alternate)
	}
	if r.result != nil {
		return json.Marshal(r.result)
	}
	return json.Marshal(nil)
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
		token = NewIntProgressToken(v)
	case float64:
		token = NewIntProgressToken(int64(v))
	case int:
		token = NewIntProgressToken(int64(v))
	case string:
		token = NewStringProgressToken(v)
	default:
		return nil
	}

	return &token
}

type PaginatedRequestParams struct {
	RequestParams `json:",inline"`
	Cursor        *string `json:"cursor"`
}

type RequestParamsMetadata struct {
	ProgressToken *ProgressToken `json:"progressToken"`
}

type PaginatedResult struct {
	Meta       map[string]any `json:"_meta,omitempty"`
	ResultType string         `json:"resultType"`
	NextCursor *string
}

type Implementation struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Icons       []Icon `json:"icons"`
	WebsiteUrl  string `json:"websiteUrl"`
}
