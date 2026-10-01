package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/futugyou/openclaw/client"
	"github.com/futugyou/openclaw/core"
)

var mutedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240"))

func pause() {
	fmt.Println(mutedStyle.Render("Press enter to continue..."))
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

var box = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1)

func titledTable(title string, t table.Model) string {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(title),
		t.View(),
	)
	return box.Render(content)
}

func panel(title, content string) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	bodyStyle := lipgloss.NewStyle().
		Padding(1, 2)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder())

	return border.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render(title),
			bodyStyle.Render(content),
		),
	)
}

func confirm(title string, defaultValue bool) (bool, error) {
	value := defaultValue

	err := huh.NewConfirm().
		Title(title).
		Value(&value).
		Run()

	return value, err
}

type TerminalUi struct {
	client *client.OpenClawHttpClient
}

func RunAsync(ctx context.Context, baseUrl, authToken, presetId string) error {
	client, err := client.NewOpenClawHttpClient(baseUrl, authToken, nil)
	if err != nil {
		return err
	}

	ui := &TerminalUi{client: client}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Print("\033[H\033[2J")
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true).
			MarginLeft(2)
		fmt.Println(titleStyle.Render("─── OpenClaw Terminal UI ───"))
		fmt.Println()

		var choice string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select an action").
					Options(
						huh.NewOption("Status", "Status"),
						huh.NewOption("Insights", "Insights"),
						huh.NewOption("Approvals", "Approvals"),
						huh.NewOption("Sessions", "Sessions"),
						huh.NewOption("Session Search", "Session Search"),
						huh.NewOption("Automations", "Automations"),
						huh.NewOption("Learning Proposals", "Learning Proposals"),
						huh.NewOption("Profiles", "Profiles"),
						huh.NewOption("Tool Presets", "Tool Presets"),
						huh.NewOption("Live Session", "Live Session"),
						huh.NewOption("Chat", "Chat"),
						huh.NewOption("Exit", "Exit"),
					).
					Value(&choice),
			),
		)

		if err := form.Run(); err != nil {
			return err
		}

		switch choice {
		case "Status":
			ui.showStatus(ctx)
		case "Insights":
			ui.ShowInsights(ctx)
		case "Approvals":
			ui.ShowApprovals(ctx)
		case "Sessions":
			ui.ShowSessions(ctx)
		case "Session Search":
			ui.ShowSessionSearch(ctx)
		case "Automations":
			ui.ShowAutomations(ctx)
		case "Learning Proposals":
			ui.ShowLearningProposals(ctx)
		case "Profiles":
			ui.ShowProfiles(ctx)
		case "Tool Presets":
			ui.ShowToolPresets(ctx)
		case "Exit":
			return nil
		}
	}
}

func (ui *TerminalUi) showStatus(ctx context.Context) error {
	dashboard, err := ui.client.GetIntegrationDashboard(ctx)
	if err != nil {
		return err
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Metric", Width: 28},
			{Title: "Value", Width: 40},
		}),
		table.WithRows([]table.Row{
			{"Health", dashboard.Status.Health.Status},
			{"Active sessions", strconv.Itoa(dashboard.Status.ActiveSessions)},
			{"Pending approvals", strconv.Itoa(dashboard.Status.PendingApprovals)},
			{"Approval grants", strconv.Itoa(dashboard.Status.ActiveApprovalGrants)},
			{"Plugin health items", strconv.Itoa(len(dashboard.Plugins.Items))},
			{"Recent runtime events", strconv.Itoa(len(dashboard.Events.Items))},
		}),
	)

	fmt.Println(t.View())
	pause()

	return nil
}

