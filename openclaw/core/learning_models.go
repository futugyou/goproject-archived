package core

import "time"

const (
	LearningProposalKindSkillDraft           = "skill_draft"
	LearningProposalKindProfileUpdate        = "profile_update"
	LearningProposalKindAutomationSuggestion = "automation_suggestion"
	LearningProposalKindHarnessChange        = "harness_change"
)

// LearningProposalStatus
const (
	LearningProposalStatusPending    = "pending"
	LearningProposalStatusApproved   = "approved"
	LearningProposalStatusRejected   = "rejected"
	LearningProposalStatusRolledBack = "rolled_back"
)

// LearningProposalRiskLevels
const (
	LearningProposalRiskLevelsLow      = "low"
	LearningProposalRiskLevelsMedium   = "medium"
	LearningProposalRiskLevelsHigh     = "high"
	LearningProposalRiskLevelsCritical = "critical"
)

// HarnessEvolutionComponents
const (
	HarnessEvolutionComponentsMemory        = "memory"
	HarnessEvolutionComponentsRetrieval     = "retrieval"
	HarnessEvolutionComponentsTools         = "tools"
	HarnessEvolutionComponentsApprovals     = "approvals"
	HarnessEvolutionComponentsVerification  = "verification"
	HarnessEvolutionComponentsRouting       = "routing"
	HarnessEvolutionComponentsPrompt        = "prompt"
	HarnessEvolutionComponentsModelProfile  = "model_profile"
	HarnessEvolutionComponentsPulse         = "pulse"
	HarnessEvolutionComponentsSecurity      = "security"
	HarnessEvolutionComponentsGovernance    = "governance"
	HarnessEvolutionComponentsContextBudget = "context_budget"
	HarnessEvolutionComponentsChannel       = "channel"
	HarnessEvolutionComponentsSandbox       = "sandbox"
	HarnessEvolutionComponentsUnknown       = "unknown"
)

// HarnessEvolutionApplyModes
const (
	HarnessEvolutionApplyModesManualOnly   = "manual_only"
	HarnessEvolutionApplyModesConfigPatch  = "config_patch"
	HarnessEvolutionApplyModesPolicyPatch  = "policy_patch"
	HarnessEvolutionApplyModesSkillUpdate  = "skill_update"
	HarnessEvolutionApplyModesMemoryUpdate = "memory_update"
	HarnessEvolutionApplyModesUnsupported  = "unsupported"
)

// LearningProposalValidationStatuses
const (
	LearningProposalValidationStatusesNotRun  = "not_run"
	LearningProposalValidationStatusesValid   = "valid"
	LearningProposalValidationStatusesWarning = "warning"
	LearningProposalValidationStatusesError   = "error"
)

// AutomationSuggestionQualityDecisions
const (
	AutomationSuggestionQualityDecisionsReadyDraft       = "ready_draft"
	AutomationSuggestionQualityDecisionsNeedsReviewDraft = "needs_review_draft"
	AutomationSuggestionQualityDecisionsLearningOnly     = "learning_only"
	AutomationSuggestionQualityDecisionsSuppressed       = "suppressed"
)

// LearningProposalFeedbackActions
const (
	LearningProposalFeedbackActionsAcceptedWithoutEdits = "accepted_without_edits"
	LearningProposalFeedbackActionsEditedThenAccepted   = "edited_then_accepted"
	LearningProposalFeedbackActionsRejected             = "rejected"
	LearningProposalFeedbackActionsEditedAfterApproval  = "edited_after_approval"
)

type LearningConfig struct {
	Enabled                           bool `json:"enabled"`
	ReviewRequired                    bool `json:"review_required"`
	SkillProposalThreshold            int  `json:"skill_proposal_threshold"`
	AutomationProposalThreshold       int  `json:"automation_proposal_threshold"`
	MaxDraftChars                     int  `json:"max_draft_chars"`
	HarnessEvolutionEnabled           bool `json:"harness_evolution_enabled"`
	HarnessEvolutionProposalThreshold int  `json:"harness_evolution_proposal_threshold"`
	HarnessEvolutionLookbackHours     int  `json:"harness_evolution_lookback_hours"`
}

