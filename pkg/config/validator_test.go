package config

import (
	"strings"
	"testing"
)

func TestValidateConfigRejectsInvalidOrchestrator(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Orchestrator = "unknown"

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for orchestrator")
	}

	validationErrors, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	for _, validationErr := range validationErrors {
		if validationErr.Field == "agents.defaults.orchestrator" {
			return
		}
	}
	t.Fatalf("expected orchestrator validation error, got %v", err)
}

func TestValidateConfigRejectsInvalidMemoryBackend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Enabled = true
	cfg.Memory.Backend = "unknown"

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for memory backend")
	}

	validationErrors, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	for _, validationErr := range validationErrors {
		if validationErr.Field == "memory.backend" {
			return
		}
	}
	t.Fatalf("expected memory.backend validation error, got %v", err)
}

func TestValidateConfigRejectsInvalidMemoryContextConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Enabled = true
	cfg.Memory.Context.Enabled = true
	cfg.Memory.Context.RecentDailyNoteDays = -1
	cfg.Memory.Context.MaxChars = 0

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for memory context config")
	}

	validationErrors, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	foundDays := false
	foundChars := false
	for _, validationErr := range validationErrors {
		if validationErr.Field == "memory.context.recent_daily_note_days" {
			foundDays = true
		}
		if validationErr.Field == "memory.context.max_chars" {
			foundChars = true
		}
	}
	if !foundDays || !foundChars {
		t.Fatalf("expected memory context validation errors, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidMCPServerConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.MCPServers = []MCPServerConfig{
		{
			Name:      "",
			Transport: "http",
			Endpoint:  "",
			Timeout:   "not-a-duration",
		},
		{
			Name:      "stdio-server",
			Transport: "stdio",
			Command:   "",
		},
		{
			Name:      "bad-transport",
			Transport: "udp",
		},
	}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for mcp server config")
	}

	validationErrors, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	requiredFields := map[string]bool{
		"agents.defaults.mcp_servers[0].name":      false,
		"agents.defaults.mcp_servers[0].endpoint":  false,
		"agents.defaults.mcp_servers[0].timeout":   false,
		"agents.defaults.mcp_servers[1].command":   false,
		"agents.defaults.mcp_servers[2].transport": false,
	}

	for _, validationErr := range validationErrors {
		if _, ok := requiredFields[validationErr.Field]; ok {
			requiredFields[validationErr.Field] = true
		}
	}

	for field, found := range requiredFields {
		if !found {
			t.Fatalf("expected validation error for %s, got %v", field, err)
		}
	}
}

func TestValidateConfigRejectsInvalidProviderGroupConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.ProviderGroups = []ProviderGroupConfig{
		{
			Name:     "",
			Strategy: "weighted",
			Members:  []string{"openai"},
		},
	}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for provider group config")
	}

	requiredFields := []string{
		"agents.defaults.provider_groups[0].name",
		"agents.defaults.provider_groups[0].strategy",
		"agents.defaults.provider_groups[0].members",
	}
	for _, field := range requiredFields {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("expected %s validation error, got %v", field, err)
		}
	}
}

func TestValidateConfigAcceptsValidProviderGroupConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.ProviderGroups = []ProviderGroupConfig{
		{
			Name:     "openai-pool",
			Strategy: "least_used",
			Members:  []string{"openai-a", "openai-b"},
		},
	}

	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected valid provider group config, got %v", err)
	}
}

func TestValidateConfigAcceptsValidMCPServerConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.MCPServers = []MCPServerConfig{
		{
			Name:      "local-stdio",
			Transport: "stdio",
			Command:   "npx",
			Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			Timeout:   "30s",
		},
		{
			Name:      "remote-http",
			Transport: "http",
			Endpoint:  "https://example.com/mcp",
			Timeout:   "5s",
		},
		{
			Name:      "remote-sse",
			Transport: "sse",
			Endpoint:  "https://example.com/sse",
			Timeout:   "5s",
		},
	}

	err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("expected valid mcp config, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidMCPEndpointURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.MCPServers = []MCPServerConfig{
		{
			Name:      "remote-http",
			Transport: "http",
			Endpoint:  "://bad-url",
		},
	}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for invalid endpoint")
	}
	if !strings.Contains(err.Error(), "agents.defaults.mcp_servers[0].endpoint") {
		t.Fatalf("expected endpoint validation error, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidWechatPollInterval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Channels.WeChat.Enabled = true
	cfg.Channels.WeChat.PollIntervalSeconds = 0

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for wechat poll interval")
	}
	if !strings.Contains(err.Error(), "channels.wechat.poll_interval_seconds") {
		t.Fatalf("expected wechat poll interval validation error, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidGotifyConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Channels.Gotify.Enabled = true
	cfg.Channels.Gotify.ServerURL = ""
	cfg.Channels.Gotify.AppToken = ""
	cfg.Channels.Gotify.Priority = 11

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for gotify config")
	}

	requiredFields := []string{
		"channels.gotify.server_url",
		"channels.gotify.app_token",
		"channels.gotify.priority",
	}
	for _, field := range requiredFields {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("expected %s validation error, got %v", field, err)
		}
	}
}

func TestValidateConfigRejectsSessionPersistenceWithoutSources(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Sessions.Enabled = true
	cfg.Sessions.Sources = SessionSourcesConfig{}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for session sources")
	}
	if !strings.Contains(err.Error(), "sessions.sources") {
		t.Fatalf("expected sessions.sources validation error, got %v", err)
	}
}

func TestValidateConfigRejectsSessionPersistenceWithoutContent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Sessions.Enabled = true
	cfg.Sessions.Content = SessionContentConfig{}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for session content")
	}
	if !strings.Contains(err.Error(), "sessions.content") {
		t.Fatalf("expected sessions.content validation error, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidSessionCleanupConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Sessions.Enabled = true
	cfg.Sessions.Cleanup.Enabled = true
	cfg.Sessions.Cleanup.IntervalMinutes = 0
	cfg.Sessions.Cleanup.MaxAgeDays = 0

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for session cleanup")
	}
	if !strings.Contains(err.Error(), "sessions.cleanup.interval_minutes") {
		t.Fatalf("expected sessions.cleanup.interval_minutes validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "sessions.cleanup.max_age_days") {
		t.Fatalf("expected sessions.cleanup.max_age_days validation error, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidWebUIRecordRetentionConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.WebUI.ToolSessionEvents.Enabled = true
	cfg.WebUI.ToolSessionEvents.RetentionDays = 0
	cfg.WebUI.SkillSnapshots.AutoPrune = true
	cfg.WebUI.SkillSnapshots.MaxCount = 0
	cfg.WebUI.SkillVersions.Enabled = true
	cfg.WebUI.SkillVersions.MaxCount = 0

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for webui retention config")
	}
	if !strings.Contains(err.Error(), "webui.tool_session_events.retention_days") {
		t.Fatalf("expected tool session event retention validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "webui.skill_snapshots.max_count") {
		t.Fatalf("expected skill snapshot max_count validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "webui.skill_versions.max_count") {
		t.Fatalf("expected skill version max_count validation error, got %v", err)
	}
}