func (ui *TerminalUi) ShowInsights(ctx context.Context) error {
	insights, err := ui.client.GetOperatorInsights(ctx, nil, nil)
	if err != nil {
		return err
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Metric", Width: 28},
			{Title: "Value", Width: 40},
		}),
		table.WithRows([]table.Row{
			{"Window", fmt.Sprintf("%s UTC - %s UTC", insights.StartUtc.Format(time.RFC3339Nano), insights.EndUtc.Format(time.RFC3339Nano))},
			{"Sessions", fmt.Sprintf("active %d, persisted %d, range %d", insights.Sessions.Active, insights.Sessions.Persisted, insights.Sessions.InRange)},
			{"Provider requests", fmt.Sprintf("%d", insights.Totals.ProviderRequests)},
			{"Tokens", fmt.Sprintf("%d (%d in / %d out)", insights.Totals.TotalTokens(), insights.Totals.InputTokens, insights.Totals.OutputTokens)},
			{"Estimated spend", fmt.Sprintf("%f", insights.Totals.EstimatedCostUsd)},
			{"Tool calls", fmt.Sprintf("%d", insights.Totals.ToolCalls)},
		}),
	)

	fmt.Println(t.View())

	rows := []table.Row{}
	if len(insights.Providers) == 0 {
		rows = []table.Row{{"none", "-", "-", "-", "-"}}
	} else {
		for i := 0; i < min(8, len(insights.Providers)); i++ {
			item := insights.Providers[i]
			rows = append(rows, table.Row{
				fmt.Sprintf("%s/%s", item.ProviderId, item.ModelId),
				fmt.Sprintf("%d", item.Requests),
				fmt.Sprintf("%d", item.TotalTokens()),
				fmt.Sprintf("%f", item.EstimatedCostUsd),
				fmt.Sprintf("%d", item.Errors),
			})
		}
	}

	providers := table.New(
		table.WithColumns([]table.Column{
			{Title: "Provider"},
			{Title: "Requests"},
			{Title: "Tokens"},
			{Title: "Cost"},
			{Title: "Errors"},
		}),
		table.WithRows(rows),
	)

	fmt.Println(titledTable("Providers", providers))

	toolsrows := []table.Row{}
	if len(insights.Tools) == 0 {
		toolsrows = []table.Row{{"none", "-", "-", "-"}}
	} else {
		for i := 0; i < min(10, len(insights.Tools)); i++ {
			item := insights.Tools[i]
			toolsrows = append(toolsrows, table.Row{
				item.ToolName,
				fmt.Sprintf("%d", item.Calls),
				fmt.Sprintf("%d", item.Failures),
				fmt.Sprintf("%f", item.AverageDurationMs),
			})
		}
	}

	tools := table.New(
		table.WithColumns([]table.Column{
			{Title: "Tool"},
			{Title: "Calls"},
			{Title: "Failures"},
			{Title: "Avg ms"},
		}),
		table.WithRows(rows),
	)

	fmt.Println(titledTable("Tools", tools))

	pause()

	return nil
}

func (ui *TerminalUi) ShowApprovals(ctx context.Context) error {
	approvals, err := ui.client.GetIntegrationApprovals(ctx, "", "")
	if err != nil {
		return err
	}

	rows := []table.Row{}
	for _, item := range approvals.Items {
		rows = append(rows, table.Row{
			item.ApprovalId,
			item.ToolName,
			item.ChannelId,
			item.SenderId,
		})
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Approval ID"},
			{Title: "Tool"},
			{Title: "Channel"},
			{Title: "Sender"},
		}),
		table.WithRows(rows),
	)

	if len(approvals.Items) == 0 {
		fmt.Println(mutedStyle.Render("No pending approvals"))
	} else {
		fmt.Println(t.View())
	}

	pause()

	return nil
}