func DefaultLearningConfig() LearningConfig {
	return LearningConfig{
		Enabled:                           true,
		ReviewRequired:                    true,
		SkillProposalThreshold:            2,
		AutomationProposalThreshold:       3,
		MaxDraftChars:                     4000,
		HarnessEvolutionProposalThreshold: 3,
		HarnessEvolutionLookbackHours:     24,
	}
}

type AutomationSuggestionIntent struct {
	Intent          string   `json:"intent"`
	TargetObject    string   `json:"target_object"`
	ExpectedOutcome string   `json:"expected_outcome"`
	CadenceHint     string   `json:"cadence_hint"`
	TriggerEvidence []string `json:"trigger_evidence"`
	Ambiguities     []string `json:"ambiguities"`
}

func DefaultAutomationSuggestionIntent() AutomationSuggestionIntent {
	return AutomationSuggestionIntent{
		Intent:          "custom_automation",
		TargetObject:    "unspecified",
		ExpectedOutcome: "unspecified",
		CadenceHint:     "daily",
	}
}

type AutomationSuggestionQualityDimension struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

type AutomationSuggestionQualityResult struct {
	Score          int                                    `json:"score"`
	Decision       string                                 `json:"decision"`
	Dimensions     []AutomationSuggestionQualityDimension `json:"dimensions"`
	BlockingIssues []string                               `json:"blocking_issues"`
	Warnings       []string                               `json:"warnings"`
}

func DefaultAutomationSuggestionQualityResult() AutomationSuggestionQualityResult {
	return AutomationSuggestionQualityResult{
		Decision: AutomationSuggestionQualityDecisionsLearningOnly,
	}
}

type LearningAutomationSuggestionPreview struct {
	WhySuggested           string   `json:"why_suggested"`
	OriginalPrompt         string   `json:"original_prompt"`
	RefinedPrompt          string   `json:"refined_prompt"`
	QualityScore           int      `json:"quality_score"`
	QualityDecision        string   `json:"quality_decision"`
	Warnings               []string `json:"warnings"`
	ExpectedOutputSections []string `json:"expected_output_sections"`
}

func DefaultLearningAutomationSuggestionPreview() LearningAutomationSuggestionPreview {
	return LearningAutomationSuggestionPreview{
		QualityDecision: AutomationSuggestionQualityDecisionsLearningOnly,
	}
}

type LearningProposalFeedbackEvent struct {
	Action             string    `json:"action"`
	ChangedFields      []string  `json:"changed_fields"`
	BeforeQualityScore *int      `json:"before_quality_score"`
	AfterQualityScore  *int      `json:"after_quality_score"`
	Summary            string    `json:"summary"`
	CreatedAtUtc       time.Time `json:"created_at_utc"`
}

func DefaultLearningProposalFeedbackEvent() LearningProposalFeedbackEvent {
	return LearningProposalFeedbackEvent{
		CreatedAtUtc: time.Now().UTC(),
	}
}

type LearningToolObservation struct {
	ToolName             string  `json:"tool_name"`
	SequenceIndex        int     `json:"sequence_index"`
	IsReadOnly           *bool   `json:"is_read_only"`
	IsMutating           *bool   `json:"is_mutating"`
	IsInteractive        *bool   `json:"is_interactive"`
	IsApprovalGated      *bool   `json:"is_approval_gated"`
	IsSandboxCapable     *bool   `json:"is_sandbox_capable"`
	ClassificationReason *string `json:"classification_reason"`
}

type ManagedLearningSkillMetadata struct {
	ManagedByLearning   bool      `json:"managed_by_learning"`
	CreatedByProposalId string    `json:"created_by_proposal_id"`
	OriginalDraftHash   *string   `json:"original_draft_hash"`
	ApprovedAtUtc       time.Time `json:"approved_at_utc"`
	SkillName           *string   `json:"skill_name"`
}

