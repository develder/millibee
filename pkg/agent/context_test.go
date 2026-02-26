package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/develder/millibee/pkg/providers"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeKeepsMultipleToolResults(t *testing.T) {
	// An assistant with 2 tool calls should keep both tool results.
	history := []providers.Message{
		{Role: "user", Content: "do two things"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{
			{ID: "call_1", Name: "tool_a"},
			{ID: "call_2", Name: "tool_b"},
		}},
		{Role: "tool", Content: "result A", ToolCallID: "call_1"},
		{Role: "tool", Content: "result B", ToolCallID: "call_2"},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 4)
	assert.Equal(t, "tool", result[2].Role)
	assert.Equal(t, "call_1", result[2].ToolCallID)
	assert.Equal(t, "tool", result[3].Role)
	assert.Equal(t, "call_2", result[3].ToolCallID)
}

func TestSanitizeKeepsThreeToolResults(t *testing.T) {
	history := []providers.Message{
		{Role: "user", Content: "do three things"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{
			{ID: "c1", Name: "a"},
			{ID: "c2", Name: "b"},
			{ID: "c3", Name: "c"},
		}},
		{Role: "tool", Content: "r1", ToolCallID: "c1"},
		{Role: "tool", Content: "r2", ToolCallID: "c2"},
		{Role: "tool", Content: "r3", ToolCallID: "c3"},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 5)
}

func TestSanitizeDropsOrphanedLeadingTools(t *testing.T) {
	// Tool messages at the start (no parent assistant) should be dropped.
	history := []providers.Message{
		{Role: "tool", Content: "orphan", ToolCallID: "c1"},
		{Role: "user", Content: "hello"},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 1)
	assert.Equal(t, "user", result[0].Role)
}

func TestSanitizeDropsToolsWithoutAssistantParent(t *testing.T) {
	// Tool after user (not after assistant with tool_calls) should be dropped.
	history := []providers.Message{
		{Role: "user", Content: "hello"},
		{Role: "tool", Content: "orphan", ToolCallID: "c1"},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 1)
	assert.Equal(t, "user", result[0].Role)
}

func TestSanitizeDropsAssistantToolCallsAtStart(t *testing.T) {
	history := []providers.Message{
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{
			{ID: "c1", Name: "a"},
		}},
		{Role: "tool", Content: "result", ToolCallID: "c1"},
		{Role: "user", Content: "hello"},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 1)
	assert.Equal(t, "user", result[0].Role)
}

func TestSanitizeDropsTrailingAssistantWithToolCalls(t *testing.T) {
	// An assistant with tool_calls at the end (results were truncated) should be dropped.
	history := []providers.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "let me check", ToolCalls: []providers.ToolCall{
			{ID: "c1", Name: "a"},
		}},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 1)
	assert.Equal(t, "user", result[0].Role)
}

func TestSanitizePreservesCompleteConversation(t *testing.T) {
	// A complete conversation with multiple rounds should be preserved.
	history := []providers.Message{
		{Role: "user", Content: "step 1"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{
			{ID: "c1", Name: "tool_a"},
		}},
		{Role: "tool", Content: "result 1", ToolCallID: "c1"},
		{Role: "assistant", Content: "done with step 1"},
		{Role: "user", Content: "step 2"},
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{
			{ID: "c2", Name: "tool_b"},
			{ID: "c3", Name: "tool_c"},
		}},
		{Role: "tool", Content: "result 2", ToolCallID: "c2"},
		{Role: "tool", Content: "result 3", ToolCallID: "c3"},
		{Role: "assistant", Content: "all done"},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 9)
}

func TestSanitizeHandlesPostCompressionTruncation(t *testing.T) {
	// Simulate what happens after TruncateHistory keeps last 4 messages
	// from a conversation that had tool calls. The first messages might
	// be tool results without their parent assistant.
	history := []providers.Message{
		{Role: "tool", Content: "result", ToolCallID: "c1"},
		{Role: "assistant", Content: "here you go"},
		{Role: "user", Content: "thanks"},
		{Role: "assistant", Content: "you're welcome"},
	}

	result := sanitizeHistoryForProvider(history)
	// The orphaned tool should be dropped
	assert.Len(t, result, 3)
	assert.Equal(t, "assistant", result[0].Role)
	assert.Equal(t, "here you go", result[0].Content)
}

func TestSanitizeEmptyHistory(t *testing.T) {
	result := sanitizeHistoryForProvider([]providers.Message{})
	assert.Empty(t, result)
}

func TestSanitizeToolAfterAssistantWithoutToolCalls(t *testing.T) {
	// Tool message after a plain assistant (no tool_calls) should be dropped.
	history := []providers.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
		{Role: "tool", Content: "stray result", ToolCallID: "c1"},
	}

	result := sanitizeHistoryForProvider(history)
	assert.Len(t, result, 2)
	assert.Equal(t, "user", result[0].Role)
	assert.Equal(t, "assistant", result[1].Role)
}

// --- Bootstrap file tests ---

func TestLoadBootstrapFiles_LoadsSOUL(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("I am MilliBee"), 0o644)

	cb := &ContextBuilder{workspace: workspace}
	result := cb.LoadBootstrapFiles()

	assert.Contains(t, result, "SOUL.md")
	assert.Contains(t, result, "I am MilliBee")
}

func TestLoadBootstrapFiles_LoadsMultiple(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("soul content"), 0o644)
	os.WriteFile(filepath.Join(workspace, "USER.md"), []byte("user prefs"), 0o644)

	cb := &ContextBuilder{workspace: workspace}
	result := cb.LoadBootstrapFiles()

	assert.Contains(t, result, "soul content")
	assert.Contains(t, result, "user prefs")
}

func TestLoadBootstrapFiles_EmptyWorkspace(t *testing.T) {
	workspace := t.TempDir()

	cb := &ContextBuilder{workspace: workspace}
	result := cb.LoadBootstrapFiles()

	assert.Empty(t, result)
}

func TestLoadBootstrapFiles_IgnoresNonBootstrapFiles(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("soul"), 0o644)
	os.WriteFile(filepath.Join(workspace, "RANDOM.md"), []byte("should not appear"), 0o644)

	cb := &ContextBuilder{workspace: workspace}
	result := cb.LoadBootstrapFiles()

	assert.Contains(t, result, "soul")
	assert.NotContains(t, result, "should not appear")
}

// --- System prompt tests ---

func TestBuildSystemPrompt_NoToolSummaries(t *testing.T) {
	workspace := t.TempDir()
	// Create skills dir to avoid nil pointer in SkillsLoader
	os.MkdirAll(filepath.Join(workspace, "skills"), 0o755)

	cb := NewContextBuilder(workspace)
	prompt := cb.BuildSystemPrompt()

	// Tool summaries should NOT be in the system prompt (they come via API tool definitions)
	assert.NotContains(t, prompt, "Available Tools")
	assert.NotContains(t, prompt, "CRITICAL")
	// But identity should still be there
	assert.Contains(t, prompt, "millibee")
}
