package config

import (
	"context"
	"fmt"
	"sync"

	_ "github.com/lib-x/entsqlite"

	"nekobot/pkg/storage/ent"
)

const runtimeSQLiteDSN = "file:%s?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(DELETE)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)"

var ensureRuntimeEntSchemaMu sync.Mutex

// OpenRuntimeEntClient opens an Ent client for the unified runtime database.
func OpenRuntimeEntClient(cfg *Config) (*ent.Client, error) {
	dbPath, err := RuntimeDBPath(cfg)
	if err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf(runtimeSQLiteDSN, dbPath)
	client, err := ent.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open runtime database: %w", err)
	}
	return client, nil
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