func DefaultManagedLearningSkillMetadata() ManagedLearningSkillMetadata {
	return ManagedLearningSkillMetadata{
		ManagedByLearning: true,
		ApprovedAtUtc:     time.Now().UTC(),
	}
}

type HarnessEvolutionProposal struct {
	Component                  string   `json:"component"`
	ChangeType                 *string  `json:"change_type"`
	FailureMode                string   `json:"failure_mode"`
	ProposedChange             string   `json:"proposed_change"`
	PredictedImprovement       *string  `json:"predicted_improvement"`
	InvariantsToPreserve       []string `json:"invariants_to_preserve"`
	FalsificationTests         []string `json:"falsification_tests"`
	EvaluationPlan             *string  `json:"evaluation_plan"`
	CanaryPlan                 *string  `json:"canary_plan"`
	RollbackPlan               *string  `json:"rollback_plan"`
	RelatedHarnessContractIds  []string `json:"related_harness_contract_ids"`
	RelatedEvidenceBundleIds   []string `json:"related_evidence_bundle_ids"`
	RelatedGovernanceLedgerIds []string `json:"related_governance_ledger_ids"`
	RelatedRegressionReportIds []string `json:"related_regression_report_ids"`
	SourceRuntimeEventIds      []string `json:"source_runtime_event_ids"`
	SourceSessionIds           []string `json:"source_session_ids"`
	RiskLevel                  string   `json:"risk_level"`
	Confidence                 float32  `json:"confidence"`
	ProposalFingerprint        *string  `json:"proposal_fingerprint"`
	ApplyMode                  string   `json:"apply_mode"`
	IsAutoApplicable           bool     `json:"is_auto_applicable"`
	RequiresRegression         bool     `json:"requires_regression"`
	RegressionCategories       []string `json:"regression_categories"`
}

func DefaultHarnessEvolutionProposal() HarnessEvolutionProposal {
	return HarnessEvolutionProposal{
		Component: HarnessEvolutionComponentsUnknown,
		RiskLevel: LearningProposalRiskLevelsMedium,
		ApplyMode: HarnessEvolutionApplyModesManualOnly,
	}
}

type HarnessEvolutionProposalCreateRequest struct {
	ActorId                    *string  `json:"actor_id"`
	Title                      *string  `json:"title"`
	Summary                    *string  `json:"summary"`
	Component                  *string  `json:"component"`
	ChangeType                 *string  `json:"change_type"`
	FailureMode                *string  `json:"failure_mode"`
	ProposedChange             *string  `json:"proposed_change"`
	PredictedImprovement       *string  `json:"predicted_improvement"`
	InvariantsToPreserve       []string `json:"invariants_to_preserve"`
	FalsificationTests         []string `json:"falsification_tests"`
	EvaluationPlan             *string  `json:"evaluation_plan"`
	CanaryPlan                 *string  `json:"canary_plan"`
	RollbackPlan               *string  `json:"rollback_plan"`
	RelatedHarnessContractIds  []string `json:"related_harness_contract_ids"`
	RelatedEvidenceBundleIds   []string `json:"related_evidence_bundle_ids"`
	RelatedGovernanceLedgerIds []string `json:"related_governance_ledger_ids"`
	RelatedRegressionReportIds []string `json:"related_regression_report_ids"`
	SourceRuntimeEventIds      []string `json:"source_runtime_event_ids"`
	SourceSessionIds           []string `json:"source_session_ids"`
	RiskLevel                  *string  `json:"risk_level"`
	Confidence                 *float32 `json:"confidence"`
	ApplyMode                  *string  `json:"apply_mode"`
	IsAutoApplicable           bool     `json:"is_auto_applicable"`
	RequiresRegression         *bool    `json:"requires_regression"`
	RegressionCategories       []string `json:"regression_categories"`
}

