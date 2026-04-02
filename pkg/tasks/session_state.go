// Package tasks defines a shared task state model for local and future remote execution units.
package tasks

import (
	"sort"
	"strings"
	"time"
)

// SessionState is the shared runtime view for one active session-level state record.
type SessionState struct {
	SessionID        string    `json:"session_id"`
	PermissionMode   string    `json:"permission_mode,omitempty"`
	PendingAction    string    `json:"pending_action,omitempty"`
	PendingRequestID string    `json:"pending_request_id,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// SetSessionPermissionMode updates the tracked permission mode for one session.
func (s *Store) SetSessionPermissionMode(sessionID, permissionMode string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.PermissionMode = strings.TrimSpace(permissionMode)
	})
}

// ClearSessionPermissionMode removes the tracked permission mode for one session.
func (s *Store) ClearSessionPermissionMode(sessionID string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.PermissionMode = ""
	})
}

// SetSessionPendingAction updates the tracked pending action for one session.
func (s *Store) SetSessionPendingAction(sessionID, pendingAction, pendingRequestID string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.PendingAction = strings.TrimSpace(pendingAction)
		state.PendingRequestID = strings.TrimSpace(pendingRequestID)
	})
}

// ClearSessionPendingAction removes the tracked pending action for one session.
func (s *Store) ClearSessionPendingAction(sessionID string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.PendingAction = ""
		state.PendingRequestID = ""
	})
}

// ListSessionStates returns all tracked session states sorted by most recent update.
func (s *Store) ListSessionStates() []SessionState {
	if s == nil {
		return []SessionState{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.sessionStates) == 0 {
		return []SessionState{}
	}

	result := make([]SessionState, 0, len(s.sessionStates))
	for _, state := range s.sessionStates {
		result = append(result, state)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].SessionID < result[j].SessionID
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

func (s *Store) updateSessionState(sessionID string, update func(*SessionState)) {
	if s == nil || update == nil {
		return
	}

	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionStates == nil {
		s.sessionStates = make(map[string]SessionState)
	}

	state := s.sessionStates[trimmedID]
	state.SessionID = trimmedID
	update(&state)
	state.PermissionMode = strings.TrimSpace(state.PermissionMode)
	state.PendingAction = strings.TrimSpace(state.PendingAction)
	state.PendingRequestID = strings.TrimSpace(state.PendingRequestID)
	if state.PendingAction == "" {
		state.PendingRequestID = ""
	}
	if state.PermissionMode == "" && state.PendingAction == "" {
		delete(s.sessionStates, trimmedID)
		return
	}
	state.UpdatedAt = time.Now()
	s.sessionStates[trimmedID] = state
}
