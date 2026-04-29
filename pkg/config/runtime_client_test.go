package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDBOpenConfigDefaultsToSQLiteFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	dbType, dsn, err := RuntimeDBOpenConfig(cfg)
	if err != nil {
		t.Fatalf("RuntimeDBOpenConfig failed: %v", err)
	}
	if dbType != "sqlite" {
		t.Fatalf("expected sqlite type, got %q", dbType)
	}
	wantPath := filepath.Join(cfg.Storage.DBDir, RuntimeDBName)
	if !strings.Contains(dsn, "file:"+wantPath) {
		t.Fatalf("expected sqlite dsn to contain %q, got %q", wantPath, dsn)
	}
	if !strings.Contains(dsn, "_pragma=foreign_keys(1)") || !strings.Contains(dsn, "_pragma=busy_timeout(10000)") {
		t.Fatalf("expected sqlite dsn pragmas, got %q", dsn)
	}
}

func TestRuntimeDBOpenConfigWrapsPlainSQLiteDSNPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBDSN = filepath.Join(t.TempDir(), "custom.db")

	dbType, dsn, err := RuntimeDBOpenConfig(cfg)
	if err != nil {
		t.Fatalf("RuntimeDBOpenConfig failed: %v", err)
	}
	if dbType != "sqlite" {
		t.Fatalf("expected sqlite type, got %q", dbType)
	}
	if !strings.HasPrefix(dsn, "file:"+cfg.Storage.DBDSN) {
		t.Fatalf("expected sqlite file dsn, got %q", dsn)
	}
	if !strings.Contains(dsn, "_pragma=synchronous(NORMAL)") {
		t.Fatalf("expected sqlite pragmas, got %q", dsn)
	}
}

func TestRuntimeDBOpenConfigUsesExternalDSNFromEnv(t *testing.T) {
	t.Setenv(DBTypeEnv, "postgres")
	t.Setenv(DBDSNEnv, "postgres://user:secret@localhost:5432/nekobot?sslmode=disable")

	dbType, dsn, err := RuntimeDBOpenConfig(DefaultConfig())
	if err != nil {
		t.Fatalf("RuntimeDBOpenConfig failed: %v", err)
	}
	if dbType != "postgres" {
		t.Fatalf("expected postgres type, got %q", dbType)
	}
	if dsn != "postgres://user:secret@localhost:5432/nekobot?sslmode=disable" {
		t.Fatalf("unexpected dsn %q", dsn)
	}
}

func TestEnsureRuntimeDBFileSkipsExternalDatabaseFileCreation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBType = "postgres"
	cfg.Storage.DBDSN = "postgres://user:secret@localhost:5432/nekobot?sslmode=disable"

	display, err := EnsureRuntimeDBFile(cfg)
	if err != nil {
		t.Fatalf("EnsureRuntimeDBFile failed: %v", err)
	}
	if strings.Contains(display, "secret") {
		t.Fatalf("expected redacted display name, got %q", display)
	}
	if !strings.HasPrefix(display, "postgres:postgres://****@localhost:5432/nekobot") {
		t.Fatalf("unexpected display name %q", display)
	}
}

func TestRedactDatabaseDSNHidesMySQLCredentials(t *testing.T) {
	got := RedactDatabaseDSN("nekobot:secret@tcp(mysql:3306)/nekobot?parseTime=true")
	if strings.Contains(got, "secret") || strings.Contains(got, "nekobot:") {
		t.Fatalf("expected credentials to be redacted, got %q", got)
	}
	if got != "****@tcp(mysql:3306)/nekobot?parseTime=true" {
		t.Fatalf("unexpected redacted dsn %q", got)
	}
}

func TestValidateConfigRejectsExternalDatabaseWithoutDSN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DBType = "mysql"

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "storage.db_dsn") {
		t.Fatalf("expected storage.db_dsn validation error, got %v", err)
	}
}
