package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/session"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/version"
)

type stubGatewayRouter struct {
	lastRuntimeID string
	reply         string
	err           error
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
}

func TestConnectionsEndpoint(t *testing.T) {
	s, token := newAuthedTestServer(t)

	now := time.Unix(1_700_000_000, 0).UTC()
	s.clients["client-b"] = &Client{
		id:          "client-b",
		send:        make(chan []byte, 1),
		userID:      "user-b",
		username:    "bob",
		connectedAt: now.Add(2 * time.Minute),
		remoteAddr:  "10.0.0.2:1234",
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
		id:          "client-a",
		send:        make(chan []byte, 1),
		userID:      "viewer-id",
		username:    "viewer",
		connectedAt: time.Unix(1_700_000_000, 0).UTC(),
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

func TestDeleteConnectionEndpointRequiresAuth(t *testing.T) {
	s, _ := newAuthedTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/test-client", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestDeleteConnectionEndpointRejectsMemberRole(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"role": "member",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/connections/test-client", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
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
}

func TestGetConnectionEndpointAllowsMemberRoleForOwnedConnection(t *testing.T) {
	s, _ := newAuthedTestServer(t)
	token := signGatewayTestToken(t, jwt.MapClaims{
		"sub":  "viewer",
		"uid":  "viewer-id",
		"role": "member",
	})
	s.clients["client-a"] = &Client{
		id:       "client-a",
		send:     make(chan []byte, 1),
		userID:   "viewer-id",
		username: "viewer",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections/client-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
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
