package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobTool_MatchTxtFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0o644)

	tool := NewGlobTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "a.txt")
	assert.Contains(t, result.ForLLM, "b.txt")
	assert.NotContains(t, result.ForLLM, "c.go")
}

func TestGlobTool_RecursiveMatch(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755)
	os.WriteFile(filepath.Join(dir, "top.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "mid.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "deep", "bot.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "skip.txt"), []byte(""), 0o644)

	tool := NewGlobTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "**/*.go",
		"path":    dir,
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "top.go")
	assert.Contains(t, result.ForLLM, "mid.go")
	assert.Contains(t, result.ForLLM, "bot.go")
	assert.NotContains(t, result.ForLLM, "skip.txt")
}

func TestGlobTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0o644)

	tool := NewGlobTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.xyz",
		"path":    dir,
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "No files matched")
}

func TestGlobTool_MissingPattern(t *testing.T) {
	tool := NewGlobTool("", false)
	result := tool.Execute(context.Background(), map[string]any{})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "pattern is required")
}

func TestGlobTool_InvalidPattern(t *testing.T) {
	tool := NewGlobTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "[invalid",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "invalid glob pattern")
}

func TestGlobTool_Restricted_OutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	other := t.TempDir()

	tool := NewGlobTool(workspace, true)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    other,
	})

	assert.True(t, result.IsError)
}

func TestGlobTool_Restricted_Success(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "hello.txt"), []byte(""), 0o644)

	tool := NewGlobTool(workspace, true)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    ".",
	})

	assert.False(t, result.IsError, "got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "hello.txt")
}

func TestGlobTool_MaxResults(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 1100; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%04d.txt", i)), []byte(""), 0o644)
	}

	tool := NewGlobTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	assert.False(t, result.IsError)
	lines := strings.Split(strings.TrimSpace(result.ForLLM), "\n")
	// Should be capped — last line should be truncation message
	assert.LessOrEqual(t, len(lines), 1002) // 1000 files + possible truncation message
	assert.Contains(t, result.ForLLM, "truncated")
}

func TestGlobTool_DefaultPath(t *testing.T) {
	// With workspace and restrict, default path should be workspace root
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "found.txt"), []byte(""), 0o644)

	tool := NewGlobTool(workspace, true)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
	})

	assert.False(t, result.IsError, "got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "found.txt")
}
