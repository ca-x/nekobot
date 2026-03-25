package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	defer client.Close()
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	defer providers.Close()

	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:         "primary",
		ProviderKind: "openai",
		APIKey:       "secret-key",
		APIBase:      "https://api.openai.com/v1",
		Models:       []string{"gpt-4.1", "gpt-4o-mini"},
		DefaultModel: "gpt-4.1",
		Timeout:      45,
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
		"models",
		"model_count",
		"default_model",
		"has_default_model",
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
	if got, _ := item["has_default_model"].(bool); !got {
		t.Fatalf("expected has_default_model true, got %+v", item["has_default_model"])
	}
	if got, _ := item["model_count"].(float64); got != 2 {
		t.Fatalf("expected model_count 2, got %+v", item["model_count"])
	}
	if secret, ok := item["api_key"].(string); ok && secret != "" {
		t.Fatalf("expected projected provider to omit raw api_key, got %q", secret)
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
