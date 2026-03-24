package skills

import (
	"context"
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
