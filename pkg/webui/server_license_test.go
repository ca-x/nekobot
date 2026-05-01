package webui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/licensing"
	"nekobot/pkg/storage/ent"
)

func TestHandleCreateUserRequiresLicenseAfterFreeLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})
	log := newTestLogger(t)
	s := &Server{config: cfg, logger: log, entClient: client}

	createTestUser(t, client, "owner-1", "owner", true)
	createTestUser(t, client, "member-1", "member", true)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{
		"username":"member-2",
		"password":"secret-123",
		"role":"member",
		"enabled":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := s.handleCreateUser(ctx); err != nil {
		t.Fatalf("handleCreateUser failed: %v", err)
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d: %s", http.StatusPaymentRequired, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user_limit_reached") {
		t.Fatalf("expected user_limit_reached response, got %s", rec.Body.String())
	}
}

func TestImportLicenseAllowsThirdEnabledUser(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})
	log := newTestLogger(t)
	s := &Server{config: cfg, logger: log, entClient: client}

	createTestUser(t, client, "owner-1", "owner", true)
	createTestUser(t, client, "member-1", "member", true)

	installID, err := licensing.EnsureInstallID(context.Background(), client)
	if err != nil {
		t.Fatalf("install id: %v", err)
	}
	pub, priv, err := licensing.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keys: %v", err)
	}
	licensing.PublicKeyBase64 = base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { licensing.PublicKeyBase64 = "" })

	licenseFile, err := licensing.GenerateLicense(licensing.GenerateOptions{InstallID: installID, MaxUsers: 5}, priv)
	if err != nil {
		t.Fatalf("generate license: %v", err)
	}
	rawLicense, err := licensing.MarshalLicense(licenseFile)
	if err != nil {
		t.Fatalf("marshal license: %v", err)
	}
	importBody, err := json.Marshal(map[string]string{"license": rawLicense})
	if err != nil {
		t.Fatalf("marshal import body: %v", err)
	}

	e := echo.New()
	importReq := httptest.NewRequest(http.MethodPost, "/api/license/import", strings.NewReader(string(importBody)))
	importReq.Header.Set("Content-Type", "application/json")
	importRec := httptest.NewRecorder()
	importCtx := e.NewContext(importReq, importRec)
	if err := s.handleImportLicense(importCtx); err != nil {
		t.Fatalf("handleImportLicense failed: %v", err)
	}
	if importRec.Code != http.StatusOK {
		t.Fatalf("expected import status %d, got %d: %s", http.StatusOK, importRec.Code, importRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{
		"username":"member-2",
		"password":"secret-123",
		"role":"member",
		"enabled":true
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	if err := s.handleCreateUser(createCtx); err != nil {
		t.Fatalf("handleCreateUser failed: %v", err)
	}
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusOK, createRec.Code, createRec.Body.String())
	}
}

func createTestUser(t *testing.T, client *ent.Client, username, role string, enabled bool) {
	t.Helper()
	hash, err := config.HashPassword("secret-123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := config.CreateUser(context.Background(), client, config.UserInput{
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		Enabled:      enabled,
	}); err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
}
