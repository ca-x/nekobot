package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/channels/wechat"
	"nekobot/pkg/config"
)

func TestHandleGetWechatBindingStatus_NoBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	store, err := wechat.NewCredentialStore(cfg)
	if err != nil {
		t.Fatalf("NewCredentialStore failed: %v", err)
	}

	s := &Server{config: cfg}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/channels/wechat/binding", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/channels/wechat/binding")
	c.Set("wechat_store", store)

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
