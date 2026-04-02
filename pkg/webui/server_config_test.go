package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providerstore"
	"nekobot/pkg/skills"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/watch"
)

func TestHandleGetConfigIncludesMemorySection(t *testing.T) {
	s := &Server{
		config: config.DefaultConfig(),
	}
	s.config.Memory.Semantic.SearchPolicy = "vector"
	s.config.Memory.ShortTerm.RawHistoryLimit = 77
	s.config.Agents.Defaults.SkillsProxy = "http://127.0.0.1:9000"

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetConfig(c); err != nil {
		t.Fatalf("handleGetConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if _, ok := payload["memory"]; !ok {
		t.Fatalf("expected memory section in response: %s", rec.Body.String())
	}

	var memory config.MemoryConfig
	if err := json.Unmarshal(payload["memory"], &memory); err != nil {
		t.Fatalf("unmarshal memory section failed: %v", err)
	}
	if memory.Semantic.SearchPolicy != "vector" || memory.ShortTerm.RawHistoryLimit != 77 {
		t.Fatalf("unexpected memory payload: %+v", memory)
	}

	var agents config.AgentsConfig
	if err := json.Unmarshal(payload["agents"], &agents); err != nil {
		t.Fatalf("unmarshal agents section failed: %v", err)
	}
	if agents.Defaults.SkillsProxy != "http://127.0.0.1:9000" {
		t.Fatalf("unexpected skills proxy payload: %+v", agents.Defaults)
	}
	if _, ok := payload["storage"]; !ok {
		t.Fatalf("expected storage section in response: %s", rec.Body.String())
	}
	if _, ok := payload["redis"]; !ok {
		t.Fatalf("expected redis section in response: %s", rec.Body.String())
	}
	if _, ok := payload["state"]; !ok {
		t.Fatalf("expected state section in response: %s", rec.Body.String())
	}
	if _, ok := payload["bus"]; !ok {
		t.Fatalf("expected bus section in response: %s", rec.Body.String())
	}
	for _, section := range []string{"audit", "undo", "preprocess", "learnings", "watch"} {
		if _, ok := payload[section]; !ok {
			t.Fatalf("expected %s section in response: %s", section, rec.Body.String())
		}
	}
}

func TestHandleSaveConfigPersistsStartupSections(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	loader := config.NewLoader()
	if _, err := loader.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Fatalf("close provider manager: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		loader:    loader,
		logger:    log,
		providers: providers,
	}

	newDBDir := filepath.Join(t.TempDir(), "migrated-db")
	body := `{"storage":{"db_dir":"` + newDBDir + `"},"logger":{"level":"debug","output_path":"","max_size":0,"max_backups":0,"max_age":0,"compress":false},"gateway":{"host":"0.0.0.0","port":19090},"webui":{"enabled":true,"port":19191,"public_base_url":"https://bot.example.com","tool_session_otp_ttl_seconds":180,"tool_session_events":{"enabled":true,"retention_days":14},"skill_snapshots":{"auto_prune":true,"max_count":10},"skill_versions":{"enabled":true,"max_count":20}},"redis":{"addr":"127.0.0.1:6380","password":"pw","db":9},"state":{"backend":"redis","file_path":"/tmp/state.json","prefix":"state:"},"bus":{"type":"redis","prefix":"bus:"}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleSaveConfig(c); err != nil {
		t.Fatalf("handleSaveConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Status          string   `json:"status"`
		RestartRequired bool     `json:"restart_required"`
		RestartSections []string `json:"restart_sections"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if payload.Status != "saved" {
		t.Fatalf("unexpected response payload: %+v", payload)
	}
	if !payload.RestartRequired {
		t.Fatalf("expected restart_required=true, got %+v", payload)
	}
	for _, section := range []string{"storage", "logger", "gateway", "webui"} {
		if !containsString(payload.RestartSections, section) {
			t.Fatalf("expected restart section %q, got %+v", section, payload.RestartSections)
		}
	}

	if s.config.Storage.DBDir != newDBDir {
		t.Fatalf("storage not applied: %+v", s.config.Storage)
	}
	if s.config.Logger.Level != "debug" {
		t.Fatalf("logger not applied: %+v", s.config.Logger)
	}
	if s.config.Gateway.Host != "0.0.0.0" || s.config.Gateway.Port != 19090 {
		t.Fatalf("gateway not applied: %+v", s.config.Gateway)
	}
	if s.config.WebUI.Port != 19191 || s.config.WebUI.PublicBaseURL != "https://bot.example.com" {
		t.Fatalf("webui not applied: %+v", s.config.WebUI)
	}
	if s.config.Redis.Addr != "127.0.0.1:6380" || s.config.Redis.DB != 9 {
		t.Fatalf("redis not applied: %+v", s.config.Redis)
	}
	if s.config.State.Backend != "redis" || s.config.State.Prefix != "state:" {
		t.Fatalf("state not applied: %+v", s.config.State)
	}
	if s.config.Bus.Type != "redis" || s.config.Bus.Prefix != "bus:" {
		t.Fatalf("bus not applied: %+v", s.config.Bus)
	}

	bootstrapReloaded, err := config.NewLoader().LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("reload bootstrap config failed: %v", err)
	}
	if bootstrapReloaded.Storage.DBDir != newDBDir {
		t.Fatalf("storage not persisted to bootstrap config: %+v", bootstrapReloaded.Storage)
	}
	if bootstrapReloaded.Logger.Level != "debug" {
		t.Fatalf("logger not persisted to bootstrap config: %+v", bootstrapReloaded.Logger)
	}
	if bootstrapReloaded.Gateway.Host != "0.0.0.0" || bootstrapReloaded.Gateway.Port != 19090 {
		t.Fatalf("gateway not persisted to bootstrap config: %+v", bootstrapReloaded.Gateway)
	}
	if bootstrapReloaded.WebUI.Port != 19191 || bootstrapReloaded.WebUI.PublicBaseURL != "https://bot.example.com" {
		t.Fatalf("webui not persisted to bootstrap config: %+v", bootstrapReloaded.WebUI)
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}

	if reloaded.Redis.Addr != "127.0.0.1:6380" || reloaded.Redis.DB != 9 {
		t.Fatalf("redis not persisted: %+v", reloaded.Redis)
	}
	if reloaded.State.Backend != "redis" || reloaded.State.Prefix != "state:" {
		t.Fatalf("state not persisted: %+v", reloaded.State)
	}
	if reloaded.Bus.Type != "redis" || reloaded.Bus.Prefix != "bus:" {
		t.Fatalf("bus not persisted: %+v", reloaded.Bus)
	}
}

