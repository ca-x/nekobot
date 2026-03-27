package mockserver

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	wxauth "nekobot/pkg/wechat/auth"
	"nekobot/pkg/wechat/client"
	wxtypes "nekobot/pkg/wechat/types"
)

func TestServerSupportsQRCodeLifecycle(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doer := newRewriteDoer(t, srv.URL())

	qrResp, err := wxauth.FetchQRCode(ctx, client.WithHTTPDoer(doer))
	if err != nil {
		t.Fatalf("FetchQRCode failed: %v", err)
	}
	if qrResp.QRCode == "" {
		t.Fatal("expected qrcode")
	}
	if qrResp.QRCodeImgContent == "" {
		t.Fatal("expected qrcode_img_content")
	}

	statusResp, err := wxauth.CheckQRStatus(ctx, qrResp.QRCode, client.WithHTTPDoer(doer))
	if err != nil {
		t.Fatalf("CheckQRStatus(wait) failed: %v", err)
	}
	if statusResp.Status != wxtypes.QRStatusWait {
		t.Fatalf("expected wait status, got %q", statusResp.Status)
	}

	srv.Engine().ScanQR()

	statusResp, err = wxauth.CheckQRStatus(ctx, qrResp.QRCode, client.WithHTTPDoer(doer))
	if err != nil {
		t.Fatalf("CheckQRStatus(scanned) failed: %v", err)
	}
	if statusResp.Status != wxtypes.QRStatusScanned {
		t.Fatalf("expected scanned status, got %q", statusResp.Status)
	}

	wantCreds := &wxtypes.Credentials{
		BotToken:    "bot-token",
		ILinkBotID:  "bot-1@im.wechat",
		BaseURL:     srv.URL(),
		ILinkUserID: "user-1",
	}
	srv.Engine().ConfirmQR(wantCreds)

	statusResp, err = wxauth.CheckQRStatus(ctx, qrResp.QRCode, client.WithHTTPDoer(doer))
	if err != nil {
		t.Fatalf("CheckQRStatus(confirmed) failed: %v", err)
	}
	if statusResp.Status != wxtypes.QRStatusConfirmed {
		t.Fatalf("expected confirmed status, got %q", statusResp.Status)
	}
	if statusResp.BotToken != wantCreds.BotToken {
		t.Fatalf("expected bot token %q, got %q", wantCreds.BotToken, statusResp.BotToken)
	}
	if statusResp.ILinkBotID != wantCreds.ILinkBotID {
		t.Fatalf("expected bot id %q, got %q", wantCreds.ILinkBotID, statusResp.ILinkBotID)
	}
}

func TestServerSupportsSDKEndpoints(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	creds := &wxtypes.Credentials{
		BotToken:    "bot-token",
		ILinkBotID:  "bot-1@im.wechat",
		BaseURL:     srv.URL(),
		ILinkUserID: "user-1",
	}
	apiClient := client.NewClient(creds)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv.Engine().InjectInbound(wxtypes.WeixinMessage{
		FromUserID:   "user-1",
		ToUserID:     creds.ILinkBotID,
		MessageType:  wxtypes.MessageTypeUser,
		MessageState: wxtypes.MessageStateFinish,
		ContextToken: "ctx-1",
		ItemList: []wxtypes.MessageItem{
			{
				Type:     wxtypes.ItemTypeText,
				TextItem: &wxtypes.TextItem{Text: "hello from inbound"},
			},
		},
	})

	updates, err := apiClient.GetUpdates(ctx, "")
	if err != nil {
		t.Fatalf("GetUpdates failed: %v", err)
	}
	if len(updates.Msgs) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates.Msgs))
	}
	if got := updates.Msgs[0].ItemList[0].TextItem.Text; got != "hello from inbound" {
		t.Fatalf("expected inbound text %q, got %q", "hello from inbound", got)
	}

	cfgResp, err := apiClient.GetConfig(ctx, "user-1", "ctx-1")
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if cfgResp.TypingTicket == "" {
		t.Fatal("expected typing ticket")
	}

	if err := apiClient.SendTyping(ctx, "user-1", cfgResp.TypingTicket, wxtypes.TypingStatusTyping); err != nil {
		t.Fatalf("SendTyping failed: %v", err)
	}
	if !srv.Engine().TypingEnabled("user-1") {
		t.Fatal("expected typing status enabled")
	}

	sendResp, err := apiClient.SendMessage(ctx, &wxtypes.SendMessageRequest{
		Msg: wxtypes.SendMsg{
			FromUserID:   creds.ILinkBotID,
			ToUserID:     "user-1",
			ClientID:     "client-1",
			MessageType:  wxtypes.MessageTypeBot,
			MessageState: wxtypes.MessageStateFinish,
			ContextToken: "ctx-1",
			ItemList: []wxtypes.MessageItem{
				{
					Type:     wxtypes.ItemTypeText,
					TextItem: &wxtypes.TextItem{Text: "hello outbound"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if sendResp.Ret != 0 {
		t.Fatalf("expected ret=0, got %d", sendResp.Ret)
	}

	sent := srv.Engine().SentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].Recipient != "user-1" {
		t.Fatalf("expected recipient %q, got %q", "user-1", sent[0].Recipient)
	}
	if sent[0].Text != "hello outbound" {
		t.Fatalf("expected text %q, got %q", "hello outbound", sent[0].Text)
	}
}

type rewriteDoer struct {
	base   *url.URL
	client *http.Client
}

func newRewriteDoer(t *testing.T, rawBaseURL string) *rewriteDoer {
	t.Helper()

	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}

	return &rewriteDoer{
		base:   baseURL,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (d *rewriteDoer) Do(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	rewritten := *d.base
	rewritten.Path = req.URL.Path
	rewritten.RawPath = req.URL.RawPath
	rewritten.RawQuery = req.URL.RawQuery
	cloned.URL = &rewritten
	cloned.Host = ""
	return d.client.Do(cloned)
}
