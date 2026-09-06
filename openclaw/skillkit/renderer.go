package skillkit

import (
	"fmt"
	"strings"
)

var SkillTemplateRequiredFiles []string = []string{
	"skill.yaml",
	"intent.md",
	"expectations.md",
	"workflow.yaml",
	"tools.yaml",
	"guardrails.md",
	"validation.md",
	"examples.md",
	"trace.md",
}

type TemplateProfile struct {
	Description              string
	Outcome                  string
	RequiredInputs           []string
	OptionalInputs           []string
	RequiredOutputs          []string
	OptionalOutputs          []string
	AllowedTools             []string
	ForbiddenTools           []string
	ApprovalRequiredTools    []string
	MustNot                  []string
	HumanApprovalRequiredFor []string
	ValidationChecks         []string
}

func ForTemplateProfile(template, category string) TemplateProfile {
	switch template {
	case "research":
		return researchProfile
	case "proposal":
		return proposalProfile
	case "compliance":
		return complianceProfile
	case "operations":
		return operationsProfile
	case "software":
		return softwareProfile
	default:
		return genericProfile(category)
	}
}

func genericProfile(category string) TemplateProfile {
	return TemplateProfile{
		Description:              fmt.Sprintf("Reusable %s skill with explicit inputs, guardrails, tool policy, and validation checks.", category),
		Outcome:                  "Produce a structured, grounded output that can be reviewed before use.",
		RequiredInputs:           []string{"source_material"},
		OptionalInputs:           []string{"project_context", "target_audience"},
		RequiredOutputs:          []string{"summary", "recommendations", "risks", "next_steps"},
		OptionalOutputs:          []string{},
		AllowedTools:             []string{"file.read", "file.search", "memory.retrieve"},
		ForbiddenTools:           []string{"email.send", "external_submit", "data_delete"},
		ApprovalRequiredTools:    []string{"external_publication", "final_recommendations"},
		MustNot:                  []string{"Invent facts or evidence.", "Take external action without approval.", "Hide uncertainty or missing information."},
		HumanApprovalRequiredFor: []string{"final_recommendations", "external_publication"},
		ValidationChecks:         []string{"Required outputs are present.", "Important claims are grounded in supplied inputs.", "Missing information is listed separately."},
	}
}

var researchProfile = TemplateProfile{
	Description:              "Extracts pain points, stakeholder needs, risks, and practical technology opportunities from community-engaged research discussions.",
	Outcome:                  "Produce a structured insight brief that helps researchers understand key issues and identify practical support opportunities without replacing community judgment.",
	RequiredInputs:           []string{"transcript_or_notes"},
	OptionalInputs:           []string{"project_context", "target_audience", "prior_discussions"},
	RequiredOutputs:          []string{"executive_summary", "key_pain_points", "stakeholder_needs", "opportunity_map", "risks_and_cautions", "follow_up_questions"},
	OptionalOutputs:          []string{},
	AllowedTools:             []string{"file.read", "file.search", "web.search", "memory.retrieve"},
	ForbiddenTools:           []string{"email.send", "external_submit", "data_delete"},
	ApprovalRequiredTools:    []string{"final_recommendations", "external_publication", "named_attribution"},
	MustNot:                  []string{"Invent participant quotes.", "Attribute views to named people unless present in the source.", "Recommend replacing community engagement with automation.", "Treat AI-generated themes as final truth without human review."},
	HumanApprovalRequiredFor: []string{"final_recommendations", "external_publication", "named_attribution"},
	ValidationChecks:         []string{"Every key claim is grounded in the transcript or provided context.", "Recommendations are framed as support tools, not replacements for relationships.", "Missing information is listed separately.", "Risks are included alongside opportunities."},
}

