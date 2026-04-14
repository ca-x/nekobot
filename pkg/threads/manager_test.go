package threads

import (
	"path/filepath"
	"testing"

	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

func TestManagerUpsertGetListDelete(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: filepath.Join(t.TempDir(), "threads-state.json")})
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mgr := NewManager(store)
	if err := mgr.Upsert(t.Context(), "thread-1", "runtime-1", "ops triage"); err != nil {
		t.Fatalf("upsert thread: %v", err)
	}
	record, ok, err := mgr.Get(t.Context(), "thread-1")
	if err != nil {
		t.Fatalf("get thread: %v", err)
	}
	if !ok || record.RuntimeID != "runtime-1" || record.Topic != "ops triage" {
		t.Fatalf("unexpected thread record: %+v ok=%v", record, ok)
	}
	items, err := mgr.List(t.Context())
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if len(items) != 1 || items[0].ID != "thread-1" {
		t.Fatalf("unexpected thread list: %+v", items)
	}
	if err := mgr.Delete(t.Context(), "thread-1"); err != nil {
		t.Fatalf("delete thread: %v", err)
	}
	_, ok, err = mgr.Get(t.Context(), "thread-1")
	if err != nil {
		t.Fatalf("get deleted thread: %v", err)
	}
	if ok {
		t.Fatalf("expected deleted thread to be absent")
	}
}
