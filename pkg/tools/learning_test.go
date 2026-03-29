package tools

import (
	"context"
	"strings"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/memory"
)

func TestLearningToolAddsLearning(t *testing.T) {
	workspace := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Learnings.Enabled = true

	manager, err := memory.NewLearningsManager(cfg)
	if err != nil {
		t.Fatalf("NewLearningsManager failed: %v", err)
	}

	tool := NewLearningTool(manager)
	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"content":    "Prefer deterministic tests over truthy assertions.",
		"category":   "testing",
		"confidence": 0.8,
		"source":     "agent",
		"metadata": map[string]interface{}{
			"session": "abc",
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "Successfully recorded learning") {
		t.Fatalf("expected success output, got %q", out)
	}

	entries, err := manager.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 learning entry, got %d", len(entries))
	}
	if entries[0].Category != "testing" {
		t.Fatalf("expected category to round-trip, got %+v", entries[0])
	}
}

func TestLearningToolRejectsMissingContent(t *testing.T) {
	tool := NewLearningTool(nil)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{}); err == nil {
		t.Fatal("expected error for missing manager/content")
	}
}
