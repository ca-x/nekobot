package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspacePath_ExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = "~/.nekobot/workspace"

	got := cfg.WorkspacePath()
	want := filepath.Join(home, ".nekobot", "workspace")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDefaultConfig_UsesNekobotWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := DefaultConfig()

	got := cfg.Agents.Defaults.Workspace
	want := filepath.Join(home, ".nekobot", "workspace")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestApplyFromCopiesRuntimeReloadableSections(t *testing.T) {
	target := DefaultConfig()
	source := DefaultConfig()

	source.Gateway.Host = "127.0.0.1"
	source.Gateway.Port = 19090
	source.Gateway.MaxConnections = 42
	source.Gateway.AllowedOrigins = []string{"https://allowed.example.com"}
	source.Logger.Level = "debug"
	source.Logger.OutputPath = filepath.Join(t.TempDir(), "nekobot.log")
	source.WebUI.Enabled = false
	source.WebUI.Port = 19091
	source.WebUI.PublicBaseURL = "https://nekobot.example.com"
	source.WebUI.ToolSessionOTPTTLSeconds = 321

	target.ApplyFrom(source)

	if target.Gateway.Host != source.Gateway.Host ||
		target.Gateway.Port != source.Gateway.Port ||
		target.Gateway.MaxConnections != source.Gateway.MaxConnections ||
		len(target.Gateway.AllowedOrigins) != len(source.Gateway.AllowedOrigins) ||
		target.Gateway.AllowedOrigins[0] != source.Gateway.AllowedOrigins[0] {
		t.Fatalf("expected gateway copied, got %+v want %+v", target.Gateway, source.Gateway)
	}
	if target.Logger != source.Logger {
		t.Fatalf("expected logger copied, got %+v want %+v", target.Logger, source.Logger)
	}
	if target.WebUI != source.WebUI {
		t.Fatalf("expected webui copied, got %+v want %+v", target.WebUI, source.WebUI)
	}
}

func TestValidatorRejectsNegativeGatewayMaxConnections(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Gateway.MaxConnections = -1

	err := NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for negative max_connections")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "gateway.max_connections") {
		t.Fatalf("expected gateway.max_connections validation error, got %v", err)
	}
}
