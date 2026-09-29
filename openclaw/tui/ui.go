package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/futugyou/openclaw/client"
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
