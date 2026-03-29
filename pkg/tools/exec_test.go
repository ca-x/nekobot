package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecToolReportsStreamingFallbackWithoutHandler(t *testing.T) {
	tool := NewExecTool(t.TempDir(), false, ExecConfig{Timeout: 5 * time.Second}, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":   "printf 'ok'",
		"streaming": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Streaming requested but no streaming handler was attached") {
		t.Fatalf("expected streaming fallback notice, got:\n%s", result)
	}
	if !strings.Contains(result, "ok") {
		t.Fatalf("expected command output in result, got:\n%s", result)
	}
}