func (ui *TerminalUi) ShowSessions(ctx context.Context) error {
	var search string

	if err := huh.NewInput().
		Title("Session filter").
		Description("blank for all").
		Value(&search).
		Run(); err != nil {
		return err
	}

	sessions, err := ui.client.ListSessions(ctx, 1, 25, core.SessionListQuery{
		Search: search,
	})
	if err != nil {
		return err
	}

	rows := []table.Row{}
	for _, item := range append(sessions.Active, sessions.Persisted.Items...) {
		if len(rows) >= 25 {
			break
		}
		rows = append(rows, table.Row{
			item.Id,
			item.ChannelId,
			item.SenderId,
			item.State.String(),
			strconv.Itoa(item.HistoryTurns),
		})
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Session ID"},
			{Title: "Channel"},
			{Title: "Sender"},
			{Title: "State"},
			{Title: "Turns"},
		}),
		table.WithRows(rows),
	)

	if len(rows) == 0 {
		fmt.Println(mutedStyle.Render("No pending approvals"))
	} else {
		fmt.Println(t.View())
	}

	var sessionId string

	if err := huh.NewInput().
		Title("Inspect session ID").
		Description("blank to return").
		Value(&sessionId).
		Run(); err != nil {
		return err
	}

	if sessionId == "" {
		return nil
	}

	detail, err := ui.client.GetSession(ctx, sessionId)
	if err != nil || (detail.Session == nil) {
		fmt.Println(mutedStyle.Render("Session not found"))
		pause()
		return nil
	}

	panelTexts := []string{}
	for _, v := range detail.Session.History[max(0, len(detail.Session.History)-12):] {
		panelTexts = append(panelTexts, fmt.Sprintf("%s: %s", v.Role, v.Content))
	}

	panelText := "(empty session)"
	if len(panelTexts) > 0 {
		panelText = strings.Join(panelTexts, "\n\n")
	}
	fmt.Println(panel(detail.Session.Id, panelText))

	fmt.Println(mutedStyle.Render(fmt.Sprintf("Active preset: %s", detail.Metadata.ActivePresetId)))

	update, err := confirm("Update this session's preset?", false)
	if err != nil {
		return err
	}

	if update {
		presets, err := ui.client.ListToolPresets(ctx)
		if err != nil {
			return err
		}
		var selected string
		options := []huh.Option[string]{}
		for _, choice := range presets.Items {
			options = append(options, huh.NewOption(choice.PresetId, choice.PresetId))
		}

		if err := huh.NewSelect[string]().
			Title("Preset").
			Options(options...).
			Value(&selected).
			Run(); err != nil {
			return err
		}

		request := core.SessionMetadataUpdateRequest{
			ActivePresetId: selected,
		}
		if detail.Metadata != nil {
			request.Starred = &detail.Metadata.Starred
			request.Tags = detail.Metadata.Tags
			request.TodoItems = detail.Metadata.TodoItems
		}

		ui.client.UpdateSessionMetadata(ctx, detail.Session.Id, request)
		fmt.Println(mutedStyle.Render(fmt.Sprintf("Preset updated to %s", selected)))
	}

	pause()
	return nil
}

func (ui *TerminalUi) ShowSessionSearch(ctx context.Context) error {
	var search string

	if err := huh.NewInput().
		Title("Search text").
		Value(&search).
		Run(); err != nil {
		return err
	}

	sessions, err := ui.client.SearchSessions(ctx, core.SessionSearchQuery{
		Text:  search,
		Limit: 25,
	})
	if err != nil {
		return err
	}
	rows := []table.Row{}
	for _, item := range sessions.Result.Items {
		rows = append(rows, table.Row{
			item.SessionId,
			item.Role,
			fmt.Sprintf("%f", item.Score),
			item.Snippet,
		})
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Session"},
			{Title: "Role"},
			{Title: "Score"},
			{Title: "Snippet"},
		}),
		table.WithRows(rows),
	)

	if len(rows) == 0 {
		fmt.Println(mutedStyle.Render("No session hits found"))
	} else {
		fmt.Println(t.View())
	}

	pause()
	return nil
}

func (ui *TerminalUi) ShowAutomations(ctx context.Context) error {
	automations, err := ui.client.GetAdminAutomations(ctx)
	if err != nil {
		return err
	}
	rows := []table.Row{}
	for _, item := range automations.Items {
		rows = append(rows, table.Row{
			item.Id,
			item.Name,
			item.Schedule,
			strconv.FormatBool(item.Enabled),
			item.Source,
		})
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "ID"},
			{Title: "Name"},
			{Title: "Schedule"},
			{Title: "Enabled"},
			{Title: "Source"},
		}),
		table.WithRows(rows),
	)

	if len(rows) == 0 {
		fmt.Println(mutedStyle.Render("No automations found"))
	} else {
		fmt.Println(t.View())
	}

	var selectedId string

	if err := huh.NewInput().
		Title("Automation ID to inspect/run ").
		Description("blank to return").
		Value(&selectedId).
		Run(); err != nil {
		return err
	}

	detail, err := ui.client.GetAdminAutomation(ctx, selectedId)
	if err != nil || detail.Automation == nil {
		fmt.Println(mutedStyle.Render("Automation not found"))
		pause()
		return nil
	}

	fmt.Println(panel(detail.Automation.Prompt, fmt.Sprintf("%s [%s]", detail.Automation.Name, detail.Automation.Id)))

	update, err := confirm("Queue this automation now?", false)
	if err != nil {
		return err
	}

	if update {
		result, err := ui.client.RunAdminAutomation(ctx, selectedId, false)
		if err != nil {
			return err
		}

		if result.Success {
			fmt.Println(mutedStyle.Render(result.Message))
		} else {
			fmt.Println(mutedStyle.Render(result.Error))
		}
	}

	pause()
	return nil
}

