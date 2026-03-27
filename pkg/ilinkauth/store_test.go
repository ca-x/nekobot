package ilinkauth

import (
	"errors"
	"testing"

	"nekobot/pkg/config"
	wxtypes "nekobot/pkg/wechat/types"
)

func TestStorePersistsSingleBindingPerUser(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	userID := "user-123"
	first := &Binding{
		UserID: userID,
		Credentials: wxtypes.Credentials{
			BotToken:    "token-1",
			ILinkBotID:  "bot-1@im.wechat",
			BaseURL:     "https://example.invalid",
			ILinkUserID: "ilink-user-1",
		},
	}
	if err := store.SaveBinding(first); err != nil {
		t.Fatalf("SaveBinding(first) failed: %v", err)
	}

	second := &Binding{
		UserID: userID,
		Credentials: wxtypes.Credentials{
			BotToken:    "token-2",
			ILinkBotID:  "bot-2@im.wechat",
			BaseURL:     "https://example.invalid",
			ILinkUserID: "ilink-user-2",
		},
	}
	if err := store.SaveBinding(second); err != nil {
		t.Fatalf("SaveBinding(second) failed: %v", err)
	}

	loaded, err := store.LoadBinding(userID)
	if err != nil {
		t.Fatalf("LoadBinding failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected binding")
	}
	if loaded.Credentials.ILinkBotID != second.Credentials.ILinkBotID {
		t.Fatalf("expected bot id %q, got %q", second.Credentials.ILinkBotID, loaded.Credentials.ILinkBotID)
	}
	if loaded.Credentials.BotToken != second.Credentials.BotToken {
		t.Fatalf("expected token %q, got %q", second.Credentials.BotToken, loaded.Credentials.BotToken)
	}
}

func TestStorePersistsBindSessionAndSyncState(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	userID := "user-123"
	session := BindSession{
		UserID:        userID,
		QRCode:        "qr-1",
		QRCodeContent: "mock-qr:qr-1",
		Status:        BindStatusPending,
	}
	if err := store.SaveBindSession(session); err != nil {
		t.Fatalf("SaveBindSession failed: %v", err)
	}

	loadedSession, err := store.LoadBindSession(userID)
	if err != nil {
		t.Fatalf("LoadBindSession failed: %v", err)
	}
	if loadedSession == nil {
		t.Fatal("expected bind session")
	}
	if loadedSession.QRCode != session.QRCode {
		t.Fatalf("expected qrcode %q, got %q", session.QRCode, loadedSession.QRCode)
	}
	if loadedSession.Status != session.Status {
		t.Fatalf("expected status %q, got %q", session.Status, loadedSession.Status)
	}

	if err := store.WriteSyncState(userID, "bot-1@im.wechat", "cursor-1"); err != nil {
		t.Fatalf("WriteSyncState failed: %v", err)
	}
	cursor, err := store.ReadSyncState(userID, "bot-1@im.wechat")
	if err != nil {
		t.Fatalf("ReadSyncState failed: %v", err)
	}
	if cursor != "cursor-1" {
		t.Fatalf("expected cursor %q, got %q", "cursor-1", cursor)
	}

	if err := store.DeleteBinding(userID); err != nil {
		t.Fatalf("DeleteBinding failed: %v", err)
	}
	if _, err := store.LoadBinding(userID); !errors.Is(err, ErrBindingNotFound) {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
	if err := store.ClearBindSession(userID); err != nil {
		t.Fatalf("ClearBindSession failed: %v", err)
	}
	sessionAfterDelete, err := store.LoadBindSession(userID)
	if err != nil {
		t.Fatalf("LoadBindSession after clear failed: %v", err)
	}
	if sessionAfterDelete != nil {
		t.Fatalf("expected nil bind session after clear, got %+v", sessionAfterDelete)
	}
}
