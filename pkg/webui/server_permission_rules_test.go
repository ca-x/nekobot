package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/permissionrules"
)

func TestHandleGetPermissionRulesAndCreatePermissionRule(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	rules, err := permissionrules.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new permission rule manager: %v", err)
	}
	if _, err := rules.Create(context.Background(), permissionrules.Rule{
		ToolName:    "exec",
		Action:      permissionrules.ActionAllow,
		Priority:    10,
		Description: "allow exec",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("seed permission rule failed: %v", err)
	}

	s := &Server{config: cfg, logger: log, entClient: client}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/permission-rules", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetPermissionRules(c); err != nil {
		t.Fatalf("handleGetPermissionRules failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal rules failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(listed))
	}

	body := `{"tool_name":"spawn","action":"ask","priority":55,"session_id":"sess-1","description":"ask for spawn","enabled":true}`
	req = httptest.NewRequest(http.MethodPost, "/api/permission-rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := s.handleCreatePermissionRule(c); err != nil {
		t.Fatalf("handleCreatePermissionRule failed: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateAndDeletePermissionRule(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	rules, err := permissionrules.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new permission rule manager: %v", err)
	}
	created, err := rules.Create(context.Background(), permissionrules.Rule{
		ToolName: "exec",
		Action:   permissionrules.ActionAllow,
		Priority: 10,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("seed permission rule failed: %v", err)
	}

	s := &Server{config: cfg, logger: log, entClient: client}

	e := echo.New()
	body := `{"tool_name":"exec","action":"deny","priority":99,"runtime_id":"runtime-a","description":"deny exec on runtime","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/permission-rules/"+created.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/permission-rules/:id")
	c.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})

	if err := s.handleUpdatePermissionRule(c); err != nil {
		t.Fatalf("handleUpdatePermissionRule failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/permission-rules/"+created.ID, nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/api/permission-rules/:id")
	c.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})

	if err := s.handleDeletePermissionRule(c); err != nil {
		t.Fatalf("handleDeletePermissionRule failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreatePermissionRuleRejectsInvalidInput(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})

	s := &Server{config: cfg, logger: log, entClient: client}
	e := echo.New()

	body := `{"tool_name":"","action":"allow","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/permission-rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleCreatePermissionRule(c); err != nil {
		t.Fatalf("handleCreatePermissionRule failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	body = `{"tool_name":"exec","action":"weird","enabled":true}`
	req = httptest.NewRequest(http.MethodPost, "/api/permission-rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := s.handleCreatePermissionRule(c); err != nil {
		t.Fatalf("handleCreatePermissionRule failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid action, got %d: %s", rec.Code, rec.Body.String())
	}
}
