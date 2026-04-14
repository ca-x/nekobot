package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExternalAgentsCommandsRegistered(t *testing.T) {
	for _, path := range [][]string{{"external-agents", "list"}, {"external-agents", "detect"}, {"external-agents", "install-hint", "claude"}} {
		cmd, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("find command %v: %v", path, err)
		}
		if cmd == nil {
			t.Fatalf("expected command for %v", path)
		}
	}
}

func TestRunExternalAgentsInstallHint(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	runExternalAgentsInstallHint(cmd, []string{"claude"})
	out := stdout.String()
	if !strings.Contains(out, "@anthropic-ai/claude-code") {
		t.Fatalf("expected claude install hint, got:\n%s", out)
	}
}

func TestRunExternalAgentsList(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	runExternalAgentsList(cmd, nil)
	out := stdout.String()
	for _, fragment := range []string{"Supported external agents", "codex", "claude", "opencode", "aider"} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected output to contain %q, got:\n%s", fragment, out)
		}
	}
}

func TestRunExternalAgentsDetect(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".codex", ".claude"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	t.Setenv("HOME", home)
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	runExternalAgentsDetect(cmd, nil)
	out := stdout.String()
	for _, fragment := range []string{"Detected external agents", "codex", ".codex", "claude", ".claude"} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected output to contain %q, got:\n%s", fragment, out)
		}
	}
}