func (ui *TerminalUi) ShowLearningProposals(ctx context.Context) error {
	proposals, err := ui.client.ListLearningProposals(ctx, "pending", "")
	if err != nil {
		return err
	}
	rows := []table.Row{}
	for _, item := range proposals.Items {
		rows = append(rows, table.Row{
			item.Id,
			item.Kind,
			item.RiskLevel,
			item.Title,
			fmt.Sprintf("%f", item.Confidence),
		})
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "ID"},
			{Title: "Kind"},
			{Title: "Risk"},
			{Title: "Title"},
			{Title: "Confidence"},
		}),
		table.WithRows(rows),
	)

	if len(rows) == 0 {
		fmt.Println(mutedStyle.Render("No pending learning proposals"))
	} else {
		fmt.Println(t.View())
	}

	var proposalId string

	if err := huh.NewInput().
		Title("Proposal ID to review").
		Description("blank to return").
		Value(&proposalId).
		Run(); err != nil || proposalId == "" {
		return err
	}

	var proposal *core.LearningProposal
	for _, v := range proposals.Items {
		if proposalId == v.Id {
			proposal = &v
			break
		}
	}

	if proposal == nil {
		fmt.Println(mutedStyle.Render("Proposal not found in current listing."))
		pause()
		return nil
	}

	detail, _ := ui.client.GetLearningProposalDetail(ctx, proposalId)
	if detail != nil && detail.Proposal != nil {
		proposal = detail.Proposal
	}

	sb := &strings.Builder{}
	fmt.Fprintf(sb, "Risk: %s\n", proposal.RiskLevel)
	fmt.Fprintf(sb, "Repeated count: %d\n", proposal.RepeatedCount)
	fmt.Fprintf(sb, "Sources: %s\n", strings.Join(proposal.SourceSessionIds, ", "))
	if len(proposal.ValidationWarnings) == 0 {
		sb.WriteString("Warnings: none\n")
	} else {
		fmt.Fprintf(sb, "Warnings: %s\n", strings.Join(proposal.ValidationWarnings, "; "))
	}

	if harness := proposal.HarnessEvolution; harness != nil {
		sb.WriteString("\n")
		fmt.Fprintf(sb, "Component: %s\n", harness.Component)
		fmt.Fprintf(sb, "Failure mode: %s\n", harness.FailureMode)
		fmt.Fprintf(sb, "Proposed change: %s\n", harness.ProposedChange)
		fmt.Fprintf(sb, "Apply mode: %s\n", harness.ApplyMode)
		fmt.Fprintf(sb, "Regression: %s\n", strings.Join(harness.RegressionCategories, ", "))
		fmt.Fprintf(sb, "Rollback plan: %s\n", harness.RollbackPlan)
	}

	sb.WriteString("\n")

	sum := proposal.DraftContent
	if sum == "" {
		sum = proposal.DraftPreview
	}
	if sum == "" && proposal.AutomationDraft != nil {
		sum = proposal.AutomationDraft.Prompt
	}
	if sum == "" && proposal.ProfileUpdate != nil {
		sum = proposal.ProfileUpdate.Summary
	}

	sb.WriteString("\n")
	if sum != "" {
		sb.WriteString(sum)
		sb.WriteString("\n")
	}

	fmt.Println(panel(proposal.Title, sb.String()))

	var action string
	options := []huh.Option[string]{}
	if detail != nil && detail.CanRollback {
		for _, v := range []string{"Approve", "Reject", "Rollback", "Return"} {
			options = append(options, huh.NewOption(v, v))
		}
	} else {
		for _, v := range []string{"Approve", "Reject", "Return"} {
			options = append(options, huh.NewOption(v, v))
		}
	}

	if err := huh.NewSelect[string]().
		Title("Review action").
		Options(options...).
		Value(&action).
		Run(); err != nil {
		return err
	}

	switch action {
	case "Approve":
		ui.client.ApproveLearningProposal(ctx, proposalId)
		fmt.Println(mutedStyle.Render("Proposal approved."))
	case "Reject":
		var reason string

		if err := huh.NewInput().
			Title("Reason").
			Description("optional").
			Value(&reason).
			Run(); err != nil {
			return err
		}

		ui.client.RejectLearningProposal(ctx, proposalId, reason)
		fmt.Println(mutedStyle.Render("Proposal rejected."))

	case "Rollback":
		var reason string

		if err := huh.NewInput().
			Title("Rollback reason").
			Description("optional").
			Value(&reason).
			Run(); err != nil {
			return err
		}

		ui.client.RollbackLearningProposal(ctx, proposalId, reason)
		fmt.Println(mutedStyle.Render("Proposal rolled back."))

	}

	pause()
	return nil

}

