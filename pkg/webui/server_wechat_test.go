package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"

	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	pkgchannels "nekobot/pkg/channels"
	"nekobot/pkg/channels/wechat"
	"nekobot/pkg/config"
	"nekobot/pkg/ilinkauth"
	"nekobot/pkg/logger"
	wxtypes "nekobot/pkg/wechat/types"
)

func TestHandleGetWechatBindingStatus_NoBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := ilinkauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	s := &Server{config: cfg, ilinkAuth: ilinkauth.NewService(store, nil)}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/channels/wechat/binding", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/channels/wechat/binding")
	c.Set("user", jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "alice",
		"uid": "user-1",
	}))
	if err := s.handleGetWechatBindingStatus(c); err != nil {
		t.Fatalf("handleGetWechatBindingStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		Bound bool `json:"bound"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if payload.Bound {
		t.Fatalf("expected bound=false, got %+v", payload)
	}
}

func TestHandleWechatBindingLifecycle_UsesSharedIlinkAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	store, err := ilinkauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	client := newTestEntClient(t, cfg)
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

	login := &wechatLoginStub{
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

	s := &Server{
		config:     cfg,
		logger:     log,
		ilinkAuth:  ilinkauth.NewService(store, login),
		accountMgr: accountMgr,
	}

	e := echo.New()

	startReq := httptest.NewRequest(http.MethodPost, "/api/channels/wechat/binding/start", nil)
	startRec := httptest.NewRecorder()
	startCtx := e.NewContext(startReq, startRec)
	startCtx.SetPath("/api/channels/wechat/binding/start")
	setWechatTestUser(startCtx, "alice", "user-1")
	if err := s.handleStartWechatBinding(startCtx); err != nil {
		t.Fatalf("handleStartWechatBinding failed: %v", err)
	}
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected start status %d, got %d: %s", http.StatusOK, startRec.Code, startRec.Body.String())
	}

	var startPayload struct {
		Binding struct {
			Status        string `json:"status"`
			QRCodeContent string `json:"qrcode_content"`
		} `json:"binding"`
	}
	decodeWechatJSON(t, startRec.Body.Bytes(), &startPayload)
	if startPayload.Binding.Status != string(ilinkauth.BindStatusPending) {
		t.Fatalf("expected pending binding, got %+v", startPayload.Binding)
	}
	if startPayload.Binding.QRCodeContent != "mock-qr:qr-1" {
		t.Fatalf("expected qrcode content, got %+v", startPayload.Binding)
	}

	pollReq := httptest.NewRequest(http.MethodPost, "/api/channels/wechat/binding/poll", nil)
	pollRec := httptest.NewRecorder()
	pollCtx := e.NewContext(pollReq, pollRec)
	pollCtx.SetPath("/api/channels/wechat/binding/poll")
	setWechatTestUser(pollCtx, "alice", "user-1")
	if err := s.handlePollWechatBinding(pollCtx); err != nil {
		t.Fatalf("handlePollWechatBinding failed: %v", err)
	}
	if pollRec.Code != http.StatusOK {
		t.Fatalf("expected poll status %d, got %d: %s", http.StatusOK, pollRec.Code, pollRec.Body.String())
	}

	var pollPayload struct {
		Bound   bool `json:"bound"`
		Account struct {
			BotID  string `json:"bot_id"`
			UserID string `json:"user_id"`
		} `json:"account"`
		Accounts []struct {
			AccountID string `json:"account_id"`
			BotID     string `json:"bot_id"`
			UserID    string `json:"user_id"`
			Active    bool   `json:"active"`
		} `json:"accounts"`
		Binding struct {
			Status string `json:"status"`
			BotID  string `json:"bot_id"`
			UserID string `json:"user_id"`
		} `json:"binding"`
	}
	decodeWechatJSON(t, pollRec.Body.Bytes(), &pollPayload)
	if !pollPayload.Bound {
		t.Fatalf("expected bound=true, got %+v", pollPayload)
	}
	if pollPayload.Binding.Status != string(ilinkauth.BindStatusConfirmed) {
		t.Fatalf("expected confirmed binding, got %+v", pollPayload.Binding)
	}
	if pollPayload.Account.BotID != "bot-1@im.wechat" {
		t.Fatalf("expected bot id %q, got %q", "bot-1@im.wechat", pollPayload.Account.BotID)
	}
	if len(pollPayload.Accounts) != 1 {
		t.Fatalf("expected one synced channel account, got %+v", pollPayload.Accounts)
	}
	if pollPayload.Accounts[0].BotID != "bot-1@im.wechat" {
		t.Fatalf("unexpected synced channel account: %+v", pollPayload.Accounts[0])
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/channels/wechat/binding", nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/channels/wechat/binding")
	setWechatTestUser(deleteCtx, "alice", "user-1")
	if err := s.handleDeleteWechatBinding(deleteCtx); err != nil {
		t.Fatalf("handleDeleteWechatBinding failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d: %s", http.StatusOK, deleteRec.Code, deleteRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/channels/wechat/binding", nil)
	statusRec := httptest.NewRecorder()
	statusCtx := e.NewContext(statusReq, statusRec)
	statusCtx.SetPath("/api/channels/wechat/binding")
	setWechatTestUser(statusCtx, "alice", "user-1")
	if err := s.handleGetWechatBindingStatus(statusCtx); err != nil {
		t.Fatalf("handleGetWechatBindingStatus(after delete) failed: %v", err)
	}

	var statusPayload struct {
		Bound bool `json:"bound"`
	}
	decodeWechatJSON(t, statusRec.Body.Bytes(), &statusPayload)
	if statusPayload.Bound {
		t.Fatalf("expected bound=false after delete, got %+v", statusPayload)
	}

	accounts, err := accountMgr.List(context.Background())
	if err != nil {
		t.Fatalf("List channel accounts failed: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected synced wechat channel account to be removed, got %+v", accounts)
	}
}

func TestHandleWechatBindingActivateAndDeleteAccount(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	store, err := ilinkauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	client := newTestEntClient(t, cfg)
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

	first, err := accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-1@im.wechat",
		DisplayName: "bot-1@im.wechat",
		Enabled:     true,
		Config: map[string]interface{}{
			"bot_token":     "token-1",
			"ilink_bot_id":  "bot-1@im.wechat",
			"ilink_user_id": "u-1",
			"base_url":      "https://example.invalid",
		},
		Metadata: map[string]interface{}{
			"owner_user_id": "user-1",
		},
	})
	if err != nil {
		t.Fatalf("create first account: %v", err)
	}
	second, err := accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-2@im.wechat",
		DisplayName: "bot-2@im.wechat",
		Enabled:     true,
		Config: map[string]interface{}{
			"bot_token":     "token-2",
			"ilink_bot_id":  "bot-2@im.wechat",
			"ilink_user_id": "u-2",
			"base_url":      "https://example.invalid",
		},
		Metadata: map[string]interface{}{
			"owner_user_id": "user-1",
		},
	})
	if err != nil {
		t.Fatalf("create second account: %v", err)
	}

	wechatStore, err := wechat.NewCredentialStore(cfg)
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}
	if err := wechatStore.SaveCredentials(&wxtypes.Credentials{
		BotToken:    "token-1",
		ILinkBotID:  "bot-1@im.wechat",
		BaseURL:     "https://example.invalid",
		ILinkUserID: "u-1",
	}, true); err != nil {
		t.Fatalf("SaveCredentials first failed: %v", err)
	}
	if err := wechatStore.SaveCredentials(&wxtypes.Credentials{
		BotToken:    "token-2",
		ILinkBotID:  "bot-2@im.wechat",
		BaseURL:     "https://example.invalid",
		ILinkUserID: "u-2",
	}, false); err != nil {
		t.Fatalf("SaveCredentials second failed: %v", err)
	}

	authSvc := ilinkauth.NewService(store, nil)
	if err := authSvc.SaveBinding(&ilinkauth.Binding{
		UserID: "user-1",
		Credentials: wxtypes.Credentials{
			BotToken:    "token-1",
			ILinkBotID:  "bot-1@im.wechat",
			BaseURL:     "https://example.invalid",
			ILinkUserID: "u-1",
		},
	}); err != nil {
		t.Fatalf("SaveBinding failed: %v", err)
	}

	s := &Server{
		config:     cfg,
		logger:     log,
		ilinkAuth:  authSvc,
		accountMgr: accountMgr,
	}

	e := echo.New()
	actReq := httptest.NewRequest(http.MethodPost, "/api/channels/wechat/binding/activate", strings.NewReader(`{"account_id":"`+second.ID+`"}`))
	actReq.Header.Set("Content-Type", "application/json")
	actRec := httptest.NewRecorder()
	actCtx := e.NewContext(actReq, actRec)
	actCtx.SetPath("/api/channels/wechat/binding/activate")
	setWechatTestUser(actCtx, "alice", "user-1")
	if err := s.handleActivateWechatBinding(actCtx); err != nil {
		t.Fatalf("handleActivateWechatBinding failed: %v", err)
	}
	if actRec.Code != http.StatusOK {
		t.Fatalf("expected activate status %d, got %d: %s", http.StatusOK, actRec.Code, actRec.Body.String())
	}

	activeBinding, err := authSvc.GetBinding("user-1")
	if err != nil {
		t.Fatalf("GetBinding failed: %v", err)
	}
	if activeBinding.Credentials.ILinkBotID != "bot-2@im.wechat" {
		t.Fatalf("expected active binding bot-2, got %+v", activeBinding)
	}
	activeCreds, err := wechatStore.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if activeCreds == nil || activeCreds.ILinkBotID != "bot-2@im.wechat" {
		t.Fatalf("expected active local creds bot-2, got %+v", activeCreds)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/channels/wechat/binding/accounts/"+first.ID, nil)
	delRec := httptest.NewRecorder()
	delCtx := e.NewContext(delReq, delRec)
	delCtx.SetPath("/api/channels/wechat/binding/accounts/:accountId")
	delCtx.SetPathValues(echo.PathValues{{Name: "accountId", Value: first.ID}})
	setWechatTestUser(delCtx, "alice", "user-1")
	if err := s.handleDeleteWechatBindingAccount(delCtx); err != nil {
		t.Fatalf("handleDeleteWechatBindingAccount failed: %v", err)
	}
	if delRec.Code != http.StatusOK {
		t.Fatalf("expected delete account status %d, got %d: %s", http.StatusOK, delRec.Code, delRec.Body.String())
	}

	accounts, err := accountMgr.List(context.Background())
	if err != nil {
		t.Fatalf("List channel accounts failed: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != second.ID {
		t.Fatalf("expected only second account to remain, got %+v", accounts)
	}
	if _, err := authSvc.GetBinding("user-1"); err != nil {
		t.Fatalf("GetBinding after account delete failed: %v", err)
	}
	_ = first
}

func TestReloadChannelsByTypePrefersEnabledWechatAccounts(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Channels.WeChat.Enabled = false

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}

	client := newTestEntClient(t, cfg)
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

	_, err = accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-1@im.wechat",
		DisplayName: "WeChat Bot 1",
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled":       true,
			"bot_token":     "token-1",
			"ilink_bot_id":  "bot-1@im.wechat",
			"ilink_user_id": "user-1",
			"base_url":      "https://example.invalid",
		},
	})
	if err != nil {
		t.Fatalf("create first account: %v", err)
	}
	_, err = accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-2@im.wechat",
		DisplayName: "WeChat Bot 2",
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled":       true,
			"bot_token":     "token-2",
			"ilink_bot_id":  "bot-2@im.wechat",
			"ilink_user_id": "user-2",
			"base_url":      "https://example.invalid",
		},
	})
	if err != nil {
		t.Fatalf("create second account: %v", err)
	}

	messageBus := bus.NewLocalBus(log, 8)
	if err := messageBus.Start(); err != nil {
		t.Fatalf("Start bus failed: %v", err)
	}
	t.Cleanup(func() {
		if err := messageBus.Stop(); err != nil {
			t.Fatalf("Stop bus failed: %v", err)
		}
	})

	manager := pkgchannels.NewManager(log, messageBus)
	if err := manager.Register(&wechatReloadStubChannel{id: "wechat", channelType: "wechat", enabled: true}); err != nil {
		t.Fatalf("register legacy channel: %v", err)
	}

	s := &Server{
		config:     cfg,
		logger:     log,
		channels:   manager,
		accountMgr: accountMgr,
	}

	if err := s.reloadChannelsByType("wechat"); err != nil {
		t.Fatalf("reloadChannelsByType failed: %v", err)
	}

	legacy, err := manager.GetChannel("wechat")
	if err != nil {
		t.Fatalf("GetChannel alias failed: %v", err)
	}
	if legacy.ID() == "wechat" {
		t.Fatalf("expected alias to resolve account runtime after reload, got legacy runtime")
	}

	items := manager.ListChannelsByType("wechat")
	if len(items) != 2 {
		t.Fatalf("expected 2 wechat account runtimes, got %d", len(items))
	}
}

