package skillkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/futugyou/openclaw/util"
)

type ISkillCritiqueProvider interface {
	Critique(ctx context.Context, pkg SkillPackage) (*SkillCritiqueResult, error)
}

type DeterministicSkillCritiqueProvider struct {
}

func IsVague(value string) bool {
	var normalized = strings.TrimSpace(value)
	if normalized == "" {
		return true
	}

	return len(normalized) < 40 ||
		strings.Contains(normalized, "help") && len(strings.Split(normalized, " ")) < 10
}

func ContainsSectionText(text, section string) bool {
	var index = strings.Index(text, section)
	if index < 0 {
		return false
	}

	var after = text[(index + len(section)):]
	for line := range strings.SplitSeq(after, "\n") {
		if strings.HasPrefix(util.TrimStart(line), "- ") {
			return true
		}
	}
	return false
}

func AddIf(findings []string, condition bool, finding string) []string {
	if condition {
		findings = append(findings, finding)
	}
	return findings
}

func (d *DeterministicSkillCritiqueProvider) Critique(ctx context.Context, pkg SkillPackage) (*SkillCritiqueResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	findings := []string{}
	var manifest = pkg.Manifest
	wfhasvalidationsteps := false
	for _, step := range manifest.Workflow.Steps {
		if step.Type == StepTypeValidation {
			wfhasvalidationsteps = true
			break
		}
	}

	findings = AddIf(findings, IsVague(manifest.Intent.Outcome), "Outcome is vague; add concrete success criteria and expected downstream use.")
	findings = AddIf(findings, len(manifest.HumanApproval.RequiredFor) == 0, "No human approval points are defined.")
	findings = AddIf(findings, len(manifest.Tools.Forbidden) == 0, "No forbidden tools are defined.")
	findings = AddIf(findings, len(manifest.Validation.Checks) == 0, "No validation checks are defined.")
	findings = AddIf(findings, !wfhasvalidationsteps, "Workflow has no validation step.")

	intent, ok := pkg.Files["intent.md"]
	findings = AddIf(findings, !ok || !ContainsSectionText(intent, "Failure Scenarios"), "No failure scenarios are documented.")

	guardrails, ok := pkg.Files["guardrails.md"]
	findings = AddIf(findings, !ok || !strings.Contains(guardrails, "Missing Information"), "Missing-information behavior is not documented.")
	findings = AddIf(findings, !ok || !strings.Contains(guardrails, "Grounding"), "Grounding or attribution rules are not documented.")

	examples, ok := pkg.Files["examples.md"]
	findings = AddIf(findings, !ok || !strings.Contains(examples, "Expected Output"), "Examples do not include an expected output outline.")

	if len(findings) == 0 {
		findings = append(findings, "No deterministic critique findings.")
	}

	var builder = strings.Builder{}
	builder.WriteString("# Skill Critique\n\n")
	fmt.Fprintf(&builder, "Skill: %s\n", manifest.Name)
	fmt.Fprintf(&builder, "GeneratedAtUtc:  %s\n\n", time.Now().UTC().Format(time.RFC3339Nano))
	builder.WriteString("## Findings\n\n")
	for _, finding := range findings {
		fmt.Fprintf(&builder, "- %s\n", finding)
	}

	builder.WriteString("\n")
	builder.WriteString("## Notes\n\n")
	builder.WriteString("- This critique is deterministic and does not call an LLM.\n")
	builder.WriteString("- Future critique providers can implement `ISkillCritiqueProvider`.\n")

	return &SkillCritiqueResult{
		Markdown: builder.String(),
		Findings: findings,
	}, nil
}
