package core

import "strings"

func DefaultDynamicTurnRoutingConfig() *DynamicTurnRoutingConfig {
	return &DynamicTurnRoutingConfig{
		Assets: DefaultDynamicTurnRoutingAssetsConfig(),
		Policy: DefaultDynamicTurnRoutingPolicyConfig(),
	}
}

type DynamicTurnRoutingConfig struct {
	Enabled    bool                            `json:"enabled"`
	BundlePath string                          `json:"bundle_path"`
	Assets     *DynamicTurnRoutingAssetsConfig `json:"assets"`
	Policy     *DynamicTurnRoutingPolicyConfig `json:"policy"`
}

func DefaultDynamicTurnRoutingAssetsConfig() *DynamicTurnRoutingAssetsConfig {
	return &DynamicTurnRoutingAssetsConfig{
		Dimensions: 384,
	}
}

type DynamicTurnRoutingAssetsConfig struct {
	ClassifierModelPath string `json:"classifier_model_path"`
	EmbeddingModelPath  string `json:"embedding_model_path"`
	TokenizerPath       string `json:"tokenizer_path"`
	ManifestPath        string `json:"manifest_path"`
	RuntimeConfigPath   string `json:"runtime_config_path"`
	Dimensions          int    `json:"dimensions"`
}

func DefaultDynamicTurnRoutingPolicyConfig() *DynamicTurnRoutingPolicyConfig {
	return &DynamicTurnRoutingPolicyConfig{
		Tiers:                              DefaultDynamicTurnRoutingTierMap(),
		EnableStickyTier:                   true,
		EnableMarginUpgrade:                true,
		EnableR1Rescue:                     true,
		EnableUnderRoutingSafety:           true,
		MarginUpgradeThreshold:             0.15,
		R1RescueThreshold:                  0.20,
		UnderRoutingSafetyThreshold:        0.45,
		DeepConversationTurnIndexThreshold: 4,
	}
}

type DynamicTurnRoutingPolicyConfig struct {
	Tiers                              *DynamicTurnRoutingTierMap `json:"tiers"`
	EnableDiagnostics                  bool                       `json:"enable_diagnostics"`
	EnableStickyTier                   bool                       `json:"enable_sticky_tier"`
	EnableMarginUpgrade                bool                       `json:"enable_margin_upgrade"`
	EnableR1Rescue                     bool                       `json:"enable_r1_rescue"`
	EnableUnderRoutingSafety           bool                       `json:"enable_under_routing_safety"`
	MarginUpgradeThreshold             float32                    `json:"margin_upgrade_threshold"`
	R1RescueThreshold                  float32                    `json:"r1_rescue_threshold"`
	UnderRoutingSafetyThreshold        float32                    `json:"under_routing_safety_threshold"`
	DeepConversationTurnIndexThreshold int                        `json:"deep_conversation_turn_index_threshold"`
}

func DefaultDynamicTurnRoutingTierMap() *DynamicTurnRoutingTierMap {
	return &DynamicTurnRoutingTierMap{
		T0: DefaultDynamicTurnRoutingTierTarget(),
		T1: DefaultDynamicTurnRoutingTierTarget(),
		T2: DefaultDynamicTurnRoutingTierTarget(),
		T3: DefaultDynamicTurnRoutingTierTarget(),
	}
}

type DynamicTurnRoutingTierMap struct {
	T0 *DynamicTurnRoutingTierTarget `json:"t0"`
	T1 *DynamicTurnRoutingTierTarget `json:"t1"`
	T2 *DynamicTurnRoutingTierTarget `json:"t2"`
	T3 *DynamicTurnRoutingTierTarget `json:"t3"`
}

func DefaultDynamicTurnRoutingTierTarget() *DynamicTurnRoutingTierTarget {
	return &DynamicTurnRoutingTierTarget{
		AllowedTools:              []string{},
		PreferredTags:             []string{},
		CacheContinuitySafeguards: DefaultCacheContinuitySafeguardsConfig(),
		PromptMode:                "full",
	}
}

