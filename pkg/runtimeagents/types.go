package runtimeagents

import "time"

func NormalizeTimestamp(ts time.Time) *time.Time {
	if ts.IsZero() || ts.UTC().Year() <= 1 {
		return nil
	}
	normalized := ts
	return &normalized
}

// RuntimeDerivedStatus exposes non-persistent control-plane telemetry for one runtime.
type RuntimeDerivedStatus struct {
	EffectiveAvailable  bool       `json:"effective_available"`
	AvailabilityReason  string     `json:"availability_reason,omitempty"`
	BoundAccountCount   int        `json:"bound_account_count"`
	EnabledBindingCount int        `json:"enabled_binding_count"`
	CurrentTaskCount    int        `json:"current_task_count"`
	LastSeenAt          *time.Time `json:"last_seen_at,omitempty"`
}

// Normalize clears sentinel/zero timestamps that should not be exposed to API consumers.
func (s *RuntimeDerivedStatus) Normalize() {
	if s == nil {
		return
	}
	s.LastSeenAt = NormalizeTimestamp(timeValue(s.LastSeenAt))
}

func timeValue(ts *time.Time) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return *ts
}

// AgentRuntime defines one independently configurable runtime object.
type AgentRuntime struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	PromptID    string                 `json:"prompt_id"`
	Skills      []string               `json:"skills"`
	Tools       []string               `json:"tools"`
	Policy      map[string]interface{} `json:"policy"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Status      *RuntimeDerivedStatus  `json:"status,omitempty"`
}