var proposalProfile = TemplateProfile{
	Description:              "Drafts a concept note or proposal from grounded project inputs.",
	Outcome:                  "Produce a concise proposal draft with clear goals, beneficiaries, activities, risks, and review points.",
	RequiredInputs:           []string{"project_brief"},
	OptionalInputs:           []string{"funder_guidance", "budget_notes", "organization_context"},
	RequiredOutputs:          []string{"concept_summary", "need_statement", "proposed_activities", "outcomes", "risks", "review_questions"},
	OptionalOutputs:          []string{},
	AllowedTools:             []string{"file.read", "file.search", "memory.retrieve"},
	ForbiddenTools:           []string{"external_submit", "email.send", "data_delete"},
	ApprovalRequiredTools:    []string{"budget_commitments", "external_submission", "final_recommendations"},
	MustNot:                  []string{"Invent funder requirements.", "Commit funds or organizational promises without approval.", "Submit externally without human review."},
	HumanApprovalRequiredFor: []string{"budget_commitments", "external_submission", "final_recommendations"},
	ValidationChecks:         []string{"Proposal claims are grounded in supplied material.", "Open assumptions are listed.", "Submission requires human approval."},
}

var complianceProfile = TemplateProfile{
	Description:              "Reviews compliance materials against provided requirements and produces an issue list.",
	Outcome:                  "Produce a grounded compliance review that separates findings, evidence, risks, and open questions.",
	RequiredInputs:           []string{"document_or_policy", "requirements"},
	OptionalInputs:           []string{"jurisdiction", "prior_findings"},
	RequiredOutputs:          []string{"findings", "evidence_table", "risk_summary", "open_questions", "recommended_reviews"},
	OptionalOutputs:          []string{},
	AllowedTools:             []string{"file.read", "file.search"},
	ForbiddenTools:           []string{"external_submit", "data_delete", "policy_change"},
	ApprovalRequiredTools:    []string{"legal_conclusions", "external_publication", "policy_change"},
	MustNot:                  []string{"Present legal advice as final.", "Invent requirements.", "Change policy without approval."},
	HumanApprovalRequiredFor: []string{"legal_conclusions", "external_publication", "policy_change"},
	ValidationChecks:         []string{"Each finding cites supplied evidence.", "Uncertain interpretations are marked for review.", "Risks are included."},
}

var operationsProfile = TemplateProfile{
	Description:              "Plans operational work with explicit checks, risks, and approval points.",
	Outcome:                  "Produce an operational task plan that is clear, bounded, and safe to review before action.",
	RequiredInputs:           []string{"task_request"},
	OptionalInputs:           []string{"constraints", "systems_context", "deadline"},
	RequiredOutputs:          []string{"task_plan", "dependencies", "risks", "approval_points", "verification_steps"},
	OptionalOutputs:          []string{},
	AllowedTools:             []string{"file.read", "file.search", "memory.retrieve"},
	ForbiddenTools:           []string{"external_submit", "data_delete", "system_modify"},
	ApprovalRequiredTools:    []string{"system_change", "external_communication", "final_execution"},
	MustNot:                  []string{"Modify production systems without approval.", "Hide operational risk.", "Skip verification steps."},
	HumanApprovalRequiredFor: []string{"system_change", "external_communication", "final_execution"},
	ValidationChecks:         []string{"Dependencies are listed.", "Risks are visible.", "Verification steps are defined."},
}

var softwareProfile = TemplateProfile{
	Description:              "Creates a development task plan or implementation prompt from repository context.",
	Outcome:                  "Produce an actionable software work plan with constraints, tests, and acceptance criteria.",
	RequiredInputs:           []string{"task_request"},
	OptionalInputs:           []string{"repo_context", "existing_findings", "target_files"},
	RequiredOutputs:          []string{"implementation_plan", "test_plan", "risks", "acceptance_criteria"},
	OptionalOutputs:          []string{},
	AllowedTools:             []string{"file.read", "file.search", "git.diff"},
	ForbiddenTools:           []string{"git.push", "external_submit", "data_delete"},
	ApprovalRequiredTools:    []string{"file_write", "shell_execution", "merge_or_release"},
	MustNot:                  []string{"Invent APIs without checking code.", "Change files or run mutating commands without approval.", "Skip validation for risky changes."},
	HumanApprovalRequiredFor: []string{"file_write", "shell_execution", "merge_or_release"},
	ValidationChecks:         []string{"Plan references concrete files or seams.", "Tests are listed.", "Known risks and unknowns are explicit."},
}

