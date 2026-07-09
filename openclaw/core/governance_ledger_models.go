package core

import (
	"strings"
	"time"
)

// --- Const Groups (Enums) ---

// GovernanceDecisions
const (
	DecisionApproved  = "approved"
	DecisionRejected  = "rejected"
	DecisionEscalated = "escalated"
	DecisionExpired   = "expired"
	DecisionRevoked   = "revoked"
	DecisionUnknown   = "unknown"
)

// GovernanceDecisionStatuses
const (
	StatusActive     = "active"
	StatusExpired    = "expired"
	StatusRevoked    = "revoked"
	StatusSuperseded = "superseded"
)

// GovernanceScopes
const (
	ScopeOnce    = "once"
	ScopeSession = "session"
	ScopeActor   = "actor"
	ScopeChannel = "channel"
	ScopeProject = "project"
	ScopeTool    = "tool"
	ScopeGlobal  = "global"
	ScopeUnknown = "unknown"
)

// GovernanceLedgerSources
const (
	SourceManual                = "manual"
	SourceToolApproval          = "tool_approval"
	SourceApprovalTimeout       = "approval_timeout"
	SourceApprovalGrantConsumed = "approval_grant_consumed"
	SourceHarnessContract       = "harness_contract"
	SourceEvidenceReview        = "evidence_review"
	SourceLearningProposal      = "learning_proposal"
	SourceUnknown               = "unknown"
)

// --- Core Models & Query Objects ---

type GovernanceLedgerEntry struct {
	Id                 string                    `json:"id"`
	CreatedAtUtc       time.Time                 `json:"created_at_utc"`
	UpdatedAtUtc       time.Time                 `json:"updated_at_utc"`
	Decision           string                    `json:"decision"`
	Status             string                    `json:"status"`
	Source             string                    `json:"source"`
	ActionType         *string                   `json:"action_type,omitempty"`
	ToolName           *string                   `json:"tool_name,omitempty"`
	ActionSummary      string                    `json:"action_summary"`
	ArgumentSummary    *string                   `json:"argument_summary,omitempty"`
	RedactedArguments  *string                   `json:"redacted_arguments,omitempty"`
	RiskLevel          string                    `json:"risk_level"`
	Scope              string                    `json:"scope"`
	ScopeKey           *string                   `json:"scope_key,omitempty"`
	SessionId          *string                   `json:"session_id,omitempty"`
	HarnessContractId  *string                   `json:"harness_contract_id,omitempty"`
	EvidenceBundleId   *string                   `json:"evidence_bundle_id,omitempty"`
	LearningProposalId *string                   `json:"learning_proposal_id,omitempty"`
	ApprovalId         *string                   `json:"approval_id,omitempty"`
	ActorId            *string                   `json:"actor_id,omitempty"`
	ChannelId          *string                   `json:"channel_id,omitempty"`
	SenderId           *string                   `json:"sender_id,omitempty"`
	DecidedBy          *string                   `json:"decided_by,omitempty"`
	DecisionReason     *string                   `json:"decision_reason,omitempty"`
	ExpiresAtUtc       *time.Time                `json:"expires_at_utc,omitempty"`
	RevokedAtUtc       *time.Time                `json:"revoked_at_utc,omitempty"`
	RevokedBy          *string                   `json:"revoked_by,omitempty"`
	RevocationReason   *string                   `json:"revocation_reason,omitempty"`
	PolicyHint         *GovernancePolicyHint     `json:"policy_hint,omitempty"`
	Tags               []string                  `json:"tags"`
	Metadata           *GovernanceLedgerMetadata `json:"metadata,omitempty"`
}

// DefaultGovernanceLedgerEntry
func DefaultGovernanceLedgerEntry() GovernanceLedgerEntry {
	now := time.Now().UTC()
	return GovernanceLedgerEntry{
		CreatedAtUtc: now,
		UpdatedAtUtc: now,
		Decision:     DecisionUnknown,
		Status:       StatusActive,
		Source:       SourceManual,
		RiskLevel:    RiskLevelUnknown,
		Scope:        ScopeUnknown,
		Tags:         []string{},
	}
}

type GovernancePolicyHint struct {
	SuggestedFutureBehavior *string `json:"suggested_future_behavior,omitempty"`
	SuggestedScope          *string `json:"suggested_scope,omitempty"`
	Confidence              *string `json:"confidence,omitempty"`
	RequiresReview          bool    `json:"requires_review"`
	Notes                   *string `json:"notes,omitempty"`
}

// DefaultGovernancePolicyHint
func DefaultGovernancePolicyHint() GovernancePolicyHint {
	return GovernancePolicyHint{
		RequiresReview: true,
	}
}

