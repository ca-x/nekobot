package externalagent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRegistryGetReturnsSupportedAdapter(t *testing.T) {
	adapter, ok := NewRegistry().Get("codex")
	if !ok {
		t.Fatal("expected codex adapter")
	}
	if adapter.Tool() != "codex" || adapter.Command() != "codex" {
		t.Fatalf("unexpected adapter contract: tool=%q command=%q", adapter.Tool(), adapter.Command())
	}
}

func TestDetectInstalledFindsExistingConfigDirs(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".codex", ".claude"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	installed, err := DetectInstalled(home)
	if err != nil {
		t.Fatalf("DetectInstalled failed: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("expected 2 installed agents, got %+v", installed)
	}
}

func TestInstallHintReturnsExpectedCommand(t *testing.T) {
	cmd, err := InstallHint("claude", "linux")
	if err != nil {
		t.Fatalf("InstallHint failed: %v", err)
	}
	want := []string{"npm", "install", "-g", "@anthropic-ai/claude-code"}
	if !reflect.DeepEqual(cmd, want) {
		t.Fatalf("unexpected install hint: got=%v want=%v", cmd, want)
	}
}
