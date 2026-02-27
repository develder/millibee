package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/develder/millibee/pkg/config"
)

type ExecTool struct {
	workingDir          string
	timeout             time.Duration
	denyPatterns        []*regexp.Regexp
	allowPatterns       []*regexp.Regexp
	restrictToWorkspace bool
	maxCommandLength    int
}

var defaultDenyPatterns = []*regexp.Regexp{
	// Destructive file/disk operations
	regexp.MustCompile(`\brm\s+-[rf]{1,2}\s+/`),        // rm -rf / (root wipe)
	regexp.MustCompile(`\brm\s+-[rf]{1,2}\s+\.\s*$`),   // rm -rf .  (workspace wipe)
	regexp.MustCompile(`\brm\s+-[rf]{1,2}\s+\*`),       // rm -rf *  (workspace wipe)
	regexp.MustCompile(`\bdel\s+/[fq]\b`),              // Windows del /f
	regexp.MustCompile(`\brmdir\s+/s\b`),               // Windows rmdir /s
	regexp.MustCompile(`\b(format|mkfs|diskpart)\b\s`), // Disk wiping
	regexp.MustCompile(`\bdd\s+if=`),                   // Raw disk writes
	regexp.MustCompile(`>\s*/dev/sd[a-z]\b`),           // Writes to disk devices

	// System control
	regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`),

	// Fork bomb
	regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),

	// HTTP tools — use web_fetch tool instead
	regexp.MustCompile(`\bcurl\b`),
	regexp.MustCompile(`\bwget\b`),

	// Git force push (data loss)
	regexp.MustCompile(`\bgit\s+push\s+.*--force\b`),
	regexp.MustCompile(`\bgit\s+push\s+-f\b`),
}

func NewExecTool(workingDir string, restrict bool) *ExecTool {
	return NewExecToolWithConfig(workingDir, restrict, nil)
}

func NewExecToolWithConfig(workingDir string, restrict bool, config *config.Config) *ExecTool {
	denyPatterns := make([]*regexp.Regexp, 0)

	if config != nil && len(config.Tools.Exec.CustomDenyPatterns) > 0 {
		fmt.Printf("Using custom deny patterns: %v\n", config.Tools.Exec.CustomDenyPatterns)
		for _, pattern := range config.Tools.Exec.CustomDenyPatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				fmt.Printf("Invalid custom deny pattern %q: %v\n", pattern, err)
				continue
			}
			denyPatterns = append(denyPatterns, re)
		}
	} else {
		denyPatterns = append(denyPatterns, defaultDenyPatterns...)
	}

	var allowPatterns []*regexp.Regexp
	maxCmdLen := 10000
	if config != nil {
		execConfig := config.Tools.Exec
		if execConfig.EnableAllowlist && len(execConfig.AllowPatterns) > 0 {
			for _, pattern := range execConfig.AllowPatterns {
				re, err := regexp.Compile(pattern)
				if err != nil {
					fmt.Printf("Invalid allow pattern %q: %v\n", pattern, err)
					continue
				}
				allowPatterns = append(allowPatterns, re)
			}
		}
		if execConfig.MaxCommandLength > 0 {
			maxCmdLen = execConfig.MaxCommandLength
		}
	}

	return &ExecTool{
		workingDir:          workingDir,
		timeout:             60 * time.Second,
		denyPatterns:        denyPatterns,
		allowPatterns:       allowPatterns,
		restrictToWorkspace: restrict,
		maxCommandLength:    maxCmdLen,
	}
}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Execute a shell command and return its output. For git operations, use the specialized git_* tools (git_status, git_log, git_pull, etc.) instead of exec."
}

func (t *ExecTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"working_dir": map[string]any{
				"type":        "string",
				"description": "Optional working directory for the command",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ExecTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	command, ok := args["command"].(string)
	if !ok {
		return ErrorResult("command is required")
	}

	cwd := t.workingDir
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		if t.restrictToWorkspace && t.workingDir != "" {
			resolvedWD, err := validatePath(wd, t.workingDir, true)
			if err != nil {
				return ErrorResult("Command blocked by safety guard (" + err.Error() + ")")
			}
			cwd = resolvedWD
		} else {
			cwd = wd
		}
	}

	if cwd == "" {
		wd, err := os.Getwd()
		if err == nil {
			cwd = wd
		}
	}

	if guardError := t.guardCommand(command, cwd); guardError != "" {
		return ErrorResult(guardError)
	}

	// timeout == 0 means no timeout
	var cmdCtx context.Context
	var cancel context.CancelFunc
	if t.timeout > 0 {
		cmdCtx, cancel = context.WithTimeout(ctx, t.timeout)
	} else {
		cmdCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", command)
	}
	if cwd != "" {
		cmd.Dir = cwd
	}

	prepareCommandForTermination(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return ErrorResult(fmt.Sprintf("failed to start command: %v", err))
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var err error
	select {
	case err = <-done:
	case <-cmdCtx.Done():
		_ = terminateProcessTree(cmd)
		select {
		case err = <-done:
		case <-time.After(2 * time.Second):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			err = <-done
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			msg := fmt.Sprintf("Command timed out after %v", t.timeout)
			return &ToolResult{
				ForLLM:  msg,
				ForUser: msg,
				IsError: true,
			}
		}
		output += fmt.Sprintf("\nExit code: %v", err)
	}

	if output == "" {
		output = "(no output)"
	}

	maxLen := 10000
	if len(output) > maxLen {
		output = output[:maxLen] + fmt.Sprintf("\n... (truncated, %d more chars)", len(output)-maxLen)
	}

	if err != nil {
		return &ToolResult{
			ForLLM:  output,
			ForUser: output,
			IsError: true,
		}
	}

	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
		IsError: false,
	}
}

// isReadOnlySafePath returns true for well-known system paths that are
// safe to read and should not be blocked by the workspace restriction.
func isReadOnlySafePath(path string) bool {
	safeDirs := []string{
		"/usr", "/bin", "/sbin", "/lib", "/etc",
		"/proc", "/sys", "/dev", "/tmp", "/opt",
	}
	lower := strings.ToLower(path)
	for _, dir := range safeDirs {
		if lower == dir || strings.HasPrefix(lower, dir+"/") {
			return true
		}
	}
	return false
}

func (t *ExecTool) guardCommand(command, cwd string) string {
	cmd := strings.TrimSpace(command)

	if t.maxCommandLength > 0 && len(cmd) > t.maxCommandLength {
		return "Command blocked by safety guard (exceeds max command length)"
	}

	lower := strings.ToLower(cmd)

	// Check HTTP tools first to give a helpful hint
	httpToolPattern := regexp.MustCompile(`\b(curl|wget)\b`)
	if httpToolPattern.MatchString(lower) {
		return "Command blocked: curl/wget are not available. Use the web_fetch tool for HTTP requests instead."
	}

	for _, pattern := range t.denyPatterns {
		if pattern.MatchString(lower) {
			return "Command blocked by safety guard (dangerous pattern detected)"
		}
	}

	if len(t.allowPatterns) > 0 {
		allowed := false
		for _, pattern := range t.allowPatterns {
			if pattern.MatchString(lower) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "Command blocked by safety guard (not in allowlist)"
		}
	}

	if t.restrictToWorkspace {
		if strings.Contains(cmd, "..\\") || strings.Contains(cmd, "../") {
			return "Command blocked by safety guard (path traversal detected)"
		}

		// Use the workspace root (not cwd) as the containment boundary
		wsPath, err := filepath.Abs(t.workingDir)
		if err != nil {
			return ""
		}

		// Strip URLs before path checking to avoid matching URL path components as filesystem paths
		urlPattern := regexp.MustCompile(`https?://\S+|ftp://\S+`)
		cmdForPathCheck := urlPattern.ReplaceAllString(cmd, "")

		pathPattern := regexp.MustCompile(`[A-Za-z]:[\\\/][^\s\"']+|/[^\s\"']+`)
		matches := pathPattern.FindAllString(cmdForPathCheck, -1)

		for _, raw := range matches {
			// Skip safe well-known paths that are read-only system locations
			if isReadOnlySafePath(raw) {
				continue
			}

			p, err := filepath.Abs(raw)
			if err != nil {
				continue
			}

			rel, err := filepath.Rel(wsPath, p)
			if err != nil {
				// On Windows, cross-drive paths can't be made relative — treat as outside
				return "Command blocked by safety guard (path outside working dir)"
			}

			if strings.HasPrefix(rel, "..") {
				return "Command blocked by safety guard (path outside working dir)"
			}
		}
	}

	return ""
}

func (t *ExecTool) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

func (t *ExecTool) SetRestrictToWorkspace(restrict bool) {
	t.restrictToWorkspace = restrict
}

func (t *ExecTool) SetAllowPatterns(patterns []string) error {
	t.allowPatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("invalid allow pattern %q: %w", p, err)
		}
		t.allowPatterns = append(t.allowPatterns, re)
	}
	return nil
}
