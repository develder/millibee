package tui

import "strings"

// SlashCommand defines a user-facing slash command.
type SlashCommand struct {
	Name        string // e.g. "/clear"
	Description string // short one-liner
	Usage       string // e.g. "/switch model to <name>"
}

// Commands returns the canonical list of slash commands.
func Commands() []SlashCommand {
	return []SlashCommand{
		{Name: "/clear", Description: "Reset conversation history", Usage: "/clear"},
		{Name: "/status", Description: "Show session info", Usage: "/status"},
		{Name: "/show", Description: "Show current setting", Usage: "/show [model|channel|agents]"},
		{Name: "/list", Description: "List available resources", Usage: "/list [models|channels|agents]"},
		{Name: "/switch", Description: "Change model or channel", Usage: "/switch [model|channel] to <name>"},
		{Name: "/optimize", Description: "Toggle prompt optimization", Usage: "/optimize [on|off]"},
		{Name: "/help", Description: "Show available commands", Usage: "/help"},
	}
}

// MatchCommands returns commands whose name starts with the given prefix.
func MatchCommands(prefix string) []SlashCommand {
	if prefix == "" || prefix[0] != '/' {
		return nil
	}
	prefix = strings.ToLower(prefix)
	var matches []SlashCommand
	for _, cmd := range Commands() {
		if strings.HasPrefix(cmd.Name, prefix) {
			matches = append(matches, cmd)
		}
	}
	return matches
}

// FormatHelp returns a formatted help string for all commands.
func FormatHelp() string {
	var sb strings.Builder
	sb.WriteString("Available commands:\n\n")
	for _, cmd := range Commands() {
		sb.WriteString("  ")
		sb.WriteString(cmd.Usage)
		sb.WriteString("\n    ")
		sb.WriteString(cmd.Description)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
