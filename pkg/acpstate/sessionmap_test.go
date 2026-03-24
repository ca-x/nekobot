package acpstate

import "testing"

func TestSessionMapSupportsStringAndInterfaceMaps(t *testing.T) {
	metadata := map[string]interface{}{
		MetadataKey: map[string]interface{}{
			"chat-1": "sess-1",
			"chat-2": 123,
		},
	}
	got := SessionMap(metadata)
	if got["chat-1"] != "sess-1" {
		t.Fatalf("expected chat-1 mapping, got %#v", got)
	}
	if _, ok := got["chat-2"]; ok {
		t.Fatalf("expected invalid entry to be skipped, got %#v", got)
	}

	metadata = map[string]interface{}{
		MetadataKey: map[string]string{"chat-3": "sess-3"},
	}
	got = SessionMap(metadata)
	if got["chat-3"] != "sess-3" {
		t.Fatalf("expected chat-3 mapping, got %#v", got)
	}
}

func TestSetConversationSessionPreservesExistingMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"driver": "acp",
		MetadataKey: map[string]interface{}{
			"chat-1": "sess-1",
		},
	}

	updated := SetConversationSession(metadata, "chat-2", "sess-2")
	if updated["driver"] != "acp" {
		t.Fatalf("expected driver metadata to be preserved, got %#v", updated)
	}
	sessions := SessionMap(updated)
	if sessions["chat-1"] != "sess-1" || sessions["chat-2"] != "sess-2" {
		t.Fatalf("unexpected session map: %#v", sessions)
	}
}
