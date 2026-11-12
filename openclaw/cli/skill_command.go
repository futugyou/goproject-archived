package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type SkillCommandInspection struct {
	Success       bool
	ErrorMessage  string
	Definition    core.SkillDefinition
	SkillRootPath string
	SkillFilePath string
	InstallSlug   string
	SourceLabel   string
	TrustLevel    string
	TrustReason   string
	Warnings      []string
}

func SkillCommandInspectionFailure(errorMessage string) SkillCommandInspection {
	return SkillCommandInspection{
		ErrorMessage: errorMessage,
		Definition: core.SkillDefinition{
			Source: core.SkillSourceExtra,
		},
		TrustLevel: "untrusted",
	}
}

const DefaultMetaRunProposalRollbackReason = "meta_run_proposal_operator_rollback"

func writeSkillsCommandError(asJson bool, command, errorCode, message string) {
	if !asJson {
		fmt.Println(message)
		return
	}

	data := map[string]string{}
	data["status"] = "error"
	data["command"] = command
	data["errorCode"] = errorCode
	data["message"] = message
	msg, _ := json.Marshal(data)

	fmt.Println([]byte(msg))
}

func skillCommandUnknownSubcommand(subcommand string, asJson bool) (int, error) {
	msg := fmt.Sprintf("Unknown skills subcommand: %s", subcommand)
	writeSkillsCommandError(asJson, "skills", "unknown_subcommand", msg)
	if !asJson {
		skillCommandPrintHelp()
	}
	return 2, errors.New(msg)
}

func skillCommandPrintHelp() {
	fmt.Println(`
openclaw skills — Inspect and install local OpenClaw skill packages

            Usage:
              openclaw skills inspect <path|tarball>
              openclaw skills install <path|tarball> [--dry-run] [--workdir <path> | --managed]
              openclaw skills list [--workdir <path> | --managed]
              openclaw skills catalog [--workdir <path> | --managed] [--kind <all|meta>] [--stable] [--json]
              openclaw skills create <name> [--kind <standard|meta>] [--description <text>] [--proposal-draft] [--workdir <path> | --managed] [--json] [--force]
              openclaw skills proposals <session-id> [--run <run-id>] [--storage <path>] [--json]
              openclaw skills proposals show <session-id> --proposal <id> [--storage <path>] [--json]
                            openclaw skills meta-runs list [--page <n>] [--page-size <n>] [--storage <path>] [--json]
                            openclaw skills meta-runs show <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs steps <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs failures [--since <5m|24h|7d>] [--name <skill>] [--limit <n>] [--storage <path>] [--json]
                            openclaw skills meta-runs <session-id> [--storage <path>] [--limit <count>] [--run <run-id>] [--verbose] [--json]
                            openclaw skills meta-runs replay <session-id> --run <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs reconstruct <session-id> --run <run-id> [--storage <path>] [--json]
                            openclaw skills meta-runs proposals <session-id> [--run <run-id>] [--storage <path>] [--json]
                            openclaw skills meta-runs proposals show <session-id> --proposal <id> [--storage <path>] [--json]
                            openclaw skills meta-runs proposals accept <session-id> --proposal <id> [--storage <path>] [--json]
                            openclaw skills meta-runs proposals dismiss <session-id> --proposal <id> [--reason <text>] [--storage <path>] [--json]
                            openclaw skills meta-runs proposals rollback <session-id> --proposal <id> [--reason <text>] [--storage <path>] [--json]
                            openclaw skills meta-runs proposals change <session-id> --proposal <id> --to <accept|dismiss> [--reason <text>] [--storage <path>] [--json]

            Notes:
              - Remote registry installs still go through "openclaw clawhub".
              - "install --dry-run" prints trust and requirement details without copying files.
                            - "meta-runs" reads persisted local session state from "./memory", " OPENCLAW_WORKSPACE/memory", or "--storage".
                            - "meta-runs --run <run-id>" limits output to one persisted run inside the session history.
                            - "meta-runs --verbose" expands per-step trace summaries for each persisted run.
                            - "meta-runs --json" emits machine-readable output for operators and scripts.
                            - "meta-runs steps" shows persisted per-step execution details for one run id.
                            - "meta-runs failures" supports "--since" and "--name" filters for operator triage windows.
                            - "meta-runs replay" is currently preview-only and reports whether persisted run history is sufficient for replay.
                                - "meta-runs reconstruct" builds an audit replay result from persisted run history and optional checkpoint state without re-executing tools or models.
                                - "meta-runs proposals" returns derived read-only proposal summaries from persisted meta-run evidence.
                                - "skills proposals" is a read-only shortcut to "skills meta-runs proposals" / "show".
                                - "skills catalog" lists installed skills for discovery and supports "--kind meta" filtering.
                                - "skills catalog --stable --kind meta" limits discovery to bundled first-party meta skills.
                                - "meta-runs proposals show" expands a single derived proposal without implying durable lifecycle state.
                                - "meta-runs proposals accept|dismiss|rollback|change" records lifecycle decisions only; it does not execute tools, models, or replay.`)
}

func SkillCommandRun(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		skillCommandPrintHelp()
		return 0, nil
	}

	subcommand := args[0]
	rest := []string{}
	if len(args) > 1 {
		rest = args[1:]
	}

	switch subcommand {
	case "inspect":
		return skillCommandInspect(rest)
	case "install":
		return skillCommandInstall(rest)
	case "list", "ls":
		return skillCommandListInstalled(rest)
	case "catalog":
		return skillCommandListCatalog(rest)
	case "create":
		return skillCommandCreateSkillScaffold(rest)
	case "proposals":
		return skillCommandListReadOnlyProposalsEntry(rest)
	case "meta-runs":
		return skillCommandListMetaRuns(rest)
	default:
		return skillCommandUnknownSubcommand(subcommand, asJson)
	}
}

func writeMetaRunsJson(sessionId string, totalCount int, runs []core.SessionMetaRunRecord) string {
	type innerDTO struct {
		SessionId  string                      `json:"sessionId"`
		TotalCount int                         `json:"totalCount"`
		ShownCount int                         `json:"shownCount"`
		Runs       []core.SessionMetaRunRecord `json:"runs"`
	}

	dto := innerDTO{
		SessionId:  sessionId,
		TotalCount: totalCount,
		ShownCount: len(runs),
		Runs:       runs,
	}
	data, _ := json.Marshal(dto)
	msg := string(data)
	fmt.Println(msg)
	return msg
}

func skillCommandListMetaRuns(args []string) (int, error) {
	if len(args) > 0 && args[0] == "list" {
		return skillCommandListMetaRunsGlobal(args[1:])
	}
	if len(args) > 0 && args[0] == "show" {
		return skillCommandShowMetaRunGlobal(args[1:])
	}
	if len(args) > 0 && args[0] == "steps" {
		return skillCommandShowMetaRunStepsGlobal(args[1:])
	}
	if len(args) > 0 && args[0] == "failures" {
		return skillCommandListMetaRunFailuresGlobal(args[1:])
	}
	if len(args) > 0 && args[0] == "replay" {
		return skillCommandPreviewMetaRunReplay(args[1:])
	}

	asJson := slices.Contains(args, "--json")
	verbose := slices.Contains(args, "--verbose")
	requestedRunId := getOptionValue(args, "--run")
	sessionId := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sessionId = args[i]
			break
		}
	}

	if sessionId == "" {
		msg := "Usage: openclaw skills meta-runs <session-id> [--storage <path>] [--limit <count>] [--run <run-id>] [--verbose] [--json]"
		writeSkillsCommandError(asJson, "skills meta-runs", "missing_session_id", msg)
		return 2, errors.New(msg)
	}

	storagePath := getOptionValue(args, "--storage")
	limit := parseIntOption(args, "--limit", 20)
	limit = util.Clamp(limit, 1, 100)

	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	ctx := context.Background()
	session, err := store.GetSession(ctx, sessionId)
	if session == nil || err != nil {
		msg := fmt.Sprintf("Session '%s' not found.", sessionId)
		writeSkillsCommandError(asJson, "skills meta-runs", "session_not_found", msg)
		return 1, errors.New(msg)
	}

	if len(session.MetaRunHistory) == 0 {
		if asJson {
			writeMetaRunsJson(sessionId, len(session.MetaRunHistory), nil)
			return 0, nil
		}

		fmt.Printf("Session: %s\n", sessionId)
		fmt.Println("Meta runs: 0")
		return 0, nil
	}

	runs := []core.SessionMetaRunRecord{}
	for i := int64(0); i < limit; i++ {
		run := session.MetaRunHistory[i]
		if requestedRunId == "" || run.RunId == requestedRunId {
			runs = append(runs, run)
		}
	}

	slices.SortFunc(runs, func(a, b core.SessionMetaRunRecord) int {
		return b.StartedAtUtc.Compare(a.CompletedAtUtc)
	})

	if requestedRunId != "" && len(runs) == 0 {
		msg := fmt.Sprintf("Run '%s' not found in session '%s'.", requestedRunId, sessionId)
		writeSkillsCommandError(asJson, "skills meta-runs", "run_not_found", msg)
		return 1, errors.New(msg)
	}

	if asJson {
		writeMetaRunsJson(sessionId, len(session.MetaRunHistory), runs)
		return 0, nil
	}

	fmt.Printf("Session: %s\n", sessionId)
	fmt.Printf("Meta runs: %d total, showing %d\n", len(session.MetaRunHistory), len(runs))
	fmt.Println("")

	for i := 0; i < len(runs); i++ {
		run := runs[i]
		fmt.Printf("Run: %s\n", run.RunId)
		fmt.Printf("Skill: %s\n", run.SkillName)
		fmt.Printf("Status:  %s\n", run.Status)
		fmt.Printf("Started: %s\n", run.StartedAtUtc.Format(time.RFC3339Nano))
		fmt.Printf("Completed: %s\n", run.CompletedAtUtc.Format(time.RFC3339Nano))
		fmt.Printf("Steps: %d\n", len(run.StepResults))
		if run.ErrorCode != "" {
			fmt.Printf("Error code: %s\n", run.ErrorCode)
		}
		if run.Error != "" {
			fmt.Printf("Error: %s\n", run.Error)
		}
		if run.FinalText != "" {
			fmt.Printf("Final text: %s\n", run.FinalText)
		}

		if verbose {
			for _, step := range run.StepResults {
				line := fmt.Sprintf("- Step: %s | kind=%s | status=%s | duration_ms=%.3f", step.Id, step.Kind, step.Status, step.DurationMs)
				if step.FailureCode != "" {
					line += fmt.Sprintf(" | failure_code=%s", step.FailureCode)
				}
				if step.Continued {
					line += " | continued=true"
				}
				fmt.Println(line)
			}

			fmt.Println("")
		} else {
			fmt.Println("")
		}
	}

	return 0, nil
}

func getReplayMissingRequirements(run *core.SessionMetaRunRecord) []core.MetaRunReplayRequirementPreview {
	var requirements = make([]core.MetaRunReplayRequirementPreview, 0)
	requirements = append(requirements, core.MetaRunReplayRequirementPreview{
		Name:   core.MetaRunReplayRequirementNamesPromptContext,
		Kind:   core.MetaRunReplayRequirementKindsNotPersisted,
		Reason: core.MetaRunReplayRequirementReasonsPromptContextNotPersisted,
	})
	requirements = append(requirements, core.MetaRunReplayRequirementPreview{
		Name:   core.MetaRunReplayRequirementNamesStepInputs,
		Kind:   core.MetaRunReplayRequirementKindsNotPersisted,
		Reason: core.MetaRunReplayRequirementReasonsStepInputsNotPersisted,
	})
	requirements = append(requirements, core.MetaRunReplayRequirementPreview{
		Name:   core.MetaRunReplayRequirementNamesToolArguments,
		Kind:   core.MetaRunReplayRequirementKindsNotPersisted,
		Reason: core.MetaRunReplayRequirementReasonsToolArgumentsNotPersisted,
	})

	for _, step := range run.StepResults {
		if step.Kind == "skill_exec" && step.ExecutionEvidence != nil {
			requirements = append(requirements, core.MetaRunReplayRequirementPreview{
				Name:   core.MetaRunReplayRequirementNamesSkillExecInputs,
				Kind:   core.MetaRunReplayRequirementKindsNotPersisted,
				Reason: core.MetaRunReplayRequirementReasonsSkillExecInputsNotPersisted,
			})
			break
		}
	}

	if !hasRetainedSteps(run) {
		requirements = append(requirements, core.MetaRunReplayRequirementPreview{
			Name:   core.MetaRunReplayRequirementNamesStepResults,
			Kind:   core.MetaRunReplayRequirementKindsNotPersisted,
			Reason: core.MetaRunReplayRequirementReasonsStepResultsNotRetained,
		})
	}

	return requirements
}

