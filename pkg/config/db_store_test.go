package config

import (
	"encoding/json"
	"sync"
	"testing"

	"nekobot/pkg/storage/ent"
)

func TestApplyDatabaseOverridesAndSaveSections(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Model = "file-model"
	cfg.Agents.Defaults.MCPServers = []MCPServerConfig{
		{
			Name:      "stdio-a",
			Transport: "stdio",
			Command:   "npx",
			Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			Timeout:   "20s",
		},
	}
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.Token = "token-a"
	cfg.Memory.Enabled = true
	cfg.Memory.Semantic.Enabled = true
	cfg.Memory.Semantic.SearchPolicy = "vector"
	cfg.Memory.Semantic.DefaultTopK = 6
	cfg.Memory.Semantic.MaxTopK = 12
	cfg.Memory.ShortTerm.Enabled = true
	cfg.Memory.ShortTerm.RawHistoryLimit = 111

	if err := ApplyDatabaseOverrides(cfg); err != nil {
		t.Fatalf("ApplyDatabaseOverrides initial failed: %v", err)
	}

	cfg.Agents.Defaults.Model = "changed-in-memory"
	cfg.Agents.Defaults.MCPServers = nil
	cfg.Channels.Telegram.Enabled = false
	cfg.Channels.Telegram.Token = "token-b"
	cfg.Memory.Semantic.SearchPolicy = "hybrid"
	cfg.Memory.ShortTerm.RawHistoryLimit = 5
	if err := ApplyDatabaseOverrides(cfg); err != nil {
		t.Fatalf("ApplyDatabaseOverrides reload failed: %v", err)
	}
	if cfg.Agents.Defaults.Model != "file-model" {
		t.Fatalf("expected model loaded from DB, got %q", cfg.Agents.Defaults.Model)
	}
	if len(cfg.Agents.Defaults.MCPServers) != 1 || cfg.Agents.Defaults.MCPServers[0].Name != "stdio-a" {
		t.Fatalf("expected MCP servers loaded from DB, got %+v", cfg.Agents.Defaults.MCPServers)
	}
	if !cfg.Channels.Telegram.Enabled || cfg.Channels.Telegram.Token != "token-a" {
		t.Fatalf("expected channels loaded from DB, got %+v", cfg.Channels.Telegram)
	}
	if cfg.Memory.Semantic.SearchPolicy != "vector" || cfg.Memory.ShortTerm.RawHistoryLimit != 111 {
		t.Fatalf("expected memory loaded from DB, got %+v", cfg.Memory)
	}

	cfg.Agents.Defaults.Model = "db-model"
	cfg.Agents.Defaults.MCPServers = []MCPServerConfig{
		{
			Name:      "http-b",
			Transport: "http",
			Endpoint:  "https://example.com/mcp",
			Timeout:   "10s",
		},
	}
	if err := SaveDatabaseSections(cfg, "agents"); err != nil {
		t.Fatalf("SaveDatabaseSections failed: %v", err)
	}
	cfg.Agents.Defaults.Model = "stale"
	cfg.Agents.Defaults.MCPServers = nil
	if err := ApplyDatabaseOverrides(cfg); err != nil {
		t.Fatalf("ApplyDatabaseOverrides second reload failed: %v", err)
	}
	if cfg.Agents.Defaults.Model != "db-model" {
		t.Fatalf("expected updated model from DB, got %q", cfg.Agents.Defaults.Model)
	}
	if len(cfg.Agents.Defaults.MCPServers) != 1 || cfg.Agents.Defaults.MCPServers[0].Name != "http-b" {
		t.Fatalf("expected updated MCP servers from DB, got %+v", cfg.Agents.Defaults.MCPServers)
	}

	cfg.Memory.Semantic.SearchPolicy = "hybrid"
	cfg.Memory.ShortTerm.RawHistoryLimit = 222
	if err := SaveDatabaseSections(cfg, "memory"); err != nil {
		t.Fatalf("SaveDatabaseSections memory failed: %v", err)
	}
	cfg.Memory.Semantic.SearchPolicy = "stale"
	cfg.Memory.ShortTerm.RawHistoryLimit = 1
	if err := ApplyDatabaseOverrides(cfg); err != nil {
		t.Fatalf("ApplyDatabaseOverrides memory reload failed: %v", err)
	}
	if cfg.Memory.Semantic.SearchPolicy != "hybrid" || cfg.Memory.ShortTerm.RawHistoryLimit != 222 {
		t.Fatalf("expected updated memory from DB, got %+v", cfg.Memory)
	}
}

