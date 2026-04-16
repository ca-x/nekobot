package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/skills"
	"nekobot/pkg/workspace"
)

type stubRemoteRegistry struct {
	searchOutput string
	searchErr    error
	installPath  string
	installErr   error
	lastQuery    string
	lastSource   string
	lastTarget   string
}

func (s *stubRemoteRegistry) Search(_ context.Context, query string) (string, error) {
	s.lastQuery = query
	return s.searchOutput, s.searchErr
}

func (s *stubRemoteRegistry) Install(_ context.Context, source, targetDir string) (string, error) {
	s.lastSource = source
	s.lastTarget = targetDir
	if s.installErr != nil {
		return "", s.installErr
	}
	return s.installPath, nil
}

func TestMarketplaceHandlers_Return503WithoutSkillsManager(t *testing.T) {
	s := &Server{}
	e := echo.New()

	tests := []struct {
		name    string
		method  string
		path    string
		handler func(*echo.Context) error
		paramID string
	}{
		{
			name:    "list skills",
			method:  http.MethodGet,
			path:    "/api/marketplace/skills",
			handler: s.handleListMarketplaceSkills,
		},
		{
			name:    "list installed skills",
			method:  http.MethodGet,
			path:    "/api/marketplace/skills/installed",
			handler: s.handleListInstalledMarketplaceSkills,
		},
		{
			name:    "get skill item",
			method:  http.MethodGet,
			path:    "/api/marketplace/skills/items/missing",
			handler: s.handleGetMarketplaceSkillItem,
			paramID: "missing",
		},
		{
			name:    "get skill content",
			method:  http.MethodGet,
			path:    "/api/marketplace/skills/items/missing/content",
			handler: s.handleGetMarketplaceSkillContent,
			paramID: "missing",
		},
		{
			name:    "enable skill",
			method:  http.MethodPost,
			path:    "/api/marketplace/skills/missing/enable",
			handler: s.handleEnableMarketplaceSkill,
			paramID: "missing",
		},
		{
			name:    "disable skill",
			method:  http.MethodPost,
			path:    "/api/marketplace/skills/missing/disable",
			handler: s.handleDisableMarketplaceSkill,
			paramID: "missing",
		},
		{
			name:    "install deps",
			method:  http.MethodPost,
			path:    "/api/marketplace/skills/missing/install-deps",
			handler: s.handleInstallMarketplaceSkillDependencies,
			paramID: "missing",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			if tc.paramID != "" {
				c.SetPath("/api/marketplace/skills/:id")
				c.SetPathValues(echo.PathValues{{Name: "id", Value: tc.paramID}})
			}

			if err := tc.handler(c); err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
			}
		})
	}
}

