package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds all TUI styles, created via a renderer for correct
// color profile detection (important for SSH sessions).
type Styles struct {
	User      lipgloss.Style
	Assistant lipgloss.Style
	Error     lipgloss.Style
	Spinner   lipgloss.Style
	Status    lipgloss.Style
}

// NewStyles creates styles using the given renderer.
// Use bubbletea.MakeRenderer(session) for SSH sessions.
func NewStyles(r *lipgloss.Renderer) Styles {
	return Styles{
		User: r.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true),
		Assistant: r.NewStyle().
			Foreground(lipgloss.Color("76")),
		Error: r.NewStyle().
			Foreground(lipgloss.Color("196")),
		Spinner: r.NewStyle().
			Foreground(lipgloss.Color("205")),
		Status: r.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("248")).
			Padding(0, 1),
	}
}

// DefaultStyles creates styles using the default renderer (local terminal).
func DefaultStyles() Styles {
	return NewStyles(lipgloss.DefaultRenderer())
}

// Package-level vars kept for backward compatibility during transition.
var (
	UserStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	AssistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	ErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	SpinnerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	StatusStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("248")).Padding(0, 1)
)
