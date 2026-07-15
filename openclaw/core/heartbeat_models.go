package core

import "time"

const (
	HeartbeatConfigDtoDefaultCronExpression    = "@hourly"
	HeartbeatConfigDtoDefaultDeliveryChannelId = "cron"

	HeartbeatTaskDtoDefaultTemplateKey   = "custom"
	HeartbeatTaskDtoDefaultPriority      = "normal"
	HeartbeatTaskDtoDefaultConditionMode = "and"

	HeartbeatValidationIssueDtoDefaultSeverity = "error"

	HeartbeatRunStatusDtoDefaultOutcome = "never"
)

// --- Structs ---

type HeartbeatConfigDto struct {
	Enabled             bool               `json:"enabled"`
	CronExpression      string             `json:"cron_expression"`
	Timezone            string             `json:"timezone"`
	DeliveryChannelId   string             `json:"delivery_channel_id"`
	DeliveryRecipientId string             `json:"delivery_recipient_id"`
	DeliverySubject     string             `json:"delivery_subject"`
	ModelId             string             `json:"model_id"`
	Tasks               []HeartbeatTaskDto `json:"tasks"`
}

func NewDefaultHeartbeatConfigDto() HeartbeatConfigDto {
	return HeartbeatConfigDto{
		CronExpression:    HeartbeatConfigDtoDefaultCronExpression,
		DeliveryChannelId: HeartbeatConfigDtoDefaultDeliveryChannelId,
		Tasks:             []HeartbeatTaskDto{},
	}
}

type HeartbeatTaskDto struct {
	Id            string                  `json:"id"`
	TemplateKey   string                  `json:"template_key"`
	Title         string                  `json:"title"`
	Target        string                  `json:"target"`
	Instruction   string                  `json:"instruction"`
	Priority      string                  `json:"priority"`
	Enabled       bool                    `json:"enabled"`
	ConditionMode string                  `json:"condition_mode"`
	Conditions    []HeartbeatConditionDto `json:"conditions"`
}

func NewDefaultHeartbeatTaskDto() HeartbeatTaskDto {
	return HeartbeatTaskDto{
		Id:            "",
		TemplateKey:   HeartbeatTaskDtoDefaultTemplateKey,
		Title:         "",
		Priority:      HeartbeatTaskDtoDefaultPriority,
		Enabled:       true,
		ConditionMode: HeartbeatTaskDtoDefaultConditionMode,
		Conditions:    []HeartbeatConditionDto{},
	}
}

type HeartbeatConditionDto struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

type HeartbeatTemplateDto struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Reason      string `json:"reason"`
}

type HeartbeatSuggestionDto struct {
	TemplateKey   string `json:"template_key"`
	Title         string `json:"title"`
	Target        string `json:"target"`
	Reason        string `json:"reason"`
	EvidenceCount int    `json:"evidence_count"`
}

type HeartbeatValidationIssueDto struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	TaskId   string `json:"task_id"`
}

func NewDefaultHeartbeatValidationIssueDto() HeartbeatValidationIssueDto {
	return HeartbeatValidationIssueDto{
		Severity: HeartbeatValidationIssueDtoDefaultSeverity,
		Code:     "",
		Message:  "",
	}
}

type HeartbeatCostEstimateDto struct {
	ProviderId                       string  `json:"provider_id"`
	ModelId                          string  `json:"model_id"`
	EstimatedSkillPromptChars        int     `json:"estimated_skill_prompt_chars"`
	EstimatedInputTokensPerRun       int     `json:"estimated_input_tokens_per_run"`
	EstimatedOkOutputTokensPerRun    int     `json:"estimated_ok_output_tokens_per_run"`
	EstimatedAlertOutputTokensPerRun int     `json:"estimated_alert_output_tokens_per_run"`
	EstimatedRunsPerMonth            int     `json:"estimated_runs_per_month"`
	EstimatedOkCostUsdPerRun         float64 `json:"estimated_ok_cost_usd_per_run"`
	EstimatedAlertCostUsdPerRun      float64 `json:"estimated_alert_cost_usd_per_run"`
	EstimatedOkCostUsdPerMonth       float64 `json:"estimated_ok_cost_usd_per_month"`
	EstimatedAlertCostUsdPerMonth    float64 `json:"estimated_alert_cost_usd_per_month"`
}

type HeartbeatRunStatusDto struct {
	Outcome            string     `json:"outcome"`
	LastRunAtUtc       *time.Time `json:"last_run_at_utc"`
	LastDeliveredAtUtc *time.Time `json:"last_delivered_at_utc"`
	DeliverySuppressed bool       `json:"delivery_suppressed"`
	InputTokens        int64      `json:"input_tokens"`
	OutputTokens       int64      `json:"output_tokens"`
	SessionId          string     `json:"session_id"`
	MessagePreview     string     `json:"message_preview"`
}

func NewDefaultHeartbeatRunStatusDto() HeartbeatRunStatusDto {
	return HeartbeatRunStatusDto{
		Outcome: HeartbeatRunStatusDtoDefaultOutcome,
	}
}

type HeartbeatPreviewResponse struct {
	Config             HeartbeatConfigDto            `json:"config"`
	ConfigPath         string                        `json:"config_path"`
	HeartbeatPath      string                        `json:"heartbeat_path"`
	MemoryMarkdownPath string                        `json:"memory_markdown_path"`
	HeartbeatMarkdown  string                        `json:"heartbeat_markdown"`
	PromptPreview      string                        `json:"prompt_preview"`
	DriftDetected      bool                          `json:"drift_detected"`
	ManagedJobActive   bool                          `json:"managed_job_active"`
	Issues             []HeartbeatValidationIssueDto `json:"issues"`
	AvailableTemplates []HeartbeatTemplateDto        `json:"available_templates"`
	Suggestions        []HeartbeatSuggestionDto      `json:"suggestions"`
	CostEstimate       HeartbeatCostEstimateDto      `json:"cost_estimate"`
}

type HeartbeatStatusResponse struct {
	Config             HeartbeatConfigDto            `json:"config"`
	ConfigPath         string                        `json:"config_path"`
	HeartbeatPath      string                        `json:"heartbeat_path"`
	MemoryMarkdownPath string                        `json:"memory_markdown_path"`
	ConfigExists       bool                          `json:"config_exists"`
	HeartbeatExists    bool                          `json:"heartbeat_exists"`
	DriftDetected      bool                          `json:"drift_detected"`
	LastRun            *HeartbeatRunStatusDto        `json:"last_run"`
	Issues             []HeartbeatValidationIssueDto `json:"issues"`
	AvailableTemplates []HeartbeatTemplateDto        `json:"available_templates"`
	Suggestions        []HeartbeatSuggestionDto      `json:"suggestions"`
	CostEstimate       HeartbeatCostEstimateDto      `json:"cost_estimate"`
}
