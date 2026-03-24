// Package session manages conversation history and sessions.
package session

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/agent"
)

// Session represents a conversation session with history.
type Session struct {
	ID        string          `json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []agent.Message `json:"messages"`
	Summary   string          `json:"summary,omitempty"`
	Source    string          `json:"source,omitempty"`
	mu        sync.RWMutex
	manager   *Manager
}

const (
	SourceCLI       = "cli"
	SourceTUI       = "tui"
	SourceWebUI     = "webui"
	SourceHeartbeat = "heartbeat"
	SourceCron      = "cron"
	SourceChannels  = "channels"
	SourceGateway   = "gateway"
)

// Manager manages multiple sessions with persistent storage.
type Manager struct {
	baseDir  string
	config   config.SessionsConfig
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new session manager.
func NewManager(baseDir string, cfg config.SessionsConfig) *Manager {
	os.MkdirAll(baseDir, 0755)
	return &Manager{
		baseDir:  baseDir,
		config:   cfg,
		sessions: make(map[string]*Session),
	}
}

// Get retrieves or creates a session by ID.
func (m *Manager) Get(sessionID string) (*Session, error) {
	return m.GetWithSource(sessionID, "")
}

// GetWithSource retrieves or creates a session by ID with an explicit source hint.
func (m *Manager) GetWithSource(sessionID, source string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already in memory
	if session, exists := m.sessions[sessionID]; exists {
		if stringsTrimmed(source) != "" && stringsTrimmed(session.Source) == "" {
			session.Source = stringsTrimmed(source)
		}
		return session, nil
	}

	// Try to load from disk
	session, err := m.load(sessionID)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("loading session %q: %w", sessionID, err)
		}

		// Create new session if not found.
		session = &Session{
			ID:        sessionID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages:  []agent.Message{},
			Source:    stringsTrimmed(source),
		}
	}
	session.manager = m
	if stringsTrimmed(source) != "" && stringsTrimmed(session.Source) == "" {
		session.Source = stringsTrimmed(source)
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

	session.manager = m
	m.sessions[sessionID] = session
	return session, nil
}

// Save persists a session to disk.
func (m *Manager) Save(session *Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	session.mu.Lock()
	session.UpdatedAt = time.Now()
	snapshot := session.snapshotLocked()
	session.mu.Unlock()

	if !m.shouldPersist(snapshot.Source) {
		return nil
	}
	filteredMessages := m.filterMessages(snapshot.Messages, snapshot.Source)

	if err := m.SaveJSONL(snapshot.ID, sessionJSONMessagesFromAgent(filteredMessages), map[string]interface{}{
		"summary": snapshot.Summary,
		"source":  snapshot.Source,
	}); err != nil {
		return fmt.Errorf("writing session jsonl: %w", err)
	}

	return nil
}

// load loads a session from disk.
func (m *Manager) load(sessionID string) (*Session, error) {
	jsonlSession, err := m.LoadJSONL(sessionID)
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        sessionID,
		CreatedAt: jsonlSession.CreatedAt,
		UpdatedAt: jsonlSession.UpdatedAt,
		Messages:  sessionAgentMessagesFromJSON(jsonlSession.Messages),
		manager:   m,
	}
	if summary, ok := jsonlSession.Metadata["summary"].(string); ok {
		session.Summary = summary
	}
	if source, ok := jsonlSession.Metadata["source"].(string); ok {
		session.Source = source
	}
	return session, nil
}

// List returns all session IDs.
func (m *Manager) List() ([]string, error) {
	return m.ListJSONL()
}

// Delete removes a session.
func (m *Manager) Delete(sessionID string) error {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	return m.DeleteJSONL(sessionID)
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(message agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, message)
	s.UpdatedAt = time.Now()
	if s.manager != nil {
		_ = s.manager.saveSnapshot(s.snapshotLocked())
	}
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
	if s.manager != nil {
		_ = s.manager.saveSnapshot(s.snapshotLocked())
	}
}

// SetSummary sets the summary for this session.
func (s *Session) SetSummary(summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Summary = summary
	s.UpdatedAt = time.Now()
	if s.manager != nil {
		_ = s.manager.saveSnapshot(s.snapshotLocked())
	}
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

type sessionSnapshot struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []agent.Message
	Summary   string
	Source    string
}

func (s *Session) snapshotLocked() sessionSnapshot {
	messages := make([]agent.Message, len(s.Messages))
	copy(messages, s.Messages)
	return sessionSnapshot{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Messages:  messages,
		Summary:   s.Summary,
		Source:    s.Source,
	}
}

func (m *Manager) saveSnapshot(snapshot sessionSnapshot) error {
	if !m.shouldPersist(snapshot.Source) {
		return nil
	}

	filtered := m.filterMessages(snapshot.Messages, snapshot.Source)
	return m.SaveJSONL(snapshot.ID, sessionJSONMessagesFromAgent(filtered), map[string]interface{}{
		"summary":    snapshot.Summary,
		"source":     snapshot.Source,
		"created_at": snapshot.CreatedAt.Format(time.RFC3339Nano),
	})
}

func (m *Manager) shouldPersist(source string) bool {
	if !m.config.Enabled {
		return false
	}

	switch stringsTrimmed(source) {
	case SourceCLI:
		return m.config.Sources.CLI
	case SourceTUI:
		return m.config.Sources.TUI
	case SourceWebUI:
		return m.config.Sources.WebUI
	case SourceHeartbeat:
		return m.config.Sources.Heartbeat
	case SourceCron:
		return m.config.Sources.Cron
	case SourceChannels:
		return m.config.Sources.Channels
	case SourceGateway:
		return m.config.Sources.Gateway
	case "":
		return false
	default:
		return false
	}
}

func (m *Manager) filterMessages(messages []agent.Message, source string) []agent.Message {
	if !m.shouldPersist(source) {
		return nil
	}

	filtered := make([]agent.Message, 0, len(messages))
	for _, msg := range messages {
		keep, trimmed := m.filterMessage(msg)
		if keep {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}

func (m *Manager) filterMessage(msg agent.Message) (bool, agent.Message) {
	filtered := msg
	switch msg.Role {
	case "user":
		return m.config.Content.UserMessages, filtered
	case "assistant":
		if !m.config.Content.AssistantMessages && !m.config.Content.ToolCalls {
			return false, agent.Message{}
		}
		if !m.config.Content.AssistantMessages {
			filtered.Content = ""
		}
		if !m.config.Content.ToolCalls {
			filtered.ToolCalls = nil
		}
		if stringsTrimmed(filtered.Content) == "" && len(filtered.ToolCalls) == 0 {
			return false, agent.Message{}
		}
		return true, filtered
	case "tool":
		if !m.config.Content.ToolResults {
			return false, agent.Message{}
		}
		return true, filtered
	case "system":
		return m.config.Content.SystemMessages, filtered
	default:
		return false, agent.Message{}
	}
}

func stringsTrimmed(v string) string {
	return strings.TrimSpace(strings.ToLower(v))
}
