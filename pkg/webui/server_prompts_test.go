package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/inboundrouter"
	"nekobot/pkg/prompts"
	"nekobot/pkg/session"
)

func TestPromptHandlers_CRUDAndResolve(t *testing.T) {
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

	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	s := &Server{
		config:    cfg,
		logger:    log,
		prompts:   promptMgr,
		entClient: client,
	}
	e := echo.New()

	createReq := httptest.NewRequest(http.MethodPost, "/api/prompts", strings.NewReader(`{
		"key":"wechat-ops",
		"name":"WeChat Ops",
		"description":"system policy",
		"mode":"system",
		"template":"channel={{channel.id}}",
		"enabled":true,
		"tags":["ops","wechat"]
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	if err := s.handleCreatePrompt(createCtx); err != nil {
		t.Fatalf("handleCreatePrompt failed: %v", err)
	}
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, createRec.Code, createRec.Body.String())
	}

	var created prompts.Prompt
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created prompt: %v", err)
	}
	if created.ID == "" || created.Key != "wechat-ops" {
		t.Fatalf("unexpected created prompt: %+v", created)
	}

	bindingReq := httptest.NewRequest(http.MethodPost, "/api/prompts/bindings", strings.NewReader(`{
		"scope":"channel",
		"target":"wechat",
		"prompt_id":"`+created.ID+`",
		"enabled":true,
		"priority":50
	}`))
	bindingReq.Header.Set("Content-Type", "application/json")
	bindingRec := httptest.NewRecorder()
	bindingCtx := e.NewContext(bindingReq, bindingRec)
	if err := s.handleCreatePromptBinding(bindingCtx); err != nil {
		t.Fatalf("handleCreatePromptBinding failed: %v", err)
	}
	if bindingRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, bindingRec.Code, bindingRec.Body.String())
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/prompts/resolve", strings.NewReader(`{
		"channel":"wechat",
		"session_id":"s-1",
		"user_id":"u-1",
		"username":"alice"
	}`))
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	resolveCtx := e.NewContext(resolveReq, resolveRec)
	if err := s.handleResolvePrompts(resolveCtx); err != nil {
		t.Fatalf("handleResolvePrompts failed: %v", err)
	}
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resolveRec.Code, resolveRec.Body.String())
	}

	var resolved prompts.ResolvedPromptSet
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("unmarshal resolved prompts: %v", err)
	}
	if !strings.Contains(resolved.SystemText, "channel=wechat") {
		t.Fatalf("expected rendered system prompt, got %+v", resolved)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/prompts/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/prompts/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := s.handleDeletePrompt(deleteCtx); err != nil {
		t.Fatalf("handleDeletePrompt failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, deleteRec.Code, deleteRec.Body.String())
	}

	bindings, err := promptMgr.ListBindings(context.Background(), "", "")
	if err != nil {
		t.Fatalf("list bindings after delete: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected prompt bindings to be removed with prompt delete, got %+v", bindings)
	}
}

func TestPromptHandlers_ContextSourcesPreview(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.MCPServers = []config.MCPServerConfig{{Name: "filesystem", Transport: "stdio"}}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}
	ag, err := agent.New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, client, promptMgr)
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	s := &Server{
		config:    cfg,
		logger:    log,
		prompts:   promptMgr,
		agent:     ag,
		entClient: client,
	}
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/prompts/context-sources", strings.NewReader(`{
		"channel":"wechat",
		"session_id":"s-1",
		"requested_provider":"openai",
		"requested_model":"gpt-5.4",
		"custom":{"runtime_id":"runtime-a"},
		"user_message":"hello"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handlePreviewContextSources(ctx); err != nil {
		t.Fatalf("handlePreviewContextSources failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Preflight struct {
			Action        string   `json:"action"`
			BudgetStatus  string   `json:"budget_status"`
			BudgetReasons []string `json:"budget_reasons"`
			Compaction    struct {
				Recommended         bool   `json:"recommended"`
				Strategy            string `json:"strategy"`
				EstimatedCharsSaved int    `json:"estimated_chars_saved"`
			} `json:"compaction"`
		} `json:"preflight"`
		BudgetStatus  string   `json:"budget_status"`
		BudgetReasons []string `json:"budget_reasons"`
		Compaction    struct {
			Recommended         bool   `json:"recommended"`
			Strategy            string `json:"strategy"`
			EstimatedCharsSaved int    `json:"estimated_chars_saved"`
		} `json:"compaction"`
		Footprint struct {
			TotalChars       int `json:"total_chars"`
			SystemChars      int `json:"system_chars"`
			FinalUserChars   int `json:"final_user_chars"`
			MemoryLimitChars int `json:"memory_limit_chars"`
		} `json:"footprint"`
		Warnings []string `json:"warnings"`
		Sources  []struct {
			Kind string `json:"kind"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal context sources preview: %v", err)
	}
	if len(payload.Sources) == 0 {
		t.Fatalf("expected non-empty sources payload")
	}
	if payload.Footprint.TotalChars <= 0 || payload.Footprint.SystemChars <= 0 || payload.Footprint.FinalUserChars <= 0 {
		t.Fatalf("expected populated footprint payload, got %+v", payload.Footprint)
	}
	if len(payload.Warnings) != 0 {
		t.Fatalf("expected no warnings for simple payload, got %+v", payload.Warnings)
	}
	if payload.BudgetStatus != "ok" {
		t.Fatalf("expected ok budget status, got %+v", payload)
	}
	if payload.Preflight.BudgetStatus != payload.BudgetStatus {
		t.Fatalf("expected preflight budget status to mirror budget status, got %+v", payload.Preflight)
	}
	if payload.Preflight.Action != "proceed" {
		t.Fatalf("expected proceed preflight action, got %+v", payload.Preflight)
	}
	if len(payload.BudgetReasons) != 0 {
		t.Fatalf("expected no budget reasons for simple payload, got %+v", payload.BudgetReasons)
	}
	if payload.Preflight.Compaction.Recommended != payload.Compaction.Recommended {
		t.Fatalf("expected preflight compaction to mirror legacy fields, got %+v vs %+v", payload.Preflight.Compaction, payload.Compaction)
	}
	if payload.Compaction.Recommended {
		t.Fatalf("expected no compaction recommendation for simple payload, got %+v", payload.Compaction)
	}
}

func TestSessionPromptHandlers_ResolveAliasAndCleanup(t *testing.T) {
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

	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}
	authProfile := &config.AuthProfile{
		UserID:   "u-1",
		Username: "alice",
		Role:     "admin",
	}

	s := &Server{
		config:     cfg,
		logger:     log,
		prompts:    promptMgr,
		entClient:  client,
		sessionMgr: session.NewManager(t.TempDir(), cfg.Sessions),
	}
	token, err := s.generateToken(authProfile)
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}
	e := echo.New()

	systemPrompt, err := promptMgr.CreatePrompt(context.Background(), prompts.Prompt{
		Key:      "system-default",
		Name:     "System Default",
		Mode:     prompts.ModeSystem,
		Template: "always on",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create system prompt: %v", err)
	}
	userPrompt, err := promptMgr.CreatePrompt(context.Background(), prompts.Prompt{
		Key:      "user-default",
		Name:     "User Default",
		Mode:     prompts.ModeUser,
		Template: "summarize first",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create user prompt: %v", err)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/chat/prompts/session/webui-chat", strings.NewReader(`{
		"system_prompt_ids":["`+systemPrompt.ID+`"],
		"user_prompt_ids":["`+userPrompt.ID+`"]
	}`))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	putCtx := e.NewContext(putReq, putRec)
	putCtx.SetPath("/api/chat/prompts/session/:id")
	putCtx.SetPathValues(echo.PathValues{{Name: "id", Value: "webui-chat"}})
	if err := s.handlePutChatSessionPrompts(putCtx); err != nil {
		t.Fatalf("handlePutChatSessionPrompts failed: %v", err)
	}
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/chat/prompts/session/webui-chat", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetPath("/api/chat/prompts/session/:id")
	getCtx.SetPathValues(echo.PathValues{{Name: "id", Value: "webui-chat"}})
	if err := s.handleGetChatSessionPrompts(getCtx); err != nil {
		t.Fatalf("handleGetChatSessionPrompts failed: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, getRec.Code, getRec.Body.String())
	}

	var bindingSet prompts.SessionBindingSet
	if err := json.Unmarshal(getRec.Body.Bytes(), &bindingSet); err != nil {
		t.Fatalf("unmarshal binding set: %v", err)
	}
	if len(bindingSet.SystemPromptIDs) != 1 || bindingSet.SystemPromptIDs[0] != systemPrompt.ID {
		t.Fatalf("unexpected system prompt ids: %+v", bindingSet.SystemPromptIDs)
	}
	if len(bindingSet.UserPromptIDs) != 1 || bindingSet.UserPromptIDs[0] != userPrompt.ID {
		t.Fatalf("unexpected user prompt ids: %+v", bindingSet.UserPromptIDs)
	}

	sessionID := webUIChatSessionID(authProfile.Username)
	if _, err := s.sessionMgr.GetWithSource(sessionID, session.SourceWebUI); err != nil {
		t.Fatalf("create session: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+sessionID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/sessions/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: sessionID}})
	if err := s.handleDeleteSession(deleteCtx); err != nil {
		t.Fatalf("handleDeleteSession failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, deleteRec.Code, deleteRec.Body.String())
	}

	remaining, err := promptMgr.GetSessionBindingSet(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get session binding set after session delete: %v", err)
	}
	if len(remaining.Bindings) != 0 {
		t.Fatalf("expected session prompt bindings to be cleaned, got %+v", remaining.Bindings)
	}
}

func TestResolveWebUIChatSessionAlias_UsesServerGeneratedToken(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		logger:    log,
		entClient: client,
	}

	token, err := s.generateToken(&config.AuthProfile{
		UserID:   "u-1",
		Username: "alice",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/prompts/session/webui-chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	sessionID, err := s.resolveWebUIChatSessionAlias(ctx, "webui-chat")
	if err != nil {
		t.Fatalf("resolveWebUIChatSessionAlias failed: %v", err)
	}
	if sessionID != webUIChatSessionID("alice") {
		t.Fatalf("expected alias to resolve to %q, got %q", webUIChatSessionID("alice"), sessionID)
	}
}

func TestResolveWebUIChatSessionAlias_UsesRuntimeAlias(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		logger:    log,
		entClient: client,
	}

	token, err := s.generateToken(&config.AuthProfile{
		UserID:   "u-1",
		Username: "alice",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/prompts/session/route:runtime-ops:webui-chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	sessionID, err := s.resolveWebUIChatSessionAlias(
		ctx,
		inboundrouter.SessionPrefix+":runtime-ops:webui-chat",
	)
	if err != nil {
		t.Fatalf("resolveWebUIChatSessionAlias failed: %v", err)
	}

	want := inboundrouter.SessionPrefix + ":runtime-ops:" + webUIChatSessionID("alice")
	if sessionID != want {
		t.Fatalf("expected alias to resolve to %q, got %q", want, sessionID)
	}
}

func TestSessionPromptHandlers_RuntimeAliasAndCleanup(t *testing.T) {
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

	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	authProfile := &config.AuthProfile{
		UserID:   "u-1",
		Username: "alice",
		Role:     "admin",
	}

	s := &Server{
		config:     cfg,
		logger:     log,
		prompts:    promptMgr,
		entClient:  client,
		sessionMgr: session.NewManager(t.TempDir(), cfg.Sessions),
	}
	token, err := s.generateToken(authProfile)
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}
	e := echo.New()

	systemPrompt, err := promptMgr.CreatePrompt(context.Background(), prompts.Prompt{
		Key:      "runtime-system",
		Name:     "Runtime System",
		Mode:     prompts.ModeSystem,
		Template: "runtime on",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create system prompt: %v", err)
	}

	aliasID := inboundrouter.SessionPrefix + ":runtime-ops:webui-chat"
	putReq := httptest.NewRequest(http.MethodPut, "/api/chat/prompts/session/"+aliasID, strings.NewReader(`{
		"system_prompt_ids":["`+systemPrompt.ID+`"],
		"user_prompt_ids":[]
	}`))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	putCtx := e.NewContext(putReq, putRec)
	putCtx.SetPath("/api/chat/prompts/session/:id")
	putCtx.SetPathValues(echo.PathValues{{Name: "id", Value: aliasID}})
	if err := s.handlePutChatSessionPrompts(putCtx); err != nil {
		t.Fatalf("handlePutChatSessionPrompts failed: %v", err)
	}
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, putRec.Code, putRec.Body.String())
	}

	resolvedSessionID := inboundrouter.SessionPrefix + ":runtime-ops:" + webUIChatSessionID(authProfile.Username)
	bindingSet, err := promptMgr.GetSessionBindingSet(context.Background(), resolvedSessionID)
	if err != nil {
		t.Fatalf("get session binding set failed: %v", err)
	}
	if len(bindingSet.SystemPromptIDs) != 1 || bindingSet.SystemPromptIDs[0] != systemPrompt.ID {
		t.Fatalf("unexpected system prompt ids: %+v", bindingSet.SystemPromptIDs)
	}

	if _, err := s.sessionMgr.GetWithSource(resolvedSessionID, session.SourceWebUI); err != nil {
		t.Fatalf("create runtime-scoped session: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+resolvedSessionID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/sessions/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: resolvedSessionID}})
	if err := s.handleDeleteSession(deleteCtx); err != nil {
		t.Fatalf("handleDeleteSession failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, deleteRec.Code, deleteRec.Body.String())
	}

	remaining, err := promptMgr.GetSessionBindingSet(context.Background(), resolvedSessionID)
	if err != nil {
		t.Fatalf("get session binding set after session delete: %v", err)
	}
	if len(remaining.Bindings) != 0 {
		t.Fatalf("expected runtime session prompt bindings to be cleaned, got %+v", remaining.Bindings)
	}
}

func TestPromptHandlers_ResolveHonorsScopeOverrideAndTemplateContext(t *testing.T) {
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

	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	s := &Server{
		config:    cfg,
		logger:    log,
		prompts:   promptMgr,
		entClient: client,
	}
	e := echo.New()

	promptItem, err := promptMgr.CreatePrompt(context.Background(), prompts.Prompt{
		Key:      "ops",
		Name:     "Ops",
		Mode:     prompts.ModeSystem,
		Template: "scope={{channel.id}} provider={{route.provider}} custom={{custom.role}} workspace={{workspace.path}}",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	for _, binding := range []prompts.Binding{
		{Scope: prompts.ScopeGlobal, PromptID: promptItem.ID, Enabled: true, Priority: 100},
		{Scope: prompts.ScopeChannel, Target: "wechat", PromptID: promptItem.ID, Enabled: true, Priority: 50},
	} {
		if _, err := promptMgr.CreateBinding(context.Background(), binding); err != nil {
			t.Fatalf("create binding %+v: %v", binding, err)
		}
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/prompts/resolve", strings.NewReader(`{
		"channel":"wechat",
		"session_id":"s-1",
		"user_id":"u-1",
		"username":"alice",
		"requested_provider":"openai",
		"workspace":"`+cfg.WorkspacePath()+`",
		"custom":{"role":"ops"}
	}`))
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	resolveCtx := e.NewContext(resolveReq, resolveRec)
	if err := s.handleResolvePrompts(resolveCtx); err != nil {
		t.Fatalf("handleResolvePrompts failed: %v", err)
	}
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resolveRec.Code, resolveRec.Body.String())
	}

	var resolved prompts.ResolvedPromptSet
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("unmarshal resolved prompts: %v", err)
	}
	if len(resolved.Applied) != 1 {
		t.Fatalf("expected one applied prompt after scope override, got %+v", resolved.Applied)
	}
	if resolved.Applied[0].Scope != prompts.ScopeChannel {
		t.Fatalf("expected channel scope to win, got %+v", resolved.Applied[0])
	}
	if !strings.Contains(resolved.SystemText, "scope=wechat") ||
		!strings.Contains(resolved.SystemText, "provider=openai") ||
		!strings.Contains(resolved.SystemText, "custom=ops") ||
		!strings.Contains(resolved.SystemText, "workspace="+cfg.WorkspacePath()) {
		t.Fatalf("unexpected resolved system text: %q", resolved.SystemText)
	}
}
