package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// Logger writes audit entries to a JSONL file.
type Logger struct {
	config    Config
	workspace string
	log       *logger.Logger

	mu       sync.Mutex
	file     *os.File
	filePath string

	cleanupRunning atomic.Bool
	lastCleanupAt  atomic.Int64
}

// NewLogger creates a new audit logger.
func NewLogger(config Config, workspace string, log *logger.Logger) *Logger {
	if config.MaxArgLength <= 0 {
		config.MaxArgLength = 1000
	}

	auditDir := filepath.Join(workspace, ".nekobot")

	return &Logger{
		config:    config,
		workspace: workspace,
		log:       log,
		filePath:  filepath.Join(auditDir, "audit.jsonl"),
	}
}

// Log records a tool execution to the audit log.
// This method is safe for concurrent use and does not block on I/O errors.
func (l *Logger) Log(entry *Entry) {
	if !l.config.Enabled {
		return
	}

	// Truncate arguments to prevent log bloat
	entry.Arguments = l.truncateArgs(entry.Arguments)

	// Write to file
	if err := l.appendEntry(entry); err != nil {
		l.log.Debug("Failed to write audit entry", zap.Error(err))
	}

	// Clean up old entries if configured
	if l.config.MaxResults > 0 || l.config.RetentionDays > 0 {
		l.scheduleCleanup()
	}
}

func (l *Logger) scheduleCleanup() {
	now := time.Now()
	last := time.Unix(0, l.lastCleanupAt.Load())
	if !last.IsZero() && now.Sub(last) < time.Minute {
		return
	}
	if !l.cleanupRunning.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer l.cleanupRunning.Store(false)
		l.lastCleanupAt.Store(time.Now().UnixNano())
		l.maybeCleanup()
	}()
}

// appendEntry appends a single entry to the JSONL file.
func (l *Logger) appendEntry(entry *Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(l.filePath), 0755); err != nil {
		return fmt.Errorf("creating audit directory: %w", err)
	}

	// Open file in append mode (create if not exists)
	if l.file == nil {
		f, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("opening audit file: %w", err)
		}
		l.file = f
	}

	// Write entry as JSON line
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}

	if _, err := l.file.Write(append(data, '\n')); err != nil {
		// Close and retry on error
		_ = l.file.Close()
		l.file = nil
		return fmt.Errorf("writing entry: %w", err)
	}

	return nil
}

// truncateArgs truncates long string values in arguments.
func (l *Logger) truncateArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}

	result := make(map[string]interface{}, len(args))
	for k, v := range args {
		result[k] = l.truncateValue(v)
	}
	return result
}

// truncateValue truncates a value if it's a long string.
func (l *Logger) truncateValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		if len(val) > l.config.MaxArgLength {
			return val[:l.config.MaxArgLength] + "... [truncated]"
		}
		return val
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, v := range val {
			result[k] = l.truncateValue(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = l.truncateValue(v)
		}
		return result
	default:
		return v
	}
}

// maybeCleanup periodically removes old entries.
func (l *Logger) maybeCleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Read all entries
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	var entries []*Entry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}

	// Filter by retention
	if l.config.RetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -l.config.RetentionDays)
		var filtered []*Entry
		for _, e := range entries {
			if e.Timestamp.After(cutoff) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Limit count
	if l.config.MaxResults > 0 && len(entries) > l.config.MaxResults {
		entries = entries[len(entries)-l.config.MaxResults:]
	}

	// Rewrite file
	if len(entries) < len(lines)-1 { // Only if we removed something
		l.writeEntries(entries)
	}
}

// writeEntries rewrites the entire audit log with the given entries.
func (l *Logger) writeEntries(entries []*Entry) error {
	// Close existing file
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	// Create temp file
	tmpPath := l.filePath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	for _, e := range entries {
		if err := encoder.Encode(e); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, l.filePath)
}

// Close closes the audit log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// ReadLast reads the last N entries from the audit log.
func (l *Logger) ReadLast(n int) ([]*Entry, error) {
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var entries []*Entry

	// Read from end
	for i := len(lines) - 1; i >= 0 && len(entries) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append([]*Entry{&entry}, entries...) // Prepend to maintain order
	}

	return entries, nil
}

// Clear removes all audit entries.
func (l *Logger) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	return os.Remove(l.filePath)
}

// FilePath returns the path to the audit log file.
func (l *Logger) FilePath() string {
	return l.filePath
}

// Stats returns statistics about the audit log.
func (l *Logger) Stats() (map[string]interface{}, error) {
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{
				"exists":  false,
				"entries": 0,
			}, nil
		}
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	info, _ := os.Stat(l.filePath)

	return map[string]interface{}{
		"exists":   true,
		"entries":  count,
		"size":     len(data),
		"file":     l.filePath,
		"modified": info.ModTime(),
	}, nil
}
