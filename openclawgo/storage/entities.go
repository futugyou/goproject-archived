package storage

import (
	"fmt"
	"time"

	"github.com/futugyou/openclawgo/models"
)

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

type AgentProfileEntity struct {
	Name                string
	DisplayName         string
	Provider            string
	Model               string
	Endpoint            string
	ApiKey              string
	DeploymentName      string
	AuthMode            string
	Instructions        string
	EnabledTools        string
	Temperature         float32
	MaxTokens           int
	IsDefault           bool
	RetrievalLevel      models.RetrievalLevel
	Kind                models.ProfileKind
	RequireToolApproval bool
	IsEnabled           bool
	LastTestedAt        *time.Time
	LastTestSucceeded   bool
	LastTestError       string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ChatMessageEntity struct {
	Id            string
	SessionId     string
	Role          string
	Content       string
	Name          string
	ToolCallId    string
	ToolCallsJson string
	CreatedAt     time.Time
	OrderIndex    int
	MessageType   string
	ToolName      string
	ToolArgsJson  string
	ToolDecision  string
	ToolDecidedBy string
	ToolDecidedAt *time.Time
	Session       *ChatSession
}

type ChatSession struct {
	Id               string
	Title            string
	Provider         string
	Model            string
	AgentProfileName string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Messages         []ChatMessageEntity
	Summaries        []SessionSummary
}

type SessionSummary struct {
	Id                  string
	SessionId           string
	Summary             string
	CoveredMessageCount int
	CreatedAt           time.Time
	Session             *ChatSession
}

type ChatSessionArtifact struct {
	Id               string
	SessionId        string
	Sequence         int
	ArtifactType     JobRunArtifactKind
	Title            string
	ContentInline    string
	ContentPath      string
	ContentSizeBytes int64
	MimeType         string
	CreatedAt        time.Time
	Metadata         string
	Session          *ChatSession
}

type JobRunArtifactKind uint

const (
	JobRunArtifactKindText     JobRunArtifactKind = 0
	JobRunArtifactKindMarkdown JobRunArtifactKind = 1
	JobRunArtifactKindJson     JobRunArtifactKind = 2
	JobRunArtifactKindFile     JobRunArtifactKind = 3
	JobRunArtifactKindLink     JobRunArtifactKind = 4
	JobRunArtifactKindError    JobRunArtifactKind = 5
)

type JobChannelConfiguration struct {
	Id            string
	JobId         string
	ChannelType   string
	ChannelConfig string
	IsEnabled     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Job           *ScheduledJob
}

type JobDefinitionStateChange struct {
	Id         string
	JobId      string
	FromStatus JobStatus
	ToStatus   JobStatus
	Reason     string
	ChangedBy  string
	ChangedAt  *time.Time
	Job        *ScheduledJob
}

type JobRunArtifact struct {
	Id               string
	JobRunId         string
	JobId            string
	Sequence         int
	ArtifactType     JobRunArtifactKind
	Title            string
	ContentInline    string
	ContentPath      string
	ContentSizeBytes int64
	MimeType         string
	CreatedAt        time.Time
	Metadata         string
	Run              *JobRun
}

const JobRunEventMaxPayloadBytes int = 4 * 1024

type JobRunEvent struct {
	Id            string
	JobRunId      string
	Sequence      int
	Timestamp     time.Time
	Kind          string
	ToolName      string
	ArgumentsJson string
	ResultJson    string
	Message       string
	DurationMs    int
	TokensUsed    int
	Run           *JobRun
}

func JobRunEventTruncate(value string) string {
	if len(value) == 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= JobRunEventMaxPayloadBytes {
		return value
	}

	var dropped = len(runes) - JobRunEventMaxPayloadBytes
	return string(runes[:JobRunEventMaxPayloadBytes]) + fmt.Sprintf("...[truncated %d chars]", dropped)
}

const (
	JobRunEventKindAgentStarted   string = "agent_started"
	JobRunEventKindToolCall       string = "tool_call"
	JobRunEventKindAgentCompleted string = "agent_completed"
	JobRunEventKindAgentFailed    string = "agent_failed"
	JobRunEventKindProfileRefused string = "profile_refused"
)

type statusTransition struct {
	From JobStatus
	To   JobStatus
}

var allowedTransitions = map[statusTransition]struct{}{
	{JobStatusDraft, JobStatusActive}:       {},
	{JobStatusDraft, JobStatusCancelled}:    {},
	{JobStatusActive, JobStatusPaused}:      {},
	{JobStatusActive, JobStatusCancelled}:   {},
	{JobStatusActive, JobStatusCompleted}:   {},
	{JobStatusPaused, JobStatusActive}:      {},
	{JobStatusPaused, JobStatusCancelled}:   {},
	{JobStatusCompleted, JobStatusArchived}: {},
	{JobStatusCancelled, JobStatusArchived}: {},
}

func IsJobStatusTransitionAllowed(from, to JobStatus) bool {
	_, ok := allowedTransitions[statusTransition{
		From: from,
		To:   to,
	}]
	return ok
}

func IsJobStatusTerminal(status JobStatus) bool {
	switch status {
	case JobStatusCompleted,
		JobStatusCancelled,
		JobStatusArchived:
		return true
	default:
		return false
	}
}

func IsJobStatusEditable(status JobStatus) bool {
	switch status {
	case JobStatusDraft,
		JobStatusPaused:
		return true
	default:
		return false
	}
}

// Whether a job in this status should be hidden from default UI lists.
func IsJobStatusHiddenByDefault(status JobStatus) bool {
	return status == JobStatusArchived
}

type McpServerDefinitionEntity struct {
	Id                     string
	Name                   string
	Transport              string
	Command                string
	ArgsJson               string
	EnvJson                string
	Url                    string
	HeadersJson            string
	Enabled                bool
	DefaultRequireApproval bool
	IsBuiltIn              bool
	LastError              string
	LastSeenUtc            *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type McpToolOverrideEntity struct {
	ServerId        string
	ToolName        string
	RequireApproval bool
	Disabled        bool
	UpdatedAt       time.Time
}

type ModelProviderDefinition struct {
	Name              string
	ProviderType      string
	DisplayName       string
	Endpoint          string
	Model             string
	ApiKey            string
	DeploymentName    string
	AuthMode          string
	IsSupported       bool
	LastTestedAt      *time.Time
	LastTestSucceeded bool
	LastTestError     string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type OAuthTokenEntity struct {
	Id                     int
	Provider               string
	UserId                 string
	AccessTokenCiphertext  string
	RefreshTokenCiphertext string
	ExpiresAtUtc           string
	Scopes                 string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type ProviderSetting struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

type SchemaVersionEntity struct {
	Key       string
	Value     string
	AppliedAt time.Time
}

type SecretAccessAuditEntity struct {
	Id              string
	Sequence        int64
	SecretName      string
	CallerType      string
	CallerId        string
	SessionId       string
	AccessedAt      time.Time
	Success         bool
	PreviousRowHash string
	RowHash         string
}

type SecretEntity struct {
	Name           string
	EncryptedValue string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	PurgeAfter     *time.Time
	Versions       []SecretVersionEntity
}

type SecretVersionEntity struct {
	Id             string
	SecretName     string
	Version        int
	EncryptedValue string
	CreatedAt      time.Time
	IsCurrent      bool
	SupersededAt   *time.Time
	Secret         *SecretEntity
}

type SkillVector struct {
	Id        string
	SkillName string
	Embedding []byte
	CreatedAt time.Time
}

type ToolApprovalLog struct {
	Id                 string
	RequestId          string
	SessionId          string
	ToolName           string
	AgentProfileName   string
	Approved           bool
	RememberForSession bool
	Source             ApprovalDecisionSource
	DecidedAt          time.Time
}

type ApprovalDecisionSource uint

const (
	ApprovalDecisionSourceUser          ApprovalDecisionSource = 0
	ApprovalDecisionSourceTimeout       ApprovalDecisionSource = 1
	ApprovalDecisionSourceSessionMemory ApprovalDecisionSource = 2
)

type ToolCallRecord struct {
	Id         string
	SessionId  string
	MessageId  string
	ToolName   string
	Arguments  string
	Result     string
	Success    bool
	DurationMs float32
	ExecutedAt time.Time
}

type ToolTestRecord struct {
	Name              string
	LastTestedAt      *time.Time
	LastTestSucceeded bool
	LastTestError     string
	LastTestMode      string
}
