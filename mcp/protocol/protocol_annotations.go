package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// Optional annotations for the client. The client can use annotations to inform
// how objects are used or displayed
type Annotations struct {
	// Describes who the intended customer of this object or data is.
	//
	// It can include multiple entries to indicate content useful for multiple
	// audiences (e.g., `["user", "assistant"]`).
	Audience []Role `json:"audience,omitempty" yaml:"audience,omitempty" mapstructure:"audience,omitempty"`

	// Describes how important this data is for operating the server.
	//
	// A value of 1 means "most important," and indicates that the data is
	// effectively required, while 0 means "least important," and indicates that
	// the data is entirely optional.
	Priority *float64 `json:"priority,omitempty" yaml:"priority,omitempty" mapstructure:"priority,omitempty"`

	// The corresponding JSON should be an ISO 8601 formatted string (for example, \"2025-01-12T15:00:58Z\").
	// Examples of when the resource was last modified include last activity in an open file or when the resource was attached.
	LastModified time.Time `json:"lastModified,omitempty" yaml:"prioritlastModifiedy,omitempty" mapstructure:"lastModified,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *Annotations) UnmarshalJSON(value []byte) error {
	type Plain Annotations
	var plain Plain
	if err := json.Unmarshal(value, &plain); err != nil {
		return err
	}
	if plain.Priority != nil && 1 < *plain.Priority {
		return fmt.Errorf("field %s: must be <= %v", "priority", 1)
	}
	if plain.Priority != nil && 0 > *plain.Priority {
		return fmt.Errorf("field %s: must be >= %v", "priority", 0)
	}
	*j = Annotations(plain)
	return nil
}