func (ui *TerminalUi) ShowProfiles(ctx context.Context) error {
	profiles, err := ui.client.ListProfiles(ctx)
	if err != nil {
		return err
	}
	rows := []table.Row{}
	for i, item := range profiles.Items {
		if i >= 25 {
			break
		}
		rows = append(rows, table.Row{
			item.ActorId,
			item.Tone,
			item.Summary,
		})
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Actor"},
			{Title: "Tone"},
			{Title: "Summary"},
		}),
		table.WithRows(rows),
	)

	if len(rows) == 0 {
		fmt.Println(mutedStyle.Render("No profiles found."))
	} else {
		fmt.Println(t.View())
	}

	var actorId string

	if err := huh.NewInput().
		Title("Actor ID to inspect/edit").
		Description("blank to return").
		Value(&actorId).
		Run(); err != nil || actorId == "" {
		return err
	}

	response, err := ui.client.GetProfile(ctx, actorId)
	if err != nil || response == nil || response.Profile == nil {
		fmt.Println(mutedStyle.Render("Profile not found"))
		pause()
		return nil
	}

	var profile = response.Profile

	content := profile.Summary
	if content == "" {
		content = "(no summary)"
	}
	fmt.Println(panel(profile.ActorId, content))

	update, err := confirm("Edit this profile?", false)
	if err != nil {
		return err
	}

	if !update {
		return nil
	}

	var updatedSummary string

	if err := huh.NewInput().
		Title("Summary").
		Value(&updatedSummary).
		Run(); err != nil || updatedSummary == "" {
		return err
	}

	var updatedTone string

	if err := huh.NewInput().
		Title("Tone").
		Value(&updatedTone).
		Run(); err != nil || updatedTone == "" {
		return err
	}

	ui.client.SaveProfile(ctx, actorId, core.UserProfile{
		ActorId:        profile.ActorId,
		ChannelId:      profile.ChannelId,
		SenderId:       profile.SenderId,
		Summary:        updatedSummary,
		Tone:           updatedTone,
		Facts:          profile.Facts,
		Preferences:    profile.Preferences,
		ActiveProjects: profile.ActiveProjects,
		RecentIntents:  profile.RecentIntents,
		UpdatedAtUtc:   time.Now(),
	})

	fmt.Println(mutedStyle.Render("Profile saved."))
	pause()
	return nil
}

func (ui *TerminalUi) ShowToolPresets(ctx context.Context) error {
	presets, err := ui.client.ListToolPresets(ctx)
	if err != nil || len(presets.Items) == 0 {

		fmt.Println(mutedStyle.Render("No presets available"))
		pause()
		return nil
	}

	rows := []table.Row{}
	for _, item := range presets.Items {
		re := "required"
		if !item.RequireToolApproval {
			re = "not required"
		}
		rows = append(rows, table.Row{
			item.PresetId,
			item.EffectiveAutonomyMode,
			re,
			item.Description,
		})
	}

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Preset"},
			{Title: "Autonomy"},
			{Title: "Approval"},
			{Title: "Description"},
		}),
		table.WithRows(rows),
	)

	fmt.Println(t.View())
	pause()
	return nil
}
