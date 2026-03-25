package toolsessions

import (
	"context"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func TestAppendEventDisabledSkipsPersistence(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.WebUI.ToolSessionEvents.Enabled = false

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer client.Close()

	mgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx := context.Background()
	session, err := mgr.CreateSession(ctx, CreateSessionInput{
		Owner:   "tester",
		Source:  SourceWebUI,
		Tool:    "codex",
		Command: "codex",
		State:   StateRunning,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := mgr.AppendEvent(ctx, session.ID, "manual", map[string]interface{}{"ok": true}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	events, err := mgr.ListEvents(ctx, session.ID, 20)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no persisted events when disabled, got %d", len(events))
	}
}

func TestCleanupEventsSkipsWhenRetentionDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.WebUI.ToolSessionEvents.Enabled = true
	cfg.WebUI.ToolSessionEvents.RetentionDays = 0

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer client.Close()

	mgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx := context.Background()
	session, err := mgr.CreateSession(ctx, CreateSessionInput{
		Owner:   "tester",
		Source:  SourceWebUI,
		Tool:    "codex",
		Command: "codex",
		State:   StateRunning,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := mgr.AppendEvent(ctx, session.ID, "kept", nil); err != nil {
		t.Fatalf("append event: %v", err)
	}

	events, err := mgr.ListEvents(ctx, session.ID, 20)
	if err != nil || len(events) == 0 {
		t.Fatalf("expected persisted events before cleanup, got len=%d err=%v", len(events), err)
	}

	deleted, err := mgr.CleanupEvents(ctx)
	if err != nil {
		t.Fatalf("cleanup events: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected no deleted events when retention disabled, got %d", deleted)
	}

	remaining, err := mgr.ListEvents(ctx, session.ID, 20)
	if err != nil {
		t.Fatalf("list events after cleanup: %v", err)
	}
	if len(remaining) != len(events) {
		t.Fatalf("expected all events retained, before=%d after=%d", len(events), len(remaining))
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
