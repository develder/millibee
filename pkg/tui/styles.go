package tui

import "github.com/charmbracelet/lipgloss"

var (
	UserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	AssistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	StatusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("248")).
			Padding(0, 1)
)
