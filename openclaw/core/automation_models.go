package core

import "time"

type AutomationsConfig struct {
	Enabled                  bool   `json:"enabled"`
	DefaultDeliveryChannelId string `json:"default_delivery_channel_id"`
	SuggestionThreshold      int    `json:"suggestion_threshold"`
}

func DefaultAutomationsConfig() *AutomationsConfig {
	return &AutomationsConfig{
		Enabled:                  true,
		DefaultDeliveryChannelId: "cron",
		SuggestionThreshold:      3,
	}
}

type AutomationRetryPolicy struct {
	Enabled    bool `json:"enabled"`
	MaxRetries int  `json:"max_retries"`
}

type AutomationDefinition struct {
	Id                          string                `json:"id"` // required
	Name                        string                `json:"name"`
	Enabled                     bool                  `json:"enabled"`
	Schedule                    string                `json:"schedule"`
	Timezone                    string                `json:"timezone,omitempty"`
	Prompt                      string                `json:"prompt"`
	ModelId                     string                `json:"model_id,omitempty"`
	ResponseMode                string                `json:"response_mode"`
	RunOnStartup                bool                  `json:"run_on_startup"`
	SessionId                   string                `json:"session_id,omitempty"`
	DeliveryChannelId           string                `json:"delivery_channel_id"`
	DeliveryRecipientId         string                `json:"delivery_recipient_id,omitempty"`
	DeliverySubject             string                `json:"delivery_subject,omitempty"`
	Tags                        []string              `json:"tags" gorm:"type:text[];not null;default:'{}'"`
	IsDraft                     bool                  `json:"is_draft"`
	Source                      string                `json:"source"`
	TemplateKey                 string                `json:"template_key,omitempty"`
	CreatedByLearningProposalId string                `json:"created_by_learning_proposal_id,omitempty"`
	Verification                *VerificationPolicy   `json:"verification,omitempty" gorm:"serializer:json"`
	RetryPolicy                 AutomationRetryPolicy `json:"retry_policy" gorm:"serializer:json"`
	CreatedAtUtc                time.Time             `json:"created_at_utc"`
	UpdatedAtUtc                time.Time             `json:"updated_at_utc"`
}

func DefaultAutomationDefinition() AutomationDefinition {
	return AutomationDefinition{
		Name:              "",
		Enabled:           true,
		Schedule:          "@hourly",
		Prompt:            "",
		ResponseMode:      "default",
		DeliveryChannelId: "cron",
		Tags:              []string{},
		Source:            "managed",
		RetryPolicy:       AutomationRetryPolicy{},
		CreatedAtUtc:      time.Now().UTC(),
		UpdatedAtUtc:      time.Now().UTC(),
	}
}

type AutomationRunState struct {
	AutomationId             string     `json:"automation_id"` // required
	Outcome                  string     `json:"outcome"`
	LifecycleState           string     `json:"lifecycle_state"`
	VerificationStatus       string     `json:"verification_status"`
	HealthState              string     `json:"health_state"`
	LastRunAtUtc             *time.Time `json:"last_run_at_utc,omitempty"`
	LastCompletedAtUtc       *time.Time `json:"last_completed_at_utc,omitempty"`
	LastDeliveredAtUtc       *time.Time `json:"last_delivered_at_utc,omitempty"`
	LastVerifiedSuccessAtUtc *time.Time `json:"last_verified_success_at_utc,omitempty"`
	QuarantinedAtUtc         *time.Time `json:"quarantined_at_utc,omitempty"`
	NextRetryAtUtc           *time.Time `json:"nextRetry_at_utc,omitempty"`
	DeliverySuppressed       bool       `json:"delivery_suppressed"`
	InputTokens              int64      `json:"input_tokens"`
	OutputTokens             int64      `json:"output_tokens"`
	FailureStreak            int        `json:"failure_streak"`
	UnverifiedStreak         int        `json:"unverified_streak"`
	NextRetryAttempt         *int       `json:"next_retry_attempt,omitempty"`
	LastRunId                string     `json:"last_run_id,omitempty"`
	SessionId                string     `json:"session_id,omitempty"`
	MessagePreview           string     `json:"message_preview,omitempty"`
	VerificationSummary      string     `json:"verification_summary,omitempty"`
	QuarantineReason         string     `json:"quarantine_reason,omitempty"`
	SignalSeverity           string     `json:"signal_severity,omitempty"`
}

