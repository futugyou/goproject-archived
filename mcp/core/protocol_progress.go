package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type ProgressNotificationParams struct {
	Meta          map[string]any            `json:"_meta,omitempty"`
	ProgressToken ProgressToken             `json:"progressToken"`
	Progress      ProgressNotificationValue `json:"progress"`
}

type ProgressNotificationValue struct {
	Progress float32  `json:"progress"`
	Total    *float32 `json:"total,omitempty"`
	Message  *string  `json:"message,omitempty"`
}

type ProgressToken struct {
	Value any // string, int64 or nil
}

func NewStringProgressToken(v string) ProgressToken {
	return ProgressToken{Value: v}
}

func NewIntProgressToken(v int64) ProgressToken {
	return ProgressToken{Value: v}
}

func (p ProgressToken) String() string {
	switch v := p.Value.(type) {
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func (p *ProgressToken) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		p.Value = nil
		return nil
	}

	var intVal int64
	if err := json.Unmarshal(data, &intVal); err == nil {
		p.Value = intVal
		return nil
	}

	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		p.Value = strVal
		return nil
	}

	return fmt.Errorf("progressToken must be a string or an integer, got: %s", string(data))
}

func (p ProgressToken) MarshalJSON() ([]byte, error) {
	switch v := p.Value.(type) {
	case string:
		return json.Marshal(v)
	case int64:
		return json.Marshal(v)
	case nil:
		return json.Marshal("")
	default:
		return nil, fmt.Errorf("invalid ProgressToken type: %T", v)
	}
}

func (p *ProgressNotificationParams) UnmarshalJSON(data []byte) error {
	type Alias ProgressNotificationParams
	var temp struct {
		ProgressToken *ProgressToken `json:"progressToken"`
		Progress      *float32       `json:"progress"`
		Total         *float32       `json:"total"`
		Message       *string        `json:"message"`
		Meta          map[string]any `json:"_meta"`
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&temp); err != nil {
		return err
	}

	if temp.Progress == nil {
		return fmt.Errorf("missing required property 'progress'")
	}

	if temp.ProgressToken == nil {
		return fmt.Errorf("missing required property 'progressToken'")
	}

	p.ProgressToken = *temp.ProgressToken
	p.Meta = temp.Meta
	p.Progress = ProgressNotificationValue{
		Progress: *temp.Progress,
		Total:    temp.Total,
		Message:  temp.Message,
	}

	return nil
}

func (p ProgressNotificationParams) MarshalJSON() ([]byte, error) {
	out := make(map[string]any)

	out["progressToken"] = p.ProgressToken

	out["progress"] = p.Progress.Progress
	if p.Progress.Total != nil {
		out["total"] = *p.Progress.Total
	}
	if p.Progress.Message != nil {
		out["message"] = *p.Progress.Message
	}

	if p.Meta != nil {
		out["_meta"] = p.Meta
	}

	return json.Marshal(out)
}
