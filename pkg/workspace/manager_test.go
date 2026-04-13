package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"nekobot/pkg/logger"
)

func TestManagerEnsureCreatesWikiScaffold(t *testing.T) {
	cfg := logger.DefaultConfig()
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	workspaceDir := t.TempDir()
	manager := NewManager(workspaceDir, log)
	if err := manager.Ensure(); err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}

	expected := []string{
		filepath.Join(workspaceDir, "wiki", "SCHEMA.md"),
		filepath.Join(workspaceDir, "wiki", "index.md"),
		filepath.Join(workspaceDir, "wiki", "log.md"),
		filepath.Join(workspaceDir, "wiki", "raw"),
		filepath.Join(workspaceDir, "wiki", "entities"),
		filepath.Join(workspaceDir, "wiki", "concepts"),
	}
	for _, path := range expected {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}
