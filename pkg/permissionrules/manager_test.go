package permissionrules

import (
	"context"
	"errors"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func TestManagerCRUD(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := NewManager(cfg, newTestLogger(t), client)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	created, err := mgr.Create(ctx, Rule{
		ToolName:    "exec",
		Priority:    100,
		Action:      ActionAllow,
		Description: "allow exec globally",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created rule id")
	}

	if _, err := mgr.Create(ctx, Rule{
		ToolName:  "spawn",
		SessionID: "sess-1",
		Priority:  50,
		Action:    ActionAsk,
		Enabled:   true,
	}); err != nil {
		t.Fatalf("Create scoped rule failed: %v", err)
	}

	listed, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(listed))
	}
	if listed[0].ToolName != "exec" {
		t.Fatalf("expected highest priority rule first, got %+v", listed[0])
	}

	updated, err := mgr.Update(ctx, created.ID, Rule{
		ToolName:    "exec",
		Priority:    120,
		Action:      ActionDeny,
		Description: "deny exec globally",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Action != ActionDeny || updated.Priority != 120 {
		t.Fatalf("unexpected updated rule: %+v", updated)
	}

	got, err := mgr.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Description != "deny exec globally" {
		t.Fatalf("unexpected fetched rule: %+v", got)
	}

	if err := mgr.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	listed, err = mgr.List(ctx)
	if err != nil {
		t.Fatalf("List after delete failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 rule after delete, got %d", len(listed))
	}
}

func TestManagerRejectsInvalidRule(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := NewManager(cfg, newTestLogger(t), client)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.Create(ctx, Rule{
		ToolName: "",
		Action:   ActionAllow,
		Enabled:  true,
	})
	if !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("expected ErrInvalidRule for empty tool name, got %v", err)
	}

	_, err = mgr.Create(ctx, Rule{
		ToolName: "exec",
		Action:   Action("invalid"),
		Enabled:  true,
	})
	if !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("expected ErrInvalidRule for invalid action, got %v", err)
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

func newTestEntClient(t *testing.T, cfg *config.Config) *ent.Client {
	t.Helper()
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		t.Fatalf("ensure runtime schema: %v", err)
	}
	return client
}
