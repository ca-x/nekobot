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

	"nekobot/pkg/config"
	"nekobot/pkg/ilinkauth"
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

	store, err := ilinkauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
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
		config:    cfg,
		ilinkAuth: ilinkauth.NewService(store, login),
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
