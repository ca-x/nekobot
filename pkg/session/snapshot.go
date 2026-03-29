// Package session provides turn-based snapshot storage for undo functionality.
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nekobot/pkg/config"
)

// MessageSnapshot is a simplified message structure for snapshot storage.
// This is used to avoid circular imports with pkg/agent.
type MessageSnapshot struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	ToolCalls  []ToolCallSnapshot     `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCallSnapshot represents a tool invocation in a snapshot.
type ToolCallSnapshot struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// TurnSnapshot represents a single turn snapshot in a session.
type TurnSnapshot struct {
	// Turn is the turn number (0-indexed).
	Turn int `json:"turn"`
	// Timestamp is when this snapshot was taken.
	Timestamp time.Time `json:"timestamp"`
	// Messages is the complete message history up to this turn.
	Messages []MessageSnapshot `json:"messages"`
	// Summary is the session summary at this turn (if any).
	Summary string `json:"summary,omitempty"`
	// IsCheckpoint marks this as a checkpoint (full snapshot vs incremental).
	IsCheckpoint bool `json:"is_checkpoint"`
	// Delta contains only the changes from the previous turn if IsCheckpoint is false.
	Delta *TurnDelta `json:"delta,omitempty"`
}

// TurnDelta represents the difference between consecutive turns.
type TurnDelta struct {
	// AddedMessages are messages added since the last snapshot.
	AddedMessages []MessageSnapshot `json:"added,omitempty"`
	// PreviousTurn is the turn number of the previous snapshot.
	PreviousTurn int `json:"previous_turn"`
}

// SnapshotStore manages turn-based snapshots for a session.
type SnapshotStore struct {
	sessionID   string
	baseDir     string
	config      config.UndoConfig
	snapshots   []TurnSnapshot
	currentTurn int
	mu          sync.RWMutex
}

// NewSnapshotStore creates a new snapshot store for a session.
func NewSnapshotStore(sessionID, baseDir string, cfg config.UndoConfig) *SnapshotStore {
	return &SnapshotStore{
		sessionID:   sessionID,
		baseDir:     baseDir,
		config:      cfg,
		snapshots:   make([]TurnSnapshot, 0),
		currentTurn: -1, // No turns yet
	}
}

// getSnapshotPath returns the path for the snapshot file.
func (s *SnapshotStore) getSnapshotPath() string {
	// Sanitize session ID for filesystem
	safeID := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, s.sessionID)
	return filepath.Join(s.baseDir, safeID+".snapshots.jsonl")
}

// SaveSnapshot saves a snapshot before a new turn.
// It uses incremental snapshots by default, with periodic checkpoints.
func (s *SnapshotStore) SaveSnapshot(messages []MessageSnapshot, summary string) error {
	if !s.config.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentTurn++
	turn := s.currentTurn

	// Determine if this should be a checkpoint (full snapshot)
	// Checkpoint every N turns or on first turn
	isCheckpoint := turn == 0 || turn%s.config.MaxTurns == 0

	snapshot := TurnSnapshot{
		Turn:         turn,
		Timestamp:    time.Now(),
		IsCheckpoint: isCheckpoint,
		Summary:      summary,
	}

	if isCheckpoint {
		// Full snapshot
		snapshot.Messages = make([]MessageSnapshot, len(messages))
		copy(snapshot.Messages, messages)
	} else {
		// Incremental snapshot - only store delta
		var prevTurn int
		if len(s.snapshots) > 0 {
			prevTurn = s.snapshots[len(s.snapshots)-1].Turn
		} else {
			prevTurn = -1
		}

		// Calculate delta against the previous fully reconstructed state.
		var addedMessages []MessageSnapshot
		if prevTurn >= 0 && prevTurn < len(s.snapshots) {
			prevSnapshot := s.snapshots[len(s.snapshots)-1]
			prevMessages, err := s.reconstructMessagesLocked(prevSnapshot)
			if err != nil {
				return fmt.Errorf("reconstructing previous snapshot: %w", err)
			}
			prevMsgCount := len(prevMessages)

			if len(messages) > prevMsgCount {
				addedMessages = make([]MessageSnapshot, len(messages)-prevMsgCount)
				copy(addedMessages, messages[prevMsgCount:])
			}
		} else {
			// First incremental or no previous data
			addedMessages = make([]MessageSnapshot, len(messages))
			copy(addedMessages, messages)
		}

		snapshot.Delta = &TurnDelta{
			AddedMessages: addedMessages,
			PreviousTurn:  prevTurn,
		}
	}

	s.snapshots = append(s.snapshots, snapshot)

	// Enforce max turns limit
	if len(s.snapshots) > s.config.MaxTurns {
		// Keep only the most recent MaxTurns snapshots, ensuring first is a checkpoint
		cutoff := len(s.snapshots) - s.config.MaxTurns
		if cutoff > 0 {
			// Find next checkpoint after cutoff
			checkpointIdx := -1
			for i := cutoff; i < len(s.snapshots); i++ {
				if s.snapshots[i].IsCheckpoint {
					checkpointIdx = i
					break
				}
			}
			if checkpointIdx > 0 {
				s.snapshots = s.snapshots[checkpointIdx:]
				// Renumber turns
				for i := range s.snapshots {
					s.snapshots[i].Turn = i
				}
				s.currentTurn = len(s.snapshots) - 1
			}
		}
	}

	// Write to disk using JSONL format
	return s.writeSnapshot(snapshot)
}

// writeSnapshot appends a snapshot to the JSONL file.
func (s *SnapshotStore) writeSnapshot(snapshot TurnSnapshot) error {
	path := s.getSnapshotPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating snapshot directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening snapshot file: %w", err)
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("encoding snapshot: %w", err)
	}

	return nil
}

func (s *SnapshotStore) rewriteSnapshotsLocked() error {
	path := s.getSnapshotPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating snapshot directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating snapshot file: %w", err)
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	for _, snapshot := range s.snapshots {
		if err := encoder.Encode(snapshot); err != nil {
			return fmt.Errorf("encoding snapshot: %w", err)
		}
	}

	return nil
}

// LoadSnapshots loads all snapshots for the session from disk.
func (s *SnapshotStore) LoadSnapshots() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.getSnapshotPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No snapshots yet
		}
		return fmt.Errorf("reading snapshot file: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var snapshots []TurnSnapshot

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var snapshot TurnSnapshot
		if err := json.Unmarshal([]byte(line), &snapshot); err != nil {
			continue // Skip invalid lines
		}
		snapshots = append(snapshots, snapshot)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning snapshot file: %w", err)
	}

	s.snapshots = snapshots
	if len(snapshots) > 0 {
		s.currentTurn = snapshots[len(snapshots)-1].Turn
	}

	return nil
}

// Undo reverts to the previous turn and returns the reverted messages.
// Returns nil if already at the beginning.
func (s *SnapshotStore) Undo() ([]MessageSnapshot, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("undo is disabled")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.snapshots) <= 1 {
		// No previous turn to revert to
		return nil, nil
	}

	// Remove the latest snapshot
	s.snapshots = s.snapshots[:len(s.snapshots)-1]

	// Get the state at the previous turn
	previousSnapshot := s.snapshots[len(s.snapshots)-1]
	s.currentTurn = previousSnapshot.Turn

	// Reconstruct full message list
	messages, err := s.reconstructMessagesLocked(previousSnapshot)
	if err != nil {
		return nil, fmt.Errorf("reconstructing messages: %w", err)
	}

	if err := s.rewriteSnapshotsLocked(); err != nil {
		return nil, fmt.Errorf("rewriting snapshots: %w", err)
	}

	return messages, nil
}

// GetCurrentMessages returns the messages at the current turn.
func (s *SnapshotStore) GetCurrentMessages() ([]MessageSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.snapshots) == 0 {
		return []MessageSnapshot{}, nil
	}

	latest := s.snapshots[len(s.snapshots)-1]
	return s.reconstructMessagesLocked(latest)
}

// reconstructMessagesLocked reconstructs the full message list from a snapshot.
// Must be called with s.mu held.
func (s *SnapshotStore) reconstructMessagesLocked(snapshot TurnSnapshot) ([]MessageSnapshot, error) {
	if snapshot.IsCheckpoint {
		// Full snapshot - just copy
		messages := make([]MessageSnapshot, len(snapshot.Messages))
		copy(messages, snapshot.Messages)
		return messages, nil
	}

	// Incremental - need to replay from last checkpoint
	// Find the last checkpoint before this snapshot
	checkpointIdx := -1
	for i := snapshot.Turn; i >= 0; i-- {
		if i < len(s.snapshots) && s.snapshots[i].IsCheckpoint {
			checkpointIdx = i
			break
		}
	}

	if checkpointIdx < 0 {
		return nil, fmt.Errorf("no checkpoint found for incremental snapshot")
	}

	// Start with checkpoint messages
	result := make([]MessageSnapshot, len(s.snapshots[checkpointIdx].Messages))
	copy(result, s.snapshots[checkpointIdx].Messages)

	// Apply deltas
	for i := checkpointIdx + 1; i <= snapshot.Turn && i < len(s.snapshots); i++ {
		snap := s.snapshots[i]
		if snap.Delta != nil {
			result = append(result, snap.Delta.AddedMessages...)
		}
	}

	return result, nil
}

// CanUndo returns true if undo is possible.
func (s *SnapshotStore) CanUndo() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots) > 1
}

// GetTurnCount returns the number of turns stored.
func (s *SnapshotStore) GetTurnCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}

// GetCurrentTurn returns the current turn number.
func (s *SnapshotStore) GetCurrentTurn() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTurn
}

// Clear removes all snapshots.
func (s *SnapshotStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.getSnapshotPath()
	s.snapshots = nil
	s.currentTurn = -1

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing snapshot file: %w", err)
	}

	return nil
}

// ListSnapshots returns metadata about all stored snapshots.
func (s *SnapshotStore) ListSnapshots() ([]SnapshotInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	infos := make([]SnapshotInfo, len(s.snapshots))
	for i, snap := range s.snapshots {
		infos[i] = SnapshotInfo{
			Turn:         snap.Turn,
			Timestamp:    snap.Timestamp,
			IsCheckpoint: snap.IsCheckpoint,
			MessageCount: len(snap.Messages),
		}
		if snap.Delta != nil {
			infos[i].DeltaMessageCount = len(snap.Delta.AddedMessages)
		}
	}
	return infos, nil
}

// SnapshotInfo contains metadata about a snapshot.
type SnapshotInfo struct {
	Turn              int       `json:"turn"`
	Timestamp         time.Time `json:"timestamp"`
	IsCheckpoint      bool      `json:"is_checkpoint"`
	MessageCount      int       `json:"message_count"`
	DeltaMessageCount int       `json:"delta_message_count,omitempty"`
}

// ToMessageSnapshot converts an agent.Message to a MessageSnapshot.
// This function is designed to avoid circular imports by not directly importing pkg/agent.
func ToMessageSnapshot(role, content, toolCallID string, toolCalls []interface{}) MessageSnapshot {
	snapshot := MessageSnapshot{
		Role:       role,
		Content:    content,
		ToolCallID: toolCallID,
	}
	if len(toolCalls) > 0 {
		snapshot.ToolCalls = make([]ToolCallSnapshot, len(toolCalls))
		for i, tc := range toolCalls {
			if tcm, ok := tc.(interface{ GetID() string }); ok {
				snapshot.ToolCalls[i].ID = tcm.GetID()
			}
			if tcm, ok := tc.(interface{ GetName() string }); ok {
				snapshot.ToolCalls[i].Name = tcm.GetName()
			}
			if tcm, ok := tc.(interface{ GetArguments() map[string]interface{} }); ok {
				snapshot.ToolCalls[i].Arguments = tcm.GetArguments()
			}
		}
	}
	return snapshot
}

// SnapshotManager manages snapshot stores for all sessions.
type SnapshotManager struct {
	baseDir string
	config  config.UndoConfig
	stores  map[string]*SnapshotStore
	mu      sync.RWMutex
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(baseDir string, cfg config.UndoConfig) *SnapshotManager {
	return &SnapshotManager{
		baseDir: baseDir,
		config:  cfg,
		stores:  make(map[string]*SnapshotStore),
	}
}

// GetStore returns or creates a snapshot store for a session.
func (m *SnapshotManager) GetStore(sessionID string) *SnapshotStore {
	m.mu.Lock()
	defer m.mu.Unlock()

	if store, exists := m.stores[sessionID]; exists {
		return store
	}

	store := NewSnapshotStore(sessionID, m.baseDir, m.config)
	m.stores[sessionID] = store
	return store
}

// RemoveStore removes a snapshot store.
func (m *SnapshotManager) RemoveStore(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if store, exists := m.stores[sessionID]; exists {
		if err := store.Clear(); err != nil {
			return err
		}
		delete(m.stores, sessionID)
	}
	return nil
}

// ListStores returns all session IDs with snapshot stores.
func (m *SnapshotManager) ListStores() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.stores))
	for id := range m.stores {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