func buildReplayPreview(sessionId string, run *core.SessionMetaRunRecord) core.MetaRunReplayPreviewResponse {
	var missingRequirements = getReplayMissingRequirements(run)
	var operatorSummary = buildReplayOperatorSummary(run)
	return core.MetaRunReplayPreviewResponse{
		SessionId:           sessionId,
		RunId:               run.RunId,
		SkillName:           run.SkillName,
		Reason:              "Persisted meta run history does not yet include enough inputs to build an executable replay plan.",
		AvailableArtifacts:  getReplayAvailableArtifacts(run),
		RetainedSteps:       getReplayRetainedSteps(run),
		Plan:                getReplayPlan(run, missingRequirements),
		MissingRequirements: missingRequirements,
		OperatorSummary:     operatorSummary,
		TriageHints:         buildReplayTriageHints(run, missingRequirements, operatorSummary),
	}
}

func buildReplayTriageHints(run *core.SessionMetaRunRecord, missingRequirements []core.MetaRunReplayRequirementPreview, summary *core.MetaRunReplayOperatorSummary) []core.MetaRunReplayTriageHint {
	var hints []core.MetaRunReplayTriageHint

	missingSkillExecInputs := false
	for _, req := range missingRequirements {
		if req.Name == core.MetaRunReplayRequirementNamesSkillExecInputs {
			missingSkillExecInputs = true
			break
		}
	}

	if missingSkillExecInputs {
		var stepIds []string
		for _, step := range run.StepResults {
			if strings.EqualFold(step.Kind, "skill_exec") && step.ExecutionEvidence == nil {
				stepIds = append(stepIds, step.Id)
			}
		}
		hints = append(hints, core.MetaRunReplayTriageHint{
			Code:             core.MetaRunReplayTriageHintCodesSkillExecInputsNotPersisted,
			Priority:         1,
			Message:          "Persist skill_exec execution evidence so replay can include stdin and parse diagnostics.",
			StepIds:          stepIds,
			RequirementNames: []string{core.MetaRunReplayRequirementNamesSkillExecInputs},
		})
	}

	var parseModeAnomalyStepIds []string
	for _, step := range run.StepResults {
		if strings.EqualFold(step.Kind, "skill_exec") && step.ExecutionEvidence != nil {
			pm := step.ExecutionEvidence.ParseMode
			if !strings.EqualFold(pm, "text") && !strings.EqualFold(pm, "json") {
				parseModeAnomalyStepIds = append(parseModeAnomalyStepIds, step.Id)
			}
		}
	}
	if len(parseModeAnomalyStepIds) > 0 {
		hints = append(hints, core.MetaRunReplayTriageHint{
			Code:     core.MetaRunReplayTriageHintCodesSkillExecParseModeAnomaly,
			Priority: 2,
			Message:  "Found skill_exec parse_mode values outside supported text/json set; validate skill definitions and evidence snapshots.",
			StepIds:  parseModeAnomalyStepIds,
		})
	}

	var likelyTruncatedCommandStepIds []string
	for _, step := range run.StepResults {
		if strings.EqualFold(step.Kind, "skill_exec") && step.ExecutionEvidence != nil {
			if isLikelyTruncatedCommandPreview(step.ExecutionEvidence.CommandPreview) {
				likelyTruncatedCommandStepIds = append(likelyTruncatedCommandStepIds, step.Id)
			}
		}
	}
	if len(likelyTruncatedCommandStepIds) > 0 {
		hints = append(hints, core.MetaRunReplayTriageHint{
			Code:     core.MetaRunReplayTriageHintCodesSkillExecCommandPreviewTruncated,
			Priority: 3,
			Message:  "Command preview appears truncated; check original skill_exec args for full command context.",
			StepIds:  likelyTruncatedCommandStepIds,
		})
	}

	// 4. Check dominant failure cluster
	if len(summary.FailureClusters) > 0 {
		dominantFailure := summary.FailureClusters[0]
		var stepIds []string
		for _, step := range run.StepResults {
			if strings.EqualFold(step.FailureCode, dominantFailure.Name) {
				stepIds = append(stepIds, step.Id)
			}
		}
		hints = append(hints, core.MetaRunReplayTriageHint{
			Code:     core.MetaRunReplayTriageHintCodesDominantFailureCluster,
			Priority: 4,
			Message:  fmt.Sprintf("Dominant failure cluster is '%s' across %d step(s); prioritize this class first.", dominantFailure.Name, dominantFailure.Count),
			StepIds:  stepIds,
		})
	}

	// Sort hints by Priority ascending, then Code case-insensitively
	sort.Slice(hints, func(i, j int) bool {
		if hints[i].Priority != hints[j].Priority {
			return hints[i].Priority < hints[j].Priority
		}
		return strings.Compare(strings.ToLower(hints[i].Code), strings.ToLower(hints[j].Code)) < 0
	})

	return hints
}

func isLikelyTruncatedCommandPreview(commandPreview string) bool {
	if commandPreview == "" {
		return false
	}

	return strings.HasSuffix(commandPreview, "...") || strings.HasSuffix(commandPreview, "…")
}

func getReplayPlan(run *core.SessionMetaRunRecord, missingRequirements []core.MetaRunReplayRequirementPreview) *core.MetaRunReplayPlanPreview {
	steps := []core.MetaRunReplayStepReadinessPreview{}
	for _, v := range run.StepResults {
		steps = append(steps, getReplayStepReadiness(v))
	}
	return &core.MetaRunReplayPlanPreview{
		Summary:               getReplayPlanSummary(run),
		Mode:                  core.MetaRunReplayModesPreviewOnly,
		ReplayableSteps:       steps,
		BlockedByRequirements: missingRequirements,
	}
}

func getReplayPlanSummary(run *core.SessionMetaRunRecord) string {
	if hasRetainedSteps(run) {
		return core.MetaRunReplayPlanSummariesAuditableNotReplayable
	}

	return core.MetaRunReplayPlanSummariesMetadataOnlyNotReplayable
}

func getReplayStepReadiness(step core.SessionMetaStepResult) core.MetaRunReplayStepReadinessPreview {
	if (step.Status == "failed" || step.FailureCode != "") && step.Continued {
		return core.MetaRunReplayStepReadinessPreview{
			Id:        step.Id,
			Readiness: core.MetaRunReplayStepReadinessKindsFailureTraceContinued,
			Reason:    core.MetaRunReplayStepReadinessReasonsFailureTraceContinued,
		}
	}

	if step.Status == "failed" || step.FailureCode != "" {
		return core.MetaRunReplayStepReadinessPreview{
			Id:        step.Id,
			Readiness: core.MetaRunReplayStepReadinessKindsFailureTraceOnly,
			Reason:    core.MetaRunReplayStepReadinessReasonsFailureTraceOnly,
		}
	}

	if step.Continued {
		return core.MetaRunReplayStepReadinessPreview{
			Id:        step.Id,
			Readiness: core.MetaRunReplayStepReadinessKindsContinuationTraceOnly,
			Reason:    core.MetaRunReplayStepReadinessReasonsContinuationTraceOnly,
		}
	}

	return core.MetaRunReplayStepReadinessPreview{
		Id:        step.Id,
		Readiness: core.MetaRunReplayStepReadinessKindsTraceOnly,
		Reason:    core.MetaRunReplayStepReadinessReasonsTraceOnly,
	}
}

func getReplayRetainedSteps(run *core.SessionMetaRunRecord) []core.MetaRunReplayStepPreview {
	result := []core.MetaRunReplayStepPreview{}
	for _, step := range run.StepResults {
		result = append(result, core.MetaRunReplayStepPreview{
			Id:          step.Id,
			Kind:        step.Kind,
			Status:      step.Status,
			FailureCode: step.FailureCode,
			DurationMs:  step.DurationMs,
			Continued:   step.Continued,
		})
	}
	return result
}
func getReplayAvailableArtifacts(run *core.SessionMetaRunRecord) []string {
	artifacts := []string{}
	if run.FinalText != "" {
		artifacts = append(artifacts, core.MetaRunReplayArtifactNamesFinalText)
	}
	if run.ErrorCode != "" {
		artifacts = append(artifacts, core.MetaRunReplayArtifactNamesErrorCode)
	}
	if run.Error != "" {
		artifacts = append(artifacts, core.MetaRunReplayArtifactNamesErrorMessage)
	}
	if hasRetainedSteps(run) {
		artifacts = append(artifacts, core.MetaRunReplayArtifactNamesStepResults)
	}

	return artifacts
}

func hasRetainedSteps(run *core.SessionMetaRunRecord) bool {
	return len(run.StepResults) > 0
}

func buildReplayOperatorSummary(run *core.SessionMetaRunRecord) *core.MetaRunReplayOperatorSummary {
	summary := &core.MetaRunReplayOperatorSummary{
		TotalSteps: len(run.StepResults),
	}

	kindMap := make(map[string]int)
	kindDisplay := make(map[string]string)
	failureMap := make(map[string]int)

	for _, step := range run.StepResults {
		isFailed := strings.EqualFold(step.Status, "failed") || strings.TrimSpace(step.FailureCode) != ""
		if isFailed {
			summary.FailedSteps++
		}

		if step.Continued {
			summary.ContinuedSteps++
		}

		if strings.EqualFold(step.Kind, "skill_exec") {
			summary.SkillExecSteps++
			if step.ExecutionEvidence == nil {
				summary.SkillExecStepsWithoutEvidence++
			}
		}

		if step.Kind != "" {
			lowerKind := strings.ToLower(step.Kind)
			if _, exists := kindMap[lowerKind]; !exists {
				kindDisplay[lowerKind] = step.Kind
			}
			kindMap[lowerKind]++
		}

		if strings.TrimSpace(step.FailureCode) != "" {
			failureMap[step.FailureCode]++
		}
	}

	stepKinds := make([]core.MetaRunReplayCountBucket, 0, len(kindMap))
	for lowerKind, count := range kindMap {
		stepKinds = append(stepKinds, core.MetaRunReplayCountBucket{
			Name:  kindDisplay[lowerKind],
			Count: count,
		})
	}
	sort.Slice(stepKinds, func(i, j int) bool {
		if stepKinds[i].Count != stepKinds[j].Count {
			return stepKinds[i].Count > stepKinds[j].Count
		}
		return strings.Compare(strings.ToLower(stepKinds[i].Name), strings.ToLower(stepKinds[j].Name)) < 0
	})
	summary.StepKinds = stepKinds

	failureClusters := make([]core.MetaRunReplayCountBucket, 0, len(failureMap))
	for code, count := range failureMap {
		failureClusters = append(failureClusters, core.MetaRunReplayCountBucket{
			Name:  code,
			Count: count,
		})
	}
	sort.Slice(failureClusters, func(i, j int) bool {
		if failureClusters[i].Count != failureClusters[j].Count {
			return failureClusters[i].Count > failureClusters[j].Count
		}
		return strings.Compare(strings.ToLower(failureClusters[i].Name), strings.ToLower(failureClusters[j].Name)) < 0
	})
	summary.FailureClusters = failureClusters

	return summary
}

func formatReplayRequirement(requirement core.MetaRunReplayRequirementPreview) string {
	return fmt.Sprintf("%s | kind=%s | reason=%s", requirement.Name, requirement.Kind, requirement.Reason)
}

func writeReplayOperatorDiagnosticsText(summary *core.MetaRunReplayOperatorSummary, triageHints []core.MetaRunReplayTriageHint) {
	fmt.Println("Operator summary:")
	fmt.Printf("Total steps: %d\n", summary.TotalSteps)
	fmt.Printf("Failed steps: %d\n", summary.FailedSteps)
	fmt.Printf("Continued steps: %d\n", summary.ContinuedSteps)
	fmt.Printf("Skill exec steps: %d\n", summary.SkillExecSteps)
	fmt.Printf("Skill exec without evidence: %d\n", summary.SkillExecStepsWithoutEvidence)
	if len(summary.StepKinds) > 0 {
		msgs := []string{}
		for i := range summary.StepKinds {
			msgs = append(msgs, fmt.Sprintf("%s=%d", summary.StepKinds[i].Name, summary.StepKinds[i].Count))
		}
		fmt.Printf("Step kind mix: %s", strings.Join(msgs, ","))
	}

	if len(summary.FailureClusters) > 0 {
		msgs := []string{}
		for i := range summary.FailureClusters {
			msgs = append(msgs, fmt.Sprintf("%s=%d", summary.FailureClusters[i].Name, summary.FailureClusters[i].Count))
		}
		fmt.Printf("Failure clusters: %s", strings.Join(msgs, ","))
	}
	if len(triageHints) == 0 {
		return
	}

	fmt.Println("Priority triage:")
	for _, hint := range triageHints {
		line := fmt.Sprintf("- P%d %s: %s", hint.Priority, hint.Code, hint.Message)
		if len(hint.StepIds) > 0 {
			line += fmt.Sprintf(" | steps=%s", strings.Join(hint.StepIds, ","))
		}
		if len(hint.RequirementNames) > 0 {
			line += fmt.Sprintf(" | requirements=%s", strings.Join(hint.RequirementNames, ","))
		}
		fmt.Println(line)
	}
}

