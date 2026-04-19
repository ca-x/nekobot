package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/logger"
	"nekobot/pkg/permissionrules"
	"nekobot/pkg/process"
	"nekobot/pkg/providers"
	"nekobot/pkg/session"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/tasks"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/version"
)

type stubGatewayRouter struct {
	lastRuntimeID string
	reply         string
	err           error
}

type stubGatewaySession struct {
	messages []agent.Message
}

func (s *stubGatewaySession) GetMessages() []agent.Message {
	msgs := make([]agent.Message, len(s.messages))
	copy(msgs, s.messages)
	return msgs
}

func (s *stubGatewaySession) AddMessage(msg agent.Message) {
	s.messages = append(s.messages, msg)
}

func (s *stubGatewayRouter) RegisterChannel(string) {}

func (s *stubGatewayRouter) UnregisterAll() {}

func (s *stubGatewayRouter) HandleInbound(context.Context, *bus.Message) error { return nil }

func (s *stubGatewayRouter) ChatWebsocket(
	ctx context.Context,
	userID, username, upstreamSessionID, content, runtimeID string,
) (string, map[string]any, error) {
	s.lastRuntimeID = runtimeID
	if s.err != nil {
		return "", nil, s.err
	}
	return s.reply, map[string]any{"runtime_id": runtimeID}, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Gateway.Port = 0 // Don't actually listen

	log, err := logger.New(&logger.Config{Level: "error"})
	if err != nil {
		t.Fatal(err)
	}

	localBus := bus.NewLocalBus(log, 10)

	// Create server without agent (will panic if chat is used, but we only test REST)
	s := &Server{
		config:     cfg,
		logger:     log,
		bus:        localBus,
		sessionMgr: session.NewManager(t.TempDir(), cfg.Sessions),
		clients:    make(map[string]*Client),
	}
	s.setupRoutes()
	return s
}

func newAuthedTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	s := newTestServer(t)
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	passwordHash, err := config.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := config.SaveAdminCredentialFromConfig(cfg, &config.AdminCredential{
		Username:     "admin",
		Nickname:     "Admin",
		PasswordHash: passwordHash,
		JWTSecret:    "gateway-jwt-secret",
	}); err != nil {
		t.Fatalf("save admin credential: %v", err)
	}

	dbPath, err := config.RuntimeDBPath(cfg)
	if err != nil {
		t.Fatalf("runtime db path: %v", err)
	}
	client, err := ent.Open("sqlite3", "file:"+dbPath+"?_fk=1")
	if err != nil {
		t.Fatalf("open ent client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "admin",
	})
	tokenString, err := token.SignedString([]byte("gateway-jwt-secret"))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}

	s.entClient = client
	s.config = cfg
	toolMgr, err := toolsessions.NewManager(cfg, s.logger, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(cfg, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	s.externalAgent = extMgr
	s.toolSess = toolMgr
	s.processMgr = process.NewManager(s.logger)
	s.setupRoutes()
	return s, tokenString
}

func signGatewayTestToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("gateway-jwt-secret"))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return tokenString
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", body["status"])
	}
}

