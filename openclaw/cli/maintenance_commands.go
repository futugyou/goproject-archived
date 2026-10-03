package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/core"
)

func MaintenanceCommandsRun(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stdout, "Usage: openclaw maintenance <scan|fix> [options]")
		return 2
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	parsed := CliArgsParse(args[1:])

	configPathOpt := parsed.GetOption("--config")
	if configPathOpt == nil {
		configPathOpt = new(core.DefaultConfigPath)
	}
	configPath, err := filepath.Abs(core.GatewaySetupPathsIntance.ExpandPath(*configPathOpt))
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	config, err := core.GatewayConfigFileInstance.Load(configPath)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	inputs := &core.MaintenanceScanInputs{
		ConfigPath: configPath,
		// SetupStatus: ?, TODO
		ModelDoctor: core.ModelDoctorEvaluatorInstance.Build(config, nil, nil),
	}

	ctx := context.Background()

	if subcommand == "scan" {
		report, err := core.MaintenanceCoordinatorInstance.Scan(ctx, config, inputs)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}

		if parsed.HasFlag("--json") {
			data, _ := json.Marshal(report)
			fmt.Fprintln(stdout, string(data))
		} else {
			WriteReport(stdout, report)
		}

		if strings.EqualFold(report.OverallStatus, core.SetupCheckStatesFail) {
			return 1
		}
		return 0
	}

	if subcommand == "fix" {
		applyOpt := parsed.GetOption("--apply")
		if applyOpt == nil {
			applyOpt = new("all")
		}

		fixReq := &core.MaintenanceFixRequest{
			DryRun: parsed.HasFlag("--dry-run"),
			Apply:  *applyOpt,
		}

		response, err := core.MaintenanceCoordinatorInstance.Fix(ctx, config, fixReq, inputs)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}

		if parsed.HasFlag("--json") {
			data, _ := json.Marshal(response)
			fmt.Fprintln(stdout, string(data))
		} else {
			WriteFixResponse(stdout, response)
		}

		if response.Success {
			return 0
		}
		return 1
	}

	fmt.Fprintln(stdout, "Usage: openclaw maintenance <scan|fix> [options]")
	return 2
}
func WriteFixResponse(output io.Writer, response *core.MaintenanceFixResponse) {
	msg := "Maintenance fix:"
	if response.DryRun {
		msg = "Maintenance dry-run:"
	}

	fmt.Fprintln(output, msg)
	for _, action := range response.Actions {
		fmt.Fprintf(output, "- %s: %s\n", action.Id, action.Summary)
	}

	if len(response.Warnings) > 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "Warnings:")
		for _, warning := range response.Warnings {
			fmt.Fprintf(output, "- %s\n", warning)

		}
	}

	fmt.Fprintln(output, "")
	fmt.Fprintf(output, "Reliability: %d/100 (%s)\n", response.Reliability.Score, response.Reliability.Status)
}

func WriteReport(output io.Writer, report *core.MaintenanceReportResponse) {
	fmt.Fprintf(output, "Maintenance status: %s\n", report.OverallStatus)
	fmt.Fprintf(output, "Generated at: %s\n", report.GeneratedAtUtc.Format("2006-01-02T15:04:05.0000000Z07:00"))
	fmt.Fprintf(output, "Reliability: %d/100 (%s)\n", report.Reliability.Score, report.Reliability.Status)
	fmt.Fprintf(output, "Storage: memory=%dB archive=%dB orphaned_metadata=%d model_eval_artifacts=%d trace_artifacts=%d\n",
		report.Storage.MemoryBytes,
		report.Storage.ArchiveBytes,
		report.Storage.OrphanedSessionMetadataEntries,
		report.Storage.ModelEvaluationArtifacts,
		report.Storage.PromptCacheTraceArtifacts,
	)
	fmt.Fprintf(output, "Prompt budget: recent_turns=%d p50=%d p95=%d agents_bytes=%d soul_bytes=%d skills=%d\n",
		report.PromptBudget.RecentTurnsAnalyzed,
		report.PromptBudget.P50InputTokens,
		report.PromptBudget.P95InputTokens,
		report.PromptBudget.AgentsFileBytes,
		report.PromptBudget.SoulFileBytes,
		report.PromptBudget.LoadedSkillCount,
	)
	fmt.Fprintf(output, "Drift: retries=%d errors=%d degraded_automations=%d quarantined_automations=%d retention_failures=%d prompt_p95_delta=%d\n",
		report.Drift.ProviderRetries,
		report.Drift.ProviderErrors,
		report.Drift.DegradedAutomations,
		report.Drift.QuarantinedAutomations,
		report.Drift.RetentionFailures,
		report.Drift.PromptP95Delta,
	)

	if len(report.Findings) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Findings:")
		for _, finding := range report.Findings {
			fmt.Fprintf(output, "- [%s] %s\n", finding.Severity, finding.Summary)
			if strings.TrimSpace(finding.RecommendedCommand) != "" {
				fmt.Fprintf(output, "  command: %s\n", finding.RecommendedCommand)
			}
		}
	}

	if len(report.Reliability.Recommendations) > 0 {
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Top recommendations:")
		for _, recommendation := range report.Reliability.Recommendations {
			fmt.Fprintf(output, "- %s: %s\n", recommendation.Summary, recommendation.Command)
		}
	}
}
