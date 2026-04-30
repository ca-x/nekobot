package notificationroutes

import (
	"context"
	"errors"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/ownership"
)

func TestManagerRouteAndBindingCRUD(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManager(t)

	route, err := mgr.CreateRoute(ctx, NotificationRoute{
		Name:             "ops-wechat",
		Description:      "Ops alerts",
		Enabled:          true,
		ChannelAccountID: "account-1",
		TargetConfigJSON: `{"target":"ops"}`,
		TenantID:         "tenant-a",
		OwnerUserID:      "user-a",
		Visibility:       "private",
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if route.ID == "" {
		t.Fatalf("expected route id")
	}
	if route.TenantID != "tenant-a" || route.OwnerUserID != "user-a" || route.Visibility != "private" {
		t.Fatalf("expected route ownership to round-trip, got %+v", route)
	}

	binding, err := mgr.CreateBinding(ctx, NotificationBinding{
		Scope:          "thread",
		Target:         "#pi-4家族群:000972d3",
		RouteID:        route.ID,
		EventTypesJSON: `["message.created"]`,
		Enabled:        true,
		TenantID:       "tenant-a",
		OwnerUserID:    "user-a",
		Visibility:     "shared",
	})
	if err != nil {
		t.Fatalf("create binding: %v", err)
	}
	if binding.ID == "" {
		t.Fatalf("expected binding id")
	}
	if binding.RouteID != route.ID || binding.Visibility != "shared" {
		t.Fatalf("unexpected binding: %+v", binding)
	}

	byRoute, err := mgr.ListBindingsByRoute(ctx, route.ID)
	if err != nil {
		t.Fatalf("list bindings by route: %v", err)
	}
	if len(byRoute) != 1 || byRoute[0].ID != binding.ID {
		t.Fatalf("expected one binding for route, got %+v", byRoute)
	}

	updated, err := mgr.UpdateRoute(ctx, route.ID, NotificationRoute{
		Name:             "ops-wechat-primary",
		Description:      "Primary ops alerts",
		Enabled:          false,
		ChannelAccountID: "account-2",
		TargetConfigJSON: `{}`,
		TenantID:         "tenant-b",
		OwnerUserID:      "user-b",
		Visibility:       "system",
	})
	if err != nil {
		t.Fatalf("update route: %v", err)
	}
	if updated.Name != "ops-wechat-primary" || updated.Enabled {
		t.Fatalf("unexpected updated route: %+v", updated)
	}
	if updated.TenantID != "tenant-b" || updated.OwnerUserID != "user-b" || updated.Visibility != "system" {
		t.Fatalf("expected updated route ownership, got %+v", updated)
	}

	if err := mgr.DeleteBinding(ctx, binding.ID); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	if err := mgr.DeleteRoute(ctx, route.ID); err != nil {
		t.Fatalf("delete route: %v", err)
	}
	if _, err := mgr.GetRoute(ctx, route.ID); err == nil {
		t.Fatalf("expected deleted route lookup to fail")
	}
}

func TestManagerFiltersRoutesByAuthContext(t *testing.T) {
	mgr := newTestManager(t)

	if _, err := mgr.CreateRoute(context.Background(), NotificationRoute{
		Name:        "private-a",
		TenantID:    "tenant-a",
		OwnerUserID: "user-a",
		Visibility:  "private",
	}); err != nil {
		t.Fatalf("create private route: %v", err)
	}
	if _, err := mgr.CreateRoute(context.Background(), NotificationRoute{
		Name:        "shared-a",
		TenantID:    "tenant-a",
		OwnerUserID: "user-b",
		Visibility:  "shared",
	}); err != nil {
		t.Fatalf("create shared route: %v", err)
	}
	if _, err := mgr.CreateRoute(context.Background(), NotificationRoute{
		Name:        "private-b",
		TenantID:    "tenant-b",
		OwnerUserID: "user-b",
		Visibility:  "private",
	}); err != nil {
		t.Fatalf("create other private route: %v", err)
	}
	if _, err := mgr.CreateRoute(context.Background(), NotificationRoute{
		Name:       "system",
		Visibility: "system",
	}); err != nil {
		t.Fatalf("create system route: %v", err)
	}

	ctx := ownership.WithAuthContext(context.Background(), ownership.AuthContext{
		UserID:   "user-a",
		TenantID: "tenant-a",
	})
	routes, err := mgr.ListRoutes(ctx)
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	got := map[string]bool{}
	for _, route := range routes {
		got[route.Name] = true
	}
	if !got["private-a"] || !got["shared-a"] || !got["system"] {
		t.Fatalf("expected own, tenant-shared, and system routes, got %+v", got)
	}
	if got["private-b"] {
		t.Fatalf("did not expect private route from another user/tenant, got %+v", got)
	}
}

func TestManagerRejectsCrossUserRouteWrite(t *testing.T) {
	mgr := newTestManager(t)
	route, err := mgr.CreateRoute(context.Background(), NotificationRoute{
		Name:        "private-a",
		TenantID:    "tenant-a",
		OwnerUserID: "user-a",
		Visibility:  "private",
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	ctx := ownership.WithAuthContext(context.Background(), ownership.AuthContext{
		UserID:   "user-b",
		TenantID: "tenant-a",
	})
	_, err = mgr.UpdateRoute(ctx, route.ID, NotificationRoute{
		Name:       "rename",
		Visibility: "shared",
	})
	if !errors.Is(err, ownership.ErrPermissionDenied) {
		t.Fatalf("expected permission denied, got %v", err)
	}
	if err := mgr.DeleteRoute(ctx, route.ID); !errors.Is(err, ownership.ErrPermissionDenied) {
		t.Fatalf("expected permission denied on delete, got %v", err)
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
		t.Fatalf("new notification routes manager: %v", err)
	}
	return mgr
}
