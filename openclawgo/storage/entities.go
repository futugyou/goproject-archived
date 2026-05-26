package storage

import "time"

type AdapterDeliveryLog struct {
	Id            string
	JobId         string
	ChannelType   string
	ChannelConfig string
	Status        DeliveryStatus
	ErrorMessage  string
	ResponseCode  int
	CreatedAt     time.Time
	DeliveredAt   *time.Time
	Job           *ScheduledJob
}

type DeliveryStatus string

const (
	DeliveryStatusPending DeliveryStatus = "Pending"
	DeliveryStatusSuccess DeliveryStatus = "Success"
	DeliveryStatusFailed  DeliveryStatus = "Failed"
)

type ScheduledJob struct {
	Id                      string
	Name                    string
	Prompt                  string
	CronExpression          string
	NextRunAt               *time.Time
	LastRunAt               *time.Time
	Status                  JobStatus
	IsRecurring             bool
	CreatedAt               time.Time
	StartAt                 *time.Time
	EndAt                   *time.Time
	TimeZone                string
	NaturalLanguageSchedule string
	AllowConcurrentRuns     bool
	AgentProfileName        string
	InputParametersJson     string
	LastOutputJson          string
	TriggerType             TriggerType
	WebhookEndpoint         string
	SourceTemplateName      string
	Runs                    []JobRun
}

type JobStatus uint

const (
	JobStatusDraft     JobStatus = 0
	JobStatusActive    JobStatus = 1
	JobStatusPaused    JobStatus = 2
	JobStatusCancelled JobStatus = 3
	JobStatusCompleted JobStatus = 4
	JobStatusArchived  JobStatus = 5
)

type TriggerType uint

const (
	TriggerTypeManual  TriggerType = 0
	TriggerTypeCron    TriggerType = 1
	TriggerTypeOneShot TriggerType = 2
	TriggerTypeWebhook TriggerType = 3
)

type JobRun struct {
	Id                     string
	JobId                  string
	Status                 string
	Result                 string
	Error                  string
	StartedAt              time.Time
	CompletedAt            *time.Time
	InputSnapshotJson      string
	TokensUsed             int
	ExecutedByAgentProfile string
	Job                    *ScheduledJob
}

type AgentInvocationLog struct {
	Id               string
	Kind             AgentInvocationKind
	SourceId         string
	AgentProfileName string
	Provider         string
	Model            string
	TokensIn         int
	TokensOut        int
	LatencyMs        int
	StartedAt        time.Time
	CompletedAt      *time.Time
	Error            string
}

type AgentInvocationKind uint

const (
	AgentInvocationKindChat   AgentInvocationKind = 0
	AgentInvocationKindJobRun AgentInvocationKind = 1
)
