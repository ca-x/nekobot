package providerstore

import (
	"context"
	"errors"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestManagerCRUDAndConfigSync(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "anthropic",
			ProviderKind: "anthropic",
			APIKey:       "k1",
			Models:       []string{"claude-sonnet-4"},
			Timeout:      30,
		},
	}

	log := newTestLogger(t)
	mgr, err := NewManager(cfg, log)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close()

	providers, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(providers) != 1 || providers[0].Name != "anthropic" {
		t.Fatalf("unexpected bootstrap providers: %+v", providers)
	}

	created, err := mgr.Create(ctx, config.ProviderProfile{
		Name:         "openai",
		ProviderKind: "openai",
		APIKey:       "k2",
		APIBase:      "https://api.openai.com/v1",
		Models:       []string{"gpt-4o"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Name != "openai" {
		t.Fatalf("unexpected created provider: %+v", created)
	}

	if _, err := mgr.Create(ctx, config.ProviderProfile{Name: "openai", ProviderKind: "openai"}); !errors.Is(err, ErrProviderExists) {
		t.Fatalf("expected ErrProviderExists, got: %v", err)
	}

	updated, err := mgr.Update(ctx, "openai", config.ProviderProfile{
		Name:         "openai-main",
		ProviderKind: "openai",
		APIBase:      "https://proxy.example/v1",
		Models:       []string{"gpt-4.1"},
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
	if len(providers) != 1 || providers[0].Name != "anthropic" {
		t.Fatalf("unexpected providers after delete: %+v", providers)
	}

	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "anthropic" {
		t.Fatalf("config providers not synced: %+v", cfg.Providers)
	}
}

func TestManagerPrefersExistingDatabaseProviders(t *testing.T) {
	ctx := context.Background()
	workspace := t.TempDir()
	log := newTestLogger(t)

	cfg1 := config.DefaultConfig()
	cfg1.Agents.Defaults.Workspace = workspace
	cfg1.Providers = []config.ProviderProfile{{Name: "anthropic", ProviderKind: "anthropic"}}

	mgr1, err := NewManager(cfg1, log)
	if err != nil {
		t.Fatalf("NewManager first failed: %v", err)
	}
	if _, err := mgr1.Create(ctx, config.ProviderProfile{Name: "openai", ProviderKind: "openai"}); err != nil {
		t.Fatalf("create in first manager failed: %v", err)
	}
	_ = mgr1.Close()

	cfg2 := config.DefaultConfig()
	cfg2.Agents.Defaults.Workspace = workspace
	cfg2.Providers = []config.ProviderProfile{{Name: "gemini", ProviderKind: "gemini"}}

	mgr2, err := NewManager(cfg2, log)
	if err != nil {
		t.Fatalf("NewManager second failed: %v", err)
	}
	defer mgr2.Close()

	providers, err := mgr2.List(ctx)
	if err != nil {
		t.Fatalf("List second failed: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers from database, got %+v", providers)
	}

	names := []string{providers[0].Name, providers[1].Name}
	if !((names[0] == "anthropic" && names[1] == "openai") || (names[0] == "openai" && names[1] == "anthropic")) {
		t.Fatalf("unexpected provider names: %v", names)
	}

	if len(cfg2.Providers) != 2 {
		t.Fatalf("expected config to sync database providers, got %+v", cfg2.Providers)
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
