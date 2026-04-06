package channelaccounts

import (
	"context"
	"errors"
	"testing"

	channelwechat "nekobot/pkg/channels/wechat"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	wxtypes "nekobot/pkg/wechat/types"
)

func TestManagerCRUD(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManager(t)

	created, err := mgr.Create(ctx, ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-a",
		DisplayName: "Bot A",
		Config: map[string]interface{}{
			"bot_id": "wx-1",
		},
		Metadata: map[string]interface{}{
			"owner": "ops",
		},
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected account id")
	}

	if _, err := mgr.Create(ctx, ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-a",
	}); !errors.Is(err, ErrAccountExists) {
		t.Fatalf("expected ErrAccountExists, got %v", err)
	}

	listed, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 account, got %d", len(listed))
	}

	updated, err := mgr.Update(ctx, created.ID, ChannelAccount{
		ChannelType: "slack",
		AccountKey:  "workspace-a",
		DisplayName: "Workspace A",
		Enabled:     true,
		Config: map[string]interface{}{
			"app_id": "A123",
		},
	})
	if err != nil {
		t.Fatalf("update account: %v", err)
	}
	if updated.ChannelType != "slack" || updated.AccountKey != "workspace-a" {
		t.Fatalf("unexpected updated account: %+v", updated)
	}

	got, err := mgr.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.ChannelType != "slack" {
		t.Fatalf("unexpected get result: %+v", got)
	}

	if err := mgr.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if err := mgr.Delete(ctx, created.ID); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestResolveForChannelIDPrefersActiveWechatAccountForAlias(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManager(t)

	first, err := mgr.Create(ctx, ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-1@im.wechat",
		DisplayName: "Bot 1",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create first account: %v", err)
	}
	second, err := mgr.Create(ctx, ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-2@im.wechat",
		DisplayName: "Bot 2",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create second account: %v", err)
	}

	store, err := channelwechat.NewCredentialStore(mgr.cfg)
	if err != nil {
		t.Fatalf("new credential store: %v", err)
	}
	if err := store.SaveCredentials(&wxtypes.Credentials{
		BotToken:    "token-1",
		ILinkBotID:  first.AccountKey,
		BaseURL:     "https://example.invalid",
		ILinkUserID: "user-1",
	}, false); err != nil {
		t.Fatalf("save first credentials: %v", err)
	}
	if err := store.SaveCredentials(&wxtypes.Credentials{
		BotToken:    "token-2",
		ILinkBotID:  second.AccountKey,
		BaseURL:     "https://example.invalid",
		ILinkUserID: "user-2",
	}, true); err != nil {
		t.Fatalf("save second credentials: %v", err)
	}

	resolved, err := mgr.ResolveForChannelID(ctx, "wechat")
	if err != nil {
		t.Fatalf("ResolveForChannelID failed: %v", err)
	}
	if resolved.ID != second.ID {
		t.Fatalf("expected active wechat account %q, got %q", second.ID, resolved.ID)
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
		t.Fatalf("new account manager: %v", err)
	}
	return mgr
}
