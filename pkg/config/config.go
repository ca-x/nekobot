// Package config provides configuration management for nanobot.
// It uses Viper for flexible configuration loading with support for:
// - Multiple formats (JSON, YAML, TOML)
// - Environment variables
// - Hot-reload
// - Default values
package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config represents the complete nanobot configuration.
type Config struct {
	Logger        LoggerConfig        `mapstructure:"logger" json:"logger"`
	Agents        AgentsConfig        `mapstructure:"agents" json:"agents"`
	Channels      ChannelsConfig      `mapstructure:"channels" json:"channels"`
	Providers     ProvidersConfig     `mapstructure:"providers" json:"providers"`
	Transcription TranscriptionConfig `mapstructure:"transcription" json:"transcription"`
	Gateway       GatewayConfig       `mapstructure:"gateway" json:"gateway"`
	Tools         ToolsConfig         `mapstructure:"tools" json:"tools"`
	Heartbeat     HeartbeatConfig     `mapstructure:"heartbeat" json:"heartbeat"`
	Redis         RedisConfig         `mapstructure:"redis" json:"redis"`
	State         StateConfig         `mapstructure:"state" json:"state"`
	Bus           BusConfig           `mapstructure:"bus" json:"bus"`
	Memory        MemoryConfig        `mapstructure:"memory" json:"memory"`
	Approval      ApprovalConfig      `mapstructure:"approval" json:"approval"`
	WebUI         WebUIConfig         `mapstructure:"webui" json:"webui"`
	mu            sync.RWMutex
}

// AgentsConfig contains agent-related configuration.
type AgentsConfig struct {
	Defaults AgentDefaults `mapstructure:"defaults" json:"defaults"`
}

// AgentDefaults defines default settings for agents.
type AgentDefaults struct {
	Workspace           string   `mapstructure:"workspace" json:"workspace"`
	RestrictToWorkspace bool     `mapstructure:"restrict_to_workspace" json:"restrict_to_workspace"`
	Provider            string   `mapstructure:"provider" json:"provider"`
	Fallback            []string `mapstructure:"fallback" json:"fallback"`
	Model               string   `mapstructure:"model" json:"model"`
	MaxTokens           int      `mapstructure:"max_tokens" json:"max_tokens"`
	Temperature         float64  `mapstructure:"temperature" json:"temperature"`
	MaxToolIterations   int      `mapstructure:"max_tool_iterations" json:"max_tool_iterations"`
	SkillsDir           string   `mapstructure:"skills_dir" json:"skills_dir"`
	SkillsAutoReload    bool     `mapstructure:"skills_auto_reload" json:"skills_auto_reload"`
	ExtendedThinking    bool     `mapstructure:"extended_thinking" json:"extended_thinking"`
	ThinkingBudget      int      `mapstructure:"thinking_budget" json:"thinking_budget"`
}

// ChannelsConfig contains all channel configurations.
type ChannelsConfig struct {
	TimeoutSeconds int              `mapstructure:"timeout_seconds" json:"timeout_seconds"`
	WhatsApp       WhatsAppConfig   `mapstructure:"whatsapp" json:"whatsapp"`
	Telegram       TelegramConfig   `mapstructure:"telegram" json:"telegram"`
	Feishu         FeishuConfig     `mapstructure:"feishu" json:"feishu"`
	Discord        DiscordConfig    `mapstructure:"discord" json:"discord"`
	MaixCam        MaixCamConfig    `mapstructure:"maixcam" json:"maixcam"`
	QQ             QQConfig         `mapstructure:"qq" json:"qq"`
	DingTalk       DingTalkConfig   `mapstructure:"dingtalk" json:"dingtalk"`
	Slack          SlackConfig      `mapstructure:"slack" json:"slack"`
	ServerChan     ServerChanConfig `mapstructure:"serverchan" json:"serverchan"`
	WeWork         WeWorkConfig     `mapstructure:"wework" json:"wework"`
	GoogleChat     GoogleChatConfig `mapstructure:"googlechat" json:"googlechat"`
	Teams          TeamsConfig      `mapstructure:"teams" json:"teams"`
	Infoflow       InfoflowConfig   `mapstructure:"infoflow" json:"infoflow"`
}

