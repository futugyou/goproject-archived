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
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

const (
	MetaRunProposalMetadataSessionId                         = "meta_run_proposal_session_id"
	MetaRunProposalMetadataProposalId                        = "meta_run_proposal_id"
	MetaRunProposalMetadataRunId                             = "meta_run_proposal_run_id"
	MetaRunProposalMetadataProposalStatus                    = "meta_run_proposal_status"
	MetaRunProposalMetadataProposalKind                      = "meta_run_proposal_kind"
	MetaRunProposalMetadataReason                            = "meta_run_proposal_reason"
	MetaRunProposalMetadataSource                            = "meta_run_proposal_source"
	MetaRunProposalMetadataProvenanceSnapshotVersion         = "meta_run_proposal_provenance_snapshot_version"
	MetaRunProposalMetadataProvenanceCapturedAtUtc           = "meta_run_proposal_provenance_captured_at_utc"
	MetaRunProposalMetadataProvenanceRunStatus               = "meta_run_proposal_provenance_run_status"
	MetaRunProposalMetadataProvenanceRunStartedAtUtc         = "meta_run_proposal_provenance_run_started_at_utc"
	MetaRunProposalMetadataProvenanceRunCompletedAtUtc       = "meta_run_proposal_provenance_run_completed_at_utc"
	MetaRunProposalMetadataProvenanceStepCount               = "meta_run_proposal_provenance_step_count"
	MetaRunProposalMetadataProvenanceStepIds                 = "meta_run_proposal_provenance_step_ids"
	MetaRunProposalMetadataProvenanceCheckpointPendingStepId = "meta_run_proposal_provenance_checkpoint_pending_step_id"
	MetaRunProposalMetadataProvenanceCheckpointPromptPresent = "meta_run_proposal_provenance_checkpoint_prompt_present"
	MetaRunProposalMetadataTransitionCount                   = "meta_run_proposal_transition_count"
	MetaRunProposalMetadataTransitionEntryPrefix             = "meta_run_proposal_transition"
	MetaRunProposalMetadataTransitionFieldAction             = "action"
	MetaRunProposalMetadataTransitionFieldFromStatus         = "from_status"
	MetaRunProposalMetadataTransitionFieldToStatus           = "to_status"
	MetaRunProposalMetadataTransitionFieldChangedAtUtc       = "changed_at_utc"
	MetaRunProposalMetadataTransitionFieldActorId            = "actor_id"
	MetaRunProposalMetadataTransitionFieldReason             = "reason"
	MetaRunProposalMetadataAuditSchemaVersion                = "meta_run_proposal_audit_schema_version"
	MetaRunProposalMetadataLastTransitionAction              = "meta_run_proposal_last_transition_action"
	MetaRunProposalMetadataLastTransitionChangedAtUtc        = "meta_run_proposal_last_transition_changed_at_utc"
	MetaRunProposalMetadataLastTransitionActorId             = "meta_run_proposal_last_transition_actor_id"
	MetaRunProposalMetadataAcceptGateProfile                 = "meta_run_proposal_accept_gate_profile"
	MetaRunProposalMetadataAcceptGatePassed                  = "meta_run_proposal_accept_gate_passed"
	MetaRunProposalMetadataAcceptGateFailedChecks            = "meta_run_proposal_accept_gate_failed_checks"
	MetaRunProposalMetadataAcceptGateCheckedAtUtc            = "meta_run_proposal_accept_gate_checked_at_utc"

	MetaRunWorkflowMetadataSessionId                   = "meta_run_workflow_session_id"
	MetaRunWorkflowMetadataProposalId                  = "meta_run_workflow_proposal_id"
	MetaRunWorkflowMetadataWorkflowId                  = "meta_run_workflow_id"
	MetaRunWorkflowMetadataStage                       = "meta_run_workflow_stage"
	MetaRunWorkflowMetadataLastAction                  = "meta_run_workflow_last_action"
	MetaRunWorkflowMetadataLastActorId                 = "meta_run_workflow_last_actor_id"
	MetaRunWorkflowMetadataLastChangedAtUtc            = "meta_run_workflow_last_changed_at_utc"
	MetaRunWorkflowMetadataTransitionCount             = "meta_run_workflow_transition_count"
	MetaRunWorkflowMetadataTransitionEntryPrefix       = "meta_run_workflow_transition"
	MetaRunWorkflowMetadataTransitionFieldAction       = "action"
	MetaRunWorkflowMetadataTransitionFieldStage        = "stage"
	MetaRunWorkflowMetadataTransitionFieldChangedAtUtc = "changed_at_utc"
	MetaRunWorkflowMetadataTransitionFieldActorId      = "actor_id"
	MetaRunWorkflowMetadataTransitionFieldReason       = "reason"
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

	asJson := slices.Contains(args, "--json")
	verbose := slices.Contains(args, "--verbose")
	requestedRunId := getOptionValue(args, "--run")
	sessionId := ""
	for i := 0; i < len(args); i++ {
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
	for i := 0; i < len(args); i++ {
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
			return b.StartedAtUtc.Compare(a.CompletedAtUtc)
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
		value, ok := tryGetMetaRunProposalMetadata(&item, MetaRunProposalMetadataSessionId)
		if ok && value == sessionid {
			_, ok := tryGetMetaRunProposalMetadata(&item, MetaRunProposalMetadataProposalId)
			if ok {
				result[item.Metadata[MetaRunProposalMetadataProposalId]] = util.Deref(toMetaRunProposalReviewRecord(item, sessionid, item.Metadata[MetaRunProposalMetadataProposalId]))
			}
		}
	}
	return result
}

func listMetaRunProposals(args []string, entrypoint string) (int, error) {
	asJson := slices.Contains(args, "--json")
	requestedRunId := getOptionValue(args, "--run")
	sessionId := ""
	for i := 0; i < len(args); i++ {
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

	snapshotVersion, ok := tryGetMetaRunProposalMetadata(proposal, MetaRunProposalMetadataProvenanceSnapshotVersion)
	if !ok {
		return nil
	}

	capturedAtRaw, ok := tryGetMetaRunProposalMetadata(proposal, MetaRunProposalMetadataProvenanceCapturedAtUtc)
	if !ok {
		return nil
	}

	capturedAtUtc, err := time.Parse(time.RFC3339, capturedAtRaw)
	if err != nil {
		return nil
	}

	runStatus, ok := tryGetMetaRunProposalMetadata(proposal, MetaRunProposalMetadataProvenanceRunStatus)
	if !ok {
		return nil
	}

	runStartedRaw, ok := tryGetMetaRunProposalMetadata(proposal, MetaRunProposalMetadataProvenanceRunStartedAtUtc)
	if !ok {
		return nil
	}

	runStartedAtUtc, err := time.Parse(time.RFC3339, runStartedRaw)
	if err != nil {
		return nil
	}

	runCompletedRaw, ok := tryGetMetaRunProposalMetadata(proposal, MetaRunProposalMetadataProvenanceRunCompletedAtUtc)
	if !ok {
		return nil
	}

	runCompletedAtUtc, err := time.Parse(time.RFC3339, runCompletedRaw)
	if err != nil {
		return nil
	}

	stepCountRaw, ok := tryGetMetaRunProposalMetadata(proposal, MetaRunProposalMetadataProvenanceStepCount)
	if !ok {
		return nil
	}

	stepCount, err := strconv.ParseInt(stepCountRaw, 10, 64)
	if err != nil {
		return nil
	}

	stepIdsRaw := proposal.Metadata[MetaRunProposalMetadataProvenanceStepIds]
	checkpointPendingStepId := proposal.Metadata[MetaRunProposalMetadataProvenanceCheckpointPendingStepId]
	checkpointPromptPresentRaw := proposal.Metadata[MetaRunProposalMetadataProvenanceCheckpointPromptPresent]
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

	schemaVersion := proposal.Metadata[MetaRunProposalMetadataAuditSchemaVersion]
	if schemaVersion == "" {
		schemaVersion = "v1"
	}

	actorId := proposal.Metadata[MetaRunProposalMetadataLastTransitionActorId]
	transitionAction := proposal.Metadata[MetaRunProposalMetadataLastTransitionAction]
	changedAtRaw := proposal.Metadata[MetaRunProposalMetadataLastTransitionChangedAtUtc]

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
	return fmt.Sprintf("%s_%04d_%s", MetaRunProposalMetadataTransitionEntryPrefix, index, field)
}

func buildMetaRunProposalProvenanceHistory(proposal *core.LearningProposal) []core.MetaRunProposalProvenanceTransition {
	if proposal == nil {
		return []core.MetaRunProposalProvenanceTransition{}
	}

	countRaw := proposal.Metadata[MetaRunProposalMetadataTransitionCount]
	count, err := strconv.ParseInt(countRaw, 10, 64)
	if count <= 0 || err != nil {
		return []core.MetaRunProposalProvenanceTransition{}
	}

	transitions := make([]core.MetaRunProposalProvenanceTransition, count)
	for index := int64(0); index < count; index++ {
		action, ok := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, MetaRunProposalMetadataTransitionFieldAction)]
		if !ok || action == "" {
			continue
		}
		fromStatus, ok := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, MetaRunProposalMetadataTransitionFieldFromStatus)]
		if !ok || fromStatus == "" {
			continue
		}
		toStatus, ok := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, MetaRunProposalMetadataTransitionFieldToStatus)]
		if !ok || toStatus == "" {
			continue
		}
		changedAtUtc, err := time.Parse(time.RFC3339, proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, MetaRunProposalMetadataTransitionFieldChangedAtUtc)])
		if err != nil {
			continue
		}

		reason := proposal.Metadata[buildMetaRunProposalTransitionMetadataKey(index, MetaRunProposalMetadataTransitionFieldReason)]
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
	for i := 0; i < len(args); i++ {
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
	for i := 0; i < len(args); i++ {
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