func formatReplayRetainedStep(step core.MetaRunReplayStepPreview) string {
	var line = fmt.Sprintf("- %s | kind=%s | status=%s | duration_ms=%.3f", step.Id, step.Kind, step.Status, step.DurationMs)
	if step.FailureCode != "" {
		line += fmt.Sprintf(" | failure_code=%s", step.FailureCode)
	}
	if step.Continued {
		line += " | continued=true"
	}
	return line
}

func skillCommandPreviewMetaRunReplay(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	sessionId := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sessionId = args[i]
			break
		}
	}

	if sessionId == "" {
		msg := "Usage: openclaw skills meta-runs replay <session-id> --run <run-id> [--storage <path>] [--json]"
		writeSkillsCommandError(asJson, "skills meta-runs replay", "missing_session_id", msg)
		return 2, errors.New(msg)
	}

	requestedRunId := getOptionValue(args, "--run")
	if requestedRunId == "" {
		msg := "--run <run-id> is required for meta-runs replay preview."
		writeSkillsCommandError(asJson, "skills meta-runs replay", "missing_run_id", msg)
		return 2, errors.New(msg)
	}

	storagePath := getOptionValue(args, "--storage")
	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	session, err := store.GetSession(context.Background(), sessionId)
	if session == nil || err != nil {
		msg := fmt.Sprintf("Session '%s' not found.", sessionId)
		writeSkillsCommandError(asJson, "skills meta-runs replay", "session_not_found", msg)
		return 1, errors.New(msg)
	}

	var run *core.SessionMetaRunRecord
	for i := range session.MetaRunHistory {
		if session.MetaRunHistory[i].RunId == requestedRunId {
			run = &session.MetaRunHistory[i]
			break
		}
	}

	if run == nil {
		msg := fmt.Sprintf("Run '%s' not found in session '%s'", requestedRunId, sessionId)
		writeSkillsCommandError(asJson, "skills meta-runs replay", "run_not_found", msg)
		return 1, errors.New(msg)
	}

	preview := buildReplayPreview(sessionId, run)
	if asJson {
		data, _ := json.Marshal(preview)
		fmt.Println(string(data))
	} else {
		fmt.Printf("Replay preview for run: %s\n", preview.RunId)
		fmt.Printf("Session: %s\n", preview.SessionId)
		fmt.Printf("Skill: %s\n", preview.SkillName)
		if preview.ReplayAvailable {
			fmt.Println("Replay available: yes")
		} else {
			fmt.Println("Replay available: no")
		}

		fmt.Printf("Reason: %s\n", preview.Reason)
		fmt.Println("Replay plan:")
		fmt.Printf("Summary: %s\n", preview.Plan.Summary)
		fmt.Printf("Mode: %s\n", preview.Plan.Mode)
		if preview.Plan.Executable {
			fmt.Println("Executable: yes")
		} else {
			fmt.Println("Executable: no")
		}

		if len(preview.Plan.ReplayableSteps) > 0 {
			msgs := []string{}
			for i := range preview.Plan.ReplayableSteps {
				item := preview.Plan.ReplayableSteps[i]
				msgs = append(msgs, fmt.Sprintf("%s | readiness=%s | reason=%s", item.Id, item.Readiness, item.Reason))
			}
			fmt.Printf("Replayable steps: %s\n", strings.Join(msgs, ", "))
		}
		if len(preview.Plan.BlockedByRequirements) > 0 {
			msgs := []string{}
			for i := range preview.Plan.BlockedByRequirements {
				msgs = append(msgs, formatReplayRequirement(preview.Plan.BlockedByRequirements[i]))
			}
			fmt.Printf("Blocked by requirements: %s\n", strings.Join(msgs, ", "))
		}
		writeReplayOperatorDiagnosticsText(preview.OperatorSummary, preview.TriageHints)
		if len(preview.AvailableArtifacts) > 0 {
			fmt.Println("Available artifacts:")
			for _, item := range preview.AvailableArtifacts {
				fmt.Printf("- %s\n", item)
			}
		}
		if len(preview.RetainedSteps) > 0 {
			fmt.Println("Retained steps:")
			for _, step := range preview.RetainedSteps {
				fmt.Println(formatReplayRetainedStep(step))
			}
		}
		if len(preview.MissingRequirements) > 0 {
			fmt.Println("Missing replay inputs:")
			for _, item := range preview.MissingRequirements {
				fmt.Printf("- %s\n", formatReplayRequirement(item))
			}
		}
	}

	return 1, fmt.Errorf("can not find session with run id: %s", requestedRunId)
}

func tryParseRelativeSinceUtc(value string) (sinceUtc *time.Time, errMsg string, result bool) {
	if value == "" {
		result = true
		return
	}

	raw := strings.TrimSpace(value)
	if len(raw) < 2 {
		errMsg = "--since must end with m/h/d, e.g. 5m, 24h, 7d."
		return
	}

	var unit = strings.ToLower(raw[len(raw)-1:])
	if unit != "m" && unit != "h" && unit != "d" {
		errMsg = "--since must end with m/h/d, e.g. 5m, 24h, 7d."
		return
	}

	amount, err := strconv.Atoi(raw[:len(raw)-1])
	if amount <= 0 || err != nil {
		errMsg = "--since requires a positive integer amount, e.g. 5m, 24h, 7d."
		return
	}

	now := time.Now().UTC()
	switch unit {
	case "m":
		sinceUtc = new(now.Add(-time.Duration(amount) * time.Minute))
	case "h":
		sinceUtc = new(now.Add(-time.Duration(amount) * time.Hour))
	case "d":
		sinceUtc = new(now.Add(-time.Duration(amount) * 24 * time.Hour))
	}

	result = sinceUtc != nil
	return
}

func skillCommandListMetaRunFailuresGlobal(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	storagePath := getOptionValue(args, "--storage")
	sinceRaw := getOptionValue(args, "--since")
	skillName := getOptionValue(args, "--name")
	limit := util.Clamp(parseIntOption(args, "--limit", 500), 1, 2000)

	sinceUtc, sinceError, ok := tryParseRelativeSinceUtc(sinceRaw)
	if !ok {
		msg := sinceError
		if msg == "" {
			msg = "--since must end with m/h/d, e.g. 5m, 24h, 7d."
		}
		writeSkillsCommandError(asJson, "skills meta-runs failures", "invalid_since", msg)
		return 2, errors.New(msg)
	}

	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	ctx := context.Background()
	sessionIds := loadSessionIds(store, 1, 500)
	failed := []MetaRunGlobalItem{}
	for _, sessionId := range sessionIds {
		session, err := store.GetSession(ctx, sessionId)
		if session == nil || err != nil {
			continue
		}

		for _, run := range session.MetaRunHistory {
			if run.Status != "failed" {
				continue
			}

			if skillName != "" && run.SkillName != skillName {
				continue
			}
			if sinceUtc != nil && run.CompletedAtUtc.Before(*sinceUtc) {
				continue
			}
			failed = append(failed, MetaRunGlobalItem{
				SessionId:      session.Id,
				RunId:          run.RunId,
				SkillName:      run.SkillName,
				Status:         run.Status,
				StartedAtUtc:   run.StartedAtUtc,
				CompletedAtUtc: run.CompletedAtUtc,
				StepCount:      len(run.StepResults),
				ErrorCode:      run.ErrorCode,
			})
		}
	}

	slices.SortFunc(failed, func(a, b MetaRunGlobalItem) int {
		return b.CompletedAtUtc.Compare(a.CompletedAtUtc)
	})

	failed = failed[0:min(int64(len(failed)), limit)]
	if asJson {
		writeMetaRunsGlobalListJson(1, 500, failed)
		return 0, nil
	}

	fmt.Printf("Failed meta runs: %d\n", len(failed))
	for _, item := range failed {
		fmt.Printf("- %s | session=%s | status=%s | error=%s\n", item.RunId, item.SessionId, item.Status, item.ErrorCode)
	}

	return 0, nil
}

func skillCommandShowMetaRunStepsGlobal(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	storagePath := getOptionValue(args, "--storage")
	runId := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			runId = args[i]
			break
		}
	}

	if runId == "" {
		msg := "Usage: openclaw skills meta-runs steps <run-id> [--storage <path>] [--json]"
		writeSkillsCommandError(asJson, "skills meta-runs steps", "missing_run_id", msg)
		return 2, errors.New(msg)
	}

	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	sessionIds := loadSessionIds(store, 1, 500)
	for _, sessionId := range sessionIds {
		session, err := store.GetSession(context.Background(), sessionId)
		if session == nil || err != nil {
			continue
		}

		var run *core.SessionMetaRunRecord
		for i := 0; i < len(session.MetaRunHistory); i++ {
			if session.MetaRunHistory[i].RunId == runId {
				run = &session.MetaRunHistory[i]
			}
		}

		if run == nil {
			continue
		}

		if asJson {
			data := map[string]any{
				"sessionId": session.Id,
				"runId":     run.RunId,
				"skillName": run.SkillName,
				"status":    run.Status,
				"stepCount": len(run.StepResults),
				"steps":     run.StepResults,
			}

			a, _ := json.Marshal(data)
			fmt.Println(string(a))
		} else {
			fmt.Printf("Run: %s\n", run.RunId)
			fmt.Printf("Session: %s\n", session.Id)
			fmt.Printf("Steps: %d\n", len(run.StepResults))
			fmt.Printf("%-20s %-14s %-12s %-12s FAILURE\n", "STEP", "KIND", "STATUS", "DURATION")
			for _, step := range run.StepResults {
				duration := fmt.Sprintf("%.3fms", step.DurationMs)
				fmt.Printf("%-20s %-14s %-12s %-12s %s\n", step.Id, step.Kind, step.Status, duration, step.FailureCode)
			}
		}

		return 0, nil
	}

	msg := fmt.Sprintf("Run '%s' not found.", runId)
	writeSkillsCommandError(asJson, "skills meta-runs steps", "run_not_found", msg)
	return 1, errors.New(msg)
}

func writeMetaRunsGlobalShowJson(sessionId string, run core.SessionMetaRunRecord) {
	data := map[string]any{
		"sessionId":      sessionId,
		"runId":          run.RunId,
		"skillName":      run.SkillName,
		"status":         run.Status,
		"startedAtUtc":   run.StartedAtUtc,
		"completedAtUtc": run.CompletedAtUtc,
		"stepCount":      len(run.StepResults),
	}

	if run.ErrorCode != "" {
		data["errorCode"] = run.ErrorCode
	}
	if run.Error != "" {
		data["error"] = run.Error
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("JSON marshal error: %v\n", err)
		return
	}

	fmt.Println(string(jsonData))
}

func skillCommandShowMetaRunGlobal(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	storagePath := getOptionValue(args, "--storage")
	runId := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			runId = args[i]
			break
		}
	}

	if runId == "" {
		msg := "Usage: openclaw skills meta-runs show <run-id> [--storage <path>] [--json]"
		writeSkillsCommandError(asJson, "skills meta-runs show", "missing_run_id", msg)
		return 2, errors.New(msg)
	}

	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	sessionIds := loadSessionIds(store, 1, 500)
	for _, sessionId := range sessionIds {
		session, err := store.GetSession(context.Background(), sessionId)
		if session == nil || err != nil {
			continue
		}

		var run *core.SessionMetaRunRecord
		for i := 0; i < len(session.MetaRunHistory); i++ {
			if session.MetaRunHistory[i].RunId == runId {
				run = &session.MetaRunHistory[i]
			}
		}

		if run == nil {
			continue
		}

		if asJson {
			writeMetaRunsGlobalShowJson(session.Id, *run)
		} else {
			fmt.Printf("Run: %s\n", run.RunId)
			fmt.Printf("Session: %s\n", session.Id)
			fmt.Printf("Skill: %s\n", run.SkillName)
			fmt.Printf("Status: %s\n", run.Status)
		}

		return 0, nil
	}

	msg := fmt.Sprintf("Run '%s' not found.", runId)
	writeSkillsCommandError(asJson, "skills meta-runs show", "run_not_found", msg)
	return 1, errors.New(msg)
}

