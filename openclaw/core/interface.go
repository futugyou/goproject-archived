package core

import (
	"context"
	"time"
)

// IContactStore 接口定义
type IContactStore interface {
	Touch(ctx context.Context, phoneE164 string) (Contact, error)
	Get(ctx context.Context, phoneE164 string) (*Contact, error)
	SetDoNotText(ctx context.Context, phoneE164 string, doNotText bool) error
}

// PlanExecuteVerifyOrchestrator 接口
type IPlanExecuteVerifyOrchestrator interface {
	EvaluateTool(ctx context.Context, toolCtx *PlanExecuteVerifyToolContext) (*PlanExecuteVerifyDecision, error)
	RecordApprovalDecision(ctx context.Context, run *PlanExecuteVerifyRun, approved bool) error
	CompleteTool(ctx context.Context, run *PlanExecuteVerifyRun, invocation ToolInvocation) (*PlanExecuteVerifyRun, error)
	VerifyRun(ctx context.Context, runID string) (*PlanExecuteVerifyRun, error)
	GetRun(id string) *PlanExecuteVerifyRun
	ListRuns(limit int) []PlanExecuteVerifyRun
}

type IEvidenceBundleStore interface {
	Save(ctx context.Context, bundle EvidenceBundle) error
	Get(ctx context.Context, id string) (*EvidenceBundle, error)
	List(ctx context.Context, query EvidenceBundleListQuery) ([]EvidenceBundle, error)
	Delete(ctx context.Context, id string) error
}

type IAutomationStore interface {
	ListAutomations(ctx context.Context) ([]AutomationDefinition, error)
	GetAutomation(ctx context.Context, automationId string) (*AutomationDefinition, error)
	SaveAutomation(ctx context.Context, automation AutomationDefinition) error
	DeleteAutomation(ctx context.Context, automationId string) error
	GetRunState(ctx context.Context, automationId string) (*AutomationRunState, error)
	SaveRunState(ctx context.Context, runState AutomationRunState) error
	ListRunRecords(ctx context.Context, automationId string, limit int) ([]AutomationRunRecord, error)
	GetRunRecord(ctx context.Context, automationId string, runId string) (*AutomationRunRecord, error)
	SaveRunRecord(ctx context.Context, runRecord AutomationRunRecord) error
	PruneRunRecords(ctx context.Context, automationId string, retainCount int) error
}

type IUserProfileStore interface {
	GetProfile(ctx context.Context, actorId string) (*UserProfile, error)
	ListProfiles(ctx context.Context) ([]UserProfile, error)
	SaveProfile(ctx context.Context, profile UserProfile) error
	DeleteProfile(ctx context.Context, actorId string) error
}

type IConnectedAccountStore interface {
	ListAccounts(ctx context.Context) ([]ConnectedAccount, error)
	GetAccount(ctx context.Context, accountID string) (*ConnectedAccount, error)
	SaveAccount(ctx context.Context, account ConnectedAccount) error
	DeleteAccount(ctx context.Context, accountID string) error
}

type IBackendSessionStore interface {
	ListBackendSessions(ctx context.Context, backendID *string) ([]BackendSessionRecord, error)
	GetBackendSession(ctx context.Context, sessionID string) (*BackendSessionRecord, error)
	SaveBackendSession(ctx context.Context, session BackendSessionRecord) error
	DeleteBackendSession(ctx context.Context, sessionID string) error
	AppendBackendEvent(ctx context.Context, evt BackendEvent) error
	ListBackendEvents(ctx context.Context, sessionID string, afterSequence int64, limit int) ([]BackendEvent, error)
}

type IBackendCredentialResolver interface {
	ResolveWithSource(ctx context.Context, provider string, source *BackendCredentialSourceConfig) (*ResolvedBackendCredential, error)
	ResolveWithSecretRef(ctx context.Context, provider string, source *ConnectedAccountSecretRef) (*ResolvedBackendCredential, error)
}

type IBackendSessionRuntime interface {
	Session() BackendSessionRecord
	AppendEvent(ctx context.Context, evt BackendEvent) error
	UpdateSession(ctx context.Context, session BackendSessionRecord) error
}

type ICodingAgentBackend interface {
	Definition() BackendDefinition
	Probe(ctx context.Context, request BackendProbeRequest) (BackendProbeResult, error)
	StartSession(ctx context.Context, request StartBackendSessionRequest, runtime IBackendSessionRuntime) (BackendSessionHandle, error)
	SendInput(ctx context.Context, sessionID string, input BackendInput) error
	StopSession(ctx context.Context, sessionID string) error
}

type ICodingAgentBackendRegistry interface {
	List() []BackendDefinition
	TryGet(backendID string) (ICodingAgentBackend, bool)
}

type IAgentWorkflowRunner interface {
	BackendId() string
	WorkflowId() string
	GetSummary() (*AgentWorkflowBackendSummary, error)
	Run(ctx context.Context, request *AgentWorkflowRequest) (*AgentWorkflowRunResult, error)
	Get(ctx context.Context, runId string) (*AgentWorkflowRunSnapshot, error)
	Respond(ctx context.Context, runId string, response *AgentWorkflowResponse) (*AgentWorkflowRunSnapshot, error)
	Stream(ctx context.Context, runId string) (<-chan *AgentWorkflowEvent, error)
}

