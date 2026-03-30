package runtimeagents

import (
	"context"
	"errors"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestManagerCRUD(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManager(t)

	created, err := mgr.Create(ctx, AgentRuntime{
		Name:        "support-main",
		DisplayName: "Support Main",
		Provider:    "openai",
		Model:       "gpt-5",
		Skills:      []string{"triage", "reply", "reply"},
		Tools:       []string{"bash", "search"},
		Policy: map[string]interface{}{
			"memory_scope": "private",
		},
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected runtime id")
	}
	if len(created.Skills) != 2 {
		t.Fatalf("expected deduped skills, got %+v", created.Skills)
	}

	if _, err := mgr.Create(ctx, AgentRuntime{Name: "support-main"}); !errors.Is(err, ErrRuntimeExists) {
		t.Fatalf("expected ErrRuntimeExists, got %v", err)
	}

	listed, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("list runtimes: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 runtime, got %d", len(listed))
	}

	updated, err := mgr.Update(ctx, created.ID, AgentRuntime{
		Name:        "support-main-v2",
		DisplayName: "Support Main V2",
		Enabled:     true,
		Provider:    "anthropic",
		Model:       "claude-sonnet-4",
		PromptID:    "prompt-1",
		Skills:      []string{"triage"},
		Tools:       []string{"bash"},
		Policy: map[string]interface{}{
			"memory_scope": "shared_and_private",
		},
	})
	if err != nil {
		t.Fatalf("update runtime: %v", err)
	}
	if updated.Name != "support-main-v2" || updated.Provider != "anthropic" {
		t.Fatalf("unexpected updated runtime: %+v", updated)
	}

	got, err := mgr.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get runtime: %v", err)
	}
	if got.Name != "support-main-v2" {
		t.Fatalf("unexpected get result: %+v", got)
	}

	if err := mgr.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete runtime: %v", err)
	}
	if err := mgr.Delete(ctx, created.ID); !errors.Is(err, ErrRuntimeNotFound) {
		t.Fatalf("expected ErrRuntimeNotFound, got %v", err)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		t.Fatalf("ensure runtime schema: %v", err)
	}

	mgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	return mgr
}