func loadSessionIds(store core.IMemoryStore, page, pageSize int) []string {
	adminStore, ok := store.(core.ISessionAdminStore)
	if !ok {
		return []string{}
	}

	ids := []string{}
	current := page
	for {
		result, err := adminStore.ListSessions(context.Background(), current, pageSize, &core.SessionListQuery{})
		if err != nil {
			continue
		}

		for _, v := range result.Items {
			ids = append(ids, v.Id)
		}

		if !result.HasMore {
			break
		}

		current++
	}

	return ids
}

type MetaRunGlobalItem struct {
	SessionId      string
	RunId          string
	SkillName      string
	Status         string
	StartedAtUtc   time.Time
	CompletedAtUtc time.Time
	StepCount      int
	ErrorCode      string
}

func writeMetaRunsGlobalListJson(page int, pageSize int, items []MetaRunGlobalItem) {
	itemList := make([]map[string]any, len(items))
	for i, item := range items {
		m := map[string]any{
			"sessionId":      item.SessionId,
			"runId":          item.RunId,
			"skillName":      item.SkillName,
			"status":         item.Status,
			"startedAtUtc":   item.StartedAtUtc,
			"completedAtUtc": item.CompletedAtUtc,
			"stepCount":      item.StepCount,
		}
		if strings.TrimSpace(item.ErrorCode) != "" {
			m["errorCode"] = item.ErrorCode
		}
		itemList[i] = m
	}

	payload := map[string]any{
		"page":     page,
		"pageSize": pageSize,
		"count":    len(items),
		"items":    itemList,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	fmt.Println(string(data))
}

func skillCommandListMetaRunsGlobal(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	storagePath := getOptionValue(args, "--storage")
	page := max(1, parseIntOption(args, "--page", 1))
	pageSize := util.Clamp(parseIntOption(args, "--page-size", 50), 1, 200)

	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	sessionIds := loadSessionIds(store, int(page), int(pageSize))
	items := []MetaRunGlobalItem{}

	for _, sessionId := range sessionIds {
		session, err := store.GetSession(context.Background(), sessionId)
		if err != nil || session == nil {
			continue
		}

		runs := session.MetaRunHistory
		slices.SortFunc(runs, func(a, b core.SessionMetaRunRecord) int {
			return b.CompletedAtUtc.Compare(a.CompletedAtUtc)
		})

		for i := 0; i < len(runs); i++ {
			run := runs[i]

			items = append(items, MetaRunGlobalItem{
				SessionId:      session.Id,
				RunId:          run.RunId,
				SkillName:      run.SkillName,
				Status:         run.Status,
				StartedAtUtc:   run.StartedAtUtc,
				CompletedAtUtc: run.CompletedAtUtc,
				StepCount:      len(run.StepResults),
				ErrorCode:      run.ErrorCode,
			})

		}
		if asJson {
			writeMetaRunsGlobalListJson(int(page), int(pageSize), items)
			return 0, nil
		}

		fmt.Printf("Global meta runs: %d\n", len(items))
		for _, item := range items {
			fmt.Printf("- %s | session=%s | status=%s | steps=%d\n", item.RunId, item.SessionId, item.Status, item.StepCount)
		}
	}

	return 0, nil
}

func skillCommandListReadOnlyProposalsEntry(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	if len(args) > 0 && (args[0] == "accept" || args[0] == "dismiss" || args[0] == "rollback" || args[0] == "change") {
		msg := "openclaw skills proposals is a read-only entry. Use `openclaw skills meta-runs proposals <accept|dismiss|rollback|change>` for lifecycle actions."
		writeSkillsCommandError(asJson, "skills proposals", "read_only_alias_lifecycle_action", msg)
		return 2, errors.New(msg)
	}

	if len(args) > 0 && args[0] == "show" {
		return showMetaRunProposal(args[1:], "skills proposals")
	}

	return listMetaRunProposals(args, "skills proposals")
}

func loadMetaRunLearningReviews(ctx context.Context, store core.ILearningProposalStore, sessionid string) map[string]core.MetaRunProposalReviewRecord {
	candidates, err := store.ListProposals(ctx, "", "meta_run_proposal")
	if err != nil {
		return map[string]core.MetaRunProposalReviewRecord{}
	}

	result := map[string]core.MetaRunProposalReviewRecord{}
	for _, item := range candidates {
		value, ok := tryGetMetaRunProposalMetadata(&item, core.MetaRunProposalMetadataSessionId)
		if ok && value == sessionid {
			_, ok := tryGetMetaRunProposalMetadata(&item, core.MetaRunProposalMetadataProposalId)
			if ok {
				result[item.Metadata[core.MetaRunProposalMetadataProposalId]] = util.Deref(toMetaRunProposalReviewRecord(item, sessionid, item.Metadata[core.MetaRunProposalMetadataProposalId]))
			}
		}
	}
	return result
}

func listMetaRunProposals(args []string, entrypoint string) (int, error) {
	asJson := slices.Contains(args, "--json")
	requestedRunId := getOptionValue(args, "--run")
	sessionId := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sessionId = args[i]
			break
		}
	}

	if sessionId == "" {
		msg := "Usage: openclaw skills meta-runs proposals <session-id> [--run <run-id>] [--storage <path>] [--json]"
		writeSkillsCommandError(asJson, "skills meta-runs proposals", "missing_session_id", msg)
		return 2, errors.New(msg)
	}

	storagePath := getOptionValue(args, "--storage")
	learningProposalStore, err := openLearningProposalStore(storagePath)
	if err != nil {
		return 1, err
	}
	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	ctx := context.Background()
	session, err := store.GetSession(ctx, sessionId)
	if session == nil || err != nil {
		msg := fmt.Sprintf("Session '%s' not found.", sessionId)
		writeSkillsCommandError(asJson, "skills meta-runs proposals", "session_not_found", msg)
		return 1, errors.New(msg)
	}

	proposals := buildDerivedProposals(session, requestedRunId)
	if requestedRunId != "" {
		find := false
		for _, run := range session.MetaRunHistory {
			if run.RunId == requestedRunId {
				find = true
				break
			}
		}

		if !find {
			msg := fmt.Sprintf("Run '%s' not found in session '%s'.", requestedRunId, sessionId)
			writeSkillsCommandError(asJson, "skills meta-runs proposals", "run_not_found", msg)
			return 1, errors.New(msg)
		}
	}

	reviews := loadMetaRunLearningReviews(ctx, learningProposalStore, sessionId)
	proposals = applyReviewSummary(proposals, reviews)

	response := core.MetaRunDerivedProposalListResponse{
		SessionId:     sessionId,
		Entrypoint:    entrypoint,
		ReadOnlyAlias: entrypoint == "skills proposals",
		Count:         len(proposals),
		Proposals:     proposals,
	}

	if asJson {
		date, _ := json.Marshal(response)
		fmt.Println(string(date))
	} else {
		writeDerivedProposalListText(response)
	}

	return 0, nil
}

func writeDerivedProposalListText(response core.MetaRunDerivedProposalListResponse) {
	panic("unimplemented")
}

func applyReviewSummary(proposals []core.MetaRunDerivedProposalSummary, reviews map[string]core.MetaRunProposalReviewRecord) []core.MetaRunDerivedProposalSummary {
	if len(proposals) == 0 {
		return proposals
	}

	result := []core.MetaRunDerivedProposalSummary{}
	for _, proposal := range proposals {
		review, ok := reviews[proposal.Id]
		if !ok {
			result = append(result, proposal)
		} else {
			result = append(result, core.MetaRunDerivedProposalSummary{
				Id:               proposal.Id,
				RunId:            proposal.RunId,
				SkillName:        proposal.SkillName,
				Status:           proposal.Status,
				Kind:             proposal.Kind,
				Title:            proposal.Title,
				Summary:          proposal.Summary,
				Source:           proposal.Source,
				AvailableActions: proposal.AvailableActions,
				ReviewStatus:     review.ReviewStatus,
				ReviewedAtUtc:    review.ReviewedAtUtc,
			})
		}
	}

	return result
}

func resolveMemoryPath(storagePath string) (string, error) {
	if storagePath != "" {
		return filepath.Abs(storagePath)
	}

	workspace := os.Getenv(envWorkspace)
	if workspace != "" {
		p, err := filepath.Abs(workspace)
		if err != nil {
			return "", err
		}
		return filepath.Join(p, "memory"), nil
	}

	return filepath.Abs("./memory")
}

func looksLikeSqlitePath(path string) bool {
	extension := filepath.Ext(path)
	return extension == ".db" || extension == ".sqlite" || extension == ".sqlite3"
}
func openLearningProposalStore(storagePath string) (core.ILearningProposalStore, error) {
	resolved, err := resolveMemoryPath(storagePath)
	if err != nil {
		return nil, err
	}
	if looksLikeSqlitePath(resolved) {
		return core.NewSqliteFeatureStore(resolved)
	}

	return core.NewFileFeatureStore(resolved)
}

func openMemoryStore(storagePath string) (core.IMemoryStore, error) {
	resolved, err := resolveMemoryPath(storagePath)
	if err != nil {
		return nil, err
	}
	if looksLikeSqlitePath(resolved) {
		return core.NewSqliteMemoryStore(resolved, false, nil, false, nil, nil)
	}
	return core.NewFileMemoryStore(resolved, 0, nil, nil, nil)
}

func buildDerivedProposals(session *core.Session, requestedRunId string) []core.MetaRunDerivedProposalSummary {
	result := []core.MetaRunDerivedProposalSummary{}
	for _, run := range session.MetaRunHistory {
		if requestedRunId == "" || run.RunId == requestedRunId {
			proposal := tryBuildDerivedProposalSummary(run, session.MetaExecutionCheckpoint)
			if proposal != nil {
				result = append(result, *proposal)
			}
		}
	}
	return result
}

func tryBuildDerivedProposalSummary(run core.SessionMetaRunRecord, checkpoint *core.SessionMetaExecutionCheckpoint) *core.MetaRunDerivedProposalSummary {
	if run.Status == "paused" {
		pendingStepId := ""
		if checkpoint != nil && checkpoint.SkillName == run.SkillName {
			pendingStepId = checkpoint.PendingStepId
		}
		summary := fmt.Sprintf("Run %s is paused and needs operator follow-up.", run.RunId)
		if pendingStepId != "" {
			summary = fmt.Sprintf("Run %s is paused at step %s.", run.RunId, pendingStepId)
		}

		return &core.MetaRunDerivedProposalSummary{
			Id:               fmt.Sprintf("meta-run:%s:paused", run.RunId),
			RunId:            run.RunId,
			SkillName:        run.SkillName,
			Status:           run.Status,
			Kind:             "paused_run_followup",
			Title:            fmt.Sprintf("Resume paused meta run %s", run.SkillName),
			Summary:          summary,
			AvailableActions: []string{"show", "accept", "dismiss"},
		}
	}

	if run.Status == "failed" {
		summary := fmt.Sprintf("Run %s failed and needs review.", run.RunId)
		if run.ErrorCode != "" {
			summary = fmt.Sprintf("Run %s failed with %s.", run.RunId, run.ErrorCode)
		}

		return &core.MetaRunDerivedProposalSummary{
			Id:               fmt.Sprintf("meta-run:%s:failed", run.RunId),
			RunId:            run.RunId,
			SkillName:        run.SkillName,
			Status:           run.Status,
			Kind:             "failed_run_review",
			Title:            fmt.Sprintf("Review failed meta run %s", run.SkillName),
			Summary:          summary,
			AvailableActions: []string{"show", "accept", "dismiss"},
		}
	}

	return nil
}
func buildMetaRunProposalDurableId(sessionId, proposalId string) string {
	return fmt.Sprintf("meta-run-proposal:%s:%s", sessionId, proposalId)
}
func buildMetaRunWorkflowDurableId(sessionId, proposalId string) string {
	return fmt.Sprintf("meta-run-workflow:%s:%s", sessionId, proposalId)
}

func getMetaRunLearningProposal(ctx context.Context, store core.ILearningProposalStore, sessionId, proposalId string) (*core.LearningProposal, error) {
	return store.GetProposal(ctx, buildMetaRunProposalDurableId(sessionId, proposalId))
}