type HarnessEvolutionDetectionRequest struct {
	LookbackHours *int `json:"lookback_hours"`
	Threshold     *int `json:"threshold"`
	Limit         *int `json:"limit"`
}

type HarnessEvolutionDetectionResponse struct {
	Proposals              []LearningProposal `json:"proposals"`
	GroupsEvaluated        int                `json:"groups_evaluated"`
	GroupsMeetingThreshold int                `json:"groups_meeting_threshold"`
}

type LearningProposal struct {
	Id                          string                               `json:"id"`
	Kind                        string                               `json:"kind"`
	Status                      string                               `json:"status"`
	ActorId                     *string                              `json:"actor_id"`
	Title                       string                               `json:"title"`
	Summary                     string                               `json:"summary"`
	SkillName                   *string                              `json:"skill_name"`
	DraftContent                *string                              `json:"draft_content"`
	DraftContentHash            *string                              `json:"draft_content_hash"`
	DraftPreview                *string                              `json:"draft_preview"`
	ProfileUpdate               *UserProfile                         `json:"profile_update"`
	AppliedProfileBefore        *UserProfile                         `json:"applied_profile_before"`
	AutomationDraft             *AutomationDefinition                `json:"automation_draft"`
	AutomationIntent            *AutomationSuggestionIntent          `json:"automation_intent"`
	AutomationQuality           *AutomationSuggestionQualityResult   `json:"automation_quality"`
	AutomationSuggestionPreview *LearningAutomationSuggestionPreview `json:"automation_suggestion_preview"`
	AppliedAutomationId         *string                              `json:"applied_automation_id"`
	ManagedSkillPath            *string                              `json:"managed_skill_path"`
	ManagedSkillMetadata        *ManagedLearningSkillMetadata        `json:"managed_skill_metadata"`
	Metadata                    map[string]string                    `json:"metadata"`
	HarnessEvolution            *HarnessEvolutionProposal            `json:"harness_evolution"`
	SourceSessionIds            []string                             `json:"source_session_ids"`
	SourceTurnIds               []string                             `json:"source_turn_ids"`
	ToolNames                   []string                             `json:"tool_names"`
	ToolSequence                []string                             `json:"tool_sequence"`
	ToolObservations            []LearningToolObservation            `json:"tool_observations"`
	FeedbackEvents              []LearningProposalFeedbackEvent      `json:"feedback_events"`
	RepeatedCount               int                                  `json:"repeated_count"`
	ProposalFingerprint         *string                              `json:"proposal_fingerprint"`
	RiskLevel                   string                               `json:"risk_level"`
	Confidence                  float32                              `json:"confidence"`
	CreatedReason               *string                              `json:"created_reason"`
	ValidationStatus            string                               `json:"validation_status"`
	ValidationWarnings          []string                             `json:"validation_warnings"`
	ValidationErrors            []string                             `json:"validation_errors"`
	CreatedAtUtc                time.Time                            `json:"created_at_utc"`
	UpdatedAtUtc                time.Time                            `json:"updated_at_utc"`
	ReviewedAtUtc               *time.Time                           `json:"reviewed_at_utc"`
	ReviewNotes                 *string                              `json:"review_notes"`
	RolledBack                  bool                                 `json:"rolled_back"`
	RolledBackAtUtc             *time.Time                           `json:"rolled_back_at_utc"`
	RollbackReason              *string                              `json:"rollback_reason"`
}

func DefaultLearningProposal() LearningProposal {
	return LearningProposal{
		Kind:             LearningProposalKindSkillDraft,
		Status:           LearningProposalStatusPending,
		RiskLevel:        LearningProposalRiskLevelsMedium,
		ValidationStatus: LearningProposalValidationStatusesNotRun,
		CreatedAtUtc:     time.Now().UTC(),
		UpdatedAtUtc:     time.Now().UTC(),
	}
}
