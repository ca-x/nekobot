package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/permissionrules"
	"nekobot/pkg/process"
	"nekobot/pkg/providers"
	"nekobot/pkg/tasks"
	"nekobot/pkg/toolsessions"
)

func TestHandleResolveExternalAgentSessionCreatesSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "manual"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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
		Created      bool                 `json:"created"`
		Session      toolsessions.Session `json:"session"`
		LaunchPolicy struct {
			ToolName       string `json:"tool_name"`
			ApprovalMode   string `json:"approval_mode"`
			PermissionRule struct {
				Matched bool   `json:"matched"`
				Action  string `json:"action"`
				Source  string `json:"source"`
			} `json:"permission_rule"`
		} `json:"launch_policy"`
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
	if payload.LaunchPolicy.ToolName != "codex" {
		t.Fatalf("expected launch tool codex, got %q", payload.LaunchPolicy.ToolName)
	}
	if payload.LaunchPolicy.ApprovalMode != "manual" {
		t.Fatalf("expected approval mode manual, got %q", payload.LaunchPolicy.ApprovalMode)
	}
	if payload.LaunchPolicy.PermissionRule.Matched {
		t.Fatalf("expected no permission rule match, got %+v", payload.LaunchPolicy.PermissionRule)
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
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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

func TestHandleResolveExternalAgentSessionDefaultsBlankWorkspace(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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
		`{"agent_kind":"codex"}`,
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
	if payload.Session.Workdir != cfg.WorkspacePath() {
		t.Fatalf("expected workdir %q, got %q", cfg.WorkspacePath(), payload.Session.Workdir)
	}
}

func TestHandleResolveExternalAgentSessionResolvesRelativeWorkspace(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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
		`{"agent_kind":"codex","workspace":"projects/demo"}`,
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
	want := filepath.Join(cfg.WorkspacePath(), "projects", "demo")
	if payload.Session.Workdir != want {
		t.Fatalf("expected workdir %q, got %q", want, payload.Session.Workdir)
	}
}

func TestHandleResolveExternalAgentSessionRejectsWorkspaceOutsideConfiguredRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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
		`{"agent_kind":"codex","workspace":"`+filepath.Join(t.TempDir(), "outside")+`"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/external-agents/resolve-session")
	if err := server.handleResolveExternalAgentSession(ctx); err != nil {
		t.Fatalf("resolve handler failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "workspace must stay within configured workspace") {
		t.Fatalf("expected workspace restriction error, got %s", rec.Body.String())
	}
}

func TestHandleResolveExternalAgentSessionRejectsCommandThatDoesNotMatchTool(t *testing.T) {
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
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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
		`{"agent_kind":"codex","tool":"codex","command":"claude --print","workspace":"`+cfg.WorkspacePath()+`"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/external-agents/resolve-session")
	if err := server.handleResolveExternalAgentSession(ctx); err != nil {
		t.Fatalf("resolve handler failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "command must launch the selected tool") {
		t.Fatalf("expected command policy error, got %s", rec.Body.String())
	}
}

func TestHandleResolveExternalAgentSessionRejectsUnknownAgentKind(t *testing.T) {
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
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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
		`{"agent_kind":"unknown","workspace":"`+cfg.WorkspacePath()+`"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/external-agents/resolve-session")
	if err := server.handleResolveExternalAgentSession(ctx); err != nil {
		t.Fatalf("resolve handler failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported agent_kind") {
		t.Fatalf("expected unsupported agent kind error, got %s", rec.Body.String())
	}
}

func TestHandleResolveExternalAgentSessionRejectsToolMismatchForKnownAgentKind(t *testing.T) {
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
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
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
		`{"agent_kind":"codex","tool":"claude","workspace":"`+cfg.WorkspacePath()+`"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/external-agents/resolve-session")
	if err := server.handleResolveExternalAgentSession(ctx); err != nil {
		t.Fatalf("resolve handler failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tool must match agent_kind launcher") {
		t.Fatalf("expected tool mismatch error, got %s", rec.Body.String())
	}
}

func TestHandleResolveExternalAgentSessionIncludesMatchedPermissionRulePreview(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "prompt"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	rules, err := permissionrules.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new permission rule manager: %v", err)
	}
	if _, err := rules.Create(context.Background(), permissionrules.Rule{
		ToolName: "codex",
		Action:   permissionrules.ActionAsk,
		Priority: 50,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed permission rule failed: %v", err)
	}

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
		entClient:     client,
		approval:      approval.NewManager(approval.Config{Mode: approval.ModeAuto}),
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
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var payload struct {
		Approval struct {
			Status    string `json:"status"`
			RequestID string `json:"request_id"`
		} `json:"approval"`
		LaunchPolicy struct {
			ToolName       string `json:"tool_name"`
			ApprovalMode   string `json:"approval_mode"`
			PermissionRule struct {
				Matched bool   `json:"matched"`
				Action  string `json:"action"`
				Source  string `json:"source"`
			} `json:"permission_rule"`
		} `json:"launch_policy"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.LaunchPolicy.ToolName != "codex" {
		t.Fatalf("expected launch tool codex, got %q", payload.LaunchPolicy.ToolName)
	}
	if payload.LaunchPolicy.ApprovalMode != "prompt" {
		t.Fatalf("expected approval mode prompt, got %q", payload.LaunchPolicy.ApprovalMode)
	}
	if !payload.LaunchPolicy.PermissionRule.Matched {
		t.Fatalf("expected permission rule match, got %+v", payload.LaunchPolicy.PermissionRule)
	}
	if payload.LaunchPolicy.PermissionRule.Action != "ask" {
		t.Fatalf("expected permission rule action ask, got %q", payload.LaunchPolicy.PermissionRule.Action)
	}
	if payload.LaunchPolicy.PermissionRule.Source != "rule" {
		t.Fatalf("expected permission rule source rule, got %q", payload.LaunchPolicy.PermissionRule.Source)
	}
	if payload.Approval.Status != "pending" || payload.Approval.RequestID == "" {
		t.Fatalf("expected pending approval payload, got %+v", payload.Approval)
	}
}

func TestHandleResolveExternalAgentSessionReturnsPendingApprovalForAskRule(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "auto"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	rules, err := permissionrules.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new permission rule manager: %v", err)
	}
	if _, err := rules.Create(context.Background(), permissionrules.Rule{
		ToolName: "codex",
		Action:   permissionrules.ActionAsk,
		Priority: 50,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed permission rule failed: %v", err)
	}

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	taskStore := tasks.NewStore()

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
		entClient:     client,
		approval:      approval.NewManager(approval.Config{Mode: approval.ModeAuto}),
		taskStore:     taskStore,
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
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var payload struct {
		Approval struct {
			Status    string `json:"status"`
			RequestID string `json:"request_id"`
		} `json:"approval"`
		Session             toolsessions.Session `json:"session"`
		SessionRuntimeState tasks.SessionState   `json:"session_runtime_state"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Approval.Status != "pending" || payload.Approval.RequestID == "" {
		t.Fatalf("expected pending approval payload, got %+v", payload.Approval)
	}
	if payload.SessionRuntimeState.SessionID != payload.Session.ID {
		t.Fatalf("expected session runtime state for %q, got %+v", payload.Session.ID, payload.SessionRuntimeState)
	}
	if payload.SessionRuntimeState.PendingRequestID != payload.Approval.RequestID {
		t.Fatalf("expected pending request id %q in session runtime state, got %+v", payload.Approval.RequestID, payload.SessionRuntimeState)
	}
	states := taskStore.ListSessionStates()
	if len(states) != 1 || states[0].PendingRequestID != payload.Approval.RequestID {
		t.Fatalf("expected pending session state for approval %q, got %+v", payload.Approval.RequestID, states)
	}
}

func TestHandleResolveExternalAgentSessionReturnsPendingApprovalForManualMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "manual"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	taskStore := tasks.NewStore()

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
		entClient:     client,
		approval:      approval.NewManager(approval.Config{Mode: approval.ModeManual}),
		taskStore:     taskStore,
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
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var payload struct {
		Approval struct {
			Status    string `json:"status"`
			RequestID string `json:"request_id"`
		} `json:"approval"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Approval.Status != "pending" || payload.Approval.RequestID == "" {
		t.Fatalf("expected pending approval payload, got %+v", payload.Approval)
	}
	states := taskStore.ListSessionStates()
	if len(states) != 1 || states[0].PendingRequestID != payload.Approval.RequestID {
		t.Fatalf("expected pending session state for approval %q, got %+v", payload.Approval.RequestID, states)
	}
}

func TestHandleResolveExternalAgentSessionRejectsDeniedByPermissionRule(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "auto"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	rules, err := permissionrules.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new permission rule manager: %v", err)
	}
	if _, err := rules.Create(context.Background(), permissionrules.Rule{
		ToolName: "codex",
		Action:   permissionrules.ActionDeny,
		Priority: 50,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed permission rule failed: %v", err)
	}

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	taskStore := tasks.NewStore()

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
		entClient:     client,
		approval:      approval.NewManager(approval.Config{Mode: approval.ModeAuto}),
		taskStore:     taskStore,
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
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
	if len(taskStore.ListSessionStates()) != 0 {
		t.Fatalf("expected no pending session state after deny, got %+v", taskStore.ListSessionStates())
	}
}

func TestApproveExternalAgentPendingRequestAllowsSubsequentResolveForSameSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "manual"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	approvalMgr := approval.NewManager(approval.Config{Mode: approval.ModeManual})
	taskStore := tasks.NewStore()

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
		entClient:     client,
		approval:      approvalMgr,
		taskStore:     taskStore,
	}
	e := echo.New()

	resolveReq := func() *httptest.ResponseRecorder {
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
		return rec
	}

	first := resolveReq()
	if first.Code != http.StatusAccepted {
		t.Fatalf("expected first resolve status %d, got %d: %s", http.StatusAccepted, first.Code, first.Body.String())
	}
	var firstPayload struct {
		Approval struct {
			RequestID string `json:"request_id"`
		} `json:"approval"`
		Session toolsessions.Session `json:"session"`
	}
	decodeJSON(t, first.Body.Bytes(), &firstPayload)
	if firstPayload.Approval.RequestID == "" {
		t.Fatal("expected approval request id")
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/approvals/"+firstPayload.Approval.RequestID+"/approve", nil)
	approveRec := httptest.NewRecorder()
	approveCtx := e.NewContext(approveReq, approveRec)
	approveCtx.SetPath("/api/approvals/:id/approve")
	approveCtx.SetPathValues(echo.PathValues{{Name: "id", Value: firstPayload.Approval.RequestID}})
	if err := server.handleApproveRequest(approveCtx); err != nil {
		t.Fatalf("approve handler failed: %v", err)
	}
	if approveRec.Code != http.StatusOK {
		t.Fatalf("expected approve status %d, got %d: %s", http.StatusOK, approveRec.Code, approveRec.Body.String())
	}
	var approvePayload struct {
		Status              string             `json:"status"`
		ID                  string             `json:"id"`
		SessionRuntimeState tasks.SessionState `json:"session_runtime_state"`
	}
	decodeJSON(t, approveRec.Body.Bytes(), &approvePayload)
	if approvePayload.Status != "approved" || approvePayload.ID != firstPayload.Approval.RequestID {
		t.Fatalf("expected approve payload with id %q, got %+v", firstPayload.Approval.RequestID, approvePayload)
	}
	if approvePayload.SessionRuntimeState.SessionID != firstPayload.Session.ID || approvePayload.SessionRuntimeState.PermissionMode != "auto" || approvePayload.SessionRuntimeState.PendingRequestID != "" {
		t.Fatalf("expected auto cleared session runtime state, got %+v", approvePayload.SessionRuntimeState)
	}
	if mode, ok := approvalMgr.GetSessionMode(firstPayload.Session.ID); !ok || mode != approval.ModeAuto {
		t.Fatalf("expected session mode auto after approval, got %q ok=%v", mode, ok)
	}

	second := resolveReq()
	if second.Code != http.StatusOK {
		t.Fatalf("expected second resolve status %d, got %d: %s", http.StatusOK, second.Code, second.Body.String())
	}
	var secondPayload struct {
		Created bool                 `json:"created"`
		Session toolsessions.Session `json:"session"`
	}
	decodeJSON(t, second.Body.Bytes(), &secondPayload)
	if secondPayload.Created {
		t.Fatal("expected reused session after approval")
	}
	if secondPayload.Session.ID != firstPayload.Session.ID {
		t.Fatalf("expected same session %q after approval, got %q", firstPayload.Session.ID, secondPayload.Session.ID)
	}
	if states := taskStore.ListSessionStates(); len(states) != 1 || states[0].PermissionMode != "auto" || states[0].PendingRequestID != "" {
		t.Fatalf("expected cleared pending state with auto mode, got %+v", states)
	}
}

func TestHandleResolveExternalAgentSessionStartsProcessWhenApproved(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "auto"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	processMgr := process.NewManager(log)

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
		entClient:     client,
		approval:      approval.NewManager(approval.Config{Mode: approval.ModeAuto}),
		processMgr:    processMgr,
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
		Session toolsessions.Session `json:"session"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	status, err := processMgr.GetStatus(payload.Session.ID)
	if err != nil {
		t.Fatalf("expected process status, got err: %v", err)
	}
	if strings.TrimSpace(status.Command) == "" {
		t.Fatalf("expected non-empty process command, got %+v", status)
	}
	t.Cleanup(func() { _ = processMgr.Kill(payload.Session.ID) })
}

func TestApproveExternalAgentPendingRequestStartsProcessImmediately(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Approval.Mode = "manual"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	approvalMgr := approval.NewManager(approval.Config{Mode: approval.ModeManual})
	processMgr := process.NewManager(log)
	taskStore := tasks.NewStore()

	server := &Server{
		config:        cfg,
		logger:        log,
		toolSess:      toolMgr,
		externalAgent: extMgr,
		entClient:     client,
		approval:      approvalMgr,
		taskStore:     taskStore,
		processMgr:    processMgr,
	}
	e := echo.New()

	resolveReq := func() *httptest.ResponseRecorder {
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
		return rec
	}

	first := resolveReq()
	if first.Code != http.StatusAccepted {
		t.Fatalf("expected first resolve status %d, got %d: %s", http.StatusAccepted, first.Code, first.Body.String())
	}
	var firstPayload struct {
		Approval struct {
			RequestID string `json:"request_id"`
		} `json:"approval"`
		Session toolsessions.Session `json:"session"`
	}
	decodeJSON(t, first.Body.Bytes(), &firstPayload)

	approveReq := httptest.NewRequest(http.MethodPost, "/api/approvals/"+firstPayload.Approval.RequestID+"/approve", nil)
	approveRec := httptest.NewRecorder()
	approveCtx := e.NewContext(approveReq, approveRec)
	approveCtx.SetPath("/api/approvals/:id/approve")
	approveCtx.SetPathValues(echo.PathValues{{Name: "id", Value: firstPayload.Approval.RequestID}})
	if err := server.handleApproveRequest(approveCtx); err != nil {
		t.Fatalf("approve handler failed: %v", err)
	}
	if approveRec.Code != http.StatusOK {
		t.Fatalf("expected approve status %d, got %d: %s", http.StatusOK, approveRec.Code, approveRec.Body.String())
	}

	status, err := processMgr.GetStatus(firstPayload.Session.ID)
	if err != nil {
		t.Fatalf("expected process status immediately after approve, got err: %v", err)
	}
	if strings.TrimSpace(status.Command) == "" {
		t.Fatalf("expected non-empty process command after approve, got %+v", status)
	}
	t.Cleanup(func() { _ = processMgr.Kill(firstPayload.Session.ID) })
}

func TestApprovePendingToolCallClearsStoredReplayContext(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	approvalMgr := approval.NewManager(approval.Config{Mode: approval.ModeManual})
	server := &Server{config: cfg, logger: newTestLogger(t), approval: approvalMgr, taskStore: tasks.NewStore()}
	e := echo.New()

	if _, err := approvalMgr.EnqueueRequest("exec", map[string]interface{}{"command": "pwd"}, "sess-tool-1"); err != nil {
		t.Fatalf("enqueue approval request: %v", err)
	}
	pending := approvalMgr.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected one pending approval, got %d", len(pending))
	}
	requestID := pending[0].ID
	if err := approval.RememberPendingToolCall(requestID, "sess-tool-1", providers.UnifiedToolCall{Name: "exec", Arguments: map[string]interface{}{"command": "pwd"}}); err != nil {
		t.Fatalf("remember pending tool call: %v", err)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/approvals/"+requestID+"/approve", nil)
	approveRec := httptest.NewRecorder()
	approveCtx := e.NewContext(approveReq, approveRec)
	approveCtx.SetPath("/api/approvals/:id/approve")
	approveCtx.SetPathValues(echo.PathValues{{Name: "id", Value: requestID}})
	if err := server.handleApproveRequest(approveCtx); err != nil {
		t.Fatalf("approve handler failed: %v", err)
	}
	if approveRec.Code != http.StatusOK {
		t.Fatalf("expected approve status %d, got %d: %s", http.StatusOK, approveRec.Code, approveRec.Body.String())
	}
	if _, ok := approval.PendingToolCallForRequest(requestID); ok {
		t.Fatalf("expected pending tool call %q to be cleared", requestID)
	}
}
