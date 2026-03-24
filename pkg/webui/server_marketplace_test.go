package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/skills"
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
