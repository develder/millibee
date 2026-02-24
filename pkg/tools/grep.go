package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GrepTool searches file contents using regular expressions.
type GrepTool struct {
	fs fileSystem
}

// NewGrepTool creates a new GrepTool with optional workspace restriction.
func NewGrepTool(workspace string, restrict bool) *GrepTool {
	var sysFs fileSystem
	if restrict {
		sysFs = &sandboxFs{workspace: workspace}
	} else {
		sysFs = &hostFs{}
	}
	return &GrepTool{fs: sysFs}
}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search file contents using a regular expression"
}

func (t *GrepTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Root directory to search in (default: workspace root)",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "File glob filter (e.g. *.go, *.txt)",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of matching lines to return (default: 100)",
			},
		},
		"required": []string{"pattern"},
	}
}

const defaultMaxGrepResults = 100

func (t *GrepTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return ErrorResult("pattern is required")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return ErrorResult(fmt.Sprintf("invalid regex pattern: %v", err))
	}

	searchPath, _ := args["path"].(string)
	if searchPath == "" {
		searchPath = "."
	}

	globFilter, _ := args["glob"].(string)

	maxResults := defaultMaxGrepResults
	if mr, ok := args["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	var matches []string
	truncated := false

	walkErr := t.fs.Walk(searchPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		// Apply glob filter on the file name
		if globFilter != "" {
			matched, _ := doublestar.Match(globFilter, d.Name())
			if !matched {
				return nil
			}
		}

		// Open and scan
		file, err := t.fs.Open(p)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()

			// Binary detection: skip file if any line contains NUL byte
			if bytes.ContainsRune(line, 0) {
				return nil
			}

			if re.Match(line) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", p, lineNum, string(line)))
				if len(matches) >= maxResults {
					truncated = true
					return fs.SkipAll
				}
			}
		}

		return nil
	})
	if walkErr != nil {
		return ErrorResult(fmt.Sprintf("failed to search: %v", walkErr))
	}

	if len(matches) == 0 {
		return NewToolResult(fmt.Sprintf("No matches found for pattern: %s", pattern))
	}

	var result strings.Builder
	for _, m := range matches {
		result.WriteString(m + "\n")
	}
	if truncated {
		result.WriteString(fmt.Sprintf("\n(results truncated at %d matches)", maxResults))
	}

	return NewToolResult(result.String())
}
