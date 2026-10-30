package cli

import (
	"strings"

	"github.com/futugyou/openclaw/core"
)

type MetaRunProposalAcceptanceGateCheck struct {
	Id     string
	Passed bool
}

type MetaRunProposalAcceptanceGateResult struct {
	ProfileId string
	Checks    []MetaRunProposalAcceptanceGateCheck
}

func (m *MetaRunProposalAcceptanceGateResult) Passed() bool {
	for _, v := range m.Checks {
		if !v.Passed {
			return false
		}
	}

	return true
}

func (m *MetaRunProposalAcceptanceGateResult) FailedChecks() []string {
	result := []string{}
	for _, v := range m.Checks {
		if !v.Passed {
			result = append(result, v.Id)
		}
	}

	return result
}

const QualityGateProfileId = "opensquilla-authoring-v1"

func qualityGateAddCheck(checks []MetaRunProposalAcceptanceGateCheck, id string, passed bool) []MetaRunProposalAcceptanceGateCheck {
	return append(checks, MetaRunProposalAcceptanceGateCheck{Id: id, Passed: passed})
}

func QualityGateEvaluate(proposal core.MetaRunDerivedProposalSummary, session core.Session) MetaRunProposalAcceptanceGateResult {
	checks := []MetaRunProposalAcceptanceGateCheck{}
	var run *core.SessionMetaRunRecord
	for _, v := range session.MetaRunHistory {
		if v.RunId == proposal.Id {
			run = &v
			break
		}
	}

	checks = qualityGateAddCheck(checks, "runtime.run_present", run != nil)
	if run == nil {
		return MetaRunProposalAcceptanceGateResult{ProfileId: QualityGateProfileId, Checks: checks}
	}

	checks = qualityGateAddCheck(
		checks,
		"runtime.identity_present",
		run.RunId != "" && run.SkillName != "")

	checks = qualityGateAddCheck(
		checks,
		"trigger.proposal_status_matches_run",
		run.Status == proposal.Status)

	if len(run.StepResults) > 0 {
		ch := true
		for _, step := range run.StepResults {
			if step.Id == "" || step.Kind == "" || step.Status == "" {
				ch = false
				break
			}
		}
		checks = qualityGateAddCheck(
			checks,
			"runtime.step_shape_valid",
			!ch)

		hasDuplicateStepIds := false
		group := map[string]struct{}{}
		for _, step := range run.StepResults {
			if _, ok := group[step.Id]; ok {
				hasDuplicateStepIds = true
				break
			}
			group[step.Id] = struct{}{}
		}

		checks = qualityGateAddCheck(checks, "runtime.step_id_unique", !hasDuplicateStepIds)
	}

	if run.Status == "paused" {
		var checkpoint = session.MetaExecutionCheckpoint
		checks = qualityGateAddCheck(
			checks,
			"safety.paused_checkpoint_consistent",
			checkpoint != nil && checkpoint.SkillName == run.SkillName && strings.TrimSpace(checkpoint.PendingStepId) != "")
	}

	if run.Status == "failed" {
		var hasFailureEvidence = run.ErrorCode != "" || run.Error != ""
		if !hasFailureEvidence {
			for _, step := range run.StepResults {
				if step.FailureCode != "" {
					hasFailureEvidence = true
					break
				}
			}
		}
		checks = qualityGateAddCheck(checks, "runtime.failed_evidence_present", hasFailureEvidence)
	}

	return MetaRunProposalAcceptanceGateResult{ProfileId: QualityGateProfileId, Checks: checks}
}
