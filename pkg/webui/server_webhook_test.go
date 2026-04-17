package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/session"
)

func TestHandleGetConfigIncludesWebhookSection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Webhook.Enabled = true
	cfg.Webhook.Path = "/api/webhooks/agent"

	s := &Server{config: cfg}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleGetConfig(ctx); err != nil {
		t.Fatalf("handleGetConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"webhook"`) {
		t.Fatalf("expected webhook section, got %s", rec.Body.String())
	}
}

func TestConfiguredWebhookPathIsRegisteredAndRoutesToHandler(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Webhook.Enabled = true
	cfg.Webhook.Path = "/api/webhooks/agent"

	sessMgr := session.NewManager(t.TempDir(), cfg.Sessions)
	server := &Server{
		config:     cfg,
		sessionMgr: sessMgr,
		webhookTestHandler: func(ctx context.Context, username, message string) (string, error) {
			return "echo: " + username + ": " + message, nil
		},
	}
	server.setup()

	token, err := server.generateToken(&config.AuthProfile{Username: "alice", Role: "owner"})
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/agent", strings.NewReader(`{"message":"hello webhook"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok status, got %#v", body["status"])
	}
	if body["session_id"] != "webhook:alice" {
		t.Fatalf("expected webhook session id, got %#v", body["session_id"])
	}
	if !strings.Contains(rec.Body.String(), "echo: alice: hello webhook") {
		t.Fatalf("expected webhook reply, got %s", rec.Body.String())
	}
}

func TestHandleTestWebhookRoutesMessageToAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Webhook.Enabled = true

	sessMgr := session.NewManager(t.TempDir(), cfg.Sessions)

	server := &Server{
		config:     cfg,
		sessionMgr: sessMgr,
		webhookTestHandler: func(ctx context.Context, username, message string) (string, error) {
			return "echo: " + username + ": " + message, nil
		},
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/test", strings.NewReader(`{"message":"hello webhook"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/webhooks/test")

	if err := server.handleTestWebhook(ctx); err != nil {
		t.Fatalf("handleTestWebhook failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected ok status, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"session_id":"webhook:alice"`) {
		t.Fatalf("expected webhook session id, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `echo: alice: hello webhook`) {
		t.Fatalf("expected webhook reply, got %s", rec.Body.String())
	}
}

func TestHandleTestWebhookRejectsWhenDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Webhook.Enabled = false

	server := &Server{config: cfg}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/test", strings.NewReader(`{"message":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/webhooks/test")

	if err := server.handleTestWebhook(ctx); err != nil {
		t.Fatalf("handleTestWebhook failed: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTestWebhookReturnsConflictWhenNoProvidersConfigured(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Webhook.Enabled = true

	sessMgr := session.NewManager(t.TempDir(), cfg.Sessions)
	server := &Server{
		config:     cfg,
		sessionMgr: sessMgr,
		webhookTestHandler: func(ctx context.Context, username, message string) (string, error) {
			return "", fmt.Errorf("no providers configured")
		},
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/test", strings.NewReader(`{"message":"hello webhook"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/webhooks/test")

	if err := server.handleTestWebhook(ctx); err != nil {
		t.Fatalf("handleTestWebhook failed: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no providers configured") {
		t.Fatalf("expected no providers configured error, got %s", rec.Body.String())
	}
}
