// Package config provides configuration management for nanobot.
// It uses Viper for flexible configuration loading with support for:
// - Multiple formats (JSON, YAML, TOML)
// - Environment variables
// - Hot-reload
// - Default values
package config

import (
	"os"
	"sync"
)

// Config represents the complete nanobot configuration.
type Config struct {
	Agents    AgentsConfig    `mapstructure:"agents" json:"agents"`
	Channels  ChannelsConfig  `mapstructure:"channels" json:"channels"`
	Providers ProvidersConfig `mapstructure:"providers" json:"providers"`
	Gateway   GatewayConfig   `mapstructure:"gateway" json:"gateway"`
	Tools     ToolsConfig     `mapstructure:"tools" json:"tools"`
	Heartbeat HeartbeatConfig `mapstructure:"heartbeat" json:"heartbeat"`
	mu        sync.RWMutex
}

// AgentsConfig contains agent-related configuration.
type AgentsConfig struct {
	Defaults AgentDefaults `mapstructure:"defaults" json:"defaults"`
}

// AgentDefaults defines default settings for agents.
type AgentDefaults struct {
	Workspace           string  `mapstructure:"workspace" json:"workspace"`
	RestrictToWorkspace bool    `mapstructure:"restrict_to_workspace" json:"restrict_to_workspace"`
	Provider            string  `mapstructure:"provider" json:"provider"`
	Model               string  `mapstructure:"model" json:"model"`
	MaxTokens           int     `mapstructure:"max_tokens" json:"max_tokens"`
	Temperature         float64 `mapstructure:"temperature" json:"temperature"`
	MaxToolIterations   int     `mapstructure:"max_tool_iterations" json:"max_tool_iterations"`
}

// ChannelsConfig contains all channel configurations.
type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `mapstructure:"whatsapp" json:"whatsapp"`
	Telegram TelegramConfig `mapstructure:"telegram" json:"telegram"`
	Feishu   FeishuConfig   `mapstructure:"feishu" json:"feishu"`
	Discord  DiscordConfig  `mapstructure:"discord" json:"discord"`
	MaixCam  MaixCamConfig  `mapstructure:"maixcam" json:"maixcam"`
	QQ       QQConfig       `mapstructure:"qq" json:"qq"`
	DingTalk DingTalkConfig `mapstructure:"dingtalk" json:"dingtalk"`
	Slack    SlackConfig    `mapstructure:"slack" json:"slack"`
}

// WhatsAppConfig for WhatsApp channel.
type WhatsAppConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	BridgeURL string   `mapstructure:"bridge_url" json:"bridge_url"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
}

// TelegramConfig for Telegram channel.
type TelegramConfig struct {
	Enabled   bool     `mapstructure:"enabled" json:"enabled"`
	Token     string   `mapstructure:"token" json:"token"`
	Proxy     string   `mapstructure:"proxy" json:"proxy"`
	AllowFrom []string `mapstructure:"allow_from" json:"allow_from"`
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

// HeartbeatConfig for periodic autonomous tasks.
type HeartbeatConfig struct {
	Enabled  bool `mapstructure:"enabled" json:"enabled"`
	Interval int  `mapstructure:"interval" json:"interval"` // minutes, min 5
}

// ProvidersConfig contains all provider configurations.
type ProvidersConfig struct {
	Anthropic    ProviderConfig `mapstructure:"anthropic" json:"anthropic"`
	OpenAI       ProviderConfig `mapstructure:"openai" json:"openai"`
	OpenRouter   ProviderConfig `mapstructure:"openrouter" json:"openrouter"`
	Groq         ProviderConfig `mapstructure:"groq" json:"groq"`
	Zhipu        ProviderConfig `mapstructure:"zhipu" json:"zhipu"`
	VLLM         ProviderConfig `mapstructure:"vllm" json:"vllm"`
	Gemini       ProviderConfig `mapstructure:"gemini" json:"gemini"`
	Nvidia       ProviderConfig `mapstructure:"nvidia" json:"nvidia"`
	Moonshot     ProviderConfig `mapstructure:"moonshot" json:"moonshot"`
	DeepSeek     ProviderConfig `mapstructure:"deepseek" json:"deepseek"`
}

// ProviderConfig defines configuration for a single provider.
type ProviderConfig struct {
	APIKey     string `mapstructure:"api_key" json:"api_key"`
	APIBase    string `mapstructure:"api_base" json:"api_base"`
	Proxy      string `mapstructure:"proxy" json:"proxy"`
	AuthMethod string `mapstructure:"auth_method" json:"auth_method"` // "api_key", "oauth", "token"
}

// GatewayConfig for gateway server.
type GatewayConfig struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

// ToolsConfig contains tool-related configuration.
type ToolsConfig struct {
	Web WebToolsConfig `mapstructure:"web" json:"web"`
}

// WebToolsConfig for web-related tools.
type WebToolsConfig struct {
	Search WebSearchConfig `mapstructure:"search" json:"search"`
}

// WebSearchConfig for web search tool.
type WebSearchConfig struct {
	APIKey     string `mapstructure:"api_key" json:"api_key"`
	MaxResults int    `mapstructure:"max_results" json:"max_results"`
}

// DefaultConfig returns a new Config with default values.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	workspace := homeDir + "/.nanobot/workspace"

	return &Config{
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
			WhatsApp: WhatsAppConfig{
				Enabled:   false,
				BridgeURL: "ws://localhost:3001",
				AllowFrom: []string{},
			},
			Telegram: TelegramConfig{
				Enabled:   false,
				AllowFrom: []string{},
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
		},
		Providers: ProvidersConfig{
			Anthropic:  ProviderConfig{},
			OpenAI:     ProviderConfig{},
			OpenRouter: ProviderConfig{},
			Groq:       ProviderConfig{},
			Zhipu:      ProviderConfig{},
			VLLM:       ProviderConfig{},
			Gemini:     ProviderConfig{},
			Nvidia:     ProviderConfig{},
			Moonshot:   ProviderConfig{},
			DeepSeek:   ProviderConfig{},
		},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 18790,
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Search: WebSearchConfig{
					MaxResults: 5,
				},
			},
		},
		Heartbeat: HeartbeatConfig{
			Enabled:  true,
			Interval: 30, // 30 minutes
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
func (c *Config) GetProviderConfig(providerName string) *ProviderConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch providerName {
	case "anthropic", "claude":
		return &c.Providers.Anthropic
	case "openai", "gpt":
		return &c.Providers.OpenAI
	case "openrouter":
		return &c.Providers.OpenRouter
	case "groq":
		return &c.Providers.Groq
	case "zhipu", "glm":
		return &c.Providers.Zhipu
	case "vllm":
		return &c.Providers.VLLM
	case "gemini", "google":
		return &c.Providers.Gemini
	case "nvidia":
		return &c.Providers.Nvidia
	case "moonshot", "kimi":
		return &c.Providers.Moonshot
	case "deepseek":
		return &c.Providers.DeepSeek
	default:
		return nil
	}
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
