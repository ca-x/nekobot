package approval

import (
	"testing"

	"nekobot/pkg/providers"
)

func TestRememberAndClearPendingToolCall(t *testing.T) {
	call := providers.UnifiedToolCall{Name: "exec", Arguments: map[string]interface{}{"command": "pwd"}}
	if err := RememberPendingToolCall("approval-1", "sess-1", call); err != nil {
		t.Fatalf("remember pending tool call: %v", err)
	}
	stored, ok := PendingToolCallForRequest("approval-1")
	if !ok {
		t.Fatal("expected stored pending tool call")
	}
	if stored.SessionID != "sess-1" || stored.Call.Name != "exec" {
		t.Fatalf("unexpected stored pending tool call: %+v", stored)
	}
	ClearPendingToolCall("approval-1")
	if _, ok := PendingToolCallForRequest("approval-1"); ok {
		t.Fatal("expected pending tool call to be cleared")
	}
}
