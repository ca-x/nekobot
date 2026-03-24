package qmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekobot/pkg/logger"
)

func TestSessionExporterSkipsMetadataAndSanitizesContent(t *testing.T) {
	sessionDir := t.TempDir()
	exportDir := t.TempDir()
	content := strings.Join([]string{
		`{"_type":"metadata","key":"chat:1","created_at":"2026-03-24T00:00:00Z","updated_at":"2026-03-24T00:00:00Z","metadata":{"source":"webui"}}`,
		`{"role":"user","content":"contact me at foo@example.com and token sk-test-1234567890abcdef"}`,
		`{"role":"assistant","content":"call 13800138000"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "chat_1.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	exporter := NewSessionExporter(newTestLogger(t), exportDir, 7)
	if err := exporter.ExportSession(context.Background(), "chat:1", filepath.Join(sessionDir, "chat_1.jsonl")); err != nil {
		t.Fatalf("ExportSession failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(exportDir, "chat:1.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	exported := string(data)
	if strings.Contains(exported, "## ") && strings.Contains(exported, "## \n\n") {
		t.Fatalf("expected exporter to skip metadata-only section, got %q", exported)
	}
	if !strings.Contains(exported, "[REDACTED_EMAIL]") {
		t.Fatalf("expected email redacted, got %q", exported)
	}
	if !strings.Contains(exported, "[REDACTED_PHONE]") {
		t.Fatalf("expected phone redacted, got %q", exported)
	}
	if !strings.Contains(exported, "[REDACTED_API_KEY]") {
		t.Fatalf("expected api key redacted, got %q", exported)
	}
}

func TestManagerExportSessionsUsesConfiguredDirs(t *testing.T) {
	sessionDir := t.TempDir()
	exportDir := t.TempDir()
	content := strings.Join([]string{
		`{"_type":"metadata","key":"chat:2","created_at":"2026-03-24T00:00:00Z","updated_at":"2026-03-24T00:00:00Z","metadata":{"source":"webui"}}`,
		`{"role":"user","content":"hello memory"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "chat_2.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	manager := &Manager{
		log:       newTestLogger(t),
		config:    Config{Sessions: SessionsConfig{Enabled: true, SessionsDir: sessionDir, ExportDir: exportDir, RetentionDays: 7}},
		available: true,
	}

	if err := manager.exportSessions(context.Background()); err != nil {
		t.Fatalf("exportSessions failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(exportDir, "chat_2.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "hello memory") {
		t.Fatalf("expected exported markdown to include session content, got %q", string(data))
	}
}

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}
