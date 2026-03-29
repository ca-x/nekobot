package session

import (
	"os"
	"path/filepath"
	"testing"

	"nekobot/pkg/config"
)

func TestSessionPersistsAllowedSourceAndFilteredContent(t *testing.T) {
	cfg := config.DefaultConfig().Sessions
	cfg.Sources = config.SessionSourcesConfig{
		WebUI: true,
	}
	cfg.Content = config.SessionContentConfig{
		UserMessages:      true,
		AssistantMessages: false,
		SystemMessages:    false,
		ToolCalls:         true,
		ToolResults:       false,
	}

	manager := NewManager(t.TempDir(), cfg)
	sess, err := manager.GetWithSource("webui-test", SourceWebUI)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	sess.AddMessage(Message{Role: "user", Content: "hello"})
	sess.AddMessage(Message{
		Role:    "assistant",
		Content: "hidden reply",
		ToolCalls: []ToolCall{{
			ID:        "call-1",
			Name:      "read_file",
			Arguments: map[string]interface{}{"path": "/tmp/demo"},
		}},
	})
	sess.AddMessage(Message{Role: "tool", Content: "tool output", ToolCallID: "call-1"})

	reloaded := NewManager(manager.baseDir, cfg)
	loaded, err := reloaded.GetExisting("webui-test")
	if err != nil {
		t.Fatalf("GetExisting failed: %v", err)
	}

	messages := loaded.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 persisted messages, got %d", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Content != "hello" {
		t.Fatalf("unexpected user message: %#v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content != "" || len(messages[1].ToolCalls) != 1 {
		t.Fatalf("unexpected assistant message: %#v", messages[1])
	}
	if messages[1].ToolCalls[0].Name != "read_file" {
		t.Fatalf("unexpected tool call: %#v", messages[1].ToolCalls[0])
	}
}

func TestSessionDoesNotPersistDisabledSource(t *testing.T) {
	cfg := config.DefaultConfig().Sessions
	cfg.Sources = config.SessionSourcesConfig{
		WebUI: false,
	}

	manager := NewManager(t.TempDir(), cfg)
	sess, err := manager.GetWithSource("disabled-webui", SourceWebUI)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}

	sess.AddMessage(Message{Role: "user", Content: "hello"})

	reloaded := NewManager(manager.baseDir, cfg)
	if _, err := reloaded.GetExisting("disabled-webui"); !os.IsNotExist(err) {
		t.Fatalf("expected session file to be absent, got err=%v", err)
	}
}

func TestGetHistorySafeExpandsToKeepAssistantToolGroup(t *testing.T) {
	sess := &Session{
		ID: "history-safe",
		Messages: []Message{
			{Role: "user", Content: "first"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "call-1", Name: "read_file", Arguments: map[string]interface{}{"path": "/tmp/a"}},
				},
			},
			{Role: "tool", Content: "file contents", ToolCallID: "call-1"},
			{Role: "assistant", Content: "done"},
		},
	}

	history := sess.GetHistorySafe(2)
	if len(history) != 3 {
		t.Fatalf("expected 3 messages after safe expansion, got %d", len(history))
	}
	if history[0].Role != "assistant" || len(history[0].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool-call turn retained, got %#v", history[0])
	}
	if history[1].Role != "tool" || history[1].ToolCallID != "call-1" {
		t.Fatalf("expected matching tool result retained, got %#v", history[1])
	}
	if history[2].Role != "assistant" || history[2].Content != "done" {
		t.Fatalf("expected trailing assistant message retained, got %#v", history[2])
	}
}

func TestSnapshotStoreSaveAndUndo(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Undo.Enabled = true
	cfg.Undo.MaxTurns = 4

	store := NewSnapshotStore("cli:test", t.TempDir(), cfg.Undo)
	first := []MessageSnapshot{{Role: "user", Content: "hello"}}
	second := append(append([]MessageSnapshot{}, first...), MessageSnapshot{Role: "assistant", Content: "world"})

	if err := store.SaveSnapshot(first, ""); err != nil {
		t.Fatalf("SaveSnapshot first failed: %v", err)
	}
	if err := store.SaveSnapshot(second, "summary"); err != nil {
		t.Fatalf("SaveSnapshot second failed: %v", err)
	}
	if !store.CanUndo() {
		t.Fatal("expected CanUndo to be true")
	}

	messages, err := store.Undo()
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message after undo, got %d", len(messages))
	}
	if messages[0].Content != "hello" {
		t.Fatalf("expected reverted message content hello, got %#v", messages[0])
	}
}

func TestSnapshotStoreLoadSnapshots(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Undo.Enabled = true
	cfg.Undo.MaxTurns = 4

	baseDir := t.TempDir()
	store := NewSnapshotStore("cli:test", baseDir, cfg.Undo)
	if err := store.SaveSnapshot([]MessageSnapshot{{Role: "user", Content: "one"}}, ""); err != nil {
		t.Fatalf("SaveSnapshot first failed: %v", err)
	}
	if err := store.SaveSnapshot([]MessageSnapshot{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
	}, ""); err != nil {
		t.Fatalf("SaveSnapshot second failed: %v", err)
	}

	reloaded := NewSnapshotStore("cli:test", baseDir, cfg.Undo)
	if err := reloaded.LoadSnapshots(); err != nil {
		t.Fatalf("LoadSnapshots failed: %v", err)
	}
	if got := reloaded.GetTurnCount(); got != 2 {
		t.Fatalf("expected 2 turns after reload, got %d", got)
	}

	path := filepath.Join(baseDir, "cli_test.snapshots.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected snapshot file %s to exist: %v", path, err)
	}
}