func getMetaRunReviewWorkflowDetail(ctx context.Context, store core.ILearningProposalStore, sessionId, proposalId string) *core.MetaRunProposalWorkflowDetail {
	durableWorkflow, err := store.GetProposal(ctx, buildMetaRunWorkflowDurableId(sessionId, proposalId))
	if durableWorkflow == nil || err != nil {
		return nil
	}

	workflowId, ok := durableWorkflow.Metadata["meta_run_workflow_id"]
	stage, ok1 := durableWorkflow.Metadata["meta_run_workflow_stage"]
	lastAction, ok2 := durableWorkflow.Metadata["meta_run_workflow_last_action"]

	if !ok || !ok1 || !ok2 || workflowId == "" || stage == "" || lastAction == "" {
		return nil
	}

	lastActorId := durableWorkflow.Metadata["meta_run_workflow_last_actor_id"]
	var lastChangedAtUtc *time.Time
	lastChangedAtRaw := durableWorkflow.Metadata["meta_run_workflow_last_changed_at_utc"]

	if lastChanged, err := time.Parse(time.RFC3339, lastChangedAtRaw); err != nil {
		lastChangedAtUtc = new(lastChanged)
	}

	countRaw := durableWorkflow.Metadata["meta_run_workflow_transition_count"]
	transitionCount, _ := strconv.ParseInt(countRaw, 10, 64)
	return &core.MetaRunProposalWorkflowDetail{
		WorkflowId:       workflowId,
		Stage:            stage,
		LastAction:       lastAction,
		LastActorId:      lastActorId,
		LastChangedAtUtc: lastChangedAtUtc,
		TransitionCount:  transitionCount,
	}
}

func mapLearningProposalStatusToReviewStatus(lifecycleStatus string) string {
	switch lifecycleStatus {
	case "approved":
		return "approved"
	case "rejected":
		return "dismissed"
	case "rolled_back":
		return "rolled_back"
	default:
		return "pending"
	}
}

func toMetaRunProposalReviewRecord(proposal core.LearningProposal, sessionId, proposalId string) *core.MetaRunProposalReviewRecord {
	reason := proposal.ReviewNotes
	if proposal.Status == "rolled_back" {
		reason = proposal.RollbackReason
	}
	reviewedAtUtc := proposal.ReviewedAtUtc
	if reviewedAtUtc == nil {
		reviewedAtUtc = new(proposal.UpdatedAtUtc)
	}
	return &core.MetaRunProposalReviewRecord{
		SessionId:     sessionId,
		ProposalId:    proposalId,
		ReviewedAtUtc: reviewedAtUtc,
		Reason:        reason,
		ReviewStatus:  mapLearningProposalStatusToReviewStatus(proposal.Status),
	}
}

func buildDerivedProposalDetail(summary core.MetaRunDerivedProposalSummary, run core.SessionMetaRunRecord, checkpoint *core.SessionMetaExecutionCheckpoint) core.MetaRunDerivedProposalDetail {
	checkpointMatches := run.Status == "paused" && checkpoint != nil && checkpoint.SkillName == run.SkillName
	var checkpointDetail *core.MetaRunDerivedProposalCheckpointDetail
	pendingStepId := ""
	pendingStepIds := []string{}
	blockedStepIds := []string{}
	timelineStepIds := []string{}
	steps := []core.MetaRunDerivedProposalStepDetail{}
	if checkpointMatches {
		pendingStepId = checkpoint.PendingStepId
		pendingStepIds = checkpoint.PendingStepIds
		blockedStepIds = checkpoint.BlockedStepIds
		checkpointDetail = &core.MetaRunDerivedProposalCheckpointDetail{
			PendingStepId:       checkpoint.PendingStepId,
			PendingStepIds:      checkpoint.PendingStepIds,
			BlockedStepIds:      checkpoint.BlockedStepIds,
			PromptPresent:       checkpoint.Prompt != "",
			OutputStepIds:       slices.Collect(maps.Keys(checkpoint.Outputs)),
			FailureAliasStepIds: slices.Collect(maps.Keys(checkpoint.FailureAliases)),
		}
	}

	for _, step := range run.StepResults {
		timelineStepIds = append(timelineStepIds, step.Id)
		steps = append(steps, core.MetaRunDerivedProposalStepDetail{
			Id:          step.Id,
			Kind:        step.Kind,
			Status:      step.Status,
			FailureCode: step.FailureCode,
			DurationMs:  step.DurationMs,
			Continued:   step.Continued,
		})
	}
	return core.MetaRunDerivedProposalDetail{
		Id:               summary.Id,
		RunId:            summary.RunId,
		SkillName:        summary.SkillName,
		Status:           summary.Status,
		Kind:             summary.Kind,
		Title:            summary.Title,
		Summary:          summary.Summary,
		Source:           summary.Source,
		AvailableActions: summary.AvailableActions,
		Checkpoint:       checkpointDetail,
		Evidence: &core.MetaRunDerivedProposalEvidenceDetail{
			TimelineStepIds: timelineStepIds,
			ErrorCode:       run.ErrorCode,
			Error:           run.Error,
			FinalText:       run.FinalText,
		},
		PendingStepId:   pendingStepId,
		PendingStepIds:  pendingStepIds,
		BlockedStepIds:  blockedStepIds,
		TimelineStepIds: timelineStepIds,
		Steps:           steps,
		ErrorCode:       run.ErrorCode,
		Error:           run.Error,
		FinalText:       run.FinalText,
	}
}

func tryGetMetaRunProposalMetadata(proposal *core.LearningProposal, key string) (string, bool) {
	metadataValue, ok := proposal.Metadata[key]
	if ok && metadataValue != "" {
		return metadataValue, true
	}

	return "", false
}

func buildMetaRunProposalProvenanceDetail(proposal *core.LearningProposal) *core.MetaRunProposalProvenanceDetail {
	if proposal == nil {
		return nil
	}

	snapshotVersion, ok := tryGetMetaRunProposalMetadata(proposal, core.MetaRunProposalMetadataProvenanceSnapshotVersion)
	if !ok {
		return nil
	}

	capturedAtRaw, ok := tryGetMetaRunProposalMetadata(proposal, core.MetaRunProposalMetadataProvenanceCapturedAtUtc)
	if !ok {
		return nil
	}

	capturedAtUtc, err := time.Parse(time.RFC3339, capturedAtRaw)
	if err != nil {
		return nil
	}

	runStatus, ok := tryGetMetaRunProposalMetadata(proposal, core.MetaRunProposalMetadataProvenanceRunStatus)
	if !ok {
		return nil
	}

	runStartedRaw, ok := tryGetMetaRunProposalMetadata(proposal, core.MetaRunProposalMetadataProvenanceRunStartedAtUtc)
	if !ok {
		return nil
	}

	runStartedAtUtc, err := time.Parse(time.RFC3339, runStartedRaw)
	if err != nil {
		return nil
	}

	runCompletedRaw, ok := tryGetMetaRunProposalMetadata(proposal, core.MetaRunProposalMetadataProvenanceRunCompletedAtUtc)
	if !ok {
		return nil
	}

	runCompletedAtUtc, err := time.Parse(time.RFC3339, runCompletedRaw)
	if err != nil {
		return nil
	}

	stepCountRaw, ok := tryGetMetaRunProposalMetadata(proposal, core.MetaRunProposalMetadataProvenanceStepCount)
	if !ok {
		return nil
	}

	stepCount, err := strconv.ParseInt(stepCountRaw, 10, 64)
	if err != nil {
		return nil
	}

	stepIdsRaw := proposal.Metadata[core.MetaRunProposalMetadataProvenanceStepIds]
	checkpointPendingStepId := proposal.Metadata[core.MetaRunProposalMetadataProvenanceCheckpointPendingStepId]
	checkpointPromptPresentRaw := proposal.Metadata[core.MetaRunProposalMetadataProvenanceCheckpointPromptPresent]
	checkpointPromptPresent := checkpointPromptPresentRaw == "true"

	return &core.MetaRunProposalProvenanceDetail{
		SnapshotVersion:         snapshotVersion,
		CapturedAtUtc:           capturedAtUtc,
		RunStatus:               runStatus,
		RunStartedAtUtc:         runStartedAtUtc,
		RunCompletedAtUtc:       runCompletedAtUtc,
		StepCount:               stepCount,
		StepIds:                 strings.Split(",", stepIdsRaw),
		CheckpointPendingStepId: checkpointPendingStepId,
		CheckpointPromptPresent: checkpointPromptPresent,
	}
}

func buildMetaRunProposalLifecycleDetail(proposal *core.LearningProposal) *core.MetaRunProposalLifecycleDetail {
	if proposal == nil {
		return nil
	}

	return &core.MetaRunProposalLifecycleDetail{
		Status:          proposal.Status,
		RolledBack:      proposal.RolledBack,
		ReviewedAtUtc:   proposal.ReviewedAtUtc,
		RolledBackAtUtc: proposal.RolledBackAtUtc,
		ReviewNotes:     proposal.ReviewNotes,
		RollbackReason:  proposal.RollbackReason,
	}
}

func buildMetaRunProposalAuditDetail(proposal *core.LearningProposal) *core.MetaRunProposalAuditDetail {
	if proposal == nil {
		return nil
	}

	schemaVersion := proposal.Metadata[core.MetaRunProposalMetadataAuditSchemaVersion]
	if schemaVersion == "" {
		schemaVersion = "v1"
	}

	actorId := proposal.Metadata[core.MetaRunProposalMetadataLastTransitionActorId]
	transitionAction := proposal.Metadata[core.MetaRunProposalMetadataLastTransitionAction]
	changedAtRaw := proposal.Metadata[core.MetaRunProposalMetadataLastTransitionChangedAtUtc]

	var changedAtUtc *time.Time
	parsed, err := time.Parse(time.RFC3339, changedAtRaw)
	if err == nil {
		changedAtUtc = new(parsed)
	}

	if actorId == "" && transitionAction == "" && changedAtUtc == nil {
		return nil
	}

	return &core.MetaRunProposalAuditDetail{
		SchemaVersion:    schemaVersion,
		ActorId:          actorId,
		ChangedAtUtc:     changedAtUtc,
		TransitionAction: transitionAction,
	}
}

func buildMetaRunProposalTransitionMetadataKey(index int64, field string) string {
	return fmt.Sprintf("%s_%04d_%s", core.MetaRunProposalMetadataTransitionEntryPrefix, index, field)
}

func buildMetaRunProposalProvenanceHistory(proposal *core.LearningProposal) []core.MetaRunProposalProvenanceTransition {
	if proposal == nil {
		return []core.MetaRunProposalProvenanceTransition{}
	}

	countRaw := proposal.Metadata[core.MetaRunProposalMetadataTransitionCount]
	count, err := strconv.ParseInt(countRaw, 10, 64)
	if count <= 0 || err != nil {
		return []core.MetaRunProposalProvenanceTransition{}
	}

	transitions := make([]core.MetaRunProposalProvenanceTransition, count)
	for index := int64(0); index < count; index++ {
		action, ok := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, core.MetaRunProposalMetadataTransitionFieldAction)]
		if !ok || action == "" {
			continue
		}
		fromStatus, ok := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, core.MetaRunProposalMetadataTransitionFieldFromStatus)]
		if !ok || fromStatus == "" {
			continue
		}
		toStatus, ok := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, core.MetaRunProposalMetadataTransitionFieldToStatus)]
		if !ok || toStatus == "" {
			continue
		}
		changedAtUtc, err := time.Parse(time.RFC3339, proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, core.MetaRunProposalMetadataTransitionFieldChangedAtUtc)])
		if err != nil {
			continue
		}

		reason := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, core.MetaRunProposalMetadataTransitionFieldReason)]
		transitions[index] = core.MetaRunProposalProvenanceTransition{
			Action:       action,
			FromStatus:   fromStatus,
			ToStatus:     toStatus,
			ChangedAtUtc: changedAtUtc,
			Reason:       reason,
		}
	}

	return transitions
}

