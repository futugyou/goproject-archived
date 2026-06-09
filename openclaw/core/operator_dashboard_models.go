package core

import "time"

type OperatorDashboardSnapshot struct {
	Sessions    DashboardSessionSummary    `json:"sessions"`
	Approvals   DashboardApprovalSummary   `json:"approvals"`
	Memory      DashboardMemorySummary     `json:"memory"`
	Automations DashboardAutomationSummary `json:"automations"`
	Learning    DashboardLearningSummary   `json:"learning"`
	Delegation  DashboardDelegationSummary `json:"delegation"`
	Channels    DashboardChannelSummary    `json:"channels"`
	Plugins     DashboardPluginSummary     `json:"plugins"`
	Reliability ReliabilitySnapshot        `json:"reliability"`
}

func NewDefaultOperatorDashboardSnapshot() *OperatorDashboardSnapshot {
	return &OperatorDashboardSnapshot{
		Sessions:    *NewDefaultDashboardSessionSummary(),
		Approvals:   *NewDefaultDashboardApprovalSummary(),
		Memory:      *NewDefaultDashboardMemorySummary(),
		Automations: *NewDefaultDashboardAutomationSummary(),
		Learning:    *NewDefaultDashboardLearningSummary(),
		Delegation:  *NewDefaultDashboardDelegationSummary(),
		Channels:    *NewDefaultDashboardChannelSummary(),
		Plugins:     *NewDefaultDashboardPluginSummary(),
		Reliability: ReliabilitySnapshot{},
	}
}

type DashboardNamedMetric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type DashboardSessionSummary struct {
	Active      int                    `json:"active"`
	Persisted   int                    `json:"persisted"`
	UniqueTotal int                    `json:"unique_total"`
	Last24Hours int                    `json:"last_24_hours"`
	Last7Days   int                    `json:"last_7_days"`
	Starred     int                    `json:"starred"`
	Channels    []DashboardNamedMetric `json:"channels"`
	States      []DashboardNamedMetric `json:"states"`
}

func NewDefaultDashboardSessionSummary() *DashboardSessionSummary {
	return &DashboardSessionSummary{
		Channels: []DashboardNamedMetric{},
		States:   []DashboardNamedMetric{},
	}
}

type DashboardApprovalSummary struct {
	Pending              int                    `json:"pending"`
	DecisionsLast24Hours int                    `json:"decisions_last_24_hours"`
	ApprovedLast24Hours  int                    `json:"approved_last_24_hours"`
	RejectedLast24Hours  int                    `json:"rejected_last_24_hours"`
	PendingByTool        []DashboardNamedMetric `json:"pending_by_tool"`
	PendingByChannel     []DashboardNamedMetric `json:"pending_by_channel"`
}

func NewDefaultDashboardApprovalSummary() *DashboardApprovalSummary {
	return &DashboardApprovalSummary{
		PendingByTool:    []DashboardNamedMetric{},
		PendingByChannel: []DashboardNamedMetric{},
	}
}

type DashboardMemorySummary struct {
	ListedNotes      int                    `json:"listed_notes"`
	CatalogTruncated bool                   `json:"catalog_truncated"`
	ByClass          []DashboardNamedMetric `json:"by_class"`
	RecentNotes      []MemoryNoteItem       `json:"recent_notes"`
	RecentActivity   []RuntimeEventEntry    `json:"recent_activity"`
}

func NewDefaultDashboardMemorySummary() *DashboardMemorySummary {
	return &DashboardMemorySummary{
		ByClass:        []DashboardNamedMetric{},
		RecentNotes:    []MemoryNoteItem{},
		RecentActivity: []RuntimeEventEntry{},
	}
}

type AutomationItem struct {
	Id                string     `json:"id"`
	Name              string     `json:"name"`
	Enabled           bool       `json:"enabled"`
	IsDraft           bool       `json:"is_draft"`
	DeliveryChannelId string     `json:"delivery_channel_id"`
	TemplateKey       *string    `json:"template_key"`
	Outcome           string     `json:"outcome"`
	LastRunAtUtc      *time.Time `json:"last_run_at_utc"`
}

func NewDefaultAutomationItem() *AutomationItem {
	return &AutomationItem{
		Name:              "",
		DeliveryChannelId: "cron",
		Outcome:           "never",
	}
}

type DashboardAutomationSummary struct {
	Total           int                  `json:"total"`
	Enabled         int                  `json:"enabled"`
	Drafts          int                  `json:"drafts"`
	NeverRun        int                  `json:"never_run"`
	QueuedOrRunning int                  `json:"queued_or_running"`
	Failing         int                  `json:"failing"`
	Items           []AutomationItem     `json:"items"`
	Templates       []AutomationTemplate `json:"templates"`
}

func NewDefaultDashboardAutomationSummary() *DashboardAutomationSummary {
	return &DashboardAutomationSummary{
		Items:     []AutomationItem{},
		Templates: []AutomationTemplate{},
	}
}

type DashboardLearningSummary struct {
	Pending       int                    `json:"pending"`
	Approved      int                    `json:"approved"`
	Rejected      int                    `json:"rejected"`
	RolledBack    int                    `json:"rolled_back"`
	PendingByKind []DashboardNamedMetric `json:"pending_by_kind"`
	Recent        []LearningProposal     `json:"recent"`
}

func NewDefaultDashboardLearningSummary() *DashboardLearningSummary {
	return &DashboardLearningSummary{
		PendingByKind: []DashboardNamedMetric{},
		Recent:        []LearningProposal{},
	}
}

// DashboardDelegationSummary 委派摘要
type DashboardDelegationSummary struct {
	Enabled     bool     `json:"enabled"`
	MaxDepth    int      `json:"max_depth"`
	Last24Hours int      `json:"last_24_hours"`
	Profiles    []string `json:"profiles"`
}

func NewDefaultDashboardDelegationSummary() *DashboardDelegationSummary {
	return &DashboardDelegationSummary{
		Profiles: []string{},
	}
}

type DashboardChannelSummary struct {
	Ready         int                   `json:"ready"`
	Degraded      int                   `json:"degraded"`
	Misconfigured int                   `json:"misconfigured"`
	Items         []ChannelReadinessDto `json:"items"`
}

func NewDefaultDashboardChannelSummary() *DashboardChannelSummary {
	return &DashboardChannelSummary{
		Items: []ChannelReadinessDto{},
	}
}

type DashboardPluginSummary struct {
	Total                 int                    `json:"total"`
	Loaded                int                    `json:"loaded"`
	Disabled              int                    `json:"disabled"`
	Quarantined           int                    `json:"quarantined"`
	NeedsReview           int                    `json:"needs_review"`
	WarningCount          int                    `json:"warning_count"`
	ErrorCount            int                    `json:"error_count"`
	TrustLevels           []DashboardNamedMetric `json:"trust_levels"`
	CompatibilityStatuses []DashboardNamedMetric `json:"compatibility_statuses"`
}

func NewDefaultDashboardPluginSummary() *DashboardPluginSummary {
	return &DashboardPluginSummary{
		TrustLevels:           []DashboardNamedMetric{},
		CompatibilityStatuses: []DashboardNamedMetric{},
	}
}
