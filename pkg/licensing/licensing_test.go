package licensing

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/storage/ent"
)

func newTestClient(t *testing.T) *ent.Client {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})
	return client
}

func TestLicenseRoundTripEnablesExpandedUserLimit(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	installID, err := EnsureInstallID(ctx, client)
	if err != nil {
		t.Fatalf("ensure install id: %v", err)
	}
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}
	PublicKeyBase64 = base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { PublicKeyBase64 = "" })

	file, err := GenerateLicense(GenerateOptions{
		InstallID: installID,
		Subject:   "Acme",
		MaxUsers:  12,
		Features:  []string{"multi_user"},
	}, priv)
	if err != nil {
		t.Fatalf("generate license: %v", err)
	}
	raw, err := MarshalLicense(file)
	if err != nil {
		t.Fatalf("marshal license: %v", err)
	}

	status, err := ImportLicense(ctx, client, raw)
	if err != nil {
		t.Fatalf("import license: %v", err)
	}
	if !status.Licensed || status.MaxUsers != 12 || status.InstallID != installID {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestLicenseRejectsInstallIDMismatch(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}
	file, err := GenerateLicense(GenerateOptions{
		InstallID: "server-a",
		MaxUsers:  5,
	}, priv)
	if err != nil {
		t.Fatalf("generate license: %v", err)
	}
	raw, err := MarshalLicense(file)
	if err != nil {
		t.Fatalf("marshal license: %v", err)
	}
	if _, err := VerifyLicense(raw, "server-b", time.Now(), pub); err == nil || !strings.Contains(err.Error(), "install_id mismatch") {
		t.Fatalf("expected install_id mismatch, got %v", err)
	}
}
