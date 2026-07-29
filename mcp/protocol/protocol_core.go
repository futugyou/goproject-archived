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

type PaginatedRequestParams struct {
	RequestParams `json:",inline"`
	Cursor        *string `json:"cursor"`
}

type RequestParamsMetadata struct {
	ProgressToken *ProgressToken `json:"progressToken"`
}
