package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandsReturnsAll(t *testing.T) {
	cmds := Commands()
	require.NotEmpty(t, cmds)

	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}

	assert.Contains(t, names, "/clear")
	assert.Contains(t, names, "/status")
	assert.Contains(t, names, "/help")
	assert.Contains(t, names, "/show")
	assert.Contains(t, names, "/list")
	assert.Contains(t, names, "/switch")
	assert.Contains(t, names, "/optimize")
}

func TestMatchCommandsExactMatch(t *testing.T) {
	matches := MatchCommands("/clear")
	require.Len(t, matches, 1)
	assert.Equal(t, "/clear", matches[0].Name)
}

func TestMatchCommandsPartialMatch(t *testing.T) {
	matches := MatchCommands("/s")
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Name
	}
	assert.Contains(t, names, "/status")
	assert.Contains(t, names, "/show")
	assert.Contains(t, names, "/switch")
}

func TestMatchCommandsSlashOnly(t *testing.T) {
	matches := MatchCommands("/")
	// Should return all commands
	assert.Equal(t, len(Commands()), len(matches))
}

func TestMatchCommandsNoMatch(t *testing.T) {
	matches := MatchCommands("/zzz")
	assert.Empty(t, matches)
}

func TestMatchCommandsEmptyString(t *testing.T) {
	matches := MatchCommands("")
	assert.Nil(t, matches)
}

func TestMatchCommandsNoSlash(t *testing.T) {
	matches := MatchCommands("clear")
	assert.Nil(t, matches)
}

func TestFormatHelpContainsAllCommands(t *testing.T) {
	help := FormatHelp()
	for _, cmd := range Commands() {
		assert.Contains(t, help, cmd.Usage, "help should contain usage for %s", cmd.Name)
		assert.Contains(t, help, cmd.Description, "help should contain description for %s", cmd.Name)
	}
}

func TestFormatHelpStartsWithHeader(t *testing.T) {
	help := FormatHelp()
	assert.True(t, strings.HasPrefix(help, "Available commands:"))
}
