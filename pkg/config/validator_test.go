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
