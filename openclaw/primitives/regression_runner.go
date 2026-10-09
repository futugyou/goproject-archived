package primitives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/google/uuid"
)

func HarnessRegressionScenariosCreateDefault() []IHarnessRegressionScenario {
	return []IHarnessRegressionScenario{
		&QuickstartConfigLoadsScenario{},
		&ProviderConfigShapeScenario{},
		&PublicBindHardeningScenario{},
		&UrlSafetyDefaultsScenario{},
		&ToolApprovalPolicyScenario{},
		&MemoryStoreRoundTripScenario{},
		&SessionStoreRoundTripScenario{},
		&HarnessContractSerializationScenario{},
		&EvidenceBundleSerializationScenario{},
		&GovernanceLedgerSerializationScenario{},
		&CapabilityBindingTrajectorySerializationScenario{},
		&McpInitializeShapeScenario{},
		&OpenAiCompatRequestShapeScenario{},
		&LearningProposalReviewFirstScenario{},
		&ManagedSkillValidationScenario{},
		&TailscaleServeProfileNonPublicScenario{},
		&HarnessRegressionDocsScenario{},
	}
}

type HarnessRegressionRunner struct {
	scenarios []IHarnessRegressionScenario
}

func NewHarnessRegressionRunner(scenarios []IHarnessRegressionScenario) *HarnessRegressionRunner {
	if len(scenarios) == 0 {
		scenarios = HarnessRegressionScenariosCreateDefault()
	}
	return &HarnessRegressionRunner{
		scenarios: scenarios,
	}
}

func (r *HarnessRegressionRunner) Scenarios() []IHarnessRegressionScenario {
	return r.scenarios
}

func (r *HarnessRegressionRunner) Run(ctx context.Context, options *HarnessRegressionOptions) (*HarnessRegressionReport, error) {
	if options == nil {
		options = &HarnessRegressionOptions{}
	}

	startedAt := time.Now().UTC()
	harnessCtx, err := r.buildContext(options)
	if err != nil {
		return nil, fmt.Errorf("build context failed: %w", err)
	}
	defer func() {
		_ = tryDeleteDirectory(harnessCtx.TempWorkspacePath)
	}()

	selected := r.selectScenarios(options.Category)
	results := make([]HarnessRegressionScenarioResult, 0, len(selected))

	if len(selected) == 0 {
		results = append(results, *BuildNoScenariosResult(options.Category))
	} else {
		for _, scenario := range selected {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			result, runErr := scenario.Run(ctx, harnessCtx)
			if runErr != nil {
				if result.Error == "" {
					result.Error = runErr.Error()
				}
			}
			results = append(results, *NormalizeHarnessRegressionScenarioResult(scenario, result))
		}
	}

	completedAt := time.Now().UTC()
	durationMs := completedAt.Sub(startedAt).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}

	summary := buildSummary(results)
	overallStatus := resolveOverallStatus(results, options.Strict)

	report := &HarnessRegressionReport{
		Id:              createReportID(startedAt),
		StartedAtUTC:    startedAt,
		CompletedAtUTC:  completedAt,
		DurationMs:      durationMs,
		ConfigPath:      harnessCtx.ConfigPath,
		ProposalId:      options.ProposalId,
		Offline:         harnessCtx.Offline,
		Strict:          options.Strict,
		OverallStatus:   overallStatus,
		Results:         results,
		Summary:         summary,
		Recommendations: buildRecommendations(results),
	}

	return report, nil
}

func buildRecommendations(results []HarnessRegressionScenarioResult) []HarnessRegressionRecommendation {
	var recommendations []HarnessRegressionRecommendation

	for _, res := range results {
		if res.Id == "onboarding.quickstart_config" && (isStatus(res, StatusSkipped) || isStatus(res, StatusFailed)) {
			recommendations = append(recommendations, HarnessRegressionRecommendation{
				Id:       "setup.verify",
				Severity: SeverityMedium,
				Summary:  "Create or verify the OpenClaw config before trusting runtime checks.",
				Command:  "openclaw setup verify --offline",
			})
			break
		}
	}

	for _, res := range results {
		if isCategory(res, HarnessRegressionCategorySecurity) && (isStatus(res, StatusFailed) || isStatus(res, StatusWarning)) {
			recommendations = append(recommendations, HarnessRegressionRecommendation{
				Id:       "security.posture",
				Severity: SeverityHigh,
				Summary:  "Review operator security posture before widening the runtime surface.",
				Command:  "openclaw admin posture",
			})
			break
		}
	}

	for _, res := range results {
		if isCategory(res, HarnessRegressionCategoryProviders) && (isStatus(res, StatusFailed) || isStatus(res, StatusWarning)) {
			recommendations = append(recommendations, HarnessRegressionRecommendation{
				Id:       "models.doctor",
				Severity: SeverityMedium,
				Summary:  "Review model profile shape and provider readiness.",
				Command:  "openclaw models doctor",
			})
			break
		}
	}

	for _, res := range results {
		if isStatus(res, StatusSkipped) || isStatus(res, StatusNotApplicable) {
			recommendations = append(recommendations, HarnessRegressionRecommendation{
				Id:       "harness.docs",
				Severity: SeverityInfo,
				Summary:  "Review skipped and not-applicable checks before using the report as release evidence.",
				Command:  "openclaw harness test --strict",
			})
			break
		}
	}

	return recommendations
}

