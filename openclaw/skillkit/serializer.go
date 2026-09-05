package skillkit

import "strings"

type StepBuilder struct {
	Id          string
	Name        string
	Type        string
	Description string
}

func (t *StepBuilder) Set(key, value string) {
	switch key {
	case "name":
		t.Name = value
	case "type":
		t.Type = value
	case "description":
		t.Description = value
	}
}

func (t *StepBuilder) ToStep() SkillWorkflowStep {
	return SkillWorkflowStep{
		Id:          t.Id,
		Name:        t.Name,
		Type:        ParseStepType(t.Type),
		Description: t.Description,
	}
}

func ParseStepType(value string) SkillWorkflowStepType {
	switch strings.ToLower(value) {
	case "input":
		return StepTypeInput

	case "generation":
		return StepTypeGeneration
	case "validation":
		return StepTypeValidation
	case "approval":
		return StepTypeApproval
	case "output":
		return StepTypeOutput
	default:
		return StepTypeReasoning
	}
}