func TestHandleSaveConfigPersistsMemorySection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providers,
	}

	body := `{"agents":{"defaults":{"workspace":"` + cfg.Agents.Defaults.Workspace + `","restrict_to_workspace":true,"provider":"","fallback":[],"provider_groups":[{"name":"openai-pool","strategy":"round_robin","members":["openai-a","openai-b"]}],"orchestrator":"blades","model":"claude-sonnet-4-5-20250929","max_tokens":8192,"temperature":0.7,"max_tool_iterations":20,"skills_dir":"","skills_auto_reload":false,"skills_proxy":"http://127.0.0.1:9001","extended_thinking":false,"thinking_budget":0,"mcp_servers":[]}},"memory":{"enabled":true,"semantic":{"enabled":true,"default_top_k":7,"max_top_k":25,"search_policy":"vector","include_scores":true},"episodic":{"enabled":true,"summary_window_messages":30,"max_summaries":400},"short_term":{"enabled":true,"raw_history_limit":333},"qmd":{"enabled":false,"command":"qmd","include_default":false,"paths":[],"sessions":{"enabled":false,"export_dir":"","retention_days":0},"update":{"on_boot":false,"interval":"30m","command_timeout":"30s","update_timeout":"5m"}}}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleSaveConfig(c); err != nil {
		t.Fatalf("handleSaveConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if s.config.Memory.Semantic.SearchPolicy != "vector" || s.config.Memory.ShortTerm.RawHistoryLimit != 333 {
		t.Fatalf("memory not applied to runtime config: %+v", s.config.Memory)
	}
	if s.config.Agents.Defaults.SkillsProxy != "http://127.0.0.1:9001" {
		t.Fatalf("skills proxy not applied to runtime config: %+v", s.config.Agents.Defaults)
	}
	if len(s.config.Agents.Defaults.ProviderGroups) != 1 {
		t.Fatalf("provider groups not applied to runtime config: %+v", s.config.Agents.Defaults.ProviderGroups)
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}

	if reloaded.Memory.Semantic.SearchPolicy != "vector" || reloaded.Memory.ShortTerm.RawHistoryLimit != 333 {
		t.Fatalf("memory section not persisted: %+v", reloaded.Memory)
	}
	if reloaded.Agents.Defaults.SkillsProxy != "http://127.0.0.1:9001" {
		t.Fatalf("skills proxy not persisted: %+v", reloaded.Agents.Defaults)
	}
	if len(reloaded.Agents.Defaults.ProviderGroups) != 1 {
		t.Fatalf("provider groups not persisted: %+v", reloaded.Agents.Defaults.ProviderGroups)
	}
}

func TestHandleSaveConfigPersistsHarnessSections(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providers,
	}

	body := `{"audit":{"enabled":true,"max_arg_length":4321,"max_results":17,"retention_days":9},"undo":{"enabled":true,"max_turns":17,"snapshot_files":true},"preprocess":{"file_mentions":{"enabled":true,"max_file_size":12345,"max_total_size":45678,"max_files":23}},"learnings":{"enabled":true,"max_raw_entries":66,"compressed_max_size":2048,"half_life_days":14,"compress_interval":"30m"},"watch":{"enabled":true,"debounce_ms":850,"patterns":[{"file_glob":"**/*.go","command":"go test ./...","fail_command":"notify-send fail"}]}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleSaveConfig(c); err != nil {
		t.Fatalf("handleSaveConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if !s.config.Audit.Enabled || s.config.Audit.MaxArgLength != 4321 || s.config.Audit.MaxResults != 17 {
		t.Fatalf("audit not applied to runtime config: %+v", s.config.Audit)
	}
	if !s.config.Undo.Enabled || s.config.Undo.MaxTurns != 17 || !s.config.Undo.SnapshotFiles {
		t.Fatalf("undo not applied to runtime config: %+v", s.config.Undo)
	}
	if !s.config.Preprocess.FileMentions.Enabled || s.config.Preprocess.FileMentions.MaxFileSize != 12345 || s.config.Preprocess.FileMentions.MaxFiles != 23 {
		t.Fatalf("preprocess not applied to runtime config: %+v", s.config.Preprocess)
	}
	if !s.config.Learnings.Enabled || s.config.Learnings.MaxRawEntries != 66 {
		t.Fatalf("learnings not applied to runtime config: %+v", s.config.Learnings)
	}
	if !s.config.Watch.Enabled || s.config.Watch.DebounceMs != 850 || len(s.config.Watch.Patterns) != 1 {
		t.Fatalf("watch not applied to runtime config: %+v", s.config.Watch)
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}

	if !reflect.DeepEqual(reloaded.Audit, s.config.Audit) {
		t.Fatalf("audit section not persisted: got %+v want %+v", reloaded.Audit, s.config.Audit)
	}
	if !reflect.DeepEqual(reloaded.Undo, s.config.Undo) {
		t.Fatalf("undo section not persisted: got %+v want %+v", reloaded.Undo, s.config.Undo)
	}
	if !reflect.DeepEqual(reloaded.Preprocess, s.config.Preprocess) {
		t.Fatalf("preprocess section not persisted: got %+v want %+v", reloaded.Preprocess, s.config.Preprocess)
	}
	if !reflect.DeepEqual(reloaded.Learnings, s.config.Learnings) {
		t.Fatalf("learnings section not persisted: got %+v want %+v", reloaded.Learnings, s.config.Learnings)
	}
	if !reflect.DeepEqual(reloaded.Watch, s.config.Watch) {
		t.Fatalf("watch section not persisted: got %+v want %+v", reloaded.Watch, s.config.Watch)
	}
}

func TestHandleSaveConfigSyncsWatcherRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Watch.Enabled = true
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.go"),
		Command:  "printf 'watch'",
	}}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	watcher, err := watch.New(cfg, log, nil)
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
		config:    cfg,
		logger:    log,
		providers: providers,
		watcher:   watcher,
	}

	body := `{"watch":{"enabled":false,"debounce_ms":850,"patterns":[{"file_glob":"**/*.go","command":"go test ./...","fail_command":"notify-send fail"}]}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleSaveConfig(c); err != nil {
		t.Fatalf("handleSaveConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	status := watcher.Status()
	if status.Enabled {
		t.Fatalf("expected watcher to be disabled after save, got %+v", status)
	}
	if status.Running {
		t.Fatalf("expected watcher to stop after save, got %+v", status)
	}
}

func TestHandleSaveConfigUpdatesSkillRetentionRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	log := newTestLogger(t)
	skillsDir := filepath.Join(cfg.WorkspacePath(), "skills")
	mgr := skills.NewManagerWithRuntimeOptions(
		log,
		skillsDir,
		false,
		"",
		skills.SnapshotRetentionConfig{AutoPrune: true, MaxCount: 20},
		skills.VersionRetentionConfig{Enabled: true, MaxCount: 20},
	)

	s := &Server{
		config:    cfg,
		logger:    log,
		skillsMgr: mgr,
	}

	body := `{"webui":{"enabled":true,"port":0,"public_base_url":"","tool_session_otp_ttl_seconds":180,"tool_session_events":{"enabled":true,"retention_days":14},"skill_snapshots":{"auto_prune":true,"max_count":3},"skill_versions":{"enabled":false,"max_count":5}}}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleSaveConfig(c); err != nil {
		t.Fatalf("handleSaveConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	snapshotRetention := mgr.SnapshotRetention()
	if snapshotRetention.MaxCount != 3 {
		t.Fatalf("expected snapshot max_count 3, got %+v", snapshotRetention)
	}
	versionRetention := mgr.VersionRetention()
	if versionRetention.Enabled || versionRetention.MaxCount != 5 {
		t.Fatalf("unexpected version retention after save: %+v", versionRetention)
	}
}

func TestHandleImportConfigPersistsMemorySection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providers,
	}

	body := `{"memory":{"enabled":true,"semantic":{"enabled":true,"default_top_k":4,"max_top_k":10,"search_policy":"hybrid","include_scores":false},"episodic":{"enabled":true,"summary_window_messages":12,"max_summaries":80},"short_term":{"enabled":true,"raw_history_limit":123},"qmd":{"enabled":false,"command":"qmd","include_default":false,"paths":[],"sessions":{"enabled":false,"export_dir":"","retention_days":0},"update":{"on_boot":false,"interval":"30m","command_timeout":"30s","update_timeout":"5m"}}},"providers":[{"name":"p1","provider_kind":"openai","api_key":"k","api_base":"https://api.openai.com/v1","models":["gpt-4o"],"default_model":"gpt-4o","timeout":60}]}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleImportConfig(c); err != nil {
		t.Fatalf("handleImportConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if s.config.Memory.Semantic.SearchPolicy != "hybrid" || s.config.Memory.ShortTerm.RawHistoryLimit != 123 {
		t.Fatalf("memory not applied to runtime config: %+v", s.config.Memory)
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}
	if reloaded.Memory.Semantic.SearchPolicy != "hybrid" || reloaded.Memory.ShortTerm.RawHistoryLimit != 123 {
		t.Fatalf("memory section not persisted by import: %+v", reloaded.Memory)
	}
}

func TestHandleImportConfigPersistsHarnessSections(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providers,
	}

	body := `{"audit":{"enabled":true,"max_arg_length":22,"max_results":8,"retention_days":5},"undo":{"enabled":true,"max_turns":8,"snapshot_files":false},"preprocess":{"file_mentions":{"enabled":true,"max_file_size":54321,"max_total_size":88888,"max_files":9}},"learnings":{"enabled":true,"max_raw_entries":11,"compressed_max_size":1024,"half_life_days":10,"compress_interval":"1h"},"watch":{"enabled":true,"debounce_ms":1500,"patterns":[{"file_glob":"frontend/src/**/*.tsx","command":"npm run build","fail_command":"echo build failed"}]}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleImportConfig(c); err != nil {
		t.Fatalf("handleImportConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}

	if !reflect.DeepEqual(reloaded.Audit, s.config.Audit) {
		t.Fatalf("audit section not persisted by import: got %+v want %+v", reloaded.Audit, s.config.Audit)
	}
	if !reflect.DeepEqual(reloaded.Undo, s.config.Undo) {
		t.Fatalf("undo section not persisted by import: got %+v want %+v", reloaded.Undo, s.config.Undo)
	}
	if !reflect.DeepEqual(reloaded.Preprocess, s.config.Preprocess) {
		t.Fatalf("preprocess section not persisted by import: got %+v want %+v", reloaded.Preprocess, s.config.Preprocess)
	}
	if !reflect.DeepEqual(reloaded.Learnings, s.config.Learnings) {
		t.Fatalf("learnings section not persisted by import: got %+v want %+v", reloaded.Learnings, s.config.Learnings)
	}
	if !reflect.DeepEqual(reloaded.Watch, s.config.Watch) {
		t.Fatalf("watch section not persisted by import: got %+v want %+v", reloaded.Watch, s.config.Watch)
	}
}

func TestHandleImportConfigSyncsWatcherRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Watch.Enabled = true
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.go"),
		Command:  "printf 'watch'",
	}}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	watcher, err := watch.New(cfg, log, nil)
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
		config:    cfg,
		logger:    log,
		providers: providers,
		watcher:   watcher,
	}

	body := `{"watch":{"enabled":false,"debounce_ms":1500,"patterns":[{"file_glob":"frontend/src/**/*.tsx","command":"npm run build","fail_command":"echo fail"}]}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleImportConfig(c); err != nil {
		t.Fatalf("handleImportConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	status := watcher.Status()
	if status.Enabled {
		t.Fatalf("expected watcher to be disabled after import, got %+v", status)
	}
	if status.Running {
		t.Fatalf("expected watcher to stop after import, got %+v", status)
	}
}

func TestHandleImportConfigPersistsBootstrapSectionsAndReportsRestart(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loader := config.NewLoader()
	if _, err := loader.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		loader:    loader,
		logger:    log,
		providers: providers,
	}

	newDBDir := filepath.Join(t.TempDir(), "imported-db")
	body := `{"storage":{"db_dir":"` + newDBDir + `"},"logger":{"level":"warn","output_path":"","max_size":0,"max_backups":0,"max_age":0,"compress":false},"gateway":{"host":"127.0.0.1","port":28080},"webui":{"enabled":true,"port":28081,"public_base_url":"https://import.example.com","tool_session_otp_ttl_seconds":120,"tool_session_events":{"enabled":true,"retention_days":7},"skill_snapshots":{"auto_prune":true,"max_count":8},"skill_versions":{"enabled":true,"max_count":12}}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleImportConfig(c); err != nil {
		t.Fatalf("handleImportConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Status            string   `json:"status"`
		SectionsSaved     int      `json:"sections_saved"`
		RestartRequired   bool     `json:"restart_required"`
		RestartSections   []string `json:"restart_sections"`
		ProvidersImported int      `json:"providers_imported"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if payload.Status != "imported" || payload.SectionsSaved != 3 || payload.ProvidersImported != 0 {
		t.Fatalf("unexpected import response: %+v", payload)
	}
	if !payload.RestartRequired {
		t.Fatalf("expected restart_required=true, got %+v", payload)
	}
	for _, section := range []string{"storage", "logger", "gateway", "webui"} {
		if !containsString(payload.RestartSections, section) {
			t.Fatalf("expected restart section %q, got %+v", section, payload.RestartSections)
		}
	}

	bootstrapReloaded, err := config.NewLoader().LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("reload bootstrap config failed: %v", err)
	}
	if bootstrapReloaded.Storage.DBDir != newDBDir {
		t.Fatalf("storage not persisted to bootstrap config: %+v", bootstrapReloaded.Storage)
	}
	if bootstrapReloaded.Logger.Level != "warn" {
		t.Fatalf("logger not persisted to bootstrap config: %+v", bootstrapReloaded.Logger)
	}
	if bootstrapReloaded.Gateway.Host != "127.0.0.1" || bootstrapReloaded.Gateway.Port != 28080 {
		t.Fatalf("gateway not persisted to bootstrap config: %+v", bootstrapReloaded.Gateway)
	}
	if bootstrapReloaded.WebUI.Port != 28081 || bootstrapReloaded.WebUI.PublicBaseURL != "https://import.example.com" {
		t.Fatalf("webui not persisted to bootstrap config: %+v", bootstrapReloaded.WebUI)
	}
}

func TestHandleSaveConfigMigratesRuntimeDBWhenStorageChanges(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = filepath.Join(t.TempDir(), "db-old")
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Semantic.SearchPolicy = "vector"
	cfg.Memory.ShortTerm.RawHistoryLimit = 88

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	loader := config.NewLoader()
	if _, err := loader.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	adminCred := &config.AdminCredential{
		Username:     "admin",
		Nickname:     "Owner",
		PasswordHash: "$2a$10$examplehash",
		JWTSecret:    "jwt-secret",
	}
	if err := config.SaveAdminCredential(client, adminCred); err != nil {
		t.Fatalf("SaveAdminCredential failed: %v", err)
	}
	if err := config.SaveDatabaseSections(cfg, "memory"); err != nil {
		t.Fatalf("SaveDatabaseSections failed: %v", err)
	}

	s := &Server{
		config:    cfg,
		loader:    loader,
		logger:    log,
		entClient: client,
	}

	newDBDir := filepath.Join(t.TempDir(), "db-new")
	body := `{"storage":{"db_dir":"` + newDBDir + `"}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleSaveConfig(c); err != nil {
		t.Fatalf("handleSaveConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = newDBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace

	newClient := newTestEntClient(t, reloaded)
	t.Cleanup(func() {
		if err := newClient.Close(); err != nil {
			t.Fatalf("close migrated ent client: %v", err)
		}
	})

	migratedCred, err := config.LoadAdminCredential(newClient)
	if err != nil {
		t.Fatalf("LoadAdminCredential on migrated DB failed: %v", err)
	}
	if migratedCred == nil || migratedCred.Username != "admin" || migratedCred.JWTSecret != "jwt-secret" {
		t.Fatalf("expected migrated admin credential, got %+v", migratedCred)
	}

	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}
	if reloaded.Memory.Semantic.SearchPolicy != "vector" || reloaded.Memory.ShortTerm.RawHistoryLimit != 88 {
		t.Fatalf("expected migrated runtime sections, got %+v", reloaded.Memory)
	}
}

func TestHandleImportConfigMigratesRuntimeDBWhenStorageChanges(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = filepath.Join(t.TempDir(), "db-old")
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Semantic.SearchPolicy = "hybrid"
	cfg.Memory.ShortTerm.RawHistoryLimit = 64

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	loader := config.NewLoader()
	if _, err := loader.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	adminCred := &config.AdminCredential{
		Username:     "owner",
		Nickname:     "Owner",
		PasswordHash: "$2a$10$examplehash",
		JWTSecret:    "import-jwt-secret",
	}
	if err := config.SaveAdminCredential(client, adminCred); err != nil {
		t.Fatalf("SaveAdminCredential failed: %v", err)
	}
	if err := config.SaveDatabaseSections(cfg, "memory"); err != nil {
		t.Fatalf("SaveDatabaseSections failed: %v", err)
	}

	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	s := &Server{
		config:    cfg,
		loader:    loader,
		logger:    log,
		entClient: client,
		providers: providers,
	}

	newDBDir := filepath.Join(t.TempDir(), "db-imported")
	body := `{"storage":{"db_dir":"` + newDBDir + `"}}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleImportConfig(c); err != nil {
		t.Fatalf("handleImportConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = newDBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace

	newClient := newTestEntClient(t, reloaded)
	t.Cleanup(func() {
		if err := newClient.Close(); err != nil {
			t.Fatalf("close imported ent client: %v", err)
		}
	})

	migratedCred, err := config.LoadAdminCredential(newClient)
	if err != nil {
		t.Fatalf("LoadAdminCredential on migrated DB failed: %v", err)
	}
	if migratedCred == nil || migratedCred.Username != "owner" || migratedCred.JWTSecret != "import-jwt-secret" {
		t.Fatalf("expected migrated admin credential, got %+v", migratedCred)
	}

	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}
	if reloaded.Memory.Semantic.SearchPolicy != "hybrid" || reloaded.Memory.ShortTerm.RawHistoryLimit != 64 {
		t.Fatalf("expected migrated runtime sections, got %+v", reloaded.Memory)
	}
}

func TestHandleExportConfigIncludesMemorySection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Semantic.SearchPolicy = "vector"
	cfg.Memory.ShortTerm.RawHistoryLimit = 432

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:         "openai",
		ProviderKind: "openai",
		APIKey:       "secret",
		APIBase:      "https://api.openai.com/v1",
		Models:       []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providers,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/config/export", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleExportConfig(c); err != nil {
		t.Fatalf("handleExportConfig failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal export payload failed: %v", err)
	}
	if _, ok := payload["memory"]; !ok {
		t.Fatalf("expected memory section in export response: %s", rec.Body.String())
	}

	var memory config.MemoryConfig
	if err := json.Unmarshal(payload["memory"], &memory); err != nil {
		t.Fatalf("unmarshal memory failed: %v", err)
	}
	if memory.Semantic.SearchPolicy != "vector" || memory.ShortTerm.RawHistoryLimit != 432 {
		t.Fatalf("unexpected exported memory section: %+v", memory)
	}
}

func TestPersistChatRoutingClearsProviderAndModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "primary"
	cfg.Agents.Defaults.Model = "claude-sonnet"
	cfg.Agents.Defaults.Fallback = []string{"backup"}

	s := &Server{
		config: cfg,
	}

	if err := s.persistChatRouting("", "", nil); err != nil {
		t.Fatalf("persistChatRouting failed: %v", err)
	}

	if cfg.Agents.Defaults.Provider != "" {
		t.Fatalf("expected provider to be cleared, got %q", cfg.Agents.Defaults.Provider)
	}
	if cfg.Agents.Defaults.Model != "" {
		t.Fatalf("expected model to be cleared, got %q", cfg.Agents.Defaults.Model)
	}
	if len(cfg.Agents.Defaults.Fallback) != 0 {
		t.Fatalf("expected fallback to be cleared, got %v", cfg.Agents.Defaults.Fallback)
	}
}

func TestPersistChatRoutingRejectsUnknownProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers = []config.ProviderProfile{
		{Name: "primary", ProviderKind: "openai"},
	}

	s := &Server{
		config: cfg,
	}

	if err := s.persistChatRouting("missing", "gpt-4o", nil); err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if cfg.Agents.Defaults.Provider != "" {
		t.Fatalf("unexpected provider mutation: %q", cfg.Agents.Defaults.Provider)
	}
}

func TestPersistChatRoutingAcceptsProviderGroupTargets(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers = []config.ProviderProfile{
		{Name: "primary", ProviderKind: "openai"},
		{Name: "backup", ProviderKind: "openai"},
	}
	cfg.Agents.Defaults.ProviderGroups = []config.ProviderGroupConfig{
		{
			Name:     "pool-a",
			Strategy: "round_robin",
			Members:  []string{"primary", "backup"},
		},
	}

	s := &Server{
		config: cfg,
	}

	if err := s.persistChatRouting("pool-a", "gpt-4.1", []string{"backup", "pool-a"}); err != nil {
		t.Fatalf("persistChatRouting failed for provider group: %v", err)
	}

	if cfg.Agents.Defaults.Provider != "pool-a" {
		t.Fatalf("expected provider group to be saved, got %q", cfg.Agents.Defaults.Provider)
	}
	if !reflect.DeepEqual(cfg.Agents.Defaults.Fallback, []string{"backup", "pool-a"}) {
		t.Fatalf("expected fallback to keep provider group target, got %v", cfg.Agents.Defaults.Fallback)
	}
}

func TestPersistChatRoutingUpdatesRouteFields(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers = []config.ProviderProfile{
		{Name: "primary", ProviderKind: "openai"},
		{Name: "backup", ProviderKind: "openai"},
	}

	s := &Server{
		config: cfg,
	}

	if err := s.persistChatRouting("primary", "gpt-4.1", []string{"backup"}); err != nil {
		t.Fatalf("persistChatRouting failed: %v", err)
	}

	if cfg.Agents.Defaults.Provider != "primary" {
		t.Fatalf("expected provider to update, got %q", cfg.Agents.Defaults.Provider)
	}
	if cfg.Agents.Defaults.Model != "gpt-4.1" {
		t.Fatalf("expected model to update, got %q", cfg.Agents.Defaults.Model)
	}
	if !reflect.DeepEqual(cfg.Agents.Defaults.Fallback, []string{"backup"}) {
		t.Fatalf("expected fallback to update, got %v", cfg.Agents.Defaults.Fallback)
	}
}

func TestHandleGetProvidersReturnsProjectedView(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Provider = "primary"

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providers.Close(); err != nil {
			t.Errorf("close provider manager: %v", err)
		}
	})

	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:          "primary",
		ProviderKind:  "openai",
		APIKey:        "secret-key",
		APIBase:       "https://api.openai.com/v1",
		Timeout:       45,
		DefaultWeight: 9,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providers,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetProviders(c); err != nil {
		t.Fatalf("handleGetProviders failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal providers payload failed: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(payload))
	}

	item := payload[0]
	requiredKeys := []string{
		"name",
		"provider_kind",
		"api_key_set",
		"api_base",
		"proxy",
		"default_weight",
		"enabled",
		"is_routing_default",
		"supports_discovery",
		"summary",
		"timeout",
	}
	for _, key := range requiredKeys {
		if _, ok := item[key]; !ok {
			t.Fatalf("expected key %q in provider payload: %+v", key, item)
		}
	}
	if got, _ := item["api_key_set"].(bool); !got {
		t.Fatalf("expected api_key_set true, got %+v", item["api_key_set"])
	}
	if got, _ := item["is_routing_default"].(bool); !got {
		t.Fatalf("expected is_routing_default true, got %+v", item["is_routing_default"])
	}
	if got, _ := item["default_weight"].(float64); got != 9 {
		t.Fatalf("expected default_weight 9, got %+v", item["default_weight"])
	}
	if got, ok := item["enabled"].(bool); !ok || !got {
		t.Fatalf("expected enabled true, got %+v", item["enabled"])
	}
	if _, ok := item["models"]; ok {
		t.Fatalf("expected projected provider to omit models, got %+v", item)
	}
	if _, ok := item["default_model"]; ok {
		t.Fatalf("expected projected provider to omit default_model, got %+v", item)
	}
	if secret, ok := item["api_key"].(string); ok && secret != "" {
		t.Fatalf("expected projected provider to omit raw api_key, got %q", secret)
	}
}

func TestHandleGetProviderTypesReturnsRegistry(t *testing.T) {
	s := &Server{
		config: config.DefaultConfig(),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/provider-types", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetProviderTypes(c); err != nil {
		t.Fatalf("handleGetProviderTypes failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal provider types payload failed: %v", err)
	}
	if len(payload) == 0 {
		t.Fatalf("expected non-empty provider type payload")
	}

	first := payload[0]
	for _, key := range []string{"id", "display_name", "icon", "supports_discovery", "auth_fields", "advanced_fields"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("expected key %q in provider type payload: %+v", key, first)
		}
	}

	foundOpenAI := false
	for _, item := range payload {
		if got, _ := item["id"].(string); got == "openai" {
			foundOpenAI = true
			if displayName, _ := item["display_name"].(string); strings.TrimSpace(displayName) == "" {
				t.Fatalf("expected openai display_name, got %+v", item)
			}
			if supports, ok := item["supports_discovery"].(bool); !ok || !supports {
				t.Fatalf("expected openai to support discovery, got %+v", item)
			}
		}
	}
	if !foundOpenAI {
		t.Fatalf("expected openai provider type in payload: %+v", payload)
	}
}

func TestHandleGetProviderRuntimeReturnsCooldownState(t *testing.T) {
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

	providerMgr, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	t.Cleanup(func() {
		if err := providerMgr.Close(); err != nil {
			t.Fatalf("close provider manager: %v", err)
		}
	})

	if _, err := providerMgr.Create(context.Background(), config.ProviderProfile{
		Name:         "primary",
		ProviderKind: "openai",
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providerMgr,
		agent:     &agent.Agent{},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/providers/runtime", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetProviderRuntime(c); err != nil {
		t.Fatalf("handleGetProviderRuntime failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal provider runtime payload failed: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 runtime item, got %d", len(payload))
	}
	if got, _ := payload[0]["name"].(string); got != "primary" {
		t.Fatalf("expected primary runtime item, got %q", got)
	}
	if available, ok := payload[0]["available"].(bool); !ok || !available {
		t.Fatalf("expected primary to default to available, got %+v", payload[0]["available"])
	}
}

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}

func newTestEntClient(t *testing.T, cfg *config.Config) *ent.Client {
	t.Helper()
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		t.Fatalf("ensure runtime schema: %v", err)
	}
	return client
}
