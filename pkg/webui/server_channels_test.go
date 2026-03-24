package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
)

func TestHandleGetChannelsIncludesWechat(t *testing.T) {
	s := &Server{config: config.DefaultConfig()}
	s.config.Channels.WeChat.Enabled = true
	s.config.Channels.WeChat.PollIntervalSeconds = 13

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetChannels(c); err != nil {
		t.Fatalf("handleGetChannels failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	raw, ok := payload["wechat"]
	if !ok {
		t.Fatalf("expected wechat channel in response: %s", rec.Body.String())
	}

	var wechatCfg config.WeChatConfig
	if err := json.Unmarshal(raw, &wechatCfg); err != nil {
		t.Fatalf("unmarshal wechat config failed: %v", err)
	}
	if !wechatCfg.Enabled || wechatCfg.PollIntervalSeconds != 13 {
		t.Fatalf("unexpected wechat config: %+v", wechatCfg)
	}
}

func TestHandleGetChannelsIncludesGotify(t *testing.T) {
	s := &Server{config: config.DefaultConfig()}
	s.config.Channels.Gotify.Enabled = true
	s.config.Channels.Gotify.ServerURL = "https://gotify.example.com"
	s.config.Channels.Gotify.Priority = 8

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetChannels(c); err != nil {
		t.Fatalf("handleGetChannels failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	raw, ok := payload["gotify"]
	if !ok {
		t.Fatalf("expected gotify channel in response: %s", rec.Body.String())
	}

	var gotifyCfg config.GotifyConfig
	if err := json.Unmarshal(raw, &gotifyCfg); err != nil {
		t.Fatalf("unmarshal gotify config failed: %v", err)
	}
	if !gotifyCfg.Enabled || gotifyCfg.ServerURL != "https://gotify.example.com" || gotifyCfg.Priority != 8 {
		t.Fatalf("unexpected gotify config: %+v", gotifyCfg)
	}
}
