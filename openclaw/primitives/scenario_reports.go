package primitives

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/futugyou/openclaw/util"
)

func ScenarioMarkdownReportBuild(report ScenarioRunReport) string {
	var sb strings.Builder

	sb.WriteString("# Agent Scenario Test Report\n\n")
	fmt.Fprintf(&sb, "- Run Id: %s\n", inlineCode(report.RunId))
	fmt.Fprintf(&sb, "- Started: `%s`\n", report.StartedAtUtc.Format(time.RFC3339Nano))
	fmt.Fprintf(&sb, "- Completed: `%s`\n", report.CompletedAtUtc.Format(time.RFC3339Nano))
	fmt.Fprintf(&sb, "- Summary: %d/%d passed, %d failed\n", report.Summary.Passed, report.Summary.Total, report.Summary.Failed)
	fmt.Fprintf(&sb, "- High/Critical failures: %d\n\n", report.Summary.HighOrCriticalFailures)

	sb.WriteString("| Scenario | Risk | Status | Failed Oracles |\n")
	sb.WriteString("| --- | --- | --- | --- |\n")

	resultsCopy := make([]ScenarioRunResult, len(report.Results))
	copy(resultsCopy, report.Results)
	sort.Slice(resultsCopy, func(i, j int) bool {
		return strings.EqualFold(resultsCopy[i].Scenario.Id, resultsCopy[j].Scenario.Id) &&
			resultsCopy[i].Scenario.Id < resultsCopy[j].Scenario.Id
	})

	for _, result := range resultsCopy {
		var failed []string
		for _, oracle := range result.OracleResults {
			if !oracle.Passed {
				failed = append(failed, oracle.Name)
			}
		}

		status := "failed"
		if result.Passed {
			status = "passed"
		}

		failedStr := ""
		if len(failed) > 0 {
			failedStr = strings.Join(failed, ", ")
		}

		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n",
			escapeTableCell(inlineCode(result.Scenario.Id)),
			result.Scenario.Risk,
			status,
			escapeTableCell(failedStr))
	}

	for _, result := range report.Results {
		if result.Passed {
			continue
		}

		sb.WriteString("\n")
		fmt.Fprintf(&sb, "## %s\n\n", escapeHeading(result.Scenario.Id))
		fmt.Fprintf(&sb, "%s\n\n", escapeParagraph(result.Scenario.Title))

		if !util.IsBlankP(result.FailureSummary) {
			fmt.Fprintf(&sb, "%s\n\n", escapeParagraph(*result.FailureSummary))
		}

		for _, oracle := range result.OracleResults {
			if oracle.Passed {
				continue
			}

			sb.WriteString("\n")
			fmt.Fprintf(&sb, "- %s: %s\n", inlineCode(oracle.Name), escapeParagraph(oracle.Message))
			for _, evidence := range oracle.Evidence {
				fmt.Fprintf(&sb, "  - %s\n", inlineCode(evidence))
			}
		}
	}

	return sb.String()
}

func inlineCode(value string) string {
	normalized := normalizeInline(value)
	delimiter := strings.Repeat("`", maxBacktickRun(normalized)+1)
	padding := ""
	if strings.HasPrefix(normalized, "`") || strings.HasSuffix(normalized, "`") {
		padding = " "
	}
	return fmt.Sprintf("%s%s%s%s%s", delimiter, padding, normalized, padding, delimiter)
}

func escapeTableCell(value string) string {
	return strings.ReplaceAll(normalizeInline(value), "|", "\\|")
}

func escapeHeading(value string) string {
	v := normalizeInline(value)
	v = strings.ReplaceAll(v, "#", "\\#")
	v = strings.ReplaceAll(v, "`", "\\`")
	return v
}

func escapeParagraph(value string) string {
	return normalizeInline(value)
}

func normalizeInline(value string) string {
	v := strings.ReplaceAll(value, "\r\n", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "\r", " ")
	return v
}

func maxBacktickRun(value string) int {
	max := 0
	current := 0
	for _, ch := range value {
		if ch == '`' {
			current++
			if current > max {
				max = current
			}
		} else {
			current = 0
		}
	}
	return max
}

type JsonTraceWriter struct {
}

func (j *JsonTraceWriter) Write(ctx context.Context, report ScenarioRunReport, outputRoot string) (*TraceWriteResult, error) {
	if err := validateRunId(report.RunId); err != nil {
		return nil, err
	}

	outputRootFullPath, err := filepath.Abs(outputRoot)
	if err != nil {
		return nil, err
	}
	runDirectory := filepath.Join(outputRootFullPath, report.RunId)
	traceDirectory := filepath.Join(runDirectory, "traces")
	if err := os.MkdirAll(traceDirectory, 0700); err != nil {
		return nil, err
	}

	tracePaths := []string{}
	for _, result := range report.Results {
		traceFileName, err := getTraceFileName(result.Scenario.Id)
		if err != nil {
			return nil, err
		}
		tracePath := filepath.Join(traceDirectory, traceFileName)
		data, err := json.Marshal(result.Trace)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(tracePath, data, 0644); err != nil {
			return nil, err
		}

		tracePaths = append(tracePaths, tracePath)
	}

	reportWithMarkdown := report
	if util.IsBlank(report.Markdown) {
		reportWithMarkdown = ScenarioRunReport{
			RunId:          report.RunId,
			StartedAtUtc:   report.StartedAtUtc,
			CompletedAtUtc: report.CompletedAtUtc,
			Summary:        report.Summary,
			Results:        report.Results,
			Markdown:       ScenarioMarkdownReportBuild(report),
		}

	}

	resultsPath := filepath.Join(runDirectory, "results.json")
	reportPath := filepath.Join(runDirectory, "report.md")
	data, err := json.Marshal(reportWithMarkdown)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(resultsPath, data, 0644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(reportPath, []byte(reportWithMarkdown.Markdown), 0644); err != nil {
		return nil, err
	}

	return &TraceWriteResult{
		RunId:         report.RunId,
		DirectoryPath: runDirectory,
		ResultsPath:   resultsPath,
		ReportPath:    reportPath,
		TracePaths:    tracePaths,
	}, nil
}

func validateRunId(runId string) error {
	trimmed := strings.TrimSpace(runId)
	if trimmed == "" || filepath.IsAbs(runId) || runId != filepath.Base(runId) || runId == "." || runId == ".." {
		return &os.PathError{Op: "ValidateRunId", Path: runId, Err: os.ErrInvalid}
	}
	return nil
}

func getTraceFileName(scenarioId string) (string, error) {
	safeName := safeFileName(scenarioId)
	traceFileName := filepath.Base(safeName + ".trace.json")
	if strings.TrimSpace(traceFileName) == "" || filepath.IsAbs(traceFileName) || traceFileName == "." || traceFileName == ".." {
		return "", &os.PathError{Op: "GetTraceFileName", Path: scenarioId, Err: os.ErrInvalid}
	}
	return traceFileName, nil
}

func safeFileName(value string) string {
	if strings.TrimSpace(value) == "" {
		return "scenario"
	}

	// Windows invalid file name characters + general cross-platform safety
	invalidChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	fileName := value
	for _, char := range invalidChars {
		fileName = strings.ReplaceAll(fileName, char, "_")
	}

	// Also handle control characters or replace generic path separators if needed
	return fileName
}
