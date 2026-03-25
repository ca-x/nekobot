package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/skills"
	"nekobot/pkg/workspace"
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

func TestHandleQMDStatusAndUpdate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.QMD.Enabled = true
	cfg.Memory.QMD.Command = "definitely-missing-qmd"

	tmpDir := t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(tmpDir, "workspace")

	log := newTestLogger(t)
	s := &Server{
		config:    cfg,
		logger:    log,
		skillsMgr: skills.NewManager(log, filepath.Join(cfg.WorkspacePath(), "skills"), false),
		workspace: workspace.NewManager(cfg.WorkspacePath(), log),
		startedAt: time.Now().Add(-2 * time.Second),
	}

	e := echo.New()

	statusReq := httptest.NewRequest(http.MethodGet, "/api/memory/qmd/status", nil)
	statusRec := httptest.NewRecorder()
	statusCtx := e.NewContext(statusReq, statusRec)
	if err := s.handleGetQMDStatus(statusCtx); err != nil {
		t.Fatalf("handleGetQMDStatus failed: %v", err)
	}
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusRec.Code)
	}

	var statusPayload map[string]interface{}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusPayload); err != nil {
		t.Fatalf("unmarshal qmd status payload failed: %v", err)
	}
	if enabled, _ := statusPayload["enabled"].(bool); !enabled {
		t.Fatalf("expected qmd enabled in payload")
	}
	if available, _ := statusPayload["available"].(bool); available {
		t.Fatalf("expected qmd unavailable for missing command")
	}
	if exportDir, _ := statusPayload["session_export_dir"].(string); strings.TrimSpace(exportDir) == "" {
		t.Fatalf("expected qmd session export dir in payload")
	}
	if _, ok := statusPayload["collections"]; !ok {
		t.Fatalf("expected collections in payload")
	}

	updateReq := httptest.NewRequest(http.MethodPost, "/api/memory/qmd/update", nil)
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	if err := s.handleUpdateQMD(updateCtx); err != nil {
		t.Fatalf("handleUpdateQMD failed: %v", err)
	}
	if updateRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, updateRec.Code)
	}

	originalNPMCommand := defaultNPMCommand
	defaultNPMCommand = "definitely-missing-npm"
	t.Cleanup(func() {
		defaultNPMCommand = originalNPMCommand
	})

	installReq := httptest.NewRequest(http.MethodPost, "/api/memory/qmd/install", nil)
	installRec := httptest.NewRecorder()
	installCtx := e.NewContext(installReq, installRec)
	if err := s.handleInstallQMD(installCtx); err != nil {
		t.Fatalf("handleInstallQMD failed: %v", err)
	}
	if installRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, installRec.Code)
	}
}

func TestHandleCleanupQMDSessionExports(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.QMD.Enabled = true
	cfg.Memory.QMD.Sessions.Enabled = true
	cfg.Memory.QMD.Sessions.RetentionDays = 1

	tmpDir := t.TempDir()
	exportDir := filepath.Join(tmpDir, "exports")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatalf("mkdir export dir: %v", err)
	}
	oldFile := filepath.Join(exportDir, "old.md")
	if err := os.WriteFile(oldFile, []byte("# old"), 0o644); err != nil {
		t.Fatalf("write old export: %v", err)
	}
	oldTime := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old export: %v", err)
	}
	newFile := filepath.Join(exportDir, "new.md")
	if err := os.WriteFile(newFile, []byte("# new"), 0o644); err != nil {
		t.Fatalf("write new export: %v", err)
	}

	cfg.Memory.QMD.Sessions.ExportDir = exportDir
	cfg.Agents.Defaults.Workspace = filepath.Join(tmpDir, "workspace")

	s := &Server{
		config: cfg,
		logger: newTestLogger(t),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/memory/qmd/sessions/cleanup", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleCleanupQMDSessionExports(ctx); err != nil {
		t.Fatalf("handleCleanupQMDSessionExports failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal cleanup payload failed: %v", err)
	}
	if deleted, _ := payload["deleted"].(float64); deleted != 1 {
		t.Fatalf("expected 1 deleted export, got %+v", payload)
	}
	if remaining, _ := payload["remaining"].(float64); remaining != 1 {
		t.Fatalf("expected 1 remaining export, got %+v", payload)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old export to be deleted, err=%v", err)
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("expected new export to remain: %v", err)
	}
}
