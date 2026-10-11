package primitives

import (
	"fmt"
	"strings"

	"github.com/futugyou/openclaw/util"
)

type ScenarioGateIssue struct {
	ScenarioId string
	IsError    bool
	Message    string
}

type ScenarioGateResult struct {
	Issues []ScenarioGateIssue
}

func (s *ScenarioGateResult) Passed() bool {
	for _, v := range s.Issues {
		if v.IsError {
			return false
		}
	}

	return true
}

var ApprovalOrSafetyOracleTypes = map[string]struct{}{
	ScenarioOracleTypesApprovalRequired:    {},
	ScenarioOracleTypesApprovalNotRequired: {},
	ScenarioOracleTypesNoUnsafeTool:        {},
}

func qualityGateError(scenario AgentScenario, message string) ScenarioGateIssue {
	scenarioid := scenario.Id
	if scenarioid == "" {
		scenarioid = "<missing>"
	}

	return ScenarioGateIssue{
		ScenarioId: scenarioid,
		Message:    message,
	}
}

func hasExplicitExpectedBehavior(expected ScenarioExpected) bool {
	return len(expected.MustCallTools) > 0 ||
		len(expected.MustNotCallTools) > 0 ||
		len(expected.FinalAnswerContains) > 0 ||
		len(expected.FinalAnswerMustNotContain) > 0 ||
		expected.MaxToolCalls != nil ||
		expected.RequiresApproval != nil ||
		len(expected.ExpectedState) > 0 ||
		len(expected.ExpectedEvents) > 0 ||
		len(expected.ForbiddenEvents) > 0
}

func isToolUseScenario(scenario AgentScenario) bool {
	return hasToken(scenario, "tool-use") || hasToken(scenario, "tool") || hasToken(scenario, "tools")
}

func isApprovalOrSafetyScenario(scenario AgentScenario) bool {
	return scenario.Expected.RequiresApproval != nil ||
		hasToken(scenario, "approval") ||
		hasToken(scenario, "safety") ||
		hasToken(scenario, "security")
}

func hasToken(scenario AgentScenario, token string) bool {
	if strings.EqualFold(scenario.Type, token) {
		return true
	}

	for _, tag := range scenario.Tags {
		if strings.EqualFold(tag, token) {
			return true
		}
	}

	return false
}

func hasApprovalOrSafetyOracle(scenario AgentScenario) bool {
	for _, oracle := range scenario.Oracles {
		if _, ok := ApprovalOrSafetyOracleTypes[oracle.Type]; ok {
			return true
		}
	}
	return false
}

func ScenarioQualityEvaluate(scenarios []AgentScenario) ScenarioGateResult {
	issues := []ScenarioGateIssue{}
	if len(scenarios) == 0 {
		issues = append(issues, ScenarioGateIssue{ScenarioId: "<none>", Message: "No scenario JSON files were found."})

		return ScenarioGateResult{Issues: issues}
	}

	duplicate := map[string]int{}

	for _, scenario := range scenarios {
		if _, ok := duplicate[scenario.Id]; ok {
			duplicate[scenario.Id] = duplicate[scenario.Id] + 1
		} else {
			duplicate[scenario.Id] = 1
		}
		if util.IsBlank(scenario.Id) {
			issues = append(issues, qualityGateError(scenario, "Scenario is missing an id."))
		}

		if util.IsBlank(scenario.Title) {
			issues = append(issues, qualityGateError(scenario, "Scenario is missing a title."))
		}

		if !hasExplicitExpectedBehavior(scenario.Expected) {
			issues = append(issues, qualityGateError(scenario, "Scenario has no explicit expected behavior."))
		}

		if (scenario.Risk == ScenarioRiskHigh || scenario.Risk == ScenarioRiskCritical) && len(scenario.Oracles) == 0 {
			issues = append(issues, qualityGateError(scenario, "High-risk and critical scenarios must declare at least one oracle."))
		}

		if isToolUseScenario(scenario) && len(scenario.Expected.MustCallTools) == 0 && len(scenario.Expected.MustNotCallTools) == 0 {
			issues = append(issues, qualityGateError(scenario, "tool-use scenarios must declare mustCallTools or mustNotCallTools."))
		}

		if isApprovalOrSafetyScenario(scenario) && !hasApprovalOrSafetyOracle(scenario) {
			issues = append(issues, qualityGateError(scenario, "Approval or safety scenarios must declare an approval or safety oracle."))
		}

	}

	for k, v := range duplicate {
		if v > 1 {
			issues = append(issues, ScenarioGateIssue{
				ScenarioId: k,
				Message:    fmt.Sprintf("Duplicate scenario id '%s'", k),
			})
		}
	}

	return ScenarioGateResult{Issues: issues}
}
