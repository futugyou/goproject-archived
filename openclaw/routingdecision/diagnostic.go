package routingdecision

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/util"
)

// DecisionRoutingDiagnostic 决策元数据结构（不包含对话文本、工具参数或凭据）
type DecisionRoutingDiagnostic struct {
	DecisionId               string             `json:"decisionId"`
	TimestampUtc             time.Time          `json:"timestampUtc"`
	RubricVersion            string             `json:"rubricVersion"`
	Provider                 string             `json:"provider"`
	Metadata                 *DecisionMetadata  `json:"metadata,omitempty"`
	Mode                     string             `json:"mode"`
	SessionId                string             `json:"sessionId"`
	BaselineTier             string             `json:"baselineTier"`
	BaselineProfileId        *string            `json:"baselineProfileId,omitempty"`
	ProposedTier             *string            `json:"proposedTier,omitempty"`
	ProposedProfileId        *string            `json:"proposedProfileId,omitempty"`
	AppliedTier              string             `json:"appliedTier"`
	AppliedProfileId         *string            `json:"appliedProfileId,omitempty"`
	Reason                   string             `json:"reason"`
	Model                    *string            `json:"model,omitempty"`
	RawChoice                *string            `json:"rawChoice,omitempty"`
	Probabilities            map[string]float64 `json:"probabilities,omitempty"`
	Confidence               *float64           `json:"confidence,omitempty"`
	HighRiskProbability      *float64           `json:"highRiskProbability,omitempty"`
	RequiresToolsProbability *float64           `json:"requiresToolsProbability,omitempty"`
	ContextTruncated         bool               `json:"contextTruncated"`
	LatencyMs                int64              `json:"latencyMs"`
	InputTokens              *int64             `json:"inputTokens,omitempty"`
	OutputTokens             *int64             `json:"outputTokens,omitempty"`
	EstimatedCostUsd         *float64           `json:"estimatedCostUsd,omitempty"`
}

func NewDecisionRoutingDiagnostic(provider, mode, sessionId, baselineTier, appliedTier, reason string) *DecisionRoutingDiagnostic {
	return &DecisionRoutingDiagnostic{
		DecisionId:    util.CleanUUID(),
		TimestampUtc:  time.Now().UTC(),
		RubricVersion: "openclaw-tiers-v1",
		Provider:      provider,
		Mode:          mode,
		SessionId:     sessionId,
		BaselineTier:  baselineTier,
		AppliedTier:   appliedTier,
		Reason:        reason,
	}
}

type IDecisionRoutingObserver interface {
	Record(diagnostic *DecisionRoutingDiagnostic)
}

type JsonlDecisionRoutingObserver struct {
	path   string
	logger *slog.Logger
	mu     sync.Mutex
}

func NewJsonlDecisionRoutingObserver(path string, logger *slog.Logger) *JsonlDecisionRoutingObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &JsonlDecisionRoutingObserver{
		path:   path,
		logger: logger,
	}
}

func (o *JsonlDecisionRoutingObserver) Record(diagnostic *DecisionRoutingDiagnostic) {
	if diagnostic == nil {
		return
	}

	o.logger.Info(
		"Decision routing",
		"provider", diagnostic.Provider,
		"mode", diagnostic.Mode,
		"baselineTier", diagnostic.BaselineTier,
		"proposedTier", diagnostic.ProposedTier,
		"appliedTier", diagnostic.AppliedTier,
		"reason", diagnostic.Reason,
		"latencyMs", diagnostic.LatencyMs,
		"inputTokens", diagnostic.InputTokens,
		"estimatedCostUsd", diagnostic.EstimatedCostUsd,
		"decisionId", diagnostic.DecisionId,
	)

	if strings.TrimSpace(o.path) == "" {
		return
	}

	data, err := json.Marshal(diagnostic)
	if err != nil {
		o.logger.Error("Failed to marshal decision routing diagnostic", "error", err)
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	dir := filepath.Dir(o.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		o.logger.Warn(fmt.Sprintf("Unable to append decision routing diagnostics (%T).", err))
		return
	}

	file, err := os.OpenFile(o.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		o.logger.Warn(fmt.Sprintf("Unable to append decision routing diagnostics (%T).", err))
		return
	}
	defer file.Close()

	data = append(data, '\n')
	if _, err := file.Write(data); err != nil {
		o.logger.Warn(fmt.Sprintf("Unable to append decision routing diagnostics (%T).", err))
	}
}
