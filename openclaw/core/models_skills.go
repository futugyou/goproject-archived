package core

import "encoding/json"

type SkillProjectionDiscovery struct {
	Status     string   `json:"status"`
	IndexCount int      `json:"index_count"`
	BoundCount int      `json:"bound_count"`
	IndexPaths []string `json:"index_paths"`
	Message    string   `json:"message"`
}

type SkillProjectionContractSet struct {
	ProducerName     string                   `json:"producer_name"`
	ProducerPriority int                      `json:"producer_priority"`
	RootPath         string                   `json:"root_path"`
	Index            *ProjectionContractIndex `json:"index"`
}

type SkillProjectionResolution struct {
	SkillName          string              `json:"skill_name"`
	HasContracts       bool                `json:"has_contracts"`
	IsBlocked          bool                `json:"is_blocked"`
	BlockReason        string              `json:"block_reason"`
	SelectedTopic      string              `json:"selected_topic"`
	SelectedTargetView string              `json:"selected_target_view"`
	ProjectionFilePath string              `json:"projection_file_path"`
	Projection         *ProjectionDocument `json:"projection"`
}

type ProjectionContractIndex struct {
	ProducerSkill          string                       `json:"producer_skill"`
	ProducerPriority       int                          `json:"producer_priority"`
	DefaultSelectionPolicy *ProjectionSelectionPolicy   `json:"default_selection_policy"`
	TopicScoring           *ProjectionTopicScoring      `json:"topic_scoring"`
	TargetViewScoring      *ProjectionTargetViewScoring `json:"target_view_scoring"`
	Topics                 []ProjectionTopicRecord      `json:"topics"`
}

type ProjectionSelectionPolicy struct {
	PreferReadyOnly           bool     `json:"prefer_ready_only"`
	BlockOnOpenQuestions      bool     `json:"block_on_open_questions"`
	FallbackOrderByTargetView []string `json:"fallback_order_by_target_view"`
}

type ProjectionTopicScoring struct {
	ClarifyWhenScoreGapBelow int                        `json:"clarify_when_score_gap_below"`
	ScoreDimensions          []ProjectionScoreDimension `json:"score_dimensions"`
	Topics                   []ProjectionTopicSignals   `json:"topics"`
}

func DefaultProjectionTopicScoring() *ProjectionTopicScoring {
	return &ProjectionTopicScoring{
		ClarifyWhenScoreGapBelow: 2,
	}
}

type ProjectionTargetViewScoring struct {
	ClarifyWhenScoreGapBelow           int                           `json:"clarify_when_score_gap_below"`
	PreferExplicitUserArtifactRequests bool                          `json:"prefer_explicit_user_artifact_requests"`
	ScoreDimensions                    []ProjectionScoreDimension    `json:"score_dimensions"`
	Views                              []ProjectionViewSignals       `json:"views"`
	WithinTopicOverrides               []ProjectionTopicViewOverride `json:"within_topic_overrides"`
}

func DefaultProjectionTargetViewScoring() *ProjectionTargetViewScoring {
	return &ProjectionTargetViewScoring{
		ClarifyWhenScoreGapBelow: 2,
	}
}

type ProjectionScoreDimension struct {
	Dimension string `json:"dimension"`
	Score     int    `json:"score"`
}

type ProjectionTopicSignals struct {
	DomainSlug                      string   `json:"domain_slug"`
	PrimaryIntentSignals            []string `json:"primary_intent_signals"`
	SupportingSignals               []string `json:"supporting_signals"`
	ExplicitArtifactSignals         []string `json:"explicit_artifact_signals"`
	DemoteWhenCompetingTopicSignals []string `json:"demote_when_competing_topic_signals"`
}

type ProjectionViewSignals struct {
	TargetView                     string   `json:"target_view"`
	ExplicitOutputSignals          []string `json:"explicit_output_signals"`
	StrongSignals                  []string `json:"strong_signals"`
	SupportingSignals              []string `json:"supporting_signals"`
	DemoteWhenCompetingViewSignals []string `json:"demote_when_competing_view_signals"`
}

type ProjectionTopicViewOverride struct {
	DomainSlug string                     `json:"domain_slug"`
	Bonuses    []ProjectionTopicViewBonus `json:"bonuses"`
}

func DefaultProjectionTopicViewOverride() *ProjectionTopicViewOverride {
	return &ProjectionTopicViewOverride{
		Bonuses: make([]ProjectionTopicViewBonus, 0),
	}
}

