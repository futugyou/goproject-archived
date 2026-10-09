package primitives

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type HarnessRegressionDocsScenario struct {
	BaseScenario
}

func NewHarnessRegressionDocsScenario() *HarnessRegressionDocsScenario {
	s := &HarnessRegressionDocsScenario{
		BaseScenario: NewBaseScenario(
			"docs.harness_regression_docs",
			"Harness regression docs",
			HarnessRegressionCategoryDocs,
			true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *HarnessRegressionDocsScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	root := findRepositoryRoot()
	if root == "" {
		return s.NotApplicable("Repository docs tree was not found from this runtime location.", "Installed binaries may not include source documentation.", ""), nil
	}

	docPath := filepath.Join(root, "docs", "HARNESS_REGRESSION.md")
	indexPath := filepath.Join(root, "docs", "README.md")
	siteMapPath := filepath.Join(root, "docs", "SITE_MAP.md")

	var missing []string
	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		missing = append(missing, "docs/HARNESS_REGRESSION.md")
	}
	if content, err := os.ReadFile(indexPath); err != nil || !strings.Contains(string(content), "HARNESS_REGRESSION.md") {
		missing = append(missing, "docs/README.md link")
	}

	if content, err := os.ReadFile(siteMapPath); err != nil || !strings.Contains(string(content), "HARNESS_REGRESSION.md") {
		missing = append(missing, "docs/SITE_MAP.md link")
	}

	if len(missing) > 0 {
		return s.Failed("Harness regression documentation is missing or not indexed.", HarnessRegressionScenarioTextJoin(missing), ""), nil
	}
	return s.Passed("Harness regression documentation is present and indexed.", ""), nil
}

func findRepositoryRoot() string {
	starts := []string{}

	if pwd, err := os.Getwd(); err == nil {
		starts = append(starts, pwd)
	}

	if execPath, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(execPath))
	}

	for _, start := range starts {
		candidate := filepath.Clean(start)
		for {
			readmePath := filepath.Join(candidate, "README.md")
			docsPath := filepath.Join(candidate, "docs")

			if info, err := os.Stat(readmePath); err == nil && !info.IsDir() {
				if info, err := os.Stat(docsPath); err == nil && info.IsDir() {
					return candidate
				}
			}

			parent := filepath.Dir(candidate)
			if parent == candidate {
				break
			}
			candidate = parent
		}
	}

	return ""
}
