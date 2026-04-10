package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_UsesConfigPathEnvWhenPathEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "from-env.json")

	seed := DefaultConfig()
	seed.Storage.DBDir = filepath.Join(tmpDir, "db")
	seed.Gateway.Host = "127.0.0.1"
	seed.Gateway.Port = 29999
	seed.WebUI.Enabled = true
	seed.WebUI.Port = 30000
	seed.WebUI.PublicBaseURL = "http://127.0.0.1:30000"

	loader := NewLoader()
	if err := loader.Save(cfgPath, seed); err != nil {
		t.Fatalf("save config: %v", err)
	}

	t.Setenv(ConfigPathEnv, cfgPath)

	got, err := NewLoader().Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got.Gateway.Port != 29999 {
		t.Fatalf("expected gateway port 29999, got %d", got.Gateway.Port)
	}
	if got.Gateway.Host != "127.0.0.1" {
		t.Fatalf("expected gateway host 127.0.0.1, got %q", got.Gateway.Host)
	}
	if got.Storage.DBDir != seed.Storage.DBDir {
		t.Fatalf("expected db dir %q, got %q", seed.Storage.DBDir, got.Storage.DBDir)
	}
	if got.WebUI.Port != 30000 || got.WebUI.PublicBaseURL != "http://127.0.0.1:30000" {
		t.Fatalf("expected webui settings preserved, got %+v", got.WebUI)
	}
}

func TestLoad_MigratesLegacySearchAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "legacy-search-key.json")

	content := `{
  "tools": {
    "web": {
      "search": {
        "api_key": "legacy-key",
        "max_results": 5,
        "duckduckgo_enabled": true,
        "duckduckgo_max_results": 5
      }
    }
  }
}`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := NewLoader().Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got.Tools.Web.Search.GetBraveAPIKey() != "legacy-key" {
		t.Fatalf("expected migrated brave key, got %q", got.Tools.Web.Search.GetBraveAPIKey())
	}
	if got.Tools.Web.Search.BraveAPIKey != "legacy-key" {
		t.Fatalf("expected brave_api_key field to be normalized, got %q", got.Tools.Web.Search.BraveAPIKey)
	}
}

func TestLoad_AutoCreatesConfigAndDatabaseForExplicitPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "custom", "config.json")
	dbDir := filepath.Join(tmpDir, "db")
	t.Setenv(DBDirEnv, dbDir)

	got, err := NewLoader().Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}

	wantWorkspace := filepath.Join(filepath.Dir(cfgPath), "workspace")
	if got.Agents.Defaults.Workspace != wantWorkspace {
		t.Fatalf("expected workspace %q, got %q", wantWorkspace, got.Agents.Defaults.Workspace)
	}

	dbPath := filepath.Join(dbDir, RuntimeDBName)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected runtime database file to be created: %v", err)
	}
}

func TestInitDefaultConfig_UsesConfigEnvAndSyncsWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tenant", "config.json")
	dbDir := filepath.Join(tmpDir, "db")
	t.Setenv(ConfigPathEnv, cfgPath)
	t.Setenv(DBDirEnv, dbDir)

	path, created, err := InitDefaultConfig()
	if err != nil {
		t.Fatalf("InitDefaultConfig failed: %v", err)
	}
	if !created {
		t.Fatalf("expected config to be created")
	}
	absCfg, _ := filepath.Abs(cfgPath)
	if path != absCfg {
		t.Fatalf("expected config path %q, got %q", absCfg, path)
	}

	loader := NewLoader()
	cfg, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	wantWorkspace := filepath.Join(filepath.Dir(absCfg), "workspace")
	if cfg.Agents.Defaults.Workspace != wantWorkspace {
		t.Fatalf("expected workspace %q, got %q", wantWorkspace, cfg.Agents.Defaults.Workspace)
	}

	cfg.Agents.Defaults.Workspace = "/tmp/old"
	if err := SaveToFile(cfg, path); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if _, created, err := InitDefaultConfig(); err != nil {
		t.Fatalf("InitDefaultConfig second failed: %v", err)
	} else if created {
		t.Fatalf("expected second InitDefaultConfig call to not create file")
	}

	cfg2, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if cfg2.Agents.Defaults.Workspace != wantWorkspace {
		t.Fatalf("expected synced workspace %q, got %q", wantWorkspace, cfg2.Agents.Defaults.Workspace)
	}

	dbPath := filepath.Join(dbDir, RuntimeDBName)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected runtime database file to exist: %v", err)
	}
}

func TestLoadFromFilePreservesCustomDBDirForExplicitConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tenant", "config.json")

	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = filepath.Join(tmpDir, "tenant", "workspace")
	cfg.Storage.DBDir = filepath.Join(tmpDir, "custom-db")
	if err := SaveToFile(cfg, cfgPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := NewLoader().LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.Storage.DBDir != cfg.Storage.DBDir {
		t.Fatalf("expected db dir %q, got %q", cfg.Storage.DBDir, loaded.Storage.DBDir)
	}
}
