package core

import "time"

type ChannelAllowlistFile struct {
	AllowedFrom  []string  `json:"allowed_from"`
	AllowedTo    []string  `json:"allowed_to"`
	UpdatedAtUtc time.Time `json:"updated_at_utc"`
}
