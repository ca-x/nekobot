package state

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nekobot/pkg/logger"
)

func TestFileStore(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	log, err := logger.New(&logger.Config{
		Level: "error",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create store
	store, err := NewFileStore(log, &FileStoreConfig{
		FilePath:     statePath,
		AutoSave:     false,
		SaveInterval: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	ctx := t.Context()

	// Test Set/Get string
	if err := store.Set(ctx, "key1", "value1"); err != nil {
		t.Fatalf("set key1: %v", err)
	}
	value, exists, err := store.GetString(ctx, "key1")
	if err != nil {
		t.Fatalf("GetString error: %v", err)
	}
	if !exists {
		t.Error("Key1 should exist")
	}
	if value != "value1" {
		t.Errorf("Expected 'value1', got '%s'", value)
	}

	// Test Set/Get int
	if err := store.Set(ctx, "key2", 42); err != nil {
		t.Fatalf("set key2: %v", err)
	}
	intVal, exists, err := store.GetInt(ctx, "key2")
	if err != nil {
		t.Fatalf("GetInt error: %v", err)
	}
	if !exists {
		t.Error("Key2 should exist")
	}
	if intVal != 42 {
		t.Errorf("Expected 42, got %d", intVal)
	}

	// Test Set/Get bool
	if err := store.Set(ctx, "key3", true); err != nil {
		t.Fatalf("set key3: %v", err)
	}
	boolVal, exists, err := store.GetBool(ctx, "key3")
	if err != nil {
		t.Fatalf("GetBool error: %v", err)
	}
	if !exists {
		t.Error("Key3 should exist")
	}
	if !boolVal {
		t.Error("Expected true, got false")
	}

	// Test Set/Get map
	testMap := map[string]interface{}{
		"nested": "value",
		"count":  10,
	}
	if err := store.Set(ctx, "key4", testMap); err != nil {
		t.Fatalf("set key4: %v", err)
	}
	mapVal, exists, err := store.GetMap(ctx, "key4")
	if err != nil {
		t.Fatalf("GetMap error: %v", err)
	}
	if !exists {
		t.Error("Key4 should exist")
	}
	if mapVal["nested"] != "value" {
		t.Error("Map value mismatch")
	}

	// Test Keys
	keys, err := store.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys error: %v", err)
	}
	if len(keys) != 4 {
		t.Errorf("Expected 4 keys, got %d", len(keys))
	}

	// Test Exists
	exists, err = store.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("key1 should exist")
	}
	exists, _ = store.Exists(ctx, "nonexistent")
	if exists {
		t.Error("nonexistent should not exist")
	}

	// Test Delete
	if err := store.Delete(ctx, "key1"); err != nil {
		t.Fatalf("delete key1: %v", err)
	}
	exists, _ = store.Exists(ctx, "key1")
	if exists {
		t.Error("key1 should be deleted")
	}

	// Test Save/Load
	if err := store.Save(); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Create new store to test load
	store2, err := NewFileStore(log, &FileStoreConfig{
		FilePath: statePath,
		AutoSave: false,
	})
	if err != nil {
		t.Fatalf("Failed to create store2: %v", err)
	}
	t.Cleanup(func() {
		if err := store2.Close(); err != nil {
			t.Fatalf("close store2: %v", err)
		}
	})

	// Verify loaded data
	intVal2, exists, err := store2.GetInt(ctx, "key2")
	if err != nil {
		t.Fatalf("GetInt error after load: %v", err)
	}
	if !exists {
		t.Error("key2 should exist after load")
	}
	if intVal2 != 42 {
		t.Errorf("Expected 42 after load, got %d", intVal2)
	}
}

func TestFileStoreAutoSave(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	log, _ := logger.New(&logger.Config{Level: "error", OutputPath: ""})

	store, err := NewFileStore(log, &FileStoreConfig{
		FilePath:     statePath,
		AutoSave:     true,
		SaveInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	ctx := t.Context()

	// Set value
	if err := store.Set(ctx, "test", "auto-save"); err != nil {
		t.Fatalf("set test: %v", err)
	}

	// Wait for auto-save
	time.Sleep(200 * time.Millisecond)

	// Verify file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("State file should exist after auto-save")
	}

	// Load in new store
	store2, err := NewFileStore(log, &FileStoreConfig{
		FilePath: statePath,
		AutoSave: false,
	})
	if err != nil {
		t.Fatalf("Failed to create store2: %v", err)
	}
	t.Cleanup(func() {
		if err := store2.Close(); err != nil {
			t.Fatalf("close store2: %v", err)
		}
	})

	value, exists, err := store2.GetString(ctx, "test")
	if err != nil || !exists || value != "auto-save" {
		t.Error("Value should persist after auto-save")
	}
}

func TestFileStoreUpdateFunc(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	log, _ := logger.New(&logger.Config{Level: "error", OutputPath: ""})

	store, err := NewFileStore(log, &FileStoreConfig{
		FilePath: statePath,
		AutoSave: false,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	ctx := t.Context()

	// Set initial value
	if err := store.Set(ctx, "counter", 0); err != nil {
		t.Fatalf("set counter: %v", err)
	}

	// Update using function
	if err := store.UpdateFunc(ctx, "counter", func(current interface{}) interface{} {
		if current == nil {
			return 1
		}
		if count, ok := current.(int); ok {
			return count + 1
		}
		return current
	}); err != nil {
		t.Fatalf("increment counter: %v", err)
	}

	// Verify
	value, exists, err := store.GetInt(ctx, "counter")
	if err != nil || !exists || value != 1 {
		t.Errorf("Expected 1, got %d", value)
	}

	// Update again
	if err := store.UpdateFunc(ctx, "counter", func(current interface{}) interface{} {
		if count, ok := current.(int); ok {
			return count + 10
		}
		return current
	}); err != nil {
		t.Fatalf("add to counter: %v", err)
	}

	value, _, err = store.GetInt(ctx, "counter")
	if err != nil {
		t.Fatalf("get updated counter: %v", err)
	}
	if value != 11 {
		t.Errorf("Expected 11, got %d", value)
	}
}

func TestFileStoreClear(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	log, _ := logger.New(&logger.Config{Level: "error", OutputPath: ""})

	store, err := NewFileStore(log, &FileStoreConfig{
		FilePath: statePath,
		AutoSave: false,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	ctx := t.Context()

	// Add some data
	if err := store.Set(ctx, "key1", "value1"); err != nil {
		t.Fatalf("set key1: %v", err)
	}
	if err := store.Set(ctx, "key2", "value2"); err != nil {
		t.Fatalf("set key2: %v", err)
	}

	keys, _ := store.Keys(ctx)
	if len(keys) != 2 {
		t.Error("Should have 2 keys before clear")
	}

	// Clear
	if err := store.Clear(ctx); err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	keys, _ = store.Keys(ctx)
	if len(keys) != 0 {
		t.Error("Should have 0 keys after clear")
	}
}

func BenchmarkFileStoreSet(b *testing.B) {
	tmpDir := b.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	log, _ := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	store, _ := NewFileStore(log, &FileStoreConfig{
		FilePath: statePath,
		AutoSave: false,
	})
	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Fatalf("close store: %v", err)
		}
	})

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		if err := store.Set(context.Background(), "benchmark", i); err != nil {
			b.Fatalf("set benchmark: %v", err)
		}
	}
}

func BenchmarkFileStoreGet(b *testing.B) {
	tmpDir := b.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	log, _ := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	store, _ := NewFileStore(log, &FileStoreConfig{
		FilePath: statePath,
		AutoSave: false,
	})
	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Fatalf("close store: %v", err)
		}
	})

	if err := store.Set(context.Background(), "benchmark", 42); err != nil {
		b.Fatalf("set benchmark: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		if _, _, err := store.GetInt(context.Background(), "benchmark"); err != nil {
			b.Fatalf("get benchmark: %v", err)
		}
	}
}
