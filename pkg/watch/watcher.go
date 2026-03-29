// Package watch provides file system watching with debouncing and command execution.
// It uses fsnotify for cross-platform file monitoring and supports glob patterns.
package watch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"nekobot/pkg/audit"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Event represents a file system event.
type Event struct {
	Path    string
	Op      fsnotify.Op
	WatchID string
}

// Watcher monitors file patterns and triggers commands on changes.
type Watcher struct {
	config   *config.WatchConfig
	patterns []config.WatchPattern
	log      *logger.Logger
	audit    *audit.Logger

	fsWatcher     *fsnotify.Watcher
	cronScheduler *cron.Cron

	// Debounce tracking: map of watch pattern index to timer
	mu             sync.RWMutex
	debounceTimers map[int]*time.Timer
	watchedPaths   map[string]int // path -> pattern index

	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	lastRunAt         time.Time
	lastCommand       string
	lastFile          string
	lastSuccess       bool
	lastError         string
	lastResultPreview string
}

// StatusSnapshot describes the current watcher runtime state.
type StatusSnapshot struct {
	Enabled           bool                  `json:"enabled"`
	Running           bool                  `json:"running"`
	DebounceMs        int                   `json:"debounce_ms"`
	Patterns          []config.WatchPattern `json:"patterns"`
	LastRunAt         time.Time             `json:"last_run_at,omitempty"`
	LastCommand       string                `json:"last_command,omitempty"`
	LastFile          string                `json:"last_file,omitempty"`
	LastSuccess       bool                  `json:"last_success"`
	LastError         string                `json:"last_error,omitempty"`
	LastResultPreview string                `json:"last_result_preview,omitempty"`
}

// New creates a new file watcher.
func New(cfg *config.Config, log *logger.Logger, auditLogger *audit.Logger) (*Watcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	watchCfg := cfg.Watch
	if !watchCfg.Enabled {
		return &Watcher{
			config:   &watchCfg,
			log:      log,
			audit:    auditLogger,
			patterns: watchCfg.Patterns,
		}, nil
	}

	// Set debounce default if not specified
	debounceMs := watchCfg.DebounceMs
	if debounceMs <= 0 {
		debounceMs = 300 // Default 300ms debounce
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		config:   &watchCfg,
		patterns: watchCfg.Patterns,
		log:      log,
		audit:    auditLogger,

		fsWatcher:      fsWatcher,
		cronScheduler:  cron.New(),
		debounceTimers: make(map[int]*time.Timer),
		watchedPaths:   make(map[string]int),
	}

	return w, nil
}

// Start begins watching configured patterns.
func (w *Watcher) Start() error {
	if w == nil {
		return fmt.Errorf("watcher is nil")
	}

	if !w.config.Enabled {
		if w.log != nil {
			w.log.Debug("Watch mode is disabled")
		}
		return nil
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}

	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.running = true
	w.mu.Unlock()

	// Start cron scheduler for periodic tasks
	w.cronScheduler.Start()

	// Schedule compress job if configured
	if w.config.DebounceMs > 0 {
		w.debug("Watch mode started",
			zap.Int("patterns", len(w.patterns)),
			zap.Int("debounce_ms", w.config.DebounceMs),
		)
	}

	// Watch all patterns
	for i, pattern := range w.patterns {
		if err := w.watchPattern(i, pattern); err != nil {
			if w.log != nil {
				w.log.Warn("Failed to watch pattern",
					zap.String("glob", pattern.FileGlob),
					zap.Error(err),
				)
			}
		}
	}

	// Start event loop
	go w.eventLoop()

	return nil
}

// Stop stops watching all patterns.
func (w *Watcher) Stop() error {
	if w == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	// Cancel context to stop pending operations
	if w.cancel != nil {
		w.cancel()
	}

	// Stop cron scheduler
	if w.cronScheduler != nil {
		w.cronScheduler.Stop()
	}

	// Stop all debounce timers
	for idx, timer := range w.debounceTimers {
		if timer != nil {
			timer.Stop()
		}
		delete(w.debounceTimers, idx)
	}

	// Close fsnotify watcher
	if w.fsWatcher != nil {
		_ = w.fsWatcher.Close()
	}

	w.running = false
	if w.log != nil {
		w.log.Info("Watch mode stopped")
	}

	return nil
}

