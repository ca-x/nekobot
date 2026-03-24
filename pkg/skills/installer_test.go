package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nekobot/pkg/logger"
)

func TestParseInstallMapSupportsGoclawInstallMetadata(t *testing.T) {
	spec := parseInstallMap(map[string]interface{}{
		"kind":     "custom",
		"command":  "./scripts/setup.sh",
		"postHook": "./scripts/verify.sh",
	})

	if spec.Method != "command" {
		t.Fatalf("expected command method, got %q", spec.Method)
	}
	if spec.Package != "./scripts/setup.sh" {
		t.Fatalf("expected command package to use command string, got %q", spec.Package)
	}
	if spec.PostHook != "./scripts/verify.sh" {
		t.Fatalf("expected post hook to be parsed, got %q", spec.PostHook)
	}
}

func TestNormalizeInstallMethodSupportsCompatibilityAliases(t *testing.T) {
	cases := map[string]string{
		"node":   "npm",
		"python": "pip",
		"custom": "command",
		"apt":    "apt",
	}

	for input, want := range cases {
		if got := normalizeInstallMethod(input); got != want {
			t.Fatalf("normalizeInstallMethod(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestInstallerCanInstallSupportsCommand(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	installer := NewInstaller(log)
	if !installer.CanInstall("command") {
		t.Fatalf("expected command installs to be available when shell exists")
	}
}

func TestInstallerInstallSupportsCommand(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	installer := NewInstaller(log)
	result := installer.Install(context.Background(), InstallSpec{
		Method:  "command",
		Package: "printf skill-command-ok",
	})

	if !result.Success {
		t.Fatalf("expected command install to succeed, got error: %v", result.Error)
	}
	if result.Output != "skill-command-ok" {
		t.Fatalf("unexpected command output: %q", result.Output)
	}
}

func TestInstallerWithProxySetsProxyEnvironment(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	installer := NewInstallerWithProxy(log, "http://127.0.0.1:8080")
	result := installer.Install(context.Background(), InstallSpec{
		Method:  "command",
		Package: "printf %s \"$HTTP_PROXY\"",
	})

	if !result.Success {
		t.Fatalf("expected proxy-aware command install to succeed, got error: %v", result.Error)
	}
	if result.Output != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected proxy output: %q", result.Output)
	}
}

func TestSkillsProxyEnvLeavesBaseEnvUntouchedWithoutProxy(t *testing.T) {
	base := []string{"A=1"}
	got := skillsProxyEnv(base, "")
	if len(got) != 1 || got[0] != "A=1" {
		t.Fatalf("unexpected env without proxy: %#v", got)
	}
}

func TestRegistryClientInstallLocalCopiesSkillFile(t *testing.T) {
	client, err := NewRegistryClient("")
	if err != nil {
		t.Fatalf("new registry client: %v", err)
	}

	root := t.TempDir()
	source := filepath.Join(root, "demo-skill.md")
	targetDir := filepath.Join(root, "installed")
	if err := os.WriteFile(source, []byte("demo"), 0o644); err != nil {
		t.Fatalf("write source skill: %v", err)
	}

	targetPath, err := client.Install(context.Background(), source, targetDir)
	if err != nil {
		t.Fatalf("install local skill: %v", err)
	}
	if targetPath != filepath.Join(targetDir, "demo-skill.md") {
		t.Fatalf("unexpected target path: %s", targetPath)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(data) != "demo" {
		t.Fatalf("unexpected installed content: %q", string(data))
	}
}