type GovernanceLedgerMetadata struct {
	CreatedBy     *string           `json:"created_by,omitempty"`
	CorrelationId *string           `json:"correlation_id,omitempty"`
	Properties    map[string]string `json:"properties"`
}

// DefaultGovernanceLedgerMetadata
func DefaultGovernanceLedgerMetadata() GovernanceLedgerMetadata {
	return GovernanceLedgerMetadata{
		Properties: make(map[string]string),
	}
}

type GovernanceLedgerListQuery struct {
	Decision       *string    `json:"decision,omitempty"`
	Status         *string    `json:"status,omitempty"`
	ToolName       *string    `json:"tool_name,omitempty"`
	ActionType     *string    `json:"action_type,omitempty"`
	RiskLevel      *string    `json:"risk_level,omitempty"`
	Scope          *string    `json:"scope,omitempty"`
	SessionId      *string    `json:"session_id,omitempty"`
	ActorId        *string    `json:"actor_id,omitempty"`
	ChannelId      *string    `json:"channel_id,omitempty"`
	DecidedBy      *string    `json:"decided_by,omitempty"`
	Tag            *string    `json:"tag,omitempty"`
	CreatedFromUtc *time.Time `json:"created_from_utc,omitempty"`
	CreatedToUtc   *time.Time `json:"created_to_utc,omitempty"`
	Limit          int        `json:"limit"`
}

// DefaultGovernanceLedgerListQuery
func DefaultGovernanceLedgerListQuery() GovernanceLedgerListQuery {
	return GovernanceLedgerListQuery{
		Limit: 100,
	}
}

type GovernanceLedgerRevokeRequest struct {
	RevokedBy *string `json:"revoked_by,omitempty"`
	Reason    *string `json:"reason,omitempty"`
}

// --- Payload Responses ---

type GovernanceLedgerListResponse struct {
	Items []GovernanceLedgerEntry `json:"items"`
}

type GovernanceLedgerDetailResponse struct {
	Entry *GovernanceLedgerEntry `json:"entry,omitempty"`
}

type GovernanceLedgerMutationResponse struct {
	Success bool                   `json:"success"`
	Entry   *GovernanceLedgerEntry `json:"entry,omitempty"`
	Message string                 `json:"message"`
	Error   *string                `json:"error,omitempty"`
}
type ToolGovernanceDescriptorCatalog struct {
	descriptors map[string]ToolGovernanceDescriptor
}