type IAutomationRunDispatcher interface {
	PrepareDispatch(ctx context.Context, request *AutomationDispatchRequest) (*InboundMessage, error)
}

type IChannelAdapter interface {
	Close(ctx context.Context) error
	ChannelId() string
	Start(ctx context.Context) error
	Send(ctx context.Context, message *OutboundMessage) error
	OnMessageReceived(handler func(ctx context.Context, msg *InboundMessage) error)
}

type IBridgedChannelControl interface {
	IChannelAdapter

	SelfId() (string, bool)
	SelfIds() []string
	SendTyping(ctx context.Context, recipientId string, isTyping bool, accountId *string) error
	SendReadReceipt(ctx context.Context, messageId string, remoteJid *string, participant *string, accountId *string) error
}

type IExecutionBackend interface {
	Name() string
	Execute(ctx context.Context, request *ExecutionRequest) (*ExecutionResult, error)
}

type IGovernanceLedgerStore interface {
	Save(ctx context.Context, entry *GovernanceLedgerEntry) error
	Get(ctx context.Context, id string) (*GovernanceLedgerEntry, error)
	List(ctx context.Context, query *GovernanceLedgerListQuery) ([]GovernanceLedgerEntry, error)
	Revoke(ctx context.Context, id string, revokedBy string, reason string) (*GovernanceLedgerEntry, error)
}

type IHarnessContractStore interface {
	Save(ctx context.Context, contract *HarnessContract) error
	Get(ctx context.Context, id string) (*HarnessContract, error)
	List(ctx context.Context, query *HarnessContractListQuery) ([]HarnessContract, error)
	Delete(ctx context.Context, id string) error
}

type ILearningProposalStore interface {
	ListProposals(ctx context.Context, status *string, kind *string) ([]LearningProposal, error)
	GetProposal(ctx context.Context, proposalId string) (*LearningProposal, error)
	SaveProposal(ctx context.Context, proposal *LearningProposal) error
}

// IMemoryNoteSearch 搜索笔记
type IMemoryNoteSearch interface {
	SearchNotes(ctx context.Context, query string, prefix *string, limit int) ([]MemoryNoteHit, error)
}

// IMemoryNoteCatalog 笔记目录
type IMemoryNoteCatalog interface {
	ListNotes(ctx context.Context, prefix string, limit int) ([]MemoryNoteCatalogEntry, error)
	GetNoteEntry(ctx context.Context, key string) (*MemoryNoteCatalogEntry, error)
}

// IMemoryRetentionStore 内存留存/清理存储
type IMemoryRetentionStore interface {
	Sweep(ctx context.Context, request *RetentionSweepRequest, protectedSessionIds map[string]struct{}) (*RetentionSweepResult, error)
	GetRetentionStats(ctx context.Context) (*RetentionStoreStats, error)
}

// IMemoryStore 核心会话与笔记存储
type IMemoryStore interface {
	GetSession(ctx context.Context, sessionId string) (*Session, error)
	SaveSession(ctx context.Context, session Session) error
	LoadNote(ctx context.Context, key string) (string, error)
	SaveNote(ctx context.Context, key string, content string) error
	DeleteNote(ctx context.Context, key string) error
	ListNotesWithPrefix(ctx context.Context, prefix string) ([]string, error)

	// ── Conversation Branching ─────────────────────────────────────────
	SaveBranch(ctx context.Context, branch SessionBranch) error
	LoadBranch(ctx context.Context, branchId string) (*SessionBranch, error)
	ListBranches(ctx context.Context, sessionId string) ([]SessionBranch, error)
	DeleteBranch(ctx context.Context, branchId string) error
}

// IModelProfileRegistry 模型配置注册表
type IModelProfileRegistry interface {
	DefaultProfileId() *string
	TryGet(profileId string) (*ModelProfile, bool)
	ListStatuses() ([]ModelProfileStatus, error)
}

// IModelSelectionPolicy 模型选择策略
type IModelSelectionPolicy interface {
	Resolve(request ModelSelectionRequest) (*ModelSelectionResult, error)
}

// IRestartableChannelAdapter 可重启的通道适配器
type IRestartableChannelAdapter interface {
	Restart(ctx context.Context) error
}

// ISandboxCapableTool 支持沙箱环境的工具
type ISandboxCapableTool interface {
	DefaultSandboxMode() ToolSandboxMode
	CreateSandboxRequest(argumentsJson string) (*SandboxExecutionRequest, error)
	FormatSandboxResult(argumentsJson string, result SandboxResult) (string, error)
}

// ISessionAdminStore 会话管理存储
type ISessionAdminStore interface {
	ListSessions(ctx context.Context, page int, pageSize int, query SessionListQuery) (*PagedSessionList, error)
}

// ISessionSearchStore 会话搜索存储
type ISessionSearchStore interface {
	SearchSessions(ctx context.Context, query SessionSearchQuery) (*SessionSearchResult, error)
}

