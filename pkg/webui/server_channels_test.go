package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/bus"
	"nekobot/pkg/channels"
	"nekobot/pkg/channels/slack"
	"nekobot/pkg/config"
	"nekobot/pkg/ilinkauth"
	wxtypes "nekobot/pkg/wechat/types"
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

func TestBuildWechatBindingPayloadIncludesCurrentBinding(t *testing.T) {
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
			BaseURL:     "https://ilinkai.weixin.qq.com",
			ILinkUserID: "wechat-user-1",
		},
	}); err != nil {
		t.Fatalf("SaveBinding failed: %v", err)
	}

	authSvc := ilinkauth.NewService(store, nil)
	s := &Server{config: cfg, ilinkAuth: authSvc}
	payload, err := s.buildWechatBindingPayload(authSvc, "user-1")
	if err != nil {
		t.Fatalf("buildWechatBindingPayload failed: %v", err)
	}

	if payload["bound"] != true {
		t.Fatalf("expected bound=true, got %#v", payload["bound"])
	}
	account, ok := payload["account"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected account payload, got %#v", payload["account"])
	}
	if account["bot_id"] != "bot-1@im.wechat" {
		t.Fatalf("unexpected bot id: %#v", account["bot_id"])
	}
	if account["user_id"] != "wechat-user-1" {
		t.Fatalf("unexpected user id: %#v", account["user_id"])
	}
}

func TestHandleUpdateChannelRejectsInvalidConfigWithoutMutatingState(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	messageBus := bus.NewLocalBus(log, 8)
	manager := channels.NewManager(log, messageBus)

	original, err := slack.NewChannel(
		log,
		config.SlackConfig{Enabled: true, BotToken: "xoxb-old", AppToken: "xapp-old"},
		messageBus,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("slack.NewChannel failed: %v", err)
	}
	if err := manager.Register(original); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	s := &Server{
		config:   cfg,
		logger:   log,
		channels: manager,
		bus:      messageBus,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/channels/slack", strings.NewReader(`{"enabled":true,"bot_token":"xoxb-new","app_token":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/channels/:name")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "slack"}})

	if err := s.handleUpdateChannel(c); err != nil {
		t.Fatalf("handleUpdateChannel failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	if s.config.Channels.Slack.Enabled {
		t.Fatalf("expected live slack config to remain unchanged, got %+v", s.config.Channels.Slack)
	}
	if s.config.Channels.Slack.BotToken != "" || s.config.Channels.Slack.AppToken != "" {
		t.Fatalf("expected live slack credentials to remain empty, got %+v", s.config.Channels.Slack)
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}
	if reloaded.Channels.Slack.Enabled {
		t.Fatalf("expected persisted slack config to remain unchanged, got %+v", reloaded.Channels.Slack)
	}

	current, err := manager.GetChannel("slack")
	if err != nil {
		t.Fatalf("expected original slack channel to remain registered: %v", err)
	}
	if current != original {
		t.Fatalf("expected original slack channel to remain active")
	}
}

func TestHandleUpdateChannelKeepsExistingRuntimeWhenPrebuildFails(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	messageBus := bus.NewLocalBus(log, 8)
	manager := channels.NewManager(log, messageBus)

	original, err := slack.NewChannel(
		log,
		config.SlackConfig{Enabled: true, BotToken: "xoxb-old", AppToken: "xapp-old"},
		messageBus,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("slack.NewChannel failed: %v", err)
	}
	if err := manager.Register(original); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	s := &Server{
		config:   cfg,
		logger:   log,
		channels: manager,
		bus:      messageBus,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/channels/slack", strings.NewReader(`{"enabled":true,"bot_token":"","app_token":"xapp-new"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/channels/:name")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "slack"}})

	if err := s.handleUpdateChannel(c); err != nil {
		t.Fatalf("handleUpdateChannel failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	current, err := manager.GetChannel("slack")
	if err != nil {
		t.Fatalf("expected slack channel to remain registered: %v", err)
	}
	if current != original {
		t.Fatalf("expected existing runtime channel to remain active")
	}
}
