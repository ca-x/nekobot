// Package process provides PTY session management for background processes.
package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"go.uber.org/zap"

	"nekobot/pkg/logger"
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
}

// Manager manages PTY sessions.
type Manager struct {
	log      *logger.Logger
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new process manager.
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		log:      log,
		sessions: make(map[string]*Session),
	}
}

// Start starts a new PTY session.
func (m *Manager) Start(ctx context.Context, sessionID, command, workdir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already exists
	if _, exists := m.sessions[sessionID]; exists {
		return fmt.Errorf("session already exists: %s", sessionID)
	}

	// Create command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}

	// Start with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("starting PTY: %w", err)
	}

	// Create session
	session := &Session{
		ID:        sessionID,
		Command:   command,
		Workdir:   workdir,
		StartedAt: time.Now(),
		Running:   true,
		PTY:       ptmx,
		Process:   cmd.Process,
		Output:    make([]string, 0),
		MaxOutput: 10000, // Keep last 10k lines
	}

	m.sessions[sessionID] = session

	// Start output capture goroutine
	go m.captureOutput(session)

	// Start wait goroutine
	go m.waitForExit(session, cmd)

	m.log.Info("PTY session started",
		zap.String("session_id", sessionID),
		zap.String("command", command),
		zap.Int("pid", cmd.Process.Pid))

	return nil
}

// captureOutput captures output from PTY.
func (m *Manager) captureOutput(session *Session) {
	scanner := bufio.NewScanner(session.PTY)
	for scanner.Scan() {
		line := scanner.Text()

		session.OutputMutex.Lock()
		session.Output = append(session.Output, line)

		// Trim if exceeds max
		if len(session.Output) > session.MaxOutput {
			session.Output = session.Output[len(session.Output)-session.MaxOutput:]
		}
		session.OutputMutex.Unlock()
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
	session.PTY.Close()

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

	if err := session.Process.Kill(); err != nil {
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
		ID:         session.ID,
		Command:    session.Command,
		Workdir:    session.Workdir,
		StartedAt:  session.StartedAt,
		ExitedAt:   session.ExitedAt,
		Running:    session.Running,
		ExitCode:   session.ExitCode,
		OutputSize: len(session.Output),
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
			ID:         session.ID,
			Command:    session.Command,
			Workdir:    session.Workdir,
			StartedAt:  session.StartedAt,
			ExitedAt:   session.ExitedAt,
			Running:    session.Running,
			ExitCode:   session.ExitCode,
			OutputSize: len(session.Output),
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

// SessionStatus represents session status information.
type SessionStatus struct {
	ID         string        `json:"id"`
	Command    string        `json:"command"`
	Workdir    string        `json:"workdir"`
	StartedAt  time.Time     `json:"started_at"`
	ExitedAt   time.Time     `json:"exited_at,omitempty"`
	Running    bool          `json:"running"`
	ExitCode   int           `json:"exit_code"`
	Duration   time.Duration `json:"duration"`
	OutputSize int           `json:"output_size"`
}
