package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/accountbindings"
	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	pkgchannels "nekobot/pkg/channels"
	"nekobot/pkg/config"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/runtimetopology"
)

func TestRuntimeTopologyHandlers_CRUDAndSnapshot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	runtimeMgr, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}
	bindingMgr, err := accountbindings.NewManager(cfg, log, client, runtimeMgr, accountMgr)
	if err != nil {
		t.Fatalf("new binding manager: %v", err)
	}
	topologySvc, err := runtimetopology.NewService(runtimeMgr, accountMgr, bindingMgr)
	if err != nil {
		t.Fatalf("new topology service: %v", err)
	}

	s := &Server{
		config:      cfg,
		logger:      log,
		entClient:   client,
		runtimeMgr:  runtimeMgr,
		accountMgr:  accountMgr,
		bindingMgr:  bindingMgr,
		topologySvc: topologySvc,
	}
	e := echo.New()

	runtimeReq := httptest.NewRequest(http.MethodPost, "/api/runtime-agents", strings.NewReader(`{
		"name":"support-main",
		"display_name":"Support Main",
		"provider":"openai",
		"model":"gpt-5",
		"skills":["triage","reply"]
	}`))
	runtimeReq.Header.Set("Content-Type", "application/json")
	runtimeRec := httptest.NewRecorder()
	runtimeCtx := e.NewContext(runtimeReq, runtimeRec)
	if err := s.handleCreateRuntimeAgent(runtimeCtx); err != nil {
		t.Fatalf("handleCreateRuntimeAgent failed: %v", err)
	}
	if runtimeRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, runtimeRec.Code, runtimeRec.Body.String())
	}
	var runtimeItem runtimeagents.AgentRuntime
	if err := json.Unmarshal(runtimeRec.Body.Bytes(), &runtimeItem); err != nil {
		t.Fatalf("unmarshal runtime: %v", err)
	}

	accountReq := httptest.NewRequest(http.MethodPost, "/api/channel-accounts", strings.NewReader(`{
		"channel_type":"wechat",
		"account_key":"bot-a",
		"display_name":"Bot A",
		"config":{"bot_id":"wx-1"}
	}`))
	accountReq.Header.Set("Content-Type", "application/json")
	accountRec := httptest.NewRecorder()
	accountCtx := e.NewContext(accountReq, accountRec)
	if err := s.handleCreateChannelAccount(accountCtx); err != nil {
		t.Fatalf("handleCreateChannelAccount failed: %v", err)
	}
	if accountRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, accountRec.Code, accountRec.Body.String())
	}
	var accountItem channelaccounts.ChannelAccount
	if err := json.Unmarshal(accountRec.Body.Bytes(), &accountItem); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}

	bindingReq := httptest.NewRequest(http.MethodPost, "/api/account-bindings", strings.NewReader(`{
		"channel_account_id":"`+accountItem.ID+`",
		"agent_runtime_id":"`+runtimeItem.ID+`",
		"binding_mode":"single_agent",
		"enabled":true,
		"allow_public_reply":true
	}`))
	bindingReq.Header.Set("Content-Type", "application/json")
	bindingRec := httptest.NewRecorder()
	bindingCtx := e.NewContext(bindingReq, bindingRec)
	if err := s.handleCreateAccountBinding(bindingCtx); err != nil {
		t.Fatalf("handleCreateAccountBinding failed: %v", err)
	}
	if bindingRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, bindingRec.Code, bindingRec.Body.String())
	}

	topologyReq := httptest.NewRequest(http.MethodGet, "/api/runtime-topology", nil)
	topologyRec := httptest.NewRecorder()
	topologyCtx := e.NewContext(topologyReq, topologyRec)
	if err := s.handleGetRuntimeTopology(topologyCtx); err != nil {
		t.Fatalf("handleGetRuntimeTopology failed: %v", err)
	}
	if topologyRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, topologyRec.Code, topologyRec.Body.String())
	}

	var snapshot runtimetopology.Snapshot
	if err := json.Unmarshal(topologyRec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("unmarshal topology snapshot: %v", err)
	}
	if snapshot.Summary.RuntimeCount != 1 || snapshot.Summary.ChannelAccountCount != 1 || snapshot.Summary.BindingCount != 1 {
		t.Fatalf("unexpected topology summary: %+v", snapshot.Summary)
	}
	if len(snapshot.Bindings) != 1 || snapshot.Bindings[0].RuntimeName != "Support Main" {
		t.Fatalf("unexpected topology bindings: %+v", snapshot.Bindings)
	}
}