func createReportID(startedAt time.Time) string {
	u := strings.ReplaceAll(uuid.New().String(), "-", "")
	raw := fmt.Sprintf("hreg_%s_%s", startedAt.Format("20060102150405"), u)
	if len(raw) > 40 {
		return raw[:40]
	}
	return raw
}

func resolveOverallStatus(results []HarnessRegressionScenarioResult, strict bool) string {
	for _, res := range results {
		if res.Required && strings.EqualFold(res.Status, StatusFailed) {
			return StatusFailed
		}
	}

	if strict {
		for _, res := range results {
			if res.Required && (strings.EqualFold(res.Status, StatusWarning) || strings.EqualFold(res.Status, StatusSkipped)) {
				return StatusFailed
			}
		}
	}

	for _, res := range results {
		if strings.EqualFold(res.Status, StatusWarning) {
			return StatusWarning
		}
	}

	return StatusPassed
}

func (r *HarnessRegressionRunner) buildContext(options *HarnessRegressionOptions) (*HarnessRegressionContext, error) {
	configPathExplicit := strings.TrimSpace(options.ConfigPath) != ""

	rawPath := options.ConfigPath
	if !configPathExplicit {
		rawPath = core.DefaultConfigPath
	}

	configPath, err := filepath.Abs(core.GatewaySetupPathsIntance.ExpandPath(rawPath))
	if err != nil {
		configPath = rawPath
	}

	configExists := fileExists(configPath)
	var config *core.GatewayConfig
	var configLoadError string

	if configExists {
		var err error
		config, err = core.GatewayConfigFileInstance.Load(configPath)
		if err != nil {
			configLoadError = err.Error()
		}
	} else {
		configLoadError = fmt.Sprintf("Config file not found: %s", configPath)
	}

	tempWorkspacePath, err := createTempWorkspace()
	if err != nil {
		return nil, fmt.Errorf("failed to create temp workspace: %w", err)
	}

	return &HarnessRegressionContext{
		ConfigPath:         configPath,
		ConfigPathExplicit: configPathExplicit,
		ConfigExists:       configExists,
		Config:             config,
		ConfigLoadError:    configLoadError,
		Offline:            options.Offline,
		Strict:             options.Strict,
		ProposalId:         options.ProposalId,
		TempWorkspacePath:  tempWorkspacePath,
	}, nil
}

func (r *HarnessRegressionRunner) selectScenarios(category string) []IHarnessRegressionScenario {
	if strings.TrimSpace(category) == "" {
		return r.scenarios
	}

	normalized := normalize(category)
	var filtered []IHarnessRegressionScenario
	for _, scenario := range r.scenarios {
		if strings.EqualFold(scenario.Category(), normalized) {
			filtered = append(filtered, scenario)
		}
	}
	return filtered
}

func buildSummary(results []HarnessRegressionScenarioResult) HarnessRegressionSummary {
	return HarnessRegressionSummary{
		Total:         len(results),
		Passed:        countStatus(results, StatusPassed),
		Failed:        countStatus(results, StatusFailed),
		Skipped:       countStatus(results, StatusSkipped),
		Warning:       countStatus(results, StatusWarning),
		NotApplicable: countStatus(results, StatusNotApplicable),
	}
}

func countStatus(results []HarnessRegressionScenarioResult, status string) int {
	cnt := 0
	for _, res := range results {
		if strings.EqualFold(res.Status, status) {
			cnt++
		}
	}
	return cnt
}

func createTempWorkspace() (string, error) {
	u := strings.ReplaceAll(uuid.New().String(), "-", "")
	dirName := fmt.Sprintf("openclaw-harness-regression-%s", u)
	dirPath := filepath.Join(os.TempDir(), dirName)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return "", err
	}
	return dirPath, nil
}

func tryDeleteDirectory(path string) error {
	if path == "" {
		return nil
	}
	return os.RemoveAll(path)
}

func isStatus(result HarnessRegressionScenarioResult, status string) bool {
	return strings.EqualFold(result.Status, status)
}

func isCategory(result HarnessRegressionScenarioResult, category string) bool {
	return strings.EqualFold(result.Category, category)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
