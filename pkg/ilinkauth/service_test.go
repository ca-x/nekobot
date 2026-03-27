package ilinkauth

import (
	"context"
	"testing"
	"time"

	"nekobot/pkg/config"
	wxtypes "nekobot/pkg/wechat/types"
)

type loginClientStub struct {
	qrResp     *wxtypes.QRCodeResponse
	statusResp *wxtypes.QRStatusResponse
}

func (s *loginClientStub) FetchQRCode(ctx context.Context) (*wxtypes.QRCodeResponse, error) {
	return s.qrResp, nil
}

func (s *loginClientStub) CheckQRStatus(ctx context.Context, qrcode string) (*wxtypes.QRStatusResponse, error) {
	return s.statusResp, nil
}

func TestServiceStartAndPollBindingLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	login := &loginClientStub{
		qrResp: &wxtypes.QRCodeResponse{
			QRCode:           "qr-1",
			QRCodeImgContent: "mock-qr:qr-1",
		},
		statusResp: &wxtypes.QRStatusResponse{
			Status:      wxtypes.QRStatusConfirmed,
			BotToken:    "bot-token",
			ILinkBotID:  "bot-1@im.wechat",
			BaseURL:     "https://example.invalid",
			ILinkUserID: "ilink-user-1",
		},
	}

	svc := NewService(store, login)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	session, err := svc.StartBinding(ctx, "user-123")
	if err != nil {
		t.Fatalf("StartBinding failed: %v", err)
	}
	if session.Status != BindStatusPending {
		t.Fatalf("expected pending status, got %q", session.Status)
	}

	polled, err := svc.PollBinding(ctx, "user-123")
	if err != nil {
		t.Fatalf("PollBinding failed: %v", err)
	}
	if polled.Status != BindStatusConfirmed {
		t.Fatalf("expected confirmed status, got %q", polled.Status)
	}
	if polled.BotID != "bot-1@im.wechat" {
		t.Fatalf("expected bot id %q, got %q", "bot-1@im.wechat", polled.BotID)
	}

	binding, err := svc.GetBinding("user-123")
	if err != nil {
		t.Fatalf("GetBinding failed: %v", err)
	}
	if binding.Credentials.BotToken != "bot-token" {
		t.Fatalf("expected token %q, got %q", "bot-token", binding.Credentials.BotToken)
	}

	syncPath := svc.SyncStatePath("user-123", binding.Credentials.ILinkBotID)
	if syncPath == "" {
		t.Fatal("expected sync state path")
	}
}