func TestHandleCreateChannelAccountRejectsEnabledWechatAccountWithoutCredentials(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

	s := &Server{
		config:     cfg,
		logger:     log,
		entClient:  client,
		accountMgr: accountMgr,
	}
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/channel-accounts", strings.NewReader(`{
		"channel_type":"wechat",
		"account_key":"bot-a@im.wechat",
		"display_name":"Bot A",
		"enabled":true,
		"config":{"enabled":true}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleCreateChannelAccount(ctx); err != nil {
		t.Fatalf("handleCreateChannelAccount failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "config.bot_token") {
		t.Fatalf("expected credentials validation error, got %s", rec.Body.String())
	}
}

func TestChannelAccountHandlersReloadRuntimeInstances(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}

	messageBus := bus.NewLocalBus(log, 8)
	manager := pkgchannels.NewManager(log, messageBus)

	s := &Server{
		config:     cfg,
		logger:     log,
		entClient:  client,
		accountMgr: accountMgr,
		channels:   manager,
		bus:        messageBus,
	}
	e := echo.New()

	createReq := httptest.NewRequest(http.MethodPost, "/api/channel-accounts", strings.NewReader(`{
		"channel_type":"gotify",
		"account_key":"alerts-a",
		"display_name":"Alerts A",
		"enabled":true,
		"config":{
			"enabled":true,
			"server_url":"https://gotify.example.com",
			"app_token":"token-1",
			"priority":5
		}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	if err := s.handleCreateChannelAccount(createCtx); err != nil {
		t.Fatalf("handleCreateChannelAccount failed: %v", err)
	}
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, createRec.Code, createRec.Body.String())
	}

	var created channelaccounts.ChannelAccount
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created account: %v", err)
	}

	channel, err := manager.GetChannel("gotify:alerts-a")
	if err != nil {
		t.Fatalf("expected runtime instance to be registered: %v", err)
	}
	if channel.ID() != "gotify:alerts-a" {
		t.Fatalf("unexpected channel id: %q", channel.ID())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/channel-accounts/"+created.ID, strings.NewReader(`{
		"channel_type":"gotify",
		"account_key":"alerts-a",
		"display_name":"Alerts A",
		"enabled":false,
		"config":{
			"enabled":false,
			"server_url":"https://gotify.example.com",
			"app_token":"token-1",
			"priority":5
		}
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.SetPath("/api/channel-accounts/:id")
	updateCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := s.handleUpdateChannelAccount(updateCtx); err != nil {
		t.Fatalf("handleUpdateChannelAccount failed: %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, updateRec.Code, updateRec.Body.String())
	}
	if _, err := manager.GetChannel("gotify:alerts-a"); err == nil {
		t.Fatalf("expected disabled runtime instance to be removed")
	}

	_, err = accountMgr.Update(context.Background(), created.ID, channelaccounts.ChannelAccount{
		ChannelType: "gotify",
		AccountKey:  "alerts-a",
		DisplayName: "Alerts A",
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled":    true,
			"server_url": "https://gotify.example.com",
			"app_token":  "token-1",
			"priority":   5,
		},
	})
	if err != nil {
		t.Fatalf("reactivate account directly: %v", err)
	}
	if err := s.reloadChannelsByType("gotify"); err != nil {
		t.Fatalf("reloadChannelsByType failed: %v", err)
	}
	if _, err := manager.GetChannel("gotify:alerts-a"); err != nil {
		t.Fatalf("expected reactivated runtime instance: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/channel-accounts/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/channel-accounts/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := s.handleDeleteChannelAccount(deleteCtx); err != nil {
		t.Fatalf("handleDeleteChannelAccount failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, deleteRec.Code, deleteRec.Body.String())
	}
	if _, err := manager.GetChannel("gotify:alerts-a"); err == nil {
		t.Fatalf("expected deleted runtime instance to be removed")
	}
}

func TestHandleDeleteRuntimeAgentRemovesBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	runtimeMgr, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}
	bindingMgr, err := accountbindings.NewManager(cfg, log, client, runtimeMgr, accountMgr)
	if err != nil {
		t.Fatalf("new binding manager: %v", err)
	}
	topologySvc, err := runtimetopology.NewService(runtimeMgr, accountMgr, bindingMgr)
	if err != nil {
		t.Fatalf("new topology service: %v", err)
	}

	runtimeItem, err := runtimeMgr.Create(context.Background(), runtimeagents.AgentRuntime{
		Name:        "ops-runtime",
		DisplayName: "Ops Runtime",
		Enabled:     true,
		Provider:    "openai",
		Model:       "gpt-5",
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	accountItem, err := accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "gotify",
		AccountKey:  "alerts-a",
		DisplayName: "Alerts A",
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled":    true,
			"server_url": "https://gotify.example.com",
			"app_token":  "token-1",
			"priority":   5,
		},
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := bindingMgr.Create(context.Background(), accountbindings.AccountBinding{
		ChannelAccountID: accountItem.ID,
		AgentRuntimeID:   runtimeItem.ID,
		BindingMode:      accountbindings.ModeSingleAgent,
		Enabled:          true,
		Priority:         100,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	s := &Server{
		config:      cfg,
		logger:      log,
		entClient:   client,
		runtimeMgr:  runtimeMgr,
		accountMgr:  accountMgr,
		bindingMgr:  bindingMgr,
		topologySvc: topologySvc,
	}
	e := echo.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/runtime-agents/"+runtimeItem.ID, nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/api/runtime-agents/:id")
	ctx.SetPathValues(echo.PathValues{{Name: "id", Value: runtimeItem.ID}})
	if err := s.handleDeleteRuntimeAgent(ctx); err != nil {
		t.Fatalf("handleDeleteRuntimeAgent failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	bindings, err := bindingMgr.List(context.Background())
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected bindings to be removed with runtime delete, got %+v", bindings)
	}

	snapshot, err := topologySvc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot topology: %v", err)
	}
	if snapshot.Summary.BindingCount != 0 {
		t.Fatalf("expected zero bindings in topology summary, got %+v", snapshot.Summary)
	}
	if len(snapshot.Bindings) != 0 {
		t.Fatalf("expected zero topology edges, got %+v", snapshot.Bindings)
	}
}
