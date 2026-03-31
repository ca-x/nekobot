package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/accountbindings"
	"nekobot/pkg/channelaccounts"
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
