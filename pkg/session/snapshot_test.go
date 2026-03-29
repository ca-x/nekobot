package session

import (
	"os"
	"testing"

	"nekobot/pkg/config"
)

func TestSnapshotStoreSaveAndUndo(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	cfg := config.UndoConfig{
		Enabled:       true,
		MaxTurns:      10,
		SnapshotFiles: true,
	}

	store := NewSnapshotStore("test-session", tmpDir, cfg)

	// Test SaveSnapshot
	messages1 := []MessageSnapshot{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	err := store.SaveSnapshot(messages1, "")
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Verify turn count
	if store.GetTurnCount() != 1 {
		t.Errorf("Expected turn count 1, got %d", store.GetTurnCount())
	}

	// Add another turn
	messages2 := []MessageSnapshot{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "I'm good!"},
	}

	err = store.SaveSnapshot(messages2, "")
	if err != nil {
		t.Fatalf("SaveSnapshot failed for turn 2: %v", err)
	}

	// Verify turn count
	if store.GetTurnCount() != 2 {
		t.Errorf("Expected turn count 2, got %d", store.GetTurnCount())
	}

	// Test CanUndo
	if !store.CanUndo() {
		t.Error("Expected CanUndo to be true")
	}

	// Test Undo
	undone, err := store.Undo()
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	if undone == nil {
		t.Fatal("Expected undone messages, got nil")
	}

	// After undo, should have first turn's messages
	if len(undone) != 2 {
		t.Errorf("Expected 2 messages after undo, got %d", len(undone))
	}

	// Verify turn count after undo
	if store.GetTurnCount() != 1 {
		t.Errorf("Expected turn count 1 after undo, got %d", store.GetTurnCount())
	}

	// Test Undo at beginning
	undone2, err := store.Undo()
	if err != nil {
		t.Fatalf("Undo at beginning failed: %v", err)
	}

	if undone2 != nil {
		t.Error("Expected nil when undoing at beginning")
	}

	// Test CanUndo at beginning
	if store.CanUndo() {
		t.Error("Expected CanUndo to be false at beginning")
	}
}

func TestSnapshotStoreCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.UndoConfig{
		Enabled:       true,
		MaxTurns:      3, // Small max turns to trigger checkpoints
		SnapshotFiles: true,
	}

	store := NewSnapshotStore("test-checkpoint", tmpDir, cfg)

	// Add multiple turns
	for i := 0; i < 5; i++ {
		messages := make([]MessageSnapshot, i+1)
		for j := 0; j <= i; j++ {
			messages[j] = MessageSnapshot{
				Role:    "user",
				Content: string(rune('A'+j)),
			}
		}
		err := store.SaveSnapshot(messages, "")
		if err != nil {
			t.Fatalf("SaveSnapshot failed for turn %d: %v", i, err)
		}
	}

	// Verify snapshots file exists
	snapshotPath := store.getSnapshotPath()
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Error("Expected snapshot file to exist")
	}

	// Test LoadSnapshots
	newStore := NewSnapshotStore("test-checkpoint", tmpDir, cfg)
	err := newStore.LoadSnapshots()
	if err != nil {
		t.Fatalf("LoadSnapshots failed: %v", err)
	}

	// Verify loaded turn count
	if newStore.GetTurnCount() == 0 {
		t.Error("Expected loaded turn count > 0")
	}
}

func TestSnapshotManager(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.UndoConfig{
		Enabled:       true,
		MaxTurns:      10,
		SnapshotFiles: true,
	}

	mgr := NewSnapshotManager(tmpDir, cfg)

	// Test GetStore
	store1 := mgr.GetStore("session-1")
	if store1 == nil {
		t.Fatal("Expected non-nil store")
	}

	// Test GetStore returns same instance
	store2 := mgr.GetStore("session-1")
	if store2 != store1 {
		t.Error("Expected same store instance")
	}

	// Test ListStores
	stores := mgr.ListStores()
	if len(stores) != 1 {
		t.Errorf("Expected 1 store, got %d", len(stores))
	}

	// Test RemoveStore
	err := mgr.RemoveStore("session-1")
	if err != nil {
		t.Fatalf("RemoveStore failed: %v", err)
	}

	stores = mgr.ListStores()
	if len(stores) != 0 {
		t.Errorf("Expected 0 stores after remove, got %d", len(stores))
	}
}

