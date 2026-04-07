// Package process provides PTY session management for background processes.
package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"go.uber.org/zap"

	"nekobot/pkg/execenv"
	"nekobot/pkg/logger"
	"nekobot/pkg/tasks"
)

// Session represents a PTY session.
type Session struct {
	ID          string
	Command     string
	Workdir     string
	StartedAt   time.Time
	ExitedAt    time.Time
	Running     bool
	ExitCode    int
	PTY         *os.File
	Process     *os.Process
	Output      []string
	OutputMutex sync.RWMutex
	MaxOutput   int // Maximum output lines to keep
	Cleanup     func() error
	cleanupOnce sync.Once
	TaskID      string
	RuntimeID   string
	taskDone    sync.Once

	cancelMu        sync.RWMutex
	cancelRequested bool
}

// Observation captures lightweight read-only runtime state inferred from recent PTY output.
type Observation struct {
	State   string `json:"state,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// AutoResponseConfig configures automatic responses to interactive prompts.
type AutoResponseConfig struct {
	Enabled          bool              `json:"enabled"`
	DefaultConfirm   string            `json:"default_confirm"`
	DefaultDeny      string            `json:"default_deny"`
	MenuSelections   map[string]string `json:"menu_selections"`
	AutoConfirmPatterns []string       `json:"auto_confirm_patterns"`
	AutoDenyPatterns    []string       `json:"auto_deny_patterns"`
}

// DefaultAutoResponseConfig returns sensible defaults for auto-response.
func DefaultAutoResponseConfig() AutoResponseConfig {
	return AutoResponseConfig{
		Enabled:        false,
		DefaultConfirm: "y",
		DefaultDeny:    "n",
		MenuSelections: make(map[string]string),
		AutoConfirmPatterns: []string{
			"continue? [y/n]",
			"proceed? [y/n]",
			"is this ok",
		},
		AutoDenyPatterns: []string{
			"abort?",
			"cancel?",
		},
	}
}

const (
	defaultPTYRows = 40
	defaultPTYCols = 120
)

var killProcess = func(proc *os.Process) error {
	return proc.Kill()
}

// Manager manages PTY sessions.
type Manager struct {
	log      *logger.Logger
	sessions map[string]*Session
	mu       sync.RWMutex
	preparer execenv.Preparer
	taskSvc  taskLifecycle
}

type taskLifecycle interface {
	Enqueue(task tasks.Task) (tasks.Task, error)
	Claim(taskID, runtimeID string) (tasks.Task, error)
	Start(taskID string) (tasks.Task, error)
	Complete(taskID string) (tasks.Task, error)
	Fail(taskID, lastError string) (tasks.Task, error)
	Cancel(taskID string) (tasks.Task, error)
}

// NewManager creates a new process manager.
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		log:      log,
		sessions: make(map[string]*Session),
		preparer: execenv.NewDefaultPreparer(),
	}
}

// SetPreparer overrides the execution environment preparer.
func (m *Manager) SetPreparer(preparer execenv.Preparer) {
	if m == nil || preparer == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preparer = preparer
}

// SetTaskService attaches a shared task lifecycle service for task-aware process sessions.
func (m *Manager) SetTaskService(svc taskLifecycle) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskSvc = svc
}

// Start starts a new PTY session with the default start spec.
func (m *Manager) Start(ctx context.Context, sessionID, command, workdir string) error {
	return m.StartWithSpec(ctx, execenv.StartSpec{
		SessionID: sessionID,
		Command:   command,
		Workdir:   workdir,
		Env:       os.Environ(),
	})
}

// StartWithSpec starts a new PTY session using an execution-environment preparation contract.
func (m *Manager) StartWithSpec(ctx context.Context, spec execenv.StartSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[spec.SessionID]; exists {
		return fmt.Errorf("session already exists: %s", spec.SessionID)
	}
	if strings.TrimSpace(spec.Command) == "" {
		return fmt.Errorf("command is required")
	}
	if m.preparer == nil {
		m.preparer = execenv.NewDefaultPreparer()
	}
	preparer := m.preparer
	taskSvc := m.taskSvc

	if err := enqueueManagedTask(taskSvc, spec); err != nil {
		return fmt.Errorf("enqueue managed task: %w", err)
	}

	prepared, err := preparer.Prepare(ctx, spec)
	if err != nil {
		failManagedTask(taskSvc, spec, fmt.Errorf("prepare execenv: %w", err), m.log)
		return fmt.Errorf("prepare execenv: %w", err)
	}

	shellPath := resolveShellPath()
	cmd := exec.CommandContext(ctx, shellPath, "-c", spec.Command)
	cmd.Env = append([]string{}, prepared.Env...)
	if prepared.Workdir != "" {
		cmd.Dir = prepared.Workdir
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: defaultPTYRows,
		Cols: defaultPTYCols,
	})
	if err != nil {
		failManagedTask(taskSvc, spec, fmt.Errorf("starting PTY: %w", err), m.log)
		runCleanup(prepared.Cleanup, m.log, spec.SessionID)
		return fmt.Errorf("starting PTY: %w", err)
	}
	if err := startManagedTask(taskSvc, spec); err != nil {
		_ = cmd.Process.Kill()
		_ = ptmx.Close()
		failManagedTask(taskSvc, spec, fmt.Errorf("start managed task: %w", err), m.log)
		runCleanup(prepared.Cleanup, m.log, spec.SessionID)
		return fmt.Errorf("start managed task: %w", err)
	}

	session := &Session{
		ID:        spec.SessionID,
		Command:   spec.Command,
		Workdir:   prepared.Workdir,
		StartedAt: time.Now(),
		Running:   true,
		PTY:       ptmx,
		Process:   cmd.Process,
		Output:    make([]string, 0),
		MaxOutput: 10000,
		Cleanup:   prepared.Cleanup,
		TaskID:    strings.TrimSpace(spec.TaskID),
		RuntimeID: strings.TrimSpace(spec.RuntimeID),
	}

	m.sessions[spec.SessionID] = session

	go m.captureOutput(session)
	go m.waitForExit(session, cmd)

	m.log.Info("PTY session started",
		zap.String("session_id", spec.SessionID),
		zap.String("command", spec.Command),
		zap.String("shell", shellPath),
		zap.Int("pid", cmd.Process.Pid))

	return nil
}

// Reset removes a session from manager and kills its process if still running.
func (m *Manager) Reset(sessionID string) error {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	session.OutputMutex.RLock()
	running := session.Running
	session.OutputMutex.RUnlock()
	if running && session.Process != nil {
		session.markCancelRequested()
		_ = killProcess(session.Process)
	}
	if session.PTY != nil {
		_ = session.PTY.Close()
	}
	session.cleanupOnce.Do(func() {
		runCleanup(session.Cleanup, m.log, sessionID)
	})
	return nil
}

// captureOutput captures raw output chunks from PTY.
func (m *Manager) captureOutput(session *Session) {
	buf := make([]byte, 4096)
	for {
		n, err := session.PTY.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			session.OutputMutex.Lock()
			session.Output = append(session.Output, chunk)

			// Trim if exceeds max
			if len(session.Output) > session.MaxOutput {
				session.Output = session.Output[len(session.Output)-session.MaxOutput:]
			}
			session.OutputMutex.Unlock()
		}
		if err != nil {
			if err != io.EOF {
				m.log.Debug("PTY output read stopped",
					zap.String("session_id", session.ID),
					zap.Error(err))
			}
			return
		}
	}
}

// waitForExit waits for the process to exit.
func (m *Manager) waitForExit(session *Session, cmd *exec.Cmd) {
	err := cmd.Wait()

	session.OutputMutex.Lock()
	session.Running = false
	session.ExitedAt = time.Now()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			session.ExitCode = exitErr.ExitCode()
		} else {
			session.ExitCode = -1
		}
	} else {
		session.ExitCode = 0
	}
	session.OutputMutex.Unlock()

	// Close PTY
	_ = session.PTY.Close()
	session.cleanupOnce.Do(func() {
		runCleanup(session.Cleanup, m.log, session.ID)
	})
	switch {
	case session.cancelRequestedState():
		cancelManagedTask(m.taskSvc, session, m.log)
	case session.ExitCode == 0:
		completeManagedTask(m.taskSvc, session, m.log)
	default:
		failManagedTask(m.taskSvc, execenv.StartSpec{
			SessionID: session.ID,
			Command:   session.Command,
			Workdir:   session.Workdir,
			RuntimeID: session.RuntimeID,
			TaskID:    session.TaskID,
		}, fmt.Errorf("process exited with code %d", session.ExitCode), m.log)
	}

	m.log.Info("PTY session exited",
		zap.String("session_id", session.ID),
		zap.Int("exit_code", session.ExitCode),
		zap.Duration("duration", time.Since(session.StartedAt)))
}

// GetOutput returns output lines from a session.
func (m *Manager) GetOutput(sessionID string, offset, limit int) ([]string, int, error) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return nil, 0, fmt.Errorf("session not found: %s", sessionID)
	}

	session.OutputMutex.RLock()
	defer session.OutputMutex.RUnlock()

	total := len(session.Output)

	// Handle offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []string{}, total, nil
	}

	// Handle limit
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}

	lines := make([]string, end-offset)
	copy(lines, session.Output[offset:end])

	return lines, total, nil
}

// Write sends data to session stdin.
func (m *Manager) Write(sessionID, data string) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if !session.Running {
		return fmt.Errorf("session not running: %s", sessionID)
	}

	_, err := io.WriteString(session.PTY, data)
	return err
}

// AutoRespond analyzes session output and sends automatic responses if configured.
func (m *Manager) AutoRespond(sessionID string, cfg AutoResponseConfig) (bool, string, error) {
	if !cfg.Enabled {
		return false, "", nil
	}

	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return false, "", fmt.Errorf("session not found: %s", sessionID)
	}

	session.OutputMutex.RLock()
	obs := classifyObservation(session.Output)
	session.OutputMutex.RUnlock()

	switch obs.State {
	case "awaiting_input":
		return m.handleAwaitingInput(session, obs.Summary, cfg)
	case "menu_prompt":
		return m.handleMenuPrompt(session, obs.Summary, cfg)
	default:
		return false, "", nil
	}
}

func (m *Manager) handleAwaitingInput(session *Session, summary string, cfg AutoResponseConfig) (bool, string, error) {
	lower := strings.ToLower(summary)

	// Check auto-confirm patterns
	for _, pattern := range cfg.AutoConfirmPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			if err := m.sendResponse(session, cfg.DefaultConfirm); err != nil {
				return false, "", err
			}
			return true, cfg.DefaultConfirm, nil
		}
}

	// Check auto-deny patterns
	for _, pattern := range cfg.AutoDenyPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			if err := m.sendResponse(session, cfg.DefaultDeny); err != nil {
				return false, "", err
			}
			return true, cfg.DefaultDeny, nil
		}
	}

	return false, "", nil
}

func (m *Manager) handleMenuPrompt(session *Session, summary string, cfg AutoResponseConfig) (bool, string, error) {
	// Try to find a configured selection for this menu
	for menuKey, selection := range cfg.MenuSelections {
		if strings.Contains(strings.ToLower(summary), strings.ToLower(menuKey)) {
			if err := m.sendResponse(session, selection); err != nil {
				return false, "", err
			}
			return true, selection, nil
		}
	}
	return false, "", nil
}

func (m *Manager) sendResponse(session *Session, response string) error {
	if !session.Running || session.PTY == nil {
		return fmt.Errorf("session not ready for response")
	}
	// Add newline if not present
	if !strings.HasSuffix(response, "\n") {
		response += "\n"
	}
	_, err := io.WriteString(session.PTY, response)
	return err
}

// Resize updates PTY window size for a running session.
func (m *Manager) Resize(sessionID string, cols, rows int) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("invalid resize values: cols=%d rows=%d", cols, rows)
	}
	if err := pty.Setsize(session.PTY, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	}); err != nil {
		return fmt.Errorf("resize PTY: %w", err)
	}
	return nil
}

// Kill terminates a session.
func (m *Manager) Kill(sessionID string) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if !session.Running {
		return fmt.Errorf("session not running: %s", sessionID)
	}

	session.markCancelRequested()
	if err := killProcess(session.Process); err != nil {
		return fmt.Errorf("killing process: %w", err)
	}

	m.log.Info("PTY session killed", zap.String("session_id", sessionID))
	return nil
}

// GetStatus returns session status.
func (m *Manager) GetStatus(sessionID string) (*SessionStatus, error) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.OutputMutex.RLock()
	defer session.OutputMutex.RUnlock()

	status := &SessionStatus{
		ID:          session.ID,
		Command:     session.Command,
		Workdir:     session.Workdir,
		StartedAt:   session.StartedAt,
		ExitedAt:    session.ExitedAt,
		Running:     session.Running,
		ExitCode:    session.ExitCode,
		OutputSize:  len(session.Output),
		Observation: classifyObservation(session.Output),
	}

	if session.Running {
		status.Duration = time.Since(session.StartedAt)
	} else {
		status.Duration = session.ExitedAt.Sub(session.StartedAt)
	}

	return status, nil
}

// List returns all sessions.
func (m *Manager) List() []*SessionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]*SessionStatus, 0, len(m.sessions))
	for _, session := range m.sessions {
		session.OutputMutex.RLock()

		status := &SessionStatus{
			ID:          session.ID,
			Command:     session.Command,
			Workdir:     session.Workdir,
			StartedAt:   session.StartedAt,
			ExitedAt:    session.ExitedAt,
			Running:     session.Running,
			ExitCode:    session.ExitCode,
			OutputSize:  len(session.Output),
			Observation: classifyObservation(session.Output),
		}

		if session.Running {
			status.Duration = time.Since(session.StartedAt)
		} else {
			status.Duration = session.ExitedAt.Sub(session.StartedAt)
		}

		session.OutputMutex.RUnlock()
		statuses = append(statuses, status)
	}

	return statuses
}

// Cleanup removes finished sessions older than the specified duration.
func (m *Manager) Cleanup(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()

	for id, session := range m.sessions {
		if !session.Running && now.Sub(session.ExitedAt) > maxAge {
			delete(m.sessions, id)
			count++
			m.log.Debug("Cleaned up session", zap.String("session_id", id))
		}
	}

	return count
}

func runCleanup(cleanup func() error, log *logger.Logger, sessionID string) {
	if cleanup == nil {
		return
	}
	if err := cleanup(); err != nil && log != nil {
		log.Warn("Process session cleanup failed", zap.String("session_id", sessionID), zap.Error(err))
	}
}

func enqueueManagedTask(svc taskLifecycle, spec execenv.StartSpec) error {
	if svc == nil || strings.TrimSpace(spec.TaskID) == "" {
		return nil
	}
	runtimeID := strings.TrimSpace(spec.RuntimeID)
	if runtimeID == "" {
		runtimeID = "process"
	}
	_, err := svc.Enqueue(tasks.Task{
		ID:        strings.TrimSpace(spec.TaskID),
		Type:      tasks.TypeRuntimeWorker,
		Summary:   strings.TrimSpace(spec.Command),
		SessionID: strings.TrimSpace(spec.SessionID),
		RuntimeID: runtimeID,
		Metadata: map[string]any{
			"source":  "process",
			"workdir": strings.TrimSpace(spec.Workdir),
		},
	})
	return err
}

func startManagedTask(svc taskLifecycle, spec execenv.StartSpec) error {
	if svc == nil || strings.TrimSpace(spec.TaskID) == "" {
		return nil
	}
	runtimeID := strings.TrimSpace(spec.RuntimeID)
	if runtimeID == "" {
		runtimeID = "process"
	}
	if _, err := svc.Claim(strings.TrimSpace(spec.TaskID), runtimeID); err != nil {
		return err
	}
	_, err := svc.Start(strings.TrimSpace(spec.TaskID))
	return err
}

func completeManagedTask(svc taskLifecycle, session *Session, log *logger.Logger) {
	if svc == nil || session == nil || strings.TrimSpace(session.TaskID) == "" {
		return
	}
	session.taskDone.Do(func() {
		if _, err := svc.Complete(strings.TrimSpace(session.TaskID)); err != nil && log != nil {
			log.Warn("Completing managed process task failed", zap.String("task_id", session.TaskID), zap.Error(err))
		}
	})
}

func failManagedTask(svc taskLifecycle, spec execenv.StartSpec, cause error, log *logger.Logger) {
	if svc == nil || strings.TrimSpace(spec.TaskID) == "" || cause == nil {
		return
	}
	if _, err := svc.Fail(strings.TrimSpace(spec.TaskID), cause.Error()); err != nil && log != nil {
		log.Warn("Failing managed process task failed", zap.String("task_id", spec.TaskID), zap.Error(err))
	}
}

func cancelManagedTask(svc taskLifecycle, session *Session, log *logger.Logger) {
	if svc == nil || session == nil || strings.TrimSpace(session.TaskID) == "" {
		return
	}
	session.taskDone.Do(func() {
		if _, err := svc.Cancel(strings.TrimSpace(session.TaskID)); err != nil && log != nil {
			log.Warn("Canceling managed process task failed", zap.String("task_id", session.TaskID), zap.Error(err))
		}
	})
}

func (s *Session) markCancelRequested() {
	if s == nil {
		return
	}
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancelRequested = true
}

func (s *Session) cancelRequestedState() bool {
	if s == nil {
		return false
	}
	s.cancelMu.RLock()
	defer s.cancelMu.RUnlock()
	return s.cancelRequested
}

func resolveShellPath() string {
	candidates := []string{
		"/bin/sh",
		"/usr/bin/sh",
		"/bin/bash",
		"/usr/bin/bash",
		"/usr/local/bin/bash",
		"/bin/zsh",
		"/usr/bin/zsh",
		"/usr/local/bin/zsh",
		"/bin/ash",
		"/usr/bin/ash",
		"/system/bin/sh",
		"/usr/bin/fish",
		"/bin/fish",
		"/usr/local/bin/fish",
	}
	for _, path := range candidates {
		if !isExecutableFile(path) {
			continue
		}
		return path
	}
	lookupNames := []string{"sh", "bash", "zsh", "ash", "fish"}
	for _, name := range lookupNames {
		lookedUp, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if isExecutableFile(lookedUp) {
			return lookedUp
		}
	}
	return "sh"
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func expandTildePath(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}

	sepIdx := strings.IndexRune(path, '/')
	prefix := path
	suffix := ""
	if sepIdx >= 0 {
		prefix = path[:sepIdx]
		suffix = path[sepIdx:]
	}

	if prefix == "~" {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return path
		}
		return home + suffix
	}

	userName := strings.TrimPrefix(prefix, "~")
	if userName == "" {
		return path
	}
	u, err := user.Lookup(userName)
	if err != nil || strings.TrimSpace(u.HomeDir) == "" {
		return path
	}
	return u.HomeDir + suffix
}

// SessionStatus represents session status information.
type SessionStatus struct {
	ID          string        `json:"id"`
	Command     string        `json:"command"`
	Workdir     string        `json:"workdir"`
	StartedAt   time.Time     `json:"started_at"`
	ExitedAt    time.Time     `json:"exited_at,omitempty"`
	Running     bool          `json:"running"`
	ExitCode    int           `json:"exit_code"`
	Duration    time.Duration `json:"duration"`
	OutputSize  int           `json:"output_size"`
	Observation Observation   `json:"observation,omitempty"`
}

func classifyObservation(chunks []string) Observation {
	if len(chunks) == 0 {
		return Observation{}
	}

	text := strings.TrimSpace(strings.Join(chunks, ""))
	if text == "" {
		return Observation{}
	}

	lines := strings.Split(text, "\n")
	summary := strings.TrimSpace(lines[len(lines)-1])
	if summary == "" && len(lines) > 1 {
		summary = strings.TrimSpace(lines[len(lines)-2])
	}

	lower := strings.ToLower(text)
	switch {
	case looksLikeMenuPrompt(lower):
		return Observation{State: "menu_prompt", Summary: summary}
	case looksLikeAwaitingInput(lower):
		return Observation{State: "awaiting_input", Summary: summary}
	case looksLikeErrorPrompt(lower):
		return Observation{State: "error_prompt", Summary: summary}
	default:
		return Observation{State: "idle", Summary: summary}
	}
}

func looksLikeAwaitingInput(input string) bool {
	lower := strings.ToLower(input)
	patterns := []string{
		"[y/n]",
		"[y/n]?",
		"[y/n]:",
		"[y/n] ",
		"[y/n]\n",
		"[y/N]",
		"[y/N]?",
		"[Y/n]",
		"[Y/n]?",
		"[y/N]:",
		"continue? [y/n]",
		"continue? [y/n",
		"continue? [y/N]",
		"continue? [y/N",
		"press enter",
		"enter to continue",
		"press any key",
		"hit any key",
		"any key to",
		"press [enter]",
		"(y/n)",
		"(y/n)?",
		"yes/no",
		"yes/no?",
		"[yes/no]",
		"confirm",
		"confirm?",
		"confirmation",
		"abort?",
		"retry?",
		"retry",
		"overwrite?",
		"overwrite",
		"delete?",
		"delete",
		"remove?",
		"remove",
		"continue?",
		"proceed?",
		"is this ok",
		"is this okay",
		"do you want to",
		"would you like to",
		"are you sure",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func looksLikeMenuPrompt(input string) bool {
	lower := strings.ToLower(input)
	return (strings.Contains(lower, "1.") && strings.Contains(lower, "2.")) ||
		(strings.Contains(lower, "1)") && strings.Contains(lower, "2)")) ||
		strings.Contains(lower, "select option") ||
		strings.Contains(lower, "choose an option") ||
		strings.Contains(lower, "reply /select") ||
		strings.Contains(lower, "select:") ||
		strings.Contains(lower, "choose:") ||
		strings.Contains(lower, "options:") ||
		strings.Contains(lower, "menu:") ||
		strings.Contains(lower, "pick an option") ||
		strings.Contains(lower, "enter your choice") ||
		strings.Contains(lower, "select one") ||
		strings.Contains(lower, "which option") ||
		strings.Contains(lower, "[1]") ||
		strings.Contains(lower, "[2]") ||
		strings.Contains(lower, "[3]")
}

func looksLikeErrorPrompt(input string) bool {
	lower := strings.ToLower(input)
	return strings.Contains(lower, "error:") ||
		strings.Contains(lower, "failed:") ||
		strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "fatal:") ||
		strings.Contains(lower, "panic:") ||
		strings.Contains(lower, "exception:") ||
		strings.Contains(lower, "cannot") ||
		strings.Contains(lower, "unable to") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "is required") ||
		strings.Contains(lower, "invalid") ||
		strings.Contains(lower, "unrecognized") ||
		strings.Contains(lower, "unknown command") ||
		strings.Contains(lower, "syntax error") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "timeout")
}
