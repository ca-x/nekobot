package modelstore

import (
	"context"
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

	created, err := mgr.Create(ctx, ModelCatalog{
		ModelID:       "gpt-4.1",
		DisplayName:   "GPT-4.1",
		Developer:     "OpenAI",
		Family:        "gpt-4",
		Type:          "chat",
		Capabilities:  []string{"text", "tools"},
		CatalogSource: "builtin",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ModelID != "gpt-4.1" {
		t.Fatalf("unexpected created model: %+v", created)
	}

	listed, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 model, got %d", len(listed))
	}
	if listed[0].Capabilities[0] != "text" {
		t.Fatalf("expected capabilities to round-trip, got %+v", listed[0])
	}

	updated, err := mgr.Update(ctx, "gpt-4.1", ModelCatalog{
		ModelID:       "gpt-4.1",
		DisplayName:   "GPT-4.1 Updated",
		Developer:     "OpenAI",
		Family:        "gpt-4",
		Type:          "chat",
		Capabilities:  []string{"text"},
		CatalogSource: "provider_discovery",
		Enabled:       false,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.DisplayName != "GPT-4.1 Updated" || updated.Enabled {
		t.Fatalf("unexpected updated model: %+v", updated)
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
