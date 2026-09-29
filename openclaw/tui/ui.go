package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"

	"charm.land/bubbles/v2/table"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/futugyou/openclaw/client"
)

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

var mutedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240"))

func pause() {
	fmt.Println(mutedStyle.Render("Press enter to continue..."))
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
