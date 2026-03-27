package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
)

func TestHandleInitStatusIncludesBootstrapSummary(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Logger.Level = "debug"
	cfg.Gateway.Host = "0.0.0.0"
	cfg.Gateway.Port = 19090
	cfg.WebUI.Port = 19191
	cfg.WebUI.PublicBaseURL = "https://bot.example.com"

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loader := config.NewLoader()
	if _, err := loader.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	client := newTestEntClient(t, cfg)
	defer client.Close()

	s := &Server{
		config:    cfg,
		loader:    loader,
		logger:    newTestLogger(t),
		entClient: client,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/init-status", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleInitStatus(ctx); err != nil {
		t.Fatalf("handleInitStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Initialized bool `json:"initialized"`
		Bootstrap   struct {
			ConfigPath string              `json:"config_path"`
			DBDir      string              `json:"db_dir"`
			Workspace  string              `json:"workspace"`
			Logger     config.LoggerConfig `json:"logger"`
			Gateway    config.GatewayConfig `json:"gateway"`
			WebUI      config.WebUIConfig  `json:"webui"`
		} `json:"bootstrap"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if payload.Initialized {
		t.Fatalf("expected initialized=false before admin setup")
	}
	if payload.Bootstrap.ConfigPath != configPath {
		t.Fatalf("unexpected config path: %+v", payload.Bootstrap)
	}
	if payload.Bootstrap.DBDir != cfg.Storage.DBDir {
		t.Fatalf("unexpected db dir: %+v", payload.Bootstrap)
	}
	if payload.Bootstrap.Workspace != cfg.Agents.Defaults.Workspace {
		t.Fatalf("unexpected workspace: %+v", payload.Bootstrap)
	}
	if payload.Bootstrap.Logger.Level != "debug" {
		t.Fatalf("unexpected logger: %+v", payload.Bootstrap.Logger)
	}
	if payload.Bootstrap.Gateway.Host != "0.0.0.0" || payload.Bootstrap.Gateway.Port != 19090 {
		t.Fatalf("unexpected gateway: %+v", payload.Bootstrap.Gateway)
	}
	if payload.Bootstrap.WebUI.Port != 19191 || payload.Bootstrap.WebUI.PublicBaseURL != "https://bot.example.com" {
		t.Fatalf("unexpected webui: %+v", payload.Bootstrap.WebUI)
	}
}

func TestHandleInitPasswordPersistsBootstrapSectionsAndReturnsRestartHint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loader := config.NewLoader()
	if _, err := loader.LoadFromFile(configPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	client := newTestEntClient(t, cfg)
	defer client.Close()

	s := &Server{
		config:    cfg,
		loader:    loader,
		logger:    newTestLogger(t),
		entClient: client,
	}

	body := `{
		"username":"root",
		"password":"secret-123",
		"bootstrap":{
			"logger":{"level":"debug","output_path":"","development":false},
			"gateway":{"enabled":true,"host":"0.0.0.0","port":19090},
			"webui":{
				"enabled":true,
				"port":19191,
				"public_base_url":"https://bot.example.com",
				"tool_session_otp_ttl_seconds":180,
				"tool_session_events":{"enabled":true,"retention_days":14},
				"skill_snapshots":{"auto_prune":true,"max_count":10},
				"skill_versions":{"enabled":true,"max_count":20}
			}
		}
	}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/init", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleInitPassword(ctx); err != nil {
		t.Fatalf("handleInitPassword failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Token           string `json:"token"`
		RestartRequired bool   `json:"restart_required"`
		RestartSections []string `json:"restart_sections"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if strings.TrimSpace(payload.Token) == "" {
		t.Fatalf("expected token in init response")
	}
	if !payload.RestartRequired {
		t.Fatalf("expected restart_required=true, got %+v", payload)
	}
	for _, section := range []string{"logger", "gateway", "webui"} {
		if !containsString(payload.RestartSections, section) {
			t.Fatalf("expected restart section %q, got %+v", section, payload.RestartSections)
		}
	}

	cred, err := config.LoadAdminCredential(client)
	if err != nil {
		t.Fatalf("LoadAdminCredential failed: %v", err)
	}
	if cred == nil || cred.Username != "root" {
		t.Fatalf("unexpected admin credential: %+v", cred)
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

	reloaded, err := config.NewLoader().LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("reload bootstrap config failed: %v", err)
	}
	if reloaded.Logger.Level != "debug" {
		t.Fatalf("logger not persisted: %+v", reloaded.Logger)
	}
	if reloaded.Gateway.Host != "0.0.0.0" || reloaded.Gateway.Port != 19090 {
		t.Fatalf("gateway not persisted: %+v", reloaded.Gateway)
	}
	if reloaded.WebUI.Port != 19191 || reloaded.WebUI.PublicBaseURL != "https://bot.example.com" {
		t.Fatalf("webui not persisted: %+v", reloaded.WebUI)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
