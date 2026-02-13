// Package qmd provides session export functionality for QMD indexing.
package qmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
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
	sb.WriteString(fmt.Sprintf("# Session: %s\n\n", sessionID))

	if len(messages) > 0 && !messages[0].Timestamp.IsZero() {
		sb.WriteString(fmt.Sprintf("**Date**: %s\n\n", messages[0].Timestamp.Format("2006-01-02 15:04:05")))
	}

	sb.WriteString("---\n\n")

	// Write messages
	for i, msg := range messages {
		// Write message header
		role := strings.Title(strings.ToLower(msg.Role))
		sb.WriteString(fmt.Sprintf("## %s\n\n", role))

		// Write message content
		sb.WriteString(msg.Content)
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

// CleanupOldExports removes exported sessions older than retention period.
func (se *SessionExporter) CleanupOldExports(ctx context.Context) error {
	if se.retention <= 0 {
		// No retention limit
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -se.retention)

	entries, err := os.ReadDir(se.exportDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading export directory: %w", err)
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

	return nil
}
