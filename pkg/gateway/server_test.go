package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

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
		config:  cfg,
		logger:  log,
		bus:     localBus,
		clients: make(map[string]*Client),
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
	json.NewDecoder(rec.Body).Decode(&body)
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
	json.NewDecoder(rec.Body).Decode(&body)

	if body["version"] != "0.11.0-alpha" {
		t.Fatalf("expected version 0.11.0-alpha, got %v", body["version"])
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
	json.NewDecoder(rec.Body).Decode(&body)
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

func TestSimpleSession(t *testing.T) {
	sess := &simpleSession{}

	if len(sess.GetMessages()) != 0 {
		t.Fatal("expected empty session")
	}

	sess.AddMessage(agent.Message{Role: "user", Content: "hello"})
	sess.AddMessage(agent.Message{Role: "assistant", Content: "hi"})

	msgs := sess.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatal("unexpected message roles")
	}
}
