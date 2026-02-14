// Package approval provides a tool execution approval system.
// It supports three modes:
//   - auto: all tool calls are automatically approved
//   - prompt: tool calls trigger a confirmation prompt in the chat
//   - manual: tool calls are queued for explicit approval via CLI
package approval

import (
	"fmt"
	"sync"
)

// Mode defines the approval behavior.
type Mode string

const (
	ModeAuto   Mode = "auto"   // Auto-approve all tool calls
	ModePrompt Mode = "prompt" // Ask user in chat before executing
	ModeManual Mode = "manual" // Queue for explicit approval
)

// Decision represents an approval decision.
type Decision string

const (
	Approved Decision = "approved"
	Denied   Decision = "denied"
	Pending  Decision = "pending"
)

// Request represents a pending approval request.
type Request struct {
	ID        string                 `json:"id"`
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	SessionID string                 `json:"session_id"`
	Decision  Decision               `json:"decision"`
	Reason    string                 `json:"reason,omitempty"`
}

// Config configures the approval system.
type Config struct {
	Mode      Mode     `json:"mode"`       // Approval mode
	Allowlist []string `json:"allowlist"`   // Tools that bypass approval (always auto-approved)
	Denylist  []string `json:"denylist"`    // Tools that are always denied
}

// Manager handles tool execution approvals.
type Manager struct {
	config   Config
	pending  map[string]*Request
	mu       sync.RWMutex
	counter  int
	// PromptFunc is called in prompt mode to ask the user.
	// Returns true if approved. Nil means auto-approve.
	PromptFunc func(req *Request) (bool, error)
}

// NewManager creates a new approval manager.
func NewManager(cfg Config) *Manager {
	return &Manager{
		config:  cfg,
		pending: make(map[string]*Request),
	}
}

// CheckApproval determines whether a tool call should be approved.
// Returns the decision and, for pending requests, the request ID.
func (m *Manager) CheckApproval(toolName string, args map[string]interface{}, sessionID string) (Decision, string, error) {
	// Check denylist first
	if m.isInList(toolName, m.config.Denylist) {
		return Denied, "", nil
	}

	// Check allowlist (bypass approval)
	if m.isInList(toolName, m.config.Allowlist) {
		return Approved, "", nil
	}

	switch m.config.Mode {
	case ModeAuto:
		return Approved, "", nil

	case ModePrompt:
		if m.PromptFunc == nil {
			return Approved, "", nil
		}
		req := &Request{
			ToolName:  toolName,
			Arguments: args,
			SessionID: sessionID,
		}
		approved, err := m.PromptFunc(req)
		if err != nil {
			return Denied, "", fmt.Errorf("prompt error: %w", err)
		}
		if approved {
			return Approved, "", nil
		}
		return Denied, "", nil

	case ModeManual:
		id := m.enqueue(toolName, args, sessionID)
		return Pending, id, nil

	default:
		return Approved, "", nil
	}
}

// Approve approves a pending request by ID.
func (m *Manager) Approve(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.pending[id]
	if !ok {
		return fmt.Errorf("request not found: %s", id)
	}
	req.Decision = Approved
	return nil
}

// Deny denies a pending request by ID with an optional reason.
func (m *Manager) Deny(id string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.pending[id]
	if !ok {
		return fmt.Errorf("request not found: %s", id)
	}
	req.Decision = Denied
	req.Reason = reason
	return nil
}

// GetPending returns all pending approval requests.
func (m *Manager) GetPending() []*Request {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var requests []*Request
	for _, req := range m.pending {
		if req.Decision == Pending {
			requests = append(requests, req)
		}
	}
	return requests
}

// GetDecision returns the decision for a specific request.
func (m *Manager) GetDecision(id string) (Decision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, ok := m.pending[id]
	if !ok {
		return "", fmt.Errorf("request not found: %s", id)
	}
	return req.Decision, nil
}

// Cleanup removes resolved requests.
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, req := range m.pending {
		if req.Decision != Pending {
			delete(m.pending, id)
		}
	}
}

func (m *Manager) enqueue(toolName string, args map[string]interface{}, sessionID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter++
	id := fmt.Sprintf("approval-%d", m.counter)
	m.pending[id] = &Request{
		ID:        id,
		ToolName:  toolName,
		Arguments: args,
		SessionID: sessionID,
		Decision:  Pending,
	}
	return id
}

func (m *Manager) isInList(name string, list []string) bool {
	for _, item := range list {
		if item == name || item == "*" {
			return true
		}
	}
	return false
}
