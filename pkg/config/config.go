package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/caarlos0/env/v11"
)

// rrCounter is a global counter for round-robin load balancing across models.
var rrCounter atomic.Uint64

// FlexibleStringSlice is a []string that also accepts JSON numbers,
// so allow_from can contain both "123" and 123.
type FlexibleStringSlice []string

func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	// Try []string first
	var ss []string
	if err := json.Unmarshal(data, &ss); err == nil {
		*f = ss
		return nil
	}

	// Try []interface{} to handle mixed types
	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	result := make([]string, 0, len(raw))
	for _, v := range raw {
		switch val := v.(type) {
		case string:
			result = append(result, val)
		case float64:
			result = append(result, fmt.Sprintf("%.0f", val))
		default:
			result = append(result, fmt.Sprintf("%v", val))
		}
	}
	*f = result
	return nil
}

type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Bindings  []AgentBinding  `json:"bindings,omitempty"`
	Session   SessionConfig   `json:"session,omitempty"`
	Channels  ChannelsConfig  `json:"channels"`
	Providers ProvidersConfig `json:"providers,omitempty"`
	ModelList []ModelConfig   `json:"model_list"` // New model-centric provider configuration
	Gateway   GatewayConfig   `json:"gateway"`
	Tools     ToolsConfig     `json:"tools"`
	Heartbeat HeartbeatConfig `json:"heartbeat"`
	Devices   DevicesConfig   `json:"devices"`
}

