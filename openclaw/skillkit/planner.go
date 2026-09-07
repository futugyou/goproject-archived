package skillkit

import "github.com/futugyou/openclaw/util"

type SkillRunPlanner struct{}

func (s *SkillRunPlanner) Plan(pkg SkillPackage, inputPaths []string) SkillRunPlan {
	issues := []SkillValidationIssue{}
	for _, inputPath := range inputPaths {
		if util.FileExists(inputPath) {
			issues = append(issues, SkillValidationIssue{Severity: SeverityPass, Area: "Inputs", Message: inputPath + " exists.", FileName: inputPath})
		} else {
			issues = append(issues, SkillValidationIssue{Severity: SeverityError, Area: "Inputs", Message: inputPath + " is missing.", FileName: inputPath})
		}
	}

	return SkillRunPlan{
		Manifest:    pkg.Manifest,
		Inputs:      inputPaths,
		InputIssues: issues,
	}
}