type DynamicTurnRoutingTierTarget struct {
	ModelProfileId               string                           `json:"model_profile_id"`
	DirectModelFallbackProfileId string                           `json:"direct_model_fallback_profile_id"`
	AllowedTools                 []string                         `json:"allowed_tools"`
	PreferredTags                []string                         `json:"preferred_tags"`
	ReasoningLevel               string                           `json:"reasoning_level"`
	ResponsePolicy               string                           `json:"response_policy"`
	ImageCapableModelProfileId   string                           `json:"image_capable_model_profile_id"`
	CacheContinuitySafeguards    *CacheContinuitySafeguardsConfig `json:"cache_continuity_safeguards"`
	PromptMode                   string                           `json:"prompt_mode"`
	DisableTools                 bool                             `json:"disable_tools"`
}

func DefaultCacheContinuitySafeguardsConfig() *CacheContinuitySafeguardsConfig {
	return &CacheContinuitySafeguardsConfig{
		MaxConversationTurns: 64,
		ResetOnProfileSwitch: true,
	}
}

type CacheContinuitySafeguardsConfig struct {
	Enabled              bool `json:"enabled"`
	MaxConversationTurns int  `json:"max_conversation_turns"`
	ResetOnProfileSwitch bool `json:"reset_on_profile_switch"`
}

type DynamicTurnRoutingTierTargets struct{}

func (DynamicTurnRoutingTierTargets) HasAnyConfigured(tiers *DynamicTurnRoutingTierMap) bool {
	if tiers == nil {
		return false
	}
	helper := DynamicTurnRoutingTierTargets{}
	return helper.IsConfigured(tiers.T0) ||
		helper.IsConfigured(tiers.T1) ||
		helper.IsConfigured(tiers.T2) ||
		helper.IsConfigured(tiers.T3)
}

func (DynamicTurnRoutingTierTargets) IsConfigured(tier *DynamicTurnRoutingTierTarget) bool {
	if tier == nil {
		return false
	}

	defaultSafeguards := DefaultCacheContinuitySafeguardsConfig()
	defaultTier := DefaultDynamicTurnRoutingTierTarget()

	hasSafeguards := tier.CacheContinuitySafeguards != nil
	safeguardsChanged := false
	if hasSafeguards {
		safeguardsChanged = tier.CacheContinuitySafeguards.Enabled != defaultSafeguards.Enabled ||
			tier.CacheContinuitySafeguards.MaxConversationTurns != defaultSafeguards.MaxConversationTurns ||
			tier.CacheContinuitySafeguards.ResetOnProfileSwitch != defaultSafeguards.ResetOnProfileSwitch
	}

	return strings.TrimSpace(tier.ModelProfileId) != "" ||
		strings.TrimSpace(tier.DirectModelFallbackProfileId) != "" ||
		len(tier.AllowedTools) > 0 ||
		len(tier.PreferredTags) > 0 ||
		strings.TrimSpace(tier.ReasoningLevel) != "" ||
		strings.TrimSpace(tier.ResponsePolicy) != "" ||
		strings.TrimSpace(tier.ImageCapableModelProfileId) != "" ||
		safeguardsChanged ||
		!strings.EqualFold(tier.PromptMode, defaultTier.PromptMode) ||
		tier.DisableTools
}

func DefaultResolvedDynamicTurnRoutingConfig() *ResolvedDynamicTurnRoutingConfig {
	return &ResolvedDynamicTurnRoutingConfig{
		Source: "disabled",
		Assets: DefaultResolvedDynamicTurnRoutingAssets(),
		Policy: DefaultDynamicTurnRoutingPolicyConfig(),
		Tiers:  DefaultDynamicTurnRoutingTierMap(),
	}
}

type ResolvedDynamicTurnRoutingConfig struct {
	Enabled bool                              `json:"enabled"`
	Source  string                            `json:"source"`
	Assets  *ResolvedDynamicTurnRoutingAssets `json:"assets"`
	Policy  *DynamicTurnRoutingPolicyConfig   `json:"policy"`
	Tiers   *DynamicTurnRoutingTierMap        `json:"tiers"`
}

func DefaultResolvedDynamicTurnRoutingAssets() *ResolvedDynamicTurnRoutingAssets {
	return &ResolvedDynamicTurnRoutingAssets{
		EmbeddingDimensions: 384,
	}
}

type ResolvedDynamicTurnRoutingAssets struct {
	ClassifierModelPath string `json:"classifier_model_path"`
	EmbeddingModelPath  string `json:"embedding_model_path"`
	TokenizerPath       string `json:"tokenizer_path"`
	ManifestPath        string `json:"manifest_path"`
	RuntimeConfigPath   string `json:"runtime_config_path"`
	EmbeddingDimensions int    `json:"embedding_dimensions"`
}