// MarshalJSON implements custom JSON marshaling for Config
// to omit providers section when empty and session when empty
func (c Config) MarshalJSON() ([]byte, error) {
	type Alias Config
	aux := &struct {
		Providers *ProvidersConfig `json:"providers,omitempty"`
		Session   *SessionConfig   `json:"session,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(&c),
	}

	// Only include providers if not empty
	if !c.Providers.IsEmpty() {
		aux.Providers = &c.Providers
	}

	// Only include session if not empty
	if c.Session.DMScope != "" || len(c.Session.IdentityLinks) > 0 {
		aux.Session = &c.Session
	}

	return json.Marshal(aux)
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
	List     []AgentConfig `json:"list,omitempty"`
}

// AgentModelConfig supports both string and structured model config.
// String format: "gpt-4" (just primary, no fallbacks)
// Object format: {"primary": "gpt-4", "fallbacks": ["claude-haiku"]}
type AgentModelConfig struct {
	Primary   string   `json:"primary,omitempty"`
	Fallbacks []string `json:"fallbacks,omitempty"`
}

func (m *AgentModelConfig) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.Primary = s
		m.Fallbacks = nil
		return nil
	}
	type raw struct {
		Primary   string   `json:"primary"`
		Fallbacks []string `json:"fallbacks"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	m.Primary = r.Primary
	m.Fallbacks = r.Fallbacks
	return nil
}

func (m AgentModelConfig) MarshalJSON() ([]byte, error) {
	if len(m.Fallbacks) == 0 && m.Primary != "" {
		return json.Marshal(m.Primary)
	}
	type raw struct {
		Primary   string   `json:"primary,omitempty"`
		Fallbacks []string `json:"fallbacks,omitempty"`
	}
	return json.Marshal(raw{Primary: m.Primary, Fallbacks: m.Fallbacks})
}

type AgentConfig struct {
	ID        string            `json:"id"`
	Default   bool              `json:"default,omitempty"`
	Name      string            `json:"name,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
	Model     *AgentModelConfig `json:"model,omitempty"`
	Skills    []string          `json:"skills,omitempty"`
	Subagents *SubagentsConfig  `json:"subagents,omitempty"`
}

type SubagentsConfig struct {
	AllowAgents []string          `json:"allow_agents,omitempty"`
	Model       *AgentModelConfig `json:"model,omitempty"`
}

type PeerMatch struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type BindingMatch struct {
	Channel   string     `json:"channel"`
	AccountID string     `json:"account_id,omitempty"`
	Peer      *PeerMatch `json:"peer,omitempty"`
	GuildID   string     `json:"guild_id,omitempty"`
	TeamID    string     `json:"team_id,omitempty"`
}

type AgentBinding struct {
	AgentID string       `json:"agent_id"`
	Match   BindingMatch `json:"match"`
}

type SessionConfig struct {
	DMScope       string              `json:"dm_scope,omitempty"`
	IdentityLinks map[string][]string `json:"identity_links,omitempty"`
}

type AgentDefaults struct {
	Workspace           string   `json:"workspace"                       env:"MILLIBEE_AGENTS_DEFAULTS_WORKSPACE"`
	RestrictToWorkspace bool     `json:"restrict_to_workspace"           env:"MILLIBEE_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE"`
	Provider            string   `json:"provider"                        env:"MILLIBEE_AGENTS_DEFAULTS_PROVIDER"`
	Model               string   `json:"model"                           env:"MILLIBEE_AGENTS_DEFAULTS_MODEL"`
	ModelFallbacks      []string `json:"model_fallbacks,omitempty"`
	ImageModel          string   `json:"image_model,omitempty"           env:"MILLIBEE_AGENTS_DEFAULTS_IMAGE_MODEL"`
	ImageModelFallbacks []string `json:"image_model_fallbacks,omitempty"`
	MaxTokens           int      `json:"max_tokens"                      env:"MILLIBEE_AGENTS_DEFAULTS_MAX_TOKENS"`
	ContextWindow       int      `json:"context_window"                  env:"MILLIBEE_AGENTS_DEFAULTS_CONTEXT_WINDOW"`
	Temperature         *float64 `json:"temperature,omitempty"           env:"MILLIBEE_AGENTS_DEFAULTS_TEMPERATURE"`
	MaxToolIterations   int      `json:"max_tool_iterations"             env:"MILLIBEE_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS"`
}

type ChannelsConfig struct {
	Console  ConsoleConfig  `json:"console"`
	Telegram TelegramConfig `json:"telegram"`
	SSH      SSHConfig      `json:"ssh"`
}

// SSHConfig holds configuration for the SSH terminal channel.
// When enabled, the gateway starts a Wish SSH server that serves
// a Bubble Tea TUI to connecting clients.
type SSHConfig struct {
	Enabled     bool                `json:"enabled"       env:"MILLIBEE_CHANNELS_SSH_ENABLED"`
	Address     string              `json:"address"       env:"MILLIBEE_CHANNELS_SSH_ADDRESS"`
	Password    string              `json:"password"      env:"MILLIBEE_CHANNELS_SSH_PASSWORD"`
	HostKeyPath string              `json:"host_key_path" env:"MILLIBEE_CHANNELS_SSH_HOST_KEY_PATH"`
	AllowFrom   FlexibleStringSlice `json:"allow_from"    env:"MILLIBEE_CHANNELS_SSH_ALLOW_FROM"`
}

// ConsoleConfig holds configuration for the interactive console channel.
// When enabled in gateway mode, stdin is read for user input and responses
// are printed to stdout.
type ConsoleConfig struct {
	Enabled bool `json:"enabled" env:"MILLIBEE_CHANNELS_CONSOLE_ENABLED"`
}

type TelegramConfig struct {
	Enabled   bool                `json:"enabled"    env:"MILLIBEE_CHANNELS_TELEGRAM_ENABLED"`
	Token     string              `json:"token"      env:"MILLIBEE_CHANNELS_TELEGRAM_TOKEN"`
	Proxy     string              `json:"proxy"      env:"MILLIBEE_CHANNELS_TELEGRAM_PROXY"`
	AllowFrom FlexibleStringSlice `json:"allow_from" env:"MILLIBEE_CHANNELS_TELEGRAM_ALLOW_FROM"`
}

type HeartbeatConfig struct {
	Enabled  bool `json:"enabled"  env:"MILLIBEE_HEARTBEAT_ENABLED"`
	Interval int  `json:"interval" env:"MILLIBEE_HEARTBEAT_INTERVAL"` // minutes, min 5
}

type DevicesConfig struct {
	Enabled    bool `json:"enabled"     env:"MILLIBEE_DEVICES_ENABLED"`
	MonitorUSB bool `json:"monitor_usb" env:"MILLIBEE_DEVICES_MONITOR_USB"`
}

type ProvidersConfig struct {
	Anthropic     ProviderConfig       `json:"anthropic"`
	OpenAI        OpenAIProviderConfig `json:"openai"`
	OpenRouter    ProviderConfig       `json:"openrouter"`
	Groq          ProviderConfig       `json:"groq"`
	Zhipu         ProviderConfig       `json:"zhipu"`
	VLLM          ProviderConfig       `json:"vllm"`
	Gemini        ProviderConfig       `json:"gemini"`
	Nvidia        ProviderConfig       `json:"nvidia"`
	Ollama        ProviderConfig       `json:"ollama"`
	Moonshot      ProviderConfig       `json:"moonshot"`
	ShengSuanYun  ProviderConfig       `json:"shengsuanyun"`
	DeepSeek      ProviderConfig       `json:"deepseek"`
	Cerebras      ProviderConfig       `json:"cerebras"`
	VolcEngine    ProviderConfig       `json:"volcengine"`
	GitHubCopilot ProviderConfig       `json:"github_copilot"`
	Antigravity   ProviderConfig       `json:"antigravity"`
	Qwen          ProviderConfig       `json:"qwen"`
	Mistral       ProviderConfig       `json:"mistral"`
}

// IsEmpty checks if all provider configs are empty (no API keys or API bases set)
// Note: WebSearch is an optimization option and doesn't count as "non-empty"
func (p ProvidersConfig) IsEmpty() bool {
	return p.Anthropic.APIKey == "" && p.Anthropic.APIBase == "" &&
		p.OpenAI.APIKey == "" && p.OpenAI.APIBase == "" &&
		p.OpenRouter.APIKey == "" && p.OpenRouter.APIBase == "" &&
		p.Groq.APIKey == "" && p.Groq.APIBase == "" &&
		p.Zhipu.APIKey == "" && p.Zhipu.APIBase == "" &&
		p.VLLM.APIKey == "" && p.VLLM.APIBase == "" &&
		p.Gemini.APIKey == "" && p.Gemini.APIBase == "" &&
		p.Nvidia.APIKey == "" && p.Nvidia.APIBase == "" &&
		p.Ollama.APIKey == "" && p.Ollama.APIBase == "" &&
		p.Moonshot.APIKey == "" && p.Moonshot.APIBase == "" &&
		p.ShengSuanYun.APIKey == "" && p.ShengSuanYun.APIBase == "" &&
		p.DeepSeek.APIKey == "" && p.DeepSeek.APIBase == "" &&
		p.Cerebras.APIKey == "" && p.Cerebras.APIBase == "" &&
		p.VolcEngine.APIKey == "" && p.VolcEngine.APIBase == "" &&
		p.GitHubCopilot.APIKey == "" && p.GitHubCopilot.APIBase == "" &&
		p.Antigravity.APIKey == "" && p.Antigravity.APIBase == "" &&
		p.Qwen.APIKey == "" && p.Qwen.APIBase == "" &&
		p.Mistral.APIKey == "" && p.Mistral.APIBase == ""
}

// MarshalJSON implements custom JSON marshaling for ProvidersConfig
// to omit the entire section when empty
func (p ProvidersConfig) MarshalJSON() ([]byte, error) {
	if p.IsEmpty() {
		return []byte("null"), nil
	}
	type Alias ProvidersConfig
	return json.Marshal((*Alias)(&p))
}

type ProviderConfig struct {
	APIKey      string `json:"api_key"                env:"MILLIBEE_PROVIDERS_{{.Name}}_API_KEY"`
	APIBase     string `json:"api_base"               env:"MILLIBEE_PROVIDERS_{{.Name}}_API_BASE"`
	Proxy       string `json:"proxy,omitempty"        env:"MILLIBEE_PROVIDERS_{{.Name}}_PROXY"`
	AuthMethod  string `json:"auth_method,omitempty"  env:"MILLIBEE_PROVIDERS_{{.Name}}_AUTH_METHOD"`
	ConnectMode string `json:"connect_mode,omitempty" env:"MILLIBEE_PROVIDERS_{{.Name}}_CONNECT_MODE"` // only for Github Copilot, `stdio` or `grpc`
}

type OpenAIProviderConfig struct {
	ProviderConfig
	WebSearch bool `json:"web_search" env:"MILLIBEE_PROVIDERS_OPENAI_WEB_SEARCH"`
}

// ModelConfig represents a model-centric provider configuration.
// It allows adding new providers (especially OpenAI-compatible ones) via configuration only.
// The model field uses protocol prefix format: [protocol/]model-identifier
// Supported protocols: openai, anthropic, antigravity, claude-cli, codex-cli, github-copilot
// Default protocol is "openai" if no prefix is specified.
type ModelConfig struct {
	// Required fields
	ModelName string `json:"model_name"` // User-facing alias for the model
	Model     string `json:"model"`      // Protocol/model-identifier (e.g., "openai/gpt-4o", "anthropic/claude-sonnet-4.6")

	// HTTP-based providers
	APIBase string `json:"api_base,omitempty"` // API endpoint URL
	APIKey  string `json:"api_key"`            // API authentication key
	Proxy   string `json:"proxy,omitempty"`    // HTTP proxy URL

	// Special providers (CLI-based, OAuth, etc.)
	AuthMethod  string `json:"auth_method,omitempty"`  // Authentication method: oauth, token
	ConnectMode string `json:"connect_mode,omitempty"` // Connection mode: stdio, grpc
	Workspace   string `json:"workspace,omitempty"`    // Workspace path for CLI-based providers

	// Optional optimizations
	RPM            int    `json:"rpm,omitempty"`              // Requests per minute limit
	MaxTokensField string `json:"max_tokens_field,omitempty"` // Field name for max tokens (e.g., "max_completion_tokens")
}

// Validate checks if the ModelConfig has all required fields.
func (c *ModelConfig) Validate() error {
	if c.ModelName == "" {
		return fmt.Errorf("model_name is required")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

type GatewayConfig struct {
	Host string `json:"host" env:"MILLIBEE_GATEWAY_HOST"`
	Port int    `json:"port" env:"MILLIBEE_GATEWAY_PORT"`
}

type BraveConfig struct {
	Enabled    bool   `json:"enabled"     env:"MILLIBEE_TOOLS_WEB_BRAVE_ENABLED"`
	APIKey     string `json:"api_key"     env:"MILLIBEE_TOOLS_WEB_BRAVE_API_KEY"`
	MaxResults int    `json:"max_results" env:"MILLIBEE_TOOLS_WEB_BRAVE_MAX_RESULTS"`
}

type TavilyConfig struct {
	Enabled    bool   `json:"enabled"     env:"MILLIBEE_TOOLS_WEB_TAVILY_ENABLED"`
	APIKey     string `json:"api_key"     env:"MILLIBEE_TOOLS_WEB_TAVILY_API_KEY"`
	BaseURL    string `json:"base_url"    env:"MILLIBEE_TOOLS_WEB_TAVILY_BASE_URL"`
	MaxResults int    `json:"max_results" env:"MILLIBEE_TOOLS_WEB_TAVILY_MAX_RESULTS"`
}

type DuckDuckGoConfig struct {
	Enabled    bool `json:"enabled"     env:"MILLIBEE_TOOLS_WEB_DUCKDUCKGO_ENABLED"`
	MaxResults int  `json:"max_results" env:"MILLIBEE_TOOLS_WEB_DUCKDUCKGO_MAX_RESULTS"`
}

type PerplexityConfig struct {
	Enabled    bool   `json:"enabled"     env:"MILLIBEE_TOOLS_WEB_PERPLEXITY_ENABLED"`
	APIKey     string `json:"api_key"     env:"MILLIBEE_TOOLS_WEB_PERPLEXITY_API_KEY"`
	MaxResults int    `json:"max_results" env:"MILLIBEE_TOOLS_WEB_PERPLEXITY_MAX_RESULTS"`
}

type WebToolsConfig struct {
	Brave        BraveConfig      `json:"brave"`
	Tavily       TavilyConfig     `json:"tavily"`
	DuckDuckGo   DuckDuckGoConfig `json:"duckduckgo"`
	Perplexity   PerplexityConfig `json:"perplexity"`
	AllowedHosts []string         `json:"allowed_hosts" env:"MILLIBEE_TOOLS_WEB_ALLOWED_HOSTS"` // Hosts that bypass SSRF private IP checks (e.g. Docker service names)
}

type CronToolsConfig struct {
	ExecTimeoutMinutes int `json:"exec_timeout_minutes" env:"MILLIBEE_TOOLS_CRON_EXEC_TIMEOUT_MINUTES"` // 0 means no timeout
}

type ExecConfig struct {
	CustomDenyPatterns []string `json:"custom_deny_patterns" env:"MILLIBEE_TOOLS_EXEC_CUSTOM_DENY_PATTERNS"`
	EnableAllowlist    bool     `json:"enable_allowlist"      env:"MILLIBEE_TOOLS_EXEC_ENABLE_ALLOWLIST"`
	AllowPatterns      []string `json:"allow_patterns"        env:"MILLIBEE_TOOLS_EXEC_ALLOW_PATTERNS"`
	MaxCommandLength   int      `json:"max_command_length"    env:"MILLIBEE_TOOLS_EXEC_MAX_COMMAND_LENGTH"`
}

type ToolPolicyConfig struct {
	Enabled        *bool `json:"enabled,omitempty"`
	MaxArgSize     int   `json:"max_arg_size,omitempty"     env:"MAX_ARG_SIZE"`
	MaxCallsPerMin int   `json:"max_calls_per_min,omitempty" env:"MAX_CALLS_PER_MIN"`
}

type SecurityConfig struct {
	DefaultMaxArgSize     int                          `json:"default_max_arg_size"      env:"MILLIBEE_TOOLS_SECURITY_DEFAULT_MAX_ARG_SIZE"`
	DefaultMaxCallsPerMin int                          `json:"default_max_calls_per_min" env:"MILLIBEE_TOOLS_SECURITY_DEFAULT_MAX_CALLS_PER_MIN"`
	ToolPolicies          map[string]ToolPolicyConfig   `json:"tool_policies,omitempty"`
}

type ToolsConfig struct {
	Web      WebToolsConfig    `json:"web"`
	Sidecars SidecarsConfig    `json:"sidecars"`
	Cron     CronToolsConfig   `json:"cron"`
	Exec     ExecConfig        `json:"exec"`
	Git      GitToolsConfig    `json:"git"`
	Skills   SkillsToolsConfig `json:"skills"`
	Security SecurityConfig    `json:"security"`
}

// GitToolsConfig holds configuration for native git tools.
type GitToolsConfig struct {
	Enabled   bool `json:"enabled" env:"MILLIBEE_TOOLS_GIT_ENABLED"`
	AllowPush bool `json:"allow_push" env:"MILLIBEE_TOOLS_GIT_ALLOW_PUSH"`
}

// SidecarsConfig holds configuration for Docker sidecar service tools.
type SidecarsConfig struct {
	Crawl4AI     Crawl4AIConfig     `json:"crawl4ai"`
	YTTranscript YTTranscriptConfig `json:"yt_transcript"`
	WhisperASR   WhisperASRConfig   `json:"whisper_asr"`
}

// Crawl4AIConfig configures the deep_scrape tool backed by Crawl4AI.
type Crawl4AIConfig struct {
	Enabled     bool   `json:"enabled"      env:"MILLIBEE_SIDECARS_CRAWL4AI_ENABLED"`
	BaseURL     string `json:"base_url"     env:"MILLIBEE_SIDECARS_CRAWL4AI_BASE_URL"`
	TimeoutSecs int    `json:"timeout_secs" env:"MILLIBEE_SIDECARS_CRAWL4AI_TIMEOUT_SECS"`
}

// YTTranscriptConfig configures the youtube_transcript tool.
type YTTranscriptConfig struct {
	Enabled     bool   `json:"enabled"      env:"MILLIBEE_SIDECARS_YT_TRANSCRIPT_ENABLED"`
	BaseURL     string `json:"base_url"     env:"MILLIBEE_SIDECARS_YT_TRANSCRIPT_BASE_URL"`
	TimeoutSecs int    `json:"timeout_secs" env:"MILLIBEE_SIDECARS_YT_TRANSCRIPT_TIMEOUT_SECS"`
}

// WhisperASRConfig configures the transcribe_audio tool backed by Whisper ASR.
type WhisperASRConfig struct {
	Enabled     bool   `json:"enabled"      env:"MILLIBEE_SIDECARS_WHISPER_ASR_ENABLED"`
	BaseURL     string `json:"base_url"     env:"MILLIBEE_SIDECARS_WHISPER_ASR_BASE_URL"`
	TimeoutSecs int    `json:"timeout_secs" env:"MILLIBEE_SIDECARS_WHISPER_ASR_TIMEOUT_SECS"`
}

type SkillsToolsConfig struct {
	Registries            SkillsRegistriesConfig `json:"registries"`
	MaxConcurrentSearches int                    `json:"max_concurrent_searches" env:"MILLIBEE_SKILLS_MAX_CONCURRENT_SEARCHES"`
	SearchCache           SearchCacheConfig      `json:"search_cache"`
}

type SearchCacheConfig struct {
	MaxSize    int `json:"max_size"    env:"MILLIBEE_SKILLS_SEARCH_CACHE_MAX_SIZE"`
	TTLSeconds int `json:"ttl_seconds" env:"MILLIBEE_SKILLS_SEARCH_CACHE_TTL_SECONDS"`
}

type SkillsRegistriesConfig struct {
	ClawHub ClawHubRegistryConfig `json:"clawhub"`
}

type ClawHubRegistryConfig struct {
	Enabled         bool   `json:"enabled"           env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_ENABLED"`
	BaseURL         string `json:"base_url"          env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_BASE_URL"`
	AuthToken       string `json:"auth_token"        env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_AUTH_TOKEN"`
	SearchPath      string `json:"search_path"       env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_SEARCH_PATH"`
	SkillsPath      string `json:"skills_path"       env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_SKILLS_PATH"`
	DownloadPath    string `json:"download_path"     env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_DOWNLOAD_PATH"`
	Timeout         int    `json:"timeout"           env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_TIMEOUT"`
	MaxZipSize      int    `json:"max_zip_size"      env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_MAX_ZIP_SIZE"`
	MaxResponseSize int    `json:"max_response_size" env:"MILLIBEE_SKILLS_REGISTRIES_CLAWHUB_MAX_RESPONSE_SIZE"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	// Clear default ModelList before unmarshaling to prevent Go's slice reuse
	// from merging default struct values with user-provided entries.
	// When json.Unmarshal appends to a slice with existing capacity, it reuses
	// the underlying memory, leaving fields not present in the JSON unchanged.
	defaultModelList := cfg.ModelList
	cfg.ModelList = nil

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// If user JSON didn't provide model_list, restore defaults
	if cfg.ModelList == nil {
		cfg.ModelList = defaultModelList
	}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Auto-migrate: if only legacy providers config exists, convert to model_list
	if len(cfg.ModelList) == 0 && cfg.HasProvidersConfig() {
		cfg.ModelList = ConvertProvidersToModelList(cfg)
	}

	// Validate model_list for uniqueness and required fields
	if err := cfg.ValidateModelList(); err != nil {
		return nil, err
	}

	// Validate configuration values
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks configuration values for correctness.
func (c *Config) Validate() error {
	// Temperature: [0.0, 2.0] if set
	if c.Agents.Defaults.Temperature != nil {
		t := *c.Agents.Defaults.Temperature
		if t < 0.0 || t > 2.0 {
			return fmt.Errorf("agents.defaults.temperature must be between 0.0 and 2.0, got %v", t)
		}
	}

	// MaxTokens > 0
	if c.Agents.Defaults.MaxTokens <= 0 {
		return fmt.Errorf("agents.defaults.max_tokens must be > 0, got %d", c.Agents.Defaults.MaxTokens)
	}

	// MaxToolIterations > 0
	if c.Agents.Defaults.MaxToolIterations <= 0 {
		return fmt.Errorf("agents.defaults.max_tool_iterations must be > 0, got %d", c.Agents.Defaults.MaxToolIterations)
	}

	// Gateway port: 1-65535
	if c.Gateway.Port < 1 || c.Gateway.Port > 65535 {
		return fmt.Errorf("gateway.port must be between 1 and 65535, got %d", c.Gateway.Port)
	}

	// Security: non-negative values
	if c.Tools.Security.DefaultMaxArgSize < 0 {
		return fmt.Errorf("tools.security.default_max_arg_size must be >= 0, got %d", c.Tools.Security.DefaultMaxArgSize)
	}
	for name, policy := range c.Tools.Security.ToolPolicies {
		if policy.MaxCallsPerMin < 0 {
			return fmt.Errorf("tools.security.tool_policies[%s].max_calls_per_min must be >= 0, got %d", name, policy.MaxCallsPerMin)
		}
		if policy.MaxArgSize < 0 {
			return fmt.Errorf("tools.security.tool_policies[%s].max_arg_size must be >= 0, got %d", name, policy.MaxArgSize)
		}
	}

	return nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func (c *Config) WorkspacePath() string {
	return expandHome(c.Agents.Defaults.Workspace)
}

func (c *Config) GetAPIKey() string {
	if c.Providers.OpenRouter.APIKey != "" {
		return c.Providers.OpenRouter.APIKey
	}
	if c.Providers.Anthropic.APIKey != "" {
		return c.Providers.Anthropic.APIKey
	}
	if c.Providers.OpenAI.APIKey != "" {
		return c.Providers.OpenAI.APIKey
	}
	if c.Providers.Gemini.APIKey != "" {
		return c.Providers.Gemini.APIKey
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIKey
	}
	if c.Providers.Groq.APIKey != "" {
		return c.Providers.Groq.APIKey
	}
	if c.Providers.VLLM.APIKey != "" {
		return c.Providers.VLLM.APIKey
	}
	if c.Providers.ShengSuanYun.APIKey != "" {
		return c.Providers.ShengSuanYun.APIKey
	}
	if c.Providers.Cerebras.APIKey != "" {
		return c.Providers.Cerebras.APIKey
	}
	return ""
}

func (c *Config) GetAPIBase() string {
	if c.Providers.OpenRouter.APIKey != "" {
		if c.Providers.OpenRouter.APIBase != "" {
			return c.Providers.OpenRouter.APIBase
		}
		return "https://openrouter.ai/api/v1"
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIBase
	}
	if c.Providers.VLLM.APIKey != "" && c.Providers.VLLM.APIBase != "" {
		return c.Providers.VLLM.APIBase
	}
	return ""
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

// GetModelConfig returns the ModelConfig for the given model name.
// If multiple configs exist with the same model_name, it uses round-robin
// selection for load balancing. Returns an error if the model is not found.
func (c *Config) GetModelConfig(modelName string) (*ModelConfig, error) {
	matches := c.findMatches(modelName)
	if len(matches) == 0 {
		return nil, fmt.Errorf("model %q not found in model_list or providers", modelName)
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}

	// Multiple configs - use round-robin for load balancing
	idx := rrCounter.Add(1) % uint64(len(matches))
	return &matches[idx], nil
}

// findMatches finds all ModelConfig entries with the given model_name.
func (c *Config) findMatches(modelName string) []ModelConfig {
	var matches []ModelConfig
	for i := range c.ModelList {
		if c.ModelList[i].ModelName == modelName {
			matches = append(matches, c.ModelList[i])
		}
	}
	return matches
}

// HasProvidersConfig checks if any provider in the old providers config has configuration.
func (c *Config) HasProvidersConfig() bool {
	v := c.Providers
	return v.Anthropic.APIKey != "" || v.Anthropic.APIBase != "" ||
		v.OpenAI.APIKey != "" || v.OpenAI.APIBase != "" ||
		v.OpenRouter.APIKey != "" || v.OpenRouter.APIBase != "" ||
		v.Groq.APIKey != "" || v.Groq.APIBase != "" ||
		v.Zhipu.APIKey != "" || v.Zhipu.APIBase != "" ||
		v.VLLM.APIKey != "" || v.VLLM.APIBase != "" ||
		v.Gemini.APIKey != "" || v.Gemini.APIBase != "" ||
		v.Nvidia.APIKey != "" || v.Nvidia.APIBase != "" ||
		v.Ollama.APIKey != "" || v.Ollama.APIBase != "" ||
		v.Moonshot.APIKey != "" || v.Moonshot.APIBase != "" ||
		v.ShengSuanYun.APIKey != "" || v.ShengSuanYun.APIBase != "" ||
		v.DeepSeek.APIKey != "" || v.DeepSeek.APIBase != "" ||
		v.Cerebras.APIKey != "" || v.Cerebras.APIBase != "" ||
		v.VolcEngine.APIKey != "" || v.VolcEngine.APIBase != "" ||
		v.GitHubCopilot.APIKey != "" || v.GitHubCopilot.APIBase != "" ||
		v.Antigravity.APIKey != "" || v.Antigravity.APIBase != "" ||
		v.Qwen.APIKey != "" || v.Qwen.APIBase != "" ||
		v.Mistral.APIKey != "" || v.Mistral.APIBase != ""
}

// ValidateModelList validates all ModelConfig entries in the model_list.
// It checks that each model config is valid.
// Note: Multiple entries with the same model_name are allowed for load balancing.
func (c *Config) ValidateModelList() error {
	for i := range c.ModelList {
		if err := c.ModelList[i].Validate(); err != nil {
			return fmt.Errorf("model_list[%d]: %w", i, err)
		}
	}
	return nil
}
