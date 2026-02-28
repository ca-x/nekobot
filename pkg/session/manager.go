// Package session manages conversation history and sessions.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nekobot/pkg/agent"
	"nekobot/pkg/fileutil"
)

// Session represents a conversation session with history.
type Session struct {
	ID        string          `json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []agent.Message `json:"messages"`
	Summary   string          `json:"summary,omitempty"`
	mu        sync.RWMutex
}

// Manager manages multiple sessions with persistent storage.
type Manager struct {
	baseDir  string
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new session manager.
func NewManager(baseDir string) *Manager {
	os.MkdirAll(baseDir, 0755)
	return &Manager{
		baseDir:  baseDir,
		sessions: make(map[string]*Session),
	}
}

// Get retrieves or creates a session by ID.
func (m *Manager) Get(sessionID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already in memory
	if session, exists := m.sessions[sessionID]; exists {
		return session, nil
	}

	// Try to load from disk
	session, err := m.load(sessionID)
	if err != nil {
		// Create new session if not found
		session = &Session{
			ID:        sessionID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages:  []agent.Message{},
		}
	}

	m.sessions[sessionID] = session
	return session, nil
}

// GetExisting retrieves an existing session by ID without creating a new one.
func (m *Manager) GetExisting(sessionID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		return session, nil
	}

	session, err := m.load(sessionID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("loading session %q: %w", sessionID, err)
	}

	m.sessions[sessionID] = session
	return session, nil
}

// Save persists a session to disk.
func (m *Manager) Save(session *Session) error {
	session.mu.Lock()
	session.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(session, "", "  ")
	session.mu.Unlock()

	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	path := m.getSessionPath(session.ID)
	if err := fileutil.WriteFileAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	return nil
}

// load loads a session from disk.
func (m *Manager) load(sessionID string) (*Session, error) {
	path := m.getSessionPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshaling session: %w", err)
	}

	return &session, nil
}

// getSessionPath returns the file path for a session.
func (m *Manager) getSessionPath(sessionID string) string {
	return filepath.Join(m.baseDir, sessionID+".json")
}

// List returns all session IDs.
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return nil, err
	}

	var sessionIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessionID := entry.Name()[:len(entry.Name())-5] // Remove .json
			sessionIDs = append(sessionIDs, sessionID)
		}
	}

	return sessionIDs, nil
}

// Delete removes a session.
func (m *Manager) Delete(sessionID string) error {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	path := m.getSessionPath(sessionID)
	return os.Remove(path)
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(message agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, message)
	s.UpdatedAt = time.Now()
}

// GetMessages returns a copy of all messages.
func (s *Session) GetMessages() []agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]agent.Message, len(s.Messages))
	copy(messages, s.Messages)
	return messages
}

// Clear clears all messages in the session.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = []agent.Message{}
	s.UpdatedAt = time.Now()
}

// SetSummary sets the summary for this session.
func (s *Session) SetSummary(summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Summary = summary
	s.UpdatedAt = time.Now()
}

// GetSummary returns the session summary.
func (s *Session) GetSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Summary
}

// GetID returns the session ID.
func (s *Session) GetID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ID
}

// GetCreatedAt returns the session creation time.
func (s *Session) GetCreatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.CreatedAt
}

// GetUpdatedAt returns the session last update time.
func (s *Session) GetUpdatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.UpdatedAt
}