func TestMarketplaceHandlers_ListEnableDisableFlow(t *testing.T) {
	const skillID = "marketplace-test-skill"
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "marketplace-test-skill.md")
	skillContent := `---
id: marketplace-test-skill
name: Marketplace Test Skill
description: Skill fixture for marketplace handlers
version: 0.1.0
author: test-suite
tags:
  - marketplace
  - test
enabled: false
requirements:
  env:
    - MARKETPLACE_TEST_TOKEN
  python_packages:
    - requests
  node_packages:
    - typescript
  custom:
    install:
      - method: command
        package: "printf marketplace-install-ok"
---
Use this skill for tests.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write test skill: %v", err)
	}

	log := newTestLogger(t)
	mgr := skills.NewManager(log, skillsDir, false)
	mgr.SetPythonPackageInstalled(func(pkg string) bool {
		return true
	})
	mgr.SetNodePackageInstalled(func(pkg string) bool {
		return true
	})
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}

	s := &Server{skillsMgr: mgr}
	e := echo.New()

	// List and verify shape.
	listReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := s.handleListMarketplaceSkills(listCtx); err != nil {
		t.Fatalf("list handler failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRec.Code)
	}

	var payload []map[string]interface{}
	if err := json.Unmarshal(listRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}

	var target map[string]interface{}
	for _, item := range payload {
		if id, _ := item["id"].(string); id == skillID {
			target = item
			break
		}
	}
	if target == nil {
		t.Fatalf("expected skill %q in list payload", skillID)
	}

	requiredKeys := []string{
		"id", "name", "description", "version", "author", "enabled", "always", "file_path", "tags",
		"eligible", "ineligibility_reasons", "install_specs", "is_installed", "missing_requirements",
	}
	for _, key := range requiredKeys {
		if _, ok := target[key]; !ok {
			t.Fatalf("expected key %q in payload item: %+v", key, target)
		}
	}
	if enabled, _ := target["enabled"].(bool); enabled {
		t.Fatalf("expected skill to be disabled initially")
	}
	if eligible, _ := target["eligible"].(bool); eligible {
		t.Fatalf("expected skill to be ineligible while required env is missing")
	}
	reasons, ok := target["ineligibility_reasons"].([]interface{})
	if !ok || len(reasons) != 1 {
		t.Fatalf("expected one ineligibility reason, got %+v", target["ineligibility_reasons"])
	}
	missingRequirements, ok := target["missing_requirements"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured missing requirements, got %+v", target["missing_requirements"])
	}
	if envs, ok := missingRequirements["env"].([]interface{}); !ok || len(envs) != 1 {
		t.Fatalf("expected missing env requirements, got %+v", missingRequirements["env"])
	}
	if pyPkgs, ok := missingRequirements["python_packages"].([]interface{}); !ok || len(pyPkgs) != 0 {
		t.Fatalf("expected missing python requirements, got %+v", missingRequirements["python_packages"])
	}
	if nodePkgs, ok := missingRequirements["node_packages"].([]interface{}); !ok || len(nodePkgs) != 0 {
		t.Fatalf("expected missing node requirements, got %+v", missingRequirements["node_packages"])
	}
	installSpecs, ok := target["install_specs"].([]interface{})
	if !ok || len(installSpecs) != 1 {
		t.Fatalf("expected one install spec, got %+v", target["install_specs"])
	}

	// Install dependencies.
	installReq := httptest.NewRequest(http.MethodPost, "/api/marketplace/skills/marketplace-test-skill/install-deps", nil)
	installRec := httptest.NewRecorder()
	installCtx := e.NewContext(installReq, installRec)
	installCtx.SetPath("/api/marketplace/skills/:id/install-deps")
	installCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})
	if err := s.handleInstallMarketplaceSkillDependencies(installCtx); err != nil {
		t.Fatalf("install deps handler failed: %v", err)
	}
	if installRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, installRec.Code)
	}

	var installPayload map[string]interface{}
	if err := json.Unmarshal(installRec.Body.Bytes(), &installPayload); err != nil {
		t.Fatalf("unmarshal install response: %v", err)
	}
	if success, _ := installPayload["success"].(bool); !success {
		t.Fatalf("expected dependency installation success, got %+v", installPayload)
	}
	results, ok := installPayload["results"].([]interface{})
	if !ok || len(results) != 1 {
		t.Fatalf("expected one install result, got %+v", installPayload["results"])
	}

	// Enable skill.
	enableReq := httptest.NewRequest(http.MethodPost, "/api/marketplace/skills/marketplace-test-skill/enable", nil)
	enableRec := httptest.NewRecorder()
	enableCtx := e.NewContext(enableReq, enableRec)
	enableCtx.SetPath("/api/marketplace/skills/:id/enable")
	enableCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})

	if err := s.handleEnableMarketplaceSkill(enableCtx); err != nil {
		t.Fatalf("enable handler failed: %v", err)
	}
	if enableRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, enableRec.Code)
	}

	var enableResp map[string]string
	if err := json.Unmarshal(enableRec.Body.Bytes(), &enableResp); err != nil {
		t.Fatalf("unmarshal enable response: %v", err)
	}
	if enableResp["status"] != "enabled" {
		t.Fatalf("expected enable status 'enabled', got %q", enableResp["status"])
	}

	enabledSkill, err := mgr.Get(skillID)
	if err != nil {
		t.Fatalf("load enabled skill: %v", err)
	}
	if !enabledSkill.Enabled {
		t.Fatalf("expected skill %q to be enabled", skillID)
	}

	// Disable skill.
	disableReq := httptest.NewRequest(http.MethodPost, "/api/marketplace/skills/marketplace-test-skill/disable", nil)
	disableRec := httptest.NewRecorder()
	disableCtx := e.NewContext(disableReq, disableRec)
	disableCtx.SetPath("/api/marketplace/skills/:id/disable")
	disableCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})

	if err := s.handleDisableMarketplaceSkill(disableCtx); err != nil {
		t.Fatalf("disable handler failed: %v", err)
	}
	if disableRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, disableRec.Code)
	}

	var disableResp map[string]string
	if err := json.Unmarshal(disableRec.Body.Bytes(), &disableResp); err != nil {
		t.Fatalf("unmarshal disable response: %v", err)
	}
	if disableResp["status"] != "disabled" {
		t.Fatalf("expected disable status 'disabled', got %q", disableResp["status"])
	}

	disabledSkill, err := mgr.Get(skillID)
	if err != nil {
		t.Fatalf("load disabled skill: %v", err)
	}
	if disabledSkill.Enabled {
		t.Fatalf("expected skill %q to be disabled", skillID)
	}
}

func TestMarketplaceHandlers_InstalledItemAndContentFlow(t *testing.T) {
	const skillID = "marketplace-test-skill"
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "marketplace-test-skill.md")
	skillContent := `---
