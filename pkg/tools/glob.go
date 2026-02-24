package tools

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct {
	fs fileSystem
}

// NewGlobTool creates a new GlobTool with optional workspace restriction.
func NewGlobTool(workspace string, restrict bool) *GlobTool {
	var sysFs fileSystem
	if restrict {
		sysFs = &sandboxFs{workspace: workspace}
	} else {
		sysFs = &hostFs{}
	}
	return &GlobTool{fs: sysFs}
}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern (e.g. **/*.go, src/**/*.ts)"
}

func (t *GlobTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern to match files (supports ** for recursive matching)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Root directory to search in (default: workspace root)",
			},
		},
		"required": []string{"pattern"},
	}
}

const maxGlobResults = 1000

func (t *GlobTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return ErrorResult("pattern is required")
	}

	if !doublestar.ValidatePattern(pattern) {
		return ErrorResult(fmt.Sprintf("invalid glob pattern: %s", pattern))
	}

	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	var matches []string
	err := t.fs.Walk(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			return nil
		}

		// Compute relative path from the walk root for pattern matching.
		// hostFs.Walk returns absolute paths; sandboxFs.Walk returns relative paths.
		rel, relErr := filepath.Rel(path, p)
		if relErr != nil {
			rel = p
		}
		// Normalize to forward slashes for consistent matching.
		rel = filepath.ToSlash(rel)

		matched, matchErr := doublestar.Match(pattern, rel)
		if matchErr != nil {
			return nil
		}
		if matched {
			matches = append(matches, rel)
			if len(matches) >= maxGlobResults {
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to walk directory: %v", err))
	}

	if len(matches) == 0 {
		return NewToolResult(fmt.Sprintf("No files matched pattern: %s", pattern))
	}

	var result strings.Builder
	for _, m := range matches {
		result.WriteString(m + "\n")
	}
	if len(matches) >= maxGlobResults {
		result.WriteString(fmt.Sprintf("\n(results truncated at %d files)", maxGlobResults))
	}

	return NewToolResult(result.String())
}
