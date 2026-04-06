package channels

import (
	"context"
	"testing"

	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func TestRegisterChannelsPrefersChannelAccountsOverLegacyConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Channels.Gotify.Enabled = true
	cfg.Channels.Gotify.ServerURL = "https://legacy.example.com"
	cfg.Channels.Gotify.AppToken = "legacy-token"
	cfg.Channels.Gotify.Priority = 5

	log := newFXTestLogger(t)
	client := newFXTestEntClient(t, cfg)
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

	_, err = accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "gotify",
		AccountKey:  "alerts-a",
		DisplayName: "Alerts A",
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled":    true,
			"server_url": "https://account.example.com",
			"app_token":  "account-token",
			"priority":   8,
		},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}

	manager := NewManager(log, nil)
	if err := RegisterChannels(manager, log, nil, nil, nil, cfg, accountMgr, nil, nil, nil); err != nil {
		t.Fatalf("RegisterChannels failed: %v", err)
	}

	ch, err := manager.GetChannel("gotify")
	if err != nil {
		t.Fatalf("GetChannel failed: %v", err)
	}
	if ch.ID() != "gotify:alerts-a" {
		t.Fatalf("expected account-scoped channel id, got %s", ch.ID())
	}
}

func TestRegisterChannelsFallsBackToLegacyConfigWithoutAccounts(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Channels.Gotify.Enabled = true
	cfg.Channels.Gotify.ServerURL = "https://legacy.example.com"
	cfg.Channels.Gotify.AppToken = "legacy-token"
	cfg.Channels.Gotify.Priority = 5

	log := newFXTestLogger(t)
	manager := NewManager(log, nil)
	if err := RegisterChannels(manager, log, nil, nil, nil, cfg, nil, nil, nil, nil); err != nil {
		t.Fatalf("RegisterChannels failed: %v", err)
	}

	ch, err := manager.GetChannel("gotify")
	if err != nil {
		t.Fatalf("GetChannel failed: %v", err)
	}
	if ch.ID() != "gotify" {
		t.Fatalf("expected legacy channel id, got %s", ch.ID())
	}
}

func TestRegisterChannelsPrefersTeamsChannelAccountsOverLegacyConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Channels.Teams.Enabled = true
	cfg.Channels.Teams.AppID = "legacy-app"
	cfg.Channels.Teams.AppPassword = "legacy-secret"

	log := newFXTestLogger(t)
	client := newFXTestEntClient(t, cfg)
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

	_, err = accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "teams",
		AccountKey:  "tenant-a",
		DisplayName: "Teams Tenant A",
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled":      true,
			"app_id":       "account-app",
			"app_password": "account-secret",
		},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}

	manager := NewManager(log, nil)
	if err := RegisterChannels(manager, log, nil, nil, nil, cfg, accountMgr, nil, nil, nil); err != nil {
		t.Fatalf("RegisterChannels failed: %v", err)
	}

	ch, err := manager.GetChannel("teams")
	if err != nil {
		t.Fatalf("GetChannel failed: %v", err)
	}
	if ch.ID() != "teams:tenant-a" {
		t.Fatalf("expected account-scoped channel id, got %s", ch.ID())
	}
}

func newFXTestLogger(t *testing.T) *logger.Logger {
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

func newFXTestEntClient(t *testing.T, cfg *config.Config) *ent.Client {
	t.Helper()
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
	return client
}
