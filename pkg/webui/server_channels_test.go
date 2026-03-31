package webui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
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

func TestHandleGetChannelsIncludesRuntimeInstances(t *testing.T) {
	cfg := config.DefaultConfig()
	log := newTestLogger(t)
	manager := channels.NewManager(log, nil)
	ch, err := channels.BuildChannelFromAccount(channelAccountFixture("gotify", "alerts-a", "Alerts A", map[string]interface{}{
		"enabled":    true,
		"server_url": "https://gotify.example.com",
		"app_token":  "token-1",
		"priority":   5,
	}), log, nil, nil, nil, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("BuildChannelFromAccount failed: %v", err)
	}
	if err := manager.Register(ch); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	s := &Server{config: cfg, channels: manager}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetChannels(c); err != nil {
		t.Fatalf("handleGetChannels failed: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	raw, ok := payload["_instances"]
	if !ok {
		t.Fatalf("expected _instances in response: %s", rec.Body.String())
	}

	var instances []map[string]interface{}
	if err := json.Unmarshal(raw, &instances); err != nil {
		t.Fatalf("unmarshal instances failed: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0]["id"] != "gotify:alerts-a" {
		t.Fatalf("unexpected instance id: %#v", instances[0]["id"])
	}
	if instances[0]["type"] != "gotify" {
		t.Fatalf("unexpected instance type: %#v", instances[0]["type"])
	}
}

func TestBuildWechatBindingPayloadIncludesCurrentBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

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
	accountItem, err := accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-1@im.wechat",
		DisplayName: "bot-1@im.wechat",
		Enabled:     true,
		Config: map[string]interface{}{
			"ilink_bot_id":  "bot-1@im.wechat",
			"ilink_user_id": "wechat-user-1",
		},
		Metadata: map[string]interface{}{
			"owner_user_id": "user-1",
		},
	})
	if err != nil {
		t.Fatalf("create channel account failed: %v", err)
	}

	s := &Server{config: cfg, ilinkAuth: authSvc, accountMgr: accountMgr}
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
	if payload["active_account_id"] != accountItem.ID {
		t.Fatalf("expected active_account_id=%q, got %#v", accountItem.ID, payload["active_account_id"])
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

func channelAccountFixture(
	channelType string,
	accountKey string,
	displayName string,
	accountConfig map[string]interface{},
) channelaccounts.ChannelAccount {
	return channelaccounts.ChannelAccount{
		ChannelType: channelType,
		AccountKey:  accountKey,
		DisplayName: displayName,
		Enabled:     true,
		Config:      accountConfig,
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

type testConfiguredChannel struct {
	id      string
	name    string
	enabled bool
}

func (c *testConfiguredChannel) ID() string { return c.id }

func (c *testConfiguredChannel) Name() string { return c.name }

func (c *testConfiguredChannel) Start(_ context.Context) error { return nil }

func (c *testConfiguredChannel) Stop(_ context.Context) error { return nil }

func (c *testConfiguredChannel) IsEnabled() bool { return c.enabled }

func (c *testConfiguredChannel) SendMessage(_ context.Context, _ *bus.Message) error { return nil }

type testProbeChannel struct {
	id        string
	name      string
	enabled   bool
	probeErr  error
	probeRuns int
}

func (c *testProbeChannel) ID() string { return c.id }

func (c *testProbeChannel) Name() string { return c.name }

func (c *testProbeChannel) Start(_ context.Context) error { return nil }

func (c *testProbeChannel) Stop(_ context.Context) error { return nil }

func (c *testProbeChannel) IsEnabled() bool { return c.enabled }

func (c *testProbeChannel) SendMessage(_ context.Context, _ *bus.Message) error { return nil }

func (c *testProbeChannel) HealthCheck(_ context.Context) error {
	c.probeRuns++
	return c.probeErr
}

func TestHandleTestChannelReturnsConfiguredWithoutProbe(t *testing.T) {
	log := newTestLogger(t)
	messageBus := bus.NewLocalBus(log, 8)
	manager := channels.NewManager(log, messageBus)
	ch := &testConfiguredChannel{id: "stub", name: "Stub", enabled: true}
	if err := manager.Register(ch); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	s := &Server{logger: log, channels: manager}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/stub/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/channels/:name/test")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "stub"}})

	if err := s.handleTestChannel(c); err != nil {
		t.Fatalf("handleTestChannel failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if payload["status"] != "configured" {
		t.Fatalf("expected configured status, got %#v", payload["status"])
	}
	if payload["reachable"] != false {
		t.Fatalf("expected reachable=false, got %#v", payload["reachable"])
	}
}

func TestHandleTestChannelUsesHealthCheckFailure(t *testing.T) {
	log := newTestLogger(t)
	messageBus := bus.NewLocalBus(log, 8)
	manager := channels.NewManager(log, messageBus)
	ch := &testProbeChannel{
		id:       "probe",
		name:     "Probe",
		enabled:  true,
		probeErr: errors.New("upstream auth failed"),
	}
	if err := manager.Register(ch); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	s := &Server{logger: log, channels: manager}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/probe/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/channels/:name/test")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "probe"}})

	if err := s.handleTestChannel(c); err != nil {
		t.Fatalf("handleTestChannel failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ch.probeRuns != 1 {
		t.Fatalf("expected one probe run, got %d", ch.probeRuns)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if payload["status"] != "unreachable" {
		t.Fatalf("expected unreachable status, got %#v", payload["status"])
	}
	if payload["reachable"] != false {
		t.Fatalf("expected reachable=false, got %#v", payload["reachable"])
	}
	if payload["error"] != "upstream auth failed" {
		t.Fatalf("expected error to round-trip, got %#v", payload["error"])
	}
}

func TestHandleTestChannelUsesHealthCheckSuccess(t *testing.T) {
	log := newTestLogger(t)
	messageBus := bus.NewLocalBus(log, 8)
	manager := channels.NewManager(log, messageBus)
	ch := &testProbeChannel{
		id:      "probe",
		name:    "Probe",
		enabled: true,
	}
	if err := manager.Register(ch); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	s := &Server{logger: log, channels: manager}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/probe/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/channels/:name/test")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "probe"}})

	if err := s.handleTestChannel(c); err != nil {
		t.Fatalf("handleTestChannel failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ch.probeRuns != 1 {
		t.Fatalf("expected one probe run, got %d", ch.probeRuns)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if payload["status"] != "ok" {
		t.Fatalf("expected ok status, got %#v", payload["status"])
	}
	if payload["reachable"] != true {
		t.Fatalf("expected reachable=true, got %#v", payload["reachable"])
	}
}
