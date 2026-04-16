// Package tasks defines a shared task state model for local and future remote execution units.
package tasks

import (
	"sort"
	"strings"
	"time"
)

const (
	SessionLifecycleIdle          = "idle"
	SessionLifecycleProcessing    = "processing"
	SessionLifecycleAwaitingInput = "awaiting_input"
)

// SessionState is the shared runtime view for one active session-level state record.
type SessionState struct {
	SessionID        string    `json:"session_id"`
	LifecycleState   string    `json:"lifecycle_state,omitempty"`
	LifecycleDetail  string    `json:"lifecycle_detail,omitempty"`
	MaxToolRounds    int       `json:"max_tool_rounds,omitempty"`
	ToolRounds       int       `json:"tool_rounds,omitempty"`
	ToolCalls        map[string]int `json:"tool_calls,omitempty"`
	PerToolLimits    map[string]int `json:"per_tool_limits,omitempty"`
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
		state.LifecycleState = SessionLifecycleAwaitingInput
		state.LifecycleDetail = strings.TrimSpace(pendingAction)
		state.PendingAction = strings.TrimSpace(pendingAction)
		state.PendingRequestID = strings.TrimSpace(pendingRequestID)
	})
}

// ClearSessionPendingAction removes the tracked pending action for one session.
func (s *Store) ClearSessionPendingAction(sessionID string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.LifecycleState = SessionLifecycleIdle
		state.LifecycleDetail = ""
		state.PendingAction = ""
		state.PendingRequestID = ""
	})
}

func (s *Store) SetSessionLifecycleState(sessionID, lifecycleState, detail string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.LifecycleState = normalizeSessionLifecycleState(lifecycleState)
		state.LifecycleDetail = strings.TrimSpace(detail)
	})
}

func (s *Store) ClearSessionLifecycleState(sessionID string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.LifecycleState = ""
		state.LifecycleDetail = ""
	})
}

func (s *Store) SetSessionToolRoundLimit(sessionID string, maxRounds int) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		if maxRounds > 0 {
			state.MaxToolRounds = maxRounds
		} else {
			state.MaxToolRounds = 0
		}
	})
}

func (s *Store) EnsureSessionToolRoundLimit(sessionID string, maxRounds int) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		if state.MaxToolRounds > 0 {
			return
		}
		if maxRounds > 0 {
			state.MaxToolRounds = maxRounds
		}
	})
}

func (s *Store) RecordSessionToolRound(sessionID string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		state.ToolRounds++
	})
}

func (s *Store) RecordSessionToolCall(sessionID, toolName string) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			return
		}
		if state.ToolCalls == nil {
			state.ToolCalls = map[string]int{}
		}
		state.ToolCalls[toolName]++
	})
}

func (s *Store) CanStartSessionToolRound(sessionID string) bool {
	state, ok := s.GetSessionState(sessionID)
	if !ok {
		return true
	}
	if state.MaxToolRounds <= 0 {
		return true
	}
	return state.ToolRounds < state.MaxToolRounds
}

func (s *Store) SetSessionToolCallLimit(sessionID, toolName string, maxCalls int) {
	s.updateSessionState(sessionID, func(state *SessionState) {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			return
		}
		if state.PerToolLimits == nil {
			state.PerToolLimits = map[string]int{}
		}
		if maxCalls > 0 {
			state.PerToolLimits[toolName] = maxCalls
		} else {
			delete(state.PerToolLimits, toolName)
		}
	})
}

func (s *Store) CanExecuteSessionToolCall(sessionID, toolName string) bool {
	state, ok := s.GetSessionState(sessionID)
	if !ok {
		return true
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return true
	}
	limit, ok := state.PerToolLimits[toolName]
	if !ok || limit <= 0 {
		return true
	}
	return state.ToolCalls[toolName] < limit
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
		result = append(result, cloneSessionState(state))
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].SessionID < result[j].SessionID
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// GetSessionState returns one tracked session state by id.
func (s *Store) GetSessionState(sessionID string) (SessionState, bool) {
	if s == nil {
		return SessionState{}, false
	}

	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return SessionState{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.sessionStates[trimmedID]
	return cloneSessionState(state), ok
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
	state.LifecycleState = normalizeSessionLifecycleState(state.LifecycleState)
	state.LifecycleDetail = strings.TrimSpace(state.LifecycleDetail)
	state.PermissionMode = strings.TrimSpace(state.PermissionMode)
	state.PendingAction = strings.TrimSpace(state.PendingAction)
	state.PendingRequestID = strings.TrimSpace(state.PendingRequestID)
	if state.PendingAction == "" {
		state.PendingRequestID = ""
	}
	if state.LifecycleState == "" {
		if state.PendingAction != "" {
			state.LifecycleState = SessionLifecycleAwaitingInput
			if state.LifecycleDetail == "" {
				state.LifecycleDetail = state.PendingAction
			}
		} else if state.PermissionMode != "" {
			state.LifecycleState = SessionLifecycleIdle
		}
	}
	if state.PermissionMode == "" && state.PendingAction == "" &&
		(state.LifecycleState == "" || (state.LifecycleState == SessionLifecycleIdle && state.LifecycleDetail == "")) &&
		state.MaxToolRounds == 0 && state.ToolRounds == 0 && len(state.ToolCalls) == 0 && len(state.PerToolLimits) == 0 {
		delete(s.sessionStates, trimmedID)
		return
	}
	state.UpdatedAt = time.Now()
	s.sessionStates[trimmedID] = state
}

func normalizeSessionLifecycleState(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "":
		return ""
	case SessionLifecycleIdle:
		return SessionLifecycleIdle
	case SessionLifecycleProcessing:
		return SessionLifecycleProcessing
	case SessionLifecycleAwaitingInput:
		return SessionLifecycleAwaitingInput
	default:
		return ""
	}
}

func cloneSessionState(state SessionState) SessionState {
	if len(state.ToolCalls) == 0 {
		state.ToolCalls = nil
	} else {
		cloned := make(map[string]int, len(state.ToolCalls))
		for key, value := range state.ToolCalls {
			cloned[key] = value
		}
		state.ToolCalls = cloned
	}
	if len(state.PerToolLimits) == 0 {
		state.PerToolLimits = nil
	} else {
		clonedLimits := make(map[string]int, len(state.PerToolLimits))
		for key, value := range state.PerToolLimits {
			clonedLimits[key] = value
		}
		state.PerToolLimits = clonedLimits
	}
	return state
}
