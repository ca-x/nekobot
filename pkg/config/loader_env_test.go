package config

import (
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