func TestSnapshotStoreClear(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.UndoConfig{
		Enabled:       true,
		MaxTurns:      10,
		SnapshotFiles: true,
	}

	store := NewSnapshotStore("test-clear", tmpDir, cfg)

	// Add some turns
	messages := []MessageSnapshot{
		{Role: "user", Content: "Hello"},
	}
	err := store.SaveSnapshot(messages, "")
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Test Clear
	err = store.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify cleared
	if store.GetTurnCount() != 0 {
		t.Errorf("Expected turn count 0 after clear, got %d", store.GetTurnCount())
	}

	// Verify file removed
	snapshotPath := store.getSnapshotPath()
	if _, err := os.Stat(snapshotPath); !os.IsNotExist(err) {
		t.Error("Expected snapshot file to be removed after clear")
	}
}

func TestMessageSnapshotConversion(t *testing.T) {
	// Test conversion helper function
	msg := MessageSnapshot{
		Role:       "assistant",
		Content:    "Hello, world!",
		ToolCallID: "",
		ToolCalls: []ToolCallSnapshot{
			{
				ID:   "call-1",
				Name: "test_tool",
				Arguments: map[string]interface{}{
					"key": "value",
				},
			},
		},
	}

	if msg.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", msg.Role)
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(msg.ToolCalls))
	}

	if msg.ToolCalls[0].Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", msg.ToolCalls[0].Name)
	}
}

func TestSnapshotStoreMaxTurns(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.UndoConfig{
		Enabled:       true,
		MaxTurns:      3,
		SnapshotFiles: true,
	}

	store := NewSnapshotStore("test-max-turns", tmpDir, cfg)

	// Add more turns than max
	for i := 0; i < 10; i++ {
		messages := make([]MessageSnapshot, i+1)
		for j := 0; j <= i; j++ {
			messages[j] = MessageSnapshot{
				Role:    "user",
				Content: string(rune('A' + j)),
			}
		}
		err := store.SaveSnapshot(messages, "")
		if err != nil {
			t.Fatalf("SaveSnapshot failed for turn %d: %v", i, err)
		}
	}

	// Verify turn count is limited
	turnCount := store.GetTurnCount()
	if turnCount > cfg.MaxTurns {
		t.Errorf("Expected turn count <= %d, got %d", cfg.MaxTurns, turnCount)
	}
}

func TestSnapshotStoreInMemoryReconstruction(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.UndoConfig{
		Enabled:       true,
		MaxTurns:      10,
		SnapshotFiles: true,
	}

	store := NewSnapshotStore("test-reconstruction", tmpDir, cfg)

	// Add turns
	messages1 := []MessageSnapshot{
		{Role: "user", Content: "First"},
		{Role: "assistant", Content: "Response 1"},
	}
	store.SaveSnapshot(messages1, "")

	messages2 := []MessageSnapshot{
		{Role: "user", Content: "First"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Second"},
		{Role: "assistant", Content: "Response 2"},
	}
	store.SaveSnapshot(messages2, "")

	// Test GetCurrentMessages
	current, err := store.GetCurrentMessages()
	if err != nil {
		t.Fatalf("GetCurrentMessages failed: %v", err)
	}

	if len(current) != 4 {
		t.Errorf("Expected 4 messages, got %d", len(current))
	}

	// Undo and verify
	_, err = store.Undo()
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	current, err = store.GetCurrentMessages()
	if err != nil {
		t.Fatalf("GetCurrentMessages after undo failed: %v", err)
	}

	if len(current) != 2 {
		t.Errorf("Expected 2 messages after undo, got %d", len(current))
	}
}
