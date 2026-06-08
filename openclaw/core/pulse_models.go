package core

import "time"

const (
	PulseDefaultsAckToken = "HEARTBEAT_OK"
	PulseDefaultsPrompt   = "Read HEARTBEAT.md if it exists in the workspace context. Follow it strictly. Do not infer or repeat old tasks from prior chats. If nothing needs attention, reply HEARTBEAT_OK."
)

const (
	PulseEventActionsRunStarted      = "pulse_run_started"
	PulseEventActionsRunCompleted    = "pulse_run_completed"
	PulseEventActionsSkipped         = "pulse_skipped"
	PulseEventActionsAlert           = "pulse_alert"
	PulseEventActionsOkSuppressed    = "pulse_ok_suppressed"
	PulseEventActionsDeliverySkipped = "pulse_delivery_skipped"
	PulseEventActionsError           = "pulse_error"
	PulseEventActionsManualWake      = "pulse_manual_wake"
)

const (
	PulseSkipReasonsDisabled           = "disabled"
	PulseSkipReasonsOutsideActiveHours = "outside-active-hours"
	PulseSkipReasonsBusy               = "busy"
	PulseSkipReasonsEmptyHeartbeatFile = "empty-heartbeat-file"
	PulseSkipReasonsVisibilityDisabled = "visibility-disabled"
	PulseSkipReasonsNoSession          = "no-session"
	PulseSkipReasonsEmptyManualWake    = "empty-manual-wake"
	PulseSkipReasonsDeliveryBlocked    = "delivery-blocked"
	PulseSkipReasonsDmBlocked          = "dm-blocked"
	PulseSkipReasonsModelUnavailable   = "model-unavailable"
	PulseSkipReasonsError              = "error"
)

type PulseConfig struct {
	Enabled                   bool                    `json:"enabled"`
	Every                     string                  `json:"every"`
	Model                     *string                 `json:"model"`
	Prompt                    string                  `json:"prompt"`
	AckToken                  string                  `json:"ack_token"`
	AckMaxChars               int                     `json:"ack_max_chars"`
	Target                    string                  `json:"target"`
	To                        *string                 `json:"to"`
	AccountId                 *string                 `json:"account_id"`
	DirectPolicy              string                  `json:"direct_policy"`
	IncludeReasoning          bool                    `json:"include_reasoning"`
	LightContext              bool                    `json:"light_context"`
	IsolatedSession           bool                    `json:"isolated_session"`
	SkipWhenBusy              bool                    `json:"skip_when_busy"`
	SuppressToolErrorWarnings bool                    `json:"suppress_tool_error_warnings"`
	Session                   string                  `json:"session"`
	ActiveHours               *PulseActiveHoursConfig `json:"active_hours"`
	Visibility                PulseVisibilityConfig   `json:"visibility"`
}

func NewDefaultPulseConfig() *PulseConfig {
	return &PulseConfig{
		Enabled:                   true,
		Every:                     "30m",
		Prompt:                    PulseDefaultsPrompt,
		AckToken:                  PulseDefaultsAckToken,
		AckMaxChars:               300,
		Target:                    "none",
		DirectPolicy:              "allow",
		SkipWhenBusy:              true,
		SuppressToolErrorWarnings: true,
		Session:                   "main",
		Visibility:                *NewDefaultPulseVisibilityConfig(),
	}
}

type PulseActiveHoursConfig struct {
	Start    string  `json:"start"`
	End      string  `json:"end"`
	Timezone *string `json:"timezone"`
}

func NewDefaultPulseActiveHoursConfig() *PulseActiveHoursConfig {
	return &PulseActiveHoursConfig{
		Start: "09:00",
		End:   "17:00",
	}
}

type PulseVisibilityConfig struct {
	ShowOk       bool `json:"show_ok"`
	ShowAlerts   bool `json:"show_alerts"`
	UseIndicator bool `json:"use_indicator"`
}

func NewDefaultPulseVisibilityConfig() *PulseVisibilityConfig {
	return &PulseVisibilityConfig{
		ShowOk:       false,
		ShowAlerts:   true,
		UseIndicator: true,
	}
}

type PulseRunRequest struct {
	Text *string `json:"text"`
	Mode string  `json:"mode"`
}

func NewDefaultPulseRunRequest() *PulseRunRequest {
	return &PulseRunRequest{
		Mode: "now",
	}
}

type PulseRunResponse struct {
	Success        bool    `json:"success"`
	Outcome        string  `json:"outcome"`
	SkipReason     *string `json:"skip_reason"`
	SessionId      *string `json:"session_id"`
	MessagePreview *string `json:"message_preview"`
}

func NewDefaultPulseRunResponse() *PulseRunResponse {
	return &PulseRunResponse{
		Outcome: "unknown",
	}
}

type PulseStatusResponse struct {
	Config             PulseConfig     `json:"config"`
	HeartbeatPath      string          `json:"heartbeat_path"`
	HeartbeatExists    bool            `json:"heartbeat_exists"`
	HeartbeatEmpty     bool            `json:"heartbeat_empty"`
	Enabled            bool            `json:"enabled"`
	Interval           string          `json:"interval"`
	LastRunAtUtc       *time.Time      `json:"last_run_at_utc"`
	LastCompletedAtUtc *time.Time      `json:"last_completed_at_utc"`
	NextRunAtUtc       *time.Time      `json:"next_run_at_utc"`
	LastResult         string          `json:"last_result"`
	LastSkipReason     *string         `json:"last_skip_reason"`
	RecentAlertCount   int             `json:"recent_alert_count"`
	RecentOkCount      int             `json:"recent_ok_count"`
	RecentAlerts       []PulseAlertDto `json:"recent_alerts"`
	PendingManualText  *string         `json:"pending_manual_text"`
}

func NewDefaultPulseStatusResponse() *PulseStatusResponse {
	return &PulseStatusResponse{
		Interval:     "30m",
		LastResult:   "never",
		RecentAlerts: []PulseAlertDto{},
	}
}

type PulseAlertDto struct {
	TimestampUtc time.Time `json:"timestamp_utc"`
	Text         string    `json:"text"`
	Severity     string    `json:"severity"`
}

func NewDefaultPulseAlertDto() *PulseAlertDto {
	return &PulseAlertDto{
		TimestampUtc: time.Now().UTC(),
		Severity:     "info",
	}
}

type PulseState struct {
	LastRunAtUtc       *time.Time           `json:"last_run_at_utc"`
	LastCompletedAtUtc *time.Time           `json:"last_completed_at_utc"`
	NextRunAtUtc       *time.Time           `json:"next_run_at_utc"`
	LastResult         string               `json:"last_result"`
	LastSkipReason     *string              `json:"last_skip_reason"`
	RecentOkCount      int                  `json:"recent_ok_count"`
	PendingManualText  *string              `json:"pending_manual_text"`
	RecentAlerts       []PulseAlertDto      `json:"recent_alerts"`
	TaskLastRunUtc     map[string]time.Time `json:"task_last_run_utc"`
}

func NewDefaultPulseState() *PulseState {
	return &PulseState{
		LastResult:     "never",
		RecentAlerts:   []PulseAlertDto{},
		TaskLastRunUtc: make(map[string]time.Time),
	}
}

type PulseTaskDefinition struct {
	Name     string `json:"name"`
	Interval string `json:"interval"`
	Prompt   string `json:"prompt"`
}
