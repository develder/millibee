package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewStyles_ReturnsNonZeroStyles(t *testing.T) {
	s := NewStyles(lipgloss.DefaultRenderer())

	// All style fields should be set (non-zero foreground color)
	if s.User.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("User style should have foreground color")
	}
	if s.Assistant.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("Assistant style should have foreground color")
	}
	if s.Error.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("Error style should have foreground color")
	}
	if s.Spinner.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("Spinner style should have foreground color")
	}
	if s.Status.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("Status style should have foreground color")
	}
}

func TestDefaultStyles_MatchesNewStyles(t *testing.T) {
	def := DefaultStyles()
	manual := NewStyles(lipgloss.DefaultRenderer())

	// Both should produce the same foreground colors
	if def.User.GetForeground() != manual.User.GetForeground() {
		t.Error("DefaultStyles().User should match NewStyles().User")
	}
	if def.Assistant.GetForeground() != manual.Assistant.GetForeground() {
		t.Error("DefaultStyles().Assistant should match NewStyles().Assistant")
	}
}
