package gateway

import (
	"path/filepath"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestControllerReloadConfigAppliesRuntimeOverrides(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	seed := config.DefaultConfig()
	seed.Storage.DBDir = filepath.Dir(cfgPath)
	seed.Agents.Defaults.Workspace = filepath.Join(filepath.Dir(cfgPath), "workspace")
	seed.Gateway.Host = "0.0.0.0"
	seed.Gateway.Port = 18790
	seed.Logger.Level = "info"
	seed.Logger.OutputPath = ""
	seed.WebUI.Enabled = true
	seed.WebUI.Port = 18791
	seed.WebUI.PublicBaseURL = ""
	seed.WebUI.ToolSessionOTPTTLSeconds = 300
	if err := config.SaveToFile(seed, cfgPath); err != nil {
		t.Fatalf("save seed config: %v", err)
	}

	persisted := config.DefaultConfig()
	persisted.Storage.DBDir = seed.Storage.DBDir
	persisted.Agents.Defaults.Workspace = seed.Agents.Defaults.Workspace
	persisted.Gateway.Host = "127.0.0.1"
	persisted.Gateway.Port = 19090
	persisted.Logger.Level = "debug"
	persisted.Logger.OutputPath = filepath.Join(t.TempDir(), "reload.log")
	persisted.WebUI.Enabled = false
	persisted.WebUI.Port = 19091
	persisted.WebUI.PublicBaseURL = "https://reload.example.com"
	persisted.WebUI.ToolSessionOTPTTLSeconds = 123
	if err := config.SaveDatabaseSections(persisted, "gateway", "logger", "webui"); err != nil {
		t.Fatalf("save runtime sections: %v", err)
	}

	live := config.DefaultConfig()
	live.Storage.DBDir = seed.Storage.DBDir
	live.Agents.Defaults.Workspace = seed.Agents.Defaults.Workspace
	log := newGatewayTestLogger(t)
	ctrl := NewController(live, config.NewLoader(), log)
	t.Setenv(config.ConfigPathEnv, cfgPath)

	if err := ctrl.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}

	if live.Gateway.Host != "127.0.0.1" || live.Gateway.Port != 19090 {
		t.Fatalf("expected gateway override applied, got %+v", live.Gateway)
	}
	if live.Logger.Level != "debug" || live.Logger.OutputPath != persisted.Logger.OutputPath {
		t.Fatalf("expected logger override applied, got %+v", live.Logger)
	}
	if live.WebUI.Enabled || live.WebUI.Port != 19091 || live.WebUI.PublicBaseURL != "https://reload.example.com" || live.WebUI.ToolSessionOTPTTLSeconds != 123 {
		t.Fatalf("expected webui override applied, got %+v", live.WebUI)
	}
}

func newGatewayTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}
