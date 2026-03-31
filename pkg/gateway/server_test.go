package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/session"
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
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
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
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/connections", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body []map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(body))
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
