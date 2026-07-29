package core

type CancelledNotificationParams struct {
	Meta map[string]any `json:"_meta,omitempty"`

	RequestId RequestId `json:"requestId"`
	Reason    *string   `json:"reason"`
}