func showMetaRunProposal(args []string, entrypoint string) (int, error) {
	asJson := slices.Contains(args, "--json")
	sessionId := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sessionId = args[i]
			break
		}
	}
	if sessionId == "" {
		msg := "Usage: openclaw skills meta-runs proposals show <session-id> --proposal <id> [--storage <path>] [--json]"
		writeSkillsCommandError(asJson, "skills meta-runs proposals show", "missing_session_id", msg)
		return 2, errors.New(msg)
	}

	proposalId := getOptionValue(args, "--proposal")
	if proposalId == "" {
		msg := "--proposal <id> is required for meta-runs proposals show."
		writeSkillsCommandError(asJson, "skills meta-runs proposals show", "missing_proposal_id", msg)
		return 2, errors.New(msg)
	}

	storagePath := getOptionValue(args, "--storage")
	learningProposalStore, err := openLearningProposalStore(storagePath)
	if err != nil {
		return 1, err
	}
	store, err := openMemoryStore(storagePath)
	if err != nil {
		return 1, err
	}

	ctx := context.Background()
	session, err := store.GetSession(ctx, sessionId)
	if session == nil || err != nil {
		msg := fmt.Sprintf("Session '%s' not found.", sessionId)
		writeSkillsCommandError(asJson, "skills meta-runs proposals show", "session_not_found", msg)
		return 1, errors.New(msg)
	}

	proposals := buildDerivedProposals(session, "")
	var summary core.MetaRunDerivedProposalSummary
	for i := 0; i < len(proposals); i++ {
		if proposals[i].Id == proposalId {
			summary = proposals[i]
			break
		}
	}

	if summary.Id == "" {
		msg := fmt.Sprintf("Proposal '%s' not found in session '%s'.", proposalId, sessionId)
		writeSkillsCommandError(asJson, "skills meta-runs proposals show", "proposal_not_found", msg)
		return 1, errors.New(msg)
	}

	var run core.SessionMetaRunRecord
	for i := 0; i < len(session.MetaRunHistory); i++ {
		if session.MetaRunHistory[i].RunId == summary.RunId {
			run = session.MetaRunHistory[i]
			break
		}
	}

	durableProposal, err := getMetaRunLearningProposal(ctx, learningProposalStore, sessionId, summary.Id)
	if err != nil {
		return 1, err
	}

	workflow := getMetaRunReviewWorkflowDetail(ctx, learningProposalStore, sessionId, summary.Id)

	var review *core.MetaRunProposalReviewRecord
	if durableProposal != nil {
		review = toMetaRunProposalReviewRecord(*durableProposal, sessionId, summary.Id)
	}

	detail := core.MetaRunDerivedProposalDetailResponse{
		SessionId:     sessionId,
		Entrypoint:    entrypoint,
		ReadOnlyAlias: entrypoint == "skills proposals",
		Proposal: applyReviewDetail(
			buildDerivedProposalDetail(summary, run, session.MetaExecutionCheckpoint),
			review,
			buildMetaRunProposalProvenanceDetail(durableProposal),
			buildMetaRunProposalLifecycleDetail(durableProposal),
			buildMetaRunProposalAuditDetail(durableProposal),
			workflow,
			buildMetaRunProposalProvenanceHistory(durableProposal)),
	}

	if asJson {
		data, _ := json.Marshal(detail)
		fmt.Println(string(data))
	} else {
		writeDerivedProposalDetailText(detail)
	}

	return 0, nil
}

func writeDerivedProposalDetailText(detail core.MetaRunDerivedProposalDetailResponse) {
	proposal := detail.Proposal
	fmt.Printf("Session: %s\n", detail.SessionId)
	fmt.Printf("Proposal: %s\n", proposal.Id)
	fmt.Printf("Run: %s\n", proposal.RunId)
	fmt.Printf("Skill: %s\n", proposal.SkillName)
	fmt.Printf("Status: %s\n", proposal.Status)
	fmt.Printf("Kind: %s\n", proposal.Kind)
	fmt.Printf("Title: %s\n", proposal.Title)
	fmt.Printf("Summary: %s\n", proposal.Summary)
	fmt.Printf("Source: %s\n", proposal.Source)
	fmt.Printf("Available actions: %s\n", strings.Join(proposal.AvailableActions, ", "))

	if proposal.Review != nil {
		fmt.Println("Review:")
		fmt.Printf("Status: %s\n", proposal.Review.Status)
		fmt.Printf("Reviewed at (UTC): %s\n", proposal.Review.ReviewedAtUtc.Format(time.RFC3339Nano))
		if strings.TrimSpace(proposal.Review.Reason) != "" {
			fmt.Printf("Reason: %s\n", proposal.Review.Reason)
		}
	}

	if proposal.Checkpoint != nil {
		fmt.Println("Checkpoint:")
		fmt.Printf("Pending step: %s\n", proposal.Checkpoint.PendingStepId)
		if len(proposal.Checkpoint.PendingStepIds) > 0 {
			fmt.Printf("Pending steps: %s\n", strings.Join(proposal.Checkpoint.PendingStepIds, ", "))
		}
		if len(proposal.Checkpoint.BlockedStepIds) > 0 {
			fmt.Printf("Blocked steps: %s\n", strings.Join(proposal.Checkpoint.BlockedStepIds, ", "))
		}
		if proposal.Checkpoint.PromptPresent {
			fmt.Println("Prompt present: yes")
		} else {
			fmt.Println("Prompt present: no")
		}
		if len(proposal.Checkpoint.OutputStepIds) > 0 {
			fmt.Printf("Output steps: %s\n", strings.Join(proposal.Checkpoint.OutputStepIds, ", "))
		}
		if len(proposal.Checkpoint.FailureAliasStepIds) > 0 {
			fmt.Printf("Failure alias steps: %s\n", strings.Join(proposal.Checkpoint.FailureAliasStepIds, ", "))
		}
	}

	if proposal.Evidence != nil {
		fmt.Println("Evidence:")
		if len(proposal.Evidence.TimelineStepIds) > 0 {
			fmt.Printf("Evidence timeline steps: %s\n", strings.Join(proposal.Evidence.TimelineStepIds, ", "))
		}
		if strings.TrimSpace(proposal.Evidence.ErrorCode) != "" {
			fmt.Printf("Evidence error code: %s\n", proposal.Evidence.ErrorCode)
		}
		if strings.TrimSpace(proposal.Evidence.Error) != "" {
			fmt.Printf("Evidence error: %s\n", proposal.Evidence.Error)
		}
		if strings.TrimSpace(proposal.Evidence.FinalText) != "" {
			fmt.Printf("Evidence final text: %s\n", proposal.Evidence.FinalText)
		}
	}

	if proposal.Provenance != nil {
		fmt.Println("Provenance:")
		fmt.Printf("Snapshot version: %s\n", proposal.Provenance.SnapshotVersion)
		fmt.Printf("Captured at (UTC): %s\n", proposal.Provenance.CapturedAtUtc.Format(time.RFC3339Nano))
		fmt.Printf("Run status snapshot: %s\n", proposal.Provenance.RunStatus)
		fmt.Printf("Step count snapshot: %d\n", proposal.Provenance.StepCount)
		if len(proposal.Provenance.StepIds) > 0 {
			fmt.Printf("Step ids snapshot: %s\n", strings.Join(proposal.Provenance.StepIds, ", "))
		}
		if strings.TrimSpace(proposal.Provenance.CheckpointPendingStepId) != "" {
			fmt.Printf("Checkpoint pending step snapshot: %s\n", proposal.Provenance.CheckpointPendingStepId)
		}
		if proposal.Provenance.CheckpointPromptPresent {
			fmt.Println("Checkpoint prompt snapshot: yes")
		} else {
			fmt.Println("Checkpoint prompt snapshot: no")
		}
	}

	if proposal.Lifecycle != nil {
		fmt.Println("Lifecycle:")
		fmt.Printf("Status: %s\n", proposal.Lifecycle.Status)
		if proposal.Lifecycle.RolledBack {
			fmt.Println("Rolled back: yes")
		} else {
			fmt.Println("Rolled back: no")
		}
		if proposal.Lifecycle.ReviewedAtUtc != nil {
			fmt.Printf("Reviewed at (UTC): %s\n", proposal.Lifecycle.ReviewedAtUtc.Format(time.RFC3339Nano))
		}
		if proposal.Lifecycle.RolledBackAtUtc != nil {
			fmt.Printf("Rolled back at (UTC): %s\n", proposal.Lifecycle.RolledBackAtUtc.Format(time.RFC3339Nano))
		}
		if strings.TrimSpace(proposal.Lifecycle.ReviewNotes) != "" {
			fmt.Printf("Review notes: %s\n", proposal.Lifecycle.ReviewNotes)
		}
		if strings.TrimSpace(proposal.Lifecycle.RollbackReason) != "" {
			fmt.Printf("Rollback reason: %s\n", proposal.Lifecycle.RollbackReason)
		}
	}

	if len(proposal.ProvenanceHistory) > 0 {
		fmt.Println("Provenance history:")
		for _, transition := range proposal.ProvenanceHistory {
			detail := fmt.Sprintf("- %s | %s -> %s | changedAtUtc=%s", transition.Action, transition.FromStatus, transition.ToStatus, transition.ChangedAtUtc.Format(time.RFC3339Nano))
			if strings.TrimSpace(transition.Reason) != "" {
				detail += fmt.Sprintf(" | reason=%s", transition.Reason)
			}
			fmt.Println(detail)
		}
	}

	if len(proposal.TimelineStepIds) > 0 {
		fmt.Printf("Timeline steps: %s\n", strings.Join(proposal.TimelineStepIds, ", "))
	}

	if len(proposal.Steps) > 0 {
		fmt.Println("Steps:")
		for _, step := range proposal.Steps {
			detail := fmt.Sprintf("- %s | kind=%s | status=%s", step.Id, step.Kind, step.Status)
			if strings.TrimSpace(step.FailureCode) != "" {
				detail += fmt.Sprintf(" | failureCode=%s", step.FailureCode)
			}
			detail += fmt.Sprintf(" | durationMs=%.14f", step.DurationMs)
			detail += fmt.Sprintf(" | continued=%t", step.Continued)
			fmt.Println(detail)
		}
	}

	if strings.TrimSpace(proposal.ErrorCode) != "" {
		fmt.Printf("Error code: %s\n", proposal.ErrorCode)
	}
	if strings.TrimSpace(proposal.Error) != "" {
		fmt.Printf("Error: %s\n", proposal.Error)
	}
	if strings.TrimSpace(proposal.FinalText) != "" {
		fmt.Printf("Final text: %s\n", proposal.FinalText)
	}
}

func applyReviewDetail(detail core.MetaRunDerivedProposalDetail, review *core.MetaRunProposalReviewRecord, provenance *core.MetaRunProposalProvenanceDetail, lifecycle *core.MetaRunProposalLifecycleDetail, audit *core.MetaRunProposalAuditDetail, workflow *core.MetaRunProposalWorkflowDetail, provenanceHistory []core.MetaRunProposalProvenanceTransition) *core.MetaRunDerivedProposalDetail {
	var reviewDetail *core.MetaRunProposalReviewDetail
	if review != nil {
		reviewDetail = &core.MetaRunProposalReviewDetail{
			Status:        review.ReviewStatus,
			ReviewedAtUtc: review.ReviewedAtUtc,
			Reason:        review.Reason,
		}
	}
	return &core.MetaRunDerivedProposalDetail{
		Id:                detail.Id,
		RunId:             detail.RunId,
		SkillName:         detail.SkillName,
		Status:            detail.Status,
		Kind:              detail.Kind,
		Title:             detail.Title,
		Summary:           detail.Summary,
		Source:            detail.Source,
		AvailableActions:  detail.AvailableActions,
		Checkpoint:        detail.Checkpoint,
		Evidence:          detail.Evidence,
		Provenance:        provenance,
		Lifecycle:         lifecycle,
		Audit:             audit,
		Workflow:          workflow,
		ProvenanceHistory: provenanceHistory,
		Review:            reviewDetail,
		PendingStepId:     detail.PendingStepId,
		PendingStepIds:    detail.PendingStepIds,
		BlockedStepIds:    detail.BlockedStepIds,
		TimelineStepIds:   detail.TimelineStepIds,
		Steps:             detail.Steps,
		ErrorCode:         detail.ErrorCode,
		Error:             detail.Error,
		FinalText:         detail.FinalText,
	}
}

type ProposalDraftQuality struct {
	ChecksPassed int
	ChecksTotal  int
	Checks       []ProposalDraftQualityCheck
	Warnings     []string
}

type ProposalDraftQualityCheck struct {
	Id             string
	Status         string
	Message        string
	Recommendation string
}