// watchPattern sets up watching for a glob pattern.
func (w *Watcher) watchPattern(idx int, pattern config.WatchPattern) error {
	glob := strings.TrimSpace(pattern.FileGlob)
	if glob == "" {
		return fmt.Errorf("empty file glob")
	}

	// Find matching paths
	matches, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("glob pattern %q: %w", glob, err)
	}

	// If no matches, try to watch the base directory
	if len(matches) == 0 {
		// Extract base directory from glob
		baseDir := extractBaseDir(glob)
		if baseDir != "" {
			if err := w.addWatchPath(baseDir, idx); err != nil {
				return err
			}
			w.debug("Watching directory for glob pattern",
				zap.String("glob", glob),
				zap.String("dir", baseDir),
			)
		}
		return nil
	}

	// Watch each matching path
	for _, path := range matches {
		if err := w.addWatchPath(path, idx); err != nil {
			if w.log != nil {
				w.log.Warn("Failed to watch path",
					zap.String("path", path),
					zap.Error(err),
				)
			}
			continue
		}
		w.debug("Watching path",
			zap.String("path", path),
			zap.String("glob", glob),
		)
	}

	return nil
}

// extractBaseDir extracts the base directory from a glob pattern.
func extractBaseDir(pattern string) string {
	// Find the first glob metacharacter
	for i, c := range pattern {
		switch c {
		case '*', '?', '[', '{':
			if i == 0 {
				return "."
			}
			dir := pattern[:i]
			// Go up to find the parent directory
			if strings.HasSuffix(dir, "/") {
				dir = dir[:len(dir)-1]
			}
			if dir == "" {
				return "."
			}
			return dir
		}
	}
	return ""
}

// addWatchPath adds a path to the fsnotify watcher.
func (w *Watcher) addWatchPath(path string, patternIdx int) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// If it's a directory, watch it
	if info.IsDir() {
		if err := w.fsWatcher.Add(path); err != nil {
			return err
		}
		w.mu.Lock()
		w.watchedPaths[path] = patternIdx
		w.mu.Unlock()
		return nil
	}

	// For files, watch the parent directory
	dir := filepath.Dir(path)
	if err := w.fsWatcher.Add(dir); err != nil {
		return err
	}
	w.mu.Lock()
	w.watchedPaths[dir] = patternIdx
	w.mu.Unlock()

	return nil
}

// eventLoop processes fsnotify events with debouncing.
func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleFSEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			if w.log != nil {
				w.log.Error("File watcher error", zap.Error(err))
			}
		}
	}
}

// handleFSEvent processes a single fsnotify event.
func (w *Watcher) handleFSEvent(event fsnotify.Event) {
	// Find matching pattern
	patternIdx, found := w.findMatchingPattern(event.Name)
	if !found {
		return
	}

	// Check if file matches any glob in the pattern
	pattern := w.patterns[patternIdx]
	if !matchesGlob(event.Name, pattern.FileGlob) {
		return
	}

	// Log the event
	w.debug("File change detected",
		zap.String("path", event.Name),
		zap.String("op", event.Op.String()),
		zap.String("glob", pattern.FileGlob),
	)

	// Debounce and execute command
	w.debounceAndExecute(patternIdx, event)
}

// findMatchingPattern finds the pattern index for a path.
func (w *Watcher) findMatchingPattern(path string) (int, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Check direct path matches
	for watchedPath, idx := range w.watchedPaths {
		if path == watchedPath || strings.HasPrefix(path, watchedPath+string(filepath.Separator)) {
			return idx, true
		}
	}
	return 0, false
}

// matchesGlob checks if a path matches a glob pattern.
func matchesGlob(path, pattern string) bool {
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	if matched {
		return true
	}

	// Also try matching just the filename
	filename := filepath.Base(path)
	matched, err = filepath.Match(pattern, filename)
	return err == nil && matched
}

// debounceAndExecute schedules command execution with debouncing.
func (w *Watcher) debounceAndExecute(patternIdx int, event fsnotify.Event) {
	debounceMs := w.config.DebounceMs
	if debounceMs <= 0 {
		debounceMs = 300
	}

	// Cancel existing timer for this pattern
	w.mu.Lock()
	if existingTimer := w.debounceTimers[patternIdx]; existingTimer != nil {
		existingTimer.Stop()
	}

	// Create new timer
	timer := time.AfterFunc(time.Duration(debounceMs)*time.Millisecond, func() {
		w.executeCommand(patternIdx, event)
	})
	w.debounceTimers[patternIdx] = timer
	w.mu.Unlock()
}

