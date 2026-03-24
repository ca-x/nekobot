package qmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/session"
)

func TestSessionExporterSkipsMetadataAndSanitizesContent(t *testing.T) {
	sessionDir := t.TempDir()
	exportDir := t.TempDir()

	cfg := config.DefaultConfig().Sessions
	cfg.Sources = config.SessionSourcesConfig{WebUI: true}
	cfg.Content = config.SessionContentConfig{UserMessages: true, AssistantMessages: true}
	mgr := session.NewManager(sessionDir, cfg)
	sess, err := mgr.GetWithSource("chat:1", session.SourceWebUI)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}
	sess.AddMessage(agent.Message{Role: "user", Content: "contact me at foo@example.com and token sk-test-1234567890abcdef"})
	sess.AddMessage(agent.Message{Role: "assistant", Content: "call 13800138000"})

	exporter := NewSessionExporter(newTestLogger(t), exportDir, 7)
	if err := exporter.ExportSession(context.Background(), "chat:1", filepath.Join(sessionDir, "chat_1.jsonl")); err != nil {
		t.Fatalf("ExportSession failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(exportDir, "chat:1.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "## ") && strings.Contains(content, "## \n\n") {
		t.Fatalf("expected exporter to skip metadata-only section, got %q", content)
	}
	if !strings.Contains(content, "[REDACTED_EMAIL]") {
		t.Fatalf("expected email redacted, got %q", content)
	}
	if !strings.Contains(content, "[REDACTED_PHONE]") {
		t.Fatalf("expected phone redacted, got %q", content)
	}
	if !strings.Contains(content, "[REDACTED_API_KEY]") {
		t.Fatalf("expected api key redacted, got %q", content)
	}
}

func TestManagerExportSessionsUsesConfiguredDirs(t *testing.T) {
	sessionDir := t.TempDir()
	exportDir := t.TempDir()

	cfg := config.DefaultConfig().Sessions
	cfg.Sources = config.SessionSourcesConfig{WebUI: true}
	cfg.Content = config.SessionContentConfig{UserMessages: true}
	mgr := session.NewManager(sessionDir, cfg)
	sess, err := mgr.GetWithSource("chat:2", session.SourceWebUI)
	if err != nil {
		t.Fatalf("GetWithSource failed: %v", err)
	}
	sess.AddMessage(agent.Message{Role: "user", Content: "hello memory"})

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