id: marketplace-test-skill
name: Marketplace Test Skill
description: Skill fixture for marketplace detail handlers
version: 0.2.0
author: test-suite
tags:
  - marketplace
  - detail
enabled: true
always: true
requirements:
  python_packages:
    - requests
  node_packages:
    - typescript
  custom:
    install:
      - kind: custom
        command: "printf marketplace-detail-install"
---
# Marketplace Test Skill

Use this skill for detail and content route tests.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write test skill: %v", err)
	}

	log := newTestLogger(t)
	mgr := skills.NewManager(log, skillsDir, false)
	mgr.SetPythonPackageInstalled(func(pkg string) bool {
		return true
	})
	mgr.SetNodePackageInstalled(func(pkg string) bool {
		return true
	})
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}

	s := &Server{skillsMgr: mgr}
	e := echo.New()

	installedReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/installed", nil)
	installedRec := httptest.NewRecorder()
	installedCtx := e.NewContext(installedReq, installedRec)
	if err := s.handleListInstalledMarketplaceSkills(installedCtx); err != nil {
		t.Fatalf("installed handler failed: %v", err)
	}
	if installedRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, installedRec.Code)
	}

	var installedPayload map[string]interface{}
	if err := json.Unmarshal(installedRec.Body.Bytes(), &installedPayload); err != nil {
		t.Fatalf("unmarshal installed payload: %v", err)
	}
	if got, _ := installedPayload["total"].(float64); got != 1 {
		t.Fatalf("expected installed total 1, got %+v", installedPayload["total"])
	}
	records, ok := installedPayload["records"].([]interface{})
	if !ok || len(records) != 1 {
		t.Fatalf("expected one installed record, got %+v", installedPayload["records"])
	}

	itemReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/items/"+skillID, nil)
	itemRec := httptest.NewRecorder()
	itemCtx := e.NewContext(itemReq, itemRec)
	itemCtx.SetPath("/api/marketplace/skills/items/:id")
	itemCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})
	if err := s.handleGetMarketplaceSkillItem(itemCtx); err != nil {
		t.Fatalf("item handler failed: %v", err)
	}
	if itemRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, itemRec.Code)
	}

	var itemPayload map[string]interface{}
	if err := json.Unmarshal(itemRec.Body.Bytes(), &itemPayload); err != nil {
		t.Fatalf("unmarshal item payload: %v", err)
	}
	requiredItemKeys := []string{
		"id", "name", "description", "version", "author", "enabled", "always", "tags", "file_path",
		"eligible", "ineligibility_reasons", "install_specs", "is_installed", "missing_requirements",
	}
	for _, key := range requiredItemKeys {
		if _, ok := itemPayload[key]; !ok {
			t.Fatalf("expected key %q in item payload: %+v", key, itemPayload)
		}
	}
	if eligible, _ := itemPayload["eligible"].(bool); !eligible {
		t.Fatalf("expected detail skill to be eligible")
	}
	missingRequirements, ok := itemPayload["missing_requirements"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected missing requirements map, got %+v", itemPayload["missing_requirements"])
	}
	if pyPkgs, ok := missingRequirements["python_packages"].([]interface{}); !ok || len(pyPkgs) != 0 {
		t.Fatalf("expected no missing python requirements, got %+v", missingRequirements["python_packages"])
	}
	if nodePkgs, ok := missingRequirements["node_packages"].([]interface{}); !ok || len(nodePkgs) != 0 {
		t.Fatalf("expected no missing node requirements, got %+v", missingRequirements["node_packages"])
	}
	installSpecs, ok := itemPayload["install_specs"].([]interface{})
	if !ok || len(installSpecs) != 1 {
		t.Fatalf("expected one install spec, got %+v", itemPayload["install_specs"])
	}

	contentReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/items/"+skillID+"/content", nil)
	contentRec := httptest.NewRecorder()
	contentCtx := e.NewContext(contentReq, contentRec)
	contentCtx.SetPath("/api/marketplace/skills/items/:id/content")
	contentCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})
	if err := s.handleGetMarketplaceSkillContent(contentCtx); err != nil {
		t.Fatalf("content handler failed: %v", err)
	}
	if contentRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, contentRec.Code)
	}

	var contentPayload map[string]interface{}
	if err := json.Unmarshal(contentRec.Body.Bytes(), &contentPayload); err != nil {
		t.Fatalf("unmarshal content payload: %v", err)
	}
	if contentPayload["raw"] == "" {
		t.Fatalf("expected raw content, got %+v", contentPayload)
	}
	if body, _ := contentPayload["body_raw"].(string); body == "" || body == contentPayload["raw"] {
		t.Fatalf("expected extracted body_raw, got %+v", contentPayload["body_raw"])
	}
}