func createDefaultWorkflow() SkillWorkflow {
	return SkillWorkflow{
		Steps: []SkillWorkflowStep{
			{Id: "collect_inputs", Name: "Collect Required Inputs", Type: StepTypeInput, Description: "Ensure all required inputs are present."},
			{Id: "analyze_source", Name: "Analyze Source Material", Type: StepTypeReasoning, Description: "Extract grounded themes and evidence."},
			{Id: "draft_output", Name: "Draft Output", Type: StepTypeGeneration, Description: "Produce the requested structured output."},
			{Id: "validate_output", Name: "Validate Output", Type: StepTypeValidation, Description: "Check output against validation rules."},
			{Id: "request_human_review", Name: "Request Human Review", Type: StepTypeApproval, Description: "Ask for human review before final use if required."},
		},
	}
}

// 1.明确能力边界
func renderIntent(manifest SkillManifest) string {
	return fmt.Sprintf(`# Intent

## Outcome

%s

## Users

- People who need repeatable, reviewable agent assistance for %s work.

## Required Inputs

%s

## Expected Outputs

%s

## Constraints

- Follow the tool policy in `+"`tools.yaml`"+`.
- Follow the guardrails in `+"`guardrails.md`"+`.
- Ask for human review when required by the manifest.

## Success Scenarios

- The output is complete, grounded in supplied inputs, and ready for human review.

## Failure Scenarios

- Required inputs are missing.
- The output includes unsupported claims.
- The workflow reaches a human approval point and no reviewer is available.`,
		manifest.Intent.Outcome,
		manifest.Category,
		formatList(manifest.Inputs.Required),
		formatList(manifest.Outputs.Required),
	)
}

// 行为准则
func renderExpectations(manifest SkillManifest) string {
	return fmt.Sprintf(`# Expectations

## Done Means

- All required outputs are present.
- Validation checks pass or unresolved issues are explicitly listed.

## Must-Have Behaviors

- Ground conclusions in the provided inputs.
- Separate known facts from inferred or uncertain points.
- Preserve missing information as a visible section instead of filling gaps.

## Must-Not-Happen Behaviors

%s

## Quality Bar

- Clear, structured, and usable by the intended reviewer.
- No external action is taken without approval when approval is required.

## Human Review Required When

%s`,
		formatList(manifest.Guardrails.MustNot),
		formatList(manifest.HumanApproval.RequiredFor),
	)
}

// 安全红线
func renderGuardrails(manifest SkillManifest) string {
	return fmt.Sprintf(`# Guardrails

## Must Not

%s

## Requires Human Approval

%s

## Missing Information Behavior

- List missing inputs or uncertain areas separately.
- Do not invent facts, quotes, evidence, or approvals.

## Attribution and Grounding Rules

- Attribute claims only when the source material supports attribution.
- Distinguish direct evidence from interpretation.`,
		formatList(manifest.Guardrails.MustNot),
		formatList(manifest.HumanApproval.RequiredFor),
	)
}

func renderValidation(manifest SkillManifest) string {
	completenessChecks := make([]string, len(manifest.Outputs.Required))
	for i, output := range manifest.Outputs.Required {
		completenessChecks[i] = fmt.Sprintf("Output includes `%s`.", output)
	}

	return fmt.Sprintf(`# Validation

## Required Checks

%s

## Output Completeness

%s

## Grounding Checks

- Every important claim is traceable to supplied inputs or clearly marked as inference.

## Risk Checks

- Risks, cautions, and missing information are visible.

## Approval Checks

- Human approval points are respected before final use or external action.`,
		formatList(manifest.Validation.Checks),
		formatList(completenessChecks),
	)
}

