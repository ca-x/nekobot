// Package qmd provides session export functionality for QMD indexing.
package qmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"nekobot/pkg/logger"
)

// SessionMessage represents a message in a session.
type SessionMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// SessionExporter handles exporting sessions to markdown.
type SessionExporter struct {
	log       *logger.Logger
	exportDir string
	retention int // days
}

// NewSessionExporter creates a new session exporter.
func NewSessionExporter(log *logger.Logger, exportDir string, retentionDays int) *SessionExporter {
	return &SessionExporter{
		log:       log,
		exportDir: exportDir,
		retention: retentionDays,
	}
}

// ExportSession exports a single session to markdown.
func (se *SessionExporter) ExportSession(ctx context.Context, sessionID string, sessionPath string) error {
	// Read session JSONL file
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return fmt.Errorf("reading session file: %w", err)
	}

	// Parse JSONL (one JSON object per line)
	lines := strings.Split(string(data), "\n")
	messages := make([]SessionMessage, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			se.log.Warn("Failed to parse message",
				zap.String("session", sessionID),
				zap.Error(err))
			continue
		}
		if rawType, ok := raw["_type"]; ok {
			var kind string
			if err := json.Unmarshal(rawType, &kind); err == nil && kind == "metadata" {
				continue
			}
		}

		var msg SessionMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			se.log.Warn("Failed to parse message",
				zap.String("session", sessionID),
				zap.Error(err))
			continue
		}

		messages = append(messages, msg)
	}

	if len(messages) == 0 {
		return fmt.Errorf("no messages in session")
	}

	// Convert to markdown
	markdown := se.convertToMarkdown(sessionID, messages)

	// Ensure export directory exists
	if err := os.MkdirAll(se.exportDir, 0755); err != nil {
		return fmt.Errorf("creating export directory: %w", err)
	}

	// Write markdown file
	mdPath := filepath.Join(se.exportDir, sessionID+".md")
	if err := os.WriteFile(mdPath, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("writing markdown: %w", err)
	}

	se.log.Debug("Exported session to markdown",
		zap.String("session", sessionID),
		zap.String("path", mdPath))

	return nil
}

// convertToMarkdown converts session messages to markdown format.
func (se *SessionExporter) convertToMarkdown(sessionID string, messages []SessionMessage) string {
	var sb strings.Builder

	// Write header
	_, _ = fmt.Fprintf(&sb, "# Session: %s\n\n", sessionID)

	if len(messages) > 0 && !messages[0].Timestamp.IsZero() {
		_, _ = fmt.Fprintf(&sb, "**Date**: %s\n\n", messages[0].Timestamp.Format("2006-01-02 15:04:05"))
	}

	sb.WriteString("---\n\n")

	// Write messages
	titleCaser := cases.Title(language.English)
	for i, msg := range messages {
		// Write message header
		role := titleCaser.String(strings.ToLower(msg.Role))
		_, _ = fmt.Fprintf(&sb, "## %s\n\n", role)

		// Write message content with basic redaction to avoid leaking common secrets into QMD.
		sb.WriteString(sanitizeText(msg.Content))
		sb.WriteString("\n\n")

		// Add separator between messages (except last)
		if i < len(messages)-1 {
			sb.WriteString("---\n\n")
		}
	}

	return sb.String()
}

// ExportAllSessions exports all sessions from a directory.
func (se *SessionExporter) ExportAllSessions(ctx context.Context, sessionsDir string) error {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return fmt.Errorf("reading sessions directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		sessionPath := filepath.Join(sessionsDir, entry.Name())

		if err := se.ExportSession(ctx, sessionID, sessionPath); err != nil {
			se.log.Warn("Failed to export session",
				zap.String("session", sessionID),
				zap.Error(err))
			continue
		}

		count++
	}

	se.log.Info("Exported sessions to markdown",
		zap.Int("count", count))

	return nil
}

var (
	apiKeyPattern   = regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token)["\s:=]+([a-zA-Z0-9_\-]{16,})`)
	passwordPattern = regexp.MustCompile(`(?i)(password|passwd|pwd)["\s:=]+([^\s]+)`)
	emailPattern    = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phonePattern    = regexp.MustCompile(`1[3-9]\d{9}`)
)

func sanitizeText(text string) string {
	text = apiKeyPattern.ReplaceAllString(text, `$1 [REDACTED_API_KEY]`)
	text = passwordPattern.ReplaceAllString(text, `$1 [REDACTED_PASSWORD]`)
	text = emailPattern.ReplaceAllString(text, "[REDACTED_EMAIL]")
	text = phonePattern.ReplaceAllString(text, "[REDACTED_PHONE]")
	return text
}

// CleanupOldExports removes exported sessions older than retention period.
func (se *SessionExporter) CleanupOldExports(ctx context.Context) error {
	_, err := se.CleanupOldExportsCount(ctx)
	return err
}

// CleanupOldExportsCount removes exported sessions older than retention period and returns the deleted count.
func (se *SessionExporter) CleanupOldExportsCount(ctx context.Context) (int, error) {
	if se.retention <= 0 {
		// No retention limit
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -se.retention)

	entries, err := os.ReadDir(se.exportDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading export directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(se.exportDir, entry.Name())
			if err := os.Remove(path); err != nil {
				se.log.Warn("Failed to remove old export",
					zap.String("path", path),
					zap.Error(err))
				continue
			}
			removed++
		}
	}

	if removed > 0 {
		se.log.Info("Cleaned up old session exports",
			zap.Int("removed", removed),
			zap.Int("retention_days", se.retention))
	}

	return removed, nil
}