// ISharedHarnessStateStore 共享测试基座状态存储
type ISharedHarnessStateStore interface {
	Save(ctx context.Context, state SharedHarnessState) error
	Get(ctx context.Context, id string) (*SharedHarnessState, error)
	GetBySession(ctx context.Context, sessionId string) (*SharedHarnessState, error)
	List(ctx context.Context, query SharedHarnessStateListQuery) ([]SharedHarnessState, error)
	Delete(ctx context.Context, id string) error
}

// IStreamingTool 支持流式返回的工具
type IStreamingTool interface {
	ExecuteStreaming(ctx context.Context, argumentsJson string) (<-chan string, error)
}

// IStructuredMemoryProvider 结构化内存提供者
type IStructuredMemoryProvider interface {
	GetStatus(ctx context.Context) (*StructuredMemoryStatusResponse, error)
	Search(ctx context.Context, query string, limit int, scope *string) (*StructuredMemorySearchResult, error)
	Open(ctx context.Context, path string, depth int, view string) (*StructuredMemoryOpenResult, error)
	Recent(ctx context.Context, days int, limit int, scope *string) (*StructuredMemoryRecentResult, error)
	Export(ctx context.Context, path string, mode string) (*StructuredMemoryExportResult, error)
	CreateHandoff(ctx context.Context, path string) (*StructuredMemoryHandoffResult, error)
	Validate(ctx context.Context) (*StructuredMemoryValidationResult, error)
	RefreshIndex(ctx context.Context) (*StructuredMemoryValidationResult, error)
}

// ITool 基础工具接口
type ITool interface {
	Name() string
	Description() string
	ParameterSchema() string
	Execute(ctx context.Context, argumentsJson string) (string, error)
}

// IToolActionDescriptorProvider 工具动作描述符解析器
type IToolActionDescriptorProvider interface {
	ResolveActionDescriptor(argumentsJson string) (*ToolActionDescriptor, error)
}

// IToolGovernanceService 工具合规/治理服务
type IToolGovernanceService interface {
	Authorize(ctx context.Context, context ToolGovernanceContext) (*GovernanceDecision, error)
	RecordResult(ctx context.Context, context ToolGovernanceContext, decision GovernanceDecision, result ToolGovernanceExecutionResult) error
}

// IToolHook 工具执行前后的钩子
type IToolHook interface {
	Name() string
	BeforeExecute(ctx context.Context, toolName string, arguments string) bool
	AfterExecute(ctx context.Context, toolName string, arguments string, result string, duration time.Duration, failed bool) error
}

type IToolHookWithContext interface {
	IToolHook
	BeforeExecuteContext(ctx context.Context, context ToolHookContext) bool
	AfterExecuteContext(ctx context.Context, context ToolHookContext, result string, duration time.Duration, failed bool) error
}
type IToolLocalExecutionPolicy interface {
	LocalExecutionSupported() bool
	LocalExecutionUnavailableFailureCode() string
	LocalExecutionUnavailableMessage() string
}

type IToolPresetResolver interface {
	Resolve(session Session, availableToolNames []string) ResolvedToolPreset
	ListPresets(availableToolNames []string) []ResolvedToolPreset
}

type IToolSandbox interface {
	Execute(ctx context.Context, request SandboxExecutionRequest) (SandboxResult, error)
}

type IToolWithContext interface {
	ITool
	ExecuteContext(ctx context.Context, argumentsJson string, toolContext ToolExecutionContext) (string, error)
}

type ITurnTokenUsageObserver interface {
	RecordTurn(record TurnTokenUsageRecord)
}

type IMessageMiddleware interface {
	/// <summary>Display name for logging/diagnostics.</summary>
	GetName() string

	Invoke(ctx context.Context, messageContext *MessageContext, next func(context.Context) error) error
}

type ISensitiveDataRedactor interface {
	GetName() string
	Redact(value string) string
}

type IRedactionPipeline interface {
	Redact(value string) string
	RedactSessionInPlace(session *Session) error
	RedactSession(session *Session) *Session
	RedactBranch(branch *SessionBranch) *SessionBranch
}

type ISentinelSubstitutionService interface {
	Substitute(ctx context.Context, sentinelContext *SentinelSubstitutionContext) (*SentinelSubstitutionResult, error)
}

type IGoalService interface {
	CreateGoal(sessionId, objective string, tokenBudget, tokensAtStart int64) (*SessionGoal, error)
	GetGoal(sessionId string) (*SessionGoal, error)
	UpdateStatus(sessionId string, newStatus GoalStatus, note *string) error
	UpdateTokenUsage(sessionId string, sessionTotalTokens int64) error
	IncrementContinuationCount(sessionId string) int
	RecordTurnHash(sessionId, normalizedText string) bool
	ClearGoal(sessionId string) error
	HasActiveGoal(sessionId string) bool
	RecordGoalHistory(goal *SessionGoal) error
}

type IToolResultInterceptor interface {
	GetOrder() int
	GetName() string
	Intercept(ctx context.Context, reductionContext ReductionContext) (string, error)
}
