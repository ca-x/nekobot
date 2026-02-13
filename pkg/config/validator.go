package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation errors:\n", len(e)))
	for _, err := range e {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}
	return sb.String()
}

// Validator validates configuration.
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new configuration validator.
func NewValidator() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// Validate validates the entire configuration.
func (v *Validator) Validate(cfg *Config) error {
	v.errors = make(ValidationErrors, 0)

	// Validate agents configuration
	v.validateAgents(&cfg.Agents)

	// Validate providers configuration
	v.validateProviders(&cfg.Providers)

	// Validate channels configuration
	v.validateChannels(&cfg.Channels)

	// Validate gateway configuration
	v.validateGateway(&cfg.Gateway)

	// Validate tools configuration
	v.validateTools(&cfg.Tools)

	// Validate heartbeat configuration
	v.validateHeartbeat(&cfg.Heartbeat)

	if len(v.errors) > 0 {
		return v.errors
	}

	return nil
}

// validateAgents validates agent configuration.
func (v *Validator) validateAgents(cfg *AgentsConfig) {
	if cfg.Defaults.Workspace == "" {
		v.addError("agents.defaults.workspace", "workspace path is required")
	}

	if cfg.Defaults.MaxTokens < 0 {
		v.addError("agents.defaults.max_tokens", "max_tokens must be non-negative")
	}

	if cfg.Defaults.Temperature < 0 || cfg.Defaults.Temperature > 2 {
		v.addError("agents.defaults.temperature", "temperature must be between 0 and 2")
	}

	if cfg.Defaults.MaxToolIterations < 1 {
		v.addError("agents.defaults.max_tool_iterations", "max_tool_iterations must be at least 1")
	}
}

// validateProviders validates provider configuration.
func (v *Validator) validateProviders(cfg *ProvidersConfig) {
	providers := map[string]ProviderConfig{
		"anthropic":  cfg.Anthropic,
		"openai":     cfg.OpenAI,
		"openrouter": cfg.OpenRouter,
		"groq":       cfg.Groq,
		"zhipu":      cfg.Zhipu,
		"vllm":       cfg.VLLM,
		"gemini":     cfg.Gemini,
		"nvidia":     cfg.Nvidia,
		"moonshot":   cfg.Moonshot,
		"deepseek":   cfg.DeepSeek,
	}

	// Check that at least one provider is configured
	hasProvider := false
	for _, provider := range providers {
		if provider.APIKey != "" || provider.APIBase != "" {
			hasProvider = true
			break
		}
	}

	if !hasProvider {
		v.addError("providers", "at least one provider must be configured")
	}

	// Validate individual providers
	for name, provider := range providers {
		if provider.APIKey == "" && provider.APIBase == "" {
			continue // Skip unconfigured providers
		}

		// Validate API base URL if provided
		if provider.APIBase != "" {
			if _, err := url.Parse(provider.APIBase); err != nil {
				v.addError(fmt.Sprintf("providers.%s.api_base", name),
					fmt.Sprintf("invalid URL: %v", err))
			}
		}

		// Validate proxy URL if provided
		if provider.Proxy != "" {
			if _, err := url.Parse(provider.Proxy); err != nil {
				v.addError(fmt.Sprintf("providers.%s.proxy", name),
					fmt.Sprintf("invalid proxy URL: %v", err))
			}
		}

		// Validate auth method
		if provider.AuthMethod != "" {
			validMethods := map[string]bool{
				"api_key": true,
				"oauth":   true,
				"token":   true,
			}
			if !validMethods[provider.AuthMethod] {
				v.addError(fmt.Sprintf("providers.%s.auth_method", name),
					"auth_method must be one of: api_key, oauth, token")
			}
		}
	}
}

// validateChannels validates channel configuration.
func (v *Validator) validateChannels(cfg *ChannelsConfig) {
	// Validate Telegram
	if cfg.Telegram.Enabled && cfg.Telegram.Token == "" {
		v.addError("channels.telegram.token", "token is required when Telegram is enabled")
	}

	// Validate Feishu
	if cfg.Feishu.Enabled {
		if cfg.Feishu.AppID == "" {
			v.addError("channels.feishu.app_id", "app_id is required when Feishu is enabled")
		}
		if cfg.Feishu.AppSecret == "" {
			v.addError("channels.feishu.app_secret", "app_secret is required when Feishu is enabled")
		}
	}

	// Validate Discord
	if cfg.Discord.Enabled && cfg.Discord.Token == "" {
		v.addError("channels.discord.token", "token is required when Discord is enabled")
	}

	// Validate WhatsApp
	if cfg.WhatsApp.Enabled && cfg.WhatsApp.BridgeURL == "" {
		v.addError("channels.whatsapp.bridge_url", "bridge_url is required when WhatsApp is enabled")
	}

	// Validate MaixCam
	if cfg.MaixCam.Enabled {
		if cfg.MaixCam.Port < 1 || cfg.MaixCam.Port > 65535 {
			v.addError("channels.maixcam.port", "port must be between 1 and 65535")
		}
	}

	// Validate QQ
	if cfg.QQ.Enabled {
		if cfg.QQ.AppID == "" {
			v.addError("channels.qq.app_id", "app_id is required when QQ is enabled")
		}
		if cfg.QQ.AppSecret == "" {
			v.addError("channels.qq.app_secret", "app_secret is required when QQ is enabled")
		}
	}

	// Validate DingTalk
	if cfg.DingTalk.Enabled {
		if cfg.DingTalk.ClientID == "" {
			v.addError("channels.dingtalk.client_id", "client_id is required when DingTalk is enabled")
		}
		if cfg.DingTalk.ClientSecret == "" {
			v.addError("channels.dingtalk.client_secret", "client_secret is required when DingTalk is enabled")
		}
	}

	// Validate Slack
	if cfg.Slack.Enabled {
		if cfg.Slack.BotToken == "" {
			v.addError("channels.slack.bot_token", "bot_token is required when Slack is enabled")
		}
		if cfg.Slack.AppToken == "" {
			v.addError("channels.slack.app_token", "app_token is required when Slack is enabled")
		}
	}
}

// validateGateway validates gateway configuration.
func (v *Validator) validateGateway(cfg *GatewayConfig) {
	if cfg.Port < 1 || cfg.Port > 65535 {
		v.addError("gateway.port", "port must be between 1 and 65535")
	}

	if cfg.Host == "" {
		v.addError("gateway.host", "host is required")
	}
}

// validateTools validates tools configuration.
func (v *Validator) validateTools(cfg *ToolsConfig) {
	if cfg.Web.Search.MaxResults < 1 {
		v.addError("tools.web.search.max_results", "max_results must be at least 1")
	}
}

// validateHeartbeat validates heartbeat configuration.
func (v *Validator) validateHeartbeat(cfg *HeartbeatConfig) {
	if cfg.Enabled && cfg.IntervalMinutes < 5 {
		v.addError("heartbeat.interval_minutes", "interval must be at least 5 minutes when heartbeat is enabled")
	}
}

// addError adds a validation error.
func (v *Validator) addError(field, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// ValidateConfig is a convenience function to validate a configuration.
func ValidateConfig(cfg *Config) error {
	validator := NewValidator()
	return validator.Validate(cfg)
}
