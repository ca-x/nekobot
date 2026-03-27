package wechat

import (
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/ilinkauth"
	wxtypes "nekobot/pkg/wechat/types"
)

func TestLoadChannelBindingReturnsOnlyUniqueBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := ilinkauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if err := store.SaveBinding(&ilinkauth.Binding{
		UserID: "user-1",
		Credentials: wxtypes.Credentials{
			BotToken:    "token-1",
			ILinkBotID:  "bot-1@im.wechat",
			BaseURL:     "https://example.invalid",
			ILinkUserID: "wechat-user-1",
		},
	}); err != nil {
		t.Fatalf("SaveBinding failed: %v", err)
	}

	authSvc := ilinkauth.NewService(store, nil)
	binding, err := loadChannelBinding(authSvc)
	if err != nil {
		t.Fatalf("loadChannelBinding failed: %v", err)
	}
	if binding == nil {
		t.Fatal("expected binding")
	}
	if binding.UserID != "user-1" {
		t.Fatalf("expected user id %q, got %q", "user-1", binding.UserID)
	}
	if binding.Credentials.ILinkBotID != "bot-1@im.wechat" {
		t.Fatalf("expected bot id %q, got %q", "bot-1@im.wechat", binding.Credentials.ILinkBotID)
	}
}

func TestLoadChannelBindingReturnsNilWithoutBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := ilinkauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	authSvc := ilinkauth.NewService(store, nil)
	binding, err := loadChannelBinding(authSvc)
	if err != nil {
		t.Fatalf("loadChannelBinding failed: %v", err)
	}
	if binding != nil {
		t.Fatalf("expected nil binding, got %+v", binding)
	}
}