func NewCatalog() *ToolGovernanceDescriptorCatalog {
	catalog := &ToolGovernanceDescriptorCatalog{
		descriptors: make(map[string]ToolGovernanceDescriptor),
	}

	read := func(name, category string, capabilities []string, network, fileSystem, external bool) ToolGovernanceDescriptor {
		return ToolGovernanceDescriptor{
			Name:                  name,
			Category:              category,
			RiskLevel:             ToolGovernanceRiskLevelLow,
			ReadOnly:              true,
			CanAccessNetwork:      network,
			CanAccessFileSystem:   fileSystem,
			CanSendDataExternally: external,
			Capabilities:          capabilities,
		}
	}

	write := func(name, category string, risk ToolGovernanceRiskLevel, capabilities []string, network, fileSystem, executeCode, external, approval bool) ToolGovernanceDescriptor {
		return ToolGovernanceDescriptor{
			Name:                  name,
			Category:              category,
			RiskLevel:             risk,
			RequiresApproval:      approval,
			ReadOnly:              false,
			CanAccessNetwork:      network,
			CanAccessFileSystem:   fileSystem,
			CanExecuteCode:        executeCode,
			CanSendDataExternally: external,
			Capabilities:          capabilities,
		}
	}

	list := []ToolGovernanceDescriptor{
		read("read_file", "filesystem", []string{"filesystem.read"}, false, true, false),
		write("write_file", "filesystem", ToolGovernanceRiskLevelHigh, []string{"filesystem.write"}, false, true, false, false, false),
		write("edit_file", "filesystem", ToolGovernanceRiskLevelHigh, []string{"filesystem.read", "filesystem.write"}, false, true, false, false, false),
		write("apply_patch", "filesystem", ToolGovernanceRiskLevelHigh, []string{"filesystem.read", "filesystem.write"}, false, true, false, false, false),
		write("shell", "execution", ToolGovernanceRiskLevelCritical, []string{"process.execute", "filesystem.read", "filesystem.write"}, false, true, true, false, true),
		write("process", "execution", ToolGovernanceRiskLevelCritical, []string{"process.execute", "process.control", "filesystem.read", "filesystem.write"}, false, true, true, false, true),
		write("code_exec", "execution", ToolGovernanceRiskLevelCritical, []string{"code.execute", "filesystem.read", "filesystem.write"}, false, true, true, false, true),
		write("git", "source-control", ToolGovernanceRiskLevelHigh, []string{"source.read", "source.write", "network.write"}, true, true, false, true, true),

		write("memory", "memory", ToolGovernanceRiskLevelMedium, []string{"memory.read", "memory.write"}, false, false, false, false, false),
		read("memory_get", "memory", []string{"memory.read"}, false, false, false),
		read("memory_search", "memory", []string{"memory.search"}, false, false, false),
		write("project_memory", "memory", ToolGovernanceRiskLevelMedium, []string{"memory.read", "memory.write"}, false, false, false, false, false),
		read("session_search", "session", []string{"session.search"}, false, false, false),
		write("sessions", "session", ToolGovernanceRiskLevelMedium, []string{"session.read", "session.message"}, false, false, false, false, false),
		read("sessions_history", "session", []string{"session.read"}, false, false, false),
		write("sessions_send", "session", ToolGovernanceRiskLevelMedium, []string{"session.message", "message.send"}, false, false, false, true, false),
		write("sessions_spawn", "session", ToolGovernanceRiskLevelMedium, []string{"session.create"}, false, false, false, false, false),
		read("session_status", "session", []string{"session.read"}, false, false, false),
		write("sessions_yield", "session", ToolGovernanceRiskLevelMedium, []string{"session.message", "message.send"}, false, false, false, true, false),
		read("agents_list", "delegation", []string{"agent.list"}, false, false, false),
		write("delegate_agent", "delegation", ToolGovernanceRiskLevelHigh, []string{"agent.delegate", "tool.invoke"}, false, false, false, false, true),

		read("profile_read", "profile", []string{"profile.read"}, false, false, false),
		write("profile_write", "profile", ToolGovernanceRiskLevelMedium, []string{"profile.write"}, false, false, false, false, false),
		write("todo", "productivity", ToolGovernanceRiskLevelMedium, []string{"todo.read", "todo.write"}, false, false, false, false, false),
		write("automation", "automation", ToolGovernanceRiskLevelHigh, []string{"automation.read", "automation.write", "automation.run"}, false, false, false, false, true),
		write("cron", "automation", ToolGovernanceRiskLevelHigh, []string{"automation.read", "automation.run"}, false, false, false, false, true),
		read("gateway", "runtime", []string{"runtime.read"}, false, false, false),

		write("message", "messaging", ToolGovernanceRiskLevelHigh, []string{"message.send", "data.export"}, false, false, false, true, true),
		read("web_search", "network", []string{"network.read", "external.http"}, true, false, true),
		read("web_fetch", "network", []string{"network.read", "external.http"}, true, false, true),
		read("x_search", "network", []string{"network.read", "external.http"}, true, false, true),
		write("browser", "browser", ToolGovernanceRiskLevelHigh, []string{"browser.navigate", "browser.evaluate", "network.read", "external.http"}, true, false, true, true, true),

		read("vision_analyze", "multimodal", []string{"vision.analyze", "data.export"}, false, false, true),
		write("text_to_speech", "multimodal", ToolGovernanceRiskLevelMedium, []string{"audio.generate", "data.export"}, false, false, false, true, false),
		write("image_gen", "multimodal", ToolGovernanceRiskLevelMedium, []string{"image.generate", "data.export"}, true, false, false, true, false),
		read("pdf_read", "document", []string{"filesystem.read", "document.read"}, false, true, false),

		write("canvas_present", "canvas", ToolGovernanceRiskLevelLow, []string{"ui.present"}, false, false, false, false, false),
		write("canvas_hide", "canvas", ToolGovernanceRiskLevelLow, []string{"ui.present"}, false, false, false, false, false),
		write("canvas_navigate", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.navigate", "network.read"}, true, false, false, false, false),
		read("canvas_snapshot", "canvas", []string{"ui.snapshot"}, false, false, false),
		write("a2ui_push", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.write"}, false, false, false, false, false),
		write("a2ui_reset", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.write"}, false, false, false, false, false),
		write("a2ui_eval", "canvas", ToolGovernanceRiskLevelHigh, []string{"ui.evaluate", "code.execute"}, false, false, true, false, true),
		write("a2ui_create_surface", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.write"}, false, false, false, false, false),
		write("a2ui_update_components", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.write"}, false, false, false, false, false),
		write("a2ui_update_data_model", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.write"}, false, false, false, false, false),
		write("a2ui_delete_surface", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.write"}, false, false, false, false, false),
		write("a2ui_sync_ui_to_data", "canvas", ToolGovernanceRiskLevelMedium, []string{"ui.write"}, false, false, false, false, false),

		write("external_cli", "external-cli", ToolGovernanceRiskLevelHigh, []string{"process.execute", "external.cli"}, false, true, true, false, true),
		write("payment", "payment", ToolGovernanceRiskLevelCritical, []string{"payment.execute", "data.export"}, false, false, false, true, true),
		read("stream_echo", "diagnostic", []string{"diagnostic.echo"}, false, false, false),

		write("calendar", "calendar", ToolGovernanceRiskLevelHigh, []string{"calendar.read", "calendar.write", "data.export"}, true, false, false, true, true),
		write("email", "email", ToolGovernanceRiskLevelHigh, []string{"email.read", "email.send", "data.export"}, true, false, false, true, true),
		write("inbox_zero", "email", ToolGovernanceRiskLevelHigh, []string{"email.read", "email.write", "data.export"}, true, false, false, true, true),
		write("database", "database", ToolGovernanceRiskLevelHigh, []string{"database.read", "database.write"}, false, true, false, false, true),
		read("home_assistant", "home-automation", []string{"home_assistant.read", "network.read"}, true, false, false),
		write("home_assistant_write", "home-automation", ToolGovernanceRiskLevelHigh, []string{"home_assistant.write", "network.write"}, true, false, false, true, true),
		read("mqtt", "iot", []string{"mqtt.read", "network.read"}, true, false, false),
		write("mqtt_publish", "iot", ToolGovernanceRiskLevelHigh, []string{"mqtt.publish", "network.write"}, true, false, false, true, true),
		read("notion", "notion", []string{"notion.read", "network.read"}, true, false, true),
		write("notion_write", "notion", ToolGovernanceRiskLevelHigh, []string{"notion.write", "network.write", "data.export"}, true, false, false, true, true),
	}

	for _, item := range list {
		catalog.descriptors[item.Name] = item
	}

	return catalog
}

// BuiltInToolNames 实例方法：获取所有内置工具的名称
func (c *ToolGovernanceDescriptorCatalog) BuiltInToolNames() []string {
	names := make([]string, 0, len(c.descriptors))
	for k := range c.descriptors {
		names = append(names, k)
	}
	return names
}

func (c *ToolGovernanceDescriptorCatalog) Contains(toolName string) bool {
	_, exists := c.descriptors[toolName]
	return exists
}

func (c *ToolGovernanceDescriptorCatalog) Resolve(toolName string, description string, actionDescriptor ToolActionDescriptor) ToolGovernanceDescriptor {
	var descriptor ToolGovernanceDescriptor
	known, exists := c.descriptors[toolName]

	if exists {
		descriptor = known
		if strings.TrimSpace(descriptor.Description) == "" {
			descriptor.Description = description
		}
	} else {
		descriptor = c.createFallback(toolName, description)
	}

	if actionDescriptor.RequiresApproval || actionDescriptor.IsMutation {
		parsedRisk := c.parseRisk(actionDescriptor.RiskLevel)
		riskToCompare := ToolGovernanceRiskLevelMedium
		if parsedRisk != nil {
			riskToCompare = *parsedRisk
		}

		// 模拟 C# 的 with 表达式
		descriptor.RequiresApproval = descriptor.RequiresApproval || actionDescriptor.RequiresApproval
		descriptor.ReadOnly = !actionDescriptor.IsMutation && descriptor.ReadOnly
		descriptor.RiskLevel = c.maxRisk(descriptor.RiskLevel, riskToCompare)
	}

	return descriptor
}

// 内部私有辅助方法（小写开头）

func (c *ToolGovernanceDescriptorCatalog) createFallback(toolName string, description string) ToolGovernanceDescriptor {
	return ToolGovernanceDescriptor{
		Name:         toolName,
		Description:  description,
		Category:     "plugin",
		RiskLevel:    ToolGovernanceRiskLevelMedium,
		ReadOnly:     false,
		Capabilities: []string{"plugin.invoke"},
	}
}

func (c *ToolGovernanceDescriptorCatalog) maxRisk(left, right ToolGovernanceRiskLevel) ToolGovernanceRiskLevel {
	if int(left) >= int(right) {
		return left
	}
	return right
}

func (c *ToolGovernanceDescriptorCatalog) parseRisk(riskLevel *string) *ToolGovernanceRiskLevel {
	if riskLevel == nil {
		return nil
	}
	trimmed := strings.ToLower(strings.TrimSpace(*riskLevel))
	if trimmed == "" {
		return nil
	}

	var result ToolGovernanceRiskLevel
	switch trimmed {
	case "low":
		result = ToolGovernanceRiskLevelLow
	case "ToolGovernanceRiskLevelMedium":
		result = ToolGovernanceRiskLevelMedium
	case "ToolGovernanceRiskLevelHigh":
		result = ToolGovernanceRiskLevelHigh
	case "ToolGovernanceRiskLevelCritical":
		result = ToolGovernanceRiskLevelCritical
	default:
		return nil
	}
	return &result
}
