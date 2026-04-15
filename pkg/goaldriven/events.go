package goaldriven

import "time"

// Event captures a GoalRun lifecycle record.
type Event struct {
	ID        string         `json:"id"`
	GoalRunID string         `json:"goal_run_id"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}
