package primitives

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
)

const (
	StatusPassed        = "passed"
	StatusFailed        = "failed"
	StatusSkipped       = "skipped"
	StatusWarning       = "warning"
	StatusNotApplicable = "notapplicable"

	SeverityInfo   = "info"
	SeverityLow    = "low"
	SeverityMedium = "medium"
	SeverityHigh   = "high"
)

type HarnessRegressionContext struct {
	ConfigPath         string              `json:"configPath"`
	ConfigPathExplicit bool                `json:"configPathExplicit"`
	ConfigExists       bool                `json:"configExists"`
	Config             *core.GatewayConfig `json:"config,omitempty"`
	ConfigLoadError    string              `json:"configLoadError,omitempty"`
	Offline            bool                `json:"offline"`
	Strict             bool                `json:"strict"`
	ProposalId         string              `json:"proposalId,omitempty"`
	TempWorkspacePath  string              `json:"tempWorkspacePath"`

	Logger io.Writer `json:"-"`
}

func (c *HarnessRegressionContext) Log(format string, a ...interface{}) {
	if c.Logger == nil {
		return
	}
	msg := fmt.Sprintf(format, a...)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	_, _ = fmt.Fprintf(c.Logger, "[%s] %s\n", timestamp, msg)
}

type HarnessRegressionScenarioResult struct {
	Id                string    `json:"id"`
	Name              string    `json:"name"`
	Category          string    `json:"category"`
	Status            string    `json:"status"`
	Severity          string    `json:"severity"`
	Required          bool      `json:"required"`
	Summary           string    `json:"summary"`
	Details           string    `json:"details,omitempty"`
	Error             string    `json:"error,omitempty"`
	StartedAtUtc      time.Time `json:"startedAtUtc"`
	CompletedAtUtc    time.Time `json:"completedAtUtc"`
	DurationMs        int64     `json:"durationMs"`
	EvidenceBundleId  string    `json:"evidenceBundleId,omitempty"`
	RelatedContractId string    `json:"relatedContractId,omitempty"`
}

func BuildNoScenariosResult(category string) *HarnessRegressionScenarioResult {
	summary := "No harness regression scenarios are registered."
	if category != "" {
		summary = fmt.Sprintf("No harness regression scenarios matched category '%s'", category)
	}
	now := time.Now()
	return &HarnessRegressionScenarioResult{
		Id:             "harness.no_scenarios",
		Name:           "No scenarios selected",
		Category:       HarnessRegressionCategoryHarness,
		Status:         HarnessRegressionScenarioStatusFailed,
		Severity:       HarnessRegressionSeverityMedium,
		Required:       true,
		Summary:        summary,
		StartedAtUtc:   now,
		CompletedAtUtc: now,
	}
}

func NormalizeHarnessRegressionScenarioResult(scenario IHarnessRegressionScenario, result HarnessRegressionScenarioResult) *HarnessRegressionScenarioResult {
	startedAt := result.StartedAtUtc
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	completedAt := result.CompletedAtUtc
	if completedAt.IsZero() {
		completedAt = startedAt
	}

	id := result.Id
	if strings.TrimSpace(id) == "" {
		id = scenario.Id()
	}

	name := result.Name
	if strings.TrimSpace(name) == "" {
		name = scenario.Name()
	}

	status := result.Status
	if strings.TrimSpace(status) == "" {
		status = StatusNotApplicable
	} else {
		status = normalize(status)
	}

	severity := result.Severity
	if strings.TrimSpace(severity) == "" {
		severity = SeverityInfo
	} else {
		severity = normalize(severity)
	}

	durationMs := result.DurationMs
	if durationMs <= 0 {
		durationMs = completedAt.Sub(startedAt).Milliseconds()
		if durationMs < 0 {
			durationMs = 0
		}
	}

	return &HarnessRegressionScenarioResult{
		Id:                id,
		Name:              name,
		Category:          normalizeOrFallback(result.Category, scenario.Category()),
		Status:            status,
		Severity:          severity,
		Required:          scenario.Required(),
		Summary:           result.Summary,
		Details:           result.Details,
		Error:             result.Error,
		StartedAtUtc:      startedAt,
		CompletedAtUtc:    completedAt,
		DurationMs:        durationMs,
		EvidenceBundleId:  result.EvidenceBundleId,
		RelatedContractId: result.RelatedContractId,
	}

}
func normalizeOrFallback(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return normalize(fallback)
	}
	return normalize(val)
}

type IHarnessRegressionScenario interface {
	Id() string
	Name() string
	Category() string
	Required() bool
	Run(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error)
}

type Evaluator interface {
	Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error)
}

type BaseScenario struct {
	id       string
	name     string
	category string
	required bool

	evaluator Evaluator
}

