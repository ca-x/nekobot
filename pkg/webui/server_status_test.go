package webui

import (
	"encoding/json"
	"errors"
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
	"nekobot/pkg/watch"
	"nekobot/pkg/workspace"
)

type stubGatewayServiceController struct {
	status       map[string]interface{}
	statusErr    error
	restartErr   error
	restartCalls int
}

func (s *stubGatewayServiceController) Status() (map[string]interface{}, error) {
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	return s.status, nil
}

func (s *stubGatewayServiceController) Restart() error {
	s.restartCalls++
	return s.restartErr
}

func TestHandleStatus_ReturnsExtendedFields(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = 18790
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic"},
	}

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	loader := config.NewLoader()
	if _, err := loader.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	runtimeDBPath, err := config.RuntimeDBPath(cfg)
	if err != nil {
		t.Fatalf("RuntimeDBPath failed: %v", err)
	}

	s := &Server{
		config:    cfg,
		loader:    loader,
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
		"config_path",
		"database_dir",
		"runtime_db_path",
		"workspace_path",
		"gateway_host",
		"gateway_port",
	}
	for _, key := range required {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in payload, got: %v", key, payload)
		}
	}
	if payload["config_path"] != configPath {
		t.Fatalf("unexpected config_path: %+v", payload["config_path"])
	}
	if payload["database_dir"] != cfg.Storage.DBDir {
		t.Fatalf("unexpected database_dir: %+v", payload["database_dir"])
	}
	if payload["runtime_db_path"] != runtimeDBPath {
		t.Fatalf("unexpected runtime_db_path: %+v", payload["runtime_db_path"])
	}
	if payload["workspace_path"] != cfg.Agents.Defaults.Workspace {
		t.Fatalf("unexpected workspace_path: %+v", payload["workspace_path"])
	}
}

func TestNewServer_AllowsNilLoader(t *testing.T) {
	cfg := config.DefaultConfig()

	var recovered interface{}
	func() {
		defer func() {
			recovered = recover()
		}()

		server := NewServer(
			cfg,
			nil,
			newTestLogger(t),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)
		if server == nil {
			t.Fatalf("expected server")
		}
		if server.serviceCtrl == nil {
			t.Fatalf("expected service controller")
		}
		ctrl, ok := server.serviceCtrl.(*gatewayServiceController)
		if !ok {
			t.Fatalf("expected gateway service controller, got %T", server.serviceCtrl)
		}
		if ctrl.configPath != "" {
			t.Fatalf("expected empty config path when loader is nil, got %q", ctrl.configPath)
		}
	}()

	if recovered != nil {
		t.Fatalf("expected NewServer not to panic, got %v", recovered)
	}
}

func TestHandleServiceStatus(t *testing.T) {
	controller := &stubGatewayServiceController{
		status: map[string]interface{}{
			"name":        "nekobot-gateway",
			"installed":   true,
			"status":      "running",
			"config_path": "/tmp/nekobot/config.json",
		},
	}

	s := &Server{
		serviceCtrl: controller,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/service", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleServiceStatus(ctx); err != nil {
		t.Fatalf("handleServiceStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if payload["status"] != "running" || payload["installed"] != true {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestHandleServiceRestart(t *testing.T) {
	controller := &stubGatewayServiceController{}
	s := &Server{
		serviceCtrl: controller,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/service/restart", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleServiceRestart(ctx); err != nil {
		t.Fatalf("handleServiceRestart failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if controller.restartCalls != 1 {
		t.Fatalf("expected restart to be called once, got %d", controller.restartCalls)
	}
}

func TestHandleServiceRestartReturnsError(t *testing.T) {
	controller := &stubGatewayServiceController{restartErr: errors.New("permission denied")}
	s := &Server{
		serviceCtrl: controller,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/service/restart", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleServiceRestart(ctx); err != nil {
		t.Fatalf("handleServiceRestart failed: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}
}

func TestHandleGetWatchStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.DebounceMs = 650
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: "**/*.go",
		Command:  "go test ./...",
	}}

	watcher := &watch.Watcher{}
	watcher.UpdateConfig(cfg.Watch)

	s := &Server{
		config:  cfg,
		watcher: watcher,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/harness/watch", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleGetWatchStatus(ctx); err != nil {
		t.Fatalf("handleGetWatchStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload watch.StatusSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal watch status failed: %v", err)
	}
	if !payload.Enabled || payload.DebounceMs != 650 || len(payload.Patterns) != 1 {
		t.Fatalf("unexpected watch status payload: %+v", payload)
	}
}

func TestHandleUpdateWatchStatusPersistsConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	watcher := &watch.Watcher{}
	watcher.UpdateConfig(cfg.Watch)

	s := &Server{
		config:  cfg,
		logger:  newTestLogger(t),
		watcher: watcher,
	}

	body := `{"enabled":true,"debounce_ms":910,"patterns":[{"file_glob":"frontend/src/**/*.tsx","command":"npm run build","fail_command":"echo fail"}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/harness/watch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleUpdateWatchStatus(ctx); err != nil {
		t.Fatalf("handleUpdateWatchStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if !s.config.Watch.Enabled || s.config.Watch.DebounceMs != 910 || len(s.config.Watch.Patterns) != 1 {
		t.Fatalf("watch config not applied: %+v", s.config.Watch)
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}
	if !reloaded.Watch.Enabled || reloaded.Watch.DebounceMs != 910 || len(reloaded.Watch.Patterns) != 1 {
		t.Fatalf("watch config not persisted: %+v", reloaded.Watch)
	}

	status := watcher.Status()
	if !status.Enabled || status.DebounceMs != 910 || len(status.Patterns) != 1 {
		t.Fatalf("watcher runtime not updated: %+v", status)
	}
}

func TestHandleUpdateWatchStatusStopsWatcherWhenDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Watch.Enabled = true
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.go"),
		Command:  "printf 'watch'",
	}}

	watcher, err := watch.New(cfg, newTestLogger(t), nil)
	if err != nil {
		t.Fatalf("watch.New failed: %v", err)
	}
	if err := watcher.Start(); err != nil {
		t.Fatalf("watcher.Start failed: %v", err)
	}
	t.Cleanup(func() {
		_ = watcher.Stop()
	})

	s := &Server{
		config:  cfg,
		logger:  newTestLogger(t),
		watcher: watcher,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/harness/watch", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleUpdateWatchStatus(ctx); err != nil {
		t.Fatalf("handleUpdateWatchStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	status := watcher.Status()
	if status.Enabled {
		t.Fatalf("expected watcher status to be disabled, got %+v", status)
	}
	if status.Running {
		t.Fatalf("expected watcher to stop when disabled, got %+v", status)
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
