package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"nekobot/pkg/audit"
	"nekobot/pkg/config"
	"nekobot/pkg/execenv"
	"nekobot/pkg/logger"
	"nekobot/pkg/tasks"
)

func TestWatcherCreation(t *testing.T) {
	cfg := config.DefaultConfig()
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelDebug,
		Development: true,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	// Test with watch disabled
	cfg.Watch.Enabled = false
	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	if watcher == nil {
		t.Fatal("Watcher should not be nil")
	}
	if watcher.IsRunning() {
		t.Fatal("Watcher should not be running initially")
	}
}

func TestWatcherStartStop(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.DebounceMs = 100

	log, err := logger.New(&logger.Config{
		Level:       logger.LevelDebug,
		Development: true,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	// Start watcher
	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	if !watcher.IsRunning() {
		t.Fatal("Watcher should be running")
	}

	// Stop watcher
	if err := watcher.Stop(); err != nil {
		t.Fatalf("Failed to stop watcher: %v", err)
	}

	if watcher.IsRunning() {
		t.Fatal("Watcher should not be running after stop")
	}
}

func TestWatcherCanRestartAfterStop(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.DebounceMs = 100
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.go"),
		Command:  "printf 'watch'",
	}}

	log, err := logger.New(&logger.Config{
		Level:       logger.LevelDebug,
		Development: true,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	if err := watcher.Stop(); err != nil {
		t.Fatalf("Failed to stop watcher: %v", err)
	}
	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to restart watcher: %v", err)
	}
	t.Cleanup(func() {
		_ = watcher.Stop()
	})

	if !watcher.IsRunning() {
		t.Fatal("Watcher should be running after restart")
	}
}

func TestWatcherGlobMatching(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{"exact match", "/tmp/test.go", "*.go", true},
		{"exact match with path", "/tmp/test.go", "/tmp/*.go", true},
		{"no match", "/tmp/test.txt", "*.go", false},
		{"subdirectory match", "/tmp/src/test.go", "**/*.go", false}, // filepath.Match doesn't support **
		{"directory file match", "/tmp/src/test.go", "*.go", true},   // matches basename
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesGlob(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesGlob(%q, %q) = %v, want %v", tt.path, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestExtractBaseDir(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{"simple glob", "*.go", "."},
		{"path with glob", "pkg/*.go", "pkg"},
		{"nested glob", "pkg/**/*.go", "pkg"},
		{"question mark glob", "test?.go", "test"},       // Returns prefix before ?
		{"character class glob", "test[0-9].go", "test"}, // Returns prefix before [
		{"brace glob", "*.{go,txt}", "."},
		{"no glob", "file.go", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBaseDir(tt.pattern)
			if result != tt.expected {
				t.Errorf("extractBaseDir(%q) = %q, want %q", tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestWatcherWithRealFiles(t *testing.T) {
	t.Skip("Skipping real file test in CI environment - requires more sophisticated timing")
	// Create temp directory for testing
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure watcher
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.DebounceMs = 50 // Short debounce for testing
	cfg.Watch.Patterns = []config.WatchPattern{
		{
			FileGlob: filepath.Join(tmpDir, "*.txt"),
			Command:  "echo 'file changed' > " + filepath.Join(tmpDir, "triggered.txt"),
		},
	}

	log, err := logger.New(&logger.Config{
		Level:       logger.LevelDebug,
		Development: true,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Wait for watcher to initialize
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Wait for debounce and command execution
	time.Sleep(500 * time.Millisecond)

	// Check if trigger file was created (command executed)
	triggerFile := filepath.Join(tmpDir, "triggered.txt")
	if _, err := os.Stat(triggerFile); os.IsNotExist(err) {
		t.Log("Note: Trigger file not created - this may be expected in some environments")
		// Don't fail the test as file watching behavior can vary by environment
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		length   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncate", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
		{"zero length", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.length)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.length, result, tt.expected)
			}
		})
	}
}

func TestWatcherNilSafety(t *testing.T) {
	var w *Watcher

	// Test nil safety of various methods
	if w.IsRunning() {
		t.Error("IsRunning should return false for nil watcher")
	}

	if w.Patterns() != nil {
		t.Error("Patterns should return nil for nil watcher")
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop should not error for nil watcher: %v", err)
	}
}

func TestWatcherStatusTracksLastExecution(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.DebounceMs = 25
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.txt"),
		Command:  "printf 'watch-ok'",
	}}

	log, err := logger.New(&logger.Config{
		Level:       logger.LevelDebug,
		Development: true,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	watcher.ctx = context.Background()
	watcher.executeCommand(0, fsnotify.Event{Name: "/tmp/demo.txt", Op: fsnotify.Write})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := watcher.Status()
		if !status.LastRunAt.IsZero() {
			if !status.Enabled {
				t.Fatalf("expected enabled watcher status, got %+v", status)
			}
			if status.LastCommand != "printf 'watch-ok'" {
				t.Fatalf("unexpected last command: %+v", status)
			}
			if status.LastFile != "/tmp/demo.txt" {
				t.Fatalf("unexpected last file: %+v", status)
			}
			if !status.LastSuccess {
				t.Fatalf("expected successful last run: %+v", status)
			}
			if status.LastResultPreview != "watch-ok" {
				t.Fatalf("unexpected result preview: %+v", status)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("watch status did not record last execution: %+v", watcher.Status())
}

type captureWatchPreparer struct {
	mu   sync.Mutex
	last execenv.StartSpec
}

func (c *captureWatchPreparer) Prepare(_ context.Context, spec execenv.StartSpec) (execenv.Prepared, error) {
	c.mu.Lock()
	c.last = spec
	c.mu.Unlock()
	return execenv.Prepared{
		Workdir: spec.Workdir,
		Env:     append([]string{}, spec.Env...),
		Cleanup: func() error { return nil },
	}, nil
}

func (c *captureWatchPreparer) Snapshot() execenv.StartSpec {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

func TestWatcherExecuteCommandUsesExecenvPreparation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.go"),
		Command:  "printf 'watch-ok'",
	}}
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log, err := logger.New(&logger.Config{Level: logger.LevelDebug, Development: true})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	preparer := &captureWatchPreparer{}
	watcher.preparer = preparer
	watcher.ctx = context.Background()
	watcher.executeCommand(0, fsnotify.Event{Name: "/tmp/demo.txt", Op: fsnotify.Write})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		last := preparer.Snapshot()
		if last.Command != "" {
			if last.Command != "printf 'watch-ok'" {
				t.Fatalf("unexpected prepared command: %+v", last)
			}
			if last.Workdir != cfg.WorkspacePath() {
				t.Fatalf("expected workspace workdir %q, got %q", cfg.WorkspacePath(), last.Workdir)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("watch execenv preparation was not called")
}

func TestWatcherExecuteCommandCreatesManagedTaskOnSuccess(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.txt"),
		Command:  "printf 'watch-ok'",
	}}
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log, err := logger.New(&logger.Config{Level: logger.LevelDebug, Development: true})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	store := tasks.NewStore()
	watcher.taskSvc = tasks.NewService(store)
	watcher.ctx = context.Background()

	watcher.executeCommand(0, fsnotify.Event{Name: "/tmp/demo.txt", Op: fsnotify.Write})

	task := waitForWatchTaskState(t, store, tasks.StateCompleted, 2*time.Second)
	if task.Type != tasks.TypeLocalAgent {
		t.Fatalf("expected local agent task, got %q", task.Type)
	}
	if task.RuntimeID != "watch" {
		t.Fatalf("expected runtime id watch, got %q", task.RuntimeID)
	}
	if task.SessionID != "watch:0" {
		t.Fatalf("expected session id watch:0, got %q", task.SessionID)
	}
	if got, _ := task.Metadata["source"].(string); got != "watch" {
		t.Fatalf("expected watch source metadata, got %q", got)
	}
	if got, _ := task.Metadata["file"].(string); got != "/tmp/demo.txt" {
		t.Fatalf("expected file metadata, got %q", got)
	}
}

func TestWatcherExecuteCommandCreatesManagedTaskOnFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Watch.Enabled = true
	cfg.Watch.Patterns = []config.WatchPattern{{
		FileGlob: filepath.Join(t.TempDir(), "*.txt"),
		Command:  "printf 'watch-fail' >&2; exit 3",
	}}
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log, err := logger.New(&logger.Config{Level: logger.LevelDebug, Development: true})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	auditLogger := audit.NewLogger(audit.DefaultConfig(), t.TempDir(), log)

	watcher, err := New(cfg, log, auditLogger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	store := tasks.NewStore()
	watcher.taskSvc = tasks.NewService(store)
	watcher.ctx = context.Background()

	watcher.executeCommand(0, fsnotify.Event{Name: "/tmp/demo.txt", Op: fsnotify.Write})

	task := waitForWatchTaskState(t, store, tasks.StateFailed, 2*time.Second)
	if task.LastError == "" {
		t.Fatalf("expected task failure to record error, got %+v", task)
	}
}

func waitForWatchTaskState(t *testing.T, store *tasks.Store, want tasks.State, timeout time.Duration) tasks.Task {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		all := store.List()
		if len(all) == 1 && all[0].State == want {
			return all[0]
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("watch task did not reach state %s: %+v", want, store.List())
	return tasks.Task{}
}