type ProjectionTopicViewBonus struct {
	TargetView         string   `json:"target_view"`
	WhenRequestSignals []string `json:"when_request_signals"`
	Score              int      `json:"score"`
}

type ProjectionTopicRecord struct {
	DomainSlug        string                 `json:"domain_slug"`
	DefaultTargetView string                 `json:"default_target_view"`
	Views             []ProjectionViewRecord `json:"views"`
}

type ProjectionViewRecord struct {
	TargetView string `json:"target_view"`
	Status     string `json:"status"`
	Path       string `json:"path"`
}

type ProjectionDocument struct {
	MappingPolicy     *ProjectionMappingPolicy     `json:"mapping_policy"`
	PromptProjection  *ProjectionPromptPayload     `json:"prompt_projection"`
	DeliveryArtifacts []ProjectionDeliveryArtifact `json:"delivery_artifacts"`
	DroppedItems      []string                     `json:"dropped_items"`
	OpenQuestions     []string                     `json:"open_questions"`
}

type ProjectionMappingPolicy struct {
	UnresolvedItemPolicy   string `json:"unresolved_item_policy"`
	PromptAssumptionPolicy string `json:"prompt_assumption_policy"`
}

type ProjectionPromptPayload struct {
	AllowedTerms           []string `json:"allowed_terms"`
	ForbiddenAssumptions   []string `json:"forbidden_assumptions"`
	RequiredClarifications []string `json:"required_clarifications"`
	ReasoningPaths         []string `json:"reasoning_paths"`
	SourceDigest           []string `json:"source_digest"`
}

type ProjectionDeliveryArtifact struct {
	ArtifactName string `json:"artifact_name"`
	ArtifactType string `json:"artifact_type"`
	Path         string `json:"path"`
	Status       string `json:"status"`
}

type SkillArtifactContract struct {
	SchemaVersion int                          `json:"schema_version"`
	Stages        []SkillArtifactStageContract `json:"stages"`
}

func DefaultSkillArtifactContract() *SkillArtifactContract {
	return &SkillArtifactContract{
		SchemaVersion: 1,
	}
}

type SkillArtifactStageContract struct {
	Name      string                          `json:"name"`
	Label     string                          `json:"label"`
	Gate      *SkillArtifactStageGateContract `json:"gate"`
	Artifacts []SkillArtifactTypeContract     `json:"artifacts"`
}

type SkillArtifactStageGateContract struct {
	RequiresStage string `json:"requires_stage"`
}

type SkillArtifactTypeContract struct {
	Type     string `json:"type"`
	Label    string `json:"label"`
	Display  string `json:"display"`
	Terminal bool   `json:"terminal"`
}

const (
	SkillArtifactKindFile = "file"
	SkillArtifactKindData = "data"

	SkillArtifactTypeTemplatePackage = "template_package"
	SkillArtifactTypeSkillPackage    = "skill_package"
	SkillArtifactTypeOntology        = "ontology"
	SkillArtifactTypeAnalysis        = "analysis"
	SkillArtifactTypePlan            = "plan"
	SkillArtifactTypeProgress        = "progress"
	SkillArtifactTypeGeneric         = "generic"

	SkillArtifactDisplayHintTree     = "tree"
	SkillArtifactDisplayHintTable    = "table"
	SkillArtifactDisplayHintCode     = "code"
	SkillArtifactDisplayHintBadge    = "badge"
	SkillArtifactDisplayHintProgress = "progress"
	SkillArtifactDisplayHintText     = "text"
)

type SkillArtifact struct {
	Kind          string           `json:"kind"`
	ArtifactType  string           `json:"artifact_type"`
	Label         string           `json:"label"`
	SkillName     string           `json:"skill_name"`
	Stage         string           `json:"stage"`
	IsTerminal    bool             `json:"is_terminal"`
	FileUrl       string           `json:"file_url"`
	FileName      string           `json:"file_name"`
	MimeType      string           `json:"mime_type"`
	FileSizeBytes int64            `json:"file_size_bytes"`
	Data          *json.RawMessage `json:"data,omitempty"`
	DisplayHint   string           `json:"display_hint"`
}

func DefaultSkillArtifact() *SkillArtifact {
	return &SkillArtifact{
		ArtifactType: SkillArtifactTypeGeneric,
	}
}