func TestMetricsEndpointRequiresAuth(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMetricsEndpointRejectsMemberRole(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestMetricsEndpointReportsPairingSourceBreakdown(t *testing.T) {
	s, token := newAuthedTestServer(t)

	generated := &Client{id: "client-generated", send: make(chan []byte, 1), session: &session.Session{ID: "client-generated", Source: session.SourceGateway}}
	requested := &Client{id: "client-requested", send: make(chan []byte, 1), requestedSessionID: "gateway-session", session: &session.Session{ID: "gateway-session", Source: session.SourceGateway}}
	legacy := &Client{id: "client-legacy", send: make(chan []byte, 1), requestedSessionID: "legacy-session", session: &session.Session{ID: "legacy-session", Source: ""}}

	s.clients[generated.id] = generated
	s.clients[requested.id] = requested
	s.clients[legacy.id] = legacy

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if got := body["gateway_connections_paired_generated"]; got != float64(1) {
		t.Fatalf("expected 1 generated paired metric, got %v", got)
	}
	if got := body["gateway_connections_paired_requested"]; got != float64(1) {
		t.Fatalf("expected 1 requested paired metric, got %v", got)
	}
	if got := body["gateway_connections_paired_legacy"]; got != float64(1) {
		t.Fatalf("expected 1 legacy paired metric, got %v", got)
	}
}

func TestResolveExternalAgentSessionEndpointCreatesSession(t *testing.T) {
	s, token := newAuthedTestServer(t)

	toolMgr, err := toolsessions.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(s.config, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	s.externalAgent = extMgr

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Created bool                 `json:"created"`
		Session toolsessions.Session `json:"session"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Created {
		t.Fatal("expected created=true")
	}
	if payload.Session.Owner != "admin" {
		t.Fatalf("expected owner admin, got %q", payload.Session.Owner)
	}
	if payload.Session.Tool != "codex" || payload.Session.Command != "codex" {
		t.Fatalf("expected codex launch identity, got tool=%q command=%q", payload.Session.Tool, payload.Session.Command)
	}
}

func TestResolveExternalAgentSessionEndpointResolvesRelativeWorkspace(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	toolMgr, err := toolsessions.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(s.config, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	s.externalAgent = extMgr

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"projects/demo"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Session      toolsessions.Session `json:"session"`
		LaunchPolicy map[string]any       `json:"launch_policy"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := filepath.Join(s.config.WorkspacePath(), "projects", "demo")
	if payload.Session.Workdir != want {
		t.Fatalf("expected workdir %q, got %q", want, payload.Session.Workdir)
	}
	if payload.LaunchPolicy["tool_name"] != "codex" {
		t.Fatalf("expected launch_policy tool_name codex, got %+v", payload.LaunchPolicy)
	}
}

func TestResolveExternalAgentSessionEndpointRejectsWorkspaceOutsideConfiguredRoot(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	toolMgr, err := toolsessions.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(s.config, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	s.externalAgent = extMgr

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+filepath.Join(t.TempDir(), "outside")+`"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "workspace must stay within configured workspace") {
		t.Fatalf("expected workspace restriction error, got %s", rec.Body.String())
	}
}

func TestResolveExternalAgentSessionEndpointIncludesMatchedPermissionRulePreview(t *testing.T) {
	s, token := newAuthedTestServer(t)

	rules, err := permissionrules.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		t.Fatalf("new permission rule manager: %v", err)
	}
	if _, err := rules.Create(context.Background(), permissionrules.Rule{
		ToolName: "codex",
		Action:   permissionrules.ActionAllow,
		Priority: 50,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed permission rule failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		LaunchPolicy map[string]any `json:"launch_policy"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.LaunchPolicy["tool_name"] != "codex" {
		t.Fatalf("expected launch_policy tool_name codex, got %+v", payload.LaunchPolicy)
	}
	permissionRule, ok := payload.LaunchPolicy["permission_rule"].(map[string]any)
	if !ok {
		t.Fatalf("expected permission_rule map, got %+v", payload.LaunchPolicy["permission_rule"])
	}
	if matched, _ := permissionRule["matched"].(bool); !matched {
		t.Fatalf("expected matched permission rule, got %+v", permissionRule)
	}
	if action, _ := permissionRule["action"].(string); action != "allow" {
		t.Fatalf("expected action allow, got %+v", permissionRule)
	}
}

func TestResolveExternalAgentSessionEndpointReturnsPendingApprovalForAskRule(t *testing.T) {
	s, token := newAuthedTestServer(t)

	rules, err := permissionrules.NewManager(s.config, s.logger, s.entClient)
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

	s.approval = approval.NewManager(approval.Config{Mode: approval.ModeAuto})
	s.taskStore = tasks.NewStore()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Session  toolsessions.Session `json:"session"`
		Approval struct {
			Status    string `json:"status"`
			RequestID string `json:"request_id"`
		} `json:"approval"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Approval.Status != "pending" || payload.Approval.RequestID == "" {
		t.Fatalf("expected pending approval payload, got %+v", payload.Approval)
	}
	states := s.taskStore.ListSessionStates()
	if len(states) != 1 || states[0].SessionID != payload.Session.ID || states[0].PendingRequestID != payload.Approval.RequestID {
		t.Fatalf("expected pending task state for session %q, got %+v", payload.Session.ID, states)
	}
	if _, err := s.processMgr.GetStatus(payload.Session.ID); err == nil {
		t.Fatalf("expected process to stay stopped while approval is pending")
	}
}

func TestResolveExternalAgentSessionEndpointReturnsPendingApprovalForManualMode(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	s.taskStore = tasks.NewStore()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Session  toolsessions.Session `json:"session"`
		Approval struct {
			Status    string `json:"status"`
			RequestID string `json:"request_id"`
		} `json:"approval"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Approval.Status != "pending" || payload.Approval.RequestID == "" {
		t.Fatalf("expected pending approval payload, got %+v", payload.Approval)
	}
	states := s.taskStore.ListSessionStates()
	if len(states) != 1 || states[0].SessionID != payload.Session.ID || states[0].PendingRequestID != payload.Approval.RequestID {
		t.Fatalf("expected pending task state for session %q, got %+v", payload.Session.ID, states)
	}
	if _, err := s.processMgr.GetStatus(payload.Session.ID); err == nil {
		t.Fatalf("expected process to stay stopped while approval is pending")
	}
}

func TestResolveExternalAgentSessionEndpointRejectsDeniedByApprovalMode(t *testing.T) {
	s, token := newAuthedTestServer(t)
	approvalMgr := approval.NewManager(approval.Config{Mode: approval.ModePrompt})
	approvalMgr.PromptFunc = func(req *approval.Request) (bool, error) {
		return false, nil
	}
	s.approval = approvalMgr

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) == "" || !strings.Contains(rec.Body.String(), `"status":"denied"`) {
		t.Fatalf("expected denied approval payload, got %s", rec.Body.String())
	}
}

func TestResolveExternalAgentSessionEndpointStartsProcessWhenApproved(t *testing.T) {
	s, token := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Session toolsessions.Session `json:"session"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	status, err := s.processMgr.GetStatus(payload.Session.ID)
	if err != nil {
		t.Fatalf("expected process status, got err: %v", err)
	}
	if strings.TrimSpace(status.Command) == "" {
		t.Fatalf("expected non-empty process command, got %+v", status)
	}
	t.Cleanup(func() { _ = s.processMgr.Kill(payload.Session.ID) })
}

func TestGatewayApproveExternalAgentPendingRequestStartsProcessImmediately(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	s.taskStore = tasks.NewStore()

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	resolveReq.Header.Set("Authorization", "Bearer "+token)
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveReq.RemoteAddr = "127.0.0.1:4321"
	resolveRec := httptest.NewRecorder()
	s.mux.ServeHTTP(resolveRec, resolveReq)

	if resolveRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", resolveRec.Code, resolveRec.Body.String())
	}
	var payload struct {
		Session  toolsessions.Session `json:"session"`
		Approval struct {
			RequestID string `json:"request_id"`
		} `json:"approval"`
		SessionRuntimeState tasks.SessionState `json:"session_runtime_state"`
	}
	if err := json.NewDecoder(resolveRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode resolve response: %v", err)
	}
	if payload.Approval.RequestID == "" {
		t.Fatal("expected approval request id")
	}
	if payload.SessionRuntimeState.SessionID != payload.Session.ID || payload.SessionRuntimeState.PendingRequestID != payload.Approval.RequestID {
		t.Fatalf("expected pending session runtime state, got %+v", payload.SessionRuntimeState)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/v1/approvals/"+payload.Approval.RequestID+"/approve", nil)
	approveReq.Header.Set("Authorization", "Bearer "+token)
	approveReq.RemoteAddr = "127.0.0.1:4321"
	approveRec := httptest.NewRecorder()
	s.mux.ServeHTTP(approveRec, approveReq)

	if approveRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", approveRec.Code, approveRec.Body.String())
	}
	var approvePayload struct {
		Status              string             `json:"status"`
		ID                  string             `json:"id"`
		SessionRuntimeState tasks.SessionState `json:"session_runtime_state"`
	}
	if err := json.NewDecoder(approveRec.Body).Decode(&approvePayload); err != nil {
		t.Fatalf("decode approve response: %v", err)
	}
	if approvePayload.Status != "approved" || approvePayload.ID != payload.Approval.RequestID {
		t.Fatalf("expected approved payload for %q, got %+v", payload.Approval.RequestID, approvePayload)
	}
	if approvePayload.SessionRuntimeState.SessionID != payload.Session.ID || approvePayload.SessionRuntimeState.PermissionMode != "auto" || approvePayload.SessionRuntimeState.PendingRequestID != "" {
		t.Fatalf("expected auto cleared session runtime state, got %+v", approvePayload.SessionRuntimeState)
	}
	status, err := s.processMgr.GetStatus(payload.Session.ID)
	if err != nil {
		t.Fatalf("expected process status after approve, got err: %v", err)
	}
	if strings.TrimSpace(status.Command) == "" {
		t.Fatalf("expected non-empty process command, got %+v", status)
	}
	t.Cleanup(func() { _ = s.processMgr.Kill(payload.Session.ID) })
}

func TestGatewayDenyExternalAgentPendingRequestPersistsReasonAndClearsPendingState(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	s.taskStore = tasks.NewStore()

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	resolveReq.Header.Set("Authorization", "Bearer "+token)
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveReq.RemoteAddr = "127.0.0.1:4321"
	resolveRec := httptest.NewRecorder()
	s.mux.ServeHTTP(resolveRec, resolveReq)

	if resolveRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", resolveRec.Code, resolveRec.Body.String())
	}
	var payload struct {
		Session  toolsessions.Session `json:"session"`
		Approval struct {
			RequestID string `json:"request_id"`
		} `json:"approval"`
	}
	if err := json.NewDecoder(resolveRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode resolve response: %v", err)
	}
	if payload.Approval.RequestID == "" {
		t.Fatal("expected approval request id")
	}

	denyReq := httptest.NewRequest(http.MethodPost, "/api/v1/approvals/"+payload.Approval.RequestID+"/deny", strings.NewReader(
		`{"reason":"operator rejected launch"}`,
	))
	denyReq.Header.Set("Authorization", "Bearer "+token)
	denyReq.Header.Set("Content-Type", "application/json")
	denyReq.RemoteAddr = "127.0.0.1:4321"
	denyRec := httptest.NewRecorder()
	s.mux.ServeHTTP(denyRec, denyReq)

	if denyRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", denyRec.Code, denyRec.Body.String())
	}
	req, ok := s.approval.GetRequest(payload.Approval.RequestID)
	if !ok {
		t.Fatalf("expected approval request %q to still exist for inspection", payload.Approval.RequestID)
	}
	if req.Reason != "operator rejected launch" {
		t.Fatalf("expected deny reason to persist, got %q", req.Reason)
	}
	states := s.taskStore.ListSessionStates()
	if len(states) != 1 || states[0].SessionID != payload.Session.ID || states[0].PendingRequestID != "" {
		t.Fatalf("expected cleared pending task state for session %q, got %+v", payload.Session.ID, states)
	}
}

func TestGatewayListApprovalsReturnsPendingRequests(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	s.taskStore = tasks.NewStore()

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex","workspace":"`+s.config.WorkspacePath()+`"}`,
	))
	resolveReq.Header.Set("Authorization", "Bearer "+token)
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveReq.RemoteAddr = "127.0.0.1:4321"
	resolveRec := httptest.NewRecorder()
	s.mux.ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", resolveRec.Code, resolveRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/approvals", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listReq.RemoteAddr = "127.0.0.1:4321"
	listRec := httptest.NewRecorder()
	s.mux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var pending []approval.Request
	if err := json.NewDecoder(listRec.Body).Decode(&pending); err != nil {
		t.Fatalf("decode approvals response: %v", err)
	}
	if len(pending) != 1 || pending[0].ToolName != "codex" {
		t.Fatalf("expected one codex approval request, got %+v", pending)
	}
}

func TestGatewayApprovePendingToolCallReplaysAgentCall(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	s.taskStore = tasks.NewStore()
	requestID := "approval-tool-1"
	if _, err := s.approval.EnqueueRequest("exec", map[string]interface{}{"command": "pwd"}, "sess-tool-1"); err != nil {
		t.Fatalf("enqueue approval request: %v", err)
	}
	pending := s.approval.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected one pending approval, got %d", len(pending))
	}
	requestID = pending[0].ID
	if err := approval.RememberPendingToolCall(requestID, "sess-tool-1", providers.UnifiedToolCall{
		Name:      "exec",
		Arguments: map[string]interface{}{"command": "pwd"},
	}); err != nil {
		t.Fatalf("remember pending tool call: %v", err)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/v1/approvals/"+requestID+"/approve", nil)
	approveReq.Header.Set("Authorization", "Bearer "+token)
	approveReq.RemoteAddr = "127.0.0.1:4321"
	approveRec := httptest.NewRecorder()
	s.mux.ServeHTTP(approveRec, approveReq)

	if approveRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", approveRec.Code, approveRec.Body.String())
	}
	if _, ok := approval.PendingToolCallForRequest(requestID); ok {
		t.Fatalf("expected pending tool call %q to be cleared", requestID)
	}
}

