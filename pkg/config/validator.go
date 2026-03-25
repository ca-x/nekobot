package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
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

	// Validate transcription configuration
	v.validateTranscription(&cfg.Transcription)

	// Validate heartbeat configuration
	v.validateHeartbeat(&cfg.Heartbeat)

	// Validate memory configuration
	v.validateMemory(&cfg.Memory)

	// Validate session persistence configuration
	v.validateSessions(&cfg.Sessions)

	// Validate web UI configuration.
	v.validateWebUI(&cfg.WebUI)

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

	orchestrator := strings.TrimSpace(strings.ToLower(cfg.Defaults.Orchestrator))
	if orchestrator == "" {
		v.addError("agents.defaults.orchestrator", "orchestrator is required")
	} else if orchestrator != "legacy" && orchestrator != "blades" {
		v.addError("agents.defaults.orchestrator", "orchestrator must be one of: legacy, blades")
	}

	for i, server := range cfg.Defaults.MCPServers {
		prefix := fmt.Sprintf("agents.defaults.mcp_servers[%d]", i)
		v.validateMCPServer(prefix, server)
	}

	for i, group := range cfg.Defaults.ProviderGroups {
		prefix := fmt.Sprintf("agents.defaults.provider_groups[%d]", i)
		v.validateProviderGroup(prefix, group)
	}
}

func (v *Validator) validateMCPServer(prefix string, cfg MCPServerConfig) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		v.addError(prefix+".name", "name is required")
	}

	transport := strings.TrimSpace(strings.ToLower(cfg.Transport))
	if transport == "" {
		v.addError(prefix+".transport", "transport is required")
		return
	}

	switch transport {
	case "stdio":
		if strings.TrimSpace(cfg.Command) == "" {
			v.addError(prefix+".command", "command is required when transport is stdio")
		}
	case "http", "websocket", "sse":
		if strings.TrimSpace(cfg.Endpoint) == "" {
			v.addError(prefix+".endpoint", "endpoint is required when transport is http, websocket, or sse")
			break
		}
		if _, err := url.Parse(cfg.Endpoint); err != nil {
			v.addError(prefix+".endpoint", fmt.Sprintf("invalid URL: %v", err))
		}
	default:
		v.addError(prefix+".transport", "transport must be one of: stdio, http, websocket, sse")
	}

	if timeout := strings.TrimSpace(cfg.Timeout); timeout != "" {
		if _, err := parseMCPTimeout(timeout); err != nil {
			v.addError(prefix+".timeout", err.Error())
		}
	}
}

func parseMCPTimeout(raw string) (int64, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout duration: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("timeout duration must be greater than 0")
	}
	return int64(d), nil
}

func (v *Validator) validateProviderGroup(prefix string, cfg ProviderGroupConfig) {
	if strings.TrimSpace(cfg.Name) == "" {
		v.addError(prefix+".name", "name is required")
	}

	strategy := strings.TrimSpace(strings.ToLower(cfg.Strategy))
	switch strategy {
	case "", "round_robin", "least_used", "random":
	default:
		v.addError(prefix+".strategy", "strategy must be one of: round_robin, least_used, random")
	}

	if len(cfg.Members) < 2 {
		v.addError(prefix+".members", "at least two provider members are required")
	}
}

// validateProviders validates provider configuration.
func (v *Validator) validateProviders(cfg *ProvidersConfig) {
	// Track profile names to ensure uniqueness
	names := make(map[string]bool)

	// Validate individual provider profiles
	for i, profile := range *cfg {
		prefix := fmt.Sprintf("providers[%d]", i)

		// Validate name
		if profile.Name == "" {
			v.addError(prefix+".name", "provider name is required")
		} else if names[profile.Name] {
			v.addError(prefix+".name", fmt.Sprintf("duplicate provider name: %s", profile.Name))
		} else {
			names[profile.Name] = true
		}

		// Validate provider type
		if strings.TrimSpace(profile.ProviderKind) == "" {
			v.addError(prefix+".provider_kind", "provider_kind is required")
		}

		// API key is required (can be placeholder for local providers)
		// if profile.APIKey == "" {
		// 	v.addError(prefix+".api_key", "API key is empty")
		// }

		// Validate API base URL if provided
		if profile.APIBase != "" {
			if _, err := url.Parse(profile.APIBase); err != nil {
				v.addError(prefix+".api_base", fmt.Sprintf("invalid URL: %v", err))
			}
		}

		// Models list is optional - some profiles might not specify models
		// Default model validation is optional - if not specified, first model will be used
	}
}