func TestMarketplaceHandlers_BuiltinSkillContentFlow(t *testing.T) {
	log := newTestLogger(t)
	mgr := skills.NewManager(log, t.TempDir(), false)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}

	s := &Server{skillsMgr: mgr, logger: log}
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/items/actionbook/content", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/api/marketplace/skills/items/:id/content")
	ctx.SetPathValues(echo.PathValues{{Name: "id", Value: "actionbook"}})

	if err := s.handleGetMarketplaceSkillContent(ctx); err != nil {
		t.Fatalf("content handler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got, _ := payload["id"].(string); got != "actionbook" {
		t.Fatalf("expected id actionbook, got %q", got)
	}
	if raw, _ := payload["raw"].(string); !strings.Contains(raw, "name: actionbook") {
		t.Fatalf("expected builtin raw skill content, got %q", raw)
	}
	if bodyRaw, _ := payload["body_raw"].(string); !strings.Contains(bodyRaw, "Actionbook supports two browser control modes") {
		t.Fatalf("expected builtin body content, got %q", bodyRaw)
	}
}

func TestMarketplaceHandlers_SearchAndInstallRemoteSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	log := newTestLogger(t)
	mgr := skills.NewManagerWithOptions(log, skillsDir, false, "http://127.0.0.1:9000")
	registry := &stubRemoteRegistry{
		searchOutput: "demo-skill\n  repo: https://example.com/demo.git",
		installPath:  filepath.Join(skillsDir, "demo"),
	}
	mgr.SetRemoteRegistry(registry)

	s := &Server{skillsMgr: mgr, logger: log}
	e := echo.New()

	searchReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/search?q=demo", nil)
	searchRec := httptest.NewRecorder()
	searchCtx := e.NewContext(searchReq, searchRec)
	if err := s.handleSearchMarketplaceSkills(searchCtx); err != nil {
		t.Fatalf("search handler failed: %v", err)
	}
	if searchRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, searchRec.Code)
	}

	var searchPayload map[string]interface{}
	if err := json.Unmarshal(searchRec.Body.Bytes(), &searchPayload); err != nil {
		t.Fatalf("unmarshal search payload: %v", err)
	}
	if got, _ := searchPayload["proxy"].(string); got != "http://127.0.0.1:9000" {
		t.Fatalf("expected proxy in payload, got %+v", searchPayload["proxy"])
	}
	if got, _ := searchPayload["output"].(string); got == "" {
		t.Fatalf("expected registry output, got %+v", searchPayload)
	}
	if registry.lastQuery != "demo" {
		t.Fatalf("expected registry query demo, got %q", registry.lastQuery)
	}

	installReq := httptest.NewRequest(
		http.MethodPost,
		"/api/marketplace/skills/install",
		strings.NewReader(`{"source":"https://example.com/demo.git"}`),
	)
	installReq.Header.Set("Content-Type", "application/json")
	installRec := httptest.NewRecorder()
	installCtx := e.NewContext(installReq, installRec)
	if err := s.handleInstallMarketplaceSkill(installCtx); err != nil {
		t.Fatalf("install handler failed: %v", err)
	}
	if installRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, installRec.Code)
	}

	var installPayload map[string]interface{}
	if err := json.Unmarshal(installRec.Body.Bytes(), &installPayload); err != nil {
		t.Fatalf("unmarshal install payload: %v", err)
	}
	if got, _ := installPayload["target"].(string); got != registry.installPath {
		t.Fatalf("expected install target %q, got %q", registry.installPath, got)
	}
	if registry.lastSource != "https://example.com/demo.git" {
		t.Fatalf("expected install source to be captured, got %q", registry.lastSource)
	}
	if registry.lastTarget != skillsDir {
		t.Fatalf("expected install target dir %q, got %q", skillsDir, registry.lastTarget)
	}
}

