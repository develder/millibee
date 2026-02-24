package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrepTool_SimpleMatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("Hello World\nGoodbye World\nHello Again"), 0o644)

	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "Hello",
		"path":    dir,
	})

	assert.False(t, result.IsError, "got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "Hello World")
	assert.Contains(t, result.ForLLM, "Hello Again")
	// Should show line numbers
	assert.Contains(t, result.ForLLM, ":1:")
	assert.Contains(t, result.ForLLM, ":3:")
}

func TestGrepTool_RegexMatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("func main() {\n\tfmt.Println(\"hi\")\n}\nfunc helper() {}"), 0o644)

	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": `func\s+\w+`,
		"path":    dir,
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "func main")
	assert.Contains(t, result.ForLLM, "func helper")
}

func TestGrepTool_WithGlobFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "match.go"), []byte("Hello from go"), 0o644)
	os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("Hello from txt"), 0o644)

	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "Hello",
		"path":    dir,
		"glob":    "*.go",
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "match.go")
	assert.NotContains(t, result.ForLLM, "skip.txt")
}

func TestGrepTool_WithSubdirectory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "top.txt"), []byte("findme"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), []byte("findme too"), 0o644)

	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "findme",
		"path":    dir,
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "top.txt")
	assert.Contains(t, result.ForLLM, "nested.txt")
}

func TestGrepTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("nothing here"), 0o644)

	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "xyz123",
		"path":    dir,
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "No matches found")
}

func TestGrepTool_MissingPattern(t *testing.T) {
	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "pattern is required")
}

func TestGrepTool_InvalidRegex(t *testing.T) {
	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "[invalid",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "invalid regex")
}

func TestGrepTool_SkipBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	// Binary file with NUL bytes
	os.WriteFile(filepath.Join(dir, "binary.dat"), []byte("match\x00binary"), 0o644)
	os.WriteFile(filepath.Join(dir, "text.txt"), []byte("match text"), 0o644)

	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "match",
		"path":    dir,
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "text.txt")
	assert.NotContains(t, result.ForLLM, "binary.dat")
}

func TestGrepTool_MaxResults(t *testing.T) {
	dir := t.TempDir()
	// Create file with many matching lines
	var content strings.Builder
	for i := 0; i < 200; i++ {
		content.WriteString("match line\n")
	}
	os.WriteFile(filepath.Join(dir, "big.txt"), []byte(content.String()), 0o644)

	tool := NewGrepTool("", false)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern":     "match",
		"path":        dir,
		"max_results": float64(10), // JSON numbers are float64
	})

	assert.False(t, result.IsError)
	lines := strings.Split(strings.TrimSpace(result.ForLLM), "\n")
	assert.LessOrEqual(t, len(lines), 12) // 10 results + possible truncation message
	assert.Contains(t, result.ForLLM, "truncated")
}

func TestGrepTool_Restricted_OutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	other := t.TempDir()

	tool := NewGrepTool(workspace, true)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "test",
		"path":    other,
	})

	assert.True(t, result.IsError)
}

func TestGrepTool_Restricted_Success(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "data.txt"), []byte("searchable content"), 0o644)

	tool := NewGrepTool(workspace, true)
	result := tool.Execute(context.Background(), map[string]any{
		"pattern": "searchable",
		"path":    ".",
	})

	assert.False(t, result.IsError, "got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "searchable content")
}
