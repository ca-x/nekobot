package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/toolsessions"
)

func TestHandleResolveExternalAgentSessionCreatesSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
	}
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+cfg.WorkspacePath()+`"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/external-agents/resolve-session")
	if err := server.handleResolveExternalAgentSession(ctx); err != nil {
		t.Fatalf("resolve handler failed: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var payload struct {
		Created bool                 `json:"created"`
		Session toolsessions.Session `json:"session"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if !payload.Created {
		t.Fatal("expected created=true for first resolve")
	}
	if payload.Session.Owner != "alice" {
		t.Fatalf("expected owner alice, got %q", payload.Session.Owner)
	}
	if payload.Session.Source != toolsessions.SourceAgent {
		t.Fatalf("expected source agent, got %q", payload.Session.Source)
	}
}

func TestHandleResolveExternalAgentSessionReusesExistingSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}

	first, created, err := extMgr.ResolveSession(context.Background(), externalagent.SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: cfg.WorkspacePath(),
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected initial session creation")
	}

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
	}
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+cfg.WorkspacePath()+`"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/external-agents/resolve-session")
	if err := server.handleResolveExternalAgentSession(ctx); err != nil {
		t.Fatalf("resolve handler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Created bool                 `json:"created"`
		Session toolsessions.Session `json:"session"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Created {
		t.Fatal("expected created=false for reused session")
	}
	if payload.Session.ID != first.ID {
		t.Fatalf("expected reused session %q, got %q", first.ID, payload.Session.ID)
	}
}
