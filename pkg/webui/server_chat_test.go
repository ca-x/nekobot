package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/session"
)

func TestPersistChatRoutingPersistsModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Agents.Defaults.Model = "old-model"
	cfg.Agents.Defaults.Fallback = []string{"openai"}
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic"},
		{Name: "openai", ProviderKind: "openai"},
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
	if got := ctx.Custom["runtime_id"]; got != "runtime-explicit" {
		t.Fatalf("unexpected runtime id: %#v", got)
	}
}
