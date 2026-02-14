package commands

import (
	"context"
	"strings"
	"testing"
)

func TestStatusHandler_IncludesRuntimeAndVersionInfo(t *testing.T) {
	resp, err := statusHandler(context.Background(), CommandRequest{Channel: "telegram"})
	if err != nil {
		t.Fatalf("statusHandler returned error: %v", err)
	}

	required := []string{
		"Channel: telegram",
		"Status: 🟢 Online",
		"Version:",
		"OS:",
		"Go:",
		"Uptime:",
		"Memory:",
	}
	for _, want := range required {
		if !strings.Contains(resp.Content, want) {
			t.Fatalf("expected status output to contain %q, got:\n%s", want, resp.Content)
		}
	}
}
