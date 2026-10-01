package tui

import (
	"bufio"
	"fmt"
	"os"

	"charm.land/bubbles/v2/table"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
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

func ask(title, description string) (string, error) {
	var search string
	input := huh.NewInput().Title(title)
	if description != "" {
		input.Description(description)
	}

	if err := input.
		Value(&search).
		Run(); err != nil {
		return search, err
	}

	return search, nil
}
