package wechat

import (
	"testing"

	"nekobot/pkg/config"
	wxtypes "nekobot/pkg/wechat/types"
)

func TestNewCredentialStoreLoadsActiveCredentials(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := NewCredentialStore(cfg)
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}

	creds := &wxtypes.Credentials{
		BotToken:    "token-1",
		ILinkBotID:  "bot-1@im.wechat",
		BaseURL:     "https://example.invalid",
		ILinkUserID: "wechat-user-1",
	}
	if err := store.ReplaceCredentials(creds); err != nil {
		t.Fatalf("ReplaceCredentials failed: %v", err)
	}

	loaded, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected credentials")
	}
	if loaded.ILinkBotID != creds.ILinkBotID {
		t.Fatalf("expected bot id %q, got %q", creds.ILinkBotID, loaded.ILinkBotID)
	}
}

func TestNewCredentialStoreReturnsNilWithoutStoredCredentials(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := NewCredentialStore(cfg)
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}

	loaded, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil credentials, got %+v", loaded)
	}
}

func TestWeChatSessionIDUsesInstancePrefixForAccountRuntime(t *testing.T) {
	ch := &Channel{id: "wechat:bot-a", channelType: "wechat"}
	if got := ch.sessionID("user-1"); got != "wechat:bot-a:user-1" {
		t.Fatalf("unexpected account runtime session id: %s", got)
	}
}

func TestWeChatSessionIDKeepsLegacyPrefixForDefaultRuntime(t *testing.T) {
	ch := &Channel{id: "wechat", channelType: "wechat"}
	if got := ch.sessionID("user-1"); got != "wechat:user-1" {
		t.Fatalf("unexpected default runtime session id: %s", got)
	}
}
