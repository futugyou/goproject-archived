package skillkit

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/util"
)

func ErrorSkillValidationIssue(area, message, fileName string) SkillValidationIssue {
	return SkillValidationIssue{
		Severity: SeverityError,
		Area:     area,
		Message:  message,
		FileName: fileName,
	}
}

func WarningSkillValidationIssue(area, message, fileName string) SkillValidationIssue {
	return SkillValidationIssue{
		Severity: SeverityWarning,
		Area:     area,
		Message:  message,
		FileName: fileName,
	}
}

func PassSkillValidationIssue(area, message, fileName string) SkillValidationIssue {
	return SkillValidationIssue{
		Severity: SeverityPass,
		Area:     area,
		Message:  message,
		FileName: fileName,
	}
}

func RequireList(issues []SkillValidationIssue, values []string, area, pass, failure, fileName string, emptySeverity SkillValidationSeverity) (resultissues []SkillValidationIssue) {
	if len(values) > 0 {
		resultissues = append(issues, PassSkillValidationIssue(area, pass, fileName))
		return
	}

	if emptySeverity == SeverityWarning {
		resultissues = append(issues, WarningSkillValidationIssue(area, failure, fileName))
	} else {
		resultissues = append(issues, ErrorSkillValidationIssue(area, failure, fileName))
	}

	return
}

func RequireText(issues []SkillValidationIssue, value, area, pass, error, fileName string) (resultissues []SkillValidationIssue) {
	if value == "" {
		resultissues = append(issues, ErrorSkillValidationIssue(area, error, fileName))
	} else {
		resultissues = append(issues, PassSkillValidationIssue(area, pass, fileName))
	}
	return
}

type SkillValidator struct{}

func (s *SkillValidator) Validate(ctx context.Context, skillRef, skillsRoot string) (*SkillValidationResult, error) {
	root, err := ResolveSkillPath(skillRef, skillsRoot)
	if err != nil {
		return nil, err
	}

	manifestPath, err := ResolvePackageFilePath(root, "skill.yaml")
	if err != nil {
		return nil, err
	}

	issues := []SkillValidationIssue{}
	if !util.FileExists(manifestPath) {
		return &SkillValidationResult{SkillId: filepath.Base(root), Issues: []SkillValidationIssue{ErrorSkillValidationIssue("Files", "skill.yaml is missing.", "skill.yaml")}}, nil
	}

	manifest, err := SkillManifestRead(ctx, manifestPath)
	if err != nil {
		return &SkillValidationResult{SkillId: filepath.Base(root), Issues: []SkillValidationIssue{ErrorSkillValidationIssue("Files", "skill.yaml could not be read", "skill.yaml")}}, nil
	}

	issues = append(issues, PassSkillValidationIssue("Files", "skill.yaml exists.", "skill.yaml"))

	for _, file := range SkillTemplateRequiredFiles {
		if file == "skill.yaml" {
			continue
		}
		f, err := ResolvePackageFilePath(root, file)
		if err == nil && util.FileExists(f) {
			issues = append(issues, PassSkillValidationIssue("Files", file+" exists.", file))
		} else {
			issues = append(issues, ErrorSkillValidationIssue("Files", file+" is missing.", file))
		}
	}

	var folderName = filepath.Base(root)
	if folderName != manifest.Id && !slices.Contains(manifest.Aliases, folderName) {
		issues = append(issues, ErrorSkillValidationIssue("Manifest", fmt.Sprintf("Manifest id '%s' does not match folder '%s' or a declared alias.", manifest.Id, folderName), "skill.yaml"))
	} else {
		issues = append(issues, PassSkillValidationIssue("Manifest", "Manifest id matches folder or alias.", "skill.yaml"))
	}

	issues = RequireText(issues, manifest.Name, "Manifest", "Name is present.", "Name is missing.", "skill.yaml")
	issues = RequireText(issues, manifest.Version, "Manifest", "Version is present.", "Version is missing.", "skill.yaml")
	issues = RequireText(issues, manifest.Category, "Manifest", "Category is present.", "Category is missing.", "skill.yaml")
	issues = RequireText(issues, manifest.Intent.Outcome, "Intent", "Intent outcome is present.", "Intent outcome is missing.", "skill.yaml")
	issues = RequireList(issues, manifest.Inputs.Required, "Policy", "Required inputs defined.", "At least one required input is required.", "skill.yaml", SeverityError)
	issues = RequireList(issues, manifest.Outputs.Required, "Policy", "Required outputs defined.", "At least one required output is required.", "skill.yaml", SeverityError)
	issues = RequireList(issues, manifest.Validation.Checks, "Policy", "Validation checks defined.", "Validation checks are empty.", "skill.yaml", SeverityWarning)
	issues = RequireList(issues, manifest.Guardrails.MustNot, "Policy", "Guardrails defined.", "Guardrails are empty.", "skill.yaml", SeverityError)
	issues = RequireList(issues, manifest.HumanApproval.RequiredFor, "Policy", "Human approval policy defined.", "Human approval policy is empty.", "skill.yaml", SeverityWarning)

	if len(manifest.Workflow.Steps) == 0 {
		issues = append(issues, ErrorSkillValidationIssue("Workflow", "Workflow must contain at least one step.", "workflow.yaml"))
	} else {
		issues = append(issues, ErrorSkillValidationIssue("Workflow", "Workflow has at least one step.", "workflow.yaml"))
	}

	allowedForbiddenOverlap := util.IntersectIgnoreCase(manifest.Tools.Allowed, manifest.Tools.Forbidden)
	approvalForbiddenOverlap := util.IntersectIgnoreCase(manifest.Tools.ApprovalRequired, manifest.Tools.Forbidden)
	if len(allowedForbiddenOverlap) > 0 {
		issues = append(issues, ErrorSkillValidationIssue("Policy", fmt.Sprintf("Allowed and forbidden tools overlap: '%s' ", strings.Join(allowedForbiddenOverlap, ", ")), "tools.yaml"))

	} else if len(approvalForbiddenOverlap) > 0 {
		issues = append(issues, ErrorSkillValidationIssue("Policy", fmt.Sprintf("Approval-required tools cannot be forbidden: '%s' ", strings.Join(approvalForbiddenOverlap, ", ")), "tools.yaml"))
	} else {
		issues = append(issues, ErrorSkillValidationIssue("Policy", "Tool policy has no conflicts.", "tools.yaml"))
	}

	return &SkillValidationResult{SkillId: manifest.Id, Issues: issues}, nil
}