func NewBaseScenario(id, name, category string, required ...bool) BaseScenario {
	req := true
	if len(required) > 0 {
		req = required[0]
	}

	return BaseScenario{
		id:       id,
		name:     name,
		category: category,
		required: req,
	}
}

func (b *BaseScenario) SetEvaluator(evaluator Evaluator) {
	b.evaluator = evaluator
}

func (b *BaseScenario) Id() string       { return b.id }
func (b *BaseScenario) Name() string     { return b.name }
func (b *BaseScenario) Category() string { return b.category }
func (b *BaseScenario) Required() bool   { return b.required }

func (b *BaseScenario) Run(ctx context.Context, regCtx *HarnessRegressionContext) (result HarnessRegressionScenarioResult, err error) {
	startedAt := time.Now().UTC()

	defer func() {
		if r := recover(); r != nil {
			if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
				err = ctx.Err()
				return
			}

			failedResult := b.Failed("Scenario threw a panic.", fmt.Sprintf("%v", r), fmt.Sprintf("%+v", r))
			result = b.complete(failedResult, startedAt)
			err = nil
		}
	}()

	if err := ctx.Err(); err != nil {
		return HarnessRegressionScenarioResult{}, err
	}

	evalResult, evalErr := b.evaluator.Evaluate(ctx, regCtx)
	if evalErr != nil {
		if ctx.Err() != nil {
			return HarnessRegressionScenarioResult{}, ctx.Err()
		}

		failedResult := b.Failed("Scenario threw an error.", evalErr.Error(), fmt.Sprintf("%+v", evalErr))
		return b.complete(failedResult, startedAt), nil
	}

	return b.complete(evalResult, startedAt), nil
}

func (b *BaseScenario) Passed(summary string, details string, severity ...string) HarnessRegressionScenarioResult {
	sev := SeverityInfo
	if len(severity) > 0 {
		sev = severity[0]
	}
	return b.build(StatusPassed, summary, details, sev, "")
}

func (b *BaseScenario) Failed(summary string, details string, errorMsg string, severity ...string) HarnessRegressionScenarioResult {
	sev := SeverityHigh
	if len(severity) > 0 {
		sev = severity[0]
	}
	return b.build(StatusFailed, summary, details, sev, errorMsg)
}

func (b *BaseScenario) Skipped(summary string, details string, severity ...string) HarnessRegressionScenarioResult {
	sev := SeverityInfo
	if len(severity) > 0 {
		sev = severity[0]
	}
	return b.build(StatusSkipped, summary, details, sev, "")
}

func (b *BaseScenario) Warning(summary string, details string, severity ...string) HarnessRegressionScenarioResult {
	sev := SeverityMedium
	if len(severity) > 0 {
		sev = severity[0]
	}
	return b.build(StatusWarning, summary, details, sev, "")
}

func (b *BaseScenario) NotApplicable(summary string, details string, severity ...string) HarnessRegressionScenarioResult {
	sev := SeverityInfo
	if len(severity) > 0 {
		sev = severity[0]
	}
	return b.build(StatusNotApplicable, summary, details, sev, "")
}

func (b *BaseScenario) build(status, summary, details, severity, errorMsg string) HarnessRegressionScenarioResult {
	return HarnessRegressionScenarioResult{
		Status:   status,
		Summary:  summary,
		Details:  details,
		Severity: severity,
		Error:    errorMsg,
	}
}

func (b *BaseScenario) complete(result HarnessRegressionScenarioResult, startedAt time.Time) HarnessRegressionScenarioResult {
	completedAt := time.Now().UTC()
	duration := completedAt.Sub(startedAt).Milliseconds()
	if duration < 0 {
		duration = 0
	}

	status := result.Status
	if strings.TrimSpace(status) == "" {
		status = StatusNotApplicable
	} else {
		status = normalize(status)
	}

	severity := result.Severity
	if strings.TrimSpace(severity) == "" {
		severity = SeverityInfo
	} else {
		severity = normalize(severity)
	}

	return HarnessRegressionScenarioResult{
		Id:                b.id,
		Name:              b.name,
		Category:          normalize(b.category),
		Status:            status,
		Severity:          severity,
		Required:          b.required,
		Summary:           result.Summary,
		Details:           result.Details,
		Error:             result.Error,
		StartedAtUtc:      startedAt,
		CompletedAtUtc:    completedAt,
		DurationMs:        duration,
		EvidenceBundleId:  result.EvidenceBundleId,
		RelatedContractId: result.RelatedContractId,
	}
}

func normalize(val string) string {
	return strings.ToLower(strings.TrimSpace(val))
}

// DEMO

type ExampleScenario struct {
	BaseScenario
}

func NewExampleScenario() *ExampleScenario {
	s := &ExampleScenario{
		BaseScenario: NewBaseScenario("SCENARIO_001", "test", "Integration", true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *ExampleScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	return s.Passed("ok", "..."), nil
}
