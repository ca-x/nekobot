package modelroute

import (
	"context"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func TestManagerCRUDAndResolution(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	providerMgr, err := NewProviderWeightReader(cfg, newTestLogger(t), client)
	if err != nil {
		t.Fatalf("NewProviderWeightReader failed: %v", err)
	}
	if _, err := providerMgr.Create(ctx, config.ProviderProfile{
		Name:          "openai-main",
		ProviderKind:  "openai",
		DefaultWeight: 7,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	if _, err := providerMgr.Create(ctx, config.ProviderProfile{
		Name:          "openai-backup",
		ProviderKind:  "openai",
		DefaultWeight: 3,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("create backup provider failed: %v", err)
	}

	mgr, err := NewManager(cfg, newTestLogger(t), client)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if _, err := mgr.Create(ctx, ModelRoute{
		ModelID:        "gpt-4.1",
		ProviderName:   "openai-main",
		Enabled:        true,
		IsDefault:      true,
		WeightOverride: 0,
		Aliases:        []string{"gpt-4.1-latest"},
		RegexRules:     []string{"^gpt-4\\.1-mini$"},
		Metadata:       map[string]interface{}{"source": "builtin"},
	}); err != nil {
		t.Fatalf("create primary route failed: %v", err)
	}
	if _, err := mgr.Create(ctx, ModelRoute{
		ModelID:        "gpt-4.1",
		ProviderName:   "openai-backup",
		Enabled:        true,
		IsDefault:      false,
		WeightOverride: 11,
	}); err != nil {
		t.Fatalf("create backup route failed: %v", err)
	}

	listed, err := mgr.ListByModel(ctx, "gpt-4.1")
	if err != nil {
		t.Fatalf("ListByModel failed: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(listed))
	}
	var primaryRoute ModelRoute
	var backupRoute ModelRoute
	for _, item := range listed {
		switch item.ProviderName {
		case "openai-main":
			primaryRoute = item
		case "openai-backup":
			backupRoute = item
		}
	}

	route, err := mgr.ResolveInput(ctx, "gpt-4.1-latest")
	if err != nil {
		t.Fatalf("ResolveInput alias failed: %v", err)
	}
	if route.ModelID != "gpt-4.1" || route.ProviderName != "openai-main" {
		t.Fatalf("unexpected alias route: %+v", route)
	}

	regexRoute, err := mgr.ResolveInput(ctx, "gpt-4.1-mini")
	if err != nil {
		t.Fatalf("ResolveInput regex failed: %v", err)
	}
	if regexRoute.ProviderName != "openai-main" {
		t.Fatalf("unexpected regex route: %+v", regexRoute)
	}

	defaultRoute, err := mgr.DefaultRoute(ctx, "gpt-4.1")
	if err != nil {
		t.Fatalf("DefaultRoute failed: %v", err)
	}
	if defaultRoute.ProviderName != "openai-main" {
		t.Fatalf("expected openai-main as default route, got %+v", defaultRoute)
	}

	weight, err := mgr.EffectiveWeight(ctx, backupRoute)
	if err != nil {
		t.Fatalf("EffectiveWeight failed: %v", err)
	}
	if weight != 11 {
		t.Fatalf("expected override weight 11, got %d", weight)
	}

	weight, err = mgr.EffectiveWeight(ctx, primaryRoute)
	if err != nil {
		t.Fatalf("EffectiveWeight provider default failed: %v", err)
	}
	if weight != 7 {
		t.Fatalf("expected provider default weight 7, got %d", weight)
	}

	updated, err := mgr.Update(ctx, "gpt-4.1", "openai-backup", ModelRoute{
		ModelID:        "gpt-4.1",
		ProviderName:   "openai-backup",
		Enabled:        false,
		IsDefault:      false,
		WeightOverride: 5,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Enabled || updated.WeightOverride != 5 {
		t.Fatalf("unexpected updated route: %+v", updated)
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
