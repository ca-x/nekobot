package config

import (
	"os"
	"path/filepath"
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
