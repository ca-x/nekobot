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

	"nekobot/pkg/accountbindings"
	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/skills"
	"nekobot/pkg/tasks"
	"nekobot/pkg/watch"
	"nekobot/pkg/workspace"
)

type stubGatewayServiceController struct {
	status       map[string]interface{}
	statusErr    error
	restartErr   error
	restartCalls int
	reloadErr    error
	reloadCalls  int
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

func (s *stubGatewayServiceController) Reload() error {
	s.reloadCalls++
	return s.reloadErr
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
		taskStore: tasks.NewStore(),
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
		"task_count",
		"task_state_counts",
		"recent_tasks",
		"agent_definition",
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
	if payload["task_count"].(float64) != 0 {
		t.Fatalf("expected zero task_count, got %+v", payload["task_count"])
	}
	if recentTasks, ok := payload["recent_tasks"].([]interface{}); !ok || len(recentTasks) != 0 {
		t.Fatalf("expected empty recent_tasks, got %+v", payload["recent_tasks"])
	}
	if runtimeStates, ok := payload["runtime_states"].([]interface{}); !ok || len(runtimeStates) != 0 {
		t.Fatalf("expected empty runtime_states, got %+v", payload["runtime_states"])
	}
	if sessionStates, ok := payload["session_runtime_states"].([]interface{}); !ok || len(sessionStates) != 0 {
		t.Fatalf("expected empty session_runtime_states, got %+v", payload["session_runtime_states"])
	}
	if payload["agent_definition"] != nil {
		t.Fatalf("expected nil agent_definition when server has no agent, got %+v", payload["agent_definition"])
	}
}

func TestHandleStatus_IncludesRecentTasks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	now := time.Now()
	store := tasks.NewStore()
	store.SetSource("test", func() []tasks.Task {
		return []tasks.Task{
			{
				ID:        "task-old",
				Type:      tasks.TypeBackgroundAgent,
				State:     tasks.StateCompleted,
				Summary:   "old task",
				CreatedAt: now.Add(-20 * time.Minute),
			},
			{
				ID:        "task-running",
				Type:      tasks.TypeBackgroundAgent,
				State:     tasks.StateRunning,
				Summary:   "running task",
				CreatedAt: now.Add(-10 * time.Minute),
				StartedAt: now.Add(-1 * time.Minute),
			},
			{
				ID:          "task-failed",
				Type:        tasks.TypeBackgroundAgent,
				State:       tasks.StateFailed,
				Summary:     "failed task",
				LastError:   "boom",
				CreatedAt:   now.Add(-15 * time.Minute),
				CompletedAt: now.Add(-30 * time.Second),
			},
		}
	})
	s := &Server{
		config:    cfg,
		startedAt: now.Add(-5 * time.Second),
		taskStore: store,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleStatus(ctx); err != nil {
		t.Fatalf("handleStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		TaskCount       int            `json:"task_count"`
		TaskStateCounts map[string]int `json:"task_state_counts"`
		RecentTasks     []tasks.Task   `json:"recent_tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status payload failed: %v", err)
	}
	if payload.TaskCount != 3 {
		t.Fatalf("expected task_count 3, got %d", payload.TaskCount)
	}
	if payload.TaskStateCounts[string(tasks.StateRunning)] != 1 {
		t.Fatalf("expected running count 1, got %+v", payload.TaskStateCounts)
	}
	if payload.TaskStateCounts[string(tasks.StateFailed)] != 1 {
		t.Fatalf("expected failed count 1, got %+v", payload.TaskStateCounts)
	}
	if len(payload.RecentTasks) != 3 {
		t.Fatalf("expected 3 recent tasks, got %d", len(payload.RecentTasks))
	}
	if payload.RecentTasks[0].ID != "task-failed" {
		t.Fatalf("expected most recent task to be task-failed, got %q", payload.RecentTasks[0].ID)
	}
	if payload.RecentTasks[1].ID != "task-running" {
		t.Fatalf("expected second task to be task-running, got %q", payload.RecentTasks[1].ID)
	}
}

func TestHandleStatus_IncludesAgentDefinition(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Provider = "openai-main"
	cfg.Agents.Defaults.Model = "gpt-5.4"
	cfg.Approval.Mode = "manual"

	log := newTestLogger(t)
	ag, err := agent.New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeManual}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	s := &Server{
		config:    cfg,
		agent:     ag,
		startedAt: time.Now().Add(-2 * time.Second),
		taskStore: tasks.NewStore(),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleStatus(ctx); err != nil {
		t.Fatalf("handleStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		AgentDefinition struct {
			ID             string `json:"id"`
			PermissionMode string `json:"permissionMode"`
			Route          struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
			} `json:"route"`
		} `json:"agent_definition"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status payload failed: %v", err)
	}
	if payload.AgentDefinition.ID != "main" {
		t.Fatalf("expected agent definition id main, got %+v", payload.AgentDefinition)
	}
	if payload.AgentDefinition.Route.Provider != "openai-main" || payload.AgentDefinition.Route.Model != "gpt-5.4" {
		t.Fatalf("unexpected agent definition route: %+v", payload.AgentDefinition.Route)
	}
	if payload.AgentDefinition.PermissionMode != "manual" {
		t.Fatalf("unexpected permission mode: %+v", payload.AgentDefinition.PermissionMode)
	}
}

func TestHandleStatus_IncludesRuntimeStates(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	runtimeMgr, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}
	bindingMgr, err := accountbindings.NewManager(cfg, log, client, runtimeMgr, accountMgr)
	if err != nil {
		t.Fatalf("new binding manager: %v", err)
	}

	runtimeItem, err := runtimeMgr.Create(t.Context(), runtimeagents.AgentRuntime{
		Name:        "ops-runtime",
		DisplayName: "Ops Runtime",
		Enabled:     true,
		Provider:    "openai",
		Model:       "gpt-5",
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	accountItem, err := accountMgr.Create(t.Context(), channelaccounts.ChannelAccount{
		ChannelType: "websocket",
		AccountKey:  "default",
		DisplayName: "Gateway Default",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := bindingMgr.Create(t.Context(), accountbindings.AccountBinding{
		ChannelAccountID: accountItem.ID,
		AgentRuntimeID:   runtimeItem.ID,
		BindingMode:      accountbindings.ModeSingleAgent,
		Enabled:          true,
		Priority:         100,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	store := tasks.NewStore()
	store.SetSource("runtime", func() []tasks.Task {
		return []tasks.Task{{
			ID:        "task-1",
			Type:      tasks.TypeRuntimeWorker,
			State:     tasks.StateRunning,
			RuntimeID: runtimeItem.ID,
			Summary:   "runtime task",
			StartedAt: time.Now().Add(-30 * time.Second),
			CreatedAt: time.Now().Add(-1 * time.Minute),
		}}
	})

	s := &Server{
		config:     cfg,
		startedAt:  time.Now().Add(-5 * time.Second),
		taskStore:  store,
		runtimeMgr: runtimeMgr,
		accountMgr: accountMgr,
		bindingMgr: bindingMgr,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleStatus(ctx); err != nil {
		t.Fatalf("handleStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		RuntimeStates []runtimeagents.AgentRuntime `json:"runtime_states"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status payload failed: %v", err)
	}
	if len(payload.RuntimeStates) != 1 {
		t.Fatalf("expected one runtime state, got %d", len(payload.RuntimeStates))
	}
	status := payload.RuntimeStates[0].Status
	if status == nil {
		t.Fatalf("expected runtime status")
	}
	if !status.EffectiveAvailable || status.EnabledBindingCount != 1 || status.CurrentTaskCount != 1 {
		t.Fatalf("unexpected runtime status: %+v", status)
	}
	if status.AvailabilityReason != "available" {
		t.Fatalf("unexpected availability reason: %q", status.AvailabilityReason)
	}
}

func TestHandleStatus_IncludesSessionRuntimeStates(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	store := tasks.NewStore()
	store.SetSessionPermissionMode("sess-1", "manual")
	store.SetSessionPendingAction("sess-1", "exec", "approval-1")

	s := &Server{
		config:    cfg,
		startedAt: time.Now().Add(-5 * time.Second),
		taskStore: store,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleStatus(ctx); err != nil {
		t.Fatalf("handleStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		SessionRuntimeStates []tasks.SessionState `json:"session_runtime_states"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status payload failed: %v", err)
	}
	if len(payload.SessionRuntimeStates) != 1 {
		t.Fatalf("expected one session runtime state, got %d", len(payload.SessionRuntimeStates))
	}
	if payload.SessionRuntimeStates[0].SessionID != "sess-1" {
		t.Fatalf("expected session id sess-1, got %q", payload.SessionRuntimeStates[0].SessionID)
	}
	if payload.SessionRuntimeStates[0].PermissionMode != "manual" {
		t.Fatalf("expected manual permission mode, got %q", payload.SessionRuntimeStates[0].PermissionMode)
	}
	if payload.SessionRuntimeStates[0].PendingRequestID != "approval-1" {
		t.Fatalf("expected approval request id approval-1, got %q", payload.SessionRuntimeStates[0].PendingRequestID)
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

func TestHandleServiceReload(t *testing.T) {
	controller := &stubGatewayServiceController{}
	s := &Server{
		serviceCtrl: controller,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/service/reload", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleServiceReload(ctx); err != nil {
		t.Fatalf("handleServiceReload failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if controller.reloadCalls != 1 {
		t.Fatalf("expected reload to be called once, got %d", controller.reloadCalls)
	}
}

func TestHandleServiceReloadReturnsError(t *testing.T) {
	controller := &stubGatewayServiceController{reloadErr: errors.New("reload failed")}
	s := &Server{
		serviceCtrl: controller,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/service/reload", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleServiceReload(ctx); err != nil {
		t.Fatalf("handleServiceReload failed: %v", err)
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
