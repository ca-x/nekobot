package config

import (
	"context"
	"fmt"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib-x/entsqlite"
	_ "github.com/lib/pq"

	"nekobot/pkg/storage/ent"
)

const runtimeSQLiteDSN = "file:%s?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(DELETE)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)"

var ensureRuntimeEntSchemaMu sync.Mutex

// OpenRuntimeEntClient opens an Ent client for the unified runtime database.
func OpenRuntimeEntClient(cfg *Config) (*ent.Client, error) {
	dbType, dsn, err := RuntimeDBOpenConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := ent.Open(entDialectForDatabaseType(dbType), dsn)
	if err != nil {
		return nil, fmt.Errorf("open runtime database: %w", err)
	}
	return client, nil
}

// RuntimeDBOpenConfig returns the normalized database type and Ent-compatible DSN.
func RuntimeDBOpenConfig(cfg *Config) (string, string, error) {
	if cfg == nil {
		return "", "", fmt.Errorf("config is nil")
	}
	dbType := cfg.DatabaseType()
	switch dbType {
	case "sqlite":
		dsn := strings.TrimSpace(cfg.DatabaseDSN())
		if dsn == "" {
			dbPath, err := RuntimeDBPath(cfg)
			if err != nil {
				return "", "", err
			}
			dsn = fmt.Sprintf(runtimeSQLiteDSN, dbPath)
		} else {
			dsn = sqliteDSNFromConfiguredValue(dsn)
		}
		return dbType, dsn, nil
	case "postgres", "mysql":
		dsn := strings.TrimSpace(cfg.DatabaseDSN())
		if dsn == "" {
			return "", "", fmt.Errorf("storage.db_dsn or %s is required for %s runtime database", DBDSNEnv, dbType)
		}
		return dbType, dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported runtime database type %q", dbType)
	}
}

func entDialectForDatabaseType(dbType string) string {
	switch normalizeDatabaseType(dbType) {
	case "postgres":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		return "sqlite3"
	}
}

func sqliteDSNFromConfiguredValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "file:") || strings.Contains(trimmed, "?") {
		return trimmed
	}
	return fmt.Sprintf(runtimeSQLiteDSN, trimmed)
}

// EnsureRuntimeEntSchema creates/updates required Ent tables in runtime DB.
func EnsureRuntimeEntSchema(client *ent.Client) error {
	if client == nil {
		return fmt.Errorf("ent client is nil")
	}

	ensureRuntimeEntSchemaMu.Lock()
	defer ensureRuntimeEntSchemaMu.Unlock()

	if err := client.Schema.Create(context.Background()); err != nil {
		return fmt.Errorf("ensure runtime schema: %w", err)
	}
	return nil
}
