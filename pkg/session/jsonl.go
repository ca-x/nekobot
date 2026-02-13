package session

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Message represents a single message in a session.
type Message struct {
	Role       string                 `json:"role"` // user, assistant, system, tool
	Content    string                 `json:"content"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"` // For tool role
	ToolCalls  []ToolCall             `json:"tool_calls,omitempty"`   // For assistant role
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// SessionJSONL represents a session stored in JSONL format.
type SessionJSONL struct {
	Key       string
	Messages  []Message
	CreatedAt time.Time
	UpdatedAt time.Time
	Metadata  map[string]interface{}
}

// SaveJSONL saves a session in JSONL format (one JSON object per line).
// This format is more streaming-friendly and easier to process incrementally.
func (m *Manager) SaveJSONL(key string, messages []Message, metadata map[string]interface{}) error {
	path := m.getJSONLPath(key)

	// Create temp file
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	// Write metadata as first line
	metadataLine := map[string]interface{}{
		"_type":      "metadata",
		"key":        key,
		"created_at": time.Now(),
		"updated_at": time.Now(),
		"metadata":   metadata,
	}
	if err := encoder.Encode(metadataLine); err != nil {
		return fmt.Errorf("encoding metadata: %w", err)
	}

	// Write messages
	for _, msg := range messages {
		if err := encoder.Encode(msg); err != nil {
			return fmt.Errorf("encoding message: %w", err)
		}
	}

	file.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// LoadJSONL loads a session from JSONL format.
func (m *Manager) LoadJSONL(key string) (*SessionJSONL, error) {
	path := m.getJSONLPath(key)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	session := &SessionJSONL{
		Key:       key,
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue // Skip invalid lines
		}

		// Check if metadata line
		if msgType, ok := raw["_type"].(string); ok && msgType == "metadata" {
			if createdAt, ok := raw["created_at"].(string); ok {
				session.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
			}
			if updatedAt, ok := raw["updated_at"].(string); ok {
				session.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
			}
			if metadata, ok := raw["metadata"].(map[string]interface{}); ok {
				session.Metadata = metadata
			}
			continue
		}

		// Parse as message
		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			session.Messages = append(session.Messages, msg)
		}
	}

	return session, nil
}

// getJSONLPath returns the path for a JSONL session file.
func (m *Manager) getJSONLPath(key string) string {
	// Sanitize key
	safeKey := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, key)

	return m.baseDir + "/" + safeKey + ".jsonl"
}

// ListJSONL lists all JSONL session files.
func (m *Manager) ListJSONL() ([]string, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			key := strings.TrimSuffix(entry.Name(), ".jsonl")
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// DeleteJSONL deletes a JSONL session file.
func (m *Manager) DeleteJSONL(key string) error {
	path := m.getJSONLPath(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// AppendMessageJSONL appends a single message to an existing JSONL file.
// This is efficient for streaming scenarios.
func (m *Manager) AppendMessageJSONL(key string, msg Message) error {
	path := m.getJSONLPath(key)

	// Open file in append mode
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	// Check if file is empty (need to write metadata first)
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	if stat.Size() == 0 {
		// Write metadata first
		metadataLine := map[string]interface{}{
			"_type":      "metadata",
			"key":        key,
			"created_at": time.Now(),
			"updated_at": time.Now(),
			"metadata":   make(map[string]interface{}),
		}
		encoder := json.NewEncoder(file)
		if err := encoder.Encode(metadataLine); err != nil {
			return fmt.Errorf("encoding metadata: %w", err)
		}
	}

	// Append message
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("encoding message: %w", err)
	}

	return nil
}