func buildProposalDraftQuality(name, description, kind string) ProposalDraftQuality {
	checks := []ProposalDraftQualityCheck{}
	warnings := []string{}

	hasName := name != ""
	check := ProposalDraftQualityCheck{
		Id:             "name_present",
		Status:         "fail",
		Message:        "Skill name is empty.",
		Recommendation: "Provide a non-empty skill name.",
	}
	if hasName {
		check.Status = "pass"
		check.Message = "Skill name is present."
		check.Recommendation = ""
	}

	checks = append(checks, check)

	hasDescription := description != ""
	descriptionStatus := "fail"
	descriptionMessage := "Skill description is empty."
	descriptionRecommendation := "Add a meaningful description that explains intent and outcomes."
	if hasDescription {
		descriptionStatus = "pass"
		descriptionMessage = "Skill description is present."
		descriptionRecommendation = ""
	}

	if hasDescription && len(description) < 16 {
		descriptionStatus = "warn"
		descriptionMessage = "Skill description is present but too short to guide proposal review."
		descriptionRecommendation = "Expand description to include expected behavior and boundaries."
		warnings = append(warnings, "description_too_short")
	}

	checks = append(checks, ProposalDraftQualityCheck{
		Id:             "description_present",
		Status:         descriptionStatus,
		Message:        descriptionMessage,
		Recommendation: descriptionRecommendation,
	})

	check = ProposalDraftQualityCheck{
		Id:             "meta_composition_seeded",
		Status:         "fail",
		Message:        "Meta composition scaffold is missing.",
		Recommendation: "Use --kind meta to seed composition scaffolding.",
	}
	if kind == "meta" {
		check.Status = "pass"
		check.Message = "Meta composition scaffold is seeded."
		check.Recommendation = ""
	}

	checks = append(checks, check)
	checksPassed := 0
	for _, v := range checks {
		if v.Status == "pass" {
			checksPassed++
		}
	}
	return ProposalDraftQuality{
		ChecksPassed: checksPassed,
		ChecksTotal:  len(checks),
		Checks:       checks,
		Warnings:     warnings,
	}
}

func isProposalDraftQualityBlocked(quality ProposalDraftQuality) bool {
	for _, check := range quality.Checks {
		if check.Status != "pass" {
			return true
		}
	}

	return false
}

func writeCreateProposalDraftQualityError(asJson bool, quality ProposalDraftQuality) string {
	var blockingChecks []ProposalDraftQualityCheck
	for _, check := range quality.Checks {
		if check.Status != "pass" {
			blockingChecks = append(blockingChecks, check)
		}
	}

	message := fmt.Sprintf("Proposal draft quality gate failed: %d/%d checks passed.", quality.ChecksPassed, quality.ChecksTotal)

	if !asJson {
		fmt.Fprintln(os.Stderr, message)
		for _, check := range blockingChecks {
			fmt.Fprintf(os.Stderr, "  - %s [%s]: %s\n", check.Id, check.Status, check.Message)
		}
		return message
	}

	blockingChecksList := make([]map[string]any, len(blockingChecks))
	for i, check := range blockingChecks {
		bc := map[string]any{
			"id":      check.Id,
			"status":  check.Status,
			"message": check.Message,
		}
		if check.Recommendation != "" {
			bc["recommendation"] = check.Recommendation
		}
		blockingChecksList[i] = bc
	}

	data := map[string]any{
		"status":    "error",
		"command":   "skills create",
		"errorCode": "proposal_draft_quality_gate_failed",
		"message":   message,
		"quality": map[string]any{
			"checksPassed":        quality.ChecksPassed,
			"checksTotal":         quality.ChecksTotal,
			"minimumChecksPassed": quality.ChecksTotal,
			"blockingChecks":      blockingChecksList,
			"warnings":            quality.Warnings,
		},
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to marshal error JSON:", err)
		return err.Error()
	}

	msg := string(jsonBytes)
	fmt.Fprintln(os.Stderr, msg)
	return msg
}

func writeCreateError(asJson bool, errorCode string, message string) {
	if !asJson {
		fmt.Fprintln(os.Stderr, message)
		return
	}

	data := map[string]any{
		"status":    "error",
		"command":   "skills create",
		"errorCode": errorCode,
		"message":   message,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to marshal error JSON:", err)
		return
	}

	fmt.Fprintln(os.Stderr, string(jsonBytes))
}

func buildSkillScaffoldMarkdown(name, description, kind string) string {
	lines := []string{
		"---",
		fmt.Sprintf("name: %s", name),
		fmt.Sprintf("description: %s", description),
	}

	if kind == "meta" {
		lines = append(lines, "kind: meta")
		lines = append(lines, "composition: {\"steps\":[{\"id\":\"draft\",\"kind\":\"llm_chat\",\"with\":{\"prompt\":\"Summarize the user intent and propose next steps.\"}}]}")
	}

	lines = append(lines, "---")
	lines = append(lines, "")
	lines = append(lines, "Describe what this skill should do and how it should be used.")

	return strings.Join(lines, "\n") + "\n"
}

func skillCommandCreateSkillScaffold(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	force := slices.Contains(args, "--force")
	proposalDraftRequested := slices.Contains(args, "--proposal-draft")
	managed := slices.Contains(args, "--managed")
	workdir := getOptionValue(args, "--workdir")
	name := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			name = args[i]
			break
		}
	}
	if name == "" {
		msg := "Usage: openclaw skills create <name> [--kind <standard|meta>] [--description <text>] [--proposal-draft] [--workdir <path> | --managed] [--json] [--force]"
		writeCreateError(asJson, "invalid_create_usage", msg)
		return 2, errors.New(msg)
	}

	kind := getOptionValue(args, "--kind")
	if kind == "" {
		kind = "standard"
	}
	if kind != "standard" && kind != "meta" {
		writeCreateError(asJson, "invalid_kind", "--kind must be one of: standard, meta.")
		return 2, errors.New("--kind must be one of: standard, meta.")
	}

	normalizeSingleLineValue := func(value string) string {

		replacer := strings.NewReplacer(
			"\r", " ",
			"\n", " ",
		)
		return replacer.Replace(value)
	}

	name = normalizeSingleLineValue(name)
	if name == "" {
		writeCreateError(asJson, "invalid_skill_name", "Skill name cannot be empty.")
		return 2, errors.New("Skill name cannot be empty.")
	}

	slug := slugify(name)
	if slug == "" {
		writeCreateError(asJson, "invalid_skill_name", "Skill name does not contain any letters or digits.")
		return 2, errors.New("Skill name does not contain any letters or digits.")
	}

	description := getOptionValue(args, "--description")
	if description == "" {
		description = fmt.Sprintf("%s workflow scaffold.", name)
	}
	description = normalizeSingleLineValue(description)
	skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}

	skillDirectory := filepath.Join(skillsDirectory, slug)
	exists := util.DirectoryExists(skillDirectory)
	if exists && !force {
		msg := fmt.Sprintf("Skill scaffold already exists: %s. Use --force to overwrite SKILL.md.", skillDirectory)
		writeCreateError(asJson, "skill_already_exists", msg)
		return 1, errors.New(msg)
	}

	kindValue := "standard"
	if kind == "meta" {
		kindValue = "meta"
	}

	if proposalDraftRequested && kindValue != "meta" {
		writeCreateError(asJson, "invalid_proposal_draft_kind", "--proposal-draft is only supported for --kind meta.")
		return 2, errors.New("--proposal-draft is only supported for --kind meta.")
	}

	var proposalDraftQuality *ProposalDraftQuality
	if proposalDraftRequested {
		proposalDraftQuality = new(buildProposalDraftQuality(name, description, kindValue))
		if isProposalDraftQualityBlocked(*proposalDraftQuality) {
			msg := writeCreateProposalDraftQualityError(asJson, *proposalDraftQuality)
			return 1, errors.New(msg)
		}
	}

	os.MkdirAll(skillDirectory, 0755)

	skillMarkdown := buildSkillScaffoldMarkdown(name, description, kindValue)
	os.WriteFile(filepath.Join(skillDirectory, "SKILL.md"), []byte(skillMarkdown), 0644)
	proposalDraftId := slug
	proposalDraftKind := "meta_skill_creator_draft"
	proposalDraftStatus := "draft"
	proposalDraftTitle := fmt.Sprintf("Meta skill draft proposal: %s", name)
	proposalDraftSummary := "Draft proposal prepared from scaffold metadata. Review and refine before lifecycle actions."

	if asJson {
		response := map[string]any{
			"name":      name,
			"slug":      slug,
			"kind":      kindValue,
			"path":      skillDirectory,
			"created":   true,
			"overwrote": exists,
		}

		if proposalDraftRequested {
			if proposalDraftQuality == nil {
				return 1, errors.New("Proposal draft quality must be available when proposal draft is requested.")
			}
			var checks []map[string]any
			for _, check := range proposalDraftQuality.Checks {
				c := map[string]any{
					"id":      check.Id,
					"status":  check.Status,
					"message": check.Message,
				}
				if strings.TrimSpace(check.Recommendation) != "" {
					c["recommendation"] = check.Recommendation
				}
				checks = append(checks, c)
			}

			response["proposalDraft"] = map[string]any{
				"available": true,
				"id":        proposalDraftId,
				"kind":      proposalDraftKind,
				"status":    proposalDraftStatus,
				"title":     proposalDraftTitle,
				"summary":   proposalDraftSummary,
				"quality": map[string]any{
					"checksPassed":        proposalDraftQuality.ChecksPassed,
					"checksTotal":         proposalDraftQuality.ChecksTotal,
					"minimumChecksPassed": proposalDraftQuality.ChecksTotal,
					"checks":              checks,
					"warnings":            proposalDraftQuality.Warnings,
				},
			}
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			panic(err)
		}

		fmt.Println(string(jsonData))
		return 0, nil
	}

	fmt.Printf("Created skill scaffold: %s [%s]\n", name, kindValue)
	fmt.Printf("Path: %s\n", skillDirectory)
	if exists {
		fmt.Println("Overwrote existing SKILL.md via --force.")
	}

	if proposalDraftRequested {
		if proposalDraftQuality == nil {
			return 1, errors.New("Proposal draft quality must be available when proposal draft is requested.")
		}

		fmt.Printf("Proposal draft: %s\n", proposalDraftStatus)
		fmt.Printf("Proposal kind: %s\n", proposalDraftKind)
		fmt.Printf("Proposal id: %s\n", proposalDraftId)
		fmt.Printf("Proposal quality: %d/%d checks passed\n", proposalDraftQuality.ChecksPassed, proposalDraftQuality.ChecksTotal)
	}

	return 0, nil
}

func skillCommandListCatalog(args []string) (int, error) {
	managed := slices.Contains(args, "--managed")
	stable := slices.Contains(args, "--stable")
	workdir := getOptionValue(args, "--workdir")
	asJson := slices.Contains(args, "--json")
	kind := getOptionValue(args, "--kind")

	if kind != "" && kind != "all" && kind != "meta" {
		writeSkillsCommandError(asJson, "skills catalog", "invalid_kind", "--kind supports only 'all' or 'meta'.")
		return 2, errors.New("--kind supports only 'all' or 'meta'.")
	}

	catalog := []SkillCommandInspection{}
	var skillsRootPath string
	var source core.SkillSource
	if stable {
		bundledDir := filepath.Join(".", "skills")
		if !util.DirectoryExists(bundledDir) {
			if asJson {
				fmt.Println("{\"count\":0,\"skills\":[]}")
				return 0, nil
			}

			fmt.Println("Stable meta catalog (0):")
			return 0, nil
		}
		skillsRootPath = bundledDir
		source = core.SkillSourceBundled
	} else {
		skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
		if err != nil {
			return 1, err
		}
		if !util.DirectoryExists(skillsDirectory) {
			if asJson {
				fmt.Println("{\"count\":0,\"skills\":[]}")
				return 0, nil
			}

			fmt.Println("No skills installed.")
			return 0, nil
		}

		skillsRootPath = skillsDirectory
	}

	inspectionResults := core.SkillInspectorInstance.InspectInstalledRoot(skillsRootPath, source)
	for _, v := range inspectionResults {
		if v.Success && v.Definition != nil {
			inspection := createInspection(&v)
			if kind == "" || kind == "all" || (kind == "meta" && inspection.Definition.Kind == core.SkillKind_Meta) {
				catalog = append(catalog, inspection)
			}
		}
	}

	slices.SortFunc(catalog, func(a, b SkillCommandInspection) int {
		return strings.Compare(a.Definition.Name, b.Definition.Name)
	})

	if asJson {
		type innerDtoItem struct {
			Name        string `json:"name"`
			Kind        string `json:"kind"`
			Description string `json:"description"`
			Trust       string `json:"trust"`
			Source      string `json:"source"`
			Path        string `json:"path"`
		}
		type innerDto struct {
			Count  int            `json:"count"`
			Skills []innerDtoItem `json:"skill"`
		}
		dto := innerDto{
			Count: len(catalog),
		}
		for _, item := range catalog {
			dto.Skills = append(dto.Skills, innerDtoItem{
				Name:        item.Definition.Name,
				Kind:        strings.ToLower(item.Definition.Kind.ToString()),
				Description: item.Definition.Description,
				Trust:       item.TrustLevel,
				Source:      item.SourceLabel,
				Path:        item.SkillRootPath,
			})
		}

		raw, _ := json.Marshal(dto)
		fmt.Println(string(raw))
		return 0, nil
	}

	header := "Skill catalog"
	if stable {
		header = "Stable meta catalog"
	}

	fmt.Printf("%s (%d):\n", header, len(catalog))
	for _, item := range catalog {
		fmt.Printf("  %s [%s] - %s\n", item.Definition.Name, strings.ToLower(item.Definition.Kind.ToString()), item.Definition.Description)
		fmt.Printf("    Trust: %s\n", item.TrustLevel)
		fmt.Printf("    Source: %s\n", item.SourceLabel)
		fmt.Printf("    Path: %s\n", item.SkillRootPath)
	}

	return 0, nil
}

