package primitives

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	HarnessRegressionScenarioStatusPassed        = "passed"
	HarnessRegressionScenarioStatusFailed        = "failed"
	HarnessRegressionScenarioStatusSkipped       = "skipped"
	HarnessRegressionScenarioStatusWarning       = "warning"
	HarnessRegressionScenarioStatusNotApplicable = "not_applicable"
)

const (
	HarnessRegressionCategoryOnboarding   = "onboarding"
	HarnessRegressionCategorySecurity     = "security"
	HarnessRegressionCategoryApprovals    = "approvals"
	HarnessRegressionCategoryMemory       = "memory"
	HarnessRegressionCategoryProviders    = "providers"
	HarnessRegressionCategoryTools        = "tools"
	HarnessRegressionCategoryMcp          = "mcp"
	HarnessRegressionCategoryOpenAiCompat = "openai_compat"
	HarnessRegressionCategorySessions     = "sessions"
	HarnessRegressionCategoryHarness      = "harness"
	HarnessRegressionCategoryDeployment   = "deployment"
	HarnessRegressionCategoryDocs         = "docs"
)

const (
	HarnessRegressionSeverityInfo     = "info"
	HarnessRegressionSeverityLow      = "low"
	HarnessRegressionSeverityMedium   = "medium"
	HarnessRegressionSeverityHigh     = "high"
	HarnessRegressionSeverityCritical = "critical"
)

type HarnessRegressionOptions struct {
	ConfigPath string `json:"configPath"`
	Category   string `json:"category"`
	Offline    bool   `json:"offline"`
	Strict     bool   `json:"strict"`
	ProposalId string `json:"proposalId"`
	OutputPath string `json:"outputPath"`
}

func NewHarnessRegressionOptions() HarnessRegressionOptions {
	return HarnessRegressionOptions{
		Offline: true,
	}
}

type HarnessRegressionReport struct {
	Id              string                            `json:"id"`
	StartedAtUTC    time.Time                         `json:"startedAtUtc"`
	CompletedAtUTC  time.Time                         `json:"completedAtUtc"`
	DurationMs      int64                             `json:"durationMs"`
	ConfigPath      string                            `json:"configPath"`
	ProposalId      string                            `json:"proposalId"`
	Offline         bool                              `json:"offline"`
	Strict          bool                              `json:"strict"`
	OverallStatus   string                            `json:"overallStatus"`
	Results         []HarnessRegressionScenarioResult `json:"results"`
	Summary         HarnessRegressionSummary          `json:"summary"`
	Recommendations []HarnessRegressionRecommendation `json:"recommendations"`
}

func NewHarnessRegressionReport() *HarnessRegressionReport {
	return &HarnessRegressionReport{
		OverallStatus:   HarnessRegressionScenarioStatusPassed,
		Results:         make([]HarnessRegressionScenarioResult, 0),
		Recommendations: make([]HarnessRegressionRecommendation, 0),
	}
}

func (h *HarnessRegressionReport) ToText() string {
	if h == nil {
		return ""
	}
	sb := &strings.Builder{}
	sb.WriteString("OpenClaw Harness Regression\n\n")
	for _, result := range h.Results {
		fmt.Fprintf(sb, "%s %s - %s\n", ScenarioResultStatusLable(result.Status), result.Id, result.Summary)
		if result.Error != "" {
			fmt.Fprintf(sb, "  error: %s\n", result.Error)
		}
	}

	sb.WriteString("\n")
	sb.WriteString("Summary:\n")
	fmt.Fprintf(sb, "%b passed, %d failed, %b skipped, %d warning, %d not applicable\n", h.Summary.Passed, h.Summary.Failed, h.Summary.Skipped, h.Summary.Warning, h.Summary.NotApplicable)

	if len(h.Recommendations) > 0 {
		sb.WriteString("\n")
		sb.WriteString("Next steps:\n")
		for _, recommendation := range h.Recommendations {
			if recommendation.Command == "" {
				sb.WriteString(fmt.Sprintf("- %s\n", recommendation.Summary))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", recommendation.Command))
			}
		}
	}
	return sb.String()
}

func (h *HarnessRegressionReport) GetExitCode() int {
	if h == nil {
		return -1
	}
	for _, result := range h.Results {
		if result.Required && result.Status == HarnessRegressionScenarioStatusFailed {
			return 1
		}
	}
	if h.Strict {
		for _, result := range h.Results {
			if result.Required && (result.Status == HarnessRegressionScenarioStatusSkipped || result.Status == HarnessRegressionScenarioStatusWarning) {
				return 1
			}
		}
	}
	return 0
}

func ScenarioResultStatusLable(status string) string {
	switch strings.ToLower(status) {
	case HarnessRegressionScenarioStatusPassed:
		return "PASS"
	case HarnessRegressionScenarioStatusFailed:
		return "FAIL"
	case HarnessRegressionScenarioStatusSkipped:
		return "SKIP"
	case HarnessRegressionScenarioStatusWarning:
		return "WARN"
	case HarnessRegressionScenarioStatusNotApplicable:
		return "N/A"
	default:
		return "????"
	}
}

type HarnessRegressionSummary struct {
	Total         int `json:"total"`
	Passed        int `json:"passed"`
	Failed        int `json:"failed"`
	Skipped       int `json:"skipped"`
	Warning       int `json:"warning"`
	NotApplicable int `json:"notApplicable"`
}

type HarnessRegressionRecommendation struct {
	Id       string `json:"id"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Command  string `json:"command"`
	Details  string `json:"details"`
}

func NewHarnessRegressionRecommendation() HarnessRegressionRecommendation {
	return HarnessRegressionRecommendation{
		Severity: HarnessRegressionSeverityInfo,
	}
}

func HarnessRegressionScenarioTextJoin(values []string) string {
	var filtered []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			filtered = append(filtered, v)
		}
	}
	return strings.Join(filtered, "\n")
}

func HarnessRegressionPathsChild(root, childName string) (string, error) {
	fullRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	baseChild := filepath.Base(childName)
	fullChild, err := filepath.Abs(filepath.Join(fullRoot, baseChild))
	if err != nil {
		return "", err
	}

	rootPrefix := fullRoot
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}

	if !strings.HasPrefix(fullChild, rootPrefix) {
		return "", fmt.Errorf("resolved harness path escaped the temp workspace: %s", childName)
	}

	return fullChild, nil
}