func TestMarketplaceHandlers_Return404ForUnknownSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	log := newTestLogger(t)
	mgr := skills.NewManager(log, skillsDir, false)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}

	s := &Server{skillsMgr: mgr}
	e := echo.New()

	tests := []struct {
		name    string
		method  string
		path    string
		route   string
		handler func(*echo.Context) error
	}{
		{
			name:    "enable unknown",
			method:  http.MethodPost,
			path:    "/api/marketplace/skills/unknown/enable",
			route:   "/api/marketplace/skills/:id/enable",
			handler: s.handleEnableMarketplaceSkill,
		},
		{
			name:    "disable unknown",
			method:  http.MethodPost,
			path:    "/api/marketplace/skills/unknown/disable",
			route:   "/api/marketplace/skills/:id/disable",
			handler: s.handleDisableMarketplaceSkill,
		},
		{
			name:    "item unknown",
			method:  http.MethodGet,
			path:    "/api/marketplace/skills/items/unknown",
			route:   "/api/marketplace/skills/items/:id",
			handler: s.handleGetMarketplaceSkillItem,
		},
		{
			name:    "content unknown",
			method:  http.MethodGet,
			path:    "/api/marketplace/skills/items/unknown/content",
			route:   "/api/marketplace/skills/items/:id/content",
			handler: s.handleGetMarketplaceSkillContent,
		},
		{
			name:    "install deps unknown",
			method:  http.MethodPost,
			path:    "/api/marketplace/skills/unknown/install-deps",
			route:   "/api/marketplace/skills/:id/install-deps",
			handler: s.handleInstallMarketplaceSkillDependencies,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath(tc.route)
			c.SetPathValues(echo.PathValues{{Name: "id", Value: "unknown"}})

			if err := tc.handler(c); err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
			}
		})
	}
}