func DefaultAutomationRunState(automationId string) AutomationRunState {
	return AutomationRunState{
		AutomationId:       automationId,
		Outcome:            "never",
		LifecycleState:     "never",
		VerificationStatus: "not_run",
		HealthState:        "unknown",
	}
}

type AutomationRunRecord struct {
	RunId               string                    `json:"run_id"`        // required
	AutomationId        string                    `json:"automation_id"` // required
	TriggerSource       string                    `json:"trigger_source"`
	LifecycleState      string                    `json:"lifecycle_state"`
	VerificationStatus  string                    `json:"verification_status"`
	ReplayOfRunId       string                    `json:"replay_of_run_id,omitempty"`
	RetryAttempt        int                       `json:"retry_attempt"`
	SessionId           string                    `json:"session_id,omitempty"`
	MessagePreview      string                    `json:"message_preview,omitempty"`
	VerificationSummary string                    `json:"verification_summary,omitempty"`
	VerificationChecks  []VerificationCheckResult `json:"verification_checks"`
	StartedAtUtc        time.Time                 `json:"started_at_utc"`
	CompletedAtUtc      *time.Time                `json:"completed_at_utc,omitempty"`
	LastDeliveredAtUtc  *time.Time                `json:"lastDelivered_at_utc,omitempty"`
	DeliverySuppressed  bool                      `json:"delivery_suppressed"`
	InputTokens         int64                     `json:"input_tokens"`
	OutputTokens        int64                     `json:"output_tokens"`
}

func DefaultAutomationRunRecord(runId, automationId string) AutomationRunRecord {
	return AutomationRunRecord{
		RunId:              runId,
		AutomationId:       automationId,
		TriggerSource:      "manual",
		LifecycleState:     "queued",
		VerificationStatus: "not_run",
		VerificationChecks: []VerificationCheckResult{},
		StartedAtUtc:       time.Now().UTC(),
	}
}

type AutomationTemplate struct {
	Key               string   `json:"key"`
	Label             string   `json:"label"`
	Description       string   `json:"description"`
	Category          string   `json:"category"`
	SuggestedName     string   `json:"suggested_name"`
	Schedule          string   `json:"schedule"`
	Prompt            string   `json:"prompt"`
	DeliveryChannelId string   `json:"delivery_channel_id"`
	DeliverySubject   string   `json:"delivery_subject,omitempty"`
	Tags              []string `json:"tags"`
	Available         bool     `json:"available"`
	Reason            string   `json:"reason,omitempty"`
}

func DefaultAutomationTemplate() AutomationTemplate {
	return AutomationTemplate{
		Key:               "",
		Label:             "",
		Description:       "",
		Category:          "",
		SuggestedName:     "",
		Schedule:          "@daily",
		Prompt:            "",
		DeliveryChannelId: "cron",
		Tags:              []string{},
	}
}

type AutomationTemplateListResponse struct {
	Items []AutomationTemplate `json:"items"`
}

func DefaultAutomationTemplateListResponse() AutomationTemplateListResponse {
	return AutomationTemplateListResponse{
		Items: []AutomationTemplate{},
	}
}

type AutomationValidationIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func DefaultAutomationValidationIssue() AutomationValidationIssue {
	return AutomationValidationIssue{
		Severity: "error",
	}
}

type AutomationPreview struct {
	Definition            AutomationDefinition        `json:"definition"`
	Issues                []AutomationValidationIssue `json:"issues"`
	Templates             []AutomationTemplate        `json:"templates"`
	PromptPreview         string                      `json:"prompt_preview"`
	EstimatedRunsPerMonth int                         `json:"estimated_runs_per_month"`
}

func DefaultAutomationPreview(def AutomationDefinition) AutomationPreview {
	return AutomationPreview{
		Definition: def,
		Issues:     []AutomationValidationIssue{},
		Templates:  []AutomationTemplate{},
	}
}
