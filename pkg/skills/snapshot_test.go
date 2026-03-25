package skills

import (
	"testing"

	"nekobot/pkg/logger"
)

func TestSnapshotManagerPruneOldestKeepsNewestSnapshots(t *testing.T) {
	log := newTestLogger(t)
	dir := t.TempDir()
	manager := NewSnapshotManager(log, dir)

	for i := 0; i < 3; i++ {
		_, err := manager.Create(map[string]*Skill{
			"skill": {
				ID:           "skill",
				Name:         "Skill",
				Version:      "1.0.0",
				Enabled:      true,
				Instructions: "test",
			},
		}, map[string]string{"index": string(rune('0' + i))})
		if err != nil {
			t.Fatalf("create snapshot %d: %v", i, err)
		}
	}

	removed, err := manager.PruneOldest(2)
	if err != nil {
		t.Fatalf("prune oldest: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected one removed snapshot, got %d", removed)
	}

	items, err := manager.List()
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 snapshots after prune, got %d", len(items))
	}
}

func newTestLogger(t *testing.T) *logger.Logger {
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
