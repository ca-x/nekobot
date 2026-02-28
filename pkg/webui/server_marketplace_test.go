package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/skills"
)

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
---
Use this skill for tests.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write test skill: %v", err)
	}

	log := newTestLogger(t)
	mgr := skills.NewManager(log, skillsDir, false)
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

	requiredKeys := []string{"id", "name", "description", "version", "author", "enabled", "always", "file_path", "tags"}
	for _, key := range requiredKeys {
		if _, ok := target[key]; !ok {
			t.Fatalf("expected key %q in payload item: %+v", key, target)
		}
	}
	if enabled, _ := target["enabled"].(bool); enabled {
		t.Fatalf("expected skill to be disabled initially")
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
		path    string
		handler func(*echo.Context) error
	}{
		{
			name:    "enable unknown",
			path:    "/api/marketplace/skills/unknown/enable",
			handler: s.handleEnableMarketplaceSkill,
		},
		{
			name:    "disable unknown",
			path:    "/api/marketplace/skills/unknown/disable",
			handler: s.handleDisableMarketplaceSkill,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/api/marketplace/skills/:id")
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
