package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/modelroute"
	"nekobot/pkg/modelstore"
	"nekobot/pkg/providerstore"
)

func TestHandleGetModelsAndCreateModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	models, err := modelstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}
	if _, err := models.Create(context.Background(), modelstore.ModelCatalog{
		ModelID:       "claude-sonnet-4-5-20250929",
		DisplayName:   "Claude Sonnet",
		CatalogSource: "builtin",
		Enabled:       true,
	}); err != nil {
		t.Fatalf("seed model failed: %v", err)
	}

	s := &Server{config: cfg, logger: log, providers: providers, entClient: client}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetModels(c); err != nil {
		t.Fatalf("handleGetModels failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal models failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 model, got %d", len(listed))
	}

	body := `{"model_id":"gpt-4.1","display_name":"GPT-4.1","catalog_source":"manual","enabled":true}`
	req = httptest.NewRequest(http.MethodPost, "/api/models", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := s.handleCreateModel(c); err != nil {
		t.Fatalf("handleCreateModel failed: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateModelRouteAndGetModelRoutes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:          "openai-main",
		ProviderKind:  "openai",
		APIKey:        "secret-key",
		DefaultWeight: 6,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	models, err := modelstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}
	if _, err := models.Create(context.Background(), modelstore.ModelCatalog{
		ModelID:       "gpt-4.1",
		DisplayName:   "GPT-4.1",
		CatalogSource: "builtin",
		Enabled:       true,
	}); err != nil {
		t.Fatalf("seed model failed: %v", err)
	}

	routes, err := modelroute.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new route manager: %v", err)
	}
	if _, err := routes.Create(context.Background(), modelroute.ModelRoute{
		ModelID:      "gpt-4.1",
		ProviderName: "openai-main",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("seed route failed: %v", err)
	}

	s := &Server{config: cfg, logger: log, providers: providers, entClient: client}

	e := echo.New()
	body := `{"model_id":"gpt-4.1","provider_name":"openai-main","enabled":true,"is_default":true,"weight_override":12,"aliases":["gpt-4.1-latest"],"regex_rules":["^gpt-4\\.1-mini$"],"metadata":{"provider_model_id":"gpt-4.1-mini"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/model-routes/gpt-4.1/openai-main", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/model-routes/:modelID/:providerName")
	c.SetPathValues(echo.PathValues{
		{Name: "modelID", Value: "gpt-4.1"},
		{Name: "providerName", Value: "openai-main"},
	})

	if err := s.handleUpdateModelRoute(c); err != nil {
		t.Fatalf("handleUpdateModelRoute failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/model-routes?model_id=gpt-4.1", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := s.handleGetModelRoutes(c); err != nil {
		t.Fatalf("handleGetModelRoutes failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal routes failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 route, got %d", len(listed))
	}
	if got, _ := listed[0]["weight_override"].(float64); got != 12 {
		t.Fatalf("expected weight override 12, got %+v", listed[0]["weight_override"])
	}
}

func TestHandleUpdateModelRouteCreatesMissingRoute(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:          "openai-main",
		ProviderKind:  "openai",
		APIKey:        "secret-key",
		DefaultWeight: 6,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	models, err := modelstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}
	if _, err := models.Create(context.Background(), modelstore.ModelCatalog{
		ModelID:       "gpt-4.1",
		DisplayName:   "GPT-4.1",
		CatalogSource: "builtin",
		Enabled:       true,
	}); err != nil {
		t.Fatalf("seed model failed: %v", err)
	}

	s := &Server{config: cfg, logger: log, providers: providers, entClient: client}
	e := echo.New()
	body := `{"model_id":"gpt-4.1","provider_name":"openai-main","enabled":true,"is_default":true,"weight_override":12,"aliases":["gpt-4.1-latest"],"regex_rules":["^gpt-4\\.1-mini$"],"metadata":{"provider_model_id":"gpt-4.1-mini"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/model-routes/gpt-4.1/openai-main", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/model-routes/:modelID/:providerName")
	c.SetPathValues(echo.PathValues{
		{Name: "modelID", Value: "gpt-4.1"},
		{Name: "providerName", Value: "openai-main"},
	})

	if err := s.handleUpdateModelRoute(c); err != nil {
		t.Fatalf("handleUpdateModelRoute failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	routes, err := modelroute.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new route manager: %v", err)
	}
	listedRoutes, err := routes.ListByModel(context.Background(), "gpt-4.1")
	if err != nil {
		t.Fatalf("list routes failed: %v", err)
	}
	if len(listedRoutes) != 1 {
		t.Fatalf("expected 1 route after create-via-update, got %d", len(listedRoutes))
	}
	if listedRoutes[0].ProviderName != "openai-main" {
		t.Fatalf("unexpected route: %+v", listedRoutes[0])
	}
}

func TestHandleDiscoverProviderModelsReturnsPreviewOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:         "openai-main",
		ProviderKind: "openai",
		APIBase:      "https://api.example.com/v1",
		APIKey:       "secret",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	s := &Server{config: cfg, logger: log, providers: providers, entClient: client}

	original := discoverOpenAICompatibleModelsFunc
	discoverOpenAICompatibleModelsFunc = func(apiBase, apiKey, proxy string, timeout int) ([]string, error) {
		return []string{"gpt-4.1", "gpt-4o-mini"}, nil
	}
	t.Cleanup(func() {
		discoverOpenAICompatibleModelsFunc = original
	})

	e := echo.New()
	body := `{"name":"openai-main","provider_kind":"openai"}`
	req := httptest.NewRequest(http.MethodPost, "/api/providers/discover-models", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleDiscoverProviderModels(c); err != nil {
		t.Fatalf("handleDiscoverProviderModels failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	models, err := modelstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}
	listedModels, err := models.List(context.Background())
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(listedModels) != 0 {
		t.Fatalf("expected preview-only discover to persist 0 models, got %d", len(listedModels))
	}
}

func TestHandleApplyDiscoveredProviderModelsMergesCatalogAndRoutes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:         "openai-main",
		ProviderKind: "openai",
		APIBase:      "https://api.example.com/v1",
		APIKey:       "secret",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	s := &Server{config: cfg, logger: log, providers: providers, entClient: client}

	e := echo.New()
	body := `{"profile":{"name":"openai-main","provider_kind":"openai"},"models":["gpt-4.1","gpt-4o-mini"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/providers/apply-discovered-models", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleApplyDiscoveredProviderModels(c); err != nil {
		t.Fatalf("handleApplyDiscoveredProviderModels failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	models, err := modelstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}
	listedModels, err := models.List(context.Background())
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(listedModels) != 2 {
		t.Fatalf("expected 2 applied models, got %d", len(listedModels))
	}

	routes, err := modelroute.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new route manager: %v", err)
	}
	listedRoutes, err := routes.ListByModel(context.Background(), "gpt-4.1")
	if err != nil {
		t.Fatalf("list routes failed: %v", err)
	}
	if len(listedRoutes) != 1 {
		t.Fatalf("expected 1 route for gpt-4.1, got %d", len(listedRoutes))
	}
	if got, _ := listedRoutes[0].Metadata["provider_model_id"].(string); got != "gpt-4.1" {
		t.Fatalf("expected provider_model_id metadata, got %+v", listedRoutes[0].Metadata)
	}
}

func TestHandleGetModelRoutesBatch(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	if _, err := providers.Create(context.Background(), config.ProviderProfile{
		Name:         "openai-main",
		ProviderKind: "openai",
		APIKey:       "secret-key",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("create provider failed: %v", err)
	}

	models, err := modelstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}
	for _, modelID := range []string{"gpt-4.1", "claude-sonnet"} {
		if _, err := models.Create(context.Background(), modelstore.ModelCatalog{
			ModelID:       modelID,
			DisplayName:   modelID,
			CatalogSource: "builtin",
			Enabled:       true,
		}); err != nil {
			t.Fatalf("seed model %s failed: %v", modelID, err)
		}
	}

	routes, err := modelroute.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new route manager: %v", err)
	}
	for _, modelID := range []string{"gpt-4.1", "claude-sonnet"} {
		if _, err := routes.Create(context.Background(), modelroute.ModelRoute{
			ModelID:      modelID,
			ProviderName: "openai-main",
			Enabled:      true,
		}); err != nil {
			t.Fatalf("seed route %s failed: %v", modelID, err)
		}
	}

	s := &Server{config: cfg, logger: log, providers: providers, entClient: client}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/model-routes?model_ids=gpt-4.1&model_ids=claude-sonnet", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetModelRoutes(c); err != nil {
		t.Fatalf("handleGetModelRoutes failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed map[string][]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal routes failed: %v", err)
	}
	if len(listed["gpt-4.1"]) != 1 || len(listed["claude-sonnet"]) != 1 {
		t.Fatalf("expected routes for both models, got %+v", listed)
	}
}

func TestHandleClearProviderCooldown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	log := newTestLogger(t)
	s := &Server{config: cfg, logger: log, agent: &agent.Agent{}}
	s.agent.ClearFailoverCooldown("primary")
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/providers/primary/clear-cooldown", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/providers/:name/clear-cooldown")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "primary"}})
	if err := s.handleClearProviderCooldown(c); err != nil {
		t.Fatalf("handleClearProviderCooldown failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
