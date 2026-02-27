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
	"nekobot/pkg/logger"
	"nekobot/pkg/providerstore"
	"nekobot/pkg/storage/ent"
)

func TestHandleGetConfigIncludesMemorySection(t *testing.T) {
	s := &Server{
		config: config.DefaultConfig(),
	}
	s.config.Memory.Semantic.SearchPolicy = "vector"
	s.config.Memory.ShortTerm.RawHistoryLimit = 77

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
}

func TestHandleSaveConfigPersistsMemorySection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer client.Close()
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	defer providers.Close()

	s := &Server{
		config:    cfg,
		logger:    log,
		providers: providers,
	}

	body := `{"memory":{"enabled":true,"semantic":{"enabled":true,"default_top_k":7,"max_top_k":25,"search_policy":"vector","include_scores":true},"episodic":{"enabled":true,"summary_window_messages":30,"max_summaries":400},"short_term":{"enabled":true,"raw_history_limit":333},"qmd":{"enabled":false,"command":"qmd","include_default":false,"paths":[],"sessions":{"enabled":false,"export_dir":"","retention_days":0},"update":{"on_boot":false,"interval":"30m","command_timeout":"30s","update_timeout":"5m"}}}}`

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

	reloaded := config.DefaultConfig()
	reloaded.Storage.DBDir = cfg.Storage.DBDir
	reloaded.Agents.Defaults.Workspace = cfg.Agents.Defaults.Workspace
	if err := config.ApplyDatabaseOverrides(reloaded); err != nil {
		t.Fatalf("ApplyDatabaseOverrides failed: %v", err)
	}

	if reloaded.Memory.Semantic.SearchPolicy != "vector" || reloaded.Memory.ShortTerm.RawHistoryLimit != 333 {
		t.Fatalf("memory section not persisted: %+v", reloaded.Memory)
	}
}

func TestHandleImportConfigPersistsMemorySection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer client.Close()
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	defer providers.Close()

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

func TestHandleExportConfigIncludesMemorySection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Semantic.SearchPolicy = "vector"
	cfg.Memory.ShortTerm.RawHistoryLimit = 432

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer client.Close()
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	defer providers.Close()

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