// WhatsAppConfig for WhatsApp channel.
type WhatsAppConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	BridgeURL string   `mapstructure:"bridge_url" json:"bridge_url"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
}

// TelegramConfig for Telegram channel.
type TelegramConfig struct {
	Enabled        bool     `mapstructure:"enabled" json:"enabled"`
	Token          string   `mapstructure:"token" json:"token"`
	Proxy          string   `mapstructure:"proxy" json:"proxy"`
	TimeoutSeconds int      `mapstructure:"timeout_seconds" json:"timeout_seconds"`
	AllowFrom      []string `mapstructure:"allow_from" json:"allow_from"`
}

// FeishuConfig for Feishu (Lark) channel.
type FeishuConfig struct {
	Enabled           bool     `mapstructure:"enabled" json:"enabled"`
	AppID             string   `mapstructure:"app_id" json:"app_id"`
	AppSecret         string   `mapstructure:"app_secret" json:"app_secret"`
	EncryptKey        string   `mapstructure:"encrypt_key" json:"encrypt_key"`
	VerificationToken string   `mapstructure:"verification_token" json:"verification_token"`
	AllowFrom         []string `mapstructure:"allow_from" json:"allow_from"`
}

// DiscordConfig for Discord channel.
type DiscordConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	Token     string   `mapstructure:"token" json:"token"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
}

// MaixCamConfig for MaixCAM channel.
type MaixCamConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	Host      string   `mapstructure:"host" json:"host"`
	Port      int      `mapstructure:"port" json:"port"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
}

// QQConfig for QQ channel.
type QQConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	AppID     string   `mapstructure:"app_id" json:"app_id"`
	AppSecret string   `mapstructure:"app_secret" json:"app_secret"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
}

// DingTalkConfig for DingTalk channel.
type DingTalkConfig struct {
	Enabled      bool     `mapstructure:"enabled" json:"enabled"`
	ClientID     string   `mapstructure:"client_id" json:"client_id"`
	ClientSecret string   `mapstructure:"client_secret" json:"client_secret"`
	AllowFrom    []string `mapstructure:"allow_from" json:"allow_from"`
}

// SlackConfig for Slack channel.
type SlackConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	BotToken  string   `mapstructure:"bot_token" json:"bot_token"`
	AppToken  string   `mapstructure:"app_token" json:"app_token"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
}

// ServerChanConfig for ServerChan Bot channel.
type ServerChanConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	BotToken  string   `mapstructure:"bot_token" json:"bot_token"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
}

// WeWorkConfig for WeWork (企业微信) channel.
type WeWorkConfig struct {
	Enabled        bool     `mapstructure:"enabled" json:"enabled"`
	CorpID         string   `mapstructure:"corp_id" json:"corp_id"`
	AgentID        string   `mapstructure:"agent_id" json:"agent_id"`
	CorpSecret     string   `mapstructure:"corp_secret" json:"corp_secret"`
	Token          string   `mapstructure:"token" json:"token"`
	EncodingAESKey string   `mapstructure:"encoding_aes_key" json:"encoding_aes_key"`
	AllowFrom      []string `mapstructure:"allow_from" json:"allow_from"`
}

// GoogleChatConfig for Google Chat channel.
type GoogleChatConfig struct {
	Enabled         bool     `mapstructure:"enabled" json:"enabled"`
	ProjectID       string   `mapstructure:"project_id" json:"project_id"`
	CredentialsFile string   `mapstructure:"credentials_file" json:"credentials_file"`
	AllowFrom       []string `mapstructure:"allow_from" json:"allow_from"`
}

// TeamsConfig for Microsoft Teams channel.
type TeamsConfig struct {
	Enabled     bool     `mapstructure:"enabled" json:"enabled"`
	AppID       string   `mapstructure:"app_id" json:"app_id"`
	AppPassword string   `mapstructure:"app_password" json:"app_password"`
	AllowFrom   []string `mapstructure:"allow_from" json:"allow_from"`
}

// InfoflowConfig for infoflow channel.
type InfoflowConfig struct {
	Enabled    bool     `mapstructure:"enabled" json:"enabled"`
	WebhookURL string   `mapstructure:"webhook_url" json:"webhook_url"`
	AESKey     string   `mapstructure:"aes_key" json:"aes_key"`
	AllowFrom  []string `mapstructure:"allow_from" json:"allow_from"`
}

// HeartbeatConfig for periodic autonomous tasks.
type HeartbeatConfig struct {
	Enabled         bool `mapstructure:"enabled" json:"enabled"`
	IntervalMinutes int  `mapstructure:"interval_minutes" json:"interval_minutes"` // minutes, min 5
}

