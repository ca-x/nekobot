// Package tasks defines a shared task state model for local and future remote execution units.
package tasks

import "time"

// Type identifies the kind of execution unit.
type Type string

const (
	TypeInteractiveMain   Type = "interactive_main"
	TypeLocalAgent        Type = "local_agent"
	TypeBackgroundAgent   Type = "background_agent"
	TypeRuntimeWorker     Type = "runtime_worker"
	TypeInProcessTeammate Type = "in_process_teammate"
	TypeRemoteAgent       Type = "remote_agent"
)

// State describes task lifecycle state.
type State string

const (
	StatePending        State = "pending"
	StateClaimed        State = "claimed"
	StateRunning        State = "running"
	StateRequiresAction State = "requires_action"
	StateFailed         State = "failed"
	StateCompleted      State = "completed"
	StateCanceled       State = "canceled"
)

// Task is the shared runtime view exposed by execution subsystems.
type Task struct {
	ID             string         `json:"id"`
	Type           Type           `json:"type"`
	State          State          `json:"state"`
	Summary        string         `json:"summary,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	RuntimeID      string         `json:"runtime_id,omitempty"`
	ActualProvider string         `json:"actual_provider,omitempty"`
	ActualModel    string         `json:"actual_model,omitempty"`
	PendingAction  string         `json:"pending_action,omitempty"`
	LastError      string         `json:"last_error,omitempty"`
	PermissionMode string         `json:"permission_mode,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	StartedAt      time.Time      `json:"started_at,omitempty"`
	CompletedAt    time.Time      `json:"completed_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// IsFinal reports whether the task is in a terminal state.
func IsFinal(state State) bool {
	return state == StateCompleted || state == StateFailed || state == StateCanceled
}
