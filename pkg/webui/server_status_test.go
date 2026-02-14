package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
)

func TestHandleStatus_ReturnsExtendedFields(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = 18790
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic"},
	}

	s := &Server{
		config:    cfg,
		startedAt: time.Now().Add(-3 * time.Second),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleStatus(c); err != nil {
		t.Fatalf("handleStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status payload failed: %v", err)
	}

	required := []string{
		"version",
		"commit",
		"build_time",
		"os",
		"arch",
		"go_version",
		"pid",
		"uptime",
		"uptime_seconds",
		"memory_alloc_bytes",
		"memory_sys_bytes",
		"provider_count",
		"gateway_host",
		"gateway_port",
	}
	for _, key := range required {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in payload, got: %v", key, payload)
		}
	}
}
