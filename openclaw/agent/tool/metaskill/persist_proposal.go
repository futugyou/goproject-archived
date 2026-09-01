package metaskill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MetaSkillPersistProposalTool struct {
}

func (e *MetaSkillPersistProposalTool) Name() string {
	return "meta_skill_persist_proposal"
}

func (e *MetaSkillPersistProposalTool) Description() string {
	return "Write proposal candidate to ~/.opensquilla/proposals/<id>/ and return JSON metadata."
}

func (e *MetaSkillPersistProposalTool) ParameterSchema() string {
	return `
	{
	"type": "object",
	"properties": {
		"skill_md": {
			"type": "string"
		},
		"lint_result": {
			"type": "string"
		},
		"smoke_result": {
			"type": "string"
		},
		"creator_mode": {
			"type": "string"
		},
		"acceptance_result": {
			"type": "string"
		},
		"runtime_e2e_result": {
			"type": "string"
		},
		"collision_result": {
			"type": "string"
		},
		"risk_result": {
			"type": "string"
		},
		"home": {
			"type": "string"
		}
	},
	"required": ["skill_md", "lint_result", "smoke_result"]
}
"`
}

type PersistProposalModel struct {
	SkillMd          string `json:"skill_md"`
	LintResult       string `json:"lint_result"`
	SmokeResult      string `json:"smoke_result"`
	CreatorMode      string `json:"creator_mode"`
	AcceptanceResult string `json:"acceptance_result"`
	RuntimeE2EResult string `json:"runtime_e2e_result"`
	CollisionResult  string `json:"collision_result"`
	RiskResult       string `json:"risk_result"`
	Home             string `json:"home"`
}

type LintPayload struct {
	Passed bool `json:"passed"`
}

type SmokePayload struct {
	G3 struct {
		Passed bool `json:"passed"`
	} `json:"G3"`
	G4 struct {
		Passed bool `json:"passed"`
	} `json:"G4"`
}

type RuntimeE2EPayload struct {
	Passed bool `json:"passed"`
}

type GatesPayload struct {
	ProposalID         string          `json:"proposal_id"`
	AutoEnableEligible bool            `json:"auto_enable_eligible"`
	CreatorMode        string          `json:"creator_mode"`
	Lint               json.RawMessage `json:"lint"`
	Smoke              json.RawMessage `json:"smoke"`
	AcceptanceCompare  json.RawMessage `json:"acceptance_compare"`
	RuntimeE2E         json.RawMessage `json:"runtime_e2e"`
	Collision          json.RawMessage `json:"collision"`
	Risk               json.RawMessage `json:"risk"`
}

func toJSONRawMessage(val string) json.RawMessage {
	if strings.TrimSpace(val) == "" {
		res, _ := json.Marshal("")
		return res
	}

	if json.Valid([]byte(val)) {
		return json.RawMessage(val)
	}

	res, _ := json.Marshal(val)
	return res
}

func (a *MetaSkillPersistProposalTool) Execute(ctx context.Context, argumentsJson string) string {
	select {
	case <-ctx.Done():
		return ctx.Err().Error()
	default:
	}

	if argumentsJson == "" {
		argumentsJson = "{}"
	}

	var doc PersistProposalModel

	if err := json.Unmarshal([]byte(argumentsJson), &doc); err != nil {
		return err.Error()
	}

	if doc.SkillMd == "" || doc.LintResult == "" || doc.SmokeResult == "" {
		return SerializeError("invalid_arguments", "'skill_md', 'lint_result', and 'smoke_result' are required.")
	}
	homeDir, err := resolveHomeDirectory(doc.Home)
	if err != nil {
		return err.Error()
	}

	proposalID := buildProposalID(doc.SkillMd)
	proposalDir := filepath.Join(homeDir, "proposals", proposalID)

	if err := os.MkdirAll(proposalDir, 0755); err != nil {
		return fmt.Sprintf("failed to create directory: %s", err.Error())
	}

	if err := os.WriteFile(filepath.Join(proposalDir, "SKILL.md"), []byte(doc.SkillMd), 0644); err != nil {
		return fmt.Sprintf("failed to write SKILL.md: %s", err.Error())
	}

	var lint LintPayload
	_ = unmarshalJSONString(doc.LintResult, &lint)

	var smoke SmokePayload
	_ = unmarshalJSONString(doc.SmokeResult, &smoke)

	var runtime RuntimeE2EPayload
	_ = unmarshalJSONString(doc.RuntimeE2EResult, &runtime)

	autoEnableEligible := evaluateAutoEnableEligible(doc.CreatorMode, lint, smoke, runtime)

	gatesJSON, err := buildGatesPayload(
		doc.LintResult,
		doc.SmokeResult,
		doc.CreatorMode,
		doc.AcceptanceResult,
		doc.RuntimeE2EResult,
		doc.CollisionResult,
		doc.RiskResult,
		proposalID,
		autoEnableEligible,
	)
	if err != nil {
		return fmt.Sprintf("failed to build gates payload: %s", err.Error())
	}

	if err := os.WriteFile(filepath.Join(proposalDir, "gates.json"), gatesJSON, 0644); err != nil {
		return fmt.Sprintf("failed to write gates.json: %s", err.Error())
	}

	return SerializePersistResult(proposalID, proposalDir, autoEnableEligible)
}

func resolveHomeDirectory(home string) (string, error) {
	if strings.TrimSpace(home) != "" {
		expanded := os.ExpandEnv(home)
		return filepath.Abs(expanded)
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home dir: %w", err)
	}
	return filepath.Join(userHome, ".opensquilla"), nil
}

func buildProposalID(skillMd string) string {
	hash := sha256.Sum256([]byte(skillMd))
	return "proposal-" + hex.EncodeToString(hash[:8])
}

func unmarshalJSONString(raw string, target interface{}) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("empty input")
	}
	return json.Unmarshal([]byte(raw), target)
}

func evaluateAutoEnableEligible(creatorMode string, lint LintPayload, smoke SmokePayload, runtime RuntimeE2EPayload) bool {
	lintPassed := lint.Passed
	smokePassed := smoke.G3.Passed && smoke.G4.Passed

	if !lintPassed || !smokePassed {
		return false
	}

	if creatorMode == "FULL_GATED" {
		return runtime.Passed
	}

	return true
}

func buildGatesPayload(
	lintResult, smokeResult, creatorMode, acceptanceResult, runtimeE2EResult, collisionResult, riskResult, proposalID string,
	autoEnableEligible bool,
) ([]byte, error) {

	payload := GatesPayload{
		ProposalID:         proposalID,
		AutoEnableEligible: autoEnableEligible,
		CreatorMode:        creatorMode,
		Lint:               toJSONRawMessage(lintResult),
		Smoke:              toJSONRawMessage(smokeResult),
		AcceptanceCompare:  toJSONRawMessage(acceptanceResult),
		RuntimeE2E:         toJSONRawMessage(runtimeE2EResult),
		Collision:          toJSONRawMessage(collisionResult),
		Risk:               toJSONRawMessage(riskResult),
	}

	return json.MarshalIndent(payload, "", "  ")
}
