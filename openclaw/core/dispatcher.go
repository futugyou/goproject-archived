package core

type AutomationDispatchRequest struct {
	AutomationId  string `json:"automation_id"`
	TriggerSource string `json:"trigger_source"`
	ReplayOfRunId string `json:"replay_of_run_id"`
	RetryAttempt  int    `json:"retry_attempt"`
	SessionId     string `json:"session_id"`
	ChannelId     string `json:"channel_id"`
	SenderId      string `json:"sender_id"`
	Prompt        string `json:"prompt"`
	Subject       string `json:"subject"`
}

func DefaultAutomationDispatchRequest() *AutomationDispatchRequest {
	return &AutomationDispatchRequest{
		TriggerSource: "manual",
	}
}