// RedisConfig is the shared Redis connection configuration.
// Configure once, used by both state and bus modules when their backend is "redis".
type RedisConfig struct {
	Addr     string `mapstructure:"addr" json:"addr"`         // Redis address (host:port), e.g. "localhost:6379"
	Password string `mapstructure:"password" json:"password"` // Redis password (optional)
	DB       int    `mapstructure:"db" json:"db"`             // Redis database number (default 0)
}

// StateConfig for key-value storage backend.
type StateConfig struct {
	Backend  string `mapstructure:"backend" json:"backend"`     // "file" or "redis"
	FilePath string `mapstructure:"file_path" json:"file_path"` // For file backend
	Prefix   string `mapstructure:"prefix" json:"prefix"`       // Redis key prefix (default "nekobot:")
}

// BusConfig for message bus backend.
type BusConfig struct {
	Type   string `mapstructure:"type" json:"type"`     // "local" or "redis"
	Prefix string `mapstructure:"prefix" json:"prefix"` // Redis key prefix (default "nekobot:bus:")
}

// ProvidersConfig contains provider configurations.
// Providers is an array of ProviderProfile.
type ProvidersConfig []ProviderProfile

// ProviderProfile defines a provider profile with type and alias.
type ProviderProfile struct {
	Name         string   `mapstructure:"name" json:"name"`                   // Alias (e.g., "openai-primary", "my-api")
	ProviderKind string   `mapstructure:"provider_kind" json:"provider_kind"` // Type: "openai", "anthropic", "gemini"
	APIKey       string   `mapstructure:"api_key" json:"api_key"`
	APIBase      string   `mapstructure:"api_base" json:"api_base"`
	Proxy        string   `mapstructure:"proxy" json:"proxy,omitempty"`                 // HTTP/SOCKS5 proxy URL (optional)
	Models       []string `mapstructure:"models" json:"models,omitempty"`               // Supported model list
	DefaultModel string   `mapstructure:"default_model" json:"default_model,omitempty"` // Default model for this provider
	Timeout      int      `mapstructure:"timeout" json:"timeout,omitempty"`             // Timeout in seconds, default 30s
}

// LoggerConfig contains logger configuration.
type LoggerConfig struct {
	Level      string `mapstructure:"level" json:"level"`             // Log level: debug, info, warn, error, fatal
	OutputPath string `mapstructure:"output_path" json:"output_path"` // Log file path, empty means stdout only
	MaxSize    int    `mapstructure:"max_size" json:"max_size"`       // Max size in MB before rotation
	MaxBackups int    `mapstructure:"max_backups" json:"max_backups"` // Max number of old log files
	MaxAge     int    `mapstructure:"max_age" json:"max_age"`         // Max days to retain old log files
	Compress   bool   `mapstructure:"compress" json:"compress"`       // Compress rotated files
}

// GatewayConfig for gateway server.
type GatewayConfig struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

// TranscriptionConfig controls speech-to-text behavior.
type TranscriptionConfig struct {
	Enabled        bool   `mapstructure:"enabled" json:"enabled"`
	Provider       string `mapstructure:"provider" json:"provider"`
	APIKey         string `mapstructure:"api_key" json:"api_key"`
	APIBase        string `mapstructure:"api_base" json:"api_base"`
	Model          string `mapstructure:"model" json:"model"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds" json:"timeout_seconds"`
}

// ToolsConfig contains tool-related configuration.
type ToolsConfig struct {
	Web  WebToolsConfig  `mapstructure:"web" json:"web"`
	Exec ExecToolsConfig `mapstructure:"exec" json:"exec"`
}

// WebToolsConfig for web-related tools.
type WebToolsConfig struct {
	Search WebSearchConfig `mapstructure:"search" json:"search"`
	Fetch  WebFetchConfig  `mapstructure:"fetch" json:"fetch"`
}

// WebSearchConfig for web search tool.
type WebSearchConfig struct {
	BraveAPIKey          string `mapstructure:"brave_api_key" json:"brave_api_key"`
	LegacyAPIKey         string `mapstructure:"api_key" json:"-"`
	MaxResults           int    `mapstructure:"max_results" json:"max_results"`
	DuckDuckGoEnabled    bool   `mapstructure:"duckduckgo_enabled" json:"duckduckgo_enabled"`
	DuckDuckGoMaxResults int    `mapstructure:"duckduckgo_max_results" json:"duckduckgo_max_results"`
}

