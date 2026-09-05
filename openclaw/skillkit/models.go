package skillkit

import (
	"strings"
)

type SkillWorkflowStepType string

const (
	StepTypeInput      SkillWorkflowStepType = "Input"
	StepTypeReasoning  SkillWorkflowStepType = "Reasoning"
	StepTypeGeneration SkillWorkflowStepType = "Generation"
	StepTypeValidation SkillWorkflowStepType = "Validation"
	StepTypeApproval   SkillWorkflowStepType = "Approval"
	StepTypeOutput     SkillWorkflowStepType = "Output"
)

type SkillValidationSeverity string

const (
	SeverityPass    SkillValidationSeverity = "Pass"
	SeverityWarning SkillValidationSeverity = "Warning"
	SeverityError   SkillValidationSeverity = "Error"
)

type SkillManifest struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Version       string                   `json:"version"`
	Category      string                   `json:"category"`
	Description   string                   `json:"description"`
	Aliases       []string                 `json:"aliases"`
	Intent        SkillIntent              `json:"intent"`
	Inputs        SkillInputs              `json:"inputs"`
	Outputs       SkillOutputs             `json:"outputs"`
	Tools         SkillToolPolicy          `json:"tools"`
	Guardrails    SkillGuardrails          `json:"guardrails"`
	HumanApproval SkillHumanApprovalPolicy `json:"humanApproval"`
	Validation    SkillValidationPolicy    `json:"validation"`
	Workflow      SkillWorkflow            `json:"workflow"`
}

func NewSkillManifest() SkillManifest {
	return SkillManifest{
		Version:       "0.1.0",
		Category:      "general",
		Aliases:       make([]string, 0),
		Intent:        SkillIntent{},
		Inputs:        NewSkillInputs(),
		Outputs:       NewSkillOutputs(),
		Tools:         NewSkillToolPolicy(),
		Guardrails:    NewSkillGuardrails(),
		HumanApproval: NewSkillHumanApprovalPolicy(),
		Validation:    NewSkillValidationPolicy(),
		Workflow:      NewSkillWorkflow(),
	}
}

type SkillIntent struct {
	Outcome string `json:"outcome"`
}

type SkillInputs struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

func NewSkillInputs() SkillInputs {
	return SkillInputs{
		Required: make([]string, 0),
		Optional: make([]string, 0),
	}
}

type SkillOutputs struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

func NewSkillOutputs() SkillOutputs {
	return SkillOutputs{
		Required: make([]string, 0),
		Optional: make([]string, 0),
	}
}

type SkillToolPolicy struct {
	Allowed          []string `json:"allowed"`
	Forbidden        []string `json:"forbidden"`
	ApprovalRequired []string `json:"approvalRequired"`
}

func NewSkillToolPolicy() SkillToolPolicy {
	return SkillToolPolicy{
		Allowed:          make([]string, 0),
		Forbidden:        make([]string, 0),
		ApprovalRequired: make([]string, 0),
	}
}

type SkillGuardrails struct {
	MustNot []string `json:"mustNot"`
}

func NewSkillGuardrails() SkillGuardrails {
	return SkillGuardrails{
		MustNot: make([]string, 0),
	}
}

type SkillHumanApprovalPolicy struct {
	RequiredFor []string `json:"requiredFor"`
}

func NewSkillHumanApprovalPolicy() SkillHumanApprovalPolicy {
	return SkillHumanApprovalPolicy{
		RequiredFor: make([]string, 0),
	}
}

type SkillValidationPolicy struct {
	Checks []string `json:"checks"`
}

func NewSkillValidationPolicy() SkillValidationPolicy {
	return SkillValidationPolicy{
		Checks: make([]string, 0),
	}
}

type SkillWorkflow struct {
	Steps []SkillWorkflowStep `json:"steps"`
}

func NewSkillWorkflow() SkillWorkflow {
	return SkillWorkflow{
		Steps: make([]SkillWorkflowStep, 0),
	}
}

type SkillWorkflowStep struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Type        SkillWorkflowStepType `json:"type"`
	Description string                `json:"description"`
}

func NewSkillWorkflowStep() SkillWorkflowStep {
	return SkillWorkflowStep{
		Type: StepTypeReasoning,
	}
}

type SkillPackage struct {
	RootPath string            `json:"rootPath"`
	Manifest SkillManifest     `json:"manifest"`
	Files    map[string]string `json:"files"`
}

func NewSkillPackage() SkillPackage {
	return SkillPackage{
		Manifest: NewSkillManifest(),
		Files:    map[string]string{},
	}
}

type SkillPackageOptions struct {
	SkillsRoot   string `json:"skillsRoot"`
	PackagesRoot string `json:"packagesRoot"`
	Template     string `json:"template"`
	Force        bool   `json:"force"`
}

func NewSkillPackageOptions() SkillPackageOptions {
	return SkillPackageOptions{
		Template: "generic",
	}
}

type SkillValidationResult struct {
	SkillID string                 `json:"skillId"`
	Issues  []SkillValidationIssue `json:"issues"`
}

func (r SkillValidationResult) Passed() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return false
		}
	}
	return true
}

type SkillValidationIssue struct {
	Severity SkillValidationSeverity `json:"severity"`
	Area     string                  `json:"area"`
	Message  string                  `json:"message"`
	FileName *string                 `json:"fileName,omitempty"`
}

type SkillRunPlan struct {
	Manifest    SkillManifest          `json:"manifest"`
	Inputs      []string               `json:"inputs"`
	InputIssues []SkillValidationIssue `json:"inputIssues"`
}

type SkillCritiqueResult struct {
	Markdown string   `json:"markdown"`
	Findings []string `json:"findings"`
}

type CaseInsensitiveMap map[string]string

func NewCaseInsensitiveMap() CaseInsensitiveMap {
	return make(CaseInsensitiveMap)
}

func (m CaseInsensitiveMap) Set(key, value string) {
	m[strings.ToLower(key)] = value
}

func (m CaseInsensitiveMap) Get(key string) (string, bool) {
	val, ok := m[strings.ToLower(key)]
	return val, ok
}