// executeCommand runs the configured command for a pattern.
func (w *Watcher) executeCommand(patternIdx int, event fsnotify.Event) {
	pattern := w.patterns[patternIdx]
	command := strings.TrimSpace(pattern.Command)

	if command == "" {
		return
	}

	// Create audit entry
	auditEntry := &audit.Entry{
		Timestamp: time.Now(),
		ToolName:  "watch",
		Arguments: map[string]interface{}{
			"pattern": pattern.FileGlob,
			"command": command,
			"file":    event.Name,
			"op":      event.Op.String(),
		},
	}

	// Execute command in background
	go func() {
		startTime := time.Now()

		// Use shell to execute the command
		cmd := execShellCommand(w.ctx, command)

		// Capture output
		output, err := cmd.CombinedOutput()
		duration := time.Since(startTime)

		// Log result
		if err != nil {
			if w.log != nil {
				w.log.Warn("Watch command failed",
					zap.String("command", command),
					zap.String("file", event.Name),
					zap.Duration("duration", duration),
					zap.Error(err),
				)
			}

			// Execute fail command if configured
			if pattern.FailCommand != "" {
				w.executeFailCommand(pattern.FailCommand, event, err)
			}

			auditEntry.Success = false
			auditEntry.Error = err.Error()
		} else {
			if w.log != nil {
				w.log.Info("Watch command executed",
					zap.String("command", command),
					zap.String("file", event.Name),
					zap.Duration("duration", duration),
				)
			}
			auditEntry.Success = true
		}

		// Set duration and result preview
		auditEntry.DurationMs = duration.Milliseconds()
		if len(output) > 0 {
			outputStr := string(output)
			if len(outputStr) > 500 {
				outputStr = outputStr[:500] + "... [truncated]"
			}
			auditEntry.ResultPreview = outputStr
		}

		w.mu.Lock()
		w.lastRunAt = time.Now()
		w.lastCommand = command
		w.lastFile = event.Name
		w.lastSuccess = err == nil
		if err != nil {
			w.lastError = err.Error()
		} else {
			w.lastError = ""
		}
		w.lastResultPreview = auditEntry.ResultPreview
		w.mu.Unlock()

		// Log to audit
		if w.audit != nil {
			w.audit.Log(auditEntry)
		}
	}()
}

// executeFailCommand runs the fail command when the main command fails.
func (w *Watcher) executeFailCommand(failCommand string, event fsnotify.Event, origErr error) {
	command := strings.TrimSpace(failCommand)
	if command == "" {
		return
	}

	w.debug("Executing fail command",
		zap.String("command", command),
		zap.String("original_error", origErr.Error()),
	)

	// Execute fail command
	cmd := execShellCommand(w.ctx, command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if w.log != nil {
			w.log.Error("Fail command failed",
				zap.String("command", command),
				zap.Error(err),
			)
		}
	} else {
		if w.log != nil {
			w.log.Info("Fail command executed",
				zap.String("command", command),
			)
		}
	}

	if len(output) > 0 {
		w.debug("Fail command output",
			zap.String("output", truncateString(string(output), 500)),
		)
	}
}

// truncateString truncates a string to the given length.
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// IsRunning returns whether the watcher is currently running.
func (w *Watcher) IsRunning() bool {
	if w == nil {
		return false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// Patterns returns the configured watch patterns.
func (w *Watcher) Patterns() []config.WatchPattern {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	patterns := make([]config.WatchPattern, len(w.patterns))
	copy(patterns, w.patterns)
	return patterns
}

// Status returns a point-in-time runtime snapshot for UI and diagnostics.
func (w *Watcher) Status() StatusSnapshot {
	if w == nil {
		return StatusSnapshot{}
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	patterns := make([]config.WatchPattern, len(w.patterns))
	copy(patterns, w.patterns)

	debounceMs := 0
	enabled := false
	if w.config != nil {
		debounceMs = w.config.DebounceMs
		enabled = w.config.Enabled
	}

	return StatusSnapshot{
		Enabled:           enabled,
		Running:           w.running,
		DebounceMs:        debounceMs,
		Patterns:          patterns,
		LastRunAt:         w.lastRunAt,
		LastCommand:       w.lastCommand,
		LastFile:          w.lastFile,
		LastSuccess:       w.lastSuccess,
		LastError:         w.lastError,
		LastResultPreview: w.lastResultPreview,
	}
}

// UpdateConfig swaps the in-memory watcher config snapshot.
func (w *Watcher) UpdateConfig(cfg config.WatchConfig) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.config = &cfg
	w.patterns = append([]config.WatchPattern(nil), cfg.Patterns...)
}

// execShellCommand creates and returns a command that executes the given shell command.
func execShellCommand(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func (w *Watcher) debug(msg string, fields ...zap.Field) {
	if w != nil && w.log != nil {
		w.log.Debug(msg, fields...)
	}
}