// WebFetchConfig for web fetch tool.
type WebFetchConfig struct {
	MaxChars int `mapstructure:"max_chars" json:"max_chars"`
}

// ExecToolsConfig for the exec tool.
type ExecToolsConfig struct {
	TimeoutSeconds int                 `mapstructure:"timeout_seconds" json:"timeout_seconds"`
	Sandbox        DockerSandboxConfig `mapstructure:"sandbox" json:"sandbox"`
}

// DockerSandboxConfig controls containerized execution for exec tool.
type DockerSandboxConfig struct {
	Enabled     bool     `mapstructure:"enabled" json:"enabled"`
	Image       string   `mapstructure:"image" json:"image"`
	NetworkMode string   `mapstructure:"network_mode" json:"network_mode"`
	Mounts      []string `mapstructure:"mounts" json:"mounts"`
	Timeout     int      `mapstructure:"timeout" json:"timeout"`
	AutoCleanup bool     `mapstructure:"auto_cleanup" json:"auto_cleanup"`
}

// DefaultConfig returns a new Config with default values.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	workspace := filepath.Join(homeDir, ".nekobot", "workspace")

	return &Config{
		Logger: LoggerConfig{
			Level:      "info",
			OutputPath: "",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
		},
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:           workspace,
				RestrictToWorkspace: true,
				Provider:            "",
				Model:               "claude-sonnet-4-5-20250929",
				MaxTokens:           8192,
				Temperature:         0.7,
				MaxToolIterations:   20,
			},
		},
		Channels: ChannelsConfig{
			TimeoutSeconds: 60,
			WhatsApp: WhatsAppConfig{
				Enabled:   false,
				BridgeURL: "ws://localhost:3001",
				AllowFrom: []string{},
			},
			Telegram: TelegramConfig{
				Enabled:        false,
				TimeoutSeconds: 60,
				AllowFrom:      []string{},
			},
			Feishu: FeishuConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			Discord: DiscordConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			MaixCam: MaixCamConfig{
				Enabled:   false,
				Host:      "0.0.0.0",
				Port:      18790,
				AllowFrom: []string{},
			},
			QQ: QQConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			DingTalk: DingTalkConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			Slack: SlackConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			ServerChan: ServerChanConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			WeWork: WeWorkConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			GoogleChat: GoogleChatConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			Teams: TeamsConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			Infoflow: InfoflowConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
		},
		Providers: []ProviderProfile{},
		Transcription: TranscriptionConfig{
			Enabled:        true,
			Provider:       "groq",
			APIBase:        "https://api.groq.com/openai/v1",
			Model:          "whisper-large-v3-turbo",
			TimeoutSeconds: 90,
		},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 18790,
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Search: WebSearchConfig{
					MaxResults:           5,
					DuckDuckGoEnabled:    true,
					DuckDuckGoMaxResults: 5,
				},
				Fetch: WebFetchConfig{
					MaxChars: 50000,
				},
			},
			Exec: ExecToolsConfig{
				TimeoutSeconds: 30,
				Sandbox: DockerSandboxConfig{
					Enabled:     false,
					Image:       "alpine:3.20",
					NetworkMode: "none",
					Mounts:      []string{},
					Timeout:     60,
					AutoCleanup: true,
				},
			},
		},
		Heartbeat: HeartbeatConfig{
			Enabled:         true,
			IntervalMinutes: 30, // 30 minutes
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
		},
		State: StateConfig{
			Backend:  "file",
			FilePath: "", // Will be set to workspace/state.json by state module
			Prefix:   "nekobot:",
		},
		Bus: BusConfig{
			Type:   "local",
			Prefix: "nekobot:bus:",
		},
		Approval: ApprovalConfig{
			Mode:      "auto",
			Allowlist: []string{},
			Denylist:  []string{},
		},
		WebUI: WebUIConfig{
			Enabled:                  true,
			Port:                     0, // 0 means gateway port + 1
			PublicBaseURL:            "",
			Username:                 "admin",
			ToolSessionOTPTTLSeconds: 180,
		},
	}
}

// WorkspacePath returns the expanded workspace path.
func (c *Config) WorkspacePath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return expandPath(c.Agents.Defaults.Workspace)
}

