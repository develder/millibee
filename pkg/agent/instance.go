package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/develder/millibee/pkg/agent/optimizer"
	"github.com/develder/millibee/pkg/config"
	"github.com/develder/millibee/pkg/memory"
	"github.com/develder/millibee/pkg/providers"
	"github.com/develder/millibee/pkg/routing"
	"github.com/develder/millibee/pkg/session"
	"github.com/develder/millibee/pkg/tools"
)

// AgentInstance represents a fully configured agent with its own workspace,
// session manager, context builder, and tool registry.
type AgentInstance struct {
	ID             string
	Name           string
	Model          string
	Fallbacks      []string
	Workspace      string
	MaxIterations  int
	MaxTokens      int
	Temperature    float64
	ContextWindow  int
	Provider       providers.LLMProvider
	Sessions       *session.SessionManager
	ContextBuilder *ContextBuilder
	Tools          *tools.ToolRegistry
	Subagents      *config.SubagentsConfig
	SkillsFilter   []string
	Candidates     []providers.FallbackCandidate
	Optimizer      *optimizer.Optimizer
}

// NewAgentInstance creates an agent instance from config.
func NewAgentInstance(
	agentCfg *config.AgentConfig,
	defaults *config.AgentDefaults,
	cfg *config.Config,
	provider providers.LLMProvider,
) *AgentInstance {
	workspace := resolveAgentWorkspace(agentCfg, defaults)
	os.MkdirAll(workspace, 0o755)

	model := resolveAgentModel(agentCfg, defaults)
	fallbacks := resolveAgentFallbacks(agentCfg, defaults)

	restrict := defaults.RestrictToWorkspace
	toolsRegistry := tools.NewToolRegistry()
	toolsRegistry.Register(tools.NewReadFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewWriteFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewListDirTool(workspace, restrict))
	toolsRegistry.Register(tools.NewExecToolWithConfig(workspace, restrict, cfg))
	toolsRegistry.Register(tools.NewEditFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewAppendFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewGlobTool(workspace, restrict))
	toolsRegistry.Register(tools.NewGrepTool(workspace, restrict))

	// Git tools
	if cfg == nil || cfg.Tools.Git.Enabled {
		allowPush := cfg != nil && cfg.Tools.Git.AllowPush
		toolsRegistry.Register(tools.NewGitStatusTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitDiffTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitLogTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitShowTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitBranchTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitCommitTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitAddTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitResetTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitCheckoutTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitPullTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitMergeTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitStashTool(workspace, restrict))
		toolsRegistry.Register(tools.NewGitPushTool(workspace, allowPush, restrict))
	}

	// Memory vault tools
	memVault := memory.NewVault(filepath.Join(workspace, "memory"))
	toolsRegistry.Register(tools.NewMemorySaveTool(memVault))
	toolsRegistry.Register(tools.NewMemorySearchTool(memVault))
	toolsRegistry.Register(tools.NewMemoryRecallTool(memVault))

	// Configure security middleware from config
	if cfg != nil {
		sec := cfg.Tools.Security
		mw := tools.NewToolMiddleware()
		for toolName, pc := range sec.ToolPolicies {
			enabled := true
			if pc.Enabled != nil {
				enabled = *pc.Enabled
			}
			maxArg := pc.MaxArgSize
			if maxArg == 0 {
				maxArg = sec.DefaultMaxArgSize
			}
			maxCalls := pc.MaxCallsPerMin
			if maxCalls == 0 {
				maxCalls = sec.DefaultMaxCallsPerMin
			}
			mw.SetPolicy(toolName, tools.ToolPolicy{
				Enabled:        enabled,
				MaxArgSize:     maxArg,
				MaxCallsPerMin: maxCalls,
			})
		}
		toolsRegistry.Middleware = mw
	}

	sessionsDir := filepath.Join(workspace, "sessions")
	sessionsManager := session.NewSessionManager(sessionsDir)

	contextBuilder := NewContextBuilder(workspace)

	agentID := routing.DefaultAgentID
	agentName := ""
	var subagents *config.SubagentsConfig
	var skillsFilter []string

	if agentCfg != nil {
		agentID = routing.NormalizeAgentID(agentCfg.ID)
		agentName = agentCfg.Name
		subagents = agentCfg.Subagents
		skillsFilter = agentCfg.Skills
	}

	maxIter := defaults.MaxToolIterations
	if maxIter == 0 {
		maxIter = 20
	}

	maxTokens := defaults.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	contextWindow := defaults.ContextWindow
	if contextWindow == 0 {
		contextWindow = 128000
	}

	temperature := 0.7
	if defaults.Temperature != nil {
		temperature = *defaults.Temperature
	}

	// Resolve fallback candidates
	modelCfg := providers.ModelConfig{
		Primary:   model,
		Fallbacks: fallbacks,
	}
	candidates := providers.ResolveCandidates(modelCfg, defaults.Provider)

	return &AgentInstance{
		ID:             agentID,
		Name:           agentName,
		Model:          model,
		Fallbacks:      fallbacks,
		Workspace:      workspace,
		MaxIterations:  maxIter,
		MaxTokens:      maxTokens,
		Temperature:    temperature,
		ContextWindow:  contextWindow,
		Provider:       provider,
		Sessions:       sessionsManager,
		ContextBuilder: contextBuilder,
		Tools:          toolsRegistry,
		Subagents:      subagents,
		SkillsFilter:   skillsFilter,
		Candidates:     candidates,
		Optimizer:      optimizer.New(),
	}
}

// resolveAgentWorkspace determines the workspace directory for an agent.
func resolveAgentWorkspace(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) string {
	if agentCfg != nil && strings.TrimSpace(agentCfg.Workspace) != "" {
		return expandHome(strings.TrimSpace(agentCfg.Workspace))
	}
	if agentCfg == nil || agentCfg.Default || agentCfg.ID == "" || routing.NormalizeAgentID(agentCfg.ID) == "main" {
		return expandHome(defaults.Workspace)
	}
	home, _ := os.UserHomeDir()
	id := routing.NormalizeAgentID(agentCfg.ID)
	return filepath.Join(home, ".millibee", "workspace-"+id)
}

// resolveAgentModel resolves the primary model for an agent.
func resolveAgentModel(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) string {
	if agentCfg != nil && agentCfg.Model != nil && strings.TrimSpace(agentCfg.Model.Primary) != "" {
		return strings.TrimSpace(agentCfg.Model.Primary)
	}
	return defaults.Model
}

// resolveAgentFallbacks resolves the fallback models for an agent.
func resolveAgentFallbacks(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) []string {
	if agentCfg != nil && agentCfg.Model != nil && agentCfg.Model.Fallbacks != nil {
		return agentCfg.Model.Fallbacks
	}
	return defaults.ModelFallbacks
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}
