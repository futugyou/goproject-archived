package primitives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/futugyou/openclaw/core"
)

type ManagedSkillValidationScenario struct {
	BaseScenario
}

func NewManagedSkillValidationScenario() *ManagedSkillValidationScenario {
	s := &ManagedSkillValidationScenario{
		BaseScenario: NewBaseScenario("tools.managed_skill_validation",
			"Managed skill validation", HarnessRegressionCategoryTools, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *ManagedSkillValidationScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	skillRoot, err := HarnessRegressionPathsChild(regCtx.TempWorkspacePath, "managed-skill")
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	if err := os.MkdirAll(skillRoot, 0700); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(
		`
---
name: harness-regression-demo
description: Deterministic managed skill validation fixture.
metadata: {"always":true}
---
Use this deterministic skill for harness regression checks.`), 0644); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	inspection := core.SkillInspectorInstace.InspectPath(skillRoot, core.SkillSourceManaged)

	if inspection != nil &&
		inspection.Success &&
		inspection.Definition != nil &&
		inspection.Definition.Name == "harness-regression-demo" {
		return s.Passed("Managed SKILL.md draft parsed successfully.", fmt.Sprintf("Skill file: %s", inspection.SkillFilePath)), nil
	}

	msg := ""
	if inspection != nil {
		msg = inspection.ErrorMessage
	}
	return s.Failed("Managed SKILL.md draft validation failed.", "", msg), nil
}