// GetProviderConfig returns the configuration for a specific provider.
func (c *Config) GetProviderConfig(providerName string) *ProviderProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Search in providers
	for i := range c.Providers {
		profile := &c.Providers[i]
		if profile.Name == providerName {
			return profile
		}
	}

	return nil
}

// GetDefaultModel returns the default model for this provider profile.
// If DefaultModel is set, returns it. Otherwise, returns the first model from Models list.
// Returns empty string if no models are configured.
func (p *ProviderProfile) GetDefaultModel() string {
	// If default model is explicitly set, use it
	if p.DefaultModel != "" {
		return p.DefaultModel
	}

	// Otherwise, use first model from models list
	if len(p.Models) > 0 {
		return p.Models[0]
	}

	return ""
}

// GetTimeout returns the timeout in seconds. Returns 30 if not set.
func (p *ProviderProfile) GetTimeout() int {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return 30 // Default 30 seconds
}

// GetBraveAPIKey returns the Brave search API key with backward compatibility.
func (w WebSearchConfig) GetBraveAPIKey() string {
	if strings.TrimSpace(w.BraveAPIKey) != "" {
		return strings.TrimSpace(w.BraveAPIKey)
	}
	return strings.TrimSpace(w.LegacyAPIKey)
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
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

// MemoryConfig for memory and QMD integration.
type MemoryConfig struct {
	QMD QMDConfig `mapstructure:"qmd" json:"qmd"`
}

// QMDConfig for QMD (Query Markdown) integration.
type QMDConfig struct {
	Enabled        bool              `mapstructure:"enabled" json:"enabled"`
	Command        string            `mapstructure:"command" json:"command"`
	IncludeDefault bool              `mapstructure:"include_default" json:"include_default"`
	Paths          []QMDPathConfig   `mapstructure:"paths" json:"paths"`
	Sessions       QMDSessionsConfig `mapstructure:"sessions" json:"sessions"`
	Update         QMDUpdateConfig   `mapstructure:"update" json:"update"`
}

// QMDPathConfig defines a custom collection path.
type QMDPathConfig struct {
	Name    string `mapstructure:"name" json:"name"`
	Path    string `mapstructure:"path" json:"path"`
	Pattern string `mapstructure:"pattern" json:"pattern"`
}

// QMDSessionsConfig for session export configuration.
type QMDSessionsConfig struct {
	Enabled       bool   `mapstructure:"enabled" json:"enabled"`
	ExportDir     string `mapstructure:"export_dir" json:"export_dir"`
	RetentionDays int    `mapstructure:"retention_days" json:"retention_days"`
}

// QMDUpdateConfig for automatic update configuration.
type QMDUpdateConfig struct {
	OnBoot         bool   `mapstructure:"on_boot" json:"on_boot"`
	Interval       string `mapstructure:"interval" json:"interval"`
	CommandTimeout string `mapstructure:"command_timeout" json:"command_timeout"`
	UpdateTimeout  string `mapstructure:"update_timeout" json:"update_timeout"`
}

// ApprovalConfig for tool execution approval system.
type ApprovalConfig struct {
	Mode      string   `mapstructure:"mode" json:"mode"`           // "auto", "prompt", or "manual"
	Allowlist []string `mapstructure:"allowlist" json:"allowlist"` // Tools that bypass approval
	Denylist  []string `mapstructure:"denylist" json:"denylist"`   // Tools that are always denied
}

// WebUIConfig for the web dashboard.
type WebUIConfig struct {
	Enabled                  bool   `mapstructure:"enabled" json:"enabled"`                                           // Enable WebUI (default true in daemon mode)
	Port                     int    `mapstructure:"port" json:"port"`                                                 // WebUI port (default: gateway port + 1)
	PublicBaseURL            string `mapstructure:"public_base_url" json:"public_base_url"`                           // Preferred external base URL for share links
	Secret                   string `mapstructure:"secret" json:"secret"`                                             // JWT secret for auth (auto-generated on first run)
	Username                 string `mapstructure:"username" json:"username"`                                         // Admin username (default: admin)
	Password                 string `mapstructure:"password" json:"password"`                                         // Admin password (set on first run)
	ToolSessionOTPTTLSeconds int    `mapstructure:"tool_session_otp_ttl_seconds" json:"tool_session_otp_ttl_seconds"` // One-time password TTL for tool sessions (seconds)
}
