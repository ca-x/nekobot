package tools

import (
	"context"
	"strings"
	"testing"
)

func TestBrowserToolStartModeFromParams(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	got, err := tool.startMode(map[string]interface{}{})
	if err != nil {
		t.Fatalf("startMode returned error for default params: %v", err)
	}
	if got != BrowserModeAuto {
		t.Fatalf("expected default auto mode, got %q", got)
	}

	got, err = tool.startMode(map[string]interface{}{"mode": "direct"})
	if err != nil {
		t.Fatalf("startMode returned error for direct mode: %v", err)
	}
	if got != BrowserModeDirect {
		t.Fatalf("expected direct mode, got %q", got)
	}
}

func TestBrowserToolExecuteRejectsInvalidMode(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "navigate",
		"url":    "https://example.com",
		"mode":   "relay",
	})
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
	if !strings.Contains(err.Error(), "invalid browser mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}
