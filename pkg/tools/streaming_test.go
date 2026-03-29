package tools

import (
	"testing"
	"time"
)

func TestStreamWriterIncludesSessionIDInUpdates(t *testing.T) {
	var updates []StreamingUpdate
	writer := NewStreamWriter(func(update StreamingUpdate) {
		updates = append(updates, update)
	}, "session-123", time.Nanosecond)

	if _, err := writer.WriteString("hello\n"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	writer.Finish(0, "")

	if len(updates) < 2 {
		t.Fatalf("expected at least 2 updates, got %d", len(updates))
	}

	if updates[0].SessionID != "session-123" {
		t.Fatalf("expected session ID on streaming update, got %#v", updates[0])
	}

	last := updates[len(updates)-1]
	if last.SessionID != "session-123" || !last.Done {
		t.Fatalf("expected final update to include session ID and done state, got %#v", last)
	}
}
