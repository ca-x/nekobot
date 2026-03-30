package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/audit"
)

func TestHandleGetHarnessAudit(t *testing.T) {
	log := newTestLogger(t)
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	auditLogger.Log(&audit.Entry{
		Timestamp:     time.Now().Add(-2 * time.Minute),
		ToolName:      "watch",
		DurationMs:    48,
		Success:       true,
		ResultPreview: "go test ./...",
		SessionID:     "session-1",
	})
	auditLogger.Log(&audit.Entry{
		Timestamp:  time.Now().Add(-1 * time.Minute),
		ToolName:   "undo",
		DurationMs: 12,
		Success:    false,
		Error:      "snapshot missing",
		SessionID:  "session-2",
	})

	s := &Server{
		logger:      log,
		auditLogger: auditLogger,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/harness/audit?limit=1", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleGetHarnessAudit(ctx); err != nil {
		t.Fatalf("handleGetHarnessAudit failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Entries []audit.Entry          `json:"entries"`
		Stats   map[string]interface{} `json:"stats"`
		Limit   int                    `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal audit payload failed: %v", err)
	}
	if payload.Limit != 1 {
		t.Fatalf("unexpected limit: %d", payload.Limit)
	}
	if len(payload.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(payload.Entries))
	}
	if payload.Entries[0].ToolName != "undo" {
		t.Fatalf("expected latest entry, got %+v", payload.Entries[0])
	}
	if entries, ok := payload.Stats["entries"].(float64); !ok || entries != 2 {
		t.Fatalf("unexpected stats: %+v", payload.Stats)
	}
}

func TestHandleClearHarnessAudit(t *testing.T) {
	log := newTestLogger(t)
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)
	auditLogger.Log(&audit.Entry{
		Timestamp:  time.Now(),
		ToolName:   "watch",
		DurationMs: 88,
		Success:    true,
	})

	s := &Server{
		logger:      log,
		auditLogger: auditLogger,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/harness/audit/clear", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleClearHarnessAudit(ctx); err != nil {
		t.Fatalf("handleClearHarnessAudit failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Status string                 `json:"status"`
		Stats  map[string]interface{} `json:"stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal clear audit payload failed: %v", err)
	}
	if payload.Status != "cleared" {
		t.Fatalf("unexpected status: %q", payload.Status)
	}
	if entries, ok := payload.Stats["entries"].(float64); !ok || entries != 0 {
		t.Fatalf("unexpected stats after clear: %+v", payload.Stats)
	}
	if exists, ok := payload.Stats["exists"].(bool); !ok || exists {
		t.Fatalf("expected removed audit file stats, got %+v", payload.Stats)
	}
}