func TestSaveDatabaseSectionsUnknownSection(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	if err := SaveDatabaseSections(cfg, "unknown_section"); err == nil {
		t.Fatalf("expected error for unknown section")
	}
}

func TestSaveAdminCredentialMigratesToUserTenantMembership(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	client, err := openRuntimeConfigClient(cfg)
	if err != nil {
		t.Fatalf("open runtime client: %v", err)
	}
	defer client.Close()

	cred := &AdminCredential{
		Username:     "admin",
		Nickname:     "Owner",
		PasswordHash: "$2a$10$examplehash",
		JWTSecret:    "jwt-secret",
	}
	if err := SaveAdminCredential(client, cred); err != nil {
		t.Fatalf("SaveAdminCredential failed: %v", err)
	}

	loaded, err := LoadAdminCredential(client)
	if err != nil {
		t.Fatalf("LoadAdminCredential failed: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected credential")
	}
	if loaded.Username != "admin" || loaded.Nickname != "Owner" {
		t.Fatalf("unexpected loaded credential: %+v", loaded)
	}
	if loaded.JWTSecret != "jwt-secret" {
		t.Fatalf("unexpected jwt secret: %s", loaded.JWTSecret)
	}

	profile, err := BuildAuthProfileByUsername(t.Context(), client, "admin")
	if err != nil {
		t.Fatalf("BuildAuthProfileByUsername failed: %v", err)
	}
	if profile.Role != "owner" {
		t.Fatalf("expected owner role, got %q", profile.Role)
	}
	if profile.TenantSlug != "default" {
		t.Fatalf("expected default tenant slug, got %q", profile.TenantSlug)
	}
}

func TestEnsureRuntimeEntSchemaConcurrentCalls(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	const workerCount = 8
	clients := make([]*ent.Client, 0, workerCount)
	for i := 0; i < workerCount; i++ {
		client, err := OpenRuntimeEntClient(cfg)
		if err != nil {
			t.Fatalf("open runtime client %d: %v", i, err)
		}
		clients = append(clients, client)
	}
	defer func() {
		for _, client := range clients {
			_ = client.Close()
		}
	}()

	errCh := make(chan error, workerCount)
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func(c *ent.Client) {
			defer wg.Done()
			errCh <- EnsureRuntimeEntSchema(c)
		}(client)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("EnsureRuntimeEntSchema concurrent call failed: %v", err)
		}
	}
}

func TestGetJWTSecretWithLegacyPayload(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	client, err := openRuntimeConfigClient(cfg)
	if err != nil {
		t.Fatalf("open runtime client: %v", err)
	}
	defer client.Close()

	legacy := &AdminCredential{
		Username:     "legacy",
		Nickname:     "Legacy",
		PasswordHash: "hash",
		JWTSecret:    "legacy-secret",
	}
	payload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	if err := upsertSectionPayload(t.Context(), client, adminCredSection, payload); err != nil {
		t.Fatalf("upsert legacy payload: %v", err)
	}

	secret, err := GetJWTSecret(client)
	if err != nil {
		t.Fatalf("GetJWTSecret failed: %v", err)
	}
	if secret != "legacy-secret" {
		t.Fatalf("expected legacy-secret, got %q", secret)
	}
}
