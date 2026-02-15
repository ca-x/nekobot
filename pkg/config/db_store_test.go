package config

import "testing"

func TestApplyDatabaseOverridesAndSaveSections(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Model = "file-model"
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.Token = "token-a"

	if err := ApplyDatabaseOverrides(cfg); err != nil {
		t.Fatalf("ApplyDatabaseOverrides initial failed: %v", err)
	}

	cfg.Agents.Defaults.Model = "changed-in-memory"
	cfg.Channels.Telegram.Enabled = false
	cfg.Channels.Telegram.Token = "token-b"
	if err := ApplyDatabaseOverrides(cfg); err != nil {
		t.Fatalf("ApplyDatabaseOverrides reload failed: %v", err)
	}
	if cfg.Agents.Defaults.Model != "file-model" {
		t.Fatalf("expected model loaded from DB, got %q", cfg.Agents.Defaults.Model)
	}
	if !cfg.Channels.Telegram.Enabled || cfg.Channels.Telegram.Token != "token-a" {
		t.Fatalf("expected channels loaded from DB, got %+v", cfg.Channels.Telegram)
	}

	cfg.Agents.Defaults.Model = "db-model"
	if err := SaveDatabaseSections(cfg, "agents"); err != nil {
		t.Fatalf("SaveDatabaseSections failed: %v", err)
	}
	cfg.Agents.Defaults.Model = "stale"
	if err := ApplyDatabaseOverrides(cfg); err != nil {
		t.Fatalf("ApplyDatabaseOverrides second reload failed: %v", err)
	}
	if cfg.Agents.Defaults.Model != "db-model" {
		t.Fatalf("expected updated model from DB, got %q", cfg.Agents.Defaults.Model)
	}
}

func TestSaveDatabaseSectionsUnknownSection(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	if err := SaveDatabaseSections(cfg, "unknown_section"); err == nil {
		t.Fatalf("expected error for unknown section")
	}
}
