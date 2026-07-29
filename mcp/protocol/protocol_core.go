package protocol

import "time"

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
