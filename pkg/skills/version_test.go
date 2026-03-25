package skills

import (
	"path/filepath"
	"testing"

	"nekobot/pkg/logger"
)

func TestVersionManagerPruneAndDeleteAll(t *testing.T) {
	dir := t.TempDir()
	vm := NewVersionManager(newVersionTestLogger(t), dir, true)
	if err := vm.Initialize(); err != nil {
		t.Fatalf("initialize version manager: %v", err)
	}

	skill := &Skill{
		ID:           "demo-skill",
		Name:         "Demo",
		Version:      "0.1.0",
		Instructions: "first",
	}
	if err := vm.TrackChange(skill, "created", "created"); err != nil {
		t.Fatalf("track created: %v", err)
	}
	skill.Instructions = "second"
	if err := vm.TrackChange(skill, "modified", "modified-1"); err != nil {
		t.Fatalf("track modified 1: %v", err)
	}
	skill.Instructions = "third"
	if err := vm.TrackChange(skill, "modified", "modified-2"); err != nil {
		t.Fatalf("track modified 2: %v", err)
	}

	if err := vm.Prune(2); err != nil {
		t.Fatalf("prune version history: %v", err)
	}
	versions, err := vm.ListVersions(skill.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions after prune, got %d", len(versions))
	}

	deleted, err := vm.DeleteAll()
	if err != nil {
		t.Fatalf("delete all histories: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 history file deleted, got %d", deleted)
	}
	if _, err := vm.GetHistory(skill.ID); err == nil {
		t.Fatalf("expected version history to be cleared from memory")
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Fatalf("abs dir: %v", err)
	}
}

func TestVersionManagerDisabledNoops(t *testing.T) {
	vm := NewVersionManager(newVersionTestLogger(t), t.TempDir(), false)
	if err := vm.Initialize(); err != nil {
		t.Fatalf("initialize disabled version manager: %v", err)
	}

	if err := vm.TrackChange(&Skill{ID: "demo", Instructions: "body"}, "created", "created"); err != nil {
		t.Fatalf("track change should noop when disabled: %v", err)
	}
	if versions := vm.DetectChanges(map[string]*Skill{"demo": {ID: "demo", Instructions: "body"}}); len(versions) != 0 {
		t.Fatalf("expected no changes when disabled, got %+v", versions)
	}
}

func newVersionTestLogger(t *testing.T) *logger.Logger {
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