func skillCommandListInstalled(args []string) (int, error) {
	managed := slices.Contains(args, "--managed")
	workdir := getOptionValue(args, "--workdir")
	skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}

	if !util.DirectoryExists(skillsDirectory) {
		fmt.Println("No skills installed.")
		return 0, nil
	}

	source := core.SkillSourceWorkspace
	if managed {
		source = core.SkillSourceManaged
	}
	inspectionResults := core.SkillInspectorInstance.InspectInstalledRoot(skillsDirectory, source)

	inspections := []SkillCommandInspection{}
	for _, inspection := range inspectionResults {
		if inspection.Success && inspection.Definition != nil {
			inspections = append(inspections, createInspection(&inspection))
		}
	}

	slices.SortFunc(inspections, func(a, b SkillCommandInspection) int {
		return strings.Compare(a.Definition.Name, b.Definition.Name)
	})

	if len(inspections) == 0 {
		fmt.Println("No skills installed.")
		return 0, nil
	}

	fmt.Printf("Installed skills (%d):\n", len(inspections))
	for i := 0; i < len(inspections); i++ {
		inspection := inspections[i]
		fmt.Printf("  %s - %s\n", inspection.Definition.Name, inspection.Definition.Description)
		fmt.Printf("    Trust: %s\n", inspection.TrustLevel)
		fmt.Printf("    Source: %s\n", inspection.SourceLabel)
		fmt.Printf("    Path: %s\n", inspection.SkillRootPath)
	}

	return 0, nil
}

func skillCommandInstall(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	sourcePath := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sourcePath = args[i]
			break
		}
	}

	if sourcePath == "" {
		writeSkillsCommandError(asJson, "skills install", "missing_source_path", "Usage: openclaw skills install <path|tarball>")
		return 2, errors.New("Usage: openclaw skills install <path|tarball>")
	}

	dryRun := slices.Contains(args, "--dry-run")
	managed := slices.Contains(args, "--managed")
	workdir := getOptionValue(args, "--workdir")

	inspected, tempDirectory := skillCommandInspectResolvedSource(sourcePath, true)
	if !inspected.Success {
		msg := inspected.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("Failed to inspect source path: %s", sourcePath)
		}
		writeSkillsCommandError(asJson, "skills install", "install_failed", msg)
		return 1, errors.New(msg)
	}

	if tempDirectory != "" {
		defer os.RemoveAll(tempDirectory)
	}

	path, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}
	skillCommandPrintInspection(inspected, path)
	if dryRun {
		return 0, nil
	}

	skillsDirectory, err := resolveSkillsDirectory(managed, workdir)
	if err != nil {
		return 1, err
	}

	os.MkdirAll(skillsDirectory, 0755)

	targetDir := filepath.Join(skillsDirectory, inspected.InstallSlug)
	if util.DirectoryExists(targetDir) {
		os.RemoveAll(targetDir)
	}

	if err := util.CopyDirectoryWithoutSymlink(inspected.SkillRootPath, targetDir); err != nil {
		writeSkillsCommandError(asJson, "skills install", "install_failed", err.Error())
		return 1, err
	}
	fmt.Printf("Installed skill '%s' to %s\n", inspected.Definition.Name, targetDir)
	return 0, nil
}

func skillCommandInspect(args []string) (int, error) {
	asJson := slices.Contains(args, "--json")
	sourcePath := ""
	for i := range args {
		if !strings.HasPrefix(args[i], "-") {
			sourcePath = args[i]
			break
		}
	}

	if sourcePath == "" {
		writeSkillsCommandError(asJson, "skills inspect", "missing_source_path", "Usage: openclaw skills inspect <path|tarball>")
		return 2, errors.New("Usage: openclaw skills inspect <path|tarball>")
	}

	return skillCommandInspectSource(sourcePath, false, asJson)
}

func buildRequirementsSummary(definition core.SkillDefinition) string {
	items := []string{}
	if len(definition.Metadata.RequireBins) > 0 {
		items = append(items, "bins=%s", strings.Join(definition.Metadata.RequireBins, ","))
	}
	if len(definition.Metadata.RequireAnyBins) > 0 {
		items = append(items, "anyBins=%s", strings.Join(definition.Metadata.RequireAnyBins, ","))
	}
	if len(definition.Metadata.RequireEnv) > 0 {
		items = append(items, "env=%s", strings.Join(definition.Metadata.RequireEnv, ","))
	}
	if len(definition.Metadata.RequireConfig) > 0 {
		items = append(items, "config=%s", strings.Join(definition.Metadata.RequireConfig, ","))
	}
	if definition.Metadata.Always {
		items = append(items, "always")
	}

	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, " | ")
}

func skillCommandPrintInspection(inspection SkillCommandInspection, installDirectory string) {
	fmt.Printf("Skill: %s\n", inspection.Definition.Name)
	fmt.Printf("Description: %s\n", inspection.Definition.Description)
	fmt.Printf("Trust: %s\n", inspection.TrustLevel)
	fmt.Printf("Trust reason: %s\n", inspection.TrustReason)
	fmt.Printf("Source: %s\n", inspection.SourceLabel)
	fmt.Printf("Path: %s\n", inspection.SkillRootPath)
	fmt.Printf("Install slug: %s\n", inspection.InstallSlug)
	fmt.Printf("User invocable: %t\n", inspection.Definition.UserInvocable)
	fmt.Printf("Disable model invocation: %t\n", inspection.Definition.DisableModelInvocation)
	if inspection.Definition.CommandDispatch != "" {
		fmt.Printf("Command dispatch: %s\n", inspection.Definition.CommandDispatch)
	}
	if inspection.Definition.CommandTool != "" {
		fmt.Printf("Command tool: %s\n", inspection.Definition.CommandTool)
	}
	if inspection.Definition.CommandArgMode != "" {
		fmt.Printf("Command arg mode: %s\n", inspection.Definition.CommandArgMode)
	}
	if inspection.Definition.Metadata.Homepage != "" {
		fmt.Printf("Homepage: %s\n", inspection.Definition.Metadata.Homepage)
	}
	if inspection.Definition.Metadata.PrimaryEnv != "" {
		fmt.Printf("Primary env: %s\n", inspection.Definition.Metadata.PrimaryEnv)
	}
	fmt.Printf("Requirements: %s\n", buildRequirementsSummary(inspection.Definition))
	for _, warning := range inspection.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}

	if installDirectory != "" {
		fmt.Printf("Install target: %s\n", filepath.Join(installDirectory, inspection.InstallSlug))
	}
}

func resolveSkillsDirectory(managed bool, workdir string) (string, error) {
	if workdir != "" {
		path, err := filepath.Abs(workdir)
		if err != nil {
			return "", fmt.Errorf("Missing %s. Set %s or pass --workdir or --managed.", envWorkspace, envWorkspace)
		}
		return filepath.Join(path, "skills"), nil
	}

	if managed {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".openclaw", "skills"), nil
	}

	workspace := os.Getenv(envWorkspace)
	if workspace != "" {
		path, err := filepath.Abs(workspace)
		if err != nil {
			return "", fmt.Errorf("Missing %s. Set %s or pass --workdir or --managed.", envWorkspace, envWorkspace)
		}
		return filepath.Join(path, "skills"), nil
	}

	return "", fmt.Errorf("Missing %s. Set %s or pass --workdir or --managed.", envWorkspace, envWorkspace)
}

func skillCommandInspectResolvedSource(sourcePath string, retainExtractedDirectory bool) (SkillCommandInspection, string) {
	resolvedSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return SkillCommandInspectionFailure(err.Error()), ""
	}

	if util.DirectoryExists(resolvedSourcePath) {
		inspection := core.SkillInspectorInstance.InspectPath(resolvedSourcePath, core.SkillSourceExtra)
		if inspection.Success {
			return createInspection(inspection), ""
		}

		msg := inspection.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("Failed to inspect %s", resolvedSourcePath)
		}
		return SkillCommandInspectionFailure(msg), ""
	}

	if util.FileExists(resolvedSourcePath) && strings.HasSuffix(resolvedSourcePath, ".tgz") {
		tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("openclaw-skill-install-%s", util.CleanUUID())[:24])
		os.MkdirAll(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		result := util.RunProcess(context.Background(), "tar", []string{"--extract", "--gzip", "--file", resolvedSourcePath, "--directory", tempDir}, tempDir, 0, 0, 0)
		path := ""
		if retainExtractedDirectory {
			path = tempDir
		}
		if result.ExitCode != 0 {
			return SkillCommandInspectionFailure(fmt.Sprintf("Failed to extract skill tarball: %s", result.Error)), path
		}

		inspection := core.SkillInspectorInstance.InspectPath(tempDir, core.SkillSourceExtra)
		if !inspection.Success {
			msg := inspection.ErrorMessage
			if msg == "" {
				msg = fmt.Sprintf("Failed to inspect extracted tarball %s", resolvedSourcePath)
			}
			return SkillCommandInspectionFailure(msg), ""
		}

		return createInspection(inspection), path
	}

	return SkillCommandInspectionFailure(fmt.Sprintf("Skill path not found: %s", sourcePath)), ""
}

func slugify(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, value)

	fields := strings.FieldsFunc(mapped, func(r rune) bool {
		return r == '-'
	})

	return strings.Join(fields, "-")
}

func createInspection(inspection *core.SkillInspectionResult) SkillCommandInspection {
	definition := inspection.Definition
	install := definition.Metadata.SkillKey
	if install == "" {
		install = definition.Name
	}
	installSlug := slugify(install)
	trustLevel := "upstream-compatible"
	if definition.Source == core.SkillSourceBundled {
		trustLevel = "first-party"
	}
	trustReason := "Skill document parsed successfully and uses the OpenClaw skill format."
	if definition.Source == core.SkillSourceBundled {
		trustReason = "Skill ships with OpenClaw."
	}

	warnings := []string{}
	if definition.Description != "" {
		warnings = append(warnings, "Skill description is empty.")
	}
	if len(definition.Metadata.RequireBins) == 0 &&
		len(definition.Metadata.RequireAnyBins) == 0 &&
		len(definition.Metadata.RequireEnv) == 0 &&
		len(definition.Metadata.RequireConfig) == 0 {
		warnings = append(warnings, "Skill does not declare host requirements.")
	}

	return SkillCommandInspection{
		Success:       true,
		Definition:    *definition,
		SkillRootPath: inspection.SkillRootPath,
		SkillFilePath: inspection.SkillFilePath,
		InstallSlug:   installSlug,
		SourceLabel:   strings.ToLower(definition.Source.ToString()),
		TrustLevel:    trustLevel,
		TrustReason:   trustReason,
		Warnings:      warnings,
	}
}

func skillCommandInspectSource(sourcePath string, printInstallTarget bool, asJson bool) (int, error) {
	inspected, _ := skillCommandInspectResolvedSource(sourcePath, false)
	if !inspected.Success {
		msg := inspected.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("Failed to inspect source path: %s", sourcePath)
		}
		writeSkillsCommandError(asJson, "skills inspect", "inspect_failed", msg)
		return 1, errors.New(msg)
	}

	installDirectory := ""
	if printInstallTarget {
		path, err := resolveSkillsDirectory(false, "")
		if err != nil {
			return 1, err
		}
		installDirectory = path
	}

	skillCommandPrintInspection(inspected, installDirectory)
	return 0, nil
}
