package webui

import (
	"encoding/json"
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

func TestSkillsHandlers_Return503WithoutSkillsManager(t *testing.T) {
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
			path:    "/api/skills",
			handler: s.handleListSkills,
		},
		{
			name:    "get skill item",
			method:  http.MethodGet,
			path:    "/api/skills/missing",
			handler: s.handleGetSkill,
			paramID: "missing",
		},
		{
			name:    "get skill content",
			method:  http.MethodGet,
			path:    "/api/skills/missing/content",
			handler: s.handleGetSkillContent,
			paramID: "missing",
		},
		{
			name:    "enable skill",
			method:  http.MethodPost,
			path:    "/api/skills/missing/enable",
			handler: s.handleEnableSkill,
			paramID: "missing",
		},
		{
			name:    "disable skill",
			method:  http.MethodPost,
			path:    "/api/skills/missing/disable",
			handler: s.handleDisableSkill,
			paramID: "missing",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			if tc.paramID != "" {
				c.SetPath("/api/skills/:id")
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

func TestSkillsHandlers_ListEnableDisableFlow(t *testing.T) {
	const skillID = "skills-test-skill"
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "skills-test-skill.md")
	skillContent := `---
id: skills-test-skill
name: Skills Test Skill
description: Skill fixture for skills handlers
version: 0.1.0
author: test-suite
tags:
  - test
enabled: false
requirements:
  env:
    - SKILLS_TEST_TOKEN
  python_packages:
    - requests
  node_packages:
    - typescript
  custom:
    install:
      - method: command
        package: "printf skills-install-ok"
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
	listReq := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := s.handleListSkills(listCtx); err != nil {
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

	// Enable skill.
	enableReq := httptest.NewRequest(http.MethodPost, "/api/skills/"+skillID+"/enable", nil)
	enableRec := httptest.NewRecorder()
	enableCtx := e.NewContext(enableReq, enableRec)
	enableCtx.SetPath("/api/skills/:id/enable")
	enableCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})

	if err := s.handleEnableSkill(enableCtx); err != nil {
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
	disableReq := httptest.NewRequest(http.MethodPost, "/api/skills/"+skillID+"/disable", nil)
	disableRec := httptest.NewRecorder()
	disableCtx := e.NewContext(disableReq, disableRec)
	disableCtx.SetPath("/api/skills/:id/disable")
	disableCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})

	if err := s.handleDisableSkill(disableCtx); err != nil {
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

func TestSkillsHandlers_ItemAndContentFlow(t *testing.T) {
	const skillID = "skills-detail-test"
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "skills-detail-test.md")
	skillContent := `---
id: skills-detail-test
name: Skills Detail Test
description: Skill fixture for detail handlers
version: 0.2.0
author: test-suite
tags:
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
        command: "printf detail-install"
---
# Skills Detail Test

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

	// Get skill item.
	itemReq := httptest.NewRequest(http.MethodGet, "/api/skills/"+skillID, nil)
	itemRec := httptest.NewRecorder()
	itemCtx := e.NewContext(itemReq, itemRec)
	itemCtx.SetPath("/api/skills/:id")
	itemCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})
	if err := s.handleGetSkill(itemCtx); err != nil {
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

	// Get skill content.
	contentReq := httptest.NewRequest(http.MethodGet, "/api/skills/"+skillID+"/content", nil)
	contentRec := httptest.NewRecorder()
	contentCtx := e.NewContext(contentReq, contentRec)
	contentCtx.SetPath("/api/skills/:id/content")
	contentCtx.SetPathValues(echo.PathValues{{Name: "id", Value: skillID}})
	if err := s.handleGetSkillContent(contentCtx); err != nil {
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

func TestSkillsHandlers_BuiltinSkillContentFlow(t *testing.T) {
	log := newTestLogger(t)
	mgr := skills.NewManager(log, t.TempDir(), false)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}

	s := &Server{skillsMgr: mgr, logger: log}
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/skills/actionbook/content", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/api/skills/:id/content")
	ctx.SetPathValues(echo.PathValues{{Name: "id", Value: "actionbook"}})

	if err := s.handleGetSkillContent(ctx); err != nil {
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

func TestSkillsHandlers_Return404ForUnknownSkill(t *testing.T) {
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
			path:    "/api/skills/unknown/enable",
			route:   "/api/skills/:id/enable",
			handler: s.handleEnableSkill,
		},
		{
			name:    "disable unknown",
			method:  http.MethodPost,
			path:    "/api/skills/unknown/disable",
			route:   "/api/skills/:id/disable",
			handler: s.handleDisableSkill,
		},
		{
			name:    "item unknown",
			method:  http.MethodGet,
			path:    "/api/skills/unknown",
			route:   "/api/skills/:id",
			handler: s.handleGetSkill,
		},
		{
			name:    "content unknown",
			method:  http.MethodGet,
			path:    "/api/skills/unknown/content",
			route:   "/api/skills/:id/content",
			handler: s.handleGetSkillContent,
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

func TestWorkspaceHandlers_InventoryAndRepair(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	skillsDir := filepath.Join(workspaceDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "workspace-test.md")
	skillContent := `---
id: workspace-test
name: Workspace Test
description: Skill fixture for workspace handlers
version: 0.1.0
author: test-suite
enabled: true
---
Workspace body.
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
	e := echo.New()

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
	if summary, ok := beforeRepair["validation_summary"].(map[string]interface{}); !ok || summary["on_turn_end"] == nil {
		t.Fatalf("expected workspace validation summary, got %+v", beforeRepair["validation_summary"])
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
