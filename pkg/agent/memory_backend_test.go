package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/configsection"
)

func TestNewMemoryStoreFromConfig_UsesKVBackend(t *testing.T) {
	workspace := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Memory.Enabled = true
	cfg.Memory.Backend = "kv"
	cfg.Memory.KVPrefix = "test-memory"

	kvStore := newTestKVStore(t)
	store := newMemoryStoreFromConfig(cfg, workspace, kvStore, nil)

	if _, ok := store.backend.(*memoryKVBackend); !ok {
		t.Fatalf("expected memoryKVBackend, got %T", store.backend)
	}

	if err := store.WriteLongTerm("hello-kv"); err != nil {
		t.Fatalf("write long-term memory to kv backend: %v", err)
	}

	if got := store.ReadLongTerm(); got != "hello-kv" {
		t.Fatalf("expected kv long-term memory %q, got %q", "hello-kv", got)
	}

	exists, err := kvStore.Exists(context.Background(), "test-memory:long_term")
	if err != nil {
		t.Fatalf("check kv key existence: %v", err)
	}
	if !exists {
		t.Fatalf("expected kv key %q to exist", "test-memory:long_term")
	}
}

func TestNewMemoryStoreFromConfig_UsesDBBackend(t *testing.T) {
	workspace := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Memory.Enabled = true
	cfg.Memory.Backend = "db"
	cfg.Memory.DBPrefix = "memtest"

	dbClient := newTestEntClient(t)
	store := newMemoryStoreFromConfig(cfg, workspace, nil, dbClient)

	dbBackend, ok := store.backend.(*memoryDBBackend)
	if !ok {
		t.Fatalf("expected memoryDBBackend, got %T", store.backend)
	}

	if err := store.WriteLongTerm("hello-db"); err != nil {
		t.Fatalf("write long-term memory to db backend: %v", err)
	}

	longTermRec, err := dbClient.ConfigSection.Query().
		Where(configsection.SectionEQ("memtest:long_term")).
		Only(context.Background())
	if err != nil {
		t.Fatalf("query long-term section: %v", err)
	}
	if longTermRec.PayloadJSON != "hello-db" {
		t.Fatalf("expected payload %q, got %q", "hello-db", longTermRec.PayloadJSON)
	}

	day := time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC)
	if err := dbBackend.WriteDaily(context.Background(), day, "db-daily-note"); err != nil {
		t.Fatalf("write daily memory to db backend: %v", err)
	}

	dailyRec, err := dbClient.ConfigSection.Query().
		Where(configsection.SectionEQ("memtest:daily:20260228")).
		Only(context.Background())
	if err != nil {
		t.Fatalf("query daily section: %v", err)
	}
	if dailyRec.PayloadJSON != "db-daily-note" {
		t.Fatalf("expected payload %q, got %q", "db-daily-note", dailyRec.PayloadJSON)
	}
}

func TestNewMemoryStoreFromConfig_FallsBackToFileWhenKVUnavailable(t *testing.T) {
	workspace := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Memory.Enabled = true
	cfg.Memory.Backend = "kv"
	cfg.Memory.FilePath = filepath.Join(workspace, "fallback-memory")

	store := newMemoryStoreFromConfig(cfg, workspace, nil, nil)
	if _, ok := store.backend.(*memoryFileBackend); !ok {
		t.Fatalf("expected fallback memoryFileBackend, got %T", store.backend)
	}

	if err := store.WriteLongTerm("fallback-content"); err != nil {
		t.Fatalf("write fallback long-term memory: %v", err)
	}

	memoryFile := filepath.Join(workspace, "fallback-memory", "MEMORY.md")
	if _, err := os.Stat(memoryFile); err != nil {
		t.Fatalf("expected fallback file backend to create %s: %v", memoryFile, err)
	}
}

func newTestKVStore(t *testing.T) state.KV {
	t.Helper()

	log, err := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	store, err := state.NewFileStore(log, &state.FileStoreConfig{
		FilePath: filepath.Join(t.TempDir(), "state.json"),
		AutoSave: false,
	})
	if err != nil {
		t.Fatalf("create test kv store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	return store
}

func newTestEntClient(t *testing.T) *ent.Client {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		t.Fatalf("ensure runtime ent schema: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}