func TestMarketplaceHandlers_InventorySnapshotsAndWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	skillsDir := filepath.Join(workspaceDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "inventory-test.md")
	skillContent := `---
id: inventory-test
name: Inventory Test
description: Skill fixture for inventory and snapshot handlers
version: 0.1.0
author: test-suite
enabled: true
---
Inventory body.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write test skill: %v", err)
	}

	log := newTestLogger(t)
	mgr := skills.NewManager(log, skillsDir, false)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}

	workspaceMgr := workspace.NewManager(workspaceDir, log)
	s := &Server{
		config:    config.DefaultConfig(),
		skillsMgr: mgr,
		workspace: workspaceMgr,
		logger:    log,
	}
	s.config.WebUI.SkillSnapshots.MaxCount = 2
	e := echo.New()

	inventoryReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/inventory", nil)
	inventoryRec := httptest.NewRecorder()
	inventoryCtx := e.NewContext(inventoryReq, inventoryRec)
	if err := s.handleGetMarketplaceInventory(inventoryCtx); err != nil {
		t.Fatalf("inventory handler failed: %v", err)
	}
	if inventoryRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, inventoryRec.Code)
	}

	var inventoryPayload map[string]interface{}
	if err := json.Unmarshal(inventoryRec.Body.Bytes(), &inventoryPayload); err != nil {
		t.Fatalf("unmarshal inventory payload: %v", err)
	}
	if got, _ := inventoryPayload["writable_dir"].(string); got != skillsDir {
		t.Fatalf("expected writable_dir %q, got %q", skillsDir, got)
	}
	sources, ok := inventoryPayload["sources"].([]interface{})
	if !ok || len(sources) == 0 {
		t.Fatalf("expected non-empty sources, got %+v", inventoryPayload["sources"])
	}
	versionHistory, ok := inventoryPayload["version_history"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected version_history in inventory payload: %+v", inventoryPayload)
	}
	if enabled, _ := versionHistory["enabled"].(bool); !enabled {
		t.Fatalf("expected version history enabled in inventory payload: %+v", versionHistory)
	}
	if skillCount, _ := versionHistory["skill_count"].(float64); skillCount < 1 {
		t.Fatalf("expected tracked version history skill_count in inventory payload: %+v", versionHistory)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/marketplace/skills/snapshots", strings.NewReader(`{"label":"before-change","note":"fixture"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	if err := s.handleCreateMarketplaceSkillSnapshot(createCtx); err != nil {
		t.Fatalf("create snapshot handler failed: %v", err)
	}
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, createRec.Code)
	}

	var created map[string]interface{}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created snapshot: %v", err)
	}
	snapshotID, _ := created["id"].(string)
	if snapshotID == "" {
		t.Fatalf("expected snapshot id in payload: %+v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/snapshots", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := s.handleListMarketplaceSkillSnapshots(listCtx); err != nil {
		t.Fatalf("list snapshots handler failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRec.Code)
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/marketplace/skills/snapshots/"+snapshotID+"/restore", nil)
	restoreRec := httptest.NewRecorder()
	restoreCtx := e.NewContext(restoreReq, restoreRec)
	restoreCtx.SetPath("/api/marketplace/skills/snapshots/:id/restore")
	restoreCtx.SetPathValues(echo.PathValues{{Name: "id", Value: snapshotID}})
	if err := s.handleRestoreMarketplaceSkillSnapshot(restoreCtx); err != nil {
		t.Fatalf("restore snapshot handler failed: %v", err)
	}
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, restoreRec.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/marketplace/skills/snapshots/"+snapshotID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/marketplace/skills/snapshots/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: snapshotID}})
	if err := s.handleDeleteMarketplaceSkillSnapshot(deleteCtx); err != nil {
		t.Fatalf("delete snapshot handler failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, deleteRec.Code)
	}

	for idx := 0; idx < 3; idx++ {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/marketplace/skills/snapshots",
			strings.NewReader(fmt.Sprintf(`{"label":"checkpoint-%d"}`, idx)),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)
		if err := s.handleCreateMarketplaceSkillSnapshot(ctx); err != nil {
			t.Fatalf("create prune snapshot %d handler failed: %v", idx, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d for prune snapshot %d, got %d", http.StatusOK, idx, rec.Code)
		}
	}

	pruneReq := httptest.NewRequest(http.MethodPost, "/api/marketplace/skills/snapshots/prune", nil)
	pruneRec := httptest.NewRecorder()
	pruneCtx := e.NewContext(pruneReq, pruneRec)
	if err := s.handlePruneMarketplaceSkillSnapshots(pruneCtx); err != nil {
		t.Fatalf("prune snapshot handler failed: %v", err)
	}
	if pruneRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, pruneRec.Code)
	}

	var pruned map[string]interface{}
	if err := json.Unmarshal(pruneRec.Body.Bytes(), &pruned); err != nil {
		t.Fatalf("unmarshal prune payload: %v", err)
	}
	if deleted, _ := pruned["deleted"].(float64); deleted != 1 {
		t.Fatalf("expected 1 pruned snapshot, got %+v", pruned)
	}

	postPruneReq := httptest.NewRequest(http.MethodGet, "/api/marketplace/skills/snapshots", nil)
	postPruneRec := httptest.NewRecorder()
	postPruneCtx := e.NewContext(postPruneReq, postPruneRec)
	if err := s.handleListMarketplaceSkillSnapshots(postPruneCtx); err != nil {
		t.Fatalf("post-prune list snapshots handler failed: %v", err)
	}
	if postPruneRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, postPruneRec.Code)
	}

	var postPrune map[string]interface{}
	if err := json.Unmarshal(postPruneRec.Body.Bytes(), &postPrune); err != nil {
		t.Fatalf("unmarshal post-prune payload: %v", err)
	}
	if total, _ := postPrune["total"].(float64); total != 2 {
		t.Fatalf("expected 2 snapshots after prune, got %+v", postPrune)
	}
	if maxCount, _ := postPrune["max_count"].(float64); maxCount != 2 {
		t.Fatalf("expected max_count 2 in snapshot list payload, got %+v", postPrune)
	}

	versionCleanupReq := httptest.NewRequest(http.MethodPost, "/api/marketplace/skills/versions/cleanup", nil)
	versionCleanupRec := httptest.NewRecorder()
	versionCleanupCtx := e.NewContext(versionCleanupReq, versionCleanupRec)
	if err := s.handleCleanupMarketplaceSkillVersions(versionCleanupCtx); err != nil {
		t.Fatalf("cleanup skill versions handler failed: %v", err)
	}
	if versionCleanupRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, versionCleanupRec.Code)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/workspace/status", nil)
	statusRec := httptest.NewRecorder()
	statusCtx := e.NewContext(statusReq, statusRec)
	if err := s.handleGetWorkspaceStatus(statusCtx); err != nil {
		t.Fatalf("workspace status handler failed: %v", err)
	}
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusRec.Code)
	}

	var beforeRepair map[string]interface{}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &beforeRepair); err != nil {
		t.Fatalf("unmarshal workspace status payload: %v", err)
	}
	if bootstrapped, _ := beforeRepair["bootstrapped"].(bool); bootstrapped {
		t.Fatalf("expected workspace to be unbootstrapped before repair")
	}
	if contract, ok := beforeRepair["contract"].(map[string]interface{}); !ok || contract["kind"] != "session" {
		t.Fatalf("expected workspace contract kind session, got %+v", beforeRepair["contract"])
	}

	repairReq := httptest.NewRequest(http.MethodPost, "/api/workspace/repair", nil)
	repairRec := httptest.NewRecorder()
	repairCtx := e.NewContext(repairReq, repairRec)
	if err := s.handleRepairWorkspace(repairCtx); err != nil {
		t.Fatalf("workspace repair handler failed: %v", err)
	}
	if repairRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, repairRec.Code)
	}

	var afterRepair map[string]interface{}
	if err := json.Unmarshal(repairRec.Body.Bytes(), &afterRepair); err != nil {
		t.Fatalf("unmarshal workspace repair payload: %v", err)
	}
	if bootstrapped, _ := afterRepair["bootstrapped"].(bool); !bootstrapped {
		t.Fatalf("expected workspace to be bootstrapped after repair")
	}
}
