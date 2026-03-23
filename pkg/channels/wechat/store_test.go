package wechat

import (
	"path/filepath"
	"testing"

	"nekobot/pkg/config"
)

func TestCredentialStoreReplaceCredentialsRemovesOldFiles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := NewCredentialStore(cfg)
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}

	first := &Credentials{
		BotToken:    "token-1",
		ILinkBotID:  "bot-1@im.wechat",
		BaseURL:     "https://ilinkai.weixin.qq.com",
		ILinkUserID: "user-1",
	}
	if err := store.ReplaceCredentials(first); err != nil {
		t.Fatalf("ReplaceCredentials(first) failed: %v", err)
	}
	firstPath := filepath.Join(store.accountsDir, NormalizeAccountID(first.ILinkBotID)+".json")
	if _, err := store.LoadCredentials(); err != nil {
		t.Fatalf("LoadCredentials after first replace failed: %v", err)
	}

	second := &Credentials{
		BotToken:    "token-2",
		ILinkBotID:  "bot-2@im.wechat",
		BaseURL:     "https://ilinkai.weixin.qq.com",
		ILinkUserID: "user-2",
	}
	if err := store.ReplaceCredentials(second); err != nil {
		t.Fatalf("ReplaceCredentials(second) failed: %v", err)
	}

	loaded, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials after second replace failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected stored credentials, got nil")
	}
	if loaded.ILinkBotID != second.ILinkBotID {
		t.Fatalf("expected bot id %q, got %q", second.ILinkBotID, loaded.ILinkBotID)
	}
	if _, err := store.fs.Stat(firstPath); err == nil {
		t.Fatalf("expected old credential file %s to be removed", firstPath)
	}

	syncPath := store.SyncStatePath(second.ILinkBotID)
	if err := store.WriteSyncState(second.ILinkBotID, "cursor-1"); err != nil {
		t.Fatalf("WriteSyncState failed: %v", err)
	}
	if err := store.ReplaceCredentials(first); err != nil {
		t.Fatalf("ReplaceCredentials(third) failed: %v", err)
	}
	if _, err := store.fs.Stat(syncPath); err == nil {
		t.Fatalf("expected sync state file %s to be removed", syncPath)
	}
}

func TestCredentialStoreBindStateLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := NewCredentialStore(cfg)
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}

	state := BindState{
		QRCode:        "qr-token",
		QRCodeContent: "https://example.com/qr",
		Status:        BindStatusPending,
	}
	if err := store.SaveBindState(state); err != nil {
		t.Fatalf("SaveBindState failed: %v", err)
	}

	loaded, err := store.LoadBindState()
	if err != nil {
		t.Fatalf("LoadBindState failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected bind state, got nil")
	}
	if loaded.QRCode != state.QRCode || loaded.Status != state.Status {
		t.Fatalf("unexpected bind state: %+v", loaded)
	}

	if err := store.ClearBindState(); err != nil {
		t.Fatalf("ClearBindState failed: %v", err)
	}
	loaded, err = store.LoadBindState()
	if err != nil {
		t.Fatalf("LoadBindState after clear failed: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil bind state after clear, got %+v", loaded)
	}
}
