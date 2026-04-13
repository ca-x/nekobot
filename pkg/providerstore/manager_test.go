package providerstore

import (
	"context"
	"errors"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func TestManagerCRUDAndConfigSync(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "stale-config-provider",
			ProviderKind: "openai",
		},
	}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	mgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	_ = mgr.Close()

	providers, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected empty providers, got %+v", providers)
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("expected config providers to sync from empty DB, got %+v", cfg.Providers)
	}

	created, err := mgr.Create(ctx, config.ProviderProfile{
		Name:          "openai",
		ProviderKind:  "openai",
		APIKey:        "k2",
		APIBase:       "https://api.openai.com/v1",
		DefaultWeight: 7,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Name != "openai" {
		t.Fatalf("unexpected created provider: %+v", created)
	}
	if created.DefaultWeight != 7 {
		t.Fatalf("expected default weight 7, got %+v", created)
	}
	if !created.Enabled {
		t.Fatalf("expected provider enabled, got %+v", created)
	}
	if len(created.Models) != 0 || created.DefaultModel != "" {
		t.Fatalf("expected connection-only provider shape, got %+v", created)
	}

	if _, err := mgr.Create(ctx, config.ProviderProfile{Name: "openai", ProviderKind: "openai", APIKey: "k3"}); !errors.Is(err, ErrProviderExists) {
		t.Fatalf("expected ErrProviderExists, got: %v", err)
	}

	updated, err := mgr.Update(ctx, "openai", config.ProviderProfile{
		Name:          "openai-main",
		ProviderKind:  "openai",
		APIBase:       "https://proxy.example/v1",
		DefaultWeight: 3,
		Enabled:       false,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "openai-main" {
		t.Fatalf("expected renamed provider, got %+v", updated)
	}
	if updated.APIKey != "k2" {
		t.Fatalf("expected API key to be preserved, got %q", updated.APIKey)
	}
	if updated.DefaultWeight != 3 || updated.Enabled {
		t.Fatalf("expected updated weight/enabled fields, got %+v", updated)
	}
	if len(updated.Models) != 0 || updated.DefaultModel != "" {
		t.Fatalf("expected connection-only provider shape after update, got %+v", updated)
	}

	if err := mgr.Delete(ctx, "openai-main"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if err := mgr.Delete(ctx, "openai-main"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("expected ErrProviderNotFound, got: %v", err)
	}

	providers, err = mgr.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected providers to be empty after delete: %+v", providers)
	}

	if len(cfg.Providers) != 0 {
		t.Fatalf("config providers not synced: %+v", cfg.Providers)
	}
}

func TestManagerPrefersExistingDatabaseProviders(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()
	log := newTestLogger(t)

	cfg1 := config.DefaultConfig()
	cfg1.Storage.DBDir = dbDir
	cfg1.Providers = []config.ProviderProfile{{Name: "anthropic", ProviderKind: "anthropic"}}
	client1 := newTestEntClient(t, cfg1)
	t.Cleanup(func() {
		if err := client1.Close(); err != nil {
			t.Fatalf("close first ent client: %v", err)
		}
	})

	mgr1, err := NewManager(cfg1, log, client1)
	if err != nil {
		t.Fatalf("NewManager first failed: %v", err)
	}
	if _, err := mgr1.Create(ctx, config.ProviderProfile{Name: "anthropic", ProviderKind: "anthropic", APIKey: "anthropic-key"}); err != nil {
		t.Fatalf("create anthropic in first manager failed: %v", err)
	}
	if _, err := mgr1.Create(ctx, config.ProviderProfile{Name: "openai", ProviderKind: "openai", APIKey: "openai-key"}); err != nil {
		t.Fatalf("create in first manager failed: %v", err)
	}
	_ = mgr1.Close()

	cfg2 := config.DefaultConfig()
	cfg2.Storage.DBDir = dbDir
	cfg2.Providers = []config.ProviderProfile{{Name: "gemini", ProviderKind: "gemini"}}
	client2 := newTestEntClient(t, cfg2)
	t.Cleanup(func() {
		if err := client2.Close(); err != nil {
			t.Fatalf("close second ent client: %v", err)
		}
	})

	mgr2, err := NewManager(cfg2, log, client2)
	if err != nil {
		t.Fatalf("NewManager second failed: %v", err)
	}
	t.Cleanup(func() {
		if err := mgr2.Close(); err != nil {
			t.Fatalf("close second manager: %v", err)
		}
	})

	providers, err := mgr2.List(ctx)
	if err != nil {
		t.Fatalf("List second failed: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers from database, got %+v", providers)
	}

	names := []string{providers[0].Name, providers[1].Name}
	if (names[0] != "anthropic" || names[1] != "openai") && (names[0] != "openai" || names[1] != "anthropic") {
		t.Fatalf("unexpected provider names: %v", names)
	}

	if len(cfg2.Providers) != 2 {
		t.Fatalf("expected config to sync database providers, got %+v", cfg2.Providers)
	}
}

func TestManagerRejectsProviderMissingRequiredAPIKey(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.Create(ctx, config.ProviderProfile{
		Name:         "openai-empty",
		ProviderKind: "openai",
		APIBase:      "https://api.openai.com/v1",
		Enabled:      true,
	})
	if err == nil {
		t.Fatalf("expected create to fail for missing required api key")
	}
	if !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("expected ErrInvalidProvider, got %v", err)
	}
	if got := len(cfg.Providers); got != 0 {
		t.Fatalf("expected invalid provider to not sync into config, got %d entries", got)
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
