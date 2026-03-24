package session

import (
	"os"
	"testing"

	"nekobot/pkg/agent"
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

	sess.AddMessage(agent.Message{Role: "user", Content: "hello"})
	sess.AddMessage(agent.Message{
		Role:    "assistant",
		Content: "hidden reply",
		ToolCalls: []agent.ToolCall{{
			ID:        "call-1",
			Name:      "read_file",
			Arguments: map[string]interface{}{"path": "/tmp/demo"},
		}},
	})
	sess.AddMessage(agent.Message{Role: "tool", Content: "tool output", ToolCallID: "call-1"})

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

	sess.AddMessage(agent.Message{Role: "user", Content: "hello"})

	reloaded := NewManager(manager.baseDir, cfg)
	if _, err := reloaded.GetExisting("disabled-webui"); !os.IsNotExist(err) {
		t.Fatalf("expected session file to be absent, got err=%v", err)
	}
}
