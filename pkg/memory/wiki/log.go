package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogManager manages wiki/log.md.
type LogManager struct {
	wikiDir string
}

// NewLogManager creates a new log manager for one wiki directory.
func NewLogManager(wikiDir string) *LogManager {
	return &LogManager{wikiDir: wikiDir}
}

// Path returns the wiki log path.
func (m *LogManager) Path() string {
	return filepath.Join(m.wikiDir, "log.md")
}

// Append adds one log entry to wiki/log.md.
func (m *LogManager) Append(entry LogEntry) error {
	date := entry.Date
	if date.IsZero() {
		date = time.Now().UTC()
	}

	var b strings.Builder
	if _, err := os.Stat(m.Path()); os.IsNotExist(err) {
		b.WriteString("# Wiki Log\n\n")
	}
	_, _ = fmt.Fprintf(&b, "## %s — %s — %s\n", date.Format(time.RFC3339), entry.Action, entry.Subject)
	for _, detail := range entry.Details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		_, _ = fmt.Fprintf(&b, "- %s\n", detail)
	}
	b.WriteString("\n")

	if err := os.MkdirAll(m.wikiDir, 0o755); err != nil {
		return fmt.Errorf("append wiki log: create wiki directory: %w", err)
	}
	file, err := os.OpenFile(m.Path(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append wiki log: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(b.String()); err != nil {
		return fmt.Errorf("append wiki log: %w", err)
	}
	return nil
}
