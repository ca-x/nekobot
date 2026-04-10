package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
)

func TestHandleGetPolicyPresets(t *testing.T) {
	s := &Server{config: config.DefaultConfig()}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/policy/presets", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleGetPolicyPresets(ctx); err != nil {
		t.Fatalf("handleGetPolicyPresets failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"permissive"`) {
		t.Fatalf("expected permissive preset, got %s", rec.Body.String())
	}
}

func TestHandleEvaluatePolicy(t *testing.T) {
	s := &Server{config: config.DefaultConfig()}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/policy/evaluate", strings.NewReader(`{
		"policy":{"name":"restricted","tools":{"deny":["exec"]}},
		"input":{"tool_name":"exec"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleEvaluatePolicy(ctx); err != nil {
		t.Fatalf("handleEvaluatePolicy failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"allowed":false`) {
		t.Fatalf("expected denied evaluation, got %s", rec.Body.String())
	}
}