// validateChannels validates channel configuration.
func (v *Validator) validateChannels(cfg *ChannelsConfig) {
	// Validate Telegram
	if cfg.Telegram.Enabled && cfg.Telegram.Token == "" {
		v.addError("channels.telegram.token", "token is required when Telegram is enabled")
	}

	// Validate Gotify
	if cfg.Gotify.Enabled {
		if strings.TrimSpace(cfg.Gotify.ServerURL) == "" {
			v.addError("channels.gotify.server_url", "server_url is required when Gotify is enabled")
		} else if _, err := url.Parse(cfg.Gotify.ServerURL); err != nil {
			v.addError("channels.gotify.server_url", fmt.Sprintf("invalid URL: %v", err))
		}
		if strings.TrimSpace(cfg.Gotify.AppToken) == "" {
			v.addError("channels.gotify.app_token", "app_token is required when Gotify is enabled")
		}
		if cfg.Gotify.Priority < 1 || cfg.Gotify.Priority > 10 {
			v.addError("channels.gotify.priority", "priority must be between 1 and 10 when Gotify is enabled")
		}
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

	// Validate WeChat
	if cfg.WeChat.Enabled && cfg.WeChat.PollIntervalSeconds < 1 {
		v.addError("channels.wechat.poll_interval_seconds", "poll_interval_seconds must be at least 1 when WeChat is enabled")
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

	// Validate Teams
	if cfg.Teams.Enabled {
		if cfg.Teams.AppID == "" {
			v.addError("channels.teams.app_id", "app_id is required when Teams is enabled")
		}
		if cfg.Teams.AppPassword == "" {
			v.addError("channels.teams.app_password", "app_password is required when Teams is enabled")
		}
	}

	// Validate Infoflow
	if cfg.Infoflow.Enabled && cfg.Infoflow.WebhookURL == "" {
		v.addError("channels.infoflow.webhook_url", "webhook_url is required when Infoflow is enabled")
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
	if cfg.Web.Search.DuckDuckGoEnabled && cfg.Web.Search.DuckDuckGoMaxResults < 1 {
		v.addError("tools.web.search.duckduckgo_max_results", "duckduckgo_max_results must be at least 1 when duckduckgo is enabled")
	}
	if cfg.Exec.TimeoutSeconds < 1 {
		v.addError("tools.exec.timeout_seconds", "timeout_seconds must be at least 1")
	}
	if cfg.Exec.Sandbox.Enabled {
		if cfg.Exec.Sandbox.Image == "" {
			v.addError("tools.exec.sandbox.image", "image is required when sandbox is enabled")
		}
		if cfg.Exec.Sandbox.Timeout < 1 {
			v.addError("tools.exec.sandbox.timeout", "timeout must be at least 1")
		}
	}
}

// validateTranscription validates transcription configuration.
func (v *Validator) validateTranscription(cfg *TranscriptionConfig) {
	if !cfg.Enabled {
		return
	}
	if cfg.Model == "" {
		v.addError("transcription.model", "model is required when transcription is enabled")
	}
	if cfg.TimeoutSeconds < 1 {
		v.addError("transcription.timeout_seconds", "timeout_seconds must be at least 1")
	}
}

// validateHeartbeat validates heartbeat configuration.
func (v *Validator) validateHeartbeat(cfg *HeartbeatConfig) {
	if cfg.Enabled && cfg.IntervalMinutes < 5 {
		v.addError("heartbeat.interval_minutes", "interval must be at least 5 minutes when heartbeat is enabled")
	}
}

// validateMemory validates memory configuration.
func (v *Validator) validateMemory(cfg *MemoryConfig) {
	if !cfg.Enabled {
		return
	}

	backend := strings.TrimSpace(strings.ToLower(cfg.Backend))
	if backend == "" {
		backend = "file"
	}
	if backend != "file" && backend != "db" && backend != "kv" {
		v.addError("memory.backend", "backend must be one of: file, db, kv")
	}

	if cfg.Context.Enabled {
		if cfg.Context.RecentDailyNoteDays < 0 {
			v.addError("memory.context.recent_daily_note_days", "recent_daily_note_days must be at least 0")
		}
		if cfg.Context.MaxChars < 1 {
			v.addError("memory.context.max_chars", "max_chars must be at least 1")
		}
	}

	if cfg.Semantic.Enabled {
		if cfg.Semantic.DefaultTopK < 1 {
			v.addError("memory.semantic.default_top_k", "default_top_k must be at least 1")
		}
		if cfg.Semantic.MaxTopK < 1 {
			v.addError("memory.semantic.max_top_k", "max_top_k must be at least 1")
		}
		if cfg.Semantic.DefaultTopK > cfg.Semantic.MaxTopK {
			v.addError("memory.semantic.default_top_k", "default_top_k must be less than or equal to max_top_k")
		}
		policy := strings.TrimSpace(strings.ToLower(cfg.Semantic.SearchPolicy))
		if policy == "" {
			v.addError("memory.semantic.search_policy", "search_policy is required when semantic memory is enabled")
		} else if policy != "vector" && policy != "hybrid" {
			v.addError("memory.semantic.search_policy", "search_policy must be one of: vector, hybrid")
		}
	}

	if cfg.Episodic.Enabled {
		if cfg.Episodic.SummaryWindowMessages < 1 {
			v.addError("memory.episodic.summary_window_messages", "summary_window_messages must be at least 1")
		}
		if cfg.Episodic.MaxSummaries < 1 {
			v.addError("memory.episodic.max_summaries", "max_summaries must be at least 1")
		}
	}

	if cfg.ShortTerm.Enabled && cfg.ShortTerm.RawHistoryLimit < 1 {
		v.addError("memory.short_term.raw_history_limit", "raw_history_limit must be at least 1")
	}
}

// validateSessions validates session persistence configuration.
func (v *Validator) validateSessions(cfg *SessionsConfig) {
	if !cfg.Enabled {
		return
	}

	if !cfg.Sources.CLI &&
		!cfg.Sources.TUI &&
		!cfg.Sources.WebUI &&
		!cfg.Sources.Heartbeat &&
		!cfg.Sources.Cron &&
		!cfg.Sources.Channels &&
		!cfg.Sources.Gateway {
		v.addError("sessions.sources", "at least one session source must be enabled when session persistence is enabled")
	}

	if !cfg.Content.UserMessages &&
		!cfg.Content.AssistantMessages &&
		!cfg.Content.SystemMessages &&
		!cfg.Content.ToolCalls &&
		!cfg.Content.ToolResults {
		v.addError("sessions.content", "at least one session content category must be enabled when session persistence is enabled")
	}

	if cfg.Cleanup.Enabled {
		if cfg.Cleanup.IntervalMinutes < 1 {
			v.addError("sessions.cleanup.interval_minutes", "interval_minutes must be at least 1 when cleanup is enabled")
		}
		if cfg.Cleanup.MaxAgeDays < 1 {
			v.addError("sessions.cleanup.max_age_days", "max_age_days must be at least 1 when cleanup is enabled")
		}
	}
}

func (v *Validator) validateWebUI(cfg *WebUIConfig) {
	if cfg.ToolSessionOTPTTLSeconds < 0 {
		v.addError("webui.tool_session_otp_ttl_seconds", "tool_session_otp_ttl_seconds cannot be negative")
	}
	if cfg.ToolSessionEvents.Enabled && cfg.ToolSessionEvents.RetentionDays < 1 {
		v.addError("webui.tool_session_events.retention_days", "retention_days must be at least 1 when tool session events are enabled")
	}
	if cfg.SkillSnapshots.AutoPrune && cfg.SkillSnapshots.MaxCount < 1 {
		v.addError("webui.skill_snapshots.max_count", "max_count must be at least 1 when skill snapshot auto prune is enabled")
	}
	if cfg.SkillVersions.Enabled && cfg.SkillVersions.MaxCount < 1 {
		v.addError("webui.skill_versions.max_count", "max_count must be at least 1 when skill version history is enabled")
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