func renderExamples(manifest SkillManifest) string {
	return fmt.Sprintf(`# Examples

## Example Input

`+"```text"+`
Provide %s for %s.
`+"```"+`

## Expected Output Outline

%s`,
		strings.Join(manifest.Inputs.Required, ", "),
		manifest.Name,
		formatList(manifest.Outputs.Required),
	)
}

func formatList(values []string) string {
	if len(values) == 0 {
		return "- None specified."
	}

	formatted := make([]string, len(values))
	for i, v := range values {
		formatted[i] = "- " + v
	}

	return strings.Join(formatted, "\n")
}

type SkillTemplateRenderer struct {
}

func (s *SkillTemplateRenderer) RenderTrace(manifest SkillManifest, status string) string {
	sb := &strings.Builder{}
	sb.WriteString("# Trace\n\n")
	sb.WriteString("## Skill\n\n")
	fmt.Fprintf(sb, "- ID: %s\n", manifest.Id)
	fmt.Fprintf(sb, "- Name: %s\n", manifest.Name)
	fmt.Fprintf(sb, "- Version: %s\n\n", manifest.Version)
	sb.WriteString("## Generated Files\n\n")
	for _, file := range SkillTemplateRequiredFiles {
		fmt.Fprintf(sb, "- %s\n", file)
	}

	sb.WriteString("\n## Open Decisions\n\n")
	sb.WriteString("- Review whether the default tool policy is sufficient for the intended environment.\n\n")
	sb.WriteString("## Validation Status\n\n")
	fmt.Fprintf(sb, "- %s\n\n", status)
	sb.WriteString("## Run History\n\n")
	sb.WriteString("- No runs recorded.\n")
	return sb.String()
}

func (s *SkillTemplateRenderer) RenderFiles(manifest SkillManifest) map[string]string {
	return map[string]string{
		"skill.yaml":      SkillManifestSerialize(&manifest),
		"intent.md":       renderIntent(manifest),
		"expectations.md": renderExpectations(manifest),
		"workflow.yaml":   SkillManifestSerializeWorkflow(manifest.Workflow),
		"tools.yaml":      SkillManifestSerializeTools(manifest.Tools),
		"guardrails.md":   renderGuardrails(manifest),
		"validation.md":   renderValidation(manifest),
		"examples.md":     renderExamples(manifest),
		"trace.md":        s.RenderTrace(manifest, "created"),
	}
}

func (s *SkillTemplateRenderer) CreateManifest(name, category, template string) *SkillManifest {
	normalizedCategory := strings.ToLower(strings.TrimSpace(category))
	if normalizedCategory == "" {
		normalizedCategory = "general"
	}

	normalizedTemplate := strings.ToLower(strings.TrimSpace(template))
	if normalizedTemplate == "" {
		normalizedTemplate = normalizedCategory
	}

	name = strings.TrimSpace(name)
	var id = SkillIdGenerate(name)
	if name == "" {
		name = "Untitled Skill"
	}
	var profile = ForTemplateProfile(normalizedTemplate, normalizedCategory)

	return &SkillManifest{
		Id:          id,
		Name:        name,
		Version:     "0.1.0",
		Category:    normalizedCategory,
		Description: profile.Description,
		Intent:      SkillIntent{Outcome: profile.Outcome},
		Inputs: SkillInputs{
			Required: profile.RequiredInputs,
			Optional: profile.OptionalInputs,
		},
		Outputs: SkillOutputs{
			Required: profile.RequiredOutputs,
			Optional: profile.OptionalOutputs,
		},
		Tools: SkillToolPolicy{
			Allowed:          profile.AllowedTools,
			Forbidden:        profile.ForbiddenTools,
			ApprovalRequired: profile.ApprovalRequiredTools,
		},
		Guardrails:    SkillGuardrails{MustNot: profile.MustNot},
		HumanApproval: SkillHumanApprovalPolicy{RequiredFor: profile.HumanApprovalRequiredFor},
		Validation:    SkillValidationPolicy{Checks: profile.ValidationChecks},
		Workflow:      createDefaultWorkflow(),
	}
}
