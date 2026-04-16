package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/accountbindings"
	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/session"
	"nekobot/pkg/tasks"
)

func TestChatRouteStateJSONIncludesContextPressureFields(t *testing.T) {
	state := chatRouteState{
		RequestedProvider: "openai",
		RequestedModel:    "gpt-5.4",
		RequestedFallback: []string{"anthropic"},
		ResolvedOrder:     []string{"openai", "anthropic"},
		ActualProvider:    "openai",
		ActualModel:       "gpt-5.4",
		Preflight: &chatRoutePreflightState{
			Action:        "consider_compaction",
			Applied:       false,
			BudgetStatus:  "warning",
			BudgetReasons: []string{"Approximate prompt chars are near the configured max tokens budget."},
			Compaction: chatRouteCompactionState{
				Recommended: true,
				Strategy:    "compress_memory",
			},
		},
		ContextBudgetStatus:   "warning",
		ContextBudgetReasons:  []string{"Approximate prompt chars are near the configured max tokens budget."},
		CompactionRecommended: true,
		CompactionStrategy:    "compress_memory",
		RuntimeID:             "runtime-1",
	}

	payload, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal route state failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal route state failed: %v", err)
	}

	if decoded["context_budget_status"] != "warning" {
		t.Fatalf("unexpected context_budget_status: %+v", decoded["context_budget_status"])
	}
	if decoded["compaction_strategy"] != "compress_memory" {
		t.Fatalf("unexpected compaction_strategy: %+v", decoded["compaction_strategy"])
	}
	if decoded["compaction_recommended"] != true {
		t.Fatalf("unexpected compaction_recommended: %+v", decoded["compaction_recommended"])
	}
	preflight, ok := decoded["preflight"].(map[string]any)
	if !ok {
		t.Fatalf("expected preflight object, got %+v", decoded["preflight"])
	}
	if preflight["budget_status"] != "warning" {
		t.Fatalf("unexpected preflight budget_status: %+v", preflight["budget_status"])
	}
	if preflight["action"] != "consider_compaction" {
		t.Fatalf("unexpected preflight action: %+v", preflight["action"])
	}
	if preflight["applied"] != false {
		t.Fatalf("unexpected preflight applied flag: %+v", preflight["applied"])
	}
	compaction, ok := preflight["compaction"].(map[string]any)
	if !ok {
		t.Fatalf("expected preflight compaction object, got %+v", preflight["compaction"])
	}
	if compaction["strategy"] != "compress_memory" {
		t.Fatalf("unexpected preflight compaction strategy: %+v", compaction["strategy"])
	}
	reasons, ok := decoded["context_budget_reasons"].([]any)
	if !ok {
		t.Fatalf("expected context_budget_reasons array, got %+v", decoded["context_budget_reasons"])
	}
	if !reflect.DeepEqual(reasons, []any{"Approximate prompt chars are near the configured max tokens budget."}) {
		t.Fatalf("unexpected context_budget_reasons: %+v", reasons)
	}
}

func TestPersistChatRoutingPersistsModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Agents.Defaults.Model = "old-model"
	cfg.Agents.Defaults.Fallback = []string{"openai"}
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic", APIKey: "anthropic-key"},
		{Name: "openai", ProviderKind: "openai", APIKey: "openai-key"},
	}

	s := &Server{config: cfg}
	if err := s.persistChatRouting("anthropic", "new-model", []string{"openai"}); err != nil {
		t.Fatalf("persistChatRouting failed: %v", err)
	}

	if cfg.Agents.Defaults.Model != "new-model" {
		t.Fatalf("expected model to persist, got %q", cfg.Agents.Defaults.Model)
	}
}

func TestHandleUndoChatSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Sessions.Sources.WebUI = true
	cfg.Undo.Enabled = true
	cfg.Undo.MaxTurns = 10

	sessionMgr := session.NewManager(t.TempDir(), cfg.Sessions)
	snapshotMgr := session.NewSnapshotManager(t.TempDir(), cfg.Undo)
	sess, err := sessionMgr.GetWithSource("webui-chat:tester", session.SourceWebUI)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	store := snapshotMgr.GetStore("webui-chat:tester")
	first := []session.MessageSnapshot{{Role: "user", Content: "first"}}
	second := []session.MessageSnapshot{{Role: "user", Content: "first"}, {Role: "assistant", Content: "reply-1"}}
	third := []session.MessageSnapshot{{Role: "user", Content: "first"}, {Role: "assistant", Content: "reply-1"}, {Role: "user", Content: "second"}}
	if err := store.SaveSnapshot(first, ""); err != nil {
		t.Fatalf("SaveSnapshot first failed: %v", err)
	}
	if err := store.SaveSnapshot(second, ""); err != nil {
		t.Fatalf("SaveSnapshot second failed: %v", err)
	}
	if err := store.SaveSnapshot(third, ""); err != nil {
		t.Fatalf("SaveSnapshot third failed: %v", err)
	}
	sess.ReplaceMessages(session.MessageSnapshotsToMessages(third))

	s := &Server{
		config:      cfg,
		sessionMgr:  sessionMgr,
		snapshotMgr: snapshotMgr,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/session/webui-chat%3Atester/undo", strings.NewReader(`{"steps":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/api/chat/session/:id/undo")
	ctx.SetPathValues(echo.PathValues{{Name: "id", Value: "webui-chat:tester"}})

	if err := s.handleUndoChatSession(ctx); err != nil {
		t.Fatalf("handleUndoChatSession failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		UndoneSteps    int `json:"undone_steps"`
		RemainingTurns int `json:"remaining_turns"`
		MessageCount   int `json:"message_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal undo payload failed: %v", err)
	}
	if payload.UndoneSteps != 2 || payload.RemainingTurns != 1 || payload.MessageCount != 1 {
		t.Fatalf("unexpected undo payload: %+v", payload)
	}

	messages := sess.GetMessages()
	if len(messages) != 1 || messages[0].Content != "first" {
		t.Fatalf("session not rolled back: %#v", messages)
	}
}

func TestClearChatSessionRemovesUndoSnapshots(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Sessions.Sources.WebUI = true
	cfg.Undo.Enabled = true
	cfg.Undo.MaxTurns = 10

	sessionMgr := session.NewManager(t.TempDir(), cfg.Sessions)
	snapshotMgr := session.NewSnapshotManager(t.TempDir(), cfg.Undo)
	sess, err := sessionMgr.GetWithSource("webui-chat:tester", session.SourceWebUI)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	store := snapshotMgr.GetStore("webui-chat:tester")
	first := []session.MessageSnapshot{{Role: "user", Content: "first"}}
	second := []session.MessageSnapshot{{Role: "user", Content: "first"}, {Role: "assistant", Content: "reply-1"}}
	if err := store.SaveSnapshot(first, ""); err != nil {
		t.Fatalf("SaveSnapshot first failed: %v", err)
	}
	if err := store.SaveSnapshot(second, ""); err != nil {
		t.Fatalf("SaveSnapshot second failed: %v", err)
	}
	sess.ReplaceMessages(session.MessageSnapshotsToMessages(second))

	s := &Server{
		config:      cfg,
		sessionMgr:  sessionMgr,
		snapshotMgr: snapshotMgr,
	}

	if err := s.clearChatSession("webui-chat:tester"); err != nil {
		t.Fatalf("clearChatSession failed: %v", err)
	}

	cleared, err := sessionMgr.GetExisting("webui-chat:tester")
	if err != nil {
		t.Fatalf("GetExisting failed: %v", err)
	}
	if len(cleared.GetMessages()) != 0 {
		t.Fatalf("expected cleared session to have no messages, got %#v", cleared.GetMessages())
	}

	reloadedStore := snapshotMgr.GetStore("webui-chat:tester")
	if err := reloadedStore.LoadSnapshots(); err != nil {
		t.Fatalf("LoadSnapshots failed: %v", err)
	}
	if reloadedStore.CanUndo() {
		t.Fatalf("expected undo snapshots to be cleared")
	}
	if got := reloadedStore.GetTurnCount(); got != 0 {
		t.Fatalf("expected zero snapshots after clear, got %d", got)
	}
}

func TestBuildWebUIChatPromptContextIncludesExplicitRuntimeID(t *testing.T) {
	ctx := buildWebUIChatPromptContext(
		"webui-chat:alice",
		"alice",
		"openai",
		"gpt-5",
		[]string{"anthropic"},
		[]string{"prompt-runtime"},
		"runtime-explicit",
	)

	if got := ctx.SessionID; got != "webui-chat:alice" {
		t.Fatalf("unexpected session id: %q", got)
	}
	if got := ctx.RequestedProvider; got != "openai" {
		t.Fatalf("unexpected provider: %q", got)
	}
	if got := ctx.RequestedModel; got != "gpt-5" {
		t.Fatalf("unexpected model: %q", got)
	}
	if got := ctx.ExplicitPromptIDs; len(got) != 1 || got[0] != "prompt-runtime" {
		t.Fatalf("unexpected explicit prompt ids: %#v", got)
	}
	if got := ctx.Custom["runtime_id"]; got != "runtime-explicit" {
		t.Fatalf("unexpected runtime id: %#v", got)
	}
}

func TestWebUIClientChatSessionIDHidesInternalUsernameSuffix(t *testing.T) {
	if got := webUIClientChatSessionID(""); got != "webui-chat" {
		t.Fatalf("unexpected base client session id: %q", got)
	}
	if got := webUIClientChatSessionID("runtime-ops"); got != "route:runtime-ops:webui-chat" {
		t.Fatalf("unexpected runtime client session id: %q", got)
	}
}

func TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults(t *testing.T) {
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
	runtimeItem, err := runtimeMgr.Create(t.Context(), runtimeagents.AgentRuntime{
		Name:        "ops-escalation",
		DisplayName: "Ops Escalation",
		Enabled:     true,
		Provider:    "anthropic",
		Model:       "claude-sonnet-4",
		PromptID:    "prompt-runtime-ops",
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	websocketAccount, err := accountMgr.Create(t.Context(), channelaccounts.ChannelAccount{
		ChannelType: "websocket",
		AccountKey:  "default",
		DisplayName: "Web Chat",
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled": true,
		},
	})
	if err != nil {
		t.Fatalf("create websocket account: %v", err)
	}
	if _, err := bindingMgr.Create(t.Context(), accountbindings.AccountBinding{
		ChannelAccountID: websocketAccount.ID,
		AgentRuntimeID:   runtimeItem.ID,
		BindingMode:      accountbindings.ModeSingleAgent,
		Enabled:          true,
	}); err != nil {
		t.Fatalf("create websocket binding: %v", err)
	}

	s := &Server{runtimeMgr: runtimeMgr, accountMgr: accountMgr, bindingMgr: bindingMgr}
	provider, model, fallback, explicitPromptIDs, err := s.resolveWebUIRuntimeSelection(
		t.Context(),
		runtimeItem.ID,
		"openai",
		"gpt-5",
		[]string{"backup"},
	)
	if err != nil {
		t.Fatalf("resolveWebUIRuntimeSelection failed: %v", err)
	}
	if provider != "anthropic" {
		t.Fatalf("unexpected provider: %q", provider)
	}
	if model != "claude-sonnet-4" {
		t.Fatalf("unexpected model: %q", model)
	}
	if len(fallback) != 0 {
		t.Fatalf("expected runtime selection to clear fallback, got %#v", fallback)
	}
	if len(explicitPromptIDs) != 1 || explicitPromptIDs[0] != "prompt-runtime-ops" {
		t.Fatalf("unexpected explicit prompt ids: %#v", explicitPromptIDs)
	}
}

func TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime(t *testing.T) {
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
	runtimeItem, err := runtimeMgr.Create(t.Context(), runtimeagents.AgentRuntime{
		Name:        "ops-escalation",
		DisplayName: "Ops Escalation",
		Enabled:     true,
		Provider:    "anthropic",
		Model:       "claude-sonnet-4",
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	s := &Server{runtimeMgr: runtimeMgr, accountMgr: accountMgr, bindingMgr: bindingMgr}
	_, _, _, _, err = s.resolveWebUIRuntimeSelection(
		t.Context(),
		runtimeItem.ID,
		"",
		"",
		nil,
	)
	if err == nil {
		t.Fatal("expected unbound runtime validation error")
	}
	if !strings.Contains(err.Error(), "not available for websocket chat") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveWebUIRuntimeSelectionFallsBackToRequestedRoute(t *testing.T) {
	s := &Server{}
	provider, model, fallback, explicitPromptIDs, err := s.resolveWebUIRuntimeSelection(
		t.Context(),
		"",
		"openai",
		"gpt-5",
		[]string{"anthropic"},
	)
	if err != nil {
		t.Fatalf("resolveWebUIRuntimeSelection failed: %v", err)
	}
	if provider != "openai" {
		t.Fatalf("unexpected provider: %q", provider)
	}
	if model != "gpt-5" {
		t.Fatalf("unexpected model: %q", model)
	}
	if len(fallback) != 1 || fallback[0] != "anthropic" {
		t.Fatalf("unexpected fallback: %#v", fallback)
	}
	if len(explicitPromptIDs) != 0 {
		t.Fatalf("expected no explicit prompt ids, got %#v", explicitPromptIDs)
	}
}

func TestHandleChatWSQueuesDaemonTaskForDaemonBackedRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer func() { _ = client.Close() }()

	runtimeMgr, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	runtimeItem, err := runtimeMgr.Create(t.Context(), runtimeagents.AgentRuntime{
		Name:        "daemon-codex",
		DisplayName: "Daemon Codex",
		Enabled:     true,
		Provider:    "daemon",
		Model:       "daemon-runtime",
		Policy: map[string]interface{}{
			"daemon_enabled":      true,
			"daemon_machine_id":   "machine-a",
			"daemon_workspace_id": "workspace-a",
		},
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	ag, err := agent.New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, client, nil)
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	s := &Server{config: cfg, logger: log, runtimeMgr: runtimeMgr, agent: ag, sessionMgr: session.NewManager(t.TempDir(), cfg.Sessions)}
	ctx := context.Background()
	_, reply, err := s.handleDaemonRuntimeChatMessage(ctx, "alice", runtimeItem.ID, "route:"+runtimeItem.ID+":webui-chat", "run tests")
	if err != nil {
		t.Fatalf("handleDaemonRuntimeChatMessage failed: %v", err)
	}
	if !strings.Contains(reply, "Daemon task queued") {
		t.Fatalf("unexpected daemon reply: %q", reply)
	}
	items := ag.TaskService().List()
	if len(items) != 1 || items[0].Type != tasks.TypeRemoteAgent {
		t.Fatalf("expected one remote-agent task, got %+v", items)
	}
	if items[0].Metadata["machine_id"] != "machine-a" {
		t.Fatalf("expected machine-a metadata, got %+v", items[0].Metadata)
	}
}
