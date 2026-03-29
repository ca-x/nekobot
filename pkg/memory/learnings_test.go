package memory

import (
	"strings"
	"testing"
	"time"

	"nekobot/pkg/config"
)

func TestLearningsManagerAddListAndReadActive(t *testing.T) {
	workspace := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Learnings.Enabled = true
	cfg.Learnings.MaxRawEntries = 500
	cfg.Learnings.CompressedMaxSize = 10000
	cfg.Learnings.HalfLifeDays = 30

	manager, err := NewLearningsManager(cfg)
	if err != nil {
		t.Fatalf("NewLearningsManager failed: %v", err)
	}

	entry := LearningEntry{
		Content:    "Use rg for fast code search.",
		Category:   "workflow",
		Confidence: 0.9,
		Source:     "agent",
		Metadata: map[string]interface{}{
			"channel": "test",
		},
	}

	if err := manager.Add(entry); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entries, err := manager.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID == "" {
		t.Fatal("expected generated ID")
	}
	if entries[0].Timestamp.IsZero() {
		t.Fatal("expected generated timestamp")
	}
	if entries[0].Metadata["channel"] != "test" {
		t.Fatalf("expected metadata to round-trip, got %+v", entries[0].Metadata)
	}

	active := manager.ReadActive()
	if !strings.Contains(active, "Use rg for fast code search.") {
		t.Fatalf("expected active learnings markdown, got %q", active)
	}
}

func TestLearningsManagerReturnsEmptyWhenDisabled(t *testing.T) {
	workspace := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Learnings.Enabled = false

	manager, err := NewLearningsManager(cfg)
	if err != nil {
		t.Fatalf("NewLearningsManager failed: %v", err)
	}

	if err := manager.Add(LearningEntry{Content: "ignored"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entries, err := manager.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries when disabled, got %d", len(entries))
	}
	if got := manager.ReadActive(); got != "" {
		t.Fatalf("expected empty active learnings when disabled, got %q", got)
	}
}

func TestLearningsCompressorPrefersRecentHighConfidenceEntries(t *testing.T) {
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	compressor := NewLearningsCompressor(config.LearningsConfig{
		Enabled:           true,
		CompressedMaxSize: 80,
		HalfLifeDays:      30,
	})

	old := now.AddDate(0, 0, -90)
	recent := now.AddDate(0, 0, -1)
	entries := []LearningEntry{
		{
			ID:         "old",
			Timestamp:  old,
			Content:    "Old low-value note.",
			Category:   "general",
			Confidence: 0.2,
			Source:     "agent",
		},
		{
			ID:         "recent",
			Timestamp:  recent,
			Content:    "Recent important learning.",
			Category:   "important",
			Confidence: 0.95,
			Source:     "agent",
		},
	}

	out := compressor.compressAt(entries, now)
	if !strings.Contains(out, "Recent important learning.") {
		t.Fatalf("expected recent learning to appear, got %q", out)
	}
	if strings.Contains(out, "Old low-value note.") {
		t.Fatalf("expected low-value old learning to be trimmed, got %q", out)
	}
	if len(out) > 80 {
		t.Fatalf("expected output <= 80 chars, got %d", len(out))
	}
}
