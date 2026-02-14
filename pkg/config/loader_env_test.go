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
	seed.Gateway.Port = 29999

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
