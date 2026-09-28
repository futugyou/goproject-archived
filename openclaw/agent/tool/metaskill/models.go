package metaskill

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
)

type CreatorToolError struct {
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

type CreatorLintResult struct {
	Status      string   `json:"status"`
	Passed      bool     `json:"passed"`
	FailedGates []string `json:"failedGates"`
	Summary     string   `json:"summary"`
}

type CreatorSmokeResult struct {
	G3       CreatorSmokeGateResult `json:"G3"`
	G4       CreatorSmokeGateResult `json:"G4"`
	Degraded bool                   `json:"degraded"`
	Summary  string                 `json:"summary,omitempty"`
}

type CreatorSmokeGateResult struct {
	Passed          bool   `json:"passed"`
	PositiveFixture string `json:"positive_fixture,omitempty"`
	NegativeFixture string `json:"negative_fixture,omitempty"`
	Classifier      string `json:"classifier,omitempty"`
	Degraded        bool   `json:"degraded"`
}

type CreatorRuntimeE2EResult struct {
	Status        string                  `json:"status"`
	Passed        bool                    `json:"passed"`
	Winner        string                  `json:"winner"`
	BaselineModel string                  `json:"baseline_model,omitempty"`
	Reason        string                  `json:"reason,omitempty"`
	Cases         []CreatorRuntimeE2ECase `json:"cases"`
}

type CreatorRuntimeE2ECase struct {
	Prompt     string `json:"prompt"`
	Winner     string `json:"winner"`
	Regression string `json:"regression"`
	Reason     string `json:"reason"`
}

type CreatorPersistResult struct {
	Status             string `json:"status"`
	ProposalIDSnake    string `json:"proposal_id,omitempty"`
	ProposalID         string `json:"proposalId"`
	Path               string `json:"path"`
	AutoEnableEligible bool   `json:"auto_enable_eligible"`
	AutoEnabled        bool   `json:"auto_enabled"`
}

var SupportedPatterns = map[string]struct{}{
	"p1_sequential": {}, "p2_fan_out_merge": {}, "p3_condition_gated": {},
}

func IsSupportedPattern(patternId string) bool {
	_, ok := SupportedPatterns[patternId]
	return ok
}

func ResolveCreatorStepKind(skillName string) string {
	if strings.EqualFold(skillName, "summarize") {
		return "llm_chat"
	}
	if strings.EqualFold(skillName, "history-explorer") {
		return "llm_chat"
	}

	return "agent"
}

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 256))
	},
}

func SerializeError(errorCode, message string) (string, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	res := CreatorToolError{
		Status:    "error",
		ErrorCode: errorCode,
		Message:   message,
	}

	if err := json.NewEncoder(buf).Encode(res); err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(buf.Bytes())), nil
}

func SerializeLintResult(passed bool, failedGates []string, summary string) (string, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if failedGates == nil {
		failedGates = []string{}
	}

	res := CreatorLintResult{
		Status:      "ok",
		Passed:      passed,
		FailedGates: failedGates,
		Summary:     summary,
	}

	if err := json.NewEncoder(buf).Encode(res); err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(buf.Bytes())), nil
}

func SerializePersistResult(proposalID, path string, autoEnableEligible bool) (string, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	res := CreatorPersistResult{
		Status:             "ok",
		ProposalIDSnake:    proposalID,
		ProposalID:         proposalID,
		Path:               path,
		AutoEnableEligible: autoEnableEligible,
		AutoEnabled:        false,
	}

	if err := json.NewEncoder(buf).Encode(res); err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(buf.Bytes())), nil
}