type wechatReloadStubChannel struct {
	id          string
	channelType string
	enabled     bool
}

func (c *wechatReloadStubChannel) ID() string                  { return c.id }
func (c *wechatReloadStubChannel) ChannelType() string         { return c.channelType }
func (c *wechatReloadStubChannel) Name() string                { return c.id }
func (c *wechatReloadStubChannel) Start(context.Context) error { return nil }
func (c *wechatReloadStubChannel) Stop(context.Context) error  { return nil }
func (c *wechatReloadStubChannel) IsEnabled() bool             { return c.enabled }
func (c *wechatReloadStubChannel) SendMessage(context.Context, *bus.Message) error {
	return nil
}

type wechatLoginStub struct {
	qrResp     *wxtypes.QRCodeResponse
	statusResp *wxtypes.QRStatusResponse
}

func (s *wechatLoginStub) FetchQRCode(context.Context) (*wxtypes.QRCodeResponse, error) {
	return s.qrResp, nil
}

func (s *wechatLoginStub) CheckQRStatus(context.Context, string) (*wxtypes.QRStatusResponse, error) {
	return s.statusResp, nil
}

func setWechatTestUser(c *echo.Context, username, userID string) {
	c.Set("user", jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": strings.TrimSpace(username),
		"uid": strings.TrimSpace(userID),
	}))
}

func decodeWechatJSON(t *testing.T, body []byte, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("unmarshal json failed: %v; body=%s", err, string(body))
	}
}