func TestResolveExternalAgentSessionEndpointRequiresAuth(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	toolMgr, err := toolsessions.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	extMgr, err := externalagent.NewManager(s.config, toolMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	s.externalAgent = extMgr

	req := httptest.NewRequest(http.MethodPost, "/api/v1/external-agents/resolve-session", strings.NewReader(
		`{"agent_kind":"codex"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	s, token := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}

	// Check required fields exist
	requiredFields := []string{
		"gateway_connections_total",
		"gateway_connections_paired",
		"gateway_connections_unpaired",
		"gateway_rate_limit_per_minute",
		"gateway_max_connections",
	}
	for _, field := range requiredFields {
		if _, ok := body[field]; !ok {
			t.Fatalf("expected metrics field %s not found", field)
		}
	}

	// Verify values with no connections
	if body["gateway_connections_total"] != float64(0) {
		t.Fatalf("expected 0 total connections, got %v", body["gateway_connections_total"])
	}
	if body["gateway_connections_paired"] != float64(0) {
		t.Fatalf("expected 0 paired connections, got %v", body["gateway_connections_paired"])
	}
	if body["gateway_connections_unpaired"] != float64(0) {
		t.Fatalf("expected 0 unpaired connections, got %v", body["gateway_connections_unpaired"])
	}
}

func TestStatusEndpoint(t *testing.T) {
	s, token := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode status response: %v", err)
	}

	if body["version"] != version.GetVersion() {
		t.Fatalf("expected version %s, got %v", version.GetVersion(), body["version"])
	}
	if body["connections"] != float64(0) {
		t.Fatalf("expected 0 connections, got %v", body["connections"])
	}
	if body["paired_connections"] != float64(0) {
		t.Fatalf("expected 0 paired connections, got %v", body["paired_connections"])
	}
}

func TestConnectionsEndpoint(t *testing.T) {
	s, token := newAuthedTestServer(t)

	now := time.Unix(1_700_000_000, 0).UTC()
	pairedSession, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}
	s.clients["client-b"] = &Client{
		id:          "client-b",
		send:        make(chan []byte, 1),
		userID:      "user-b",
		username:    "bob",
		connectedAt: now.Add(2 * time.Minute),
		remoteAddr:  "10.0.0.2:1234",
		session:     pairedSession,
	}
	s.clients["client-a"] = &Client{
		id:          "client-a",
		send:        make(chan []byte, 1),
		userID:      "user-a",
		username:    "alice",
		connectedAt: now,
		remoteAddr:  "10.0.0.1:1234",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(body))
	}
	if got := body[0]["id"]; got != "client-a" {
		t.Fatalf("expected first connection client-a, got %v", got)
	}
	if got := body[1]["id"]; got != "client-b" {
		t.Fatalf("expected second connection client-b, got %v", got)
	}
	if got := body[0]["user_id"]; got != "user-a" {
		t.Fatalf("expected first user_id user-a, got %v", got)
	}
	if got := body[0]["username"]; got != "alice" {
		t.Fatalf("expected first username alice, got %v", got)
	}
	if got := body[0]["remote_addr"]; got != "10.0.0.1:1234" {
		t.Fatalf("expected first remote_addr, got %v", got)
	}
	if got := body[0]["connected_at"]; got != now.Format(time.RFC3339) {
		t.Fatalf("expected first connected_at %q, got %v", now.Format(time.RFC3339), got)
	}
	if got := body[0]["session_id"]; got != nil {
		t.Fatalf("expected nil session_id without session, got %v", got)
	}
	if got := body[0]["paired"]; got != false {
		t.Fatalf("expected first paired false, got %v", got)
	}
	if got := body[0]["paired_session_id"]; got != nil {
		t.Fatalf("expected first paired_session_id nil, got %v", got)
	}
	if got := body[0]["session_source"]; got != nil {
		t.Fatalf("expected first session_source nil without paired session, got %v", got)
	}
	if got := body[1]["paired"]; got != true {
		t.Fatalf("expected second paired true, got %v", got)
	}
	if got := body[1]["paired_session_id"]; got != "paired-session" {
		t.Fatalf("expected second paired_session_id paired-session, got %v", got)
	}
	if got := body[1]["session_source"]; got != "requested" {
		t.Fatalf("expected second session_source requested, got %v", got)
	}
}

func TestGatewayStatusEndpointRequiresAuth(t *testing.T) {
	s, _ := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGatewayConnectionsEndpointRequiresAuth(t *testing.T) {
	s, _ := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGatewayStatusEndpointRejectsMemberRole(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGatewayConnectionsEndpointAllowsMemberRoleForOwnedConnectionsOnly(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})
	s.clients["client-b"] = &Client{
		id:          "client-b",
		send:        make(chan []byte, 1),
		userID:      "other-id",
		username:    "other",
		connectedAt: time.Unix(1_700_000_100, 0).UTC(),
		remoteAddr:  "10.0.0.2:1234",
	}
	s.clients["client-a"] = &Client{
		id:                 "client-a",
		send:               make(chan []byte, 1),
		userID:             "viewer-id",
		username:           "viewer",
		connectedAt:        time.Unix(1_700_000_000, 0).UTC(),
		remoteAddr:         "10.0.0.1:1234",
		sessionSource:      "requested",
		requestedSessionID: "sess-1",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode member connections response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 owned connection, got %d", len(body))
	}
	if got := body[0]["id"]; got != "client-a" {
		t.Fatalf("expected owned connection client-a, got %v", got)
	}
	if got := body[0]["user_id"]; got != "viewer-id" {
		t.Fatalf("expected owned user_id viewer-id, got %v", got)
	}
	if got := body[0]["remote_addr"]; got != "" {
		t.Fatalf("expected member remote_addr redacted, got %v", got)
	}
	if got := body[0]["session_source"]; got != nil {
		t.Fatalf("expected member session_source redacted, got %v", got)
	}
	if got := body[0]["requested_session_id"]; got != nil {
		t.Fatalf("expected member requested_session_id redacted, got %v", got)
	}
}

func TestGatewayAuthenticateRequestAllowsMemberRoleForWebsocketPath(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := s.authenticateRequest(req)
	if err != nil {
		t.Fatalf("expected member token to authenticate, got %v", err)
	}
	if authCtx.userID != "viewer-id" {
		t.Fatalf("expected user id viewer-id, got %q", authCtx.userID)
	}
	if authCtx.username != "viewer" {
		t.Fatalf("expected username viewer, got %q", authCtx.username)
	}
	if authCtx.role != "member" {
		t.Fatalf("expected role member, got %q", authCtx.role)
	}
}

func TestDeleteConnectionEndpointRemovesClient(t *testing.T) {
	s, token := newAuthedTestServer(t)

	client := &Client{
		id:       "test-client",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
	}
	s.clients[client.id] = client

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/"+client.id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if len(s.clients) != 0 {
		t.Fatalf("expected client to be removed, got %d active clients", len(s.clients))
	}
	select {
	case _, ok := <-client.send:
		if ok {
			t.Fatal("expected client send channel to be closed")
		}
	default:
		t.Fatal("expected client send channel to be closed")
	}
}

func TestDeleteConnectionEndpointReturnsNotFoundForUnknownClient(t *testing.T) {
	s, token := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/missing", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteConnectionsEndpointRemovesAllClientsForAdmin(t *testing.T) {
	s, token := newAuthedTestServer(t)

	s.clients["client-a"] = &Client{id: "client-a", send: make(chan []byte, 1), userID: "user-a", username: "alice"}
	s.clients["client-b"] = &Client{id: "client-b", send: make(chan []byte, 1), userID: "user-b", username: "bob"}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode bulk delete response: %v", err)
	}
	if body["deleted"] != 2 {
		t.Fatalf("expected deleted=2, got %+v", body)
	}
	if body["remaining"] != 0 {
		t.Fatalf("expected remaining=0, got %+v", body)
	}
	if len(s.clients) != 0 {
		t.Fatalf("expected all clients removed, got %d", len(s.clients))
	}
}

func TestDeleteConnectionsEndpointAllowsMemberRoleForOwnedClientsOnly(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	owned := &Client{id: "client-a", send: make(chan []byte, 1), userID: "viewer-id", username: "viewer"}
	other := &Client{id: "client-b", send: make(chan []byte, 1), userID: "other-id", username: "other"}
	s.clients[owned.id] = owned
	s.clients[other.id] = other

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode bulk delete response: %v", err)
	}
	if body["deleted"] != 1 {
		t.Fatalf("expected deleted=1, got %+v", body)
	}
	if body["remaining"] != 0 {
		t.Fatalf("expected visible remaining=0, got %+v", body)
	}
	if len(s.clients) != 1 {
		t.Fatalf("expected one remaining client, got %d", len(s.clients))
	}
	if _, ok := s.clients[other.id]; !ok {
		t.Fatal("expected non-owned client to remain")
	}
	if _, ok := s.clients[owned.id]; ok {
		t.Fatal("expected owned client to be removed")
	}
}

func TestDeleteConnectionsEndpointHidesOtherUsersRemainingCountFromMember(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	s.clients["client-a"] = &Client{id: "client-a", send: make(chan []byte, 1), userID: "viewer-id", username: "viewer"}
	s.clients["client-b"] = &Client{id: "client-b", send: make(chan []byte, 1), userID: "other-id", username: "other-1"}
	s.clients["client-c"] = &Client{id: "client-c", send: make(chan []byte, 1), userID: "other-id-2", username: "other-2"}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode bulk delete response: %v", err)
	}
	if body["deleted"] != 1 {
		t.Fatalf("expected deleted=1, got %+v", body)
	}
	if body["remaining"] != 0 {
		t.Fatalf("expected visible remaining=0, got %+v", body)
	}
	if len(s.clients) != 2 {
		t.Fatalf("expected two non-owned clients to remain, got %d", len(s.clients))
	}
}

func TestDeleteConnectionsEndpointDeletesOwnedClientsWhenMemberHasOnlyOwnedConnections(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	s.clients["client-a"] = &Client{id: "client-a", send: make(chan []byte, 1), userID: "viewer-id", username: "viewer"}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode bulk delete response: %v", err)
	}
	if body["deleted"] != 1 {
		t.Fatalf("expected deleted=1, got %+v", body)
	}
	if body["remaining"] != 0 {
		t.Fatalf("expected remaining=0, got %+v", body)
	}
	if len(s.clients) != 0 {
		t.Fatalf("expected member bulk delete to remove owned client, got %d", len(s.clients))
	}
}

func TestDeleteConnectionsEndpointRequiresAuth(t *testing.T) {
	s, _ := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestDeleteConnectionEndpointRequiresAuth(t *testing.T) {
	s, _ := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/test-client", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestDeleteConnectionEndpointReturnsNotFoundForMemberWithoutOwnedTarget(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/test-client", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteConnectionEndpointRejectsMemberRoleForOwnedConnection(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	client := &Client{
		id:       "client-a",
		send:     make(chan []byte, 1),
		userID:   "viewer-id",
		username: "viewer",
	}
	s.clients[client.id] = client

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/"+client.id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if len(s.clients) != 0 {
		t.Fatalf("expected client to be removed, got %d active clients", len(s.clients))
	}
}

func TestDeleteConnectionEndpointRejectsMemberRoleForOtherUsersConnection(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})

	client := &Client{
		id:       "client-a",
		send:     make(chan []byte, 1),
		userID:   "other-id",
		username: "other",
	}
	s.clients[client.id] = client

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/"+client.id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if len(s.clients) != 1 {
		t.Fatalf("expected client to remain, got %d active clients", len(s.clients))
	}
}

func TestGetConnectionEndpointReturnsConnectionDetails(t *testing.T) {
	s, token := newAuthedTestServer(t)
	now := time.Unix(1_700_001_000, 0).UTC()

	sess, err := s.sessionMgr.GetWithSource("gateway-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	s.clients["client-a"] = &Client{
		id:          "client-a",
		send:        make(chan []byte, 1),
		userID:      "user-a",
		username:    "alice",
		session:     sess,
		connectedAt: now,
		remoteAddr:  "10.0.0.5:1234",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/client-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode connection response: %v", err)
	}
	if got := body["id"]; got != "client-a" {
		t.Fatalf("expected id client-a, got %v", got)
	}
	if got := body["session_id"]; got != "gateway-session" {
		t.Fatalf("expected session_id gateway-session, got %v", got)
	}
	if got := body["remote_addr"]; got != "10.0.0.5:1234" {
		t.Fatalf("expected remote_addr 10.0.0.5:1234, got %v", got)
	}
	if got := body["connected_at"]; got != now.Format(time.RFC3339) {
		t.Fatalf("expected connected_at %q, got %v", now.Format(time.RFC3339), got)
	}
	if got := body["paired"]; got != true {
		t.Fatalf("expected paired true, got %v", got)
	}
	if got := body["paired_session_id"]; got != "gateway-session" {
		t.Fatalf("expected paired_session_id gateway-session, got %v", got)
	}
	if got := body["session_source"]; got != "requested" {
		t.Fatalf("expected session_source requested, got %v", got)
	}
}

func TestConnectionsEndpointMarksLegacyGatewaySessionSource(t *testing.T) {
	s, token := newAuthedTestServer(t)
	now := time.Unix(1_700_001_000, 0).UTC()

	legacySession, err := s.sessionMgr.GetWithSource("legacy-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}
	legacySession.Source = ""

	s.clients["client-a"] = &Client{
		id:          "client-a",
		send:        make(chan []byte, 1),
		userID:      "user-a",
		username:    "alice",
		session:     legacySession,
		connectedAt: now,
		remoteAddr:  "10.0.0.5:1234",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(body))
	}
	if got := body[0]["session_source"]; got != "legacy" {
		t.Fatalf("expected session_source legacy, got %v", got)
	}
}

func TestGetConnectionEndpointAllowsMemberRoleForOwnedConnection(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})
	s.clients["client-a"] = &Client{
		id:                 "client-a",
		send:               make(chan []byte, 1),
		userID:             "viewer-id",
		username:           "viewer",
		sessionSource:      "requested",
		requestedSessionID: "sess-1",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/client-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode connection response: %v", err)
	}
	if got := body["remote_addr"]; got != "" {
		t.Fatalf("expected member remote_addr redacted, got %v", got)
	}
	if got := body["session_source"]; got != nil {
		t.Fatalf("expected member session_source redacted, got %v", got)
	}
	if got := body["requested_session_id"]; got != nil {
		t.Fatalf("expected member requested_session_id redacted, got %v", got)
	}
}

func TestGetConnectionEndpointRejectsMemberRoleForOtherUsersConnection(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})
	s.clients["client-a"] = &Client{
		id:       "client-a",
		send:     make(chan []byte, 1),
		userID:   "other-id",
		username: "other",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/client-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetConnectionEndpointReturnsNotFoundForUnknownClient(t *testing.T) {
	s, token := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/missing", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetConnectionEndpointRequiresAuth(t *testing.T) {
	s, _ := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/client-a", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestStatusEndpointCountsConnectionsDeterministically(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.clients["client-b"] = &Client{id: "client-b", send: make(chan []byte, 1)}
	s.clients["client-a"] = &Client{id: "client-a", send: make(chan []byte, 1)}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"connections\":2") {
		t.Fatalf("expected response to report 2 connections, got %s", rec.Body.String())
	}
}

func TestStatusEndpointReportsPairingSourceBreakdown(t *testing.T) {
	s, token := newAuthedTestServer(t)

	generated := &Client{id: "client-generated", send: make(chan []byte, 1), session: &session.Session{ID: "client-generated", Source: session.SourceGateway}}
	requested := &Client{id: "client-requested", send: make(chan []byte, 1), requestedSessionID: "gateway-session", session: &session.Session{ID: "gateway-session", Source: session.SourceGateway}}
	legacy := &Client{id: "client-legacy", send: make(chan []byte, 1), requestedSessionID: "legacy-session", session: &session.Session{ID: "legacy-session", Source: ""}}

	s.clients[generated.id] = generated
	s.clients[requested.id] = requested
	s.clients[legacy.id] = legacy

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if got := body["paired_generated_connections"]; got != float64(1) {
		t.Fatalf("expected 1 generated paired connection, got %v", got)
	}
	if got := body["paired_requested_connections"]; got != float64(1) {
		t.Fatalf("expected 1 requested paired connection, got %v", got)
	}
	if got := body["paired_legacy_connections"]; got != float64(1) {
		t.Fatalf("expected 1 legacy paired connection, got %v", got)
	}
}

func TestStatusEndpointReportsUnpairedConnections(t *testing.T) {
	s, token := newAuthedTestServer(t)

	pairedSession, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	s.clients["client-a"] = &Client{id: "client-a", send: make(chan []byte, 1), session: pairedSession}
	s.clients["client-b"] = &Client{id: "client-b", send: make(chan []byte, 1)}
	s.clients["client-c"] = &Client{id: "client-c", send: make(chan []byte, 1)}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if body["connections"] != float64(3) {
		t.Fatalf("expected 3 connections, got %v", body["connections"])
	}
	if body["paired_connections"] != float64(1) {
		t.Fatalf("expected 1 paired connection, got %v", body["paired_connections"])
	}
	if body["unpaired_connections"] != float64(2) {
		t.Fatalf("expected 2 unpaired connections, got %v", body["unpaired_connections"])
	}
}

func TestStatusEndpointReportsPairedConnections(t *testing.T) {
	s, token := newAuthedTestServer(t)

	pairedSession, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	s.clients["client-a"] = &Client{
		id:      "client-a",
		send:    make(chan []byte, 1),
		session: pairedSession,
	}
	s.clients["client-b"] = &Client{
		id:   "client-b",
		send: make(chan []byte, 1),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if body["connections"] != float64(2) {
		t.Fatalf("expected 2 connections, got %v", body["connections"])
	}
	if body["paired_connections"] != float64(1) {
		t.Fatalf("expected 1 paired connection, got %v", body["paired_connections"])
	}
}

func TestWSChatRequiresAuth(t *testing.T) {
	s := newTestServer(t)

	// Regular HTTP request to WS endpoint should fail
	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRemoveClient(t *testing.T) {
	s := newTestServer(t)

	// Add a fake client
	client := &Client{
		id:   "test-client",
		send: make(chan []byte, 10),
	}
	s.clients["test-client"] = client

	if len(s.clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(s.clients))
	}

	s.removeClient(client)

	if len(s.clients) != 0 {
		t.Fatalf("expected 0 clients after removal, got %d", len(s.clients))
	}
}

func TestRemoveClientIdempotent(t *testing.T) {
	s := newTestServer(t)

	client := &Client{
		id:   "test-client",
		send: make(chan []byte, 10),
	}
	s.clients["test-client"] = client

	s.removeClient(client)
	// Second removal should not panic
	s.removeClient(client)

	if len(s.clients) != 0 {
		t.Fatalf("expected 0 clients, got %d", len(s.clients))
	}
}

func TestGetOrCreateSessionUsesGatewaySource(t *testing.T) {
	s := newTestServer(t)

	sess, err := s.getOrCreateSession("gateway-test")
	if err != nil {
		t.Fatalf("getOrCreateSession failed: %v", err)
	}

	managed, ok := sess.(*session.Session)
	if !ok {
		t.Fatalf("expected *session.Session, got %T", sess)
	}
	if managed.Source != session.SourceGateway {
		t.Fatalf("expected source %q, got %q", session.SourceGateway, managed.Source)
	}
}

func TestResolveGatewaySessionIDUsesRequestedExistingGatewaySession(t *testing.T) {
	s := newTestServer(t)

	if _, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway); err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ws/chat?session_id=paired-session", nil)

	sessionID, err := s.resolveGatewaySessionID(req, "generated-client")
	if err != nil {
		t.Fatalf("resolveGatewaySessionID returned error: %v", err)
	}
	if sessionID != "paired-session" {
		t.Fatalf("expected paired-session, got %q", sessionID)
	}
}

func TestResolveGatewaySessionIDRejectsUnknownRequestedSession(t *testing.T) {
	s, token := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ws/chat?session_id=missing-session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestWSChatRejectsUnknownRequestedSessionBeforeUpgrade(t *testing.T) {
	s, token := newAuthedTestServer(t)
	server := httptest.NewServer(s.mux)
	t.Cleanup(server.Close)

	wsURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	wsURL.Scheme = "ws"
	wsURL.Path = "/ws/chat"
	query := wsURL.Query()
	query.Set("session_id", "missing-session")
	wsURL.RawQuery = query.Encode()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL.String(), header)
	if err == nil {
		t.Fatal("expected websocket dial to fail")
	}
	if resp == nil {
		t.Fatalf("expected http response, got nil (err=%v)", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWSChatReturnsSessionUnavailableBeforeUpgrade(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.sessionMgr = nil

	server := httptest.NewServer(s.mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		conn.Close()
		t.Fatal("expected websocket dial to fail when session creation is unavailable")
	}
	if resp == nil {
		t.Fatalf("expected handshake response, got nil error=%v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestWSChatSessionUnavailableReleasesConnectionSlot(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Gateway.MaxConnections = 1
	s.sessionMgr = nil

	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	s.sessionMgr = session.NewManager(t.TempDir(), s.config.Sessions)

	server := httptest.NewServer(s.mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := "<nil>"
		if resp != nil {
			status = fmt.Sprintf("%d", resp.StatusCode)
		}
		t.Fatalf("second websocket dial failed: %v (status=%s)", err, status)
	}
	defer conn.Close()
}

func TestWSChatDoesNotLeaveSessionBehindWhenUpgradeFails(t *testing.T) {
	s, token := newAuthedTestServer(t)

	var generatedSessionID string
	block := make(chan struct{})
	s.beforeWSUpgrade = func(sessionID string) {
		generatedSessionID = sessionID
		close(block)
	}

	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	<-block

	if generatedSessionID == "" {
		t.Fatal("expected generated session id before upgrade failure")
	}
	if _, err := s.sessionMgr.GetExisting(generatedSessionID); err == nil {
		t.Fatalf("expected generated session %q to be absent after upgrade failure", generatedSessionID)
	}
}

func TestWSChatUpgradeFailureKeepsRequestedExistingSession(t *testing.T) {
	s, token := newAuthedTestServer(t)

	if _, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway); err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	s.beforeWSUpgrade = func(string) {}

	req := httptest.NewRequest(http.MethodGet, "/ws/chat?session_id=paired-session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if _, err := s.sessionMgr.GetExisting("paired-session"); err != nil {
		t.Fatalf("expected requested existing session to survive upgrade failure, got %v", err)
	}
}

func TestWSChatWelcomeDeliveryFailureKeepsRequestedExistingSession(t *testing.T) {
	s, token := newAuthedTestServer(t)

	if _, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway); err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	s.beforeWelcomeSend = func(client *Client) {
		for i := 0; i < cap(client.send); i++ {
			client.send <- []byte("buffer-full")
		}
	}

	server := httptest.NewServer(s.mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat?session_id=" + url.QueryEscape("paired-session")
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := "<nil>"
		if resp != nil {
			status = fmt.Sprintf("%d", resp.StatusCode)
		}
		t.Fatalf("websocket dial failed: %v (status=%s)", err, status)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if _, err := s.sessionMgr.GetExisting("paired-session"); err != nil {
		t.Fatalf("expected requested existing session to survive welcome delivery failure, got %v", err)
	}
}
func TestResolveGatewaySessionIDRejectsNonGatewaySession(t *testing.T) {
	s := newTestServer(t)

	if _, err := s.sessionMgr.GetWithSource("webui-session", session.SourceWebUI); err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ws/chat?session_id=webui-session", nil)

	if _, err := s.resolveGatewaySessionID(req, "generated-client"); err == nil {
		t.Fatal("expected non-gateway session to be rejected")
	}
}

func TestWSChatUsesRequestedExistingGatewaySession(t *testing.T) {
	s, token := newAuthedTestServer(t)

	if _, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway); err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	server := httptest.NewServer(s.mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat?session_id=" + url.QueryEscape("paired-session")
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := "<nil>"
		if resp != nil {
			status = fmt.Sprintf("%d", resp.StatusCode)
		}
		t.Fatalf("websocket dial failed: %v (status=%s)", err, status)
	}
	defer conn.Close()

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read welcome message: %v", err)
	}
	if msg.Type != "system" {
		t.Fatalf("expected system welcome message, got %#v", msg)
	}
	if msg.SessionID != "paired-session" {
		t.Fatalf("expected paired session id, got %q", msg.SessionID)
	}
}

func TestWSChatAllowsRequestedLegacyGatewaySessionWithEmptySource(t *testing.T) {
	s, token := newAuthedTestServer(t)

	legacySession, err := s.sessionMgr.GetWithSource("legacy-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}
	legacySession.Source = ""

	server := httptest.NewServer(s.mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat?session_id=" + url.QueryEscape("legacy-session")
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := "<nil>"
		if resp != nil {
			status = fmt.Sprintf("%d", resp.StatusCode)
		}
		t.Fatalf("websocket dial failed: %v (status=%s)", err, status)
	}
	defer conn.Close()

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read welcome message: %v", err)
	}
	if msg.SessionID != "legacy-session" {
		t.Fatalf("expected legacy session id, got %q", msg.SessionID)
	}
}

func TestWSChatAllowsOnlyOneConcurrentAttachForRequestedSession(t *testing.T) {
	s, token := newAuthedTestServer(t)

	if _, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway); err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	var ready sync.WaitGroup
	ready.Add(1)
	release := make(chan struct{})
	var firstHookHit atomic.Bool
	s.beforeWSUpgrade = func(sessionID string) {
		if sessionID != "paired-session" {
			return
		}
		if !firstHookHit.CompareAndSwap(false, true) {
			return
		}
		ready.Done()
		<-release
	}

	server := httptest.NewServer(s.mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat?session_id=" + url.QueryEscape("paired-session")
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	type dialResult struct {
		conn *websocket.Conn
		resp *http.Response
		err  error
	}

	results := make(chan dialResult, 2)
	for i := 0; i < 2; i++ {
		go func() {
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			results <- dialResult{conn: conn, resp: resp, err: err}
		}()
	}

	ready.Wait()
	close(release)

	firstResult := <-results
	secondResult := <-results

	successes := 0
	conflicts := 0
	for _, result := range []dialResult{firstResult, secondResult} {
		if result.err == nil {
			successes++
			defer result.conn.Close()
			var msg WSMessage
			if err := result.conn.ReadJSON(&msg); err != nil {
				t.Fatalf("read welcome message: %v", err)
			}
			if msg.SessionID != "paired-session" {
				t.Fatalf("expected paired session id, got %q", msg.SessionID)
			}
			continue
		}
		if result.resp == nil {
			t.Fatalf("expected handshake response for failed dial, got nil error=%v", result.err)
		}
		if result.resp.StatusCode == http.StatusConflict {
			conflicts++
			continue
		}
		t.Fatalf("expected failed dial to return 409, got %d", result.resp.StatusCode)
	}

	if successes != 1 {
		t.Fatalf("expected exactly one successful attach, got %d", successes)
	}
	if conflicts != 1 {
		t.Fatalf("expected exactly one conflicting attach, got %d", conflicts)
	}
}

func TestWSChatConcurrentAttachConflictKeepsRequestedExistingSession(t *testing.T) {
	s, _ := newAuthedTestServer(t)

	if _, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway); err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	releaseReservation, err := s.reservePairingSessionID("paired-session", true)
	if err != nil {
		t.Fatalf("reservePairingSessionID first call failed: %v", err)
	}
	defer releaseReservation()

	if _, err := s.reservePairingSessionID("paired-session", true); err == nil {
		t.Fatal("expected second reservePairingSessionID call to conflict")
	}

	if _, err := s.sessionMgr.GetExisting("paired-session"); err != nil {
		t.Fatalf("expected requested existing session to survive concurrent attach conflict, got %v", err)
	}
}

func TestProcessMessagePassesExplicitRuntimeIDToRouter(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{reply: "router reply"}
	s.router = router

	sess, err := s.sessionMgr.GetWithSource("gateway-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	client := &Client{
		id:       "gateway-session",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  sess,
	}

	s.processMessage(client, WSMessage{
		Type:      "message",
		Content:   "hello",
		RuntimeID: "runtime-explicit",
	})

	if got := router.lastRuntimeID; got != "runtime-explicit" {
		t.Fatalf("expected runtime id %q, got %q", "runtime-explicit", got)
	}
}

func TestProcessMessageUsesPairedSessionIDForRouterAndResponse(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{reply: "router reply"}
	s.router = router

	sess, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	client := &Client{
		id:       "connection-1",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  sess,
	}

	s.processMessage(client, WSMessage{
		Type:    "message",
		Content: "hello",
	})

	select {
	case payload := <-client.send:
		var msg WSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal ws message: %v", err)
		}
		if msg.SessionID != "paired-session" {
			t.Fatalf("expected response session_id paired-session, got %q", msg.SessionID)
		}
	default:
		t.Fatal("expected websocket reply")
	}
}

func TestProcessMessageRejectsMismatchedInboundSessionID(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{reply: "router reply"}
	s.router = router

	sess, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	client := &Client{
		id:       "connection-1",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  sess,
	}

	s.processMessage(client, WSMessage{
		Type:      "message",
		Content:   "hello",
		SessionID: "other-session",
	})

	select {
	case payload := <-client.send:
		var msg WSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal ws message: %v", err)
		}
		if msg.Type != "error" {
			t.Fatalf("expected error message, got %#v", msg)
		}
		if !strings.Contains(msg.Content, "session mismatch") {
			t.Fatalf("expected session mismatch error, got %q", msg.Content)
		}
	default:
		t.Fatal("expected websocket error reply")
	}
	if got := router.lastRuntimeID; got != "" {
		t.Fatalf("expected router not to be called, got runtime id %q", got)
	}
}

func TestProcessMessageAllowsMatchingInboundSessionID(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{reply: "router reply"}
	s.router = router

	sess, err := s.sessionMgr.GetWithSource("paired-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	client := &Client{
		id:       "connection-1",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  sess,
	}

	s.processMessage(client, WSMessage{
		Type:      "message",
		Content:   "hello",
		SessionID: "paired-session",
		RuntimeID: "runtime-explicit",
	})

	select {
	case payload := <-client.send:
		var msg WSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal ws message: %v", err)
		}
		if msg.Type != "message" {
			t.Fatalf("expected message reply, got %#v", msg)
		}
		if msg.SessionID != "paired-session" {
			t.Fatalf("expected paired session_id, got %q", msg.SessionID)
		}
	default:
		t.Fatal("expected websocket reply")
	}
	if got := router.lastRuntimeID; got != "runtime-explicit" {
		t.Fatalf("expected runtime id %q, got %q", "runtime-explicit", got)
	}
}

func TestProcessMessageAllowsMatchingInboundSessionIDForUnpairedConnection(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{reply: "router reply"}
	s.router = router

	client := &Client{
		id:       "connection-1",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  &stubGatewaySession{},
	}

	s.processMessage(client, WSMessage{
		Type:      "message",
		Content:   "hello",
		SessionID: "connection-1",
		RuntimeID: "runtime-explicit",
	})

	select {
	case payload := <-client.send:
		var msg WSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal ws message: %v", err)
		}
		if msg.Type != "message" {
			t.Fatalf("expected message reply, got %#v", msg)
		}
		if msg.SessionID != "connection-1" {
			t.Fatalf("expected connection session_id, got %q", msg.SessionID)
		}
	default:
		t.Fatal("expected websocket reply")
	}
	if got := router.lastRuntimeID; got != "runtime-explicit" {
		t.Fatalf("expected runtime id %q, got %q", "runtime-explicit", got)
	}
}

func TestProcessMessageDoesNotFallbackWhenExplicitRuntimeFails(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{err: context.DeadlineExceeded}
	s.router = router

	sess, err := s.sessionMgr.GetWithSource("gateway-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	client := &Client{
		id:       "gateway-session",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  sess,
	}

	s.processMessage(client, WSMessage{
		Type:      "message",
		Content:   "hello",
		RuntimeID: "runtime-explicit",
	})

	select {
	case payload := <-client.send:
		var msg WSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal ws message: %v", err)
		}
		if msg.Type != "error" {
			t.Fatalf("expected error message, got %#v", msg)
		}
	default:
		t.Fatal("expected websocket error message")
	}
}

func TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{reply: "router reply"}
	s.router = router

	inboundHits := 0
	s.bus.RegisterInboundHandler("websocket", func(ctx context.Context, msg *bus.Message) error {
		inboundHits++
		return nil
	})
	if err := s.bus.Start(); err != nil {
		t.Fatalf("start bus: %v", err)
	}
	t.Cleanup(func() {
		if err := s.bus.Stop(); err != nil {
			t.Fatalf("stop bus: %v", err)
		}
	})

	sess, err := s.sessionMgr.GetWithSource("gateway-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	client := &Client{
		id:       "gateway-session",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  sess,
	}

	s.processMessage(client, WSMessage{
		Type:    "message",
		Content: "hello",
	})

	if inboundHits != 0 {
		t.Fatalf("expected websocket inbound bus path to stay unused, got %d hits", inboundHits)
	}
}

func TestProcessMessageDoesNotFallbackWhenRouterReturnsEmptyReply(t *testing.T) {
	s := newTestServer(t)
	router := &stubGatewayRouter{}
	s.router = router

	sess, err := s.sessionMgr.GetWithSource("gateway-session", session.SourceGateway)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	client := &Client{
		id:       "gateway-session",
		send:     make(chan []byte, 1),
		userID:   "user-1",
		username: "alice",
		session:  sess,
	}

	s.processMessage(client, WSMessage{
		Type:    "message",
		Content: "hello",
	})

	select {
	case payload := <-client.send:
		var msg WSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal ws message: %v", err)
		}
		if msg.Type != "error" {
			t.Fatalf("expected error message, got %#v", msg)
		}
	default:
		t.Fatal("expected websocket error message")
	}
}

func TestGatewayCheckOriginAllowsConfiguredOrigins(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.AllowedOrigins = []string{
		"https://allowed.example.com",
		"https://console.example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Origin", "https://allowed.example.com")
	if !s.checkOrigin(req) {
		t.Fatal("expected configured origin to be allowed")
	}

	req = httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Origin", "https://blocked.example.com")
	if s.checkOrigin(req) {
		t.Fatal("expected unconfigured origin to be rejected")
	}
}

func TestGatewayCheckOriginAllowsRequestsWithoutOrigin(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.AllowedOrigins = []string{"https://allowed.example.com"}

	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	if !s.checkOrigin(req) {
		t.Fatal("expected empty origin to be allowed for non-browser clients")
	}
}

func TestGatewayCheckClientIPAllowsRequestsWhenListUnset(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.RemoteAddr = "203.0.113.10:4321"

	if err := s.checkClientIP(req); err != nil {
		t.Fatalf("expected empty allowlist to permit request, got %v", err)
	}
}

func TestGatewayCheckClientIPAllowsConfiguredIP(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.AllowedIPs = []string{"203.0.113.10", "::1"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.RemoteAddr = "203.0.113.10:4321"

	if err := s.checkClientIP(req); err != nil {
		t.Fatalf("expected configured ip to be allowed, got %v", err)
	}
}

func TestGatewayCheckClientIPRejectsUnconfiguredIP(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.AllowedIPs = []string{"203.0.113.10"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.RemoteAddr = "198.51.100.7:4321"

	if err := s.checkClientIP(req); err == nil {
		t.Fatal("expected unconfigured ip to be rejected")
	}
}

func TestGatewayStatusEndpointRejectsDisallowedIP(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Gateway.AllowedIPs = []string{"203.0.113.10"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "198.51.100.7:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGatewayStatusEndpointAllowsConfiguredIP(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Gateway.AllowedIPs = []string{"203.0.113.10"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "203.0.113.10:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestWSChatRejectsDisallowedIP(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Gateway.AllowedIPs = []string{"203.0.113.10"}

	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "198.51.100.7:4321"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGatewayRejectsConnectionsAboveConfiguredLimit(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.MaxConnections = 1
	s.clients["client-a"] = &Client{id: "client-a", send: make(chan []byte, 1)}

	if err := s.checkConnectionLimit(); err == nil {
		t.Fatal("expected connection limit error")
	}
}

func TestGatewayAllowsConnectionsWhenLimitUnset(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.MaxConnections = 0
	s.clients["client-a"] = &Client{id: "client-a", send: make(chan []byte, 1)}
	s.clients["client-b"] = &Client{id: "client-b", send: make(chan []byte, 1)}

	if err := s.checkConnectionLimit(); err != nil {
		t.Fatalf("expected unlimited connections, got %v", err)
	}
}

func TestGatewayRateLimitAllowsRequestsWhenUnset(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.RemoteAddr = "203.0.113.10:4321"

	if err := s.checkRateLimit(req); err != nil {
		t.Fatalf("expected unset rate limit to allow request, got %v", err)
	}
	if err := s.checkRateLimit(req); err != nil {
		t.Fatalf("expected repeated request to pass when rate limit disabled, got %v", err)
	}
}

func TestGatewayRateLimitRejectsSecondRequestFromSameIP(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.RateLimitPerMinute = 1
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.RemoteAddr = "203.0.113.10:4321"

	if err := s.checkRateLimit(req); err != nil {
		t.Fatalf("expected first request to pass, got %v", err)
	}
	if err := s.checkRateLimit(req); err == nil {
		t.Fatal("expected second request from same ip to be rate limited")
	}
}

func TestGatewayRateLimitUsesPerIPBuckets(t *testing.T) {
	s := newTestServer(t)
	s.config.Gateway.RateLimitPerMinute = 1

	reqA := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	reqA.RemoteAddr = "203.0.113.10:4321"
	reqB := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	reqB.RemoteAddr = "198.51.100.7:4321"

	if err := s.checkRateLimit(reqA); err != nil {
		t.Fatalf("expected first ip request to pass, got %v", err)
	}
	if err := s.checkRateLimit(reqB); err != nil {
		t.Fatalf("expected second ip to have an independent bucket, got %v", err)
	}
}

func TestGatewayStatusEndpointRejectsRateLimitedRequest(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Gateway.RateLimitPerMinute = 1

	first := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	first.Header.Set("Authorization", "Bearer "+token)
	first.RemoteAddr = "203.0.113.10:4321"
	firstRec := httptest.NewRecorder()
	s.mux.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", firstRec.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	second.Header.Set("Authorization", "Bearer "+token)
	second.RemoteAddr = "203.0.113.10:4321"
	secondRec := httptest.NewRecorder()
	s.mux.ServeHTTP(secondRec, second)

	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request 429, got %d", secondRec.Code)
	}
}

func TestWSChatRejectsRateLimitedRequest(t *testing.T) {
	s, token := newAuthedTestServer(t)
	s.config.Gateway.RateLimitPerMinute = 1

	first := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	first.Header.Set("Authorization", "Bearer "+token)
	first.RemoteAddr = "203.0.113.10:4321"
	firstRec := httptest.NewRecorder()
	s.mux.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first request to seed limiter successfully, got %d", firstRec.Code)
	}

	wsReq := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	wsReq.Header.Set("Authorization", "Bearer "+token)
	wsReq.RemoteAddr = "203.0.113.10:4321"
	wsRec := httptest.NewRecorder()
	s.mux.ServeHTTP(wsRec, wsReq)

	if wsRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected websocket request 429, got %d", wsRec.Code)
	}
}

func TestIsGRPCRequest(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/grpc", nil)
	req.ProtoMajor = 2
	req.Header.Set("Content-Type", "application/grpc+proto")
	if !s.isGRPCRequest(req) {
		t.Fatalf("expected grpc request to be detected")
	}
}
